package localrun

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/agentcourt/adj/internal/launcherprompt"
	lawyerlaunch "github.com/agentcourt/adj/internal/lawyer"
	"github.com/agentcourt/adj/internal/mcpcap"
	"github.com/agentcourt/adj/internal/mcpchild"
	"github.com/agentcourt/adj/internal/pimodel"
	headless "github.com/agentcourt/adj/runtime/agent"
	"github.com/agentcourt/adj/runtime/corehealth"
	"github.com/agentcourt/adj/runtime/modelrequest"
	"github.com/agentcourt/adj/runtime/runstate"
)

const (
	defaultOpenClawImage          = "ghcr.io/openclaw/openclaw:latest"
	defaultOpenClawModel          = "gpt-5.5"
	defaultOpenClawThinking       = "low"
	defaultOpenClawTimeoutSeconds = 3600
	defaultOpenClawAuth           = "codex"
	defaultOpenClawStartDelay     = 15
	defaultPiImage                = "agentcourt-pi-sandbox"
	defaultPiMCPAdapter           = "/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter"
	defaultPiWebSearch            = "/opt/pi-extensions/pi-web-access/node_modules/pi-web-access/index.ts"
	defaultPiMCPServer            = "aard"
	defaultCaseAPIStartupWait     = 10 * time.Minute
	defaultMCPStartupWait         = 30 * time.Second
	defaultCouncilRosterWait      = 2 * time.Minute
	defaultCouncilOutputCheck     = 5 * time.Second
	defaultAgentErrorCaseWait     = 2 * time.Second
	agentStopWait                 = 30 * time.Second
	councilFailureAgentExited     = "agent_exited"
	councilFailureOutputLimit     = "agent_output_limit_exceeded"
)

type LawyerRunner string

const (
	LawyerOpenClaw LawyerRunner = "openclaw"
	LawyerPi       LawyerRunner = "pi"
	LawyerCodex    LawyerRunner = "codex"
	LawyerClaude   LawyerRunner = "claude"
)

type LawyerProfile struct {
	Name            string
	Runner          LawyerRunner
	Provider        headless.Provider
	Command         string
	Model           string
	ReasoningEffort string
	AuthMode        headless.AuthMode
	CredentialsFile string
	APIKeyEnv       string
	Resume          *bool
	Environment     []string
	StateDir        string
	WorkDir         string
}

const (
	DefaultRunCouncilTimeoutSeconds = 15 * 60
	DefaultRunLawyerTimeoutSeconds  = DefaultRunCouncilTimeoutSeconds
	DefaultCouncilOutputLimitBytes  = 128 * 1024 * 1024
	DefaultCouncilMaxOutputTokens   = 4096
	DefaultAutoLawyers              = "both"
	DefaultDockerCommand            = "docker"
	DefaultPodmanCommand            = "podman"
	DefaultCoreCommand              = "aard"
	DefaultMCPCommand               = "aard-mcp"
)

type Options struct {
	CoreCommand               string
	CoreWorkingDir            string
	MCPCommand                string
	MCPWorkingDir             string
	ComplaintPath             string
	CaseFiles                 []string
	OutputDir                 string
	CoreOutputDir             string
	LogsDir                   string
	PolicyPath                string
	CouncilSize               int
	JudgmentStandard          string
	PromptDir                 string
	PromptFiles               map[string]string
	CommonRoot                string
	CouncilPoolPath           string
	CaseAPIAddr               string
	MCPListenAddr             string
	CouncilTimeoutSeconds     int
	LawyerTimeoutSeconds      int
	MaxResponseBytes          int
	InvalidAttemptLimit       int
	EnginePath                string
	RunID                     string
	CaseID                    string
	LauncherPromptDir         string
	LauncherPromptFiles       map[string]string
	AutoLawyers               string
	PlaintiffLawyer           LawyerProfile
	DefendantLawyer           LawyerProfile
	MCPPublicBaseURL          string
	DockerCommand             string
	PodmanCommand             string
	OpenClawImage             string
	OpenClawModel             string
	OpenClawThinking          string
	OpenClawTimeoutSeconds    int
	OpenClawAuth              string
	OpenClawCodexAuthPath     string
	OpenClawStartDelaySeconds int
	OpenClawNetwork           string
	LawyerWebSearch           *bool
	PiImage                   string
	PiMCPAdapter              string
	CouncilOutputLimitBytes   int64
	DockerMCPHost             string
	PodmanMCPHost             string
	CoreEnvironment           []string
	MCPEnvironment            []string
	ParticipantEnvironment    []string
	Log                       io.Writer
	ProcessObserver           runstate.Observer
}

type Result struct {
	CaseID  string         `json:"case_id"`
	RunID   string         `json:"run_id"`
	Status  string         `json:"status"`
	Phase   string         `json:"phase"`
	Answers map[string]int `json:"answers"`
	Error   string         `json:"error,omitempty"`
	Failure map[string]any `json:"failure,omitempty"`

	raw json.RawMessage
}

func (r Result) MarshalJSON() ([]byte, error) {
	if len(r.raw) > 0 {
		return append([]byte(nil), r.raw...), nil
	}
	type resultAlias Result
	return json.Marshal(resultAlias(r))
}

type instructionData struct {
	CaseID             string
	RoleID             string
	MemberID           string
	MCPServer          string
	MCPURL             string
	MCPJSON            string
	Workspace          string
	SearchInstructions string
}

type processRecord struct {
	name            string
	kind            string
	command         *exec.Cmd
	done            chan processExit
	stopCommand     string
	environment     []string
	containerIDPath string
	containerIDDir  string
	councilTarget   *councilProcessTarget

	stdoutPath string
	stderrPath string
	finished   chan struct{}

	stdoutCounter *processOutputCounter

	mu            sync.Mutex
	exited        bool
	forcedReason  string
	forcedMessage string
	forcedDetails map[string]any
}

type processExit struct {
	waitErr   error
	stdoutErr error
	stderrErr error
	recordErr error
	canceled  bool
}

func (e processExit) err() error {
	return errors.Join(e.waitErr, e.stdoutErr, e.stderrErr, e.recordErr)
}

func (e processExit) finalizationErr() error {
	return errors.Join(e.stdoutErr, e.stderrErr, e.recordErr)
}

type councilProcessTarget struct {
	memberID      string
	opportunityID string
}

type councilRosterResponse struct {
	CouncilRoster []councilRosterEntry `json:"council_roster"`
}

type councilStatusResponse struct {
	Status string             `json:"status"`
	Turn   *councilStatusTurn `json:"turn"`
	Error  any                `json:"error"`
}

type councilStatusTurn struct {
	MemberID      string `json:"member_id"`
	OpportunityID string `json:"opportunity_id"`
}

type lawyerStatusResponse struct {
	Status     string `json:"status"`
	Phase      string `json:"phase"`
	CaseStatus string `json:"case_status"`
}

type processOutputSize struct {
	Stdout int64
	Stderr int64
	Total  int64
}

type processOutputCounter struct {
	dst   io.Writer
	count atomic.Int64
}

func newProcessOutputCounter(dst io.Writer) *processOutputCounter {
	return &processOutputCounter{dst: dst}
}

func (w *processOutputCounter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if n > 0 {
		w.count.Add(int64(n))
	}
	return n, err
}

func (w *processOutputCounter) Size() int64 {
	return w.count.Load()
}

type councilRosterEntry struct {
	MemberID    string             `json:"member_id"`
	Model       string             `json:"model"`
	RequestSpec *modelrequest.Spec `json:"request_spec"`
}

type ownedPiHome struct {
	path string
	info os.FileInfo
}

type runState struct {
	opts            Options
	launcherPrompts launcherprompt.Sources
	logDir          string
	caseBase        string
	mcpBase         string
	mcpPublicBase   string
	signingKey      []byte
	lawyers         *lawyerlaunch.Supervisor
	processes       []*processRecord
	secretFiles     []string
	councilStarts   map[string]bool
	councilHomes    map[string]ownedPiHome
	councilModels   map[string]string
	agentErrs       chan error

	mu               sync.Mutex
	councilConfigMu  sync.Mutex
	councilProcessMu sync.Mutex
}

type openClawAuthConfig struct {
	Mode          string
	CodexAuthPath string
	APIKeyEnv     string
}

