// Package lawyer starts and supervises lawyer agents for one case.
package lawyer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	headless "github.com/agentcourt/adj/runtime/agent"
	"github.com/agentcourt/adj/runtime/runstate"
)

const (
	openClawCodexContainerHome = "/aar-codex"
	openClawContainerState     = "/home/node/.openclaw"
	openClawContainerConfig    = "/home/node/.openclaw/openclaw.json"
	openClawCleanupWorkspace   = "/aar-retained-work"
	OpenClawWorkspacePath      = "/home/node/work"
	OpenClawEvidencePath       = "/home/node/evidence"
	piContainerState           = "/home/user/state"
	PiWorkspacePath            = "/home/user/work"
	PiEvidencePath             = "/home/user/evidence"
	stopWait                   = 30 * time.Second
)

var (
	mcpNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

type Runner string

const (
	RunnerOpenClaw Runner = "openclaw"
	RunnerPi       Runner = "pi"
	RunnerCodex    Runner = "codex"
	RunnerClaude   Runner = "claude"
)

type OpenClawAuthMode string

const (
	OpenClawAuthCodex  OpenClawAuthMode = "codex"
	OpenClawAuthAPIKey OpenClawAuthMode = "api-key"
)

type OpenClawAuth struct {
	Mode          OpenClawAuthMode
	CodexAuthPath string
	APIKeyEnv     string
}

type OpenClawProfile struct {
	Command                  string
	Image                    string
	Provider                 string
	Model                    string
	Thinking                 string
	AgentTimeoutSeconds      int
	LawyerTurnTimeoutSeconds int
	Network                  string
	Auth                     OpenClawAuth
}

type Profile struct {
	Runner   Runner
	Headless headless.Profile
	OpenClaw OpenClawProfile
}

type Runtime struct {
	OutputDir       string
	LogsDir         string
	ContainerPrefix string
	PodmanCommand   string
	PiImage         string
	BaseEnvironment []string
	Observer        runstate.Observer
}

type Assignment struct {
	CaseID      string
	RunID       string
	RoleID      string
	Profile     string
	Prompt      string
	MCP         headless.MCPServer
	MCPCommand  string
	WebSearch   *bool
	Environment []string
	StateDir    string
	WorkDir     string
	EvidenceDir string
	VerifyExit  func(context.Context, string, string) error
}

type Supervisor struct {
	runtime Runtime

	startStopMu sync.Mutex
	mu          sync.Mutex
	processes   []*processRecord
	errors      chan error
	stopped     bool
	stopOnce    sync.Once
	stopErr     error
}

type processRecord struct {
	name          string
	kind          string
	command       *exec.Cmd
	containerStop *containerStop
	finished      chan struct{}

	mu     sync.Mutex
	exited bool
	exit   processExit
}

type containerStop struct {
	command     string
	name        string
	processName string
	done        chan struct{}
	once        sync.Once
	mu          sync.Mutex
	started     bool
	err         error
}

type processExit struct {
	terminalErr    error
	waitErr        error
	stopErr        error
	stdoutErr      error
	stderrErr      error
	cleanupErr     error
	statusErr      error
	markerErr      error
	participantErr error
	recordErr      error
	canceled       bool
}

func (e processExit) err() error {
	return errors.Join(e.terminalErr, e.waitErr, e.stopErr, e.stdoutErr, e.stderrErr, e.cleanupErr, e.statusErr, e.markerErr, e.participantErr, e.recordErr)
}

func (e processExit) finalizationErr() error {
	return errors.Join(e.terminalErr, e.stopErr, e.stdoutErr, e.stderrErr, e.cleanupErr, e.statusErr, e.markerErr, e.participantErr, e.recordErr)
}

type processStartOptions struct {
	env              []string
	dir              string
	roleID           string
	verifyExit       func(context.Context, string, string) error
	afterExit        func() error
	cleanup          func() error
	markSuccessful   func() error
	markUnsuccessful func() error
	participant      *runstate.ParticipantStart
	readUsage        func(string) (*runstate.TokenUsage, error)
	stateDir         string
}

func New(runtime Runtime) (*Supervisor, error) {
	runtime.OutputDir = strings.TrimSpace(runtime.OutputDir)
	runtime.LogsDir = strings.TrimSpace(runtime.LogsDir)
	runtime.ContainerPrefix = strings.TrimSpace(runtime.ContainerPrefix)
	if runtime.OutputDir == "" || runtime.LogsDir == "" {
		return nil, errors.New("lawyer output and log directories are required")
	}
	if runtime.ContainerPrefix == "" {
		return nil, errors.New("lawyer container prefix is required")
	}
	if runtime.BaseEnvironment == nil {
		runtime.BaseEnvironment = os.Environ()
	} else {
		runtime.BaseEnvironment = append([]string(nil), runtime.BaseEnvironment...)
	}
	return &Supervisor{runtime: runtime, errors: make(chan error, 16)}, nil
}

func (s *Supervisor) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errors
}

