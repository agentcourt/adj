package adjudicate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestLoadSettingsRejectsUnknownFieldsAndTrailingValues(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "top-level field",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{},"extra":true}`,
			want: "unknown field",
		},
		{
			name: "unselected procedure field",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{"arb":{"plaintiff_profile":"p","defendant_profile":"p","invented":true},"simple":{"model":"openai://model"}},"agent_profiles":{"p":{"runner":"codex"}}}`,
			want: "unknown field",
		},
		{
			name: "removed instructions file",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{"simple":{"model":"openai://model"}},"agent_profiles":{"p":{"runner":"codex","instructions_file":"lawyer.md"}}}`,
			want: "unknown field",
		},
		{
			name: "removed working directory",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{"simple":{"model":"openai://model"}},"agent_profiles":{"p":{"runner":"codex","working_directory":"work"}}}`,
			want: "unknown field",
		},
		{
			name: "removed timeout",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{"simple":{"model":"openai://model"}},"agent_profiles":{"p":{"runner":"codex","timeout":"1m"}}}`,
			want: "unknown field",
		},
		{
			name: "removed output limit",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{"simple":{"model":"openai://model"}},"agent_profiles":{"p":{"runner":"codex","output_limit_bytes":1024}}}`,
			want: "unknown field",
		},
		{
			name: "removed simple permission",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{"simple":{"model":"openai://model","allow_api_key":true}}}`,
			want: "unknown field",
		},
		{
			name: "removed simple credential source",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{"simple":{"model":"openai://model","credential_source":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}}}}`,
			want: "unknown field",
		},
		{
			name: "removed procedure evidence standard",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence"},"procedures":{"simple":{"model":"openai://model","evidence_standard":"preponderance_of_the_evidence"}}}`,
			want: "unknown field",
		},
		{
			name: "removed quick council settings",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence"},"procedures":{"quick":{"request_pool":"pool.jsonl"}}}`,
			want: "unknown field",
		},
		{
			name: "removed arb council backend",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence"},"procedures":{"arb":{"council_backend":"direct"}}}`,
			want: "unknown field",
		},
		{
			name: "removed adc persona pool",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence"},"procedures":{"adc":{"persona_pool":"pool.jsonl"}}}`,
			want: "unknown field",
		},
		{
			name: "trailing value",
			data: `{"schema_version":"adjudicate.settings.v1","common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},"procedures":{}} {}`,
			want: "unexpected JSON value",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestFile(t, "settings.json", test.data)
			_, err := LoadSettings(path, ProcedureSimple)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadSettings() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestLoadSettingsValidatesEveryProcedure(t *testing.T) {
	path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","council_pool":"pool.jsonl","council_size":3,"required_votes":2,"allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "agent_profiles":{},
  "procedures":{
    "simple":{"model":"openai://model"},
    "quick":{"plaintiff_profile":"missing","defendant_profile":"missing"}
  }
}`)
	_, err := LoadSettings(path, ProcedureSimple)
	if err == nil || !strings.Contains(err.Error(), `procedure quick: agent profile "missing" is not defined`) {
		t.Fatalf("LoadSettings() error = %v", err)
	}
}

func TestLoadSettingsResolvesPathsAndCommonLawyerProfile(t *testing.T) {
	root := t.TempDir()
	settingsDir := filepath.Join(root, "settings")
	if err := os.Mkdir(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(settingsDir, "settings.json")
	data := `{
  "schema_version":"adjudicate.settings.v1",
  "common":{
    "output_root":"../records",
	"agent_state_root":"../agent-state",
    "lawyer_profile":"lawyer",
	"evidence_standard":"preponderance_of_the_evidence",
	"council_pool":"pools/council.jsonl",
	"council_size":3,
	"required_votes":2,
	"allow_api_key":true,
	"provider_credentials":{"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},
    "document_limits":{"count":10,"per_file_bytes":100,"total_bytes":500}
  },
  "agent_profiles":{
    "lawyer":{
      "runner":"codex",
      "command":"../bin/codex",
	  "reasoning_effort":"xhigh",
      "authentication":{"source":"subscription","path":"credentials/codex.json"}
    }
  },
	"procedures":{
		"quick":{
			"core_command":"../bin/quick",
			"core_working_directory":"work/core",
			"mcp_command":"../bin/quick-mcp",
			"web_search":false,
			"prompt_dir":"prompts/core",
			"launcher_prompt_dir":"prompts/launcher",
			"prompt_files":{
				"lawyer.common":"prompts/common.md",
				"search.enabled":"prompts/search-on.md",
				"search.disabled":"prompts/search-off.md",
				"lawyer.proponent":"prompts/for.md",
				"lawyer.opponent":"prompts/against.md",
				"council.system":"prompts/council.md",
				"mcp.session.instructions":"prompts/mcp-session.md"
			},
			"launcher_prompt_files":{
				"participant":"prompts/participant.md",
				"participant.pi":"prompts/pi-participant.md"
			}
		}
  }
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadSettings(path, ProcedureQuick)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Common.OutputRoot != filepath.Join(root, "records") {
		t.Fatalf("output root = %q", settings.Common.OutputRoot)
	}
	if settings.Common.AgentStateRoot != filepath.Join(root, "agent-state") {
		t.Fatalf("agent state root = %q", settings.Common.AgentStateRoot)
	}
	profile := settings.AgentProfiles["lawyer"]
	if profile.Command != filepath.Join(root, "bin", "codex") {
		t.Fatalf("agent command = %q", profile.Command)
	}
	if profile.ReasoningEffort != "xhigh" {
		t.Fatalf("agent reasoning effort = %q", profile.ReasoningEffort)
	}
	if profile.Authentication.Path != filepath.Join(settingsDir, "credentials", "codex.json") {
		t.Fatalf("authentication path = %q", profile.Authentication.Path)
	}
	quick := settings.Procedure.Quick
	if quick.PlaintiffProfile != "lawyer" || quick.DefendantProfile != "lawyer" {
		t.Fatalf("lawyer profiles = %q, %q", quick.PlaintiffProfile, quick.DefendantProfile)
	}
	if quick.WebSearch {
		t.Fatal("explicit false enabled Quick web search")
	}
	if settings.Common.CouncilPool != filepath.Join(settingsDir, "pools", "council.jsonl") {
		t.Fatalf("council pool = %q", settings.Common.CouncilPool)
	}
	if quick.CoreCommand != filepath.Join(root, "bin", "quick") {
		t.Fatalf("core command = %q", quick.CoreCommand)
	}
	if quick.MCPCommand != filepath.Join(root, "bin", "quick-mcp") {
		t.Fatalf("MCP command = %q", quick.MCPCommand)
	}
	if quick.CoreWorkingDir != filepath.Join(settingsDir, "work", "core") {
		t.Fatalf("core working directory = %q", quick.CoreWorkingDir)
	}
	if quick.MCPWorkingDir != filepath.Join(settingsDir, "work", "core") {
		t.Fatalf("MCP working directory = %q", quick.MCPWorkingDir)
	}
	if quick.PromptDir != filepath.Join(settingsDir, "prompts", "core") || quick.LauncherPromptDir != filepath.Join(settingsDir, "prompts", "launcher") {
		t.Fatalf("prompt directories = %q, %q", quick.PromptDir, quick.LauncherPromptDir)
	}
	allPromptFiles := PromptFilePaths{}
	for id, path := range quick.PromptFiles {
		allPromptFiles[id] = path
	}
	for id, path := range quick.LauncherPromptFiles {
		allPromptFiles[id] = path
	}
	for name, got := range map[string]string{
		"participant":     allPromptFiles["participant"],
		"pi participant":  allPromptFiles["participant.pi"],
		"lawyer":          allPromptFiles["lawyer.common"],
		"search enabled":  allPromptFiles["search.enabled"],
		"search disabled": allPromptFiles["search.disabled"],
		"proponent":       allPromptFiles["lawyer.proponent"],
		"opponent":        allPromptFiles["lawyer.opponent"],
		"council":         allPromptFiles["council.system"],
		"MCP session":     allPromptFiles["mcp.session.instructions"],
	} {
		want := filepath.Join(settingsDir, "prompts", map[string]string{
			"participant":     "participant.md",
			"pi participant":  "pi-participant.md",
			"lawyer":          "common.md",
			"search enabled":  "search-on.md",
			"search disabled": "search-off.md",
			"proponent":       "for.md",
			"opponent":        "against.md",
			"council":         "council.md",
			"MCP session":     "mcp-session.md",
		}[name])
		if got != want {
			t.Fatalf("%s prompt file = %q, want %q", name, got, want)
		}
	}
}

func TestFormalMCPSettingsDefaultToCoreWorkingDirectory(t *testing.T) {
	base := t.TempDir()
	profiles := map[string]ResolvedAgentProfile{
		"lawyer": {Runner: RunnerOpenClaw},
	}
	arb := &ARBSettings{CoreWorkingDir: "arb-core", PlaintiffProfile: "lawyer", DefendantProfile: "lawyer"}
	if err := resolveARB(arb, profiles, base, ""); err != nil {
		t.Fatal(err)
	}
	arbd := &ARBDSettings{CoreWorkingDir: "arbd-core", PlaintiffProfile: "lawyer", DefendantProfile: "lawyer", JudgmentStandard: "score", PromptFiles: PromptFilePaths{"mcp.session.instructions": "session.md"}}
	if err := resolveARBD(arbd, profiles, base, ""); err != nil {
		t.Fatal(err)
	}
	adc := &ADCSettings{CoreWorkingDir: "adc-core", PlaintiffProfile: "lawyer", DefendantProfile: "lawyer"}
	if err := resolveADC(adc, profiles, base, ""); err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]struct {
		command     string
		wantCommand string
		workDir     string
		wantWorkDir string
	}{
		"arb":  {arb.MCPCommand, "aar-mcp", arb.MCPWorkingDir, filepath.Join(base, "arb-core")},
		"arbd": {arbd.MCPCommand, "aard-mcp", arbd.MCPWorkingDir, filepath.Join(base, "arbd-core")},
		"adc":  {adc.MCPCommand, "adc-mcp", adc.MCPWorkingDir, filepath.Join(base, "adc-core")},
	} {
		if got.command != got.wantCommand {
			t.Fatalf("%s MCP command = %q, want %q", name, got.command, got.wantCommand)
		}
		if got.workDir != got.wantWorkDir {
			t.Fatalf("%s MCP working directory = %q, want %q", name, got.workDir, got.wantWorkDir)
		}
	}

	bad := &ARBSettings{PlaintiffProfile: "lawyer", DefendantProfile: "lawyer", PromptFiles: PromptFilePaths{"mcp.": "bad.md"}}
	if err := resolveARB(bad, profiles, base, ""); err == nil || !strings.Contains(err.Error(), "MCP prompt ID is empty") {
		t.Fatalf("empty MCP prompt ID error = %v", err)
	}
}

func TestManualLawyerSettingsRequireOnlyAutomaticProfiles(t *testing.T) {
	base := t.TempDir()
	profiles := map[string]ResolvedAgentProfile{"local": {Runner: RunnerOpenClaw}}

	for name, resolve := range map[string]func() error{
		"arb": func() error {
			settings := ARBSettings{AutoLawyers: "none"}
			return resolveARB(&settings, nil, base, "")
		},
		"arbd": func() error {
			settings := ARBDSettings{AutoLawyers: "none", JudgmentStandard: "score"}
			return resolveARBD(&settings, nil, base, "")
		},
		"adc": func() error {
			settings := ADCSettings{AutoLawyers: "none", TrialMode: "bench"}
			return resolveADC(&settings, nil, base, "")
		},
		"quick": func() error {
			settings := QuickSettings{AutoLawyers: "none"}
			_, err := resolveQuick(&settings, nil, base, "")
			return err
		},
	} {
		if err := resolve(); err != nil {
			t.Fatalf("%s full manual mode: %v", name, err)
		}
	}

	for name, resolve := range map[string]func() error{
		"arb": func() error {
			settings := ARBSettings{AutoLawyers: "plaintiff", PlaintiffProfile: "local", DefendantProfile: "absent"}
			return resolveARB(&settings, profiles, base, "")
		},
		"arbd": func() error {
			settings := ARBDSettings{AutoLawyers: "plaintiff", PlaintiffProfile: "local", DefendantProfile: "absent", JudgmentStandard: "score"}
			return resolveARBD(&settings, profiles, base, "")
		},
		"adc": func() error {
			settings := ADCSettings{AutoLawyers: "plaintiff", PlaintiffProfile: "local", DefendantProfile: "absent", TrialMode: "bench"}
			return resolveADC(&settings, profiles, base, "")
		},
		"quick": func() error {
			settings := QuickSettings{AutoLawyers: "plaintiff", PlaintiffProfile: "local", DefendantProfile: "absent"}
			_, err := resolveQuick(&settings, profiles, base, "")
			return err
		},
	} {
		if err := resolve(); err != nil {
			t.Fatalf("%s partial manual mode: %v", name, err)
		}
	}
}

func TestCommonLawyerProfileSuppliesARBDRoleProfiles(t *testing.T) {
	procedures := withCommonLawyerProfile(ProcedureSettings{ARBD: &ARBDSettings{
		PlaintiffProfile: "plaintiff",
	}}, "common")
	if procedures.ARBD == nil {
		t.Fatal("ARBD settings are absent")
	}
	if procedures.ARBD.PlaintiffProfile != "plaintiff" || procedures.ARBD.DefendantProfile != "common" {
		t.Fatalf("ARBD role profiles = %q, %q", procedures.ARBD.PlaintiffProfile, procedures.ARBD.DefendantProfile)
	}
}

func TestLoadSettingsDefaultsAgentStateUnderOutputRoot(t *testing.T) {
	path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
	"procedures":{"simple":{"model":"openai://model","reasoning_effort":"xhigh","max_output_tokens":16384,"max_tool_calls":8,"web_search":false,"prompt_dir":"prompts/simple","prompt_files":{"decision":"prompts/decision.md","search.enabled":"prompts/search-on.md","search.disabled":"prompts/search-off.md"}}}
}`)
	settings, err := LoadSettings(path, ProcedureSimple)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(path), "out", ".agents")
	if settings.Common.AgentStateRoot != want {
		t.Fatalf("agent state root = %q, want %q", settings.Common.AgentStateRoot, want)
	}
	if settings.Procedure.Simple.WebSearch {
		t.Fatal("explicit false enabled Simple web search")
	}
	if settings.Procedure.Simple.ReasoningEffort != "xhigh" || settings.Procedure.Simple.MaxOutputTokens != 16384 || settings.Procedure.Simple.MaxToolCalls != 8 {
		t.Fatalf("Simple model request settings = %q, %d, %d", settings.Procedure.Simple.ReasoningEffort, settings.Procedure.Simple.MaxOutputTokens, settings.Procedure.Simple.MaxToolCalls)
	}
	if settings.Procedure.Simple.PromptDir != filepath.Join(filepath.Dir(path), "prompts", "simple") {
		t.Fatalf("prompt directory = %q", settings.Procedure.Simple.PromptDir)
	}
	for name, got := range map[string]string{
		"decision":        settings.Procedure.Simple.PromptFiles["decision"],
		"search enabled":  settings.Procedure.Simple.PromptFiles["search.enabled"],
		"search disabled": settings.Procedure.Simple.PromptFiles["search.disabled"],
	} {
		filename := map[string]string{"decision": "decision.md", "search enabled": "search-on.md", "search disabled": "search-off.md"}[name]
		want := filepath.Join(filepath.Dir(path), "prompts", filename)
		if got != want {
			t.Fatalf("%s prompt file = %q, want %q", name, got, want)
		}
	}
	for _, bad := range []struct {
		settings SimpleSettings
		want     string
	}{
		{settings: SimpleSettings{Model: "openai://model", ReasoningEffort: "ultra"}, want: "reasoning_effort"},
		{settings: SimpleSettings{Model: "openai://model", MaxOutputTokens: -1}, want: "max_output_tokens"},
		{settings: SimpleSettings{Model: "openai://model", MaxToolCalls: -1}, want: "max_tool_calls"},
	} {
		if _, err := resolveSimple(&bad.settings, nil, filepath.Dir(path), ""); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Fatalf("resolve Simple settings error = %v, want %q", err, bad.want)
		}
	}
}

func TestAgentProfileNameMustBeOnePathComponent(t *testing.T) {
	for _, name := range []string{"../lawyer", "group/lawyer", " lawyer"} {
		t.Run(name, func(t *testing.T) {
			path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "agent_profiles":{`+strconv.Quote(name)+`:{"runner":"codex"}},
  "procedures":{"simple":{"model":"openai://model"}}
}`)
			_, err := LoadSettings(path, ProcedureSimple)
			if err == nil || !strings.Contains(err.Error(), "one path component") {
				t.Fatalf("LoadSettings() error = %v", err)
			}
		})
	}
}

