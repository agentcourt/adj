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

	localrun "github.com/agentcourt/adj/runtime/localrun/arb"
)

func TestARBRunnerBuildsCanonicalCaseAndMapsResult(t *testing.T) {
	t.Setenv("ARB_OPENROUTER_KEY", "selected-secret")
	t.Setenv("OPENROUTER_API_KEY", "unselected-secret")
	t.Setenv("OPENAI_API_KEY", "unselected-openai")
	t.Setenv("ARB_LAWYER_KEY", "lawyer-secret")
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

	var options localrun.Options
	runner := ARBRunner{run: func(_ context.Context, provided localrun.Options) (localrun.Result, error) {
		options = provided
		return localrun.Result{
			CaseID:     "case-1",
			RunID:      "run-1",
			Status:     "ok",
			Phase:      "closed",
			Resolution: "demonstrated",
			Provider:   ProviderManagement{RequestCount: 4},
		}, nil
	}}
	request := arbProcedureRequest(recordDir)
	request.Documents = DocumentManifest{
		SchemaVersion: DocumentManifestSchemaVersion,
		Documents:     []Document{{Path: "fact.txt", Bytes: 4}},
		TotalBytes:    4,
	}
	outcome, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != StatusOK || outcome.Phase != "closed" || outcome.Decision == nil || outcome.Decision.Value != "demonstrated" {
		t.Fatalf("outcome = %#v", outcome)
	}
	if outcome.Provider.RequestCount != 4 {
		t.Fatalf("provider accounting = %#v", outcome.Provider)
	}
	if !json.Valid(outcome.ProcedureResult) {
		t.Fatalf("procedure result = %q", outcome.ProcedureResult)
	}
	complaintPath := filepath.Join(recordDir, "inputs", "arb", "complaint.md")
	complaint, err := os.ReadFile(complaintPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(complaint) != "# Proposition\n\nThe proposition\n" {
		t.Fatalf("complaint = %q", complaint)
	}
	entries, err := os.ReadDir(filepath.Dir(complaintPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "complaint.md" {
		t.Fatalf("complaint directory entries = %#v", entries)
	}
	if !reflect.DeepEqual(options.CaseFiles, []string{documentPath}) {
		t.Fatalf("case files = %#v", options.CaseFiles)
	}
	if options.OutputDir != recordDir || options.CoreOutputDir != filepath.Join(recordDir, "core") || options.LogsDir != filepath.Join(recordDir, "logs") {
		t.Fatalf("output paths = run %q core %q logs %q", options.OutputDir, options.CoreOutputDir, options.LogsDir)
	}
	if options.MCPCommand != "aar-mcp-test" || options.MCPWorkingDir != "/mcp-work" {
		t.Fatalf("MCP command options = %#v", options)
	}
	for _, name := range []string{"ARB_OPENROUTER_KEY", "ARB_LAWYER_KEY", "OPENROUTER_API_KEY", "OPENAI_API_KEY"} {
		if _, ok := environmentValue(options.MCPEnvironment, name); ok {
			t.Fatalf("MCP environment contains credential %s", name)
		}
	}
	if options.CouncilPoolPath != "/council.jsonl" || options.CouncilSize != 3 || options.RequiredVotes != 2 || options.EvidenceStandard != "preponderance_of_the_evidence" {
		t.Fatalf("common procedure options = %#v", options)
	}
	if options.LauncherPromptDir != "/launcher/arb" || options.LauncherPromptFiles["participant.pi"] != "/launcher/pi.md" {
		t.Fatalf("launcher prompt options = %#v", options)
	}
	if value, ok := environmentValue(options.CoreEnvironment, "OPENROUTER_API_KEY"); !ok || value != "selected-secret" {
		t.Fatalf("selected OpenRouter credential is absent")
	}
	for _, name := range []string{"ARB_OPENROUTER_KEY", "ARB_LAWYER_KEY"} {
		if _, ok := environmentValue(options.CoreEnvironment, name); ok {
			t.Fatalf("core environment contains credential source %s", name)
		}
	}
	if _, ok := environmentValue(options.CoreEnvironment, "OPENAI_API_KEY"); ok {
		t.Fatal("unselected OpenAI credential remains in the child environment")
	}
	for _, name := range []string{"ARB_OPENROUTER_KEY", "ARB_LAWYER_KEY", "OPENROUTER_API_KEY", "OPENAI_API_KEY"} {
		if _, ok := environmentValue(options.ParticipantEnvironment, name); ok {
			t.Fatalf("participant environment contains provider credential %s", name)
		}
	}
	if value, ok := environmentValue(options.PlaintiffLawyer.Environment, "ARB_LAWYER_KEY"); !ok || value != "lawyer-secret" {
		t.Fatal("plaintiff environment lacks its explicit credential")
	}
	if _, ok := environmentValue(options.DefendantLawyer.Environment, "ARB_LAWYER_KEY"); ok {
		t.Fatal("defendant environment contains plaintiff credential")
	}
	if options.PlaintiffLawyer.Runner != localrun.LawyerCodex || options.DefendantLawyer.Runner != localrun.LawyerClaude {
		t.Fatalf("lawyer profiles = %#v %#v", options.PlaintiffLawyer, options.DefendantLawyer)
	}
	if options.PlaintiffLawyer.AuthMode != "api_key" || options.PlaintiffLawyer.APIKeyEnv != "ARB_LAWYER_KEY" {
		t.Fatalf("plaintiff authentication = %#v", options.PlaintiffLawyer)
	}
	if options.PlaintiffLawyer.Provider != "openai" || options.PlaintiffLawyer.ReasoningEffort != "xhigh" || options.DefendantLawyer.Provider != "anthropic" || options.DefendantLawyer.ReasoningEffort != "high" {
		t.Fatalf("lawyer provider and reasoning settings = %#v %#v", options.PlaintiffLawyer, options.DefendantLawyer)
	}
	if options.LawyerWebSearch == nil || !*options.LawyerWebSearch {
		t.Fatalf("lawyer web search = %#v", options.LawyerWebSearch)
	}
	if options.PlaintiffLawyer.StateDir != filepath.Join(request.SessionDir, "plaintiff", "plaintiff") || options.PlaintiffLawyer.WorkDir != filepath.Join(recordDir, "work", "plaintiff") {
		t.Fatalf("plaintiff state and work directories = %#v", options.PlaintiffLawyer)
	}
	if options.DefendantLawyer.StateDir != filepath.Join(request.SessionDir, "defendant", "defendant") || options.DefendantLawyer.WorkDir != filepath.Join(recordDir, "work", "defendant") {
		t.Fatalf("defendant state and work directories = %#v", options.DefendantLawyer)
	}
}

func TestMapARBResultPreservesFailureAndProviderError(t *testing.T) {
	failure := map[string]any{"reason": "deadline_expired"}
	outcome, err := mapARBResult(localrun.Result{Status: "failed", Phase: "failed", Failure: failure, Provider: ProviderManagement{RequestCount: 2}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != StatusFailed || !json.Valid(outcome.Failure) || !strings.Contains(string(outcome.Failure), "deadline_expired") {
		t.Fatalf("outcome = %#v", outcome)
	}
	if outcome.Provider.RequestCount != 2 {
		t.Fatalf("provider accounting = %#v", outcome.Provider)
	}

	processErr := errors.New("exit status 1")
	_, err = mapARBResult(localrun.Result{Status: "failed", Phase: "failed", Error: "provider rejected credentials", ErrorClass: "provider_authentication", Failure: failure}, processErr)
	if !errors.Is(err, processErr) || runnerErrorClass(err) != "provider_authentication" {
		t.Fatalf("error = %v", err)
	}

	_, err = mapARBResult(localrun.Result{Status: "ok", Phase: "closed", Resolution: "no_majority", ErrorClass: "provider_transient"}, processErr)
	if !errors.Is(err, processErr) || runnerErrorClass(err) != "procedure_run" {
		t.Fatalf("successful-case cleanup error = %v", err)
	}
}

func TestARBCaseFilePathsRejectsEscape(t *testing.T) {
	_, err := formalCaseFilePaths(t.TempDir(), DocumentManifest{Documents: []Document{{Path: "../secret"}}})
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("error = %v", err)
	}
}

func arbProcedureRequest(recordDir string) ProcedureRequest {
	return ProcedureRequest{
		Request: Request{CaseID: "case-1", RunID: "run-1", Proposition: "The proposition"},
		Settings: ResolvedSettings{
			Common: CommonSettings{
				EvidenceStandard: "preponderance_of_the_evidence",
				CouncilPool:      "/council.jsonl",
				CouncilSize:      3,
				RequiredVotes:    2,
				ProviderCredentials: map[string]CredentialMetadata{
					"openrouter": {Source: AuthAPIKey, EnvironmentVariable: "ARB_OPENROUTER_KEY"},
				},
			},
			AgentProfiles: map[string]ResolvedAgentProfile{
				"plaintiff": {
					Runner: RunnerCodex, Provider: ProviderOpenAI, ReasoningEffort: "xhigh",
					Resume:         true,
					Authentication: Authentication{Source: AuthAPIKey, EnvironmentVariable: "ARB_LAWYER_KEY"},
				},
				"defendant": {
					Runner: RunnerClaude, Provider: ProviderAnthropic, ReasoningEffort: "high",
					Resume:         true,
					Authentication: Authentication{Source: AuthSubscription, Path: "/credentials/claude.json"},
				},
			},
			Procedure: ResolvedProcedureSettings{ARB: &ARBSettings{
				CoreCommand:         "aar",
				CoreWorkingDir:      "/core-work",
				MCPCommand:          "aar-mcp-test",
				MCPWorkingDir:       "/mcp-work",
				PlaintiffProfile:    "plaintiff",
				DefendantProfile:    "defendant",
				LauncherPromptDir:   "/launcher/arb",
				LauncherPromptFiles: PromptFilePaths{"participant.pi": "/launcher/pi.md"},
				WebSearch:           defaultEnabled(nil),
				Timeout:             Duration(30 * time.Second),
			}},
		},
		RecordDir:  recordDir,
		CoreDir:    filepath.Join(recordDir, "core"),
		LogsDir:    filepath.Join(recordDir, "logs"),
		SessionDir: "/agent-state/case-1/arb",
	}
}
