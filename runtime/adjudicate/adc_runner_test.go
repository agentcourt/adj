package adjudicate

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	headless "github.com/agentcourt/adj/runtime/agent"
	localrun "github.com/agentcourt/adj/runtime/localrun/adc"
)

func TestADCRunnerBuildsJuryRequest(t *testing.T) {
	t.Setenv("DIRECT_OPENAI", "openai-selected")
	t.Setenv("DIRECT_OPENROUTER", "openrouter-selected")
	t.Setenv("ROLE_KEY", "role-selected")
	t.Setenv("OPENAI_API_KEY", "ambient-openai")
	t.Setenv("OPENROUTER_API_KEY", "ambient-openrouter")

	var received localrun.Options
	runner := ADCRunner{run: func(_ context.Context, opts localrun.Options) (localrun.Result, error) {
		received = opts
		return adcTestResult("judgment_entered", "closed", "demonstrated"), nil
	}}
	request := adcTestRequest()
	request.Settings.Procedure.ADC.CourtSubmissionErrorLimit = 7
	outcome, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != StatusOK || outcome.Decision == nil || outcome.Decision.Value != "demonstrated" {
		t.Fatalf("outcome = %#v", outcome)
	}
	if outcome.Provider.RequestCount != 3 {
		t.Fatalf("provider accounting = %#v", outcome.Provider)
	}
	if received.Proposition != request.Request.Proposition || received.DocumentsDir != filepath.Join(request.RecordDir, "inputs", "documents") {
		t.Fatalf("matter options = %#v", received)
	}
	if received.OutputDir != request.RecordDir || received.CoreOutputDir != request.CoreDir || received.LogsDir != request.LogsDir {
		t.Fatalf("output paths = run %q core %q logs %q", received.OutputDir, received.CoreOutputDir, received.LogsDir)
	}
	if received.MCPCommand != "/bin/adc-mcp" || received.MCPWorkingDir != "/core/adc-mcp" {
		t.Fatalf("MCP command options = %#v", received)
	}
	if received.EvidenceStandard != "clear_and_convincing" || received.MaxDocumentFiles != 12 || received.MaxDocumentFileBytes != 2048 || received.MaxDocumentsTotalBytes != 8192 {
		t.Fatalf("common options = %#v", received)
	}
	if received.JurorPersonasPath != "/settings/council.jsonl" || received.JurorCount != 7 || received.MinimumConcurring != 6 || received.UnanimousRequired != "false" {
		t.Fatalf("jury options = %#v", received)
	}
	if len(received.CouncilAllowedEndpoints) != 2 || received.CouncilAllowedEndpoints[0] != "openai" || received.CouncilAllowedEndpoints[1] != "openrouter" || received.CouncilMinEndpoints != 2 {
		t.Fatalf("jury endpoint options = %#v", received)
	}
	if received.TrialMode != "jury" || received.LawyerTimeoutSeconds != 30 || received.JurorTimeoutSeconds != 30 || received.TimeoutSeconds != 30 {
		t.Fatalf("runtime options = %#v", received)
	}
	if received.CourtSubmissionErrorLimit != 7 {
		t.Fatalf("court submission error limit = %d", received.CourtSubmissionErrorLimit)
	}
	if received.LauncherPromptDir != "/launcher/adc" || received.LauncherPromptFiles["juror.pi"] != "/launcher/juror.md" {
		t.Fatalf("launcher prompt options = %#v", received)
	}
	if received.PlaintiffLawyer.Runner != localrun.LawyerCodex || received.PlaintiffLawyer.AuthMode != headless.AuthSubscription || received.PlaintiffLawyer.CredentialsFile != "/credentials/codex.json" {
		t.Fatalf("plaintiff profile = %#v", received.PlaintiffLawyer)
	}
	if received.DefendantLawyer.Runner != localrun.LawyerClaude || received.DefendantLawyer.AuthMode != headless.AuthAPIKey || received.DefendantLawyer.APIKeyEnv != "ROLE_KEY" {
		t.Fatalf("defendant profile = %#v", received.DefendantLawyer)
	}
	if received.PlaintiffLawyer.Provider != "openai" || received.PlaintiffLawyer.ReasoningEffort != "xhigh" || received.DefendantLawyer.Provider != "openrouter" || received.DefendantLawyer.ReasoningEffort != "high" {
		t.Fatalf("lawyer provider and reasoning settings = %#v %#v", received.PlaintiffLawyer, received.DefendantLawyer)
	}
	if received.LawyerWebSearch == nil || !*received.LawyerWebSearch {
		t.Fatalf("lawyer web search = %#v", received.LawyerWebSearch)
	}
	if received.PlaintiffLawyer.StateDir != "/agent-state/case-1/adc/plaintiff/plaintiff" || received.PlaintiffLawyer.WorkDir != "/records/case-1/work/plaintiff" {
		t.Fatalf("plaintiff state and work directories = %#v", received.PlaintiffLawyer)
	}
	if received.DefendantLawyer.StateDir != "/agent-state/case-1/adc/defendant/defendant" || received.DefendantLawyer.WorkDir != "/records/case-1/work/defendant" {
		t.Fatalf("defendant state and work directories = %#v", received.DefendantLawyer)
	}
	if value, _ := environmentValue(received.CoreEnvironment, "OPENAI_API_KEY"); value != "openai-selected" {
		t.Fatalf("OPENAI_API_KEY = %q", value)
	}
	if value, _ := environmentValue(received.CoreEnvironment, "OPENROUTER_API_KEY"); value != "openrouter-selected" {
		t.Fatalf("OPENROUTER_API_KEY = %q", value)
	}
	if _, ok := environmentValue(received.CoreEnvironment, "ROLE_KEY"); ok {
		t.Fatal("core environment contains participant credential source ROLE_KEY")
	}
	for _, name := range []string{"DIRECT_OPENAI", "DIRECT_OPENROUTER", "ROLE_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		if _, ok := environmentValue(received.ParticipantEnvironment, name); ok {
			t.Fatalf("participant environment contains provider credential %s", name)
		}
		if _, ok := environmentValue(received.MCPEnvironment, name); ok {
			t.Fatalf("MCP environment contains provider credential %s", name)
		}
	}
	if value, ok := environmentValue(received.DefendantLawyer.Environment, "ROLE_KEY"); !ok || value != "role-selected" {
		t.Fatalf("defendant credential = %q, %t", value, ok)
	}
	if _, ok := environmentValue(received.PlaintiffLawyer.Environment, "ROLE_KEY"); ok {
		t.Fatal("plaintiff environment contains defendant credential")
	}
}