func TestOneSettingsFileSelectsEveryProcedure(t *testing.T) {
	path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{
    "lawyer_profile":"lawyer",
    "evidence_standard":"preponderance_of_the_evidence",
    "council_pool":"pool.jsonl",
    "council_size":3,
    "required_votes":2,
    "allow_api_key":true,
    "provider_credentials":{
      "openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},
      "openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}
    },
    "document_limits":{"count":10,"per_file_bytes":100,"total_bytes":500}
  },
  "agent_profiles":{"lawyer":{"runner":"openclaw"}},
  "procedures":{
    "arb":{},
	"arbd":{"judgment_standard":"score from 0 through 100"},
    "adc":{"trial_mode":"bench"},
    "simple":{"model":"openrouter://model"},
    "quick":{}
  }
}`)
	for _, procedure := range []Procedure{ProcedureSimple, ProcedureQuick, ProcedureARBD, ProcedureARB, ProcedureADC} {
		settings, err := LoadSettings(path, procedure)
		if err != nil {
			t.Fatalf("LoadSettings(%q): %v", procedure, err)
		}
		if settings.Procedure.Name != procedure {
			t.Fatalf("LoadSettings(%q) selected %q", procedure, settings.Procedure.Name)
		}
		var webSearch bool
		switch procedure {
		case ProcedureARB:
			webSearch = settings.Procedure.ARB.WebSearch != nil && *settings.Procedure.ARB.WebSearch
		case ProcedureARBD:
			webSearch = settings.Procedure.ARBD.WebSearch != nil && *settings.Procedure.ARBD.WebSearch
		case ProcedureADC:
			webSearch = settings.Procedure.ADC.WebSearch != nil && *settings.Procedure.ADC.WebSearch
		case ProcedureSimple:
			webSearch = settings.Procedure.Simple.WebSearch
		case ProcedureQuick:
			webSearch = settings.Procedure.Quick.WebSearch
		}
		if !webSearch {
			t.Fatalf("omitted %s web_search did not default to true", procedure)
		}
	}
}

func TestFormalWebSearchExplicitFalse(t *testing.T) {
	disabled := false
	profiles := map[string]ResolvedAgentProfile{
		"lawyer": {Runner: RunnerOpenClaw},
	}
	arb := ARBSettings{PlaintiffProfile: "lawyer", DefendantProfile: "lawyer", WebSearch: &disabled}
	if err := resolveARB(&arb, profiles, t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	arbd := ARBDSettings{PlaintiffProfile: "lawyer", DefendantProfile: "lawyer", JudgmentStandard: "score from 0 through 100", WebSearch: &disabled}
	if err := resolveARBD(&arbd, profiles, t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	adc := ADCSettings{PlaintiffProfile: "lawyer", DefendantProfile: "lawyer", TrialMode: "bench", WebSearch: &disabled}
	if err := resolveADC(&adc, profiles, t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	for name, configured := range map[string]*bool{
		"arb":  arb.WebSearch,
		"arbd": arbd.WebSearch,
		"adc":  adc.WebSearch,
	} {
		if configured == nil || *configured {
			t.Fatalf("explicit false enabled %s web search", name)
		}
	}
}

func TestResolveCommonAdjudicationSettings(t *testing.T) {
	tests := []struct {
		name       string
		common     CommonSettings
		procedures ProcedureSettings
		wantError  string
	}{
		{
			name:       "simple without council",
			common:     CommonSettings{EvidenceStandard: "clear_and_convincing"},
			procedures: ProcedureSettings{Simple: &SimpleSettings{}},
		},
		{
			name:       "missing evidence standard",
			procedures: ProcedureSettings{Simple: &SimpleSettings{}},
			wantError:  "common evidence_standard is required",
		},
		{
			name:       "unsupported evidence standard",
			common:     CommonSettings{EvidenceStandard: "preponderance"},
			procedures: ProcedureSettings{Simple: &SimpleSettings{}},
			wantError:  "common evidence_standard must be",
		},
		{
			name:       "ARBD without evidence standard or votes",
			common:     CommonSettings{CouncilPool: "pool.jsonl", CouncilSize: 3},
			procedures: ProcedureSettings{ARBD: &ARBDSettings{}},
		},
		{
			name:       "missing council pool",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence", CouncilSize: 3, RequiredVotes: 2},
			procedures: ProcedureSettings{Quick: &QuickSettings{}},
			wantError:  "common council_pool is required",
		},
		{
			name:       "missing council size",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence", CouncilPool: "pool.jsonl", RequiredVotes: 1},
			procedures: ProcedureSettings{ARB: &ARBSettings{}},
			wantError:  "common council_size must be positive",
		},
		{
			name:       "nonmajority threshold",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence", CouncilPool: "pool.jsonl", CouncilSize: 5, RequiredVotes: 2},
			procedures: ProcedureSettings{Quick: &QuickSettings{}},
			wantError:  "common required_votes must be a majority between 3 and council_size",
		},
		{
			name:       "threshold above council size",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence", CouncilPool: "pool.jsonl", CouncilSize: 3, RequiredVotes: 4},
			procedures: ProcedureSettings{ARB: &ARBSettings{}},
			wantError:  "common required_votes must be a majority between 2 and council_size",
		},
		{
			name:       "ADC jury requires council",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence"},
			procedures: ProcedureSettings{ADC: &ADCSettings{TrialMode: "jury"}},
			wantError:  "common council_pool is required",
		},
		{
			name:       "ADC jury size",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence", CouncilPool: "pool.jsonl", CouncilSize: 5, RequiredVotes: 3},
			procedures: ProcedureSettings{ADC: &ADCSettings{TrialMode: "auto"}},
			wantError:  "ADC jury council_size must be between 6 and 12",
		},
		{
			name:       "ADC jury concurring votes",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence", CouncilPool: "pool.jsonl", CouncilSize: 7, RequiredVotes: 4},
			procedures: ProcedureSettings{ADC: &ADCSettings{TrialMode: "jury"}},
			wantError:  "ADC jury required_votes must be at least 6",
		},
		{
			name:       "ADC bench without council",
			common:     CommonSettings{EvidenceStandard: "preponderance_of_the_evidence"},
			procedures: ProcedureSettings{ADC: &ADCSettings{TrialMode: "bench"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			common := test.common
			err := resolveCommonAdjudicationSettings(&common, test.procedures, "/settings", "/home/test")
			if test.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("resolveCommonAdjudicationSettings() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestResolveADCNormalizesTrialMode(t *testing.T) {
	profiles := map[string]ResolvedAgentProfile{"lawyer": {Runner: RunnerCodex}}
	settings := ADCSettings{PlaintiffProfile: "lawyer", DefendantProfile: "lawyer"}
	if err := resolveADC(&settings, profiles, "/settings", "/home/test"); err != nil {
		t.Fatal(err)
	}
	if settings.TrialMode != "auto" {
		t.Fatalf("trial mode = %q", settings.TrialMode)
	}
	settings.TrialMode = "summary"
	if err := resolveADC(&settings, profiles, "/settings", "/home/test"); err == nil || !strings.Contains(err.Error(), "trial_mode") {
		t.Fatalf("invalid trial mode error = %v", err)
	}
}

func TestCommonLawyerProfileMustExist(t *testing.T) {
	path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"lawyer_profile":"missing","evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "procedures":{"simple":{"model":"openai://model"}}
}`)
	_, err := LoadSettings(path, ProcedureSimple)
	if err == nil || !strings.Contains(err.Error(), `common lawyer profile "missing" is not defined`) {
		t.Fatalf("LoadSettings() error = %v", err)
	}
}