func (s *Supervisor) Start(ctx context.Context, profile Profile, assignment Assignment) error {
	if s == nil {
		return errors.New("lawyer supervisor is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateAssignment(assignment); err != nil {
		return err
	}

	s.startStopMu.Lock()
	defer s.startStopMu.Unlock()
	if s.stopped {
		return errors.New("lawyer supervisor is stopped")
	}

	var (
		process *processRecord
		err     error
	)
	switch profile.Runner {
	case RunnerOpenClaw:
		process, err = s.startOpenClaw(ctx, profile.OpenClaw, assignment)
	case RunnerPi, RunnerCodex, RunnerClaude:
		process, err = s.startHeadless(ctx, profile, assignment)
	default:
		return fmt.Errorf("unsupported lawyer runner %q", profile.Runner)
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.processes = append(s.processes, process)
	s.mu.Unlock()
	return nil
}

func validateAssignment(assignment Assignment) error {
	if strings.TrimSpace(assignment.CaseID) == "" || strings.TrimSpace(assignment.RoleID) == "" {
		return errors.New("lawyer case and role IDs are required")
	}
	if strings.TrimSpace(assignment.Prompt) == "" {
		return errors.New("lawyer prompt is required")
	}
	if !mcpNamePattern.MatchString(assignment.MCP.Name) {
		return fmt.Errorf("MCP server name %q must contain only letters, digits, underscores, or hyphens", assignment.MCP.Name)
	}
	parsed, err := url.Parse(assignment.MCP.URL)
	if err != nil {
		return fmt.Errorf("parse MCP server URL %q: %w", assignment.MCP.URL, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("MCP server URL %q must be an absolute HTTP or HTTPS URL", assignment.MCP.URL)
	}
	if parsed.User != nil {
		return fmt.Errorf("MCP server URL %q must not contain user information", assignment.MCP.URL)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("MCP server URL %q must not contain a fragment", assignment.MCP.URL)
	}
	if assignment.VerifyExit == nil {
		return errors.New("lawyer exit verifier is required")
	}
	return nil
}

func participantProfileName(name string, runner Runner) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	return string(runner)
}

func (s *Supervisor) startHeadless(ctx context.Context, profile Profile, assignment Assignment) (process *processRecord, returnErr error) {
	wantRunner := headless.Runner(profile.Runner)
	if profile.Headless.Runner != wantRunner {
		return nil, fmt.Errorf("headless profile runner is %q; expected %q", profile.Headless.Runner, wantRunner)
	}
	stateDir, workDir, evidenceDir, err := s.resolveAssignmentDirs(assignment)
	if err != nil {
		return nil, err
	}

	baseEnvironment := s.assignmentEnvironment(assignment)
	invocation, err := headless.Prepare(profile.Headless, headless.Assignment{
		StateDir:    stateDir,
		WorkDir:     workDir,
		EvidenceDir: evidenceDir,
		Prompt:      assignment.Prompt,
		MCP:         assignment.MCP,
		WebSearch:   assignment.WebSearch,
	}, baseEnvironment)
	if err != nil {
		return nil, fmt.Errorf("prepare %s %s lawyer: %w", assignment.RoleID, profile.Runner, err)
	}
	started := false
	defer func() {
		if !started {
			returnErr = errors.Join(returnErr, invocation.Cleanup())
		}
	}()

	command := invocation.Command
	args := invocation.Args
	processEnv := invocation.Env
	processDir := invocation.Dir
	stopName := ""
	kind := string(profile.Runner)
	if profile.Runner == RunnerPi {
		if strings.TrimSpace(s.runtime.PodmanCommand) == "" || strings.TrimSpace(s.runtime.PiImage) == "" {
			return nil, errors.New("Pi lawyer requires a container command and image")
		}
		name := safeName(s.runtime.ContainerPrefix + "-" + assignment.CaseID + "-" + assignment.RoleID + "-pi")
		args, processEnv, err = piContainer(invocation, name, s.runtime.PiImage, baseEnvironment)
		if err != nil {
			return nil, err
		}
		command = s.runtime.PodmanCommand
		processDir = ""
		stopName = name
		kind = "podman"
	}
	process, err = s.startProcess(ctx, string(profile.Runner)+"-"+assignment.RoleID, kind, command, args, stopName, processStartOptions{
		env:              processEnv,
		dir:              processDir,
		roleID:           assignment.RoleID,
		verifyExit:       assignment.VerifyExit,
		cleanup:          invocation.Cleanup,
		markSuccessful:   invocation.MarkSuccessful,
		markUnsuccessful: invocation.MarkUnsuccessful,
		participant: &runstate.ParticipantStart{
			Role:            assignment.RoleID,
			Profile:         participantProfileName(assignment.Profile, profile.Runner),
			Runner:          string(profile.Runner),
			ReasoningEffort: strings.TrimSpace(profile.Headless.ReasoningEffort),
			Resumed:         invocation.Resumed,
			StateDir:        invocation.StateDir,
			WorkDir:         workDir,
			WebSearch:       webSearchObservation(profile, assignment.WebSearch),
		},
		readUsage: func(path string) (*runstate.TokenUsage, error) {
			usage, err := invocation.FinishUsage(profile.Headless.Runner, path)
			var refusal *headless.ProviderRefusalError
			if errors.As(err, &refusal) {
				if strings.TrimSpace(refusal.Provider) == "" {
					refusal.Provider = string(profile.Headless.Provider)
				}
				if strings.TrimSpace(refusal.Model) == "" {
					refusal.Model = profile.Headless.Model
				}
			}
			return usage, err
		},
		stateDir: invocation.StateDir,
	})
	if err != nil {
		return nil, err
	}
	started = true
	return process, nil
}

func (s *Supervisor) resolveAssignmentDirs(assignment Assignment) (stateDir, workDir, evidenceDir string, err error) {
	stateDir = strings.TrimSpace(assignment.StateDir)
	if stateDir == "" {
		stateDir = filepath.Join(s.runtime.OutputDir, "agents", assignment.RoleID)
	}
	stateDir, err = filepath.Abs(stateDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve %s lawyer state path: %w", assignment.RoleID, err)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return "", "", "", fmt.Errorf("create %s lawyer state directory: %w", assignment.RoleID, err)
	}

	workDir = strings.TrimSpace(assignment.WorkDir)
	if workDir == "" {
		workDir = filepath.Join(stateDir, "work")
	}
	workDir, err = filepath.Abs(workDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve %s lawyer working path: %w", assignment.RoleID, err)
	}
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return "", "", "", fmt.Errorf("create %s lawyer working directory: %w", assignment.RoleID, err)
	}

	evidenceDir = strings.TrimSpace(assignment.EvidenceDir)
	if evidenceDir == "" {
		return stateDir, workDir, "", nil
	}
	evidenceDir, err = filepath.Abs(evidenceDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve %s lawyer evidence path: %w", assignment.RoleID, err)
	}
	info, err := os.Stat(evidenceDir)
	if err != nil {
		return "", "", "", fmt.Errorf("inspect %s lawyer evidence directory: %w", assignment.RoleID, err)
	}
	if !info.IsDir() {
		return "", "", "", fmt.Errorf("%s lawyer evidence path %q is not a directory", assignment.RoleID, evidenceDir)
	}
	return stateDir, workDir, evidenceDir, nil
}

func (s *Supervisor) assignmentEnvironment(assignment Assignment) []string {
	env := s.runtime.BaseEnvironment
	if assignment.Environment != nil {
		env = assignment.Environment
	}
	for _, name := range []string{"ADJ_MCP_COMMAND", "ADJ_MCP_URL", "ADJ_MCP_BEARER_TOKEN"} {
		env = removeEnvironmentValue(env, name)
	}
	if assignment.MCPCommand != "" {
		env = setEnvironmentValue(env, "ADJ_MCP_COMMAND", assignment.MCPCommand)
		env = setEnvironmentValue(env, "ADJ_MCP_URL", assignment.MCP.URL)
		env = setEnvironmentValue(env, "ADJ_MCP_BEARER_TOKEN", assignment.MCP.BearerToken)
	}
	return env
}

func mcpCommandContainerArgs(args, env []string) []string {
	if command, ok := environmentValue(env, "ADJ_MCP_COMMAND"); ok && command != "" {
		args = append(args, "-v", command+":/opt/adj/mcp:ro",
			"-e", "ADJ_MCP_COMMAND=/opt/adj/mcp", "-e", "ADJ_MCP_URL", "-e", "ADJ_MCP_BEARER_TOKEN")
	}
	return args
}

func piContainer(invocation *headless.Invocation, name, image string, baseEnv []string) ([]string, []string, error) {
	if invocation == nil {
		return nil, nil, errors.New("Pi invocation is required")
	}
	credentialTargetEnv, credentialSourceEnv := invocation.CredentialEnvironmentNames()
	credentialEnv := strings.TrimSpace(credentialTargetEnv)
	credential := ""
	if credentialEnv != "" {
		var ok bool
		credential, ok = environmentValue(invocation.Env, credentialEnv)
		if !ok || credential == "" {
			return nil, nil, fmt.Errorf("Pi invocation credential environment variable %s is absent or empty", credentialEnv)
		}
	} else if strings.TrimSpace(credentialSourceEnv) != "" {
		return nil, nil, errors.New("Pi invocation has a credential source without a target environment variable")
	}
	translate := func(value string) string {
		if value == invocation.StateDir {
			return piContainerState
		}
		if strings.HasPrefix(value, invocation.StateDir+string(filepath.Separator)) {
			return piContainerState + strings.TrimPrefix(value, invocation.StateDir)
		}
		return value
	}
	innerArgs := make([]string, len(invocation.Args))
	for index, value := range invocation.Args {
		innerArgs[index] = translate(value)
	}
	args := []string{
		"run", "--rm",
		"--name", name,
		"--network", "host",
		"--user", "0:0",
		"-e", "HOME=" + piContainerState + "/home",
		"-e", "PI_CODING_AGENT_DIR=" + piContainerState + "/config",
		"-e", "PI_CODING_AGENT_SESSION_DIR=" + piContainerState + "/sessions",
	}
	if credentialEnv != "" {
		args = append(args, "-e", credentialEnv)
	}
	args = append(args,
		"-v", invocation.StateDir+":"+piContainerState,
		"-v", invocation.Dir+":"+PiWorkspacePath,
		"-w", PiWorkspacePath,
	)
	if invocation.EvidenceDir != "" {
		args = append(args, "-v", invocation.EvidenceDir+":"+PiEvidencePath+":ro")
	}
	args = mcpCommandContainerArgs(args, baseEnv)
	args = append(args, image)
	args = append(args, innerArgs...)
	processEnv := filterProviderEnvironment(baseEnv)
	processEnv = removeEnvironmentValue(processEnv, "PI_ALLOW_BROWSER_COOKIES")
	processEnv = removeEnvironmentValue(processEnv, "FEYNMAN_ALLOW_BROWSER_COOKIES")
	if sourceEnv := strings.TrimSpace(credentialSourceEnv); sourceEnv != "" {
		processEnv = removeEnvironmentValue(processEnv, sourceEnv)
	}
	if credentialEnv != "" {
		processEnv = setEnvironmentValue(processEnv, credentialEnv, credential)
	}
	return args, processEnv, nil
}

func (s *Supervisor) startOpenClaw(ctx context.Context, profile OpenClawProfile, assignment Assignment) (process *processRecord, returnErr error) {
	baseEnvironment := s.assignmentEnvironment(assignment)
	if err := validateOpenClawProfile(profile, baseEnvironment); err != nil {
		return nil, err
	}
	_, workDir, evidenceDir, err := s.resolveAssignmentDirs(assignment)
	if err != nil {
		return nil, err
	}
	mcpJSON, err := json.Marshal(map[string]any{
		"url":       assignment.MCP.URL,
		"transport": "streamable-http",
		"headers":   map[string]string{"Authorization": "Bearer " + assignment.MCP.BearerToken},
	})
	if err != nil {
		return nil, fmt.Errorf("encode OpenClaw MCP configuration: %w", err)
	}
	name := safeName(s.runtime.ContainerPrefix + "-" + assignment.CaseID + "-" + assignment.RoleID)
	authArgs, commandPrefix, processEnv, cleanup, err := s.openClawAuth(profile.Provider, profile.Auth, assignment.RoleID, baseEnvironment)
	if err != nil {
		return nil, err
	}
	started := false
	defer func() {
		if !started {
			returnErr = errors.Join(returnErr, cleanup())
		}
	}()
	configPrefix, err := openClawConfigPatchCommand(profile.Provider, profile.LawyerTurnTimeoutSeconds, assignment.WebSearch)
	if err != nil {
		return nil, err
	}
	hostUID := os.Geteuid()
	hostGID := os.Getegid()
	args := openClawContainerArgs(profile, name, workDir, evidenceDir)
	args = append(args, authArgs...)
	args = mcpCommandContainerArgs(args, baseEnvironment)
	for _, variable := range []string{"AAR_MCP_NAME", "AAR_MCP_JSON", "AAR_SESSION_KEY", "AAR_ASSIGNMENT", "AAR_PRINCIPAL"} {
		args = append(args, "-e", variable)
	}
	processEnv = setEnvironmentValue(processEnv, "AAR_MCP_NAME", assignment.MCP.Name)
	processEnv = setEnvironmentValue(processEnv, "AAR_MCP_JSON", string(mcpJSON))
	processEnv = setEnvironmentValue(processEnv, "AAR_SESSION_KEY", openClawSessionKey(s.runtime.ContainerPrefix, assignment))
	processEnv = setEnvironmentValue(processEnv, "AAR_ASSIGNMENT", assignment.Prompt)
	processEnv = setEnvironmentValue(processEnv, "AAR_PRINCIPAL", assignment.RoleID)
	args = append(args,
		profile.Image,
		"sh", "-lc",
		openClawAgentCommand(commandPrefix, configPrefix, profile.Model, profile.Thinking, profile.AgentTimeoutSeconds),
	)
	process, err = s.startProcess(ctx, "openclaw-"+assignment.RoleID, "docker", profile.Command, args, name, processStartOptions{
		env:        processEnv,
		roleID:     assignment.RoleID,
		verifyExit: assignment.VerifyExit,
		afterExit: func() error {
			return restoreOpenClawWorkspaceOwnership(profile.Command, profile.Image, workDir, hostUID, hostGID)
		},
		cleanup: cleanup,
		participant: &runstate.ParticipantStart{
			Role:            assignment.RoleID,
			Profile:         participantProfileName(assignment.Profile, RunnerOpenClaw),
			Runner:          string(RunnerOpenClaw),
			ReasoningEffort: strings.TrimSpace(profile.Thinking),
			WorkDir:         workDir,
			WebSearch:       webSearchObservation(Profile{Runner: RunnerOpenClaw, OpenClaw: profile}, assignment.WebSearch),
		},
		readUsage: headless.InspectOpenClawOutputFile,
	})
	if err != nil {
		return nil, err
	}
	started = true
	return process, nil
}

func validateOpenClawProfile(profile OpenClawProfile, baseEnv []string) error {
	if strings.TrimSpace(profile.Command) == "" || strings.TrimSpace(profile.Image) == "" || strings.TrimSpace(profile.Model) == "" || strings.TrimSpace(profile.Thinking) == "" {
		return errors.New("OpenClaw command, image, model, and thinking level are required")
	}
	if profile.AgentTimeoutSeconds <= 0 || profile.LawyerTurnTimeoutSeconds <= 0 {
		return errors.New("OpenClaw agent and lawyer-turn timeouts must be positive")
	}
	if profile.Network != "" && profile.Network != "host" {
		return fmt.Errorf("invalid OpenClaw network %q; expected host or empty", profile.Network)
	}
	provider, err := resolveOpenClawProvider(profile.Provider)
	if err != nil {
		return err
	}
	model := strings.ToLower(strings.TrimSpace(profile.Model))
	if (provider == "openai" && strings.Contains(model, "/") && !strings.HasPrefix(model, "openai/")) || (provider == "anthropic" && !strings.HasPrefix(model, "anthropic/")) {
		return fmt.Errorf("OpenClaw model %q does not match provider %q", profile.Model, provider)
	}
	switch profile.Auth.Mode {
	case OpenClawAuthCodex:
		if provider != "openai" {
			return fmt.Errorf("OpenClaw provider %q does not support Codex authentication", provider)
		}
		if strings.TrimSpace(profile.Auth.CodexAuthPath) == "" {
			return errors.New("OpenClaw Codex authentication requires a credential path")
		}
	case OpenClawAuthAPIKey:
		name := strings.TrimSpace(profile.Auth.APIKeyEnv)
		if !envNamePattern.MatchString(name) {
			return fmt.Errorf("OpenClaw API-key source environment variable %q is invalid", name)
		}
		value, ok := environmentValue(baseEnv, name)
		if !ok || value == "" {
			return fmt.Errorf("OpenClaw API-key source environment variable %s is absent or empty", name)
		}
	default:
		return fmt.Errorf("unsupported OpenClaw authentication mode %q", profile.Auth.Mode)
	}
	return nil
}

func openClawContainerArgs(profile OpenClawProfile, name, workDir, evidenceDir string) []string {
	args := []string{"run", "--rm", "--name", name, "--user", "0:0"}
	if profile.Network != "" {
		args = append(args, "--network", profile.Network)
	}
	if profile.Network != "host" {
		args = append(args, "--add-host=host.docker.internal:host-gateway")
	}
	args = append(args,
		"-e", "HOME=/home/node",
		"-e", "OPENCLAW_STATE_DIR="+openClawContainerState,
		"-e", "OPENCLAW_CONFIG_PATH="+openClawContainerConfig,
		"-e", "AAR_RETAINED_WORKSPACE="+OpenClawWorkspacePath,
		"-v", workDir+":"+OpenClawWorkspacePath,
		"-w", OpenClawWorkspacePath,
	)
	if evidenceDir != "" {
		args = append(args, "-v", evidenceDir+":"+OpenClawEvidencePath+":ro")
	}
	return args
}

func restoreOpenClawWorkspaceOwnership(command, image, workDir string, hostUID, hostGID int) error {
	args := []string{
		"run", "--rm",
		"--network", "none",
		"--user", "0:0",
		"-v", workDir + ":" + openClawCleanupWorkspace,
		"--entrypoint", "/usr/bin/chown",
		image,
		"-R", "--", strconv.Itoa(hostUID) + ":" + strconv.Itoa(hostGID), openClawCleanupWorkspace,
	}
	out, err := exec.Command(command, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(out))
	if message != "" {
		return fmt.Errorf("restore retained OpenClaw workspace ownership: %w: %s", err, message)
	}
	return fmt.Errorf("restore retained OpenClaw workspace ownership: %w", err)
}

func openClawSessionKey(prefix string, assignment Assignment) string {
	parts := []string{"agent", prefix, assignment.CaseID}
	if runID := strings.TrimSpace(assignment.RunID); runID != "" {
		parts = append(parts, runID)
	}
	parts = append(parts, assignment.RoleID)
	return strings.Join(parts, ":")
}

func (s *Supervisor) openClawAuth(providerName string, auth OpenClawAuth, role string, baseEnvironment []string) ([]string, string, []string, func() error, error) {
	provider, err := resolveOpenClawProvider(providerName)
	if err != nil {
		return nil, "", nil, noCleanup, err
	}
	processEnv := filterProviderEnvironment(baseEnvironment)
	switch auth.Mode {
	case OpenClawAuthAPIKey:
		name := strings.TrimSpace(auth.APIKeyEnv)
		key, ok := environmentValue(baseEnvironment, name)
		if !ok || key == "" {
			return nil, "", nil, noCleanup, fmt.Errorf("OpenClaw API-key source environment variable %s is absent or empty", name)
		}
		processEnv = removeEnvironmentValue(processEnv, name)
		target := "OPENAI_API_KEY"
		if provider == "anthropic" {
			target = "ANTHROPIC_API_KEY"
		}
		processEnv = setEnvironmentValue(processEnv, target, key)
		return []string{"-e", target}, "", processEnv, noCleanup, nil
	case OpenClawAuthCodex:
		home, cleanup, err := s.stageOpenClawCodexAuth(role, auth.CodexAuthPath)
		if err != nil {
			return nil, "", nil, noCleanup, err
		}
		args := []string{
			"-v", home + ":" + openClawCodexContainerHome + ":rw",
			"-e", "CODEX_HOME=" + openClawCodexContainerHome,
		}
		return args, openClawCodexAuthCommand(), processEnv, cleanup, nil
	default:
		return nil, "", nil, noCleanup, fmt.Errorf("unsupported OpenClaw authentication mode %q", auth.Mode)
	}
}

func resolveOpenClawProvider(configured string) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(configured))
	if provider == "" {
		provider = "openai"
	}
	if provider != "openai" && provider != "anthropic" {
		return "", fmt.Errorf("unsupported OpenClaw provider %q", provider)
	}
	return provider, nil
}