func TestADCRunnerBenchUsesOnlyOpenAIAndNoCouncil(t *testing.T) {
	t.Setenv("DIRECT_OPENAI", "openai-selected")
	t.Setenv("ROLE_KEY", "role-selected")
	t.Setenv("OPENAI_API_KEY", "ambient-openai")
	t.Setenv("OPENROUTER_API_KEY", "ambient-openrouter")

	request := adcTestRequest()
	request.Settings.Procedure.ADC.TrialMode = "bench"
	delete(request.Settings.Common.ProviderCredentials, "openrouter")
	var received localrun.Options
	runner := ADCRunner{run: func(_ context.Context, opts localrun.Options) (localrun.Result, error) {
		received = opts
		return adcTestResult("closed", "none", "no_decision"), nil
	}}
	outcome, err := runner.Run(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Decision == nil || outcome.Decision.Value != "no_decision" {
		t.Fatalf("outcome = %#v", outcome)
	}
	if received.JurorPersonasPath != "" || received.JurorCount != 0 || received.MinimumConcurring != 0 {
		t.Fatalf("bench jury options = %#v", received)
	}
	if _, ok := environmentValue(received.CoreEnvironment, "OPENROUTER_API_KEY"); ok {
		t.Fatalf("bench environment contains OPENROUTER_API_KEY")
	}
}

func TestADCRunnerPreservesRunError(t *testing.T) {
	want := errors.New("core failed")
	runner := ADCRunner{run: func(context.Context, localrun.Options) (localrun.Result, error) {
		return adcTestResult("judgment_entered", "closed", "demonstrated"), want
	}}
	request := adcTestRequest()
	t.Setenv("DIRECT_OPENAI", "openai-selected")
	t.Setenv("DIRECT_OPENROUTER", "openrouter-selected")
	t.Setenv("ROLE_KEY", "role-selected")
	_, err := runner.Run(context.Background(), request)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func TestMapADCResultRejectsIncompleteResult(t *testing.T) {
	tests := []struct {
		name   string
		result localrun.Result
		want   string
	}{
		{name: "no case", result: localrun.Result{FinalState: map[string]any{}}, want: "no case state"},
		{name: "no phase", result: adcTestResult("closed", "", "no_decision"), want: "empty phase"},
		{name: "nonterminal", result: adcTestResult("trial", "deliberation", "pending"), want: "nonterminal case status"},
		{name: "resolution", result: adcTestResult("closed", "none", "pending"), want: "invalid resolution"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := mapADCResult(test.result, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v", err)
			}
			if test.result.Provider.RequestCount > 0 && providerAccountingFromError(err).RequestCount != test.result.Provider.RequestCount {
				t.Fatalf("provider accounting = %#v", providerAccountingFromError(err))
			}
		})
	}
}

func TestResolvedADCLawyerProfileRejectsUnknownAuthentication(t *testing.T) {
	_, err := resolvedADCLawyerProfile(ResolvedSettings{AgentProfiles: map[string]ResolvedAgentProfile{
		"lawyer": {Runner: RunnerCodex, Authentication: Authentication{Source: "token"}},
	}}, "lawyer")
	if err == nil || !strings.Contains(err.Error(), "unsupported authentication") {
		t.Fatalf("error = %v", err)
	}
}

func adcTestRequest() ProcedureRequest {
	resume := true
	return ProcedureRequest{
		Request: Request{
			Procedure:   ProcedureADC,
			CaseID:      "case-1",
			RunID:       "run-1",
			Proposition: "The sky is blue.",
		},
		Settings: ResolvedSettings{
			Common: CommonSettings{
				EvidenceStandard:        "clear_and_convincing",
				CouncilPool:             "/settings/council.jsonl",
				CouncilAllowedEndpoints: []string{"openai", "openrouter"},
				CouncilMinEndpoints:     2,
				CouncilSize:             7,
				RequiredVotes:           6,
				DocumentLimits:          DocumentLimits{Count: 12, PerFile: 2048, Total: 8192},
				ProviderCredentials: map[string]CredentialMetadata{
					"openai":     {Source: AuthAPIKey, EnvironmentVariable: "DIRECT_OPENAI"},
					"openrouter": {Source: AuthAPIKey, EnvironmentVariable: "DIRECT_OPENROUTER"},
				},
			},
			AgentProfiles: map[string]ResolvedAgentProfile{
				"plaintiff": {
					Runner: RunnerCodex, Provider: ProviderOpenAI, Command: "/bin/codex", ReasoningEffort: "xhigh", Resume: resume,
					Authentication: Authentication{Source: AuthSubscription, Path: "/credentials/codex.json"},
				},
				"defendant": {
					Runner: RunnerClaude, Provider: ProviderOpenRouter, Command: "/bin/claude", ReasoningEffort: "high", Resume: resume,
					Authentication: Authentication{Source: AuthAPIKey, EnvironmentVariable: "ROLE_KEY"},
				},
			},
			Procedure: ResolvedProcedureSettings{
				Name: ProcedureADC,
				ADC: &ADCSettings{
					CoreCommand: "/bin/adc", CoreWorkingDir: "/core/adc",
					MCPCommand: "/bin/adc-mcp", MCPWorkingDir: "/core/adc-mcp",
					PlaintiffProfile: "plaintiff", DefendantProfile: "defendant",
					LauncherPromptDir:   "/launcher/adc",
					LauncherPromptFiles: PromptFilePaths{"juror.pi": "/launcher/juror.md"},
					WebSearch:           defaultEnabled(nil),
					TrialMode:           "jury", Timeout: Duration(30 * time.Second),
				},
			},
		},
		RecordDir:  "/records/case-1",
		CoreDir:    "/records/case-1/core",
		LogsDir:    "/records/case-1/logs",
		SessionDir: "/agent-state/case-1/adc",
	}
}

func adcTestResult(status, phase, resolution string) localrun.Result {
	return localrun.Result{Provider: ProviderManagement{RequestCount: 3}, FinalState: map[string]any{
		"case": map[string]any{
			"status":     status,
			"phase":      phase,
			"resolution": resolution,
		},
	}}
}