func TestAgentAuthenticationDefaultsAndAPIKeyOptIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
	"agent_profiles":{
    "openclaw":{"runner":"openclaw"},
    "codex":{"runner":"codex"},
    "claude":{"runner":"claude"},
	"pi-openai":{"runner":"pi","model":"openai/gpt-5.6-sol"},
    "pi":{"runner":"pi","model":"openrouter/openai/gpt-5.6-sol","authentication":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}}
  },
  "procedures":{"simple":{"model":"openai://model"}}
}`)
	settings, err := LoadSettings(path, ProcedureSimple)
	if err != nil {
		t.Fatal(err)
	}
	if settings.AgentProfiles["openclaw"].Resume {
		t.Fatal("OpenClaw defaults to session resumption")
	}
	for name, want := range map[string]AgentProvider{
		"openclaw": ProviderOpenAI,
		"codex":    ProviderOpenAI,
		"claude":   ProviderAnthropic,
	} {
		if got := settings.AgentProfiles[name].Provider; got != want {
			t.Fatalf("profile %q provider = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"codex", "claude", "pi-openai", "pi"} {
		if !settings.AgentProfiles[name].Resume {
			t.Fatalf("profile %q does not default to session resumption", name)
		}
	}
	if got := settings.AgentProfiles["codex"].Authentication; got.Source != AuthSubscription || got.Path != filepath.Join(home, ".codex", "auth.json") {
		t.Fatalf("Codex authentication = %#v", got)
	}
	if got := settings.AgentProfiles["openclaw"].Authentication; got.Source != AuthSubscription || got.Path != filepath.Join(home, ".codex", "auth.json") {
		t.Fatalf("OpenClaw authentication = %#v", got)
	}
	if got := settings.AgentProfiles["claude"].Authentication; got.Source != AuthSubscription || got.Path != filepath.Join(home, ".claude", ".credentials.json") {
		t.Fatalf("Claude authentication = %#v", got)
	}
	if got := settings.AgentProfiles["pi"].Authentication; got.Source != AuthAPIKey || got.EnvironmentVariable != "OPENROUTER_API_KEY" || got.Path != "" {
		t.Fatalf("Pi authentication = %#v", got)
	}
	if got := settings.AgentProfiles["pi-openai"].Authentication; got.Source != AuthSubscription || got.Path != filepath.Join(home, ".codex", "auth.json") || got.EnvironmentVariable != "" {
		t.Fatalf("OpenAI Pi authentication = %#v", got)
	}
}

func TestAgentProfileRunnerRestrictions(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{
			name:    "OpenClaw resume",
			profile: `{"runner":"openclaw","resume":true}`,
			want:    "openclaw does not support session resumption",
		},
		{
			name:    "OpenClaw command",
			profile: `{"runner":"openclaw","command":"openclaw"}`,
			want:    "openclaw does not accept a command",
		},
		{
			name:    "Pi command",
			profile: `{"runner":"pi","command":"pi","authentication":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}}`,
			want:    "pi does not accept a command",
		},
		{
			name:    "OpenClaw provider",
			profile: `{"runner":"openclaw","model":"anthropic/claude-sonnet-4"}`,
			want:    `does not match provider "openai"`,
		},
		{
			name:    "reasoning effort",
			profile: `{"runner":"codex","reasoning_effort":"maximum"}`,
			want:    "reasoning_effort must be low, medium, high, or xhigh",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "agent_profiles":{"agent":`+test.profile+`},
  "procedures":{"simple":{"model":"openai://model"}}
}`)
			_, err := LoadSettings(path, ProcedureSimple)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadSettings() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestAgentProfileProviderResolution(t *testing.T) {
	tests := []struct {
		name         string
		profile      AgentProfile
		wantProvider AgentProvider
		wantError    string
	}{
		{
			name: "Claude through OpenRouter",
			profile: AgentProfile{
				Runner:   RunnerClaude,
				Provider: ProviderOpenRouter,
				Model:    "openai/gpt-5.6-sol",
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OPENROUTER_API_KEY",
				},
			},
			wantProvider: ProviderOpenRouter,
		},
		{
			name: "OpenClaw with Anthropic",
			profile: AgentProfile{
				Runner:   RunnerOpenClaw,
				Provider: ProviderAnthropic,
				Model:    "anthropic/claude-opus-4-8",
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "ANTHROPIC_API_KEY",
				},
			},
			wantProvider: ProviderAnthropic,
		},
		{
			name: "Pi infers provider",
			profile: AgentProfile{
				Runner: RunnerPi,
				Model:  "openrouter/anthropic/claude-opus-4-8",
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OPENROUTER_API_KEY",
				},
			},
			wantProvider: ProviderOpenRouter,
		},
		{
			name: "Pi explicit provider matches model",
			profile: AgentProfile{
				Runner:   RunnerPi,
				Provider: ProviderAnthropic,
				Model:    "anthropic/claude-opus-4-8",
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "ANTHROPIC_API_KEY",
				},
			},
			wantProvider: ProviderAnthropic,
		},
		{
			name: "Pi provider conflicts with model",
			profile: AgentProfile{
				Runner:   RunnerPi,
				Provider: ProviderAnthropic,
				Model:    "openrouter/anthropic/claude-opus-4-8",
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OPENROUTER_API_KEY",
				},
			},
			wantError: `Pi provider "anthropic" does not match model provider "openrouter"`,
		},
		{
			name: "Pi omitted provider with unqualified model",
			profile: AgentProfile{
				Runner: RunnerPi,
				Model:  "gpt-5.6-sol",
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OPENAI_API_KEY",
				},
			},
			wantError: "Pi model must use provider/model form",
		},
		{
			name: "Pi omitted provider and model",
			profile: AgentProfile{
				Runner: RunnerPi,
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OPENAI_API_KEY",
				},
			},
			wantError: "Pi model must use provider/model form",
		},
		{
			name: "Pi omitted provider with unsupported model prefix",
			profile: AgentProfile{
				Runner: RunnerPi,
				Model:  "other/model",
				Authentication: Authentication{
					Source:              AuthAPIKey,
					EnvironmentVariable: "OTHER_API_KEY",
				},
			},
			wantError: `unsupported Pi provider "other"`,
		},
		{
			name: "Pi OpenRouter requires explicit API key",
			profile: AgentProfile{
				Runner: RunnerPi,
				Model:  "openrouter/openai/gpt-5.6-sol",
			},
			wantError: `requires explicit api_key authentication`,
		},
		{
			name: "Claude OpenRouter subscription",
			profile: AgentProfile{
				Runner:   RunnerClaude,
				Provider: ProviderOpenRouter,
				Model:    "openai/gpt-5.6-sol",
			},
			wantError: `requires explicit api_key authentication`,
		},
		{
			name: "OpenClaw Anthropic subscription",
			profile: AgentProfile{
				Runner:   RunnerOpenClaw,
				Provider: ProviderAnthropic,
				Model:    "anthropic/claude-opus-4-8",
			},
			wantError: `requires explicit api_key authentication`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := test.profile.resolve(t.TempDir(), t.TempDir())
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("resolve() error = %v, want substring %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if resolved.Provider != test.wantProvider {
				t.Fatalf("provider = %q, want %q", resolved.Provider, test.wantProvider)
			}
		})
	}
}