func Run(ctx context.Context, opts Options) (result Result, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = applyDefaults(opts)
	corePromptFiles, mcpPromptFiles, err := mcpchild.SplitPromptFiles(opts.PromptFiles)
	if err != nil {
		return Result{}, fmt.Errorf("split ARBD prompt files: %w", err)
	}
	opts.PromptFiles = corePromptFiles
	launcherPrompts, err := launcherprompt.Resolve("arbd", opts.LauncherPromptDir, opts.LauncherPromptFiles)
	if err != nil {
		return Result{}, err
	}
	if err := validateOptions(opts); err != nil {
		return Result{}, err
	}
	opts, err = absoluteOutputLayout(opts)
	if err != nil {
		return Result{}, err
	}
	if err := prepareOutputLayout(opts.OutputDir, opts.CoreOutputDir, opts.LogsDir); err != nil {
		return Result{}, err
	}
	state := &runState{
		opts:            opts,
		launcherPrompts: launcherPrompts,
		logDir:          opts.LogsDir,
		councilStarts:   map[string]bool{},
		councilHomes:    map[string]ownedPiHome{},
		councilModels:   map[string]string{},
		agentErrs:       make(chan error, 32),
	}
	state.lawyers, err = lawyerlaunch.New(lawyerlaunch.Runtime{
		OutputDir:       opts.OutputDir,
		LogsDir:         state.logDir,
		ContainerPrefix: "aard",
		PodmanCommand:   opts.PodmanCommand,
		PiImage:         opts.PiImage,
		BaseEnvironment: opts.ParticipantEnvironment,
		Observer:        opts.ProcessObserver,
	})
	if err != nil {
		return Result{}, fmt.Errorf("create lawyer supervisor: %w", err)
	}
	state.signingKey, err = mcpcap.GenerateKey()
	if err != nil {
		return Result{}, fmt.Errorf("generate ARBD MCP signing key: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	completions := launcherCompletions{}
	defer func() {
		completionErr := completions.shutdown(cancel)
		lawyerErr := state.lawyers.Stop()
		agentErr := state.stopAgents()
		secretErr := state.cleanupSecrets()
		err = errors.Join(err, completionErr, lawyerErr, agentErr, secretErr)
		if err != nil && ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
		}
	}()

	caseAPIAddr, err := resolveListenAddr(opts.CaseAPIAddr, "127.0.0.1")
	if err != nil {
		return Result{}, fmt.Errorf("resolve case API address: %w", err)
	}
	state.caseBase = "http://" + caseAPIAddr
	mcpListenAddr := strings.TrimSpace(opts.MCPListenAddr)
	if mcpListenAddr == "" {
		mcpListenAddr = "0.0.0.0:0"
	}
	if _, _, err := net.SplitHostPort(mcpListenAddr); err != nil {
		return Result{}, fmt.Errorf("parse MCP listen address %q: %w", mcpListenAddr, err)
	}
	if len(manualLawyerRoles(opts.AutoLawyers)) > 0 {
		if err := validateManualLawyerAddress(opts.MCPPublicBaseURL, mcpListenAddr); err != nil {
			return Result{}, err
		}
	}
	caseDone, err := startCoreCase(runCtx, opts, caseAPIAddr, state.logDir)
	if err != nil {
		return Result{}, err
	}
	completions.caseDone = caseDone
	completions.casePending = true
	if err := state.waitForCaseAPI(runCtx, &completions); err != nil {
		cancel()
		return Result{}, err
	}

	mcpProcess, err := mcpchild.Start(runCtx, mcpchild.Request{
		Command:        opts.MCPCommand,
		CommandArgs:    []string{"serve"},
		WorkingDir:     opts.MCPWorkingDir,
		Environment:    opts.MCPEnvironment,
		ListenAddr:     mcpListenAddr,
		CaseAPIBase:    state.caseBase,
		SigningKey:     state.signingKey,
		PromptDir:      opts.PromptDir,
		PromptFiles:    mcpPromptFiles,
		LogsDir:        state.logDir,
		LogPrefix:      "mcp",
		Name:           "aard-mcp",
		Observer:       opts.ProcessObserver,
		StartupTimeout: defaultMCPStartupWait,
	})
	if err != nil {
		cancel()
		return Result{}, fmt.Errorf("start ARBD MCP: %w", err)
	}
	mcpDone := mcpProcess.Done()
	completions.mcpDone = mcpDone
	completions.mcpPending = true
	selectedMCPListenAddr, mcpPort, err := mcpchild.SelectedListenAddress(mcpListenAddr, mcpProcess.Address())
	if err != nil {
		cancel()
		return Result{}, err
	}
	state.mcpBase = "http://" + mcpProcess.Address()
	state.mcpPublicBase, err = publicMCPBase(opts.MCPPublicBaseURL, selectedMCPListenAddr)
	if err != nil {
		cancel()
		return Result{}, err
	}
	if err := state.waitForMCP(runCtx, &completions); err != nil {
		cancel()
		return Result{}, err
	}
	roster, err := state.waitForCouncilRoster(runCtx, &completions)
	if err != nil {
		cancel()
		return Result{}, err
	}
	for _, entry := range roster {
		if _, _, err := validatedPiRequest(entry); err != nil {
			cancel()
			return Result{}, err
		}
	}

	for _, role := range manualLawyerRoles(opts.AutoLawyers) {
		if err := state.writeRemoteLawyerSkill(role); err != nil {
			cancel()
			return Result{}, err
		}
	}
	startedPlaintiff := false
	if autoLawyerEnabled(opts.AutoLawyers, "plaintiff") {
		if err := state.startLawyer(runCtx, "plaintiff", mcpPort); err != nil {
			cancel()
			return Result{}, err
		}
		startedPlaintiff = true
	}
	if autoLawyerEnabled(opts.AutoLawyers, "defendant") {
		if startedPlaintiff && lawyerProfile(opts, "plaintiff").Runner == LawyerOpenClaw && lawyerProfile(opts, "defendant").Runner == LawyerOpenClaw {
			if err := state.waitOpenClawStartDelay(runCtx); err != nil {
				cancel()
				return Result{}, err
			}
		}
		if err := state.startLawyer(runCtx, "defendant", mcpPort); err != nil {
			cancel()
			return Result{}, err
		}
	}
	councilTicker := time.NewTicker(time.Second)
	defer councilTicker.Stop()
	handleAgentError := func(exit error) (Result, error) {
		if outcome, ok := waitForCaseOutcome(caseDone, defaultAgentErrorCaseWait); ok {
			completions.casePending = false
			cancel()
			if err := <-mcpDone; err != nil && !cancellationOnly(err) {
				completions.mcpPending = false
				return outcome.result, err
			}
			completions.mcpPending = false
			if writeErr := writeRunSummary(opts.OutputDir, outcome.result, opts); writeErr != nil {
				return outcome.result, writeErr
			}
			return outcome.result, outcome.err
		}
		cancel()
		return Result{}, exit
	}

	for {
		select {
		case outcome := <-caseDone:
			completions.casePending = false
			cancel()
			if err := <-mcpDone; err != nil && !cancellationOnly(err) {
				completions.mcpPending = false
				return outcome.result, err
			}
			completions.mcpPending = false
			if writeErr := writeRunSummary(opts.OutputDir, outcome.result, opts); writeErr != nil {
				return outcome.result, writeErr
			}
			return outcome.result, outcome.err
		case err := <-mcpDone:
			completions.mcpPending = false
			cancel()
			if err == nil {
				return Result{}, fmt.Errorf("MCP server exited before case completion")
			}
			return Result{}, fmt.Errorf("MCP server failed: %w", err)
		case exit := <-state.agentErrs:
			return handleAgentError(exit)
		case exit := <-state.lawyers.Errors():
			return handleAgentError(exit)
		case <-councilTicker.C:
			if err := state.startReadyCouncil(runCtx, roster, mcpPort); err != nil {
				cancel()
				return Result{}, err
			}
		case <-ctx.Done():
			cancel()
			return Result{}, ctx.Err()
		}
	}
}

func cancellationOnly(err error) bool {
	if err == nil {
		return false
	}
	type joinedErrors interface {
		Unwrap() []error
	}
	if joined, ok := err.(joinedErrors); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !cancellationOnly(child) {
				return false
			}
		}
		return true
	}
	return errors.Is(err, context.Canceled)
}

func waitForCaseOutcome(caseDone <-chan caseOutcome, wait time.Duration) (caseOutcome, bool) {
	if wait <= 0 {
		select {
		case outcome := <-caseDone:
			return outcome, true
		default:
			return caseOutcome{}, false
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case outcome := <-caseDone:
		return outcome, true
	case <-timer.C:
		return caseOutcome{}, false
	}
}

type caseOutcome struct {
	result Result
	err    error
}

type launcherCompletions struct {
	caseDone    <-chan caseOutcome
	mcpDone     <-chan error
	casePending bool
	mcpPending  bool
}

func (c *launcherCompletions) shutdown(cancel context.CancelFunc) error {
	cancel()
	var errs []error
	if c.casePending {
		outcome := <-c.caseDone
		c.casePending = false
		if outcome.err != nil {
			errs = append(errs, fmt.Errorf("finish core after launcher shutdown: %w", outcome.err))
		}
	}
	if c.mcpPending {
		mcpErr := <-c.mcpDone
		c.mcpPending = false
		if mcpErr != nil {
			errs = append(errs, fmt.Errorf("finish MCP after launcher shutdown: %w", mcpErr))
		}
	}
	return errors.Join(errs...)
}

func startCoreCase(ctx context.Context, opts Options, caseAPIAddr string, logDir string) (<-chan caseOutcome, error) {
	stdoutPath := filepath.Join(logDir, "aard.stdout")
	stderrPath := filepath.Join(logDir, "aard.stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("create core stdout log: %w", err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create core stderr log: %w", err), stdout.Close())
	}
	runPath := filepath.Join(opts.CoreOutputDir, "run.json")
	previousRun, runExisted, err := readOptionalFile(runPath)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("read existing core result: %w", err), stdout.Close(), stderr.Close())
	}

	cmd := exec.CommandContext(ctx, opts.CoreCommand, coreCaseArgs(opts, caseAPIAddr)...)
	if strings.TrimSpace(opts.CoreWorkingDir) != "" {
		cmd.Dir = strings.TrimSpace(opts.CoreWorkingDir)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = append([]string(nil), opts.CoreEnvironment...)
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("start core case: %w", err), stdout.Close(), stderr.Close())
	}
	finishProcess, err := runstate.Start(opts.ProcessObserver, runstate.ProcessStart{
		Name:      "aard-core",
		Kind:      "core",
		PID:       cmd.Process.Pid,
		StartedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, errors.Join(fmt.Errorf("record core process start: %w", err), cmd.Process.Kill(), cmd.Wait(), stdout.Close(), stderr.Close())
	}

	done := make(chan caseOutcome, 1)
	go func() {
		waitErr := cmd.Wait()
		closeErr := errors.Join(stdout.Close(), stderr.Close())
		result, resultErr := readCoreResult(runPath, previousRun, runExisted)
		processErr := errors.Join(coreProcessError(waitErr, stderrPath), closeErr, resultErr)
		state := runstate.Completed
		if ctx.Err() != nil {
			state = runstate.Canceled
		} else if processErr != nil {
			state = runstate.Failed
		}
		var exitCode *int
		if cmd.ProcessState != nil {
			value := cmd.ProcessState.ExitCode()
			exitCode = &value
		}
		finishErr := runstate.Finish(finishProcess, runstate.ProcessFinish{
			State:      state,
			FinishedAt: time.Now().UTC(),
			ExitCode:   exitCode,
			Error:      runstate.ErrorText(processErr),
		})
		done <- caseOutcome{
			result: result,
			err:    errors.Join(processErr, finishErr),
		}
	}()
	return done, nil
}

func coreCaseArgs(opts Options, caseAPIAddr string) []string {
	args := []string{
		"case",
		"--complaint", opts.ComplaintPath,
		"--out-dir", opts.CoreOutputDir,
		"--case-id", opts.CaseID,
		"--caseapi-addr", caseAPIAddr,
		"--council-backend", "councilapi",
	}
	addString := func(name string, value string) {
		if strings.TrimSpace(value) != "" {
			args = append(args, name, strings.TrimSpace(value))
		}
	}
	addInt := func(name string, value int) {
		if value > 0 {
			args = append(args, name, fmt.Sprintf("%d", value))
		}
	}
	for _, path := range opts.CaseFiles {
		addString("--file", path)
	}
	addString("--run-id", opts.RunID)
	addString("--policy", opts.PolicyPath)
	addInt("--council-size", opts.CouncilSize)
	addString("--judgment-standard", opts.JudgmentStandard)
	addString("--prompt-dir", opts.PromptDir)
	for _, id := range sortedPromptFileIDs(opts.PromptFiles) {
		args = append(args, "--prompt-file", id+"="+strings.TrimSpace(opts.PromptFiles[id]))
	}
	addString("--common-root", opts.CommonRoot)
	addString("--council-pool", opts.CouncilPoolPath)
	addInt("--timeout-seconds", opts.CouncilTimeoutSeconds)
	addInt("--lawyer-timeout-seconds", opts.LawyerTimeoutSeconds)
	addInt("--max-response-bytes", opts.MaxResponseBytes)
	addInt("--invalid-attempt-limit", opts.InvalidAttemptLimit)
	addString("--engine", opts.EnginePath)
	return args
}

func sortedPromptFileIDs(paths map[string]string) []string {
	ids := make([]string, 0, len(paths))
	for id, path := range paths {
		if strings.TrimSpace(id) != "" && strings.TrimSpace(path) != "" {
			ids = append(ids, strings.TrimSpace(id))
		}
	}
	sort.Strings(ids)
	return ids
}

func readOptionalFile(path string) ([]byte, bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

func readCoreResult(path string, previous []byte, existed bool) (Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("read core result %s: %w", path, err)
	}
	if existed && bytes.Equal(raw, previous) {
		return Result{}, fmt.Errorf("core process did not replace existing result %s", path)
	}
	var result Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return Result{}, fmt.Errorf("decode core result %s: %w", path, err)
	}
	result.raw = append(json.RawMessage(nil), bytes.TrimSpace(raw)...)
	return result, nil
}

func coreProcessError(waitErr error, stderrPath string) error {
	if waitErr == nil {
		return nil
	}
	raw, readErr := readFileTail(stderrPath, 64*1024)
	message := strings.TrimSpace(string(raw))
	if message == "" {
		return errors.Join(fmt.Errorf("core case process: %w", waitErr), readErr)
	}
	return errors.Join(fmt.Errorf("core case process: %w: %s", waitErr, message), readErr)
}