func (s *Supervisor) stageOpenClawCodexAuth(role, source string) (string, func() error, error) {
	home, err := filepath.Abs(filepath.Join(s.runtime.OutputDir, "openclaw-"+role+"-codex"))
	if err != nil {
		return "", noCleanup, fmt.Errorf("resolve OpenClaw Codex home path: %w", err)
	}
	if err := os.Mkdir(home, 0o700); err != nil {
		return "", noCleanup, fmt.Errorf("create OpenClaw Codex home: %w", err)
	}
	cleanup := func() error {
		if err := os.RemoveAll(home); err != nil {
			return fmt.Errorf("remove staged OpenClaw Codex home %s: %w", home, err)
		}
		return nil
	}
	if err := os.Chmod(home, 0o700); err != nil {
		return "", noCleanup, errors.Join(fmt.Errorf("chmod OpenClaw Codex home: %w", err), cleanup())
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return "", noCleanup, errors.Join(fmt.Errorf("read Codex auth file %s: %w", source, err), cleanup())
	}
	target := filepath.Join(home, "auth.json")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return "", noCleanup, errors.Join(fmt.Errorf("write staged Codex auth file: %w", err), cleanup())
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", noCleanup, errors.Join(fmt.Errorf("install staged Codex auth file: %w", err), os.Remove(tmp), cleanup())
	}
	if err := os.Chmod(target, 0o600); err != nil {
		return "", noCleanup, errors.Join(fmt.Errorf("chmod staged Codex auth file: %w", err), cleanup())
	}
	return home, cleanup, nil
}

