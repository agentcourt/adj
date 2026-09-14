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
	"github.com/agentcourt/adj/internal/arbexample"
	headless "github.com/agentcourt/adj/runtime/agent"
	"github.com/agentcourt/adj/runtime/localrun/arb"
)

type explicitFileList struct {
	values []string
}

type optionalBoolFlag struct {
	set   bool
	value bool
}

func (f *optionalBoolFlag) String() string {
	if f == nil || !f.set {
		return ""
	}
	return strconv.FormatBool(f.value)
}

func (f *optionalBoolFlag) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	f.set = true
	f.value = parsed
	return nil
}

func (f *optionalBoolFlag) IsBoolFlag() bool { return true }

func (f *optionalBoolFlag) Pointer() *bool {
	if f == nil || !f.set {
		return nil
	}
	value := f.value
	return &value
}

func (f *explicitFileList) String() string {
	return strings.Join(f.values, ",")
}

func (f *explicitFileList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("--file must not be empty")
	}
	f.values = append(f.values, value)
	return nil
}

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
	fs := flag.NewFlagSet("aar-run", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	var caseFiles explicitFileList
	var promptFiles cliio.Assignments
	var launcherPromptFiles cliio.Assignments
	var councilEndpoints cliio.StringList
	var plaintiffLawyerResume optionalBoolFlag
	var defendantLawyerResume optionalBoolFlag
	coreCommand := fs.String("aar-bin", localrun.DefaultCoreCommand, "Core aar executable")
	coreWorkingDir := fs.String("aar-working-dir", "", "Optional working directory for the core aar process")
	mcpCommand := fs.String("mcp-bin", localrun.DefaultMCPCommand, "ARB MCP executable")
	mcpWorkingDir := fs.String("mcp-working-dir", "", "Optional working directory for the ARB MCP process; defaults to --aar-working-dir")
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	fs.Var(&caseFiles, "file", "Explicit case file path or glob. May be repeated")
	outDir := fs.String("out-dir", "", "Launcher run output directory")
	policyPath := fs.String("policy", "", "Policy JSON file")
	councilSize := fs.Int("council-size", 0, "Override policy council_size")
	requiredVotes := fs.Int("required-votes", 0, "Override policy required_votes_for_decision")
	evidenceStandard := fs.String("evidence-standard", "", "Override policy evidence_standard")
	promptDir := fs.String("prompt-dir", "", "Complete ARB prompt directory")
	fs.Var(&promptFiles, "prompt-file", "Prompt override as ID=PATH; repeat for a partial set")
	launcherPromptDir := fs.String("launcher-prompt-dir", "", "Complete ARB launcher prompt directory")
	fs.Var(&launcherPromptFiles, "launcher-prompt-file", "Launcher prompt override as ID=PATH; repeat for a partial set")
	commonRoot := fs.String("common-root", "", "Optional common directory passed to the core case")
	councilPool := fs.String("council-pool", "", "Council JSONL request-spec pool file")
	fs.Var(&councilEndpoints, "council-endpoint", "Allowed council endpoint; repeat as needed")
	minimumDistinctCouncilEndpoints := fs.Int("minimum-distinct-council-endpoints", 0, "Minimum distinct endpoints represented in the council")
	caseAPIAddr := fs.String("caseapi-addr", "127.0.0.1:0", "Private case API listen address")
	mcpListenAddr := fs.String("mcp-listen", "0.0.0.0:0", "MCP listen address")
	councilTimeoutSeconds := fs.Int("council-timeout-seconds", localrun.DefaultRunCouncilTimeoutSeconds, "Council turn timeout seconds")
	lawyerTimeoutSeconds := fs.Int("lawyer-timeout-seconds", localrun.DefaultRunLawyerTimeoutSeconds, "Lawyer turn timeout seconds")
	maxResponseBytes := fs.Int("max-response-bytes", 0, "Override runtime max parsed response bytes")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", 0, "Override runtime invalid-attempt limit")
	enginePath := fs.String("engine", "", "Optional Lean engine binary passed to the core case")
	runID := fs.String("run-id", "", "Run ID override")
	caseID := fs.String("case-id", "", "Case ID")
	autoLawyers := fs.String("auto-lawyers", localrun.DefaultAutoLawyers, "Lawyers started by aar-run: both, plaintiff, defendant, or none")
	lawyerWebSearch := fs.Bool("lawyer-web-search", true, "Allow lawyer web search")
	plaintiffLawyer := fs.String("plaintiff-lawyer", string(localrun.LawyerOpenClaw), "Plaintiff lawyer runner: openclaw, pi, codex, or claude")
	plaintiffLawyerProvider := fs.String("plaintiff-lawyer-provider", "", "Plaintiff provider")
	plaintiffLawyerCommand := fs.String("plaintiff-lawyer-command", "", "Plaintiff Codex or Claude command")
	plaintiffLawyerModel := fs.String("plaintiff-lawyer-model", "", "Plaintiff model; Pi requires a provider-qualified model")
	plaintiffLawyerReasoning := fs.String("plaintiff-lawyer-reasoning-effort", "", "Plaintiff reasoning effort")
	plaintiffLawyerAuth := fs.String("plaintiff-lawyer-auth", "", "Plaintiff authentication: subscription or api-key")
	plaintiffLawyerCredentials := fs.String("plaintiff-lawyer-credentials", "", "Plaintiff subscription credential file")
	plaintiffLawyerAPIKeyEnv := fs.String("plaintiff-lawyer-api-key-env", "", "Plaintiff API-key source environment variable")
	fs.Var(&plaintiffLawyerResume, "plaintiff-lawyer-resume", "Resume the plaintiff session after a successful invocation")
	defendantLawyer := fs.String("defendant-lawyer", string(localrun.LawyerOpenClaw), "Defendant lawyer runner: openclaw, pi, codex, or claude")
	defendantLawyerProvider := fs.String("defendant-lawyer-provider", "", "Defendant provider")
	defendantLawyerCommand := fs.String("defendant-lawyer-command", "", "Defendant Codex or Claude command")
	defendantLawyerModel := fs.String("defendant-lawyer-model", "", "Defendant model; Pi requires a provider-qualified model")
	defendantLawyerReasoning := fs.String("defendant-lawyer-reasoning-effort", "", "Defendant reasoning effort")
	defendantLawyerAuth := fs.String("defendant-lawyer-auth", "", "Defendant authentication: subscription or api-key")
	defendantLawyerCredentials := fs.String("defendant-lawyer-credentials", "", "Defendant subscription credential file")
	defendantLawyerAPIKeyEnv := fs.String("defendant-lawyer-api-key-env", "", "Defendant API-key source environment variable")
	fs.Var(&defendantLawyerResume, "defendant-lawyer-resume", "Resume the defendant session after a successful invocation")
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
	councilOutputLimitBytes := fs.Int64("council-output-limit-bytes", localrun.DefaultCouncilOutputLimitBytes, "Total stdout plus stderr byte limit per Pi council agent")
	dockerMCPHost := fs.String("docker-mcp-host", "", "Host name used by Docker containers to reach MCP")
	podmanMCPHost := fs.String("podman-mcp-host", "", "Host name used by Podman containers to reach MCP")
	fs.Usage = func() {
		fmt.Fprintf(flagOutput, "Usage: aar-run [EXAMPLE] [options]\n\n")
		fs.PrintDefaults()
	}
	help, err := cliio.Parse(fs, args, flagOutput)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	example := ""
	if fs.NArg() > 1 {
		return fmt.Errorf("aar-run accepts at most one example name")
	}
	if fs.NArg() == 1 {
		example = strings.TrimSpace(fs.Arg(0))
		if example == "" || strings.Contains(example, "/") || strings.HasPrefix(example, ".") || strings.Contains(example, "..") {
			return fmt.Errorf("invalid example name: %s", example)
		}
	}
	now := time.Now().UTC().Format("20060102150405")
	if example != "" {
		if strings.TrimSpace(*complaintPath) == "" {
			*complaintPath, err = arbexample.ComplaintPath(*coreWorkingDir, example)
			if err != nil {
				return err
			}
		}
		if strings.TrimSpace(*caseID) == "" {
			*caseID = "arb-" + example + "-" + now
		}
		if strings.TrimSpace(*outDir) == "" {
			*outDir = filepath.Join("out", example+"-openclaw-pi-"+now)
		}
	} else if strings.TrimSpace(*caseID) == "" {
		*caseID = "arb-" + now
	}
	if strings.TrimSpace(*runID) == "" {
		*runID = "run-" + strings.TrimSpace(*caseID)
	}
	if strings.TrimSpace(*outDir) == "" {
		*outDir = filepath.Join("out", strings.TrimSpace(*caseID))
	}
	opts := localrun.Options{
		CoreCommand:             strings.TrimSpace(*coreCommand),
		CoreWorkingDir:          strings.TrimSpace(*coreWorkingDir),
		MCPCommand:              strings.TrimSpace(*mcpCommand),
		MCPWorkingDir:           strings.TrimSpace(*mcpWorkingDir),
		ComplaintPath:           *complaintPath,
		CaseFiles:               caseFiles.values,
		OutputDir:               *outDir,
		PolicyPath:              *policyPath,
		CouncilSize:             *councilSize,
		RequiredVotes:           *requiredVotes,
		EvidenceStandard:        *evidenceStandard,
		PromptDir:               strings.TrimSpace(*promptDir),
		PromptFiles:             promptFiles.Map(),
		LauncherPromptDir:       strings.TrimSpace(*launcherPromptDir),
		LauncherPromptFiles:     launcherPromptFiles.Map(),
		CommonRoot:              *commonRoot,
		CouncilPoolPath:         *councilPool,
		CouncilAllowedEndpoints: councilEndpoints.Values(),
		CouncilMinEndpoints:     *minimumDistinctCouncilEndpoints,
		CaseAPIAddr:             *caseAPIAddr,
		MCPListenAddr:           *mcpListenAddr,
		CouncilTimeoutSeconds:   *councilTimeoutSeconds,
		LawyerTimeoutSeconds:    *lawyerTimeoutSeconds,
		MaxResponseBytes:        *maxResponseBytes,
		InvalidAttemptLimit:     *invalidAttemptLimit,
		EnginePath:              *enginePath,
		RunID:                   *runID,
		CaseID:                  *caseID,
		AutoLawyers:             *autoLawyers,
		LawyerWebSearch:         lawyerWebSearch,
		PlaintiffLawyer: localrun.LawyerProfile{
			Runner:          localrun.LawyerRunner(strings.TrimSpace(*plaintiffLawyer)),
			Provider:        headless.Provider(strings.TrimSpace(*plaintiffLawyerProvider)),
			Command:         strings.TrimSpace(*plaintiffLawyerCommand),
			Model:           strings.TrimSpace(*plaintiffLawyerModel),
			ReasoningEffort: strings.TrimSpace(*plaintiffLawyerReasoning),
			AuthMode:        agentAuthMode(*plaintiffLawyerAuth),
			CredentialsFile: strings.TrimSpace(*plaintiffLawyerCredentials),
			APIKeyEnv:       strings.TrimSpace(*plaintiffLawyerAPIKeyEnv),
			Resume:          plaintiffLawyerResume.Pointer(),
		},
		DefendantLawyer: localrun.LawyerProfile{
			Runner:          localrun.LawyerRunner(strings.TrimSpace(*defendantLawyer)),
			Provider:        headless.Provider(strings.TrimSpace(*defendantLawyerProvider)),
			Command:         strings.TrimSpace(*defendantLawyerCommand),
			Model:           strings.TrimSpace(*defendantLawyerModel),
			ReasoningEffort: strings.TrimSpace(*defendantLawyerReasoning),
			AuthMode:        agentAuthMode(*defendantLawyerAuth),
			CredentialsFile: strings.TrimSpace(*defendantLawyerCredentials),
			APIKeyEnv:       strings.TrimSpace(*defendantLawyerAPIKeyEnv),
			Resume:          defendantLawyerResume.Pointer(),
		},
		MCPPublicBaseURL:          *mcpPublicBaseURL,
		DockerCommand:             *dockerCommand,
		PodmanCommand:             *podmanCommand,
		OpenClawImage:             *openClawImage,
		OpenClawModel:             *openClawModel,
		OpenClawThinking:          *openClawThinking,
		OpenClawTimeoutSeconds:    *openClawTimeoutSeconds,
		OpenClawAuth:              *openClawAuth,
		OpenClawCodexAuthPath:     *openClawCodexAuth,
		OpenClawStartDelaySeconds: *openClawStartDelaySeconds,
		OpenClawNetwork:           *openClawNetwork,
		PiImage:                   *piImage,
		PiMCPAdapter:              *piMCPAdapter,
		CouncilOutputLimitBytes:   *councilOutputLimitBytes,
		DockerMCPHost:             *dockerMCPHost,
		PodmanMCPHost:             *podmanMCPHost,
		Log:                       stderr,
	}
	result, err := localrun.Run(ctx, opts)
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

func agentAuthMode(value string) headless.AuthMode {
	value = strings.TrimSpace(value)
	if value == "api-key" {
		return headless.AuthAPIKey
	}
	return headless.AuthMode(value)
}