func TestAgentProfileCommandResolution(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	settingsDir := filepath.Join(root, "settings")
	if err := os.Mkdir(settingsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(settingsDir, "settings.json")
	data := `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "agent_profiles":{
    "codex":{"runner":"codex","command":"../bin/codex"},
    "claude":{"runner":"claude","command":"~/bin/claude"}
  },
  "procedures":{"simple":{"model":"openai://model"}}
}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadSettings(path, ProcedureSimple)
	if err != nil {
		t.Fatal(err)
	}
	if got := settings.AgentProfiles["codex"].Command; got != filepath.Join(root, "bin", "codex") {
		t.Fatalf("Codex command = %q", got)
	}
	if got := settings.AgentProfiles["claude"].Command; got != filepath.Join(home, "bin", "claude") {
		t.Fatalf("Claude command = %q", got)
	}
}

func TestSettingsHomeDirectoryRequirement(t *testing.T) {
	simple := SettingsFile{Procedures: ProcedureSettings{Simple: &SimpleSettings{Model: "openai://model"}}}
	if simple.needsHomeDirectory() {
		t.Fatal("simple settings without home-relative paths require a home directory")
	}
	subscription := simple
	subscription.AgentProfiles = map[string]AgentProfile{"lawyer": {Runner: RunnerCodex}}
	if !subscription.needsHomeDirectory() {
		t.Fatal("default Codex subscription path does not require a home directory")
	}
	apiKey := simple
	apiKey.AgentProfiles = map[string]AgentProfile{"lawyer": {
		Runner: RunnerCodex,
		Authentication: Authentication{
			Source:              AuthAPIKey,
			EnvironmentVariable: "CODEX_KEY",
		},
	}}
	if apiKey.needsHomeDirectory() {
		t.Fatal("API-key profile without home-relative paths requires a home directory")
	}
	simple.Common.OutputRoot = "~/records"
	if !simple.needsHomeDirectory() {
		t.Fatal("home-relative output root does not require a home directory")
	}
}

func TestSimpleModelQueryIsRejectedWithoutEchoingIt(t *testing.T) {
	secret := "secret-value"
	path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "procedures":{"simple":{"model":"openai://model?token=`+secret+`"}}
}`)
	_, err := LoadSettings(path, ProcedureSimple)
	if err == nil || !strings.Contains(err.Error(), "must not include a query") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error contains model query: %v", err)
	}
}