func noCleanup() error { return nil }

func openClawConfigPatchCommand(providerName string, lawyerTimeoutSeconds int, webSearch *bool) (string, error) {
	if lawyerTimeoutSeconds <= 0 {
		return "", fmt.Errorf("lawyer timeout must be positive")
	}
	provider, err := resolveOpenClawProvider(providerName)
	if err != nil {
		return "", err
	}
	timeoutMS := lawyerTimeoutSeconds * 1000
	patch := map[string]any{
		"agents": map[string]any{
			"defaults": map[string]any{
				"workspace": OpenClawWorkspacePath,
			},
		},
		"plugins": map[string]any{
			"entries": map[string]any{
				"codex": map[string]any{
					"enabled": true,
					"config": map[string]any{
						"appServer": map[string]any{
							"turnCompletionIdleTimeoutMs":                 timeoutMS,
							"postToolRawAssistantCompletionIdleTimeoutMs": timeoutMS,
						},
					},
				},
			},
		},
		"tools": map[string]any{
			"profile":   "full",
			"allow":     nil,
			"alsoAllow": nil,
			"deny":      nil,
			"exec": map[string]any{
				"mode": "full",
			},
		},
	}
	if webSearch != nil {
		search := map[string]any{
			"enabled":     *webSearch,
			"provider":    nil,
			"apiKey":      nil,
			"openaiCodex": nil,
		}
		plugins := patch["plugins"].(map[string]any)["entries"].(map[string]any)
		plugins["duckduckgo"] = map[string]any{"enabled": *webSearch && provider == "anthropic"}
		if *webSearch && provider == "openai" {
			search["openaiCodex"] = map[string]any{
				"enabled":        true,
				"mode":           "live",
				"allowedDomains": nil,
				"contextSize":    nil,
				"userLocation":   nil,
			}
		}
		if *webSearch && provider == "anthropic" {
			search["provider"] = "duckduckgo"
		}
		tools := patch["tools"].(map[string]any)
		tools["web"] = map[string]any{"search": search}
	}
	raw, err := json.Marshal(patch)
	if err != nil {
		return "", fmt.Errorf("marshal OpenClaw config patch: %w", err)
	}
	return fmt.Sprintf("cat > /tmp/aar-openclaw-config.json <<'JSON'\n%s\nJSON\nopenclaw config patch --file /tmp/aar-openclaw-config.json\n", raw), nil
}

