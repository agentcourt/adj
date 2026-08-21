package proceeding

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jsmorph/adj/common/persona"
)

func loadCaseFiles(dir string, complaintPath string) ([]CaseFile, error) {
	complaintInfo, err := os.Stat(complaintPath)
	if err != nil {
		return nil, fmt.Errorf("stat complaint: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read case dir: %w", err)
	}
	out := make([]CaseFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if skipCaseFile(name) {
			continue
		}
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat case file %s: %w", name, err)
		}
		if os.SameFile(complaintInfo, info) {
			continue
		}
		file, err := loadCaseFile(path, name)
		if err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	slices.SortFunc(out, func(a, b CaseFile) int {
		return strings.Compare(a.EvidenceID, b.EvidenceID)
	})
	return out, nil
}

func loadCaseFilesFromPaths(paths []string) ([]CaseFile, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no case files specified")
	}
	out := make([]CaseFile, 0, len(paths))
	seen := map[string]string{}
	for _, rawPath := range paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			return nil, fmt.Errorf("case file path must not be empty")
		}
		name := filepath.Base(path)
		if prior, ok := seen[name]; ok {
			return nil, fmt.Errorf("duplicate case file name %q from %s and %s", name, prior, path)
		}
		file, err := loadCaseFile(path, name)
		if err != nil {
			return nil, err
		}
		seen[name] = path
		out = append(out, file)
	}
	slices.SortFunc(out, func(a, b CaseFile) int {
		return strings.Compare(a.EvidenceID, b.EvidenceID)
	})
	return out, nil
}

func loadCaseFile(path string, name string) (CaseFile, error) {
	mimeType, readable := caseFileKind(name)
	info, err := os.Stat(path)
	if err != nil {
		return CaseFile{}, fmt.Errorf("stat case file %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return CaseFile{}, fmt.Errorf("case file %s is not a regular file", name)
	}
	file := CaseFile{
		EvidenceID:   name,
		Name:         name,
		Path:         path,
		MimeType:     mimeType,
		TextReadable: readable,
		SizeBytes:    int(info.Size()),
	}
	return file, nil
}

func caseFileKind(name string) (string, bool) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt":
		return "text/plain", true
	case ".md":
		return "text/markdown", true
	case ".pem":
		return "application/x-pem-file", true
	case ".b64":
		return "text/plain", true
	default:
		return "application/octet-stream", false
	}
}

func skipCaseFile(name string) bool {
	if strings.HasSuffix(name, "~") {
		return true
	}
	switch name {
	case ".gitignore", "README.md", "complaint.md", "situation.md", "sign.sh", "confession.sig", "samantha_private.pem":
		return true
	default:
		return false
	}
}

func councilPoolMeta(path string, baseDir string) ([]persona.Spec, error) {
	specs, err := persona.LoadRecordsFile(path, baseDir)
	if err != nil {
		return nil, err
	}
	for index, spec := range specs {
		if spec.RequestSpec == nil {
			return nil, fmt.Errorf("council pool record %d has no request_spec; JSONL request-spec records are required", index+1)
		}
	}
	return specs, nil
}

func sampleCouncil(path string, baseDir string, count int) ([]CouncilSeat, error) {
	specs, err := councilPoolMeta(path, baseDir)
	if err != nil {
		return nil, err
	}
	if count <= 0 {
		return nil, fmt.Errorf("council size must be positive")
	}
	if count > len(specs) {
		return nil, fmt.Errorf("council size %d exceeds available pool %d", count, len(specs))
	}
	indexes := make([]int, len(specs))
	for i := range specs {
		indexes[i] = i
	}
	out := make([]CouncilSeat, 0, count)
	for i := 0; i < count; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(indexes))))
		if err != nil {
			return nil, fmt.Errorf("sample council pool: %w", err)
		}
		pick := int(n.Int64())
		spec := specs[indexes[pick]]
		indexes = append(indexes[:pick], indexes[pick+1:]...)
		out = append(out, CouncilSeat{
			MemberID:    fmt.Sprintf("C%d", i+1),
			Model:       spec.Model,
			PersonaFile: spec.File,
			RequestSpec: spec.RequestSpec,
			PersonaText: spec.Text,
		})
	}
	return out, nil
}

func councilSeatMaps(council []CouncilSeat) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(council))
	for _, seat := range council {
		if _, err := councilSeatRequestSpecMap(seat); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"member_id":              seat.MemberID,
			"model":                  seat.Model,
			"persona_filename":       seat.PersonaFile,
			"status":                 "seated",
			"failure_reason":         "",
			"failure_opportunity_id": "",
			"failure_message":        "",
		})
	}
	return out, nil
}

