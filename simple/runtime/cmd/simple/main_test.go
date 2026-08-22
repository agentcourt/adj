package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jsmorph/adj/simple/runtime/proceeding"
)

func TestDispatchHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	if err := dispatch(context.Background(), []string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("dispatch help: %v", err)
	}
	if !strings.Contains(stdout.String(), "Decide one proposition") {
		t.Fatalf("help output = %q", stdout.String())
	}
}

func TestRunCaseRejectsImplicitAPIKeyUse(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	err := dispatch(context.Background(), []string{"case", "--proposition", "P", "--out-dir", t.TempDir(), "--model", "openai://test", "--evidence-standard", "preponderance", "--max-documents", "1", "--max-document-bytes", "1", "--max-documents-bytes", "1"}, &stdout, &bytes.Buffer{})
	if err == nil || !isReportedError(err) {
		t.Fatalf("dispatch error = %v", err)
	}
	if !strings.Contains(stdout.String(), "--allow-api-key") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCaseMapsFlags(t *testing.T) {
	original := runProcedure
	defer func() { runProcedure = original }()
	var got proceeding.Options
	runProcedure = func(_ context.Context, opts proceeding.Options) (proceeding.Result, error) {
		got = opts
		return proceeding.Result{SchemaVersion: proceeding.RunSchemaVersion, Procedure: "simple", Status: "ok"}, nil
	}
	var stdout bytes.Buffer
	err := dispatch(context.Background(), []string{
		"case",
		"--proposition", "P",
		"--documents", "docs",
		"--out-dir", "out",
		"--case-id", "case",
		"--run-id", "run",
		"--model", "openai://test",
		"--reasoning-effort", "xhigh",
		"--max-output-tokens", "16384",
		"--max-tool-calls", "8",
		"--evidence-standard", "clear and convincing evidence",
		"--prompt-dir", "prompt-set",
		"--prompt-file", "tool.decision=tool.md",
		"--prompt-file", "decision=decision.md",
		"--prompt-file", "search.enabled=on.md",
		"--prompt-file", "search.disabled=off.md",
		"--allow-api-key",
		"--web-search=false",
		"--max-documents", "3",
		"--max-document-bytes", "100",
		"--max-documents-bytes", "200",
		"--timeout-seconds", "30",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("dispatch error = %v", err)
	}
	if got.Proposition != "P" || got.DocumentsRoot != "docs" || got.EvidenceStandard != "clear and convincing evidence" || !got.AllowAPIKey {
		t.Fatalf("options = %#v", got)
	}
	if got.ReasoningEffort != "xhigh" || got.MaxOutputTokens != 16384 || got.MaxToolCalls != 8 {
		t.Fatalf("model request options = %#v", got)
	}
	if got.PromptDir != "prompt-set" || got.PromptFiles["tool.decision"] != "tool.md" || got.PromptFiles["decision"] != "decision.md" || got.PromptFiles["search.enabled"] != "on.md" || got.PromptFiles["search.disabled"] != "off.md" {
		t.Fatalf("prompt paths = %#v", got)
	}
	if got.MaxDocuments != 3 || got.MaxDocumentBytes != 100 || got.MaxDocumentsBytes != 200 || got.TimeoutSeconds != 30 {
		t.Fatalf("limits = %#v", got)
	}
	if got.WebSearch == nil || *got.WebSearch {
		t.Fatalf("web search = %v, want false", got.WebSearch)
	}
	if !strings.Contains(stdout.String(), `"status":"ok"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
	got = proceeding.Options{}
	stdout.Reset()
	if err := dispatch(context.Background(), []string{"case"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("dispatch with defaults: %v", err)
	}
	if got.WebSearch == nil || !*got.WebSearch {
		t.Fatalf("default web search = %v, want true", got.WebSearch)
	}
	if got.ReasoningEffort != "" || got.MaxOutputTokens != 0 || got.MaxToolCalls != 0 {
		t.Fatalf("default model request options = %#v", got)
	}
}

func TestRunCaseReportsProcedureErrorResultOnce(t *testing.T) {
	original := runProcedure
	defer func() { runProcedure = original }()
	runProcedure = func(_ context.Context, _ proceeding.Options) (proceeding.Result, error) {
		return proceeding.Result{SchemaVersion: proceeding.RunSchemaVersion, Procedure: "simple", Status: "error", Error: "failed"}, errors.New("failed")
	}
	var stdout bytes.Buffer
	err := dispatch(context.Background(), []string{"case"}, &stdout, &bytes.Buffer{})
	if err == nil || !isReportedError(err) {
		t.Fatalf("dispatch error = %v", err)
	}
	if strings.Count(strings.TrimSpace(stdout.String()), "\n") != 0 {
		t.Fatalf("stdout contains multiple lines: %q", stdout.String())
	}
}