func webSearchObservation(profile Profile, enabled *bool) *runstate.WebSearch {
	if enabled == nil {
		return nil
	}
	observation := &runstate.WebSearch{Enabled: *enabled}
	if !*enabled {
		return observation
	}
	switch profile.Runner {
	case RunnerCodex:
		observation.Mechanism = "native"
		observation.Providers = []string{"openai"}
	case RunnerClaude:
		if profile.Headless.Provider == headless.ProviderOpenRouter {
			observation.Mechanism = "gateway"
			observation.Providers = []string{"openrouter"}
		} else {
			observation.Mechanism = "native"
			observation.Providers = []string{"anthropic"}
		}
	case RunnerPi:
		observation.Mechanism = "pi_web_access"
		observation.Providers = []string{"openai", "exa"}
	case RunnerOpenClaw:
		provider, err := resolveOpenClawProvider(profile.OpenClaw.Provider)
		if err != nil {
			return observation
		}
		if provider == "anthropic" {
			observation.Mechanism = "managed"
			observation.Providers = []string{"duckduckgo"}
		} else {
			observation.Mechanism = "native"
			observation.Providers = []string{"openai"}
		}
	}
	return observation
}

func openClawAgentCommand(commandPrefix, configPrefix, model, thinking string, timeoutSeconds int) string {
	return fmt.Sprintf(`set -eu
%s%sopenclaw mcp set "$AAR_MCP_NAME" "$AAR_MCP_JSON"
aar_openclaw_agent_attempts="${AAR_OPENCLAW_AGENT_ATTEMPTS:-3}"
case "$aar_openclaw_agent_attempts" in
    ''|*[!0-9]*)
        echo "error: AAR_OPENCLAW_AGENT_ATTEMPTS must be a positive integer" >&2
        exit 2
        ;;
esac
if [ "$aar_openclaw_agent_attempts" -lt 1 ]; then
    echo "error: AAR_OPENCLAW_AGENT_ATTEMPTS must be a positive integer" >&2
    exit 2
fi
aar_openclaw_agent_attempt=1
while :; do
    aar_openclaw_err="/tmp/aar-openclaw-agent-$aar_openclaw_agent_attempt.stderr"
    if openclaw agent --local --model %q --thinking %q --timeout %d --session-key "$AAR_SESSION_KEY" --message "$AAR_ASSIGNMENT" --json 2>"$aar_openclaw_err"; then
        cat "$aar_openclaw_err" >&2
        exit 0
    else
        aar_openclaw_status=$?
    fi
    cat "$aar_openclaw_err" >&2
    if [ "$aar_openclaw_agent_attempt" -ge "$aar_openclaw_agent_attempts" ]; then
        exit "$aar_openclaw_status"
    fi
    if ! grep -Eq 'stream disconnected before completion|LLM request timed out' "$aar_openclaw_err"; then
        exit "$aar_openclaw_status"
    fi
    printf 'openclaw agent attempt %%s failed after a recoverable model transport error; retrying\n' "$aar_openclaw_agent_attempt" >&2
    aar_openclaw_agent_attempt=$((aar_openclaw_agent_attempt + 1))
    sleep 10
done
`, commandPrefix, configPrefix, model, thinking, timeoutSeconds)
}

