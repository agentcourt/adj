package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	service "github.com/agentcourt/adj/runtime/adjudicate"
)

type fakeEngine struct {
	calls   int
	request service.Request
	result  service.Result
	err     error
	ctxErr  error
}

func (e *fakeEngine) Run(ctx context.Context, request service.Request) (service.Result, error) {
	e.calls++
	e.request = request
	e.ctxErr = ctx.Err()
	return e.result, e.err
}

func TestRunBuildsRequestAndWritesOneResult(t *testing.T) {
	t.Parallel()

	randomBytes := make([]byte, randomIDBytes*2)
	for index := range randomBytes {
		randomBytes[index] = byte(index)
	}
	engine := &fakeEngine{result: testResult("", "")}
	dependencies := testDependencies(engine, bytes.NewReader(randomBytes))
	var stdout, stderr bytes.Buffer
	err := runWithDependencies(context.Background(), []string{
		"--proc", "simple",
		"--proposition", "The sky is blue.",
		"--documents", "./evidence",
		"--settings", "./settings.json",
		"--out-dir", "./out/case",
	}, &stdout, &stderr, dependencies)
	if err != nil {
		t.Fatalf("runWithDependencies error = %v", err)
	}
	if engine.calls != 1 {
		t.Fatalf("engine calls = %d, want 1", engine.calls)
	}
	wantCaseID := "case-000102030405060708090a0b0c0d0e0f"
	wantRunID := "run-101112131415161718191a1b1c1d1e1f"
	if engine.request.SchemaVersion != service.RequestSchemaVersion || engine.request.Procedure != service.ProcedureSimple {
		t.Fatalf("request identity = %#v", engine.request)
	}
	if engine.request.CaseID != wantCaseID || engine.request.RunID != wantRunID {
		t.Fatalf("generated IDs = %q, %q", engine.request.CaseID, engine.request.RunID)
	}
	if engine.request.Proposition != "The sky is blue." || engine.request.Documents.Root != "./evidence" {
		t.Fatalf("matter = %#v", engine.request)
	}
	if engine.request.SettingsFile != "./settings.json" || engine.request.OutDir != "./out/case" {
		t.Fatalf("paths = %#v", engine.request)
	}
	decoded := decodeSingleResult(t, stdout.Bytes())
	if decoded.Status != service.StatusOK {
		t.Fatalf("stdout result = %#v", decoded)
	}
	if strings.Contains(stdout.String(), "settings.json") || strings.Contains(stdout.String(), "The sky is blue.") {
		t.Fatalf("stdout contains request input: %s", stdout.String())
	}
}

func TestRunPreservesExplicitIDsWithoutReadingRandomness(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{result: testResult("case-explicit", "run-explicit")}
	dependencies := testDependencies(engine, errorReader{err: errors.New("random read")})
	var stdout bytes.Buffer
	err := runWithDependencies(context.Background(), []string{
		"--proc", "simple",
		"--proposition", "P",
		"--settings", "settings.json",
		"--case-id", "case-explicit",
		"--run-id", "run-explicit",
	}, &stdout, io.Discard, dependencies)
	if err != nil {
		t.Fatalf("runWithDependencies error = %v", err)
	}
	if engine.request.CaseID != "case-explicit" || engine.request.RunID != "run-explicit" {
		t.Fatalf("request IDs = %q, %q", engine.request.CaseID, engine.request.RunID)
	}
}

func TestRunWritesErrorResultAndReturnsEngineError(t *testing.T) {
	t.Parallel()

	runErr := errors.New("engine failed")
	engine := &fakeEngine{
		result: service.Result{
			SchemaVersion: service.ResultSchemaVersion,
			Procedure:     service.ProcedureSimple,
			CaseID:        "case-1",
			RunID:         "run-1",
			Status:        service.StatusError,
			Phase:         "error",
			ErrorClass:    "procedure_run",
			Error:         "provider failed",
		},
		err: runErr,
	}
	var stdout bytes.Buffer
	err := runWithDependencies(context.Background(), requiredArgs(), &stdout, io.Discard, testDependencies(engine, bytes.NewReader(make([]byte, randomIDBytes*2))))
	if !errors.Is(err, runErr) {
		t.Fatalf("error = %v, want engine error", err)
	}
	result := decodeSingleResult(t, stdout.Bytes())
	if result.ErrorClass != "procedure_run" || result.Error != "provider failed" {
		t.Fatalf("stdout result = %#v", result)
	}
}