func readFileTail(path string, limit int64) (raw []byte, err error) {
	if limit <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", path, closeErr))
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	offset := info.Size() - limit
	if offset < 0 {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	raw, err = io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if index := bytes.IndexByte(raw, '\n'); index >= 0 {
			raw = raw[index+1:]
		}
	}
	return raw, nil
}

type rosterOutcome struct {
	roster []councilRosterEntry
	err    error
}

func applyDefaults(opts Options) Options {
	if opts.CoreEnvironment == nil {
		opts.CoreEnvironment = os.Environ()
	} else {
		opts.CoreEnvironment = append([]string(nil), opts.CoreEnvironment...)
	}
	if opts.ParticipantEnvironment == nil {
		opts.ParticipantEnvironment = os.Environ()
	} else {
		opts.ParticipantEnvironment = append([]string(nil), opts.ParticipantEnvironment...)
	}
	if opts.MCPEnvironment == nil {
		opts.MCPEnvironment = os.Environ()
	} else {
		opts.MCPEnvironment = append([]string(nil), opts.MCPEnvironment...)
	}
	if strings.TrimSpace(opts.OutputDir) != "" {
		if strings.TrimSpace(opts.CoreOutputDir) == "" {
			opts.CoreOutputDir = filepath.Join(opts.OutputDir, "aard-output")
		}
		if strings.TrimSpace(opts.LogsDir) == "" {
			opts.LogsDir = filepath.Join(opts.OutputDir, "logs")
		}
	}
	if strings.TrimSpace(opts.CoreCommand) == "" {
		opts.CoreCommand = DefaultCoreCommand
	}
	if strings.TrimSpace(opts.MCPCommand) == "" {
		opts.MCPCommand = DefaultMCPCommand
	}
	if strings.TrimSpace(opts.MCPWorkingDir) == "" {
		opts.MCPWorkingDir = opts.CoreWorkingDir
	}
	if strings.TrimSpace(opts.DockerCommand) == "" {
		opts.DockerCommand = DefaultDockerCommand
	}
	if strings.TrimSpace(opts.PodmanCommand) == "" {
		opts.PodmanCommand = DefaultPodmanCommand
	}
	if strings.TrimSpace(opts.OpenClawImage) == "" {
		opts.OpenClawImage = defaultOpenClawImage
	}
	if strings.TrimSpace(opts.OpenClawModel) == "" {
		opts.OpenClawModel = defaultOpenClawModel
	}
	if strings.TrimSpace(opts.OpenClawThinking) == "" {
		opts.OpenClawThinking = defaultOpenClawThinking
	}
	if opts.LawyerWebSearch == nil {
		enabled := true
		opts.LawyerWebSearch = &enabled
	}
	if opts.OpenClawTimeoutSeconds <= 0 {
		opts.OpenClawTimeoutSeconds = defaultOpenClawTimeoutSeconds
	}
	if strings.TrimSpace(opts.OpenClawAuth) == "" {
		opts.OpenClawAuth = defaultOpenClawAuth
	}
	if opts.OpenClawStartDelaySeconds < 0 {
		opts.OpenClawStartDelaySeconds = defaultOpenClawStartDelay
	}
	if strings.TrimSpace(opts.OpenClawCodexAuthPath) == "" {
		opts.OpenClawCodexAuthPath = defaultCodexAuthPath(opts.ParticipantEnvironment)
	}
	if opts.CouncilTimeoutSeconds <= 0 {
		opts.CouncilTimeoutSeconds = DefaultRunCouncilTimeoutSeconds
	}
	if opts.LawyerTimeoutSeconds <= 0 {
		opts.LawyerTimeoutSeconds = DefaultRunLawyerTimeoutSeconds
	}
	if strings.TrimSpace(opts.PiImage) == "" {
		if image, ok := environmentValue(opts.ParticipantEnvironment, "PI_CONTAINER_IMAGE"); ok && strings.TrimSpace(image) != "" {
			opts.PiImage = image
		} else {
			opts.PiImage = defaultPiImage
		}
	}
	if strings.TrimSpace(opts.PiMCPAdapter) == "" {
		opts.PiMCPAdapter = defaultPiMCPAdapter
	}
	if opts.CouncilOutputLimitBytes == 0 {
		opts.CouncilOutputLimitBytes = DefaultCouncilOutputLimitBytes
	}
	opts.OpenClawNetwork = strings.TrimSpace(opts.OpenClawNetwork)
	if strings.TrimSpace(opts.DockerMCPHost) == "" && opts.OpenClawNetwork == "host" {
		opts.DockerMCPHost = "127.0.0.1"
	}
	if strings.TrimSpace(opts.DockerMCPHost) == "" {
		opts.DockerMCPHost = "host.docker.internal"
	}
	if strings.TrimSpace(opts.PodmanMCPHost) == "" {
		opts.PodmanMCPHost = "127.0.0.1"
	}
	if strings.TrimSpace(opts.AutoLawyers) == "" {
		opts.AutoLawyers = DefaultAutoLawyers
	}
	if opts.PlaintiffLawyer.Runner == "" {
		opts.PlaintiffLawyer.Runner = LawyerOpenClaw
	}
	if opts.DefendantLawyer.Runner == "" {
		opts.DefendantLawyer.Runner = LawyerOpenClaw
	}
	return opts
}

func lawyerWebSearchEnabled(configured *bool) bool {
	return configured == nil || *configured
}

func validateOptions(opts Options) error {
	coreEnvironment := opts.CoreEnvironment
	participantEnvironment := opts.ParticipantEnvironment
	if strings.TrimSpace(opts.ComplaintPath) == "" {
		return fmt.Errorf("complaint path is required")
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return fmt.Errorf("output dir is required")
	}
	if strings.TrimSpace(opts.CoreOutputDir) == "" {
		return fmt.Errorf("core output dir is required")
	}
	if strings.TrimSpace(opts.LogsDir) == "" {
		return fmt.Errorf("logs dir is required")
	}
	if err := validateOutputLayout(opts.OutputDir, opts.CoreOutputDir, opts.LogsDir); err != nil {
		return err
	}
	if strings.TrimSpace(opts.CaseID) == "" {
		return fmt.Errorf("case id is required")
	}
	if value, ok := environmentValue(coreEnvironment, "OPENROUTER_API_KEY"); !ok || strings.TrimSpace(value) == "" {
		return fmt.Errorf("OPENROUTER_API_KEY is required for Pi council")
	}
	if _, err := autoLawyerRoles(opts.AutoLawyers); err != nil {
		return err
	}
	if opts.OpenClawNetwork != "" && opts.OpenClawNetwork != "host" {
		return fmt.Errorf("invalid OpenClaw network %q; expected host or empty", opts.OpenClawNetwork)
	}
	if opts.CouncilOutputLimitBytes < 0 {
		return fmt.Errorf("council output limit bytes must be non-negative")
	}
	for _, role := range autoLawyerRolesOrEmpty(opts.AutoLawyers) {
		if err := validateLawyerProfile(lawyerProfile(opts, role), opts, participantEnvironment); err != nil {
			return fmt.Errorf("%s lawyer profile: %w", role, err)
		}
	}
	return nil
}

func validateOutputLayout(outputDir string, coreOutputDir string, logsDir string) error {
	output, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve output dir: %w", err)
	}
	core, err := filepath.Abs(coreOutputDir)
	if err != nil {
		return fmt.Errorf("resolve core output dir: %w", err)
	}
	logs, err := filepath.Abs(logsDir)
	if err != nil {
		return fmt.Errorf("resolve logs dir: %w", err)
	}
	if filepath.Dir(core) != output {
		return fmt.Errorf("core output dir must be a direct child of output dir")
	}
	if filepath.Dir(logs) != output {
		return fmt.Errorf("logs dir must be a direct child of output dir")
	}
	if core == logs {
		return fmt.Errorf("core output dir and logs dir must differ")
	}
	return nil
}

func absoluteOutputLayout(opts Options) (Options, error) {
	var err error
	opts.OutputDir, err = filepath.Abs(opts.OutputDir)
	if err != nil {
		return Options{}, fmt.Errorf("resolve output dir: %w", err)
	}
	opts.CoreOutputDir, err = filepath.Abs(opts.CoreOutputDir)
	if err != nil {
		return Options{}, fmt.Errorf("resolve core output dir: %w", err)
	}
	opts.LogsDir, err = filepath.Abs(opts.LogsDir)
	if err != nil {
		return Options{}, fmt.Errorf("resolve logs dir: %w", err)
	}
	return opts, nil
}

func prepareOutputLayout(outputDir string, coreOutputDir string, logsDir string) error {
	output, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve output dir: %w", err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	resolvedOutput, err := filepath.EvalSymlinks(output)
	if err != nil {
		return fmt.Errorf("resolve created output dir: %w", err)
	}
	coreInfo, err := prepareOutputChild(output, resolvedOutput, coreOutputDir, "core output")
	if err != nil {
		return err
	}
	logsInfo, err := prepareOutputChild(output, resolvedOutput, logsDir, "logs")
	if err != nil {
		return err
	}
	if os.SameFile(coreInfo, logsInfo) {
		return fmt.Errorf("core output dir and logs dir resolve to the same directory")
	}
	return nil
}

func prepareOutputChild(output string, resolvedOutput string, childPath string, label string) (os.FileInfo, error) {
	child, err := filepath.Abs(childPath)
	if err != nil {
		return nil, fmt.Errorf("resolve %s dir: %w", label, err)
	}
	if filepath.Dir(child) != output {
		return nil, fmt.Errorf("%s dir must be a direct child of output dir", label)
	}
	if err := os.Mkdir(child, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create %s dir: %w", label, err)
	}
	info, err := os.Lstat(child)
	if err != nil {
		return nil, fmt.Errorf("inspect %s dir: %w", label, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s dir is not a regular directory", label)
	}
	resolvedChild, err := filepath.EvalSymlinks(child)
	if err != nil {
		return nil, fmt.Errorf("resolve %s dir after creation: %w", label, err)
	}
	if resolvedChild != filepath.Join(resolvedOutput, filepath.Base(child)) {
		return nil, fmt.Errorf("%s dir resolves outside output dir", label)
	}
	resolvedInfo, err := os.Lstat(child)
	if err != nil {
		return nil, fmt.Errorf("inspect resolved %s dir: %w", label, err)
	}
	if !resolvedInfo.IsDir() || resolvedInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, resolvedInfo) {
		return nil, fmt.Errorf("%s dir identity changed during validation", label)
	}
	entries, err := os.ReadDir(child)
	if err != nil {
		return nil, fmt.Errorf("read %s dir: %w", label, err)
	}
	if len(entries) != 0 {
		return nil, fmt.Errorf("%s dir must be empty", label)
	}
	finalInfo, err := os.Lstat(child)
	if err != nil {
		return nil, fmt.Errorf("inspect empty %s dir: %w", label, err)
	}
	if !os.SameFile(resolvedInfo, finalInfo) {
		return nil, fmt.Errorf("%s dir identity changed during validation", label)
	}
	return finalInfo, nil
}

func autoLawyerRolesOrEmpty(mode string) []string {
	roles, err := autoLawyerRoles(mode)
	if err != nil {
		return nil
	}
	return roles
}

func lawyerProfile(opts Options, role string) LawyerProfile {
	var profile LawyerProfile
	if role == "plaintiff" {
		profile = opts.PlaintiffLawyer
	} else {
		profile = opts.DefendantLawyer
	}
	if profile.Runner == "" {
		profile.Runner = LawyerOpenClaw
	}
	return profile
}

