package quick

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/agentcourt/adj/common/recordio"
)

const (
	inputSchema      = "adj.quick.input.v2"
	runtimeSchema    = "adj.quick.runtime.v1"
	transcriptSchema = "adj.quick.transcript.v2"
	resultSchema     = ResultSchemaVersion
)

type records struct {
	dir string
	mu  sync.Mutex
}

func (r *records) writeInput(cfg Config) error {
	return recordio.WriteJSON(filepath.Join(r.dir, "input.json"), inputRecord{
		SchemaVersion:           inputSchema,
		Procedure:               Procedure,
		Proposition:             cfg.Proposition,
		CaseID:                  cfg.CaseID,
		RunID:                   cfg.RunID,
		DocumentsSource:         cfg.DocumentsDir,
		CouncilPoolPath:         cfg.CouncilPoolPath,
		CouncilAllowedEndpoints: append([]string(nil), cfg.CouncilAllowedEndpoints...),
		CouncilMinEndpoints:     cfg.CouncilMinEndpoints,
		CouncilSize:             cfg.CouncilSize,
		RequiredVotes:           cfg.RequiredVotes,
		EvidenceStandard:        cfg.EvidenceStandard,
		LawyerWebSearchEnabled:  cfg.LawyerWebSearchEnabled,
		CaseAPIAddr:             cfg.CaseAPIAddr,
		LawyerTimeout:           cfg.LawyerTimeout.String(),
		CouncilTimeout:          cfg.CouncilTimeout.String(),
		MaxResponseBytes:        cfg.MaxResponseBytes,
		MaxArgumentChars:        cfg.MaxArgumentChars,
		InvalidAttemptLimit:     cfg.InvalidAttemptLimit,
		DocumentLimits:          cfg.DocumentLimits,
		CouncilRequestAttempts:  cfg.CouncilRequestAttempts,
		ParallelCouncil:         cfg.ParallelCouncil,
		DirectAPIKeyAuthorized:  cfg.AllowAPIKey,
	})
}

func (r *records) writeRuntime(runtime runtimeRecord) error {
	return recordio.WriteJSONAtomic(filepath.Join(r.dir, "runtime.json"), runtime)
}

func (r *records) writeTranscript(transcript Transcript) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return recordio.WriteJSONAtomic(filepath.Join(r.dir, "transcript.json"), transcript)
}

func (r *records) appendEvent(event Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return recordio.AppendJSONLine(filepath.Join(r.dir, "events.ndjson"), event)
}

func (r *records) appendWorkNote(value map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return recordio.AppendJSONLine(filepath.Join(r.dir, "work-notes.jsonl"), value)
}

func (r *records) writeRun(result Result) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return recordio.WriteJSONAtomic(filepath.Join(r.dir, "run.json"), result)
}

func createEmptyEvents(path string) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", path, closeErr))
		}
	}()
	return nil
}
