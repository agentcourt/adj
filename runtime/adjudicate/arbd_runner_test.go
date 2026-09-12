package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	localrun "github.com/agentcourt/adj/runtime/localrun/arbd"
	"github.com/agentcourt/adj/runtime/runstate"
)

type arbdAdapterObserver struct{}

func (*arbdAdapterObserver) StartProcess(runstate.ProcessStart) (runstate.FinishProcess, error) {
	return nil, nil
}

func TestARBDRunnerBuildsAARDCaseAndMapsTerminalResults(t *testing.T) {
	t.Setenv("ARBD_OPENROUTER_KEY", "council-secret")
	t.Setenv("ARBD_CLAUDE_KEY", "defendant-secret")
	t.Setenv("OPENROUTER_API_KEY", "unselected-council-secret")
	t.Setenv("OPENAI_API_KEY", "unselected-lawyer-secret")
	recordDir := t.TempDir()
	for _, directory := range []string{"core", "logs", filepath.Join("inputs", "documents")} {
		if err := os.MkdirAll(filepath.Join(recordDir, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	documentPath := filepath.Join(recordDir, "inputs", "documents", "fact.txt")
	if err := os.WriteFile(documentPath, []byte("fact"), 0o644); err != nil {
		t.Fatal(err)
	}

	observer := &arbdAdapterObserver{}
	var options localrun.Options
	runner := ARBDRunner{run: func(_ context.Context, provided localrun.Options) (localrun.Result, error) {
		options = provided
		return localrun.Result{
			CaseID:  "case-1",
			RunID:   "run-1",
			Status:  "ok",
			Phase:   "closed",
			Answers: map[string]int{"C2": 81, "C1": 72},
		}, nil
	}}
	request := ProcedureRequest{
		Request: Request{CaseID: "case-1", RunID: "run-1", Proposition: "How strong is the evidence?"},
		Settings: ResolvedSettings{
			Common: CommonSettings{
				EvidenceStandard:        "clear_and_convincing",
				CouncilPool:             "/council.jsonl",
				CouncilAllowedEndpoints: []string{"openrouter"},
				CouncilMinEndpoints:     1,
				CouncilSize:             2,
				RequiredVotes:           2,
				ProviderCredentials: map[string]CredentialMetadata{
					"openrouter": {Source: AuthAPIKey, EnvironmentVariable: "ARBD_OPENROUTER_KEY"},
				},
			},
			AgentProfiles: map[string]ResolvedAgentProfile{
				"plaintiff": {
					Runner: RunnerPi, Provider: ProviderOpenAI,
					Model: "openai/gpt-5.6-sol", ReasoningEffort: "xhigh", Resume: true,
					Authentication: Authentication{
						Source: AuthSubscription,
						Path:   "/credentials/codex.json",
					},
				},
				"defendant": {
					Runner: RunnerClaude, Provider: ProviderAnthropic,
					Model: "claude-opus-5", ReasoningEffort: "high", Resume: true,
					Authentication: Authentication{
						Source:              AuthAPIKey,
						EnvironmentVariable: "ARBD_CLAUDE_KEY",
					},
				},
			},
			Procedure: ResolvedProcedureSettings{ARBD: &ARBDSettings{
				CoreCommand:         "aard",
				CoreWorkingDir:      "/core-work",
				MCPCommand:          "aard-mcp-test",
				MCPWorkingDir:       "/mcp-work",
				PlaintiffProfile:    "plaintiff",
				DefendantProfile:    "defendant",
				JudgmentStandard:    "score from 0 through 100",
				LauncherPromptDir:   "/launcher/arbd",
				LauncherPromptFiles: PromptFilePaths{"skill.openclaw": "/launcher/skill.md"},
				WebSearch:           defaultEnabled(nil),
				Timeout:             Duration(30 * time.Second),
			}},
		},
		Documents: DocumentManifest{
			SchemaVersion: DocumentManifestSchemaVersion,
			Documents:     []Document{{Path: "fact.txt", Bytes: 4}},
			TotalBytes:    4,
		},
		RecordDir:  recordDir,
		CoreDir:    filepath.Join(recordDir, "core"),
		LogsDir:    filepath.Join(recordDir, "logs"),
		SessionDir: filepath.Join(recordDir, "session"),
		Observer:   observer,
	}
	outcome, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != StatusOK || outcome.Phase != "closed" || outcome.Decision == nil {
		t.Fatalf("outcome = %#v", outcome)
	}
	if outcome.Decision.Kind != "council_answers" || outcome.Decision.Value != `{"C1":72,"C2":81}` {
		t.Fatalf("decision = %#v", outcome.Decision)
	}
	var native localrun.Result
	if err := json.Unmarshal(outcome.ProcedureResult, &native); err != nil {
		t.Fatalf("decode native procedure result: %v", err)
	}
	if native.CaseID != "case-1" || native.RunID != "run-1" || native.Answers["C2"] != 81 {
		t.Fatalf("native procedure result = %#v", native)
	}
	complaint, err := os.ReadFile(filepath.Join(recordDir, "inputs", "arbd-complaint.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(complaint) != "# Question\n\nHow strong is the evidence?\n" {
		t.Fatalf("complaint = %q", complaint)
	}
	if !reflect.DeepEqual(options.CaseFiles, []string{documentPath}) {
		t.Fatalf("case files = %#v", options.CaseFiles)
	}
	if options.CoreCommand != "aard" || options.CoreWorkingDir != "/core-work" || options.OutputDir != recordDir || options.CoreOutputDir != request.CoreDir || options.LogsDir != request.LogsDir {
		t.Fatalf("core and output options = %#v", options)
	}
	if options.MCPCommand != "aard-mcp-test" || options.MCPWorkingDir != "/mcp-work" {
		t.Fatalf("MCP command options = %#v", options)
	}
	if options.CouncilPoolPath != "/council.jsonl" || options.CouncilSize != 2 || options.JudgmentStandard != "score from 0 through 100" {
		t.Fatalf("procedure options = %#v", options)
	}
	if !reflect.DeepEqual(options.CouncilAllowedEndpoints, []string{"openrouter"}) || options.CouncilMinEndpoints != 1 {
		t.Fatalf("council endpoint options = %#v", options)
	}
	if options.LauncherPromptDir != "/launcher/arbd" || options.LauncherPromptFiles["skill.openclaw"] != "/launcher/skill.md" {
		t.Fatalf("launcher prompt options = %#v", options)
	}
	if options.CouncilTimeoutSeconds != 30 || options.LawyerTimeoutSeconds != 30 {
		t.Fatalf("timeouts = %#v", options)
	}
	if options.PlaintiffLawyer.Name != "plaintiff" || options.PlaintiffLawyer.Runner != localrun.LawyerPi || options.PlaintiffLawyer.Model != "openai/gpt-5.6-sol" || options.PlaintiffLawyer.ReasoningEffort != "xhigh" || options.PlaintiffLawyer.AuthMode != "subscription" || options.PlaintiffLawyer.CredentialsFile != "/credentials/codex.json" {
		t.Fatalf("plaintiff profile = %#v", options.PlaintiffLawyer)
	}
	if options.DefendantLawyer.Name != "defendant" || options.DefendantLawyer.Runner != localrun.LawyerClaude || options.DefendantLawyer.Model != "claude-opus-5" || options.DefendantLawyer.ReasoningEffort != "high" || options.DefendantLawyer.APIKeyEnv != "ARBD_CLAUDE_KEY" {
		t.Fatalf("defendant profile = %#v", options.DefendantLawyer)
	}
	if options.LawyerWebSearch == nil || !*options.LawyerWebSearch {
		t.Fatalf("lawyer web search = %#v", options.LawyerWebSearch)
	}
	if options.PlaintiffLawyer.StateDir != filepath.Join(request.SessionDir, "plaintiff", "plaintiff") || options.PlaintiffLawyer.WorkDir != filepath.Join(recordDir, "work", "plaintiff") || options.DefendantLawyer.StateDir != filepath.Join(request.SessionDir, "defendant", "defendant") || options.DefendantLawyer.WorkDir != filepath.Join(recordDir, "work", "defendant") {
		t.Fatalf("lawyer state and work directories = %#v %#v", options.PlaintiffLawyer, options.DefendantLawyer)
	}
	if options.ProcessObserver != observer {
		t.Fatal("process observer was not forwarded")
	}
	if value, ok := environmentValue(options.CoreEnvironment, "OPENROUTER_API_KEY"); !ok || value != "council-secret" {
		t.Fatalf("core OpenRouter credential = %q, %t", value, ok)
	}
	for _, name := range []string{"ARBD_OPENROUTER_KEY", "ARBD_CLAUDE_KEY", "OPENAI_API_KEY"} {
		if _, ok := environmentValue(options.CoreEnvironment, name); ok {
			t.Fatalf("core environment contains credential %s", name)
		}
		if _, ok := environmentValue(options.MCPEnvironment, name); ok {
			t.Fatalf("MCP environment contains credential %s", name)
		}
	}
	for _, name := range []string{"OPENROUTER_API_KEY", "ARBD_OPENROUTER_KEY", "ARBD_CLAUDE_KEY", "OPENAI_API_KEY"} {
		if _, ok := environmentValue(options.ParticipantEnvironment, name); ok {
			t.Fatalf("participant environment contains credential %s", name)
		}
	}
	if value, ok := environmentValue(options.DefendantLawyer.Environment, "ARBD_CLAUDE_KEY"); !ok || value != "defendant-secret" {
		t.Fatalf("defendant credential = %q, %t", value, ok)
	}

	failure, err := mapARBDResult("case-1", "run-1", localrun.Result{
		CaseID: "case-1", RunID: "run-1", Status: "failed", Phase: "failed",
		Failure: map[string]any{"reason": "deadline_expired"},
	}, nil)
	if err != nil || failure.Status != StatusFailed || !strings.Contains(string(failure.Failure), "deadline_expired") {
		t.Fatalf("procedural failure = %#v, %v", failure, err)
	}
	runErr := errors.New("cleanup failed")
	_, err = mapARBDResult("case-1", "run-1", localrun.Result{
		CaseID: "case-1", RunID: "run-1", Status: "ok", Phase: "closed", Answers: map[string]int{"C1": 72},
	}, runErr)
	if !errors.Is(err, runErr) {
		t.Fatalf("cleanup error = %v", err)
	}
	_, err = mapARBDResult("case-1", "run-1", localrun.Result{
		CaseID: "case-1", RunID: "run-1", Status: "ok", Phase: "closed", Answers: map[string]int{"C1": 101},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "outside 0 through 100") {
		t.Fatalf("invalid answer error = %v", err)
	}
}