func validateLawyerProfile(profile LawyerProfile, opts Options, baseEnvironment []string) error {
	baseEnvironment = lawyerEnvironment(profile, opts, baseEnvironment)
	switch profile.Runner {
	case LawyerOpenClaw:
		if strings.TrimSpace(profile.Command) != "" {
			return errors.New("OpenClaw uses the configured Docker command")
		}
		if profile.Resume != nil && *profile.Resume {
			return errors.New("OpenClaw does not support session resumption")
		}
		_, err := resolveOpenClawLawyerAuth(profile, opts, baseEnvironment)
		return err
	case LawyerPi:
		if strings.TrimSpace(profile.Command) != "" {
			return errors.New("Pi uses the configured Podman command and Pi image")
		}
		return headless.ValidateProfile(headlessProfile(profile, opts), baseEnvironment)
	case LawyerCodex, LawyerClaude:
		return headless.ValidateProfile(headlessProfile(profile, opts), baseEnvironment)
	default:
		return fmt.Errorf("unsupported runner %q", profile.Runner)
	}
}

func headlessProfile(profile LawyerProfile, opts Options) headless.Profile {
	return headless.Profile{
		Runner:             headless.Runner(profile.Runner),
		Provider:           profile.Provider,
		Command:            strings.TrimSpace(profile.Command),
		Model:              strings.TrimSpace(profile.Model),
		ReasoningEffort:    strings.TrimSpace(profile.ReasoningEffort),
		MCPAdapter:         strings.TrimSpace(opts.PiMCPAdapter),
		WebSearchExtension: defaultPiWebSearch,
		Auth: headless.Auth{
			Mode:            profile.AuthMode,
			CredentialsFile: strings.TrimSpace(profile.CredentialsFile),
			APIKeyEnv:       strings.TrimSpace(profile.APIKeyEnv),
		},
		Resume: profile.Resume,
	}
}

func lawyerEnvironment(profile LawyerProfile, opts Options, fallback []string) []string {
	if profile.Environment != nil {
		return profile.Environment
	}
	credentialNames := []string{"OPENAI_API_KEY", "OPENROUTER_API_KEY", "ANTHROPIC_API_KEY", "CODEX_API_KEY", "ANTHROPIC_AUTH_TOKEN"}
	for _, configured := range []LawyerProfile{opts.PlaintiffLawyer, opts.DefendantLawyer} {
		if name := strings.TrimSpace(configured.APIKeyEnv); name != "" {
			credentialNames = append(credentialNames, name)
		}
	}
	environment := removeEnvironmentNames(fallback, credentialNames)
	name := strings.TrimSpace(profile.APIKeyEnv)
	if name == "" && profile.Runner == LawyerOpenClaw && (profile.AuthMode == headless.AuthAPIKey || (profile.AuthMode == "" && strings.EqualFold(opts.OpenClawAuth, "api-key"))) {
		name = "OPENAI_API_KEY"
	}
	if value, ok := environmentValue(fallback, name); name != "" && ok {
		environment = append(environment, name+"="+value)
	}
	return environment
}

func removeEnvironmentNames(environment []string, names []string) []string {
	removed := make(map[string]struct{}, len(names))
	for _, name := range names {
		removed[name] = struct{}{}
	}
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, remove := removed[name]; remove {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func lawyerWorkDir(profile LawyerProfile, opts Options, role string) (string, error) {
	workDir := strings.TrimSpace(profile.WorkDir)
	if workDir == "" {
		stateDir := strings.TrimSpace(profile.StateDir)
		if stateDir == "" {
			stateDir = filepath.Join(opts.OutputDir, "agents", role)
		}
		workDir = filepath.Join(stateDir, "work")
	}
	resolved, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("resolve %s lawyer work directory: %w", role, err)
	}
	return resolved, nil
}

func lawyerPromptWorkspace(profile LawyerProfile, workDir string) string {
	switch profile.Runner {
	case LawyerOpenClaw:
		return lawyerlaunch.OpenClawWorkspacePath
	case LawyerPi:
		return lawyerlaunch.PiWorkspacePath
	default:
		return workDir
	}
}

func autoLawyerRoles(mode string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "both":
		return []string{"plaintiff", "defendant"}, nil
	case "plaintiff":
		return []string{"plaintiff"}, nil
	case "defendant":
		return []string{"defendant"}, nil
	case "none":
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid auto lawyer mode %q; expected both, plaintiff, defendant, or none", mode)
	}
}

func autoLawyerEnabled(mode string, role string) bool {
	roles, err := autoLawyerRoles(mode)
	if err != nil {
		return false
	}
	for _, current := range roles {
		if current == role {
			return true
		}
	}
	return false
}

func manualLawyerRoles(mode string) []string {
	manual := []string{}
	for _, role := range []string{"plaintiff", "defendant"} {
		if !autoLawyerEnabled(mode, role) {
			manual = append(manual, role)
		}
	}
	return manual
}

func defaultCodexAuthPath(environment []string) string {
	if codexHome, ok := environmentValue(environment, "CODEX_HOME"); ok && strings.TrimSpace(codexHome) != "" {
		return filepath.Join(codexHome, "auth.json")
	}
	if home, ok := environmentValue(environment, "HOME"); ok && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".codex", "auth.json")
	}
	return filepath.Join(".codex", "auth.json")
}

func resolveOpenClawAuth(opts Options) (openClawAuthConfig, error) {
	baseEnvironment := opts.ParticipantEnvironment
	if baseEnvironment == nil {
		baseEnvironment = os.Environ()
	}
	return resolveOpenClawLawyerAuth(LawyerProfile{}, opts, baseEnvironment)
}

func resolveOpenClawLawyerAuth(profile LawyerProfile, opts Options, baseEnvironment []string) (openClawAuthConfig, error) {
	if strings.TrimSpace(profile.CredentialsFile) != "" && strings.TrimSpace(profile.APIKeyEnv) != "" {
		return openClawAuthConfig{}, errors.New("OpenClaw authentication cannot name both subscription credentials and an API-key source")
	}
	if profile.AuthMode == headless.AuthSubscription && strings.TrimSpace(profile.APIKeyEnv) != "" {
		return openClawAuthConfig{}, errors.New("OpenClaw subscription authentication cannot name an API-key source")
	}
	if profile.AuthMode == headless.AuthAPIKey && strings.TrimSpace(profile.CredentialsFile) != "" {
		return openClawAuthConfig{}, errors.New("OpenClaw API-key authentication cannot name subscription credentials")
	}
	if profile.AuthMode == headless.AuthSubscription {
		path := strings.TrimSpace(profile.CredentialsFile)
		if path == "" {
			path = opts.OpenClawCodexAuthPath
		}
		path, err := validateCodexAuthPath(path, baseEnvironment)
		if err != nil {
			return openClawAuthConfig{}, err
		}
		return openClawAuthConfig{Mode: "codex", CodexAuthPath: path}, nil
	}
	if profile.AuthMode == headless.AuthAPIKey {
		name := strings.TrimSpace(profile.APIKeyEnv)
		if name == "" {
			name = "OPENAI_API_KEY"
		}
		if !validEnvironmentVariableName(name) {
			return openClawAuthConfig{}, fmt.Errorf("invalid OpenClaw API-key source environment variable %q", name)
		}
		if value, ok := environmentValue(baseEnvironment, name); !ok || strings.TrimSpace(value) == "" {
			return openClawAuthConfig{}, fmt.Errorf("%s is required for OpenClaw API-key authentication", name)
		}
		return openClawAuthConfig{Mode: "api-key", APIKeyEnv: name}, nil
	}
	if profile.AuthMode != "" {
		return openClawAuthConfig{}, fmt.Errorf("invalid OpenClaw authentication mode %q", profile.AuthMode)
	}

	mode := strings.ToLower(strings.TrimSpace(opts.OpenClawAuth))
	if mode == "" {
		mode = defaultOpenClawAuth
	}
	switch mode {
	case "codex":
		path, err := validateCodexAuthPath(opts.OpenClawCodexAuthPath, baseEnvironment)
		if err != nil {
			return openClawAuthConfig{}, err
		}
		return openClawAuthConfig{Mode: "codex", CodexAuthPath: path}, nil
	case "api-key":
		if value, ok := environmentValue(baseEnvironment, "OPENAI_API_KEY"); !ok || strings.TrimSpace(value) == "" {
			return openClawAuthConfig{}, fmt.Errorf("OPENAI_API_KEY is required when --openclaw-auth=api-key")
		}
		return openClawAuthConfig{Mode: "api-key", APIKeyEnv: "OPENAI_API_KEY"}, nil
	default:
		return openClawAuthConfig{}, fmt.Errorf("invalid OpenClaw auth mode %q; expected codex or api-key", mode)
	}
}

func validateCodexAuthPath(path string, environment []string) (string, error) {
	path = expandUserPath(strings.TrimSpace(path), environment)
	if path == "" {
		return "", fmt.Errorf("Codex auth path is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read Codex auth file %s: %w", path, err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", fmt.Errorf("decode Codex auth file %s: %w", path, err)
	}
	if len(decoded) == 0 {
		return "", fmt.Errorf("Codex auth file %s is empty", path)
	}
	return path, nil
}

func expandUserPath(path string, environment []string) string {
	home, _ := environmentValue(environment, "HOME")
	home = strings.TrimSpace(home)
	if path == "~" {
		if home != "" {
			return home
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func resolveListenAddr(value string, defaultHost string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" && !strings.HasSuffix(value, ":0") {
		return value, nil
	}
	host := defaultHost
	if value != "" {
		parsedHost, _, err := net.SplitHostPort(value)
		if err == nil && parsedHost != "" {
			host = parsedHost
		}
	}
	probeHost := host
	if probeHost == "0.0.0.0" || probeHost == "::" || probeHost == "" {
		probeHost = "127.0.0.1"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(probeHost, "0"))
	if err != nil {
		return "", err
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	closeErr := ln.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return net.JoinHostPort(host, port), nil
}

func publicMCPBase(value string, listenAddr string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "http://" + listenAddr, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse MCP public base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("MCP public base URL must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("MCP public base URL requires a host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("MCP public base URL must not contain a query or fragment")
	}
	return value, nil
}

func validateManualLawyerAddress(publicBase string, listenAddr string) error {
	if strings.TrimSpace(publicBase) != "" {
		return nil
	}
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return fmt.Errorf("parse MCP listen address %q: %w", listenAddr, err)
	}
	switch host {
	case "", "0.0.0.0", "::":
		return fmt.Errorf("manual lawyer mode requires --mcp-public-base-url when --mcp-listen uses a wildcard host")
	default:
		return nil
	}
}

func mcpEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/mcp"
}

func (s *runState) issueMCPCapability(assignmentType, principalID string) (string, error) {
	capability, err := mcpcap.Issue(s.signingKey, mcpcap.Claims{
		Audience:       "aard",
		CaseID:         s.opts.CaseID,
		AssignmentType: assignmentType,
		PrincipalID:    principalID,
	})
	if err != nil {
		return "", fmt.Errorf("issue ARBD MCP capability for %s %s: %w", assignmentType, principalID, err)
	}
	return capability, nil
}

func waitForHealth(ctx context.Context, rawURL string, timeout time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			closeErr := resp.Body.Close()
			if closeErr != nil {
				return closeErr
			}
			if resp.StatusCode == http.StatusNoContent {
				return nil
			}
		}
		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("%s did not become healthy within %s", rawURL, timeout)
		case <-ticker.C:
		}
	}
}