func openClawCodexAuthCommand() string {
	return `unset OPENAI_API_KEY
codex_token="$(node -e 'const fs=require("fs"); const home=process.env.CODEX_HOME; if (!home) process.exit(2); const d=JSON.parse(fs.readFileSync(home + "/auth.json", "utf8")); const t=d.tokens && d.tokens.access_token; if (!t) process.exit(3); process.stdout.write(t);')"
printf '%s\n' "$codex_token" | openclaw models auth paste-token --provider openai --profile-id openai:codex >/dev/null
unset codex_token
`
}

func (s *Supervisor) startProcess(ctx context.Context, name, kind, command string, args []string, stopName string, options processStartOptions) (*processRecord, error) {
	stdoutPath := filepath.Join(s.runtime.LogsDir, name+".stdout")
	stderrPath := filepath.Join(s.runtime.LogsDir, name+".stderr")
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return nil, fmt.Errorf("create %s stdout log: %w", name, err)
	}
	stderr, err := os.Create(stderrPath)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create %s stderr log: %w", name, err), closeNamed(stdout, name+" stdout log"))
	}
	var stop *containerStop
	if strings.TrimSpace(stopName) != "" {
		stop = &containerStop{
			command:     command,
			name:        stopName,
			processName: name,
			done:        make(chan struct{}),
		}
	}
	cmd := exec.CommandContext(ctx, command, args...)
	if stop != nil {
		cmd.Cancel = stop.stop
		cmd.WaitDelay = stopWait
	}
	if options.env != nil {
		cmd.Env = append([]string(nil), options.env...)
	}
	if strings.TrimSpace(options.dir) != "" {
		cmd.Dir = options.dir
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	record := &processRecord{
		name:          name,
		kind:          kind,
		command:       cmd,
		containerStop: stop,
		finished:      make(chan struct{}),
	}
	if err := cmd.Start(); err != nil {
		cleanupErr := error(nil)
		if options.cleanup != nil {
			cleanupErr = options.cleanup()
		}
		cmd.Env = nil
		return nil, errors.Join(fmt.Errorf("start %s: %w", name, err), cleanupErr, closeNamed(stdout, name+" stdout log"), closeNamed(stderr, name+" stderr log"))
	}
	startedAt := time.Now().UTC()
	finishProcess, err := runstate.Start(s.runtime.Observer, runstate.ProcessStart{
		Name:      name,
		Kind:      kind,
		Role:      options.roleID,
		PID:       cmd.Process.Pid,
		StartedAt: startedAt,
	})
	if err != nil {
		stopErr := stopStartedCommand(cmd, stop)
		waitErr := cmd.Wait()
		afterExitErr := error(nil)
		if options.afterExit != nil {
			afterExitErr = options.afterExit()
		}
		cleanupErr := error(nil)
		if options.cleanup != nil {
			cleanupErr = options.cleanup()
		}
		markerErr := error(nil)
		if options.markUnsuccessful != nil {
			markerErr = options.markUnsuccessful()
		}
		cmd.Env = nil
		return nil, errors.Join(fmt.Errorf("record %s process start: %w", name, err), stopErr, waitErr, afterExitErr, cleanupErr, markerErr, closeNamed(stdout, name+" stdout log"), closeNamed(stderr, name+" stderr log"))
	}
	var finishParticipant runstate.FinishParticipant
	if options.participant != nil {
		participant := *options.participant
		participant.StartedAt = startedAt
		finishParticipant, err = runstate.StartParticipant(s.runtime.Observer, participant)
		if err != nil {
			startErr := fmt.Errorf("record %s participant start: %w", name, err)
			stopErr := stopStartedCommand(cmd, stop)
			waitErr := cmd.Wait()
			afterExitErr := error(nil)
			if options.afterExit != nil {
				afterExitErr = options.afterExit()
			}
			cleanupErr := error(nil)
			if options.cleanup != nil {
				cleanupErr = options.cleanup()
			}
			markerErr := error(nil)
			if options.markUnsuccessful != nil {
				markerErr = options.markUnsuccessful()
			}
			cmd.Env = nil
			stdoutErr := closeNamed(stdout, name+" stdout log")
			stderrErr := closeNamed(stderr, name+" stderr log")
			exitCode := cmd.ProcessState.ExitCode()
			recordErr := runstate.Finish(finishProcess, runstate.ProcessFinish{
				State:      runstate.Failed,
				FinishedAt: time.Now().UTC(),
				ExitCode:   &exitCode,
				Error:      startErr.Error(),
			})
			return nil, errors.Join(startErr, stopErr, waitErr, afterExitErr, cleanupErr, markerErr, stdoutErr, stderrErr, recordErr)
		}
	}
	go func() {
		exit := processExit{waitErr: cmd.Wait()}
		if stop != nil {
			exit.stopErr = stop.result()
		}
		if options.afterExit != nil {
			exit.cleanupErr = options.afterExit()
		}
		if options.cleanup != nil {
			exit.cleanupErr = errors.Join(exit.cleanupErr, options.cleanup())
		}
		cmd.Env = nil
		exit.stdoutErr = closeNamed(stdout, name+" stdout log")
		exit.stderrErr = closeNamed(stderr, name+" stderr log")
		var (
			usage             *runstate.TokenUsage
			refusal           *runstate.ParticipantRefusal
			stateBytes        *int64
			participantObsErr error
		)
		if options.readUsage != nil && ctx.Err() == nil {
			var usageErr error
			usage, usageErr = options.readUsage(stdoutPath)
			if usageErr != nil {
				var providerRefusal *headless.ProviderRefusalError
				if errors.As(usageErr, &providerRefusal) {
					exit.terminalErr = usageErr
					runner := ""
					if options.participant != nil {
						runner = options.participant.Runner
					}
					var refusalErr error
					refusal, refusalErr = participantRefusalRecord(s.runtime.OutputDir, stdoutPath, options.stateDir, runner, providerRefusal)
					participantObsErr = errors.Join(participantObsErr, refusalErr)
				} else {
					participantObsErr = errors.Join(participantObsErr, usageErr)
				}
			}
		}
		if exit.err() == nil && ctx.Err() == nil {
			exit.statusErr = options.verifyExit(ctx, options.roleID, name)
		}
		if exit.err() == nil && participantObsErr == nil && ctx.Err() == nil && options.markSuccessful != nil {
			exit.markerErr = options.markSuccessful()
		}
		if finishParticipant != nil {
			if options.stateDir != "" {
				value, sizeErr := regularFileBytes(options.stateDir)
				if sizeErr != nil {
					participantObsErr = errors.Join(participantObsErr, fmt.Errorf("measure retained state %q: %w", options.stateDir, sizeErr))
				} else {
					stateBytes = &value
				}
			}
			exit.participantErr = errors.Join(participantObsErr, finishParticipant(runstate.ParticipantFinish{
				FinishedAt: time.Now().UTC(),
				StateBytes: stateBytes,
				Usage:      usage,
				Refusal:    refusal,
				Error:      runstate.ErrorText(participantObsErr),
			}))
		} else {
			exit.participantErr = participantObsErr
		}
		exit.canceled = ctx.Err() != nil
		if (exit.err() != nil || exit.canceled) && options.markUnsuccessful != nil {
			exit.markerErr = errors.Join(exit.markerErr, options.markUnsuccessful())
		}
		processErr := exit.err()
		state := runstate.Completed
		if exit.canceled {
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
		if exit.recordErr != nil && options.markUnsuccessful != nil {
			exit.markerErr = errors.Join(exit.markerErr, options.markUnsuccessful())
		}
		processErr = exit.err()
		record.markExited(exit)
		if exit.canceled || processErr == nil {
			return
		}
		select {
		case s.errors <- fmt.Errorf("%s process %s failed: %w", kind, name, processErr):
		case <-ctx.Done():
		}
	}()
	return record, nil
}

func closeNamed(closer io.Closer, name string) error {
	if closer == nil {
		return nil
	}
	if err := closer.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	return nil
}

func participantRefusalRecord(outputDir, stdoutPath, stateDir, runner string, refusal *headless.ProviderRefusalError) (*runstate.ParticipantRefusal, error) {
	if refusal == nil {
		return nil, nil
	}
	stdoutArtifact, artifactErr := relativeRunArtifact(outputDir, stdoutPath)
	record := &runstate.ParticipantRefusal{
		Provider:         refusal.Provider,
		Model:            refusal.Model,
		StopReason:       refusal.StopReason,
		RawStopReason:    refusal.RawStopReason,
		WillRetry:        refusal.WillRetry,
		Category:         refusal.Category,
		Explanation:      refusal.Explanation,
		SessionID:        refusal.SessionID,
		RequestID:        refusal.RequestID,
		ResponseID:       refusal.ResponseID,
		RefusedMessageID: refusal.RefusedMessageID,
		StdoutArtifact:   stdoutArtifact,
	}
	sessionPath, sessionErr := retainedSessionPath(runner, stateDir, refusal.SessionID)
	record.SessionPath = sessionPath
	return record, errors.Join(artifactErr, sessionErr)
}

func retainedSessionPath(runner, stateDir, sessionID string) (string, error) {
	switch Runner(strings.TrimSpace(runner)) {
	case RunnerPi:
		return retainedPiSessionPath(stateDir, sessionID)
	default:
		return "", nil
	}
}

func relativeRunArtifact(outputDir, path string) (string, error) {
	root, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("resolve run record directory %q: %w", outputDir, err)
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve participant output path %q: %w", path, err)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", fmt.Errorf("locate participant output %q beneath run record %q: %w", resolved, root, err)
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("participant output %q is outside run record %q", resolved, root)
	}
	return filepath.ToSlash(relative), nil
}

func retainedPiSessionPath(stateDir, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || strings.TrimSpace(stateDir) == "" {
		return "", nil
	}
	if !mcpNamePattern.MatchString(sessionID) {
		return "", fmt.Errorf("Pi session ID %q contains an unsupported character", sessionID)
	}
	dir := filepath.Join(stateDir, "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read retained Pi sessions %q: %w", dir, err)
	}
	suffix := "_" + sessionID + ".jsonl"
	var match string
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		if match != "" {
			return "", fmt.Errorf("retained Pi session ID %q matches more than one file in %q", sessionID, dir)
		}
		match = filepath.Join(dir, entry.Name())
	}
	if match == "" {
		return "", fmt.Errorf("retained Pi session ID %q has no file in %q", sessionID, dir)
	}
	return match, nil
}

func regularFileBytes(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect %q: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() > math.MaxInt64-total {
			return fmt.Errorf("regular-file size exceeds int64 at %q", path)
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

func (p *processRecord) markExited(exit processExit) {
	p.mu.Lock()
	p.exited = true
	p.exit = exit
	close(p.finished)
	p.mu.Unlock()
}

func (p *processRecord) isExited() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.exited
}

func (s *Supervisor) Stop() error {
	if s == nil {
		return nil
	}
	s.stopOnce.Do(func() {
		s.startStopMu.Lock()
		s.stopped = true
		s.mu.Lock()
		processes := append([]*processRecord(nil), s.processes...)
		s.mu.Unlock()
		s.startStopMu.Unlock()

		var errs []error
		for _, process := range processes {
			if err := stopProcess(process); err != nil {
				errs = append(errs, err)
			}
		}
		s.stopErr = errors.Join(errs...)
	})
	return s.stopErr
}

func (s *Supervisor) Wait(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	processes := append([]*processRecord(nil), s.processes...)
	s.mu.Unlock()
	var errs []error
	for _, process := range processes {
		exit, err := waitProcessExitContext(ctx, process)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if processErr := exit.err(); processErr != nil {
			errs = append(errs, fmt.Errorf("process %s exited: %w", process.name, processErr))
		}
	}
	return errors.Join(errs...)
}

func stopProcess(process *processRecord) error {
	if process.command.Process == nil {
		return nil
	}
	if process.isExited() {
		return collectProcessFinalization(process)
	}
	var stopErr error
	if process.containerStop != nil {
		stopErr = process.containerStop.stop()
	} else if err := process.command.Process.Kill(); err != nil && !process.isExited() {
		stopErr = fmt.Errorf("kill %s: %w", process.name, err)
	}
	if stopErr != nil && !process.isExited() {
		if err := process.command.Process.Kill(); err != nil && !process.isExited() {
			stopErr = errors.Join(stopErr, fmt.Errorf("kill %s after stop failure: %w", process.name, err))
		}
	}
	exit, waitErr := waitProcessExit(process, stopWait)
	if waitErr != nil {
		return errors.Join(stopErr, waitErr)
	}
	if process.containerStop != nil {
		return exit.finalizationErr()
	}
	return errors.Join(stopErr, exit.finalizationErr())
}

func collectProcessFinalization(process *processRecord) error {
	exit, err := waitProcessExit(process, stopWait)
	if err != nil {
		return err
	}
	processErr := exit.err()
	if exit.canceled {
		processErr = exit.finalizationErr()
	}
	if processErr != nil {
		return fmt.Errorf("process %s exited: %w", process.name, processErr)
	}
	return nil
}

func stopStartedCommand(command *exec.Cmd, stop *containerStop) error {
	var stopErr error
	if stop != nil {
		stopErr = stop.stop()
	}
	killErr := command.Process.Kill()
	if errors.Is(killErr, os.ErrProcessDone) {
		killErr = nil
	}
	return errors.Join(stopErr, killErr)
}

func (s *containerStop) stop() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		s.mu.Lock()
		s.started = true
		s.mu.Unlock()

		err := removeNamedContainer(s.command, s.name, s.processName)
		s.mu.Lock()
		s.err = err
		close(s.done)
		s.mu.Unlock()
	})
	return s.result()
}