func TestRunSynthesizesErrorResultWhenEngineReturnsNoResult(t *testing.T) {
	t.Parallel()

	runErr := errors.New("settings invalid")
	engine := &fakeEngine{err: runErr}
	var stdout bytes.Buffer
	err := runWithDependencies(context.Background(), requiredArgs(), &stdout, io.Discard, testDependencies(engine, bytes.NewReader(make([]byte, randomIDBytes*2))))
	if !errors.Is(err, runErr) {
		t.Fatalf("error = %v, want engine error", err)
	}
	result := decodeSingleResult(t, stdout.Bytes())
	if result.Status != service.StatusError || result.ErrorClass != "engine_result" || !strings.Contains(result.Error, runErr.Error()) {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunWritesErrorResultWhenEngineCreationFails(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("registry failed")
	dependencies := commandDependencies{
		newEngine: func() (adjudicationEngine, error) { return nil, wantErr },
		random:    bytes.NewReader(make([]byte, randomIDBytes*2)),
	}
	var stdout bytes.Buffer
	err := runWithDependencies(context.Background(), requiredArgs(), &stdout, io.Discard, dependencies)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v", err)
	}
	result := decodeSingleResult(t, stdout.Bytes())
	if result.Status != service.StatusError || result.ErrorClass != "engine_create" || !strings.Contains(result.Error, wantErr.Error()) {
		t.Fatalf("result = %#v", result)
	}
}

func TestRunReturnsEngineAndOutputErrors(t *testing.T) {
	t.Parallel()

	runErr := errors.New("engine failed")
	writeErr := errors.New("write failed")
	engine := &fakeEngine{result: testResult("case-1", "run-1"), err: runErr}
	err := runWithDependencies(context.Background(), requiredArgs(), errorWriter{err: writeErr}, io.Discard, testDependencies(engine, bytes.NewReader(make([]byte, randomIDBytes*2))))
	if !errors.Is(err, runErr) || !errors.Is(err, writeErr) {
		t.Fatalf("error = %v, want engine and write errors", err)
	}
}

func TestRunReturnsEncodingErrorWithEngineError(t *testing.T) {
	t.Parallel()

	runErr := errors.New("engine failed")
	engine := &fakeEngine{
		result: service.Result{
			SchemaVersion:   service.ResultSchemaVersion,
			ProcedureResult: json.RawMessage("{"),
		},
		err: runErr,
	}
	err := runWithDependencies(context.Background(), requiredArgs(), io.Discard, io.Discard, testDependencies(engine, bytes.NewReader(make([]byte, randomIDBytes*2))))
	if !errors.Is(err, runErr) || !strings.Contains(err.Error(), "marshal adjudication result") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunRejectsArgumentsAndMissingFlags(t *testing.T) {
	t.Parallel()

	engine := &fakeEngine{}
	dependencies := testDependencies(engine, bytes.NewReader(make([]byte, randomIDBytes*2)))
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "positional", args: append(requiredArgs(), "extra"), want: "no positional"},
		{name: "procedure", args: []string{"--proposition", "P", "--settings", "s.json"}, want: "--proc is required"},
		{name: "proposition", args: []string{"--proc", "simple", "--settings", "s.json"}, want: "--proposition is required"},
		{name: "settings", args: []string{"--proc", "simple", "--proposition", "P"}, want: "--settings is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			err := runWithDependencies(context.Background(), test.args, &stdout, io.Discard, dependencies)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
	if engine.calls != 0 {
		t.Fatalf("engine calls = %d, want 0", engine.calls)
	}
}

func TestRunHelpUsesCommandOutputWriter(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	if err := runWithDependencies(context.Background(), []string{"-h"}, io.Discard, &stderr, commandDependencies{}); err != nil {
		t.Fatalf("help error = %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage: adjudicate") {
		t.Fatalf("help output = %q", stderr.String())
	}
	helpErr := errors.New("help write failed")
	err := runWithDependencies(context.Background(), []string{"-h"}, io.Discard, errorWriter{err: helpErr}, commandDependencies{})
	if !errors.Is(err, helpErr) {
		t.Fatalf("help error = %v", err)
	}
}

func TestRunReturnsRandomReadFailure(t *testing.T) {
	t.Parallel()

	randomErr := errors.New("random failed")
	engine := &fakeEngine{}
	err := runWithDependencies(context.Background(), requiredArgs(), io.Discard, io.Discard, testDependencies(engine, errorReader{err: randomErr}))
	if !errors.Is(err, randomErr) || !strings.Contains(err.Error(), "generate case ID") {
		t.Fatalf("error = %v", err)
	}
	if engine.calls != 0 {
		t.Fatalf("engine calls = %d, want 0", engine.calls)
	}
}

func TestRunPassesContextToEngine(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := &fakeEngine{result: testResult("case-1", "run-1")}
	if err := runWithDependencies(ctx, requiredArgs(), io.Discard, io.Discard, testDependencies(engine, bytes.NewReader(make([]byte, randomIDBytes*2)))); err != nil {
		t.Fatalf("runWithDependencies error = %v", err)
	}
	if !errors.Is(engine.ctxErr, context.Canceled) {
		t.Fatalf("engine context error = %v", engine.ctxErr)
	}
}

func TestDefaultRegistryContainsAvailableProcedures(t *testing.T) {
	t.Parallel()

	registry, err := defaultRegistry()
	if err != nil {
		t.Fatalf("defaultRegistry error = %v", err)
	}
	arb, ok := registry.Lookup(service.ProcedureARB)
	if !ok {
		t.Fatal("arb procedure is not registered")
	}
	if _, ok := arb.Runner.(service.ARBRunner); !ok {
		t.Fatalf("arb runner type = %T", arb.Runner)
	}
	if !arb.Capabilities.Documents || !arb.Capabilities.ParticipantAPI || !arb.Capabilities.LeanReplay || !arb.Capabilities.Sessions || !arb.Capabilities.Council {
		t.Fatalf("arb capabilities = %#v", arb.Capabilities)
	}
	arbd, ok := registry.Lookup(service.ProcedureARBD)
	if !ok {
		t.Fatal("arbd procedure is not registered")
	}
	if _, ok := arbd.Runner.(service.ARBDRunner); !ok {
		t.Fatalf("arbd runner type = %T", arbd.Runner)
	}
	if !arbd.Capabilities.Documents || !arbd.Capabilities.ParticipantAPI || !arbd.Capabilities.LeanReplay || !arbd.Capabilities.Sessions || !arbd.Capabilities.Council {
		t.Fatalf("arbd capabilities = %#v", arbd.Capabilities)
	}
	adc, ok := registry.Lookup(service.ProcedureADC)
	if !ok {
		t.Fatal("adc procedure is not registered")
	}
	if _, ok := adc.Runner.(service.ADCRunner); !ok {
		t.Fatalf("adc runner type = %T", adc.Runner)
	}
	if !adc.Capabilities.Documents || !adc.Capabilities.ParticipantAPI || !adc.Capabilities.LeanReplay || !adc.Capabilities.Sessions || !adc.Capabilities.Council {
		t.Fatalf("adc capabilities = %#v", adc.Capabilities)
	}
	registration, ok := registry.Lookup(service.ProcedureSimple)
	if !ok {
		t.Fatal("simple procedure is not registered")
	}
	if _, ok := registration.Runner.(service.SimpleRunner); !ok {
		t.Fatalf("simple runner type = %T", registration.Runner)
	}
	if !registration.Capabilities.Documents || registration.Capabilities.ParticipantAPI || registration.Capabilities.LeanReplay || registration.Capabilities.Sessions || registration.Capabilities.Council {
		t.Fatalf("simple capabilities = %#v", registration.Capabilities)
	}
	quick, ok := registry.Lookup(service.ProcedureQuick)
	if !ok {
		t.Fatal("quick procedure is not registered")
	}
	if _, ok := quick.Runner.(service.QuickRunner); !ok {
		t.Fatalf("quick runner type = %T", quick.Runner)
	}
	if !quick.Capabilities.Documents || !quick.Capabilities.ParticipantAPI || quick.Capabilities.LeanReplay || !quick.Capabilities.Sessions || !quick.Capabilities.Council {
		t.Fatalf("quick capabilities = %#v", quick.Capabilities)
	}
}

func decodeSingleResult(t *testing.T, raw []byte) service.Result {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var result service.Result
	if err := decoder.Decode(&result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, raw)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("stdout has more than one JSON value: error = %v, extra = %s", err, extra)
	}
	return result
}

func testDependencies(engine adjudicationEngine, random io.Reader) commandDependencies {
	return commandDependencies{
		newEngine: func() (adjudicationEngine, error) { return engine, nil },
		random:    random,
	}
}

func requiredArgs() []string {
	return []string{"--proc", "simple", "--proposition", "P", "--settings", "settings.json"}
}

func testResult(caseID, runID string) service.Result {
	finished := time.Date(2026, 8, 18, 12, 0, 1, 0, time.UTC)
	return service.Result{
		SchemaVersion: service.ResultSchemaVersion,
		Procedure:     service.ProcedureSimple,
		CaseID:        caseID,
		RunID:         runID,
		StartedAt:     time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		FinishedAt:    &finished,
		Status:        service.StatusOK,
		Phase:         "closed",
		Decision:      &service.Decision{Kind: "binary", Value: "demonstrated", Rationale: "Supported."},
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

type errorWriter struct {
	err error
}

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }
