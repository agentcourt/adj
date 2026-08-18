package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jsmorph/adj/common/documents"
	"github.com/jsmorph/adj/quick"
)

func TestDispatchHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := dispatch(context.Background(), []string{"help", "case"}, &stdout, &stderr); err != nil {
		t.Fatalf("dispatch help: %v", err)
	}
	if !strings.Contains(stderr.String(), "--evidence-standard") || !strings.Contains(stderr.String(), "--max-document-files") || !strings.Contains(stderr.String(), "-parallel-council") || !strings.Contains(stderr.String(), "--allow-api-key") {
		t.Fatalf("help output omits required flags: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCasePassesParallelCouncilOption(t *testing.T) {
	original := runProcedure
	defer func() { runProcedure = original }()
	var received quick.Options
	runProcedure = func(_ context.Context, opts quick.Options) (quick.Result, error) {
		received = opts
		return quick.Result{
			SchemaVersion: quick.ResultSchemaVersion,
			Procedure:     quick.Procedure,
			CaseID:        opts.CaseID,
			RunID:         opts.RunID,
			Status:        "ok",
			Phase:         "complete",
			Documents:     documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}},
			Council:       []quick.CouncilMember{},
			Arguments:     []quick.Argument{},
			Votes:         []quick.Vote{},
			Events:        []quick.Event{},
		}, nil
	}
	root := t.TempDir()
	var stdout bytes.Buffer
	err := runCase(context.Background(), []string{
		"--proposition", "p",
		"--out-dir", filepath.Join(root, "out"),
		"--council-pool", filepath.Join(root, "pool.jsonl"),
		"--council-size", "1",
		"--required-votes", "1",
		"--evidence-standard", "preponderance",
		"--case-id", "case",
		"--run-id", "run",
		"--max-document-files", "1",
		"--max-document-file-bytes", "1",
		"--max-documents-total-bytes", "1",
		"--parallel-council",
		"--allow-api-key",
	}, &stdout, io.Discard)
	if err != nil {
		t.Fatalf("runCase: %v", err)
	}
	decodeCommandResult(t, stdout.Bytes())
	if !received.ParallelCouncil {
		t.Fatal("parallel council option was false")
	}
}

func TestRunCaseRejectsMissingInputsBeforeProviderUse(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := runCase(context.Background(), []string{"--proposition", "p"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("runCase error = %v", err)
	}
	result := decodeCommandResult(t, stdout.Bytes())
	if result.SchemaVersion != quick.ResultSchemaVersion || result.Procedure != quick.Procedure || result.Status != "failed" || result.Phase != "failed" {
		t.Fatalf("result identity = %#v", result)
	}
	if result.CaseID != "quick-1" || !strings.HasPrefix(result.RunID, "quick-") || result.Proposition != "p" {
		t.Fatalf("result inputs = %#v", result)
	}
	if result.StartedAt.IsZero() || result.FinishedAt.IsZero() || result.Documents.SchemaVersion != documents.SchemaVersion {
		t.Fatalf("result record = %#v", result)
	}
	if result.Council == nil || result.Arguments == nil || result.Votes == nil || result.Events == nil {
		t.Fatalf("result collections are nil: %#v", result)
	}
}

func TestRunCaseReportsConfigurationErrorAsCompleteResult(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	err := runCase(context.Background(), []string{
		"--proposition", "p",
		"--out-dir", filepath.Join(root, "out"),
		"--council-pool", filepath.Join(root, "pool.jsonl"),
		"--council-size", "0",
		"--required-votes", "0",
		"--evidence-standard", "preponderance",
		"--case-id", "case",
		"--run-id", "run",
		"--max-document-files", "1",
		"--max-document-file-bytes", "1",
		"--max-documents-total-bytes", "1",
		"--allow-api-key",
	}, &stdout, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "council size") {
		t.Fatalf("runCase error = %v", err)
	}
	result := decodeCommandResult(t, stdout.Bytes())
	if result.SchemaVersion != quick.ResultSchemaVersion || result.CaseID != "case" || result.RunID != "run" || result.Status != "failed" || result.Phase != "failed" {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.Error, "council size") {
		t.Fatalf("result error = %q", result.Error)
	}
}

func TestRunCaseReportsFlagParseError(t *testing.T) {
	var stdout bytes.Buffer
	err := runCase(context.Background(), []string{"--unknown"}, &stdout, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "flag provided") {
		t.Fatalf("runCase error = %v", err)
	}
	result := decodeCommandResult(t, stdout.Bytes())
	if result.SchemaVersion != quick.ResultSchemaVersion || result.Status != "failed" || result.Error == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestWriteResultDetectsShortWrite(t *testing.T) {
	err := writeResult(shortWriter{}, quick.Result{SchemaVersion: quick.ResultSchemaVersion})
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("writeResult error = %v", err)
	}
}

func TestRunCaseRedactsMalformedModelQueryFromOutput(t *testing.T) {
	root := t.TempDir()
	secret := "command-secret"
	poolPath := filepath.Join(root, "pool.jsonl")
	pool := `{"endpoint":"openrouter","model":"model?token=` + secret + `#bad","persona":"persona.txt"}` + "\n"
	if err := os.WriteFile(poolPath, []byte(pool), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	err := runCase(context.Background(), []string{
		"--proposition", "p",
		"--out-dir", filepath.Join(root, "out"),
		"--council-pool", poolPath,
		"--council-size", "1",
		"--required-votes", "1",
		"--evidence-standard", "preponderance",
		"--case-id", "case",
		"--run-id", "run",
		"--max-document-files", "1",
		"--max-document-file-bytes", "1",
		"--max-documents-total-bytes", "1",
		"--allow-api-key",
	}, &stdout, io.Discard)
	if err == nil {
		t.Fatal("runCase succeeded")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(stdout.String(), secret) {
		t.Fatalf("credential leaked through command: error=%v stdout=%s", err, stdout.String())
	}
	result := decodeCommandResult(t, stdout.Bytes())
	if result.Status != "failed" || !strings.Contains(result.Error, "[redacted]") {
		t.Fatalf("result = %#v", result)
	}
}

func TestPrintUsageDetectsShortWrite(t *testing.T) {
	if err := printUsage(shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("printUsage error = %v", err)
	}
}

func decodeCommandResult(t *testing.T, raw []byte) quick.Result {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var result quick.Result
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, raw)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("extra command output: error=%v value=%s", err, extra)
	}
	return result
}

type shortWriter struct{}

func (shortWriter) Write(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, nil
	}
	return len(value) - 1, nil
}