func (s *containerStop) result() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	started := s.started
	done := s.done
	s.mu.Unlock()
	if !started {
		return nil
	}
	<-done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func removeNamedContainer(command, name, processName string) error {
	command = strings.TrimSpace(command)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if command == "" {
		return fmt.Errorf("container runtime command is empty for %s", processName)
	}
	out, err := exec.Command(command, "container", "rm", "-f", name).CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(out))
	if containerRemovalMissingOutput(message) {
		return nil
	}
	if message != "" {
		return fmt.Errorf("%s container rm -f %s: %w: %s", command, name, err, message)
	}
	return fmt.Errorf("%s container rm -f %s: %w", command, name, err)
}

func containerRemovalMissingOutput(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "no such container") ||
		strings.Contains(message, "no container with name or id") ||
		strings.Contains(message, "does not exist")
}

func waitProcessExit(process *processRecord, timeout time.Duration) (processExit, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-process.finished:
		process.mu.Lock()
		exit := process.exit
		process.mu.Unlock()
		return exit, nil
	case <-timer.C:
		return processExit{}, fmt.Errorf("timed out waiting for %s process %s to exit after stop", process.kind, process.name)
	}
}

func waitProcessExitContext(ctx context.Context, process *processRecord) (processExit, error) {
	select {
	case <-process.finished:
		process.mu.Lock()
		exit := process.exit
		process.mu.Unlock()
		return exit, nil
	case <-ctx.Done():
		return processExit{}, fmt.Errorf("wait for %s process %s: %w", process.kind, process.name, ctx.Err())
	}
}

func environmentValue(env []string, name string) (string, bool) {
	for index := len(env) - 1; index >= 0; index-- {
		key, value, ok := strings.Cut(env[index], "=")
		if ok && key == name {
			return value, true
		}
	}
	return "", false
}

func setEnvironmentValue(env []string, name, value string) []string {
	env = removeEnvironmentValue(env, name)
	return append(env, name+"="+value)
}

func removeEnvironmentValue(env []string, name string) []string {
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key == name {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func filterProviderEnvironment(env []string) []string {
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && (strings.HasSuffix(key, "_API_KEY") || key == "ANTHROPIC_AUTH_TOKEN" || key == "ANTHROPIC_OAUTH_TOKEN" || key == "ANTHROPIC_BASE_URL") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func safeName(value string) string {
	value = strings.ToLower(value)
	var out bytes.Buffer
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			out.WriteRune(character)
		} else {
			out.WriteByte('-')
		}
	}
	name := strings.Trim(out.String(), "-_.")
	if len(name) > 63 {
		name = strings.Trim(name[:63], "-_.")
	}
	if name == "" {
		return "lawyer"
	}
	return name
}
