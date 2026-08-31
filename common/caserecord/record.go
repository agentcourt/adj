package caserecord

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/casemanifest"
)

type layout struct {
	recordRoot string
	corePrefix string
	manifest   casemanifest.Manifest
}

type builder struct {
	layout     layout
	record     Record
	artifacts  map[string]Artifact
	docketKeys map[string]bool
	nextOrder  int
}

func Build(options Options) (Record, error) {
	layout, err := discoverLayout(options.Dir)
	if err != nil {
		return Record{}, err
	}
	b := &builder{
		layout: layout,
		record: Record{
			SchemaVersion: SchemaVersion,
			GeneratedAt:   time.Now().UTC(),
			Procedure:     layout.manifest.Procedure,
			CaseID:        layout.manifest.CaseID,
			RunID:         layout.manifest.RunID,
			Sources: []Source{{
				ID:        "record",
				Kind:      "record",
				Path:      layout.recordRoot,
				PathBase:  SourcePathBaseAbsolute,
				Available: true,
			}},
		},
		artifacts:  make(map[string]Artifact),
		docketKeys: make(map[string]bool),
	}
	if err := b.catalogRecord(options); err != nil {
		return Record{}, err
	}
	if options.IncludeSessions {
		if err := b.catalogSessions(); err != nil {
			return Record{}, err
		}
	}
	if err := b.buildDocket(options.IncludeWorkNotes); err != nil {
		return Record{}, err
	}
	b.finish()
	return b.record, nil
}

func discoverLayout(dir string) (layout, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return layout{}, fmt.Errorf("case record directory is required")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return layout{}, fmt.Errorf("inspect case record directory %s: %w", dir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return layout{}, fmt.Errorf("case record directory %s is a symbolic link", dir)
	}
	if !info.IsDir() {
		return layout{}, fmt.Errorf("case record path %s is not a directory", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return layout{}, fmt.Errorf("resolve case record directory %s: %w", dir, err)
	}
	candidates := []string{"", "core", "aar-output", "aard-output", "adc-output"}
	var found []string
	for _, candidate := range candidates {
		manifestPath := filepath.Join(abs, candidate, casemanifest.FileName)
		manifestInfo, statErr := os.Lstat(manifestPath)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return layout{}, fmt.Errorf("inspect case manifest %s: %w", manifestPath, statErr)
		}
		if !manifestInfo.Mode().IsRegular() {
			return layout{}, fmt.Errorf("case manifest %s is not a regular file", manifestPath)
		}
		found = append(found, candidate)
	}
	if len(found) == 0 {
		return layout{}, fmt.Errorf("no case manifest found in %s or a documented core child", abs)
	}
	if len(found) != 1 {
		return layout{}, fmt.Errorf("case record directory %s contains multiple case manifests in documented locations", abs)
	}
	coreDir := filepath.Join(abs, found[0])
	manifest, err := readManifest(filepath.Join(coreDir, casemanifest.FileName))
	if err != nil {
		return layout{}, err
	}
	return layout{
		recordRoot: abs,
		corePrefix: filepath.ToSlash(found[0]),
		manifest:   manifest,
	}, nil
}

func readManifest(path string) (casemanifest.Manifest, error) {
	var manifest casemanifest.Manifest
	if err := readJSON(path, &manifest); err != nil {
		return manifest, fmt.Errorf("read case manifest: %w", err)
	}
	if manifest.SchemaVersion != casemanifest.SchemaVersion {
		return manifest, fmt.Errorf("unsupported case manifest schema %q", manifest.SchemaVersion)
	}
	switch manifest.Procedure {
	case casemanifest.ProcedureADC, casemanifest.ProcedureARB, casemanifest.ProcedureARBD, casemanifest.ProcedureSimple, casemanifest.ProcedureQuick:
	default:
		return manifest, fmt.Errorf("unsupported case manifest procedure %q", manifest.Procedure)
	}
	if strings.TrimSpace(manifest.CaseID) == "" || strings.TrimSpace(manifest.RunID) == "" || manifest.StartedAt.IsZero() {
		return manifest, fmt.Errorf("case manifest has incomplete identity or start time")
	}
	return manifest, nil
}

func (b *builder) catalogRecord(options Options) error {
	return b.walkSource(Source{ID: "record", Kind: "record", Path: b.layout.recordRoot, Available: true}, func(relative string) bool {
		relative = filepath.ToSlash(relative)
		if isPrivateWorkPath(relative) {
			return false
		}
		if isWorkNotePath(relative) {
			return options.IncludeWorkNotes
		}
		if isSessionOrLogPath(relative) || isEmbeddedSessionPath(relative) {
			return options.IncludeSessions
		}
		return true
	})
}