func waitForCaseHealth(ctx context.Context, rawURL, caseID, runID string, timeout time.Duration) error {
	return corehealth.Wait(ctx, http.DefaultClient, rawURL, caseID, runID, timeout)
}

func (s *runState) waitForCaseAPI(ctx context.Context, completions *launcherCompletions) error {
	healthDone := make(chan error, 1)
	go func() {
		healthDone <- waitForCaseHealth(ctx, s.caseBase+"/health", s.opts.CaseID, s.opts.RunID, defaultCaseAPIStartupWait)
	}()
	select {
	case err := <-healthDone:
		return err
	case outcome := <-completions.caseDone:
		completions.casePending = false
		if outcome.err != nil {
			return outcome.err
		}
		return fmt.Errorf("case finished before case API became healthy")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *runState) waitForMCP(ctx context.Context, completions *launcherCompletions) error {
	healthDone := make(chan error, 1)
	go func() {
		healthDone <- waitForHealth(ctx, s.mcpBase+"/health", defaultMCPStartupWait)
	}()
	select {
	case err := <-healthDone:
		return err
	case outcome := <-completions.caseDone:
		completions.casePending = false
		if outcome.err != nil {
			return outcome.err
		}
		return fmt.Errorf("case finished before MCP became healthy")
	case err := <-completions.mcpDone:
		completions.mcpPending = false
		if err == nil {
			return fmt.Errorf("MCP server exited before health check")
		}
		return fmt.Errorf("MCP server failed before health check: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *runState) startLawyer(ctx context.Context, role string, mcpPort string) error {
	profile := lawyerProfile(s.opts, role)
	workDir, err := lawyerWorkDir(profile, s.opts, role)
	if err != nil {
		return err
	}
	server := headlessMCPServerName(s.opts.CaseID, role)
	mcpHost := "127.0.0.1"
	promptID := "participant.headless"
	if profile.Runner == LawyerOpenClaw {
		mcpHost = s.opts.DockerMCPHost
		server = "aard-" + s.opts.CaseID + "-" + role
		promptID = "participant.openclaw"
	} else if profile.Runner == LawyerPi {
		mcpHost = s.opts.PodmanMCPHost
		promptID = "participant.pi"
	}
	mcpURL := mcpEndpoint("http://" + net.JoinHostPort(mcpHost, mcpPort))
	capability, err := s.issueMCPCapability("lawyer", role)
	if err != nil {
		return err
	}
	instructions, err := s.renderLauncherPrompt(promptID, instructionData{
		CaseID:             s.opts.CaseID,
		RoleID:             role,
		MCPServer:          server,
		MCPURL:             mcpURL,
		Workspace:          lawyerPromptWorkspace(profile, workDir),
		SearchInstructions: launcherprompt.SearchInstructions(lawyerWebSearchEnabled(s.opts.LawyerWebSearch)),
	})
	if err != nil {
		return err
	}
	launchProfile := lawyerlaunch.Profile{Runner: lawyerlaunch.Runner(profile.Runner)}
	if profile.Runner == LawyerOpenClaw {
		auth, err := resolveOpenClawLawyerAuth(profile, s.opts, lawyerEnvironment(profile, s.opts, s.opts.ParticipantEnvironment))
		if err != nil {
			return err
		}
		model := strings.TrimSpace(profile.Model)
		if model == "" {
			model = s.opts.OpenClawModel
		}
		thinking := strings.TrimSpace(profile.ReasoningEffort)
		if thinking == "" {
			thinking = s.opts.OpenClawThinking
		}
		launchProfile.OpenClaw = lawyerlaunch.OpenClawProfile{
			Command:                  s.opts.DockerCommand,
			Image:                    s.opts.OpenClawImage,
			Provider:                 string(profile.Provider),
			Model:                    model,
			Thinking:                 thinking,
			AgentTimeoutSeconds:      s.opts.OpenClawTimeoutSeconds,
			LawyerTurnTimeoutSeconds: effectiveLawyerTurnTimeoutSeconds(s.opts),
			Network:                  s.opts.OpenClawNetwork,
			Auth: lawyerlaunch.OpenClawAuth{
				Mode:          lawyerlaunch.OpenClawAuthMode(auth.Mode),
				CodexAuthPath: auth.CodexAuthPath,
				APIKeyEnv:     auth.APIKeyEnv,
			},
		}
	} else {
		launchProfile.Headless = headlessProfile(profile, s.opts)
	}
	return s.lawyers.Start(ctx, launchProfile, lawyerlaunch.Assignment{
		CaseID:      s.opts.CaseID,
		RunID:       s.opts.RunID,
		RoleID:      role,
		Profile:     profile.Name,
		Prompt:      instructions,
		WebSearch:   s.opts.LawyerWebSearch,
		Environment: lawyerEnvironment(profile, s.opts, s.opts.ParticipantEnvironment),
		StateDir:    profile.StateDir,
		WorkDir:     workDir,
		MCP: headless.MCPServer{
			Name:        server,
			URL:         mcpURL,
			BearerToken: capability,
		},
		VerifyExit: s.handleLawyerExit,
	})
}

func headlessMCPServerName(caseID, role string) string {
	name := containerName("aard-" + caseID + "-" + role)
	return strings.ReplaceAll(name, ".", "-")
}

func (s *runState) writeRemoteLawyerSkill(role string) error {
	server := "aard-" + s.opts.CaseID + "-" + role
	mcpURL := mcpEndpoint(s.mcpPublicBase)
	capability, err := s.issueMCPCapability("lawyer", role)
	if err != nil {
		return err
	}
	mcpJSON, err := json.Marshal(map[string]any{
		"url":       mcpURL,
		"transport": "streamable-http",
		"headers":   map[string]string{"Authorization": "Bearer " + capability},
	})
	if err != nil {
		return err
	}
	instructions, err := s.renderLauncherPrompt("skill.openclaw", instructionData{
		CaseID:             s.opts.CaseID,
		RoleID:             role,
		MCPServer:          server,
		MCPURL:             mcpURL,
		MCPJSON:            string(mcpJSON),
		SearchInstructions: launcherprompt.RemoteSearchInstructions(lawyerWebSearchEnabled(s.opts.LawyerWebSearch)),
	})
	if err != nil {
		return err
	}
	name := "openclaw-" + role + "-lawyer-skill.md"
	path := filepath.Join(s.opts.OutputDir, name)
	if err := writeExclusiveFile(path, []byte(instructions), 0o600, func() { s.trackSecretFile(path) }); err != nil {
		return fmt.Errorf("write remote lawyer skill %s: %w", path, err)
	}
	if s.opts.Log != nil {
		if _, err := fmt.Fprintf(s.opts.Log, "remote %s lawyer skill written to %s\n", role, path); err != nil {
			return fmt.Errorf("write remote lawyer skill log: %w", err)
		}
	}
	return nil
}

func writeExclusiveFile(path string, raw []byte, mode os.FileMode, created func()) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	created()
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", path, closeErr))
		}
	}()
	n, err := f.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

func effectiveLawyerTurnTimeoutSeconds(opts Options) int {
	if opts.LawyerTimeoutSeconds > 0 {
		return opts.LawyerTimeoutSeconds
	}
	return DefaultRunLawyerTimeoutSeconds
}

func (s *runState) waitOpenClawStartDelay(ctx context.Context) error {
	delay := time.Duration(s.opts.OpenClawStartDelaySeconds) * time.Second
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case err := <-s.agentErrs:
		return err
	case err := <-s.lawyers.Errors():
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *runState) waitForCouncilRoster(ctx context.Context, completions *launcherCompletions) ([]councilRosterEntry, error) {
	rosterDone := make(chan rosterOutcome, 1)
	go func() {
		roster, err := s.pollCouncilRoster(ctx)
		rosterDone <- rosterOutcome{roster: roster, err: err}
	}()
	select {
	case outcome := <-rosterDone:
		return outcome.roster, outcome.err
	case outcome := <-completions.caseDone:
		completions.casePending = false
		if outcome.err != nil {
			return nil, outcome.err
		}
		return nil, fmt.Errorf("case finished before council roster became available")
	case err := <-completions.mcpDone:
		completions.mcpPending = false
		if err == nil {
			return nil, fmt.Errorf("MCP server exited before council roster became available")
		}
		return nil, fmt.Errorf("MCP server failed before council roster became available: %w", err)
	case err := <-s.agentErrs:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *runState) pollCouncilRoster(ctx context.Context) ([]councilRosterEntry, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, defaultCouncilRosterWait)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	statusURL := s.caseBase + "/lawyerapi/v1/status?case_id=" + url.QueryEscape(s.opts.CaseID) + "&role_id=observer"
	for {
		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, statusURL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil {
				return nil, readErr
			}
			if closeErr != nil {
				return nil, closeErr
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var status councilRosterResponse
				dec := json.NewDecoder(bytes.NewReader(body))
				dec.UseNumber()
				if err := dec.Decode(&status); err != nil {
					return nil, err
				}
				if len(status.CouncilRoster) > 0 {
					if err := os.WriteFile(filepath.Join(s.logDir, "observer-status.json"), body, 0o644); err != nil {
						return nil, err
					}
					return status.CouncilRoster, nil
				}
			}
		}
		select {
		case <-deadlineCtx.Done():
			return nil, fmt.Errorf("council roster did not become available within %s", defaultCouncilRosterWait)
		case <-ticker.C:
		}
	}
}

