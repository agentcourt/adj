package proceeding

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jsmorph/adj/common/persona"
)

func loadCaseFiles(dir string) ([]CaseFile, error) {
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
		file, err := loadCaseFile(filepath.Join(dir, name), name)
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
	if info.IsDir() {
		return CaseFile{}, fmt.Errorf("case file %s is a directory", name)
	}
	file := CaseFile{
		EvidenceID:   name,
		Name:         name,
		Path:         path,
		MimeType:     mimeType,
		TextReadable: readable,
		SizeBytes:    int(info.Size()),
	}
	if readable {
		raw, err := os.ReadFile(path)
		if err != nil {
			return CaseFile{}, fmt.Errorf("read case file %s: %w", name, err)
		}
		file.Text = string(raw)
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
			Status:      "seated",
			RequestSpec: spec.RequestSpec,
			PersonaText: spec.Text,
		})
	}
	return out, nil
}

func councilSeatMaps(council []CouncilSeat) []map[string]any {
	out := make([]map[string]any, 0, len(council))
	for _, seat := range council {
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
	return out
}

func councilSeatRoster(council []CouncilSeat, caseMembers []map[string]any) []map[string]any {
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
		if requestSpec := councilSeatRequestSpecMap(seat); len(requestSpec) > 0 {
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
	return out
}

func councilSeatRequestSpecMap(seat CouncilSeat) map[string]any {
	if seat.RequestSpec != nil {
		raw, err := json.Marshal(seat.RequestSpec)
		if err == nil {
			var out map[string]any
			if json.Unmarshal(raw, &out) == nil {
				return out
			}
		}
	}
	return nil
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

func appendJSONLine(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write(append(raw, '\n')); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeJSONFileAtomic(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp json file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
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

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