func (b *builder) catalogSessions() error {
	runPath := filepath.Join(b.layout.recordRoot, "run.json")
	var run struct {
		Management struct {
			Participants []struct {
				Role     string `json:"role"`
				StateDir string `json:"state_dir"`
			} `json:"participants"`
		} `json:"management"`
	}
	if err := readJSON(runPath, &run); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read session locations from run.json: %w", err)
	}
	seen := make(map[string]bool)
	for _, participant := range run.Management.Participants {
		path := strings.TrimSpace(participant.StateDir)
		if path == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve %s session directory: %w", participant.Role, err)
		}
		if seen[abs] {
			continue
		}
		seen[abs] = true
		source := Source{
			ID:       fmt.Sprintf("session-%d", len(b.record.Sources)),
			Kind:     "session",
			Path:     abs,
			PathBase: SourcePathBaseAbsolute,
			Role:     strings.TrimSpace(participant.Role),
		}
		info, statErr := os.Lstat(abs)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				source.Error = "not found"
				b.record.Sources = append(b.record.Sources, source)
				continue
			}
			return fmt.Errorf("inspect session directory %s: %w", abs, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			source.Error = "not a regular directory"
			b.record.Sources = append(b.record.Sources, source)
			continue
		}
		source.Available = true
		b.record.Sources = append(b.record.Sources, source)
		if err := b.walkSource(source, func(string) bool { return true }); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) walkSource(source Source, include func(string) bool) error {
	return filepath.WalkDir(source.Path, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk %s at %s: %w", source.Kind, path, walkErr)
		}
		if path == source.Path {
			return nil
		}
		relative, err := filepath.Rel(source.Path, path)
		if err != nil {
			return fmt.Errorf("resolve path below %s: %w", source.Path, err)
		}
		relative = filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if !include(relative + "/") {
				return filepath.SkipDir
			}
			return nil
		}
		if !include(relative) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect artifact %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		mediaType := mime.TypeByExtension(filepath.Ext(relative))
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		access := "case_record"
		if source.Kind == "session" || isSessionOrLogPath(relative) || isEmbeddedSessionPath(relative) {
			access = "sessions"
		} else if isWorkNotePath(relative) {
			access = "work_notes"
		}
		artifact := Artifact{
			SourceID:        source.ID,
			Path:            relative,
			MediaType:       mediaType,
			SizeBytes:       info.Size(),
			Category:        artifactCategory(relative, source.Kind),
			Access:          access,
			Timestamp:       info.ModTime().UTC(),
			TimestampSource: "file_modification",
		}
		b.artifacts[artifactKey(source.ID, relative)] = artifact
		return nil
	})
}

func (b *builder) finish() {
	b.record.Artifacts = make([]Artifact, 0, len(b.artifacts))
	for _, artifact := range b.artifacts {
		b.record.Artifacts = append(b.record.Artifacts, artifact)
	}
	sort.Slice(b.record.Artifacts, func(i, j int) bool {
		left, right := b.record.Artifacts[i], b.record.Artifacts[j]
		if !left.Timestamp.Equal(right.Timestamp) {
			return left.Timestamp.Before(right.Timestamp)
		}
		if left.SourceID != right.SourceID {
			return left.SourceID < right.SourceID
		}
		return left.Path < right.Path
	})
	sort.SliceStable(b.record.Docket, func(i, j int) bool {
		left, right := b.record.Docket[i], b.record.Docket[j]
		if !left.Timestamp.Equal(right.Timestamp) {
			return left.Timestamp.Before(right.Timestamp)
		}
		if left.order != right.order {
			return left.order < right.order
		}
		return left.ID < right.ID
	})
	for index := range b.record.Docket {
		b.record.Docket[index].Sequence = index + 1
	}
}

func (b *builder) addDocket(item DocketItem) {
	if item.Timestamp.IsZero() {
		item.Timestamp = b.layout.manifest.StartedAt.UTC()
		item.TimestampSource = "case_manifest"
	}
	if item.Access == "" {
		item.Access = "case_record"
	}
	key := item.ID
	if key == "" {
		key = fmt.Sprintf("item-%d", b.nextOrder+1)
		item.ID = key
	}
	if b.docketKeys[key] {
		return
	}
	b.docketKeys[key] = true
	b.nextOrder++
	item.order = b.nextOrder
	b.record.Docket = append(b.record.Docket, item)
}

func (b *builder) coreRelative(name string) string {
	if b.layout.corePrefix == "" {
		return filepath.ToSlash(name)
	}
	return filepath.ToSlash(filepath.Join(b.layout.corePrefix, name))
}

func (b *builder) coreReference(name string) Reference {
	return Reference{SourceID: "record", Path: b.coreRelative(name)}
}

func (b *builder) artifactReference(relative string) []Reference {
	relative = filepath.ToSlash(relative)
	if _, ok := b.artifacts[artifactKey("record", relative)]; !ok {
		return nil
	}
	return []Reference{{SourceID: "record", Path: relative}}
}

