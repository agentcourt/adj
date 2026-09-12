package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/agentcourt/adj/common/cliio"
	"github.com/agentcourt/adj/runtime/localrun/adc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runLocal(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if _, writeErr := fmt.Fprintf(os.Stderr, "error: %v\n", err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func runLocal(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("adc-run", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	var promptFiles cliio.Assignments
	var launcherPromptFiles cliio.Assignments
	var councilEndpoints cliio.StringList
	coreCommand := fs.String("adc-bin", localrun.DefaultCoreCommand, "Core adc executable")
	coreWorkingDir := fs.String("adc-working-dir", "", "Optional working directory for the core adc process")
	mcpCommand := fs.String("mcp-bin", localrun.DefaultMCPCommand, "ADC MCP executable")
	mcpWorkingDir := fs.String("mcp-working-dir", "", "Optional working directory for the ADC MCP process; defaults to --adc-working-dir")
	complaintPath := fs.String("complaint", "", "Complaint markdown path")
	scenarioPath := fs.String("scenario", "", "Scenario JSON path")
	courtRef := fs.String("court", localrun.DefaultCourt, "Court profile name or JSON path")
	outDir := fs.String("out-dir", "", "Launcher run output directory")
	model := fs.String("model", "", "Runtime model")
	promptDir := fs.String("prompt-dir", "", "Complete ADC prompt directory")
	fs.Var(&promptFiles, "prompt-file", "Prompt override as ID=PATH; repeat for a partial set")
	launcherPromptDir := fs.String("launcher-prompt-dir", "", "Complete ADC launcher prompt directory")
	fs.Var(&launcherPromptFiles, "launcher-prompt-file", "Launcher prompt override as ID=PATH; repeat for a partial set")
	nonJurorModel := fs.String("non-juror-model", localrun.DefaultNonJurorModel, "Runtime model for judge, lawyers, and clerk during complaint preparation")
	plaintiffModel := fs.String("plaintiff-model", "", "Runtime model for plaintiff counsel during complaint preparation")
	defendantModel := fs.String("defendant-model", "", "Runtime model for defense counsel during complaint preparation")
	judgeModel := fs.String("judge-model", "", "Runtime model for the judge during complaint preparation")
	clerkModel := fs.String("clerk-model", "", "Runtime model for the clerk during complaint preparation")
	plannerModel := fs.String("planner-model", localrun.DefaultPlannerModel, "Model for neutral intake and strategy planning")
	reportModel := fs.String("report-model", localrun.DefaultReportModel, "Model for digest generation")
	temperature := fs.String("temperature", "", "Override runtime temperature")
	nonJurorTemperature := fs.String("non-juror-temperature", "", "Override non-juror complaint-preparation temperature")
	jurorTemperature := fs.String("juror-temperature", "", "Override runtime temperature for direct jurors")
	jurorPersonas := fs.String("juror-personas", "", "Explicit juror JSONL request-spec pool passed to the core case")
	fs.Var(&councilEndpoints, "council-endpoint", "Allowed juror endpoint; repeat as needed")
	minimumDistinctCouncilEndpoints := fs.Int("minimum-distinct-council-endpoints", 0, "Minimum distinct endpoints assigned across juror candidates")
	trialMode := fs.String("trial-mode", "auto", "Trial mode for complaint preparation: auto, jury, or bench")
	skipVoirDire := fs.Bool("skip-voir-dire", false, "Skip questionnaires and voir dire during complaint preparation")
	jurorCount := fs.Int("juror-count", 0, "Jury size, 6 through 12")
	minimumConcurring := fs.Int("minimum-concurring", 0, "Minimum concurring jurors, 6 through 12")
	unanimousRequired := fs.String("unanimous-required", "", "Whether the jury verdict must be unanimous: true or false")
	online := fs.Bool("online", false, "Enable web search for internal direct model calls")
	offline := fs.Bool("offline", false, "Disable internal LLM calls for a prepared scenario")
	caseAPIAddr := fs.String("caseapi-addr", "127.0.0.1:0", "Private case API listen address")
	mcpListenAddr := fs.String("mcp-listen", "0.0.0.0:0", "MCP listen address")
	jurorTimeoutSeconds := fs.Int("juror-timeout-seconds", localrun.DefaultRunJurorTimeoutSeconds, "Juror opportunity timeout seconds")
	lawyerTimeoutSeconds := fs.Int("lawyer-timeout-seconds", localrun.DefaultRunLawyerTimeoutSeconds, "Lawyer opportunity timeout seconds")
	timeoutSeconds := fs.Int("timeout-seconds", localrun.DefaultLLMTimeoutSeconds, "Internal LLM HTTP timeout seconds")
	maxResponseBytes := fs.Int("max-response-bytes", localrun.DefaultMaxResponseBytes, "Maximum bytes allowed in one direct-runtime model response")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", localrun.DefaultInvalidAttemptLimit, "Maximum invalid submissions before an opportunity fails")
	enginePath := fs.String("engine", "", "Optional Lean engine command string passed to the core case")
	runID := fs.String("run-id", "", "Run ID override")
	caseID := fs.String("case-id", "", "Case ID")
	autoLawyers := fs.String("auto-lawyers", localrun.DefaultAutoLawyers, "Lawyers started by adc-run: both, plaintiff, defendant, or none")
	lawyerWebSearch := fs.Bool("lawyer-web-search", true, "Allow lawyer web search")
	mcpPublicBaseURL := fs.String("mcp-public-base-url", "", "Externally reachable MCP base URL written to manual-lawyer skills. Required for a wildcard --mcp-listen address")
	dockerCommand := fs.String("docker", localrun.DefaultDockerCommand, "Docker command")
	podmanCommand := fs.String("podman", localrun.DefaultPodmanCommand, "Podman command")
	openClawImage := fs.String("openclaw-image", "", "OpenClaw container image")
	openClawModel := fs.String("openclaw-model", "", "OpenClaw model")
	openClawThinking := fs.String("openclaw-thinking", "", "OpenClaw thinking setting")
	openClawTimeoutSeconds := fs.Int("openclaw-timeout-seconds", 0, "OpenClaw agent timeout seconds")
	openClawAuth := fs.String("openclaw-auth", "", "OpenClaw auth mode: codex or api-key")
	openClawCodexAuth := fs.String("openclaw-codex-auth", "", "Codex auth.json path for OpenClaw")
	openClawStartDelaySeconds := fs.Int("openclaw-lawyer-start-delay-seconds", -1, "Delay between plaintiff and defendant OpenClaw startup; 0 disables")
	openClawNetwork := fs.String("openclaw-network", "", "Docker network for OpenClaw lawyer containers: host or empty")
	piImage := fs.String("pi-image", "", "Pi container image")
	piMCPAdapter := fs.String("pi-mcp-adapter", "", "Pi MCP adapter path or package source")
	jurorOutputLimitBytes := fs.Int64("juror-output-limit-bytes", localrun.DefaultJurorOutputLimitBytes, "Total stdout plus stderr byte limit per Pi juror agent")
	dockerMCPHost := fs.String("docker-mcp-host", "", "Host name used by Docker containers to reach MCP")
	podmanMCPHost := fs.String("podman-mcp-host", "", "Host name used by Podman containers to reach MCP")
	fs.Usage = func() {
		fmt.Fprintf(flagOutput, "Usage: adc-run (--complaint FILE | --scenario FILE) [options]\n\n")
		fs.PrintDefaults()
	}
	help, err := cliio.Parse(fs, args, flagOutput)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc-run accepts no positional arguments")
	}
	hasComplaint := strings.TrimSpace(*complaintPath) != ""
	hasScenario := strings.TrimSpace(*scenarioPath) != ""
	if hasComplaint == hasScenario {
		return fmt.Errorf("exactly one of --complaint or --scenario is required")
	}
	if hasComplaint && *offline {
		return fmt.Errorf("--offline cannot prepare a complaint-based run")
	}
	if err := validateNumericFlags(*temperature, *nonJurorTemperature, *jurorTemperature, *unanimousRequired, *jurorCount, *minimumConcurring); err != nil {
		return err
	}
	now := time.Now().UTC().Format("20060102150405")
	if strings.TrimSpace(*caseID) == "" {
		*caseID = "adc-" + now
	}
	if strings.TrimSpace(*runID) == "" {
		*runID = "run-" + strings.TrimSpace(*caseID)
	}
	if strings.TrimSpace(*outDir) == "" {
		*outDir = filepath.Join("out", strings.TrimSpace(*caseID))
	}
	result, err := localrun.Run(ctx, localrun.Options{
		CoreCommand:               strings.TrimSpace(*coreCommand),
		CoreWorkingDir:            strings.TrimSpace(*coreWorkingDir),
		MCPCommand:                strings.TrimSpace(*mcpCommand),
		MCPWorkingDir:             strings.TrimSpace(*mcpWorkingDir),
		ComplaintPath:             strings.TrimSpace(*complaintPath),
		ScenarioPath:              strings.TrimSpace(*scenarioPath),
		OutputDir:                 strings.TrimSpace(*outDir),
		Court:                     strings.TrimSpace(*courtRef),
		Model:                     strings.TrimSpace(*model),
		PromptDir:                 strings.TrimSpace(*promptDir),
		PromptFiles:               promptFiles.Map(),
		LauncherPromptDir:         strings.TrimSpace(*launcherPromptDir),
		LauncherPromptFiles:       launcherPromptFiles.Map(),
		DigestModel:               strings.TrimSpace(*reportModel),
		NonJurorModel:             strings.TrimSpace(*nonJurorModel),
		PlaintiffModel:            strings.TrimSpace(*plaintiffModel),
		DefendantModel:            strings.TrimSpace(*defendantModel),
		JudgeModel:                strings.TrimSpace(*judgeModel),
		ClerkModel:                strings.TrimSpace(*clerkModel),
		PlannerModel:              strings.TrimSpace(*plannerModel),
		Temperature:               strings.TrimSpace(*temperature),
		NonJurorTemperature:       strings.TrimSpace(*nonJurorTemperature),
		JurorTemperature:          strings.TrimSpace(*jurorTemperature),
		JurorPersonasPath:         strings.TrimSpace(*jurorPersonas),
		CouncilAllowedEndpoints:   councilEndpoints.Values(),
		CouncilMinEndpoints:       *minimumDistinctCouncilEndpoints,
		TrialMode:                 strings.TrimSpace(*trialMode),
		SkipVoirDire:              *skipVoirDire,
		JurorCount:                *jurorCount,
		MinimumConcurring:         *minimumConcurring,
		UnanimousRequired:         strings.TrimSpace(*unanimousRequired),
		Online:                    *online,
		Offline:                   *offline,
		CaseAPIAddr:               strings.TrimSpace(*caseAPIAddr),
		MCPListenAddr:             strings.TrimSpace(*mcpListenAddr),
		JurorTimeoutSeconds:       *jurorTimeoutSeconds,
		LawyerTimeoutSeconds:      *lawyerTimeoutSeconds,
		TimeoutSeconds:            *timeoutSeconds,
		MaxResponseBytes:          *maxResponseBytes,
		InvalidAttemptLimit:       *invalidAttemptLimit,
		EnginePath:                strings.TrimSpace(*enginePath),
		RunID:                     strings.TrimSpace(*runID),
		CaseID:                    strings.TrimSpace(*caseID),
		AutoLawyers:               strings.TrimSpace(*autoLawyers),
		LawyerWebSearch:           lawyerWebSearch,
		MCPPublicBaseURL:          strings.TrimSpace(*mcpPublicBaseURL),
		DockerCommand:             strings.TrimSpace(*dockerCommand),
		PodmanCommand:             strings.TrimSpace(*podmanCommand),
		OpenClawImage:             strings.TrimSpace(*openClawImage),
		OpenClawModel:             strings.TrimSpace(*openClawModel),
		OpenClawThinking:          strings.TrimSpace(*openClawThinking),
		OpenClawTimeoutSeconds:    *openClawTimeoutSeconds,
		OpenClawAuth:              strings.TrimSpace(*openClawAuth),
		OpenClawCodexAuthPath:     strings.TrimSpace(*openClawCodexAuth),
		OpenClawStartDelaySeconds: *openClawStartDelaySeconds,
		OpenClawNetwork:           strings.TrimSpace(*openClawNetwork),
		PiImage:                   strings.TrimSpace(*piImage),
		PiMCPAdapter:              strings.TrimSpace(*piMCPAdapter),
		JurorOutputLimitBytes:     *jurorOutputLimitBytes,
		DockerMCPHost:             strings.TrimSpace(*dockerMCPHost),
		PodmanMCPHost:             strings.TrimSpace(*podmanMCPHost),
		Log:                       stderr,
	})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal run result: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, string(raw)); err != nil {
		return fmt.Errorf("write run result: %w", err)
	}
	return nil
}

func validateNumericFlags(temperature string, nonJurorTemperature string, jurorTemperature string, unanimousRequired string, jurorCount int, minimumConcurring int) error {
	for name, raw := range map[string]string{
		"--temperature":           temperature,
		"--non-juror-temperature": nonJurorTemperature,
		"--juror-temperature":     jurorTemperature,
	} {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if _, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
	}
	if strings.TrimSpace(unanimousRequired) != "" {
		if _, err := strconv.ParseBool(strings.TrimSpace(unanimousRequired)); err != nil {
			return fmt.Errorf("parse --unanimous-required: %w", err)
		}
	}
	if jurorCount < 0 || jurorCount > 0 && (jurorCount < 6 || jurorCount > 12) {
		return fmt.Errorf("--juror-count must be zero or between 6 and 12")
	}
	if minimumConcurring < 0 || minimumConcurring > 0 && (minimumConcurring < 6 || minimumConcurring > 12) {
		return fmt.Errorf("--minimum-concurring must be zero or between 6 and 12")
	}
	if jurorCount > 0 && minimumConcurring > jurorCount {
		return fmt.Errorf("--minimum-concurring cannot exceed --juror-count")
	}
	return nil
}