func councilSeatRoster(council []CouncilSeat, caseMembers []map[string]any) ([]map[string]any, error) {
	statusByID := map[string]string{}
	failureByID := map[string]map[string]any{}
	for _, member := range caseMembers {
		memberID := mapString(member["member_id"])
		if memberID == "" {
			continue
		}
		statusByID[memberID] = mapString(member["status"])
		failureByID[memberID] = map[string]any{
			"failure_reason":         mapString(member["failure_reason"]),
			"failure_opportunity_id": mapString(member["failure_opportunity_id"]),
			"failure_message":        mapString(member["failure_message"]),
		}
	}
	out := make([]map[string]any, 0, len(council))
	for _, seat := range council {
		status := statusByID[seat.MemberID]
		if status == "" {
			status = "seated"
		}
		entry := map[string]any{
			"member_id":        seat.MemberID,
			"model":            seat.Model,
			"persona_filename": seat.PersonaFile,
			"status":           status,
		}
		if status == "failed" {
			for key, value := range failureByID[seat.MemberID] {
				if mapString(value) != "" {
					entry[key] = value
				}
			}
		}
		requestSpec, err := councilSeatRequestSpecMap(seat)
		if err != nil {
			return nil, err
		}
		if len(requestSpec) > 0 {
			entry["request_spec"] = requestSpec
			if provider, _ := requestSpec["provider"].(map[string]any); len(provider) > 0 {
				entry["provider"] = provider
			}
			if request, _ := requestSpec["request"].(map[string]any); len(request) > 0 {
				entry["request"] = request
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

func councilSeatRequestSpecMap(seat CouncilSeat) (map[string]any, error) {
	if seat.RequestSpec == nil {
		return nil, nil
	}
	raw, err := json.Marshal(seat.RequestSpec)
	if err != nil {
		return nil, fmt.Errorf("marshal council member %s request spec: %w", seat.MemberID, err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal council member %s request spec: %w", seat.MemberID, err)
	}
	return out, nil
}

func caseFileMetas(files []CaseFile) []CaseFileMeta {
	out := make([]CaseFileMeta, 0, len(files))
	for _, file := range files {
		out = append(out, CaseFileMeta{
			EvidenceID:   file.EvidenceID,
			Name:         file.Name,
			MimeType:     file.MimeType,
			TextReadable: file.TextReadable,
		})
	}
	return out
}

func finalCouncil(council []CouncilSeat, state map[string]any) []CouncilSeat {
	caseObj := mapAny(state["case"])
	rawCouncil := mapList(caseObj["council_members"])
	byID := map[string]map[string]any{}
	for _, raw := range rawCouncil {
		memberID := mapString(raw["member_id"])
		if memberID != "" {
			byID[memberID] = raw
		}
	}
	out := make([]CouncilSeat, 0, len(council))
	seen := map[string]bool{}
	for _, seat := range council {
		raw := byID[seat.MemberID]
		status := mapString(raw["status"])
		if status == "" {
			status = seat.Status
		}
		if status == "" {
			status = "seated"
		}
		seat.Status = status
		if status == "failed" {
			seat.FailureReason = mapString(raw["failure_reason"])
			seat.FailureOpportunityID = mapString(raw["failure_opportunity_id"])
			seat.FailureMessage = mapString(raw["failure_message"])
		}
		out = append(out, seat)
		seen[seat.MemberID] = true
	}
	for _, raw := range rawCouncil {
		memberID := mapString(raw["member_id"])
		if memberID == "" || seen[memberID] {
			continue
		}
		status := mapString(raw["status"])
		if status == "" {
			status = "seated"
		}
		out = append(out, CouncilSeat{
			MemberID:             memberID,
			Model:                mapString(raw["model"]),
			PersonaFile:          mapString(raw["persona_filename"]),
			Status:               status,
			FailureReason:        mapString(raw["failure_reason"]),
			FailureOpportunityID: mapString(raw["failure_opportunity_id"]),
			FailureMessage:       mapString(raw["failure_message"]),
		})
	}
	return out
}

func appendJSONLine(path string, value any) (err error) {
	wire, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", path, closeErr))
		}
	}()
	if _, err := f.Write(append(wire, '\n')); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	wire, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	wire = append(wire, '\n')
	if err := os.WriteFile(path, wire, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func cloneMapJSON(in map[string]any) (map[string]any, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func readJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(target); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("parse %s after first JSON value: %w", path, err)
	}
	return fmt.Errorf("parse %s: file must contain exactly one JSON value", path)
}

func writeJSONFileAtomic(path string, value any) (err error) {
	wire, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	wire = append(wire, '\n')
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp json file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	closed := false
	renamed := false
	defer func() {
		if !closed {
			if closeErr := tmp.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close %s: %w", tmpName, closeErr))
			}
		}
		if !renamed {
			if removeErr := os.Remove(tmpName); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("remove %s: %w", tmpName, removeErr))
			}
		}
	}()
	if _, err := tmp.Write(wire); err != nil {
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		closed = true
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	closed = true
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	renamed = true
	return nil
}

func mapString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func optionalStringParam(params map[string]any, key string) (string, error) {
	value, ok := params[key]
	if !ok {
		return "", nil
	}
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return strings.TrimSpace(s), nil
}

func requiredIntParam(params map[string]any, key string) (int, error) {
	value, ok := params[key]
	if !ok || value == nil {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		if float64(int(v)) == v {
			return int(v), nil
		}
	case json.Number:
		n, err := v.Int64()
		if err == nil {
			return int(n), nil
		}
	}
	return 0, fmt.Errorf("%s must be an integer", key)
}

func mapAny(value any) map[string]any {
	out, _ := value.(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	return out
}

func mapList(value any) []map[string]any {
	switch v := value.(type) {
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, raw := range v {
			entry, _ := raw.(map[string]any)
			if entry != nil {
				out = append(out, entry)
			}
		}
		return out
	default:
		return nil
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneJSONLikeMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneJSONLikeValue(value)
	}
	return out
}

func cloneJSONLikeMapList(in []map[string]any) []map[string]any {
	if in == nil {
		return nil
	}
	out := make([]map[string]any, len(in))
	for i, value := range in {
		out[i] = cloneJSONLikeMap(value)
	}
	return out
}

func cloneJSONLikeValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneJSONLikeMap(value)
	case []map[string]any:
		return cloneJSONLikeMapList(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = cloneJSONLikeValue(item)
		}
		return out
	case map[string]string:
		out := make(map[string]string, len(value))
		for key, item := range value {
			out[key] = item
		}
		return out
	case []string:
		return append([]string(nil), value...)
	case []byte:
		return append([]byte(nil), value...)
	default:
		return value
	}
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