func (b *builder) artifactTime(relative string) (time.Time, string) {
	if artifact, ok := b.artifacts[artifactKey("record", filepath.ToSlash(relative))]; ok {
		return artifact.Timestamp, artifact.TimestampSource
	}
	return b.layout.manifest.StartedAt.UTC(), "case_manifest"
}

func readJSON(path string, value any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	decodeErr := json.NewDecoder(file).Decode(value)
	closeErr := file.Close()
	if decodeErr != nil {
		err := fmt.Errorf("decode %s: %w", path, decodeErr)
		if closeErr != nil {
			return errors.Join(err, fmt.Errorf("close %s: %w", path, closeErr))
		}
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", path, closeErr)
	}
	return nil
}

func readJSONLines(path string) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var records []map[string]any
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 16*1024*1024)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			decodeErr := fmt.Errorf("decode %s line %d: %w", path, len(records)+1, err)
			if closeErr := file.Close(); closeErr != nil {
				return nil, errors.Join(decodeErr, fmt.Errorf("close %s: %w", path, closeErr))
			}
			return nil, decodeErr
		}
		records = append(records, record)
	}
	scanErr := scanner.Err()
	closeErr := file.Close()
	if scanErr != nil {
		return nil, fmt.Errorf("read %s: %w", path, scanErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close %s: %w", path, closeErr)
	}
	return records, nil
}

func WriteJSON(writer io.Writer, record Record) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(record); err != nil {
		return fmt.Errorf("write case record JSON: %w", err)
	}
	return nil
}

func artifactKey(sourceID, path string) string {
	return sourceID + ":" + filepath.ToSlash(path)
}

func isWorkNotePath(path string) bool {
	base := filepath.Base(strings.TrimSuffix(filepath.ToSlash(path), "/"))
	return base == "work-notes.ndjson" || base == "work-notes.jsonl"
}

func isSessionOrLogPath(path string) bool {
	path = strings.TrimSuffix(filepath.ToSlash(path), "/")
	return path == "logs" || strings.HasPrefix(path, "logs/") || path == "session" || strings.HasPrefix(path, "session/")
}

func isEmbeddedSessionPath(path string) bool {
	path = strings.TrimSuffix(filepath.ToSlash(path), "/")
	return path == "agents" || strings.HasPrefix(path, "agents/")
}

func isPrivateWorkPath(path string) bool {
	path = strings.TrimSuffix(filepath.ToSlash(path), "/")
	base := filepath.Base(path)
	embeddedWork := strings.HasPrefix(path, "agents/") && (strings.Contains(path, "/work/") || strings.HasSuffix(path, "/work"))
	return path == "work" || strings.HasPrefix(path, "work/") || embeddedWork || path == "work-product" || strings.HasPrefix(path, "work-product/") || strings.Contains(path, "/work-product/") || strings.HasSuffix(path, "/work-product") || base == "plaintiff-strategy.md" || base == "defense-strategy.md"
}

func artifactCategory(path, sourceKind string) string {
	if sourceKind == "session" {
		return "participant_session"
	}
	path = filepath.ToSlash(path)
	base := filepath.Base(path)
	if isWorkNotePath(path) {
		return "work_notes"
	}
	if isSessionOrLogPath(path) {
		return "process_log"
	}
	if isEmbeddedSessionPath(path) {
		return "participant_session"
	}
	if strings.Contains(path, "/documents/") || strings.HasPrefix(path, "documents/") || strings.Contains(path, "/input-files/") || strings.HasPrefix(path, "input-files/") || strings.Contains(path, "/evidence-store/") || strings.HasPrefix(path, "evidence-store/") || strings.Contains(path, "/submitted-evidence/") || strings.HasPrefix(path, "submitted-evidence/") {
		return "evidence"
	}
	switch base {
	case "case-manifest.json":
		return "case_manifest"
	case "complaint.md":
		return "complaint"
	case "state.json":
		return "state"
	case "certificate.json":
		return "certificate"
	case "transcript.md", "transcript.json":
		return "transcript"
	case "digest.md":
		return "digest"
	case "events.ndjson":
		return "event_log"
	case "decision.json":
		return "decision"
	case "model-request.json":
		return "model_request"
	case "model-response.json":
		return "model_response"
	case "run.json", "local-run.json":
		return "run_record"
	case "council.json":
		return "council_record"
	case "evidence-manifest.json", "documents.json":
		return "evidence_manifest"
	case "input.json", "adjudicate-request.json", "normalized-case.json", "generated-scenario.json":
		return "case_input"
	case "runtime.json", "policy.json", "resolved-settings.json":
		return "configuration"
	default:
		return "unclassified"
	}
}