func (s *runState) startReadyCouncil(ctx context.Context, roster []councilRosterEntry, mcpPort string) error {
	for _, entry := range roster {
		status, err := s.councilStatus(ctx, entry.MemberID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(status.Status) != "ready" || status.Turn == nil {
			continue
		}
		if strings.TrimSpace(status.Turn.MemberID) != strings.TrimSpace(entry.MemberID) {
			continue
		}
		opportunityID := strings.TrimSpace(status.Turn.OpportunityID)
		if opportunityID == "" {
			return fmt.Errorf("council status for %s is ready without opportunity_id", entry.MemberID)
		}
		s.mu.Lock()
		started := s.councilStarts[opportunityID]
		if !started {
			s.councilStarts[opportunityID] = true
		}
		s.mu.Unlock()
		if started {
			continue
		}
		if err := s.startPiCouncil(ctx, entry, mcpPort, opportunityID); err != nil {
			return err
		}
	}
	return nil
}

func (s *runState) councilStatus(ctx context.Context, memberID string) (status councilStatusResponse, err error) {
	statusURL := s.caseBase + "/councilapi/v1/get?case_id=" + url.QueryEscape(s.opts.CaseID) + "&member_id=" + url.QueryEscape(memberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return councilStatusResponse{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return councilStatusResponse{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close council status response: %w", closeErr))
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return councilStatusResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return councilStatusResponse{}, fmt.Errorf("council status for %s returned HTTP %d: %s", memberID, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&status); err != nil {
		return councilStatusResponse{}, err
	}
	return status, nil
}

func (s *runState) lawyerStatus(ctx context.Context, roleID string) (status lawyerStatusResponse, err error) {
	statusURL := s.caseBase + "/lawyerapi/v1/status?case_id=" + url.QueryEscape(s.opts.CaseID) + "&role_id=" + url.QueryEscape(roleID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return lawyerStatusResponse{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return lawyerStatusResponse{}, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close lawyer status response: %w", closeErr))
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return lawyerStatusResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return lawyerStatusResponse{}, fmt.Errorf("lawyer status for %s returned HTTP %d: %s", roleID, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&status); err != nil {
		return lawyerStatusResponse{}, err
	}
	return status, nil
}

func (s *runState) handleLawyerExit(ctx context.Context, roleID string, name string) error {
	status, err := s.lawyerStatus(ctx, roleID)
	if err != nil {
		return fmt.Errorf("check lawyer status after %s exit: %w", name, err)
	}
	if openClawLawyerExitAllowed(status) {
		return nil
	}
	return fmt.Errorf("lawyer process %s exited before case completion", name)
}

func openClawLawyerExitAllowed(status lawyerStatusResponse) bool {
	switch strings.TrimSpace(status.Status) {
	case "done", "failed":
		return true
	}
	switch strings.TrimSpace(status.CaseStatus) {
	case "closed", "failed":
		return true
	}
	if strings.TrimSpace(status.Phase) == "deliberation" && strings.TrimSpace(status.Status) == "waiting" {
		return true
	}
	if strings.TrimSpace(status.Phase) == "closed" {
		return true
	}
	return false
}

func (s *runState) startPiCouncil(ctx context.Context, entry councilRosterEntry, mcpPort string, opportunityID string) error {
	memberID := strings.TrimSpace(entry.MemberID)
	if err := validateCouncilMemberID(memberID); err != nil {
		return err
	}
	entry.MemberID = memberID
	opportunityID = strings.TrimSpace(opportunityID)
	if opportunityID == "" {
		return fmt.Errorf("council opportunity id is required for %s", entry.MemberID)
	}
	server := defaultPiMCPServer
	mcpURL := mcpEndpoint("http://" + net.JoinHostPort(s.opts.PodmanMCPHost, mcpPort))
	capability, err := s.issueMCPCapability("council", entry.MemberID)
	if err != nil {
		return err
	}
	instructions, err := s.renderLauncherPrompt("council.pi", instructionData{
		CaseID:    s.opts.CaseID,
		MemberID:  entry.MemberID,
		MCPServer: server,
		MCPURL:    mcpURL,
	})
	if err != nil {
		return err
	}
	s.councilProcessMu.Lock()
	defer s.councilProcessMu.Unlock()
	if err := s.stopPriorCouncilProcesses(entry.MemberID, opportunityID); err != nil {
		return err
	}
	home, model, err := s.prepareCouncilHome(entry, server, mcpURL, capability)
	if err != nil {
		return fmt.Errorf("prepare Pi home: %w", err)
	}
	processName := councilProcessName(entry.MemberID, opportunityID)
	container := piContainerName(s.opts.CaseID, entry.MemberID, opportunityID)
	args := piRunArgs(s.opts, container, home, model, instructions)
	proc, err := s.startProcess(ctx, processName, "podman", s.opts.PodmanCommand, args, s.opts.CoreEnvironment, container, "", &councilProcessTarget{
		memberID:      entry.MemberID,
		opportunityID: opportunityID,
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.processes = append(s.processes, proc)
	s.mu.Unlock()
	return nil
}

func (s *runState) councilHome(memberID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if owned, ok := s.councilHomes[memberID]; ok {
		info, err := os.Lstat(owned.path)
		if err != nil {
			return "", fmt.Errorf("inspect owned Pi home: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(owned.info, info) {
			return "", fmt.Errorf("owned Pi home for %s was replaced", memberID)
		}
		return owned.path, nil
	}
	home, err := createOutputSubdir(s.opts.OutputDir, "pi-"+memberID, 0o755)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(home)
	if err != nil {
		return "", fmt.Errorf("inspect claimed Pi home: %w", err)
	}
	if s.councilHomes == nil {
		s.councilHomes = map[string]ownedPiHome{}
	}
	s.councilHomes[memberID] = ownedPiHome{path: home, info: info}
	return home, nil
}

func (s *runState) prepareCouncilHome(entry councilRosterEntry, server string, mcpURL string, capability string) (string, string, error) {
	s.councilConfigMu.Lock()
	defer s.councilConfigMu.Unlock()
	home, err := s.councilHome(entry.MemberID)
	if err != nil {
		return "", "", err
	}
	if s.councilModels == nil {
		s.councilModels = map[string]string{}
	}
	_, model, err := validatedPiRequest(entry)
	if err != nil {
		return "", "", err
	}
	if configuredModel := s.councilModels[entry.MemberID]; configuredModel != "" {
		if configuredModel != model {
			return "", "", fmt.Errorf("Pi council model for %s changed from %s to %s", entry.MemberID, configuredModel, model)
		}
		return home, configuredModel, nil
	}
	s.trackSecretFile(filepath.Join(home, ".mcp.json"))
	s.trackSecretFile(filepath.Join(home, ".pi", "agent", "auth.json"))
	configuredModel, err := writePiConfig(home, entry, server, mcpURL, capability)
	if err != nil {
		return "", "", err
	}
	s.councilModels[entry.MemberID] = configuredModel
	return home, configuredModel, nil
}

func councilProcessName(memberID string, opportunityID string) string {
	digest := sha256.Sum256([]byte(opportunityID))
	return "pi-" + memberID + "-" + hex.EncodeToString(digest[:6])
}

func piContainerName(caseID string, memberID string, opportunityID string) string {
	return containerName("aard-" + caseID + "-" + councilProcessName(memberID, opportunityID))
}

func piRunArgs(opts Options, name string, home string, model string, instructions string) []string {
	return []string{
		"run", "--rm",
		"--name", name,
		"--network", "host",
		"--user", "0:0",
		"-e", "HOME=/home/user",
		"-e", "TMPDIR=/home/user",
		"-e", "PI_CODING_AGENT_DIR=/home/user/.pi/agent",
		"-e", "OPENROUTER_API_KEY",
		"-e", "NODE_OPTIONS",
		"-v", home + ":/home/user",
		"-w", "/home/user",
		opts.PiImage,
		"--provider", "openrouter",
		"--model", model,
		"-e", opts.PiMCPAdapter,
		"--mode", "json",
		"-p", instructions,
	}
}

func outputSubdir(outputDir string, name string) (string, error) {
	output, err := filepath.Abs(outputDir)
	if err != nil {
		return "", err
	}
	subdir, err := filepath.Abs(filepath.Join(output, name))
	if err != nil {
		return "", err
	}
	if filepath.Dir(subdir) != output {
		return "", fmt.Errorf("output subdirectory must be a direct child of output dir")
	}
	return subdir, nil
}

func createOutputSubdir(outputDir string, name string, mode os.FileMode) (string, error) {
	subdir, err := outputSubdir(outputDir, name)
	if err != nil {
		return "", err
	}
	if err := os.Mkdir(subdir, mode); err != nil {
		return "", err
	}
	info, err := os.Lstat(subdir)
	if err != nil {
		return "", fmt.Errorf("inspect created output subdirectory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("created output subdirectory is not a regular directory")
	}
	resolvedOutput, err := filepath.EvalSymlinks(filepath.Dir(subdir))
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	resolvedSubdir, err := filepath.EvalSymlinks(subdir)
	if err != nil {
		return "", fmt.Errorf("resolve created output subdirectory: %w", err)
	}
	if resolvedSubdir != filepath.Join(resolvedOutput, filepath.Base(subdir)) {
		return "", fmt.Errorf("created output subdirectory resolves outside output dir")
	}
	return subdir, nil
}

func validateCouncilMemberID(memberID string) error {
	if memberID == "" {
		return fmt.Errorf("council roster entry has empty member_id")
	}
	if memberID == "." || memberID == ".." {
		return fmt.Errorf("council member_id %q is invalid", memberID)
	}
	for _, r := range memberID {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return fmt.Errorf("council member_id contains invalid character %q", r)
	}
	return nil
}

func (s *runState) renderLauncherPrompt(id string, data instructionData) (string, error) {
	return s.launcherPrompts.Render(id, instructionValues(data))
}

func instructionValues(data instructionData) map[string]string {
	return map[string]string{
		"{{CASE_ID}}":             data.CaseID,
		"{{ROLE_ID}}":             data.RoleID,
		"{{MEMBER_ID}}":           data.MemberID,
		"{{MCP_SERVER}}":          data.MCPServer,
		"{{MCP_URL}}":             data.MCPURL,
		"{{MCP_JSON}}":            data.MCPJSON,
		"{{WORKSPACE}}":           data.Workspace,
		"{{SEARCH_INSTRUCTIONS}}": data.SearchInstructions,
	}
}

func writePiConfig(home string, entry councilRosterEntry, server string, mcpURL string, capability string) (string, error) {
	spec, model, err := validatedPiRequest(entry)
	if err != nil {
		return "", err
	}
	settingsDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return "", fmt.Errorf("create Pi settings dir: %w", err)
	}
	settings := map[string]any{
		"defaultProvider": "openrouter",
		"defaultModel":    model,
		"quietStartup":    true,
	}
	if err := writeJSONFile(filepath.Join(settingsDir, "settings.json"), settings); err != nil {
		return "", err
	}
	modelEntry := map[string]any{
		"id":   model,
		"name": "AARD " + entry.MemberID + " " + model,
	}
	spec = spec.WithFallbackMaxOutputTokens(DefaultCouncilMaxOutputTokens)
	if maxTokens := spec.MaxOutputTokens(); maxTokens != nil {
		modelEntry["maxTokens"] = *maxTokens
	}
	samplingParams := map[string]any{}
	if spec.Request.Temperature != nil {
		samplingParams["temperature"] = *spec.Request.Temperature
	}
	if spec.Request.TopP != nil {
		samplingParams["top_p"] = *spec.Request.TopP
	}
	if len(samplingParams) > 0 {
		modelEntry["samplingParams"] = samplingParams
	}
	if cost := pimodel.OpenRouterCost(spec.VariantMetadata); len(cost) > 0 {
		modelEntry["cost"] = cost
	}
	if compat := pimodel.OpenRouterCompat(spec.ProviderBody(), spec.VariantMetadata); len(compat) > 0 {
		modelEntry["compat"] = compat
	}
	providerEntry := map[string]any{
		"baseUrl": "https://openrouter.ai/api/v1",
		"apiKey":  "$OPENROUTER_API_KEY",
		"api":     "openai-completions",
		"models":  []map[string]any{modelEntry},
	}
	if len(spec.Headers) > 0 {
		providerEntry["headers"] = spec.Headers
	}
	models := map[string]any{
		"providers": map[string]any{
			"openrouter": providerEntry,
		},
	}
	if err := writeJSONFile(filepath.Join(settingsDir, "models.json"), models); err != nil {
		return "", err
	}
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			server: map[string]any{
				"url":       mcpURL,
				"transport": "streamable-http",
				"lifecycle": "keep-alive",
				"headers":   map[string]string{"Authorization": "Bearer " + capability},
			},
		},
	}
	if err := writeJSONFileMode(filepath.Join(home, ".mcp.json"), mcpConfig, 0o600); err != nil {
		return "", err
	}
	return model, nil
}

func validatedPiRequest(entry councilRosterEntry) (modelrequest.Spec, string, error) {
	spec, err := piRequestSpec(entry)
	if err != nil {
		return modelrequest.Spec{}, "", err
	}
	if spec.Endpoint != "openrouter" {
		return modelrequest.Spec{}, "", fmt.Errorf("Pi council requires openrouter endpoint for %s; got %s", entry.MemberID, spec.Endpoint)
	}
	model := spec.UpstreamModel()
	return spec, model, nil
}

func piRequestSpec(entry councilRosterEntry) (modelrequest.Spec, error) {
	if entry.RequestSpec != nil {
		return *entry.RequestSpec, nil
	}
	return modelrequest.Spec{}, fmt.Errorf("council roster entry %s has no request_spec; JSONL council pool records are required", entry.MemberID)
}

func writeJSONFile(path string, value any) error {
	return writeJSONFileMode(path, value, 0o644)
}

func writeJSONFileMode(path string, value any, mode os.FileMode) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

func (s *runState) startProcess(ctx context.Context, name string, kind string, command string, args []string, environment []string, container string, lawyerRole string, councilTarget *councilProcessTarget) (*processRecord, error) {
	stdoutPath := filepath.Join(s.logDir, name+".stdout")
	stderrPath := filepath.Join(s.logDir, name+".stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("create %s stdout log: %w", name, err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		createErr := fmt.Errorf("create %s stderr log: %w", name, err)
		if closeErr := stdout.Close(); closeErr != nil {
			createErr = errors.Join(createErr, fmt.Errorf("close %s stdout log: %w", name, closeErr))
		}
		return nil, createErr
	}
	stdoutWriter := io.Writer(stdout)
	var stdoutFilter *piTailLogWriter
	if councilTarget != nil && strings.HasPrefix(name, "pi-") {
		stdoutFilter = newPiTailLogWriter(stdout)
		stdoutWriter = stdoutFilter
	}
	stdoutCounter := newProcessOutputCounter(stdoutWriter)
	closeStdout := func() error {
		if stdoutFilter == nil {
			return stdout.Close()
		}
		return errors.Join(stdoutFilter.Flush(), stdout.Close())
	}
	containerIDPath, containerIDDir, err := createContainerIDPath(s.logDir, container)
	if err != nil {
		return nil, errors.Join(err, closeStdout(), stderr.Close())
	}
	args, err = addContainerIDPath(args, containerIDPath)
	if err != nil {
		return nil, errors.Join(err, cleanupContainerIDPath(containerIDPath, containerIDDir), closeStdout(), stderr.Close())
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdout = stdoutCounter
	cmd.Stderr = stderr
	cmd.Env = append([]string(nil), environment...)
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(fmt.Errorf("start %s: %w", name, err), cleanupContainerIDPath(containerIDPath, containerIDDir), closeStdout(), stderr.Close())
	}
	role := strings.TrimSpace(lawyerRole)
	if role == "" && councilTarget != nil {
		role = councilTarget.memberID
	}
	finishProcess, err := runstate.Start(s.opts.ProcessObserver, runstate.ProcessStart{
		Name:      name,
		Kind:      kind,
		Role:      role,
		PID:       cmd.Process.Pid,
		StartedAt: time.Now().UTC(),
	})
	if err != nil {
		observerRecord := &processRecord{name: name, stopCommand: command, environment: append([]string(nil), environment...), containerIDPath: containerIDPath}
		containerFound, containerErr := removeProcessContainer(observerRecord)
		killErr := cmd.Process.Kill()
		waitErr := cmd.Wait()
		if !containerFound {
			var retryErr error
			containerFound, retryErr = removeProcessContainer(observerRecord)
			containerErr = errors.Join(containerErr, retryErr)
			if !containerFound {
				containerErr = errors.Join(containerErr, fmt.Errorf("container ID for %s was not recorded before the runtime client exited", name))
			}
		}
		containerIDErr := cleanupContainerIDPath(containerIDPath, containerIDDir)
		return nil, errors.Join(fmt.Errorf("record %s process start: %w", name, err), killErr, waitErr, containerErr, containerIDErr, closeStdout(), stderr.Close())
	}
	var ownedCouncilTarget *councilProcessTarget
	if councilTarget != nil {
		target := *councilTarget
		ownedCouncilTarget = &target
	}
	record := &processRecord{
		name:            name,
		kind:            kind,
		command:         cmd,
		done:            make(chan processExit, 1),
		stopCommand:     command,
		environment:     append([]string(nil), environment...),
		containerIDPath: containerIDPath,
		containerIDDir:  containerIDDir,
		councilTarget:   ownedCouncilTarget,
		stdoutPath:      stdoutPath,
		stderrPath:      stderrPath,
		finished:        make(chan struct{}),

		stdoutCounter: stdoutCounter,
	}
	go func() {
		waitErr := cmd.Wait()
		stdoutErr := closeStdout()
		stderrErr := stderr.Close()
		exit := processExit{
			waitErr:   waitErr,
			stdoutErr: stdoutErr,
			stderrErr: stderrErr,
			canceled:  ctx.Err() != nil,
		}
		processErr := exit.err()
		state := runstate.Completed
		if ctx.Err() != nil {
			state = runstate.Canceled
		} else if processErr != nil {
			state = runstate.Failed
		}
		var exitCode *int
		if cmd.ProcessState != nil {
			value := cmd.ProcessState.ExitCode()
			exitCode = &value
		}
		exit.recordErr = runstate.Finish(finishProcess, runstate.ProcessFinish{
			State:      state,
			FinishedAt: time.Now().UTC(),
			ExitCode:   exitCode,
			Error:      runstate.ErrorText(processErr),
		})
		processErr = exit.err()
		record.markExited()
		record.done <- exit
		if ctx.Err() != nil {
			return
		}
		if record.councilTarget != nil {
			if err := s.handleCouncilProcessExit(ctx, record, *record.councilTarget, processErr); err != nil {
				s.agentErrs <- err
			}
			return
		}
		if processErr != nil {
			s.agentErrs <- fmt.Errorf("%s process %s failed: %w", kind, name, processErr)
			return
		}
		if strings.TrimSpace(lawyerRole) != "" {
			if err := s.handleLawyerExit(ctx, lawyerRole, name); err != nil {
				s.agentErrs <- err
			}
			return
		}
		s.agentErrs <- fmt.Errorf("%s process %s exited before case completion", kind, name)
	}()
	if record.councilTarget != nil {
		go s.monitorCouncilOutput(ctx, record, *record.councilTarget, defaultCouncilOutputCheck)
	}
	return record, nil
}

func (p *processRecord) markExited() {
	p.mu.Lock()
	p.exited = true
	close(p.finished)
	p.mu.Unlock()
}

func (p *processRecord) isExited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

func (p *processRecord) setForcedFailure(reason string, message string, details map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.forcedReason != "" {
		return
	}
	p.forcedReason = strings.TrimSpace(reason)
	p.forcedMessage = strings.TrimSpace(message)
	p.forcedDetails = cloneLocalMap(details)
}

func (p *processRecord) forcedFailure() (string, string, map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.forcedReason, p.forcedMessage, cloneLocalMap(p.forcedDetails)
}

func (s *runState) monitorCouncilOutput(ctx context.Context, proc *processRecord, target councilProcessTarget, interval time.Duration) {
	if s.opts.CouncilOutputLimitBytes <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if proc.isExited() {
				return
			}
			size, err := councilProcessOutputSize(proc)
			if err != nil {
				s.agentErrs <- fmt.Errorf("check council output for %s: %w", proc.name, err)
				return
			}
			if size.Total <= s.opts.CouncilOutputLimitBytes {
				continue
			}
			message, details := councilOutputLimitFailure(proc.name, target, size, s.opts.CouncilOutputLimitBytes)
			proc.setForcedFailure(councilFailureOutputLimit, message, details)
			var stopErr error
			if strings.TrimSpace(proc.containerIDPath) != "" {
				containerFound, err := removeProcessContainer(proc)
				stopErr = err
				if containerFound && stopErr == nil {
					select {
					case <-proc.finished:
						return
					case <-time.After(250 * time.Millisecond):
					}
				}
			}
			if err := proc.command.Process.Kill(); err != nil {
				if proc.isExited() {
					return
				}
				s.agentErrs <- errors.Join(stopErr, fmt.Errorf("kill %s after council output limit exceeded: %w", proc.name, err))
				return
			}
			if stopErr != nil {
				s.agentErrs <- stopErr
			}
			return
		case <-proc.finished:
			return
		case <-ctx.Done():
			return
		}
	}
}

func councilProcessOutputSize(proc *processRecord) (processOutputSize, error) {
	stdoutInfo, err := os.Stat(proc.stdoutPath)
	if err != nil {
		return processOutputSize{}, fmt.Errorf("stat stdout log %s: %w", proc.stdoutPath, err)
	}
	stderrInfo, err := os.Stat(proc.stderrPath)
	if err != nil {
		return processOutputSize{}, fmt.Errorf("stat stderr log %s: %w", proc.stderrPath, err)
	}
	stdoutBytes := stdoutInfo.Size()
	stderrBytes := stderrInfo.Size()
	if proc.stdoutCounter != nil {
		stdoutBytes = proc.stdoutCounter.Size()
	}
	if stdoutBytes > int64(^uint64(0)>>1)-stderrBytes {
		return processOutputSize{}, fmt.Errorf("process output size overflow for %s", proc.name)
	}
	return processOutputSize{
		Stdout: stdoutBytes,
		Stderr: stderrBytes,
		Total:  stdoutBytes + stderrBytes,
	}, nil
}

func councilOutputLimitFailure(procName string, target councilProcessTarget, size processOutputSize, limit int64) (string, map[string]any) {
	message := fmt.Sprintf(
		"Council member %s agent process exceeded the output limit before completing opportunity %s: %d bytes written, limit %d bytes.",
		target.memberID,
		target.opportunityID,
		size.Total,
		limit,
	)
	return message, map[string]any{
		"process_name":       procName,
		"output_bytes":       size.Total,
		"stdout_bytes":       size.Stdout,
		"stderr_bytes":       size.Stderr,
		"output_limit_bytes": limit,
	}
}

func cloneLocalMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		if strings.TrimSpace(key) != "" && value != nil {
			out[key] = value
		}
	}
	return out
}

func (s *runState) handleCouncilProcessExit(ctx context.Context, proc *processRecord, target councilProcessTarget, waitErr error) error {
	status, err := s.councilStatus(ctx, target.memberID)
	if err != nil {
		return fmt.Errorf("check council status after %s exit: %w", proc.name, err)
	}
	if strings.TrimSpace(status.Status) != "ready" || status.Turn == nil {
		return nil
	}
	if strings.TrimSpace(status.Turn.MemberID) != strings.TrimSpace(target.memberID) {
		return nil
	}
	if strings.TrimSpace(status.Turn.OpportunityID) != strings.TrimSpace(target.opportunityID) {
		return nil
	}
	reason, forcedMessage, forcedDetails := proc.forcedFailure()
	if reason == "" {
		reason = councilFailureAgentExited
	}
	message := fmt.Sprintf("Council member %s agent process exited before completing opportunity %s.", target.memberID, target.opportunityID)
	details := map[string]any{
		"process_name": proc.name,
	}
	for key, value := range forcedDetails {
		details[key] = value
	}
	if forcedMessage != "" {
		message = forcedMessage
	}
	if waitErr != nil {
		if forcedMessage == "" {
			message = fmt.Sprintf("Council member %s agent process failed before completing opportunity %s: %s.", target.memberID, target.opportunityID, waitErr.Error())
		}
		details["process_error"] = waitErr.Error()
	}
	return s.reportCouncilFailure(ctx, target.memberID, target.opportunityID, reason, message, details)
}

func (s *runState) reportCouncilFailure(ctx context.Context, memberID string, opportunityID string, reason string, message string, details map[string]any) (err error) {
	payload := map[string]any{
		"case_id":        s.opts.CaseID,
		"member_id":      memberID,
		"opportunity_id": opportunityID,
		"reason":         reason,
		"message":        message,
		"details":        details,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	failURL := s.caseBase + "/councilapi/v1/fail"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, failURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("report council failure for %s: %w", memberID, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close council failure response: %w", closeErr))
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("report council failure for %s returned HTTP %d: %s", memberID, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var response map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&response); err != nil {
		return fmt.Errorf("decode council failure response for %s: %w", memberID, err)
	}
	if ok, _ := response["ok"].(bool); !ok {
		message := ""
		if errObj, ok := response["error"].(map[string]any); ok {
			message = fmt.Sprint(errObj["message"])
		}
		if strings.TrimSpace(message) == "" {
			message = strings.TrimSpace(string(body))
		}
		return fmt.Errorf("report council failure for %s was rejected: %s", memberID, message)
	}
	return nil
}

func (s *runState) stopAgents() error {
	s.mu.Lock()
	processes := append([]*processRecord{}, s.processes...)
	s.mu.Unlock()
	var errs []error
	for _, proc := range processes {
		if strings.TrimSpace(proc.containerIDPath) != "" {
			if err := stopContainerProcess(proc); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if proc.command.Process == nil {
			continue
		}
		if proc.isExited() {
			if err := collectProcessFinalization(proc); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := proc.command.Process.Kill(); err != nil {
			errs = append(errs, fmt.Errorf("kill %s: %w", proc.name, err))
			continue
		}
		exit, err := waitProcessExit(proc, agentStopWait)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if finalizationErr := exit.finalizationErr(); finalizationErr != nil {
			errs = append(errs, fmt.Errorf("finalize %s: %w", proc.name, finalizationErr))
		}
	}
	return errors.Join(errs...)
}

func (s *runState) stopPriorCouncilProcesses(memberID string, opportunityID string) error {
	memberID = strings.TrimSpace(memberID)
	opportunityID = strings.TrimSpace(opportunityID)
	s.mu.Lock()
	var prior []*processRecord
	for _, proc := range s.processes {
		if proc.councilTarget == nil {
			continue
		}
		if strings.TrimSpace(proc.councilTarget.memberID) != memberID {
			continue
		}
		if strings.TrimSpace(proc.councilTarget.opportunityID) == opportunityID {
			continue
		}
		prior = append(prior, proc)
	}
	s.mu.Unlock()

	var errs []error
	for _, proc := range prior {
		stopErr := stopContainerProcess(proc)
		if proc.isExited() {
			s.removeProcess(proc)
		} else {
			stopErr = errors.Join(stopErr, fmt.Errorf("council process %s did not exit", proc.name))
		}
		if stopErr != nil {
			errs = append(errs, fmt.Errorf("stop prior council process %s: %w", proc.name, stopErr))
		}
	}
	return errors.Join(errs...)
}

func (s *runState) removeProcess(target *processRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, proc := range s.processes {
		if proc != target {
			continue
		}
		copy(s.processes[i:], s.processes[i+1:])
		s.processes[len(s.processes)-1] = nil
		s.processes = s.processes[:len(s.processes)-1]
		return
	}
}

func stopContainerProcess(proc *processRecord) error {
	alreadyExited := proc.isExited()
	containerFound, removeErr := removeProcessContainer(proc)
	if proc.command == nil || proc.command.Process == nil {
		return errors.Join(removeErr, cleanupProcessContainerID(proc))
	}
	var killErr error
	if (!containerFound || removeErr != nil) && !alreadyExited {
		if err := proc.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			killErr = fmt.Errorf("kill %s after container ownership lookup or removal failure: %w", proc.name, err)
		}
	}
	exit, waitErr := waitProcessExit(proc, agentStopWait)
	if !containerFound {
		var retryErr error
		containerFound, retryErr = removeProcessContainer(proc)
		removeErr = errors.Join(removeErr, retryErr)
		if !containerFound {
			removeErr = errors.Join(removeErr, fmt.Errorf("container ID for %s was not recorded before the runtime client exited", proc.name))
		}
	}
	cleanupErr := cleanupProcessContainerID(proc)
	if waitErr != nil {
		return errors.Join(removeErr, killErr, waitErr, cleanupErr)
	}
	forcedReason, _, _ := proc.forcedFailure()
	if alreadyExited && !exit.canceled && forcedReason == "" {
		if processErr := exit.err(); processErr != nil {
			return errors.Join(removeErr, killErr, fmt.Errorf("process %s exited: %w", proc.name, processErr), cleanupErr)
		}
		return errors.Join(removeErr, killErr, cleanupErr)
	}
	if finalizationErr := exit.finalizationErr(); finalizationErr != nil {
		return errors.Join(removeErr, killErr, fmt.Errorf("finalize %s: %w", proc.name, finalizationErr), cleanupErr)
	}
	return errors.Join(removeErr, killErr, cleanupErr)
}

func collectProcessFinalization(proc *processRecord) error {
	exit, err := waitProcessExit(proc, agentStopWait)
	if err != nil {
		return err
	}
	if processErr := exit.err(); processErr != nil {
		return fmt.Errorf("process %s exited: %w", proc.name, processErr)
	}
	return nil
}

func removeProcessContainer(proc *processRecord) (bool, error) {
	containerID, found, err := readProcessContainerID(proc)
	if err != nil || !found {
		return found, err
	}
	command := strings.TrimSpace(proc.stopCommand)
	if command == "" {
		return true, fmt.Errorf("container runtime command is empty for %s", proc.name)
	}
	remove := exec.Command(command, "container", "rm", "-f", containerID)
	remove.Env = append([]string(nil), proc.environment...)
	out, err := remove.CombinedOutput()
	if err == nil {
		return true, nil
	}
	message := strings.TrimSpace(string(out))
	if containerRemovalMissingOutput(message) {
		return true, nil
	}
	if message != "" {
		return true, fmt.Errorf("%s container rm -f %s: %w: %s", command, containerID, err, message)
	}
	return true, fmt.Errorf("%s container rm -f %s: %w", command, containerID, err)
}

func readProcessContainerID(proc *processRecord) (string, bool, error) {
	path := strings.TrimSpace(proc.containerIDPath)
	if path == "" {
		return "", false, fmt.Errorf("container ID path is empty for %s", proc.name)
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read container ID for %s: %w", proc.name, err)
	}
	containerID := strings.TrimSpace(string(raw))
	if !canonicalContainerID(containerID) {
		return "", false, fmt.Errorf("container ID for %s is not a 64-character lowercase hexadecimal value", proc.name)
	}
	return containerID, true, nil
}

func canonicalContainerID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func createContainerIDPath(logDir string, identity string) (string, string, error) {
	dir, err := os.MkdirTemp(logDir, "."+containerName(identity)+"-container-")
	if err != nil {
		return "", "", fmt.Errorf("reserve container ID path for %s: %w", identity, err)
	}
	return filepath.Join(dir, "cid"), dir, nil
}

func addContainerIDPath(args []string, path string) ([]string, error) {
	if len(args) == 0 || args[0] != "run" {
		return nil, fmt.Errorf("container command must begin with run")
	}
	withID := make([]string, 0, len(args)+2)
	withID = append(withID, args[0], "--cidfile", path)
	withID = append(withID, args[1:]...)
	return withID, nil
}

func cleanupProcessContainerID(proc *processRecord) error {
	return cleanupContainerIDPath(proc.containerIDPath, proc.containerIDDir)
}

func cleanupContainerIDPath(path string, dir string) error {
	path = strings.TrimSpace(path)
	dir = strings.TrimSpace(dir)
	if path == "" || dir == "" || filepath.Dir(path) != dir {
		return fmt.Errorf("container ID path ownership is invalid")
	}
	var errs []error
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove container ID file %s: %w", path, err))
	}
	if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		errs = append(errs, fmt.Errorf("remove container ID directory %s: %w", dir, err))
	}
	return errors.Join(errs...)
}

func containerRemovalMissingOutput(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "no such container") ||
		strings.Contains(message, "no container with name or id") ||
		strings.Contains(message, "does not exist")
}

func waitProcessExit(proc *processRecord, timeout time.Duration) (processExit, error) {
	select {
	case exit := <-proc.done:
		return exit, nil
	case <-time.After(timeout):
		return processExit{}, fmt.Errorf("timed out waiting for %s process %s to exit after stop", proc.kind, proc.name)
	}
}

func (s *runState) cleanupSecrets() error {
	s.mu.Lock()
	files := append([]string{}, s.secretFiles...)
	s.mu.Unlock()
	var errs []error
	for _, path := range files {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove staged secret file %s: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func (s *runState) trackSecretFile(path string) {
	s.mu.Lock()
	s.secretFiles = append(s.secretFiles, path)
	s.mu.Unlock()
}

func writeRunSummary(outDir string, result Result, opts Options) error {
	return writeExclusiveJSON(filepath.Join(outDir, "local-run.json"), map[string]any{
		"case_id":                             result.CaseID,
		"run_id":                              result.RunID,
		"status":                              result.Status,
		"phase":                               result.Phase,
		"answers":                             result.Answers,
		"error":                               result.Error,
		"failure":                             result.Failure,
		"auto_lawyers":                        opts.AutoLawyers,
		"mcp_public_base_url":                 opts.MCPPublicBaseURL,
		"openclaw_lawyer_start_delay_seconds": opts.OpenClawStartDelaySeconds,
		"council_output_limit_bytes":          opts.CouncilOutputLimitBytes,
	})
}

func writeExclusiveJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	return publishExclusiveFile(path, append(raw, '\n'), 0o644)
}

func publishExclusiveFile(path string, raw []byte, mode os.FileMode) (returnErr error) {
	token, err := randomToken()
	if err != nil {
		return fmt.Errorf("create temporary run summary name: %w", err)
	}
	temporaryPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+"."+token+".tmp")
	temporary, err := os.OpenFile(temporaryPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return fmt.Errorf("create temporary run summary: %w", err)
	}
	open := true
	removeTemporary := true
	defer func() {
		if open {
			if closeErr := temporary.Close(); closeErr != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("close temporary run summary: %w", closeErr))
			}
		}
		if removeTemporary {
			if removeErr := os.Remove(temporaryPath); removeErr != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary run summary: %w", removeErr))
			}
		}
	}()
	n, err := temporary.Write(raw)
	if err != nil {
		return fmt.Errorf("write temporary run summary: %w", err)
	}
	if n != len(raw) {
		return fmt.Errorf("write temporary run summary: %w", io.ErrShortWrite)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync temporary run summary: %w", err)
	}
	closeErr := temporary.Close()
	open = false
	if closeErr != nil {
		return fmt.Errorf("close temporary run summary: %w", closeErr)
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return fmt.Errorf("publish run summary: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove temporary run summary after publication: %w", err)
	}
	removeTemporary = false
	return nil
}

func environmentValue(environment []string, name string) (string, bool) {
	for index := len(environment) - 1; index >= 0; index-- {
		key, value, ok := strings.Cut(environment[index], "=")
		if ok && key == name {
			return value, true
		}
	}
	return "", false
}

func validEnvironmentVariableName(value string) bool {
	if value == "" {
		return false
	}
	for index, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '_' || (index > 0 && character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func randomToken() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "aard-" + hex.EncodeToString(buf[:]), nil
}

func containerName(value string) string {
	value = strings.ToLower(value)
	digest := sha256.Sum256([]byte(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-_.")
	if len(out) > 63 {
		suffix := "-" + hex.EncodeToString(digest[:6])
		out = strings.Trim(out[:63-len(suffix)], "-_.") + suffix
	}
	if out == "" {
		return "aard"
	}
	return out
}