func TestAPIKeyAuthenticationRequiresExplicitSource(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "profile environment without source",
			data: `{"runner":"codex","authentication":{"environment_variable":"OPENAI_API_KEY"}}`,
			want: "subscription authentication cannot name an environment variable",
		},
		{
			name: "profile source without environment",
			data: `{"runner":"codex","authentication":{"source":"api_key"}}`,
			want: "requires environment_variable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "agent_profiles":{"agent":`+test.data+`},
  "procedures":{"simple":{"model":"openai://model"}}
}`)
			_, err := LoadSettings(path, ProcedureSimple)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadSettings() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestCommonProviderCredentialsRequireExplicitPermissionAndMetadata(t *testing.T) {
	tests := []struct {
		name      string
		common    string
		procedure string
		wantError string
	}{
		{
			name:      "permission omitted",
			common:    `{"evidence_standard":"preponderance_of_the_evidence","provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}}`,
			procedure: `{"model":"openai://model"}`,
			wantError: "common allow_api_key is required",
		},
		{
			name:      "permission denied",
			common:    `{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":false,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}}`,
			procedure: `{"model":"openai://model"}`,
			wantError: "current adjudication procedures require common allow_api_key",
		},
		{
			name:      "credential map empty",
			common:    `{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}}`,
			procedure: `{"model":"openai://model"}`,
			wantError: "provider_credentials is empty",
		},
		{
			name:      "subscription source",
			common:    `{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"subscription"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}}`,
			procedure: `{"model":"openai://model"}`,
			wantError: "must use api_key",
		},
		{
			name:      "invalid environment name",
			common:    `{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI-KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}}`,
			procedure: `{"model":"openai://model"}`,
			wantError: "invalid environment_variable",
		},
		{
			name:      "unsupported provider",
			common:    `{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"anthropic":{"source":"api_key","environment_variable":"ANTHROPIC_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}}`,
			procedure: `{"model":"openai://model"}`,
			wantError: "unsupported provider",
		},
		{
			name:      "selected provider absent",
			common:    `{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}}`,
			procedure: `{"model":"openai://model"}`,
			wantError: `procedure simple requires common provider credential "openai"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := writeTestFile(t, "settings.json", `{"schema_version":"adjudicate.settings.v1","common":`+test.common+`,"procedures":{"simple":`+test.procedure+`}}`)
			_, err := LoadSettings(path, ProcedureSimple)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("LoadSettings() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestResolvedSettingsRecordCredentialMetadataWithoutValue(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secret-value")
	path := writeTestFile(t, "settings.json", `{
  "schema_version":"adjudicate.settings.v1",
  "common":{"evidence_standard":"preponderance_of_the_evidence","allow_api_key":true,"provider_credentials":{"openai":{"source":"api_key","environment_variable":"OPENAI_API_KEY"},"openrouter":{"source":"api_key","environment_variable":"OPENROUTER_API_KEY"}},"document_limits":{"count":1,"per_file_bytes":1,"total_bytes":1}},
  "procedures":{"simple":{"model":"openai://model"}}
}`)
	settings, err := LoadSettings(path, ProcedureSimple)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-value") {
		t.Fatalf("resolved settings contain credential value: %s", data)
	}
	if !strings.Contains(string(data), "OPENAI_API_KEY") {
		t.Fatalf("resolved settings omit credential source metadata: %s", data)
	}
}

func TestDocumentLimitsAreRequired(t *testing.T) {
	path := writeTestFile(t, "settings.json", `{"schema_version":"adjudicate.settings.v1","common":{},"procedures":{"simple":{"model":"openai://model"}}}`)
	_, err := LoadSettings(path, ProcedureSimple)
	if err == nil || !strings.Contains(err.Error(), "document limits must be positive") {
		t.Fatalf("LoadSettings() error = %v", err)
	}
}

func writeTestFile(t *testing.T, name, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
