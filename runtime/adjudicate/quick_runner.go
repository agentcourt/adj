package adjudicate

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentcourt/adj/internal/launcherprompt"
	lawyerlaunch "github.com/agentcourt/adj/internal/lawyer"
	"github.com/agentcourt/adj/internal/mcpcap"
	"github.com/agentcourt/adj/internal/mcpchild"
	headless "github.com/agentcourt/adj/runtime/agent"
	"github.com/agentcourt/adj/runtime/corehealth"
)

const (
	quickStartupTimeout            = 10 * time.Minute
	quickOpportunityWaitTimeout    = 30 * time.Second
	quickOpportunityRequestTimeout = quickOpportunityWaitTimeout + 2*time.Second
)

type lawyerSupervisor interface {
	Start(context.Context, lawyerlaunch.Profile, lawyerlaunch.Assignment) error
	Errors() <-chan error
	Wait(context.Context) error
	Stop() error
}

type quickMCPProcess interface {
	Address() string
	Done() <-chan error
}

type QuickRunner struct {
	startCore     func(context.Context, coreProcessRequest) (coreProcessWaiter, error)
	startMCP      func(context.Context, mcpchild.Request) (quickMCPProcess, error)
	newSupervisor func(lawyerlaunch.Runtime) (lawyerSupervisor, error)
	probeHealth   func(context.Context, string) error
	probeCore     func(context.Context, string, string, string) error
}

type quickRuntimeRecord struct {
	CaseID      string `json:"case_id"`
	RunID       string `json:"run_id"`
	CaseAPIBase string `json:"case_api_base"`
}

type quickCoreArgument struct {
	Role          string `json:"role"`
	OpportunityID string `json:"opportunity_id"`
}

type quickCoreResult struct {
	Status     string              `json:"status"`
	Phase      string              `json:"phase"`
	Resolution string              `json:"resolution"`
	Error      string              `json:"error"`
	ErrorClass string              `json:"error_class"`
	Arguments  []quickCoreArgument `json:"arguments"`
	Provider   ProviderManagement  `json:"provider"`
}

type quickLawyerOpportunityStatus struct {
	OK           bool                              `json:"ok"`
	CaseID       string                            `json:"case_id"`
	RoleID       string                            `json:"role_id"`
	StateVersion *uint64                           `json:"state_version"`
	Status       string                            `json:"status"`
	Turn         *quickLawyerOpportunityStatusTurn `json:"turn"`
}

type quickLawyerOpportunityStatusTurn struct {
	RoleID        string `json:"role_id"`
	OpportunityID string `json:"opportunity_id"`
	Completed     *bool  `json:"completed"`
}

type quickOpportunityStageResult struct {
	coreFinal *coreProcessOutcome
	mcpExited bool
	err       error
}

func (r QuickRunner) Run(ctx context.Context, request ProcedureRequest) (out ProcedureOutcome, returnErr error) {
	settings := request.Settings.Procedure.Quick
	if settings == nil {
		return ProcedureOutcome{}, fmt.Errorf("resolved quick settings are absent")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	baseEnvironment := os.Environ()
	env, err := coreProviderEnvironmentFor(request.Settings, baseEnvironment, "openrouter")
	if err != nil {
		return ProcedureOutcome{}, err
	}
	participantEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	mcpEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	corePromptFiles, mcpPromptFiles, err := mcpchild.SplitPromptFiles(map[string]string(settings.PromptFiles))
	if err != nil {
		return ProcedureOutcome{}, fmt.Errorf("split Quick prompt files: %w", err)
	}
	launcherPrompts, err := launcherprompt.Resolve("quick", settings.LauncherPromptDir, map[string]string(settings.LauncherPromptFiles))
	if err != nil {
		return ProcedureOutcome{}, err
	}
	lawyerPrompts := make(map[string]string, 2)
	lawyerWorkDirs := make(map[string]string, 2)
	lawyerEnvironments := make(map[string][]string, 2)
	evidenceDir, err := filepath.Abs(filepath.Join(request.RecordDir, "inputs", "documents"))
	if err != nil {
		return ProcedureOutcome{}, fmt.Errorf("resolve Quick evidence directory: %w", err)
	}
	for _, role := range []string{"plaintiff", "defendant"} {
		if !automaticLawyerEnabled(settings.AutoLawyers, role) {
			continue
		}
		profileName := settings.PlaintiffProfile
		if role == "defendant" {
			profileName = settings.DefendantProfile
		}
		profile, ok := request.Settings.AgentProfiles[profileName]
		if !ok {
			return ProcedureOutcome{}, fmt.Errorf("quick %s lawyer profile %q is absent", role, profileName)
		}
		lawyerEnvironment, err := participantEnvironmentFor(baseEnvironment, request.Settings, profileName)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare quick %s lawyer environment: %w", role, err)
		}
		lawyerEnvironments[role] = lawyerEnvironment
		workDir, err := filepath.Abs(participantWorkDir(request.RecordDir, role))
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("resolve Quick %s lawyer workspace: %w", role, err)
		}
		lawyerWorkDirs[role] = workDir
		workspacePromptPath, evidencePromptPath := quickLauncherPaths(profile.Runner, workDir, evidenceDir)
		serverName := quickMCPServerName(request.Request.CaseID, role)
		prompt, err := quickParticipantPrompt(launcherPrompts, role, request.Request.CaseID, serverName, workspacePromptPath, evidencePromptPath, profile.Runner == RunnerPi)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare quick %s lawyer prompt: %w", role, err)
		}
		lawyerPrompts[role] = prompt
	}
	caseAPIBearerToken, caseAPIBearerTokenPath, cleanupCaseAPIBearerToken, err := createQuickCaseAPIBearerToken()
	if err != nil {
		return ProcedureOutcome{}, err
	}
	defer func() {
		if cleanupCaseAPIBearerToken != nil {
			returnErr = errors.Join(returnErr, cleanupCaseAPIBearerToken())
		}
	}()
	secretFiles := []string{}
	defer func() {
		for _, path := range secretFiles {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove Quick remote-lawyer file %q: %w", path, err))
			}
		}
	}()

	args := []string{
		"case",
		"--proposition", request.Request.Proposition,
		"--documents", filepath.Join(request.RecordDir, "inputs", "documents"),
		"--out-dir", request.CoreDir,
		"--council-pool", request.Settings.Common.CouncilPool,
		"--council-size", strconv.Itoa(request.Settings.Common.CouncilSize),
		"--required-votes", strconv.Itoa(request.Settings.Common.RequiredVotes),
		"--evidence-standard", request.Settings.Common.EvidenceStandard,
		"--caseapi-addr", "127.0.0.1:0",
		"--lawyerapi-bearer-token-file", caseAPIBearerTokenPath,
		"--case-id", request.Request.CaseID,
		"--run-id", request.Request.RunID,
		"--max-document-files", strconv.Itoa(request.Settings.Common.DocumentLimits.Count),
		"--max-document-file-bytes", strconv.FormatInt(request.Settings.Common.DocumentLimits.PerFile, 10),
		"--max-documents-total-bytes", strconv.FormatInt(request.Settings.Common.DocumentLimits.Total, 10),
		"--lawyer-web-search=" + strconv.FormatBool(settings.WebSearch),
		"--allow-api-key",
	}
	args = appendCorePromptArgs(args, settings.PromptDir, PromptFilePaths(corePromptFiles))
	if settings.Timeout != 0 {
		timeout := time.Duration(settings.Timeout).String()
		args = append(args, "--lawyer-timeout", timeout, "--council-timeout", timeout)
	}
	if settings.ParallelCouncil {
		args = append(args, "--parallel-council")
	}

	startCore := r.startCore
	if startCore == nil {
		startCore = startCoreProcess
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	process, err := startCore(runCtx, coreProcessRequest{
		Command:    settings.CoreCommand,
		Args:       args,
		Dir:        settings.CoreWorkingDir,
		Env:        env,
		LogsDir:    request.LogsDir,
		LogPrefix:  "quick",
		ResultPath: filepath.Join(request.CoreDir, "run.json"),
		Observer:   request.Observer,
	})
	if err != nil {
		return ProcedureOutcome{}, err
	}

	var (
		coreFinal  *coreProcessOutcome
		mcpDone    <-chan error
		supervisor lawyerSupervisor
	)
	defer func() {
		cancel()
		if supervisor != nil {
			returnErr = errors.Join(returnErr, supervisor.Stop())
		}
		if coreFinal == nil {
			outcome := <-process.Done()
			coreFinal = &outcome
			returnErr = errors.Join(returnErr, outcome.err)
		}
		if mcpDone != nil {
			returnErr = errors.Join(returnErr, <-mcpDone)
		}
	}()

	runtime, early, err := r.waitForQuickRuntime(runCtx, process.Done(), filepath.Join(request.CoreDir, "runtime.json"), request.Request.CaseID, request.Request.RunID)
	if early != nil {
		coreFinal = early
		return mapQuickCoreOutcome(*early)
	}
	if err != nil {
		return ProcedureOutcome{}, err
	}
	if err := cleanupCaseAPIBearerToken(); err != nil {
		return ProcedureOutcome{}, err
	}
	cleanupCaseAPIBearerToken = nil

	signingKey, err := mcpcap.GenerateKey()
	if err != nil {
		return ProcedureOutcome{}, err
	}
	capabilities := make(map[string]string, 2)
	for _, role := range []string{"plaintiff", "defendant"} {
		capability, issueErr := mcpcap.Issue(signingKey, mcpcap.Claims{
			Audience:       "quick",
			CaseID:         request.Request.CaseID,
			AssignmentType: "lawyer",
			PrincipalID:    role,
		})
		if issueErr != nil {
			return ProcedureOutcome{}, fmt.Errorf("issue quick %s MCP capability: %w", role, issueErr)
		}
		capabilities[role] = capability
	}
	startMCP := r.startMCP
	if startMCP == nil {
		startMCP = func(ctx context.Context, request mcpchild.Request) (quickMCPProcess, error) {
			return mcpchild.Start(ctx, request)
		}
	}
	type mcpStartResult struct {
		process quickMCPProcess
		err     error
	}
	mcpStarted := make(chan mcpStartResult, 1)
	go func() {
		mcpProcess, startErr := startMCP(runCtx, mcpchild.Request{
			Command:            settings.MCPCommand,
			CommandArgs:        []string{"serve"},
			WorkingDir:         settings.MCPWorkingDir,
			Environment:        mcpEnvironment,
			ListenAddr:         quickMCPListenAddr(settings.MCPListenAddr),
			CaseAPIBase:        runtime.CaseAPIBase,
			SigningKey:         signingKey,
			CaseAPIBearerToken: []byte(caseAPIBearerToken),
			PromptDir:          settings.PromptDir,
			PromptFiles:        mcpPromptFiles,
			LogsDir:            request.LogsDir,
			LogPrefix:          "quick-mcp",
			Name:               "quick-mcp",
			Observer:           request.Observer,
			StartupTimeout:     quickStartupTimeout,
		})
		mcpStarted <- mcpStartResult{process: mcpProcess, err: startErr}
	}()
	var mcpProcess quickMCPProcess
	select {
	case early := <-process.Done():
		coreFinal = &early
		cancel()
		started := <-mcpStarted
		if started.process != nil {
			mcpDone = started.process.Done()
		}
		mapped, mapErr := mapQuickCoreOutcome(early)
		return mapped, errors.Join(mapErr, started.err)
	case started := <-mcpStarted:
		if started.err != nil {
			return ProcedureOutcome{}, fmt.Errorf("start quick MCP: %w", started.err)
		}
		mcpProcess = started.process
	}
	if mcpProcess == nil {
		return ProcedureOutcome{}, fmt.Errorf("start quick MCP: process is nil")
	}
	mcpDone = mcpProcess.Done()
	mcpAddress := mcpProcess.Address()
	selectedMCPListenAddr, _, err := mcpchild.SelectedListenAddress(quickMCPListenAddr(settings.MCPListenAddr), mcpAddress)
	if err != nil {
		return ProcedureOutcome{}, err
	}
	mcpPublicBase, err := quickPublicMCPBase(settings.MCPPublicBaseURL, selectedMCPListenAddr, settings.AutoLawyers)
	if err != nil {
		return ProcedureOutcome{}, err
	}
	mcpStartupCtx, mcpStartupCancel := context.WithTimeout(runCtx, quickStartupTimeout)
	probeHealth := r.probeHealth
	if probeHealth == nil {
		probeHealth = probeHTTPHealth
	}
	err = probeHealth(mcpStartupCtx, "http://"+mcpAddress+"/health")
	mcpStartupCancel()
	if err != nil {
		return ProcedureOutcome{}, fmt.Errorf("wait for quick MCP: %w", err)
	}
	for _, role := range quickManualLawyerRoles(settings.AutoLawyers) {
		path, err := writeQuickRemoteLawyerSkill(request.RecordDir, role, request.Request.CaseID, mcpPublicBase, capabilities[role], settings.WebSearch, launcherPrompts)
		if err != nil {
			return ProcedureOutcome{}, err
		}
		secretFiles = append(secretFiles, path)
	}

	if settings.AutoLawyers != "none" {
		newSupervisor := r.newSupervisor
		if newSupervisor == nil {
			newSupervisor = func(runtime lawyerlaunch.Runtime) (lawyerSupervisor, error) {
				return lawyerlaunch.New(runtime)
			}
		}
		supervisor, err = newSupervisor(lawyerlaunch.Runtime{
			OutputDir:       request.RecordDir,
			LogsDir:         request.LogsDir,
			ContainerPrefix: "quick",
			PodmanCommand:   defaultPodmanCommand,
			PiImage:         defaultPiImage,
			BaseEnvironment: participantEnvironment,
			Observer:        request.Observer,
		})
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("create quick lawyer supervisor: %w", err)
		}
	}
	finishCore := func(outcome coreProcessOutcome) (ProcedureOutcome, error) {
		coreFinal = &outcome
		mapped, mapErr := mapQuickCoreOutcome(outcome)
		if mapErr != nil {
			return ProcedureOutcome{}, mapErr
		}
		if supervisor == nil {
			return mapped, nil
		}
		waitCtx, waitCancel := context.WithTimeout(ctx, 30*time.Second)
		defer waitCancel()
		return mapped, supervisor.Wait(waitCtx)
	}

	startLawyer := func(role string) error {
		profileName := settings.PlaintiffProfile
		if role == "defendant" {
			profileName = settings.DefendantProfile
		}
		resolvedProfile, ok := request.Settings.AgentProfiles[profileName]
		if !ok {
			return fmt.Errorf("quick %s lawyer profile %q is absent", role, profileName)
		}
		profile, err := resolvedLawyerLaunchProfile(resolvedProfile, settings.Timeout)
		if err != nil {
			return fmt.Errorf("resolve quick %s lawyer profile: %w", role, err)
		}
		mcpBase := "http://" + mcpAddress
		if resolvedProfile.Runner == RunnerOpenClaw && profile.OpenClaw.Network != "host" {
			parsedAddress := strings.SplitN(mcpAddress, ":", 2)
			if len(parsedAddress) != 2 {
				return fmt.Errorf("invalid MCP address %q", mcpAddress)
			}
			mcpBase = "http://host.docker.internal:" + parsedAddress[1]
		}
		serverName := quickMCPServerName(request.Request.CaseID, role)
		assignmentURL := quickMCPURL(mcpBase)
		return supervisor.Start(runCtx, profile, lawyerlaunch.Assignment{
			CaseID:      request.Request.CaseID,
			RunID:       request.Request.RunID,
			RoleID:      role,
			Profile:     profileName,
			Prompt:      lawyerPrompts[role],
			WebSearch:   &settings.WebSearch,
			Environment: lawyerEnvironments[role],
			StateDir:    participantStateDir(request.SessionDir, profileName, role),
			WorkDir:     lawyerWorkDirs[role],
			EvidenceDir: evidenceDir,
			MCP: headless.MCPServer{
				Name:        serverName,
				URL:         assignmentURL,
				BearerToken: capabilities[role],
			},
			VerifyExit: quickLawyerExitVerifier(runtime.CaseAPIBase, request.Request.CaseID, request.CoreDir, caseAPIBearerToken),
		})
	}
	if automaticLawyerEnabled(settings.AutoLawyers, "plaintiff") {
		if err := startLawyer("plaintiff"); err != nil {
			return ProcedureOutcome{}, fmt.Errorf("start quick plaintiff lawyer: %w", err)
		}
	}

	defendantReady := make(chan error, 1)
	go func() {
		defendantReady <- waitForQuickLawyerOpportunity(runCtx, runtime.CaseAPIBase, request.Request.CaseID, "defendant", caseAPIBearerToken)
	}()
	var supervisorErrors <-chan error
	if supervisor != nil {
		supervisorErrors = supervisor.Errors()
	}
	stage := waitForQuickOpportunityStage(ctx, process.Done(), mcpDone, supervisorErrors, defendantReady)
	if stage.mcpExited {
		mcpDone = nil
	}
	if stage.coreFinal != nil {
		return finishCore(*stage.coreFinal)
	}
	if stage.err != nil {
		return ProcedureOutcome{}, stage.err
	}
	if automaticLawyerEnabled(settings.AutoLawyers, "defendant") {
		if err := startLawyer("defendant"); err != nil {
			return ProcedureOutcome{}, fmt.Errorf("start quick defendant lawyer: %w", err)
		}
	}

	select {
	case outcome := <-process.Done():
		return finishCore(outcome)
	case err := <-mcpDone:
		mcpDone = nil
		return ProcedureOutcome{}, quickMCPExitError(err)
	case err := <-supervisorErrors:
		return ProcedureOutcome{}, err
	case <-ctx.Done():
		return ProcedureOutcome{}, ctx.Err()
	}
}

func (r QuickRunner) waitForQuickRuntime(ctx context.Context, coreDone <-chan coreProcessOutcome, path, caseID, runID string) (quickRuntimeRecord, *coreProcessOutcome, error) {
	probe := r.probeCore
	if probe == nil {
		probe = func(ctx context.Context, healthURL, caseID, runID string) error {
			return corehealth.Probe(ctx, http.DefaultClient, healthURL, caseID, runID)
		}
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, quickStartupTimeout)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var lastProbeErr error
	for {
		raw, err := os.ReadFile(path)
		if err == nil {
			var runtime quickRuntimeRecord
			if err := json.Unmarshal(raw, &runtime); err != nil {
				return quickRuntimeRecord{}, nil, fmt.Errorf("decode quick runtime record: %w", err)
			}
			if strings.TrimSpace(runtime.CaseAPIBase) == "" {
				return quickRuntimeRecord{}, nil, fmt.Errorf("quick runtime record has no case_api_base")
			}
			if runtime.CaseID != caseID || runtime.RunID != runID {
				return quickRuntimeRecord{}, nil, fmt.Errorf("quick runtime identity is case %q run %q; expected case %q run %q", runtime.CaseID, runtime.RunID, caseID, runID)
			}
			if err := probe(deadlineCtx, runtime.CaseAPIBase+"/health", caseID, runID); err == nil {
				return runtime, nil, nil
			} else {
				lastProbeErr = err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return quickRuntimeRecord{}, nil, fmt.Errorf("read quick runtime record: %w", err)
		}
		select {
		case outcome := <-coreDone:
			return quickRuntimeRecord{}, &outcome, nil
		case <-deadlineCtx.Done():
			return quickRuntimeRecord{}, nil, errors.Join(
				fmt.Errorf("quick core did not become healthy within %s", quickStartupTimeout),
				lastProbeErr,
				deadlineCtx.Err(),
			)
		case <-ticker.C:
		}
	}
}

func mapQuickCoreOutcome(outcome coreProcessOutcome) (ProcedureOutcome, error) {
	var result quickCoreResult
	decodeErr := decodeCoreResult(outcome.result, &result)
	if decodeErr != nil {
		return ProcedureOutcome{}, errors.Join(outcome.err, decodeErr)
	}
	if result.Status != "ok" {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = fmt.Sprintf("quick core returned status %q", result.Status)
		}
		return ProcedureOutcome{}, &CoreRunError{Class: result.ErrorClass, Err: errors.Join(errors.New(message), outcome.err), Provider: result.Provider}
	}
	if outcome.err != nil {
		return ProcedureOutcome{}, &CoreRunError{Err: outcome.err, Provider: result.Provider}
	}
	if result.Resolution != "demonstrated" && result.Resolution != "not_demonstrated" && result.Resolution != "no_majority" {
		return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("quick core returned invalid resolution %q", result.Resolution), Provider: result.Provider}
	}
	return ProcedureOutcome{
		Status: StatusOK,
		Phase:  result.Phase,
		Decision: &Decision{
			Kind:  "binary",
			Value: result.Resolution,
		},
		ProcedureResult: append(json.RawMessage(nil), outcome.result...),
		Provider:        result.Provider,
	}, nil
}

func waitForQuickLawyerOpportunity(ctx context.Context, caseAPIBase, caseID, role, caseAPIBearerToken string) error {
	return waitForQuickLawyerOpportunityWithTimeouts(ctx, caseAPIBase, caseID, role, caseAPIBearerToken, quickOpportunityWaitTimeout, quickOpportunityRequestTimeout)
}

func waitForQuickLawyerOpportunityWithTimeouts(ctx context.Context, caseAPIBase, caseID, role, caseAPIBearerToken string, waitTimeout, requestTimeout time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	waitURL := strings.TrimRight(caseAPIBase, "/") + "/lawyerapi/v1/wait"
	var (
		afterVersion    uint64
		hasAfterVersion bool
	)
	for {
		query := url.Values{
			"case_id":    {caseID},
			"role_id":    {role},
			"timeout_ms": {strconv.FormatInt(waitTimeout.Milliseconds(), 10)},
		}
		if hasAfterVersion {
			query.Set("after_version", strconv.FormatUint(afterVersion, 10))
		}
		requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		raw, requestErr := readQuickAPI(requestCtx, waitURL+"?"+query.Encode(), caseAPIBearerToken, "quick lawyer wait")
		cancel()
		if requestErr != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
			if errors.Is(requestErr, context.DeadlineExceeded) {
				continue
			}
			return fmt.Errorf("wait for quick %s lawyer opportunity: %w", role, requestErr)
		}
		var status quickLawyerOpportunityStatus
		if err := json.Unmarshal(raw, &status); err != nil {
			return fmt.Errorf("decode quick %s lawyer wait response: %w", role, err)
		}
		if !status.OK {
			return fmt.Errorf("quick %s lawyer wait response is not ok", role)
		}
		if status.CaseID != caseID || status.RoleID != role {
			return fmt.Errorf("quick %s lawyer wait response identifies case %q role %q; expected case %q role %q", role, status.CaseID, status.RoleID, caseID, role)
		}
		switch status.Status {
		case "ready":
			if status.Turn == nil {
				return fmt.Errorf("quick %s lawyer wait response is ready without a turn", role)
			}
			if status.Turn.RoleID != role || status.Turn.OpportunityID != "arguments:"+role {
				return fmt.Errorf("quick %s lawyer wait response has turn role %q opportunity %q", role, status.Turn.RoleID, status.Turn.OpportunityID)
			}
			if status.Turn.Completed == nil || *status.Turn.Completed {
				return fmt.Errorf("quick %s lawyer wait response has no active turn", role)
			}
			return nil
		case "waiting":
			if status.StateVersion == nil {
				return fmt.Errorf("quick %s lawyer wait response is waiting without state_version", role)
			}
			afterVersion = *status.StateVersion
			hasAfterVersion = true
		case "done", "failed":
			<-ctx.Done()
			return ctx.Err()
		default:
			return fmt.Errorf("quick %s lawyer wait response has invalid status %q", role, status.Status)
		}
	}
}

func waitForQuickOpportunityStage(ctx context.Context, coreDone <-chan coreProcessOutcome, mcpDone <-chan error, supervisorErrors <-chan error, ready <-chan error) quickOpportunityStageResult {
	select {
	case err := <-ready:
		return quickOpportunityStageResult{err: err}
	case outcome := <-coreDone:
		return quickOpportunityStageResult{coreFinal: &outcome}
	case err := <-mcpDone:
		return quickOpportunityStageResult{mcpExited: true, err: quickMCPExitError(err)}
	case err := <-supervisorErrors:
		return quickOpportunityStageResult{err: err}
	case <-ctx.Done():
		return quickOpportunityStageResult{err: ctx.Err()}
	}
}

func quickMCPExitError(err error) error {
	if err == nil {
		return fmt.Errorf("quick MCP exited before core completion")
	}
	return fmt.Errorf("quick MCP failed before core completion: %w", err)
}

func quickLawyerExitVerifier(caseAPIBase, caseID, coreDir, caseAPIBearerToken string) func(context.Context, string, string) error {
	return func(ctx context.Context, role, processName string) error {
		resultURL := strings.TrimRight(caseAPIBase, "/") + "/lawyerapi/v1/result?case_id=" + url.QueryEscape(caseID) + "&role_id=" + url.QueryEscape(role)
		deadlineCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		var lastErr error
		for {
			raw, requestErr := readQuickResult(deadlineCtx, resultURL, caseAPIBearerToken)
			if requestErr == nil {
				found, parseErr := quickResultContainsArgument(raw, role)
				if parseErr != nil {
					return fmt.Errorf("decode quick result after %s exit: %w", processName, parseErr)
				}
				if !found {
					return fmt.Errorf("quick lawyer process %s exited before filing the %s argument", processName, role)
				}
				return nil
			}
			lastErr = requestErr
			durable, readErr := os.ReadFile(filepath.Join(coreDir, "run.json"))
			if readErr == nil {
				found, parseErr := quickResultContainsArgument(durable, role)
				if parseErr != nil {
					return fmt.Errorf("decode durable quick result after %s exit: %w", processName, parseErr)
				}
				if !found {
					return fmt.Errorf("quick lawyer process %s exited before filing the %s argument", processName, role)
				}
				return nil
			}
			if !errors.Is(readErr, os.ErrNotExist) {
				lastErr = errors.Join(lastErr, fmt.Errorf("read durable quick result: %w", readErr))
			}
			select {
			case <-deadlineCtx.Done():
				return errors.Join(fmt.Errorf("verify quick lawyer process %s exit: %w", processName, deadlineCtx.Err()), lastErr)
			case <-ticker.C:
			}
		}
	}
}

func readQuickResult(ctx context.Context, resultURL, caseAPIBearerToken string) (raw []byte, returnErr error) {
	return readQuickAPI(ctx, resultURL, caseAPIBearerToken, "quick result")
}

func readQuickAPI(ctx context.Context, requestURL, caseAPIBearerToken, responseName string) (raw []byte, returnErr error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+caseAPIBearerToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, resp.Body.Close())
	}()
	raw, err = io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", responseName, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}

func createQuickCaseAPIBearerToken() (token, path string, cleanup func() error, returnErr error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", nil, fmt.Errorf("generate Quick case API bearer token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	dir, err := os.MkdirTemp("", "adj-quick-caseapi-")
	if err != nil {
		return "", "", nil, fmt.Errorf("create private Quick case API directory: %w", err)
	}
	cleanup = func() error {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("remove private Quick case API directory: %w", err)
		}
		return nil
	}
	path = filepath.Join(dir, "bearer-token")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", "", nil, errors.Join(fmt.Errorf("create Quick case API bearer-token file: %w", err), cleanup())
	}
	written, writeErr := file.Write([]byte(token))
	if written != len(token) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return "", "", nil, errors.Join(fmt.Errorf("write Quick case API bearer-token file: %w", err), cleanup())
	}
	return token, path, cleanup, nil
}

func quickResultContainsArgument(raw []byte, role string) (bool, error) {
	var response struct {
		Arguments []quickCoreArgument `json:"arguments"`
		Result    *quickCoreResult    `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return false, err
	}
	arguments := response.Arguments
	if response.Result != nil {
		arguments = response.Result.Arguments
	}
	return quickArgumentsContainRole(arguments, role), nil
}

func quickArgumentsContainRole(arguments []quickCoreArgument, role string) bool {
	wantOpportunity := "arguments:" + role
	for _, argument := range arguments {
		if argument.Role == role && argument.OpportunityID == wantOpportunity {
			return true
		}
	}
	return false
}

func probeHTTPHealth(ctx context.Context, healthURL string) (returnErr error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, resp.Body.Close())
	}()
	if resp.StatusCode != http.StatusNoContent {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return errors.Join(fmt.Errorf("health check returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))), readErr)
	}
	return nil
}

func splitQuickPromptFiles(paths PromptFilePaths) (PromptFilePaths, PromptFilePaths, error) {
	core, mcp, err := mcpchild.SplitPromptFiles(map[string]string(paths))
	return PromptFilePaths(core), PromptFilePaths(mcp), err
}

func quickMCPListenAddr(configured string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return "127.0.0.1:0"
	}
	return configured
}

func quickPublicMCPBase(configured, selectedListenAddr, autoLawyers string) (string, error) {
	configured = strings.TrimRight(strings.TrimSpace(configured), "/")
	if configured == "" {
		host, _, err := net.SplitHostPort(selectedListenAddr)
		if err != nil {
			return "", fmt.Errorf("parse Quick MCP listen address %q: %w", selectedListenAddr, err)
		}
		if len(quickManualLawyerRoles(autoLawyers)) > 0 && (host == "" || host == "0.0.0.0" || host == "::") {
			return "", fmt.Errorf("manual Quick lawyer mode requires mcp_public_base_url when mcp_listen uses a wildcard host")
		}
		return "http://" + selectedListenAddr, nil
	}
	parsed, err := url.Parse(configured)
	if err != nil {
		return "", fmt.Errorf("parse Quick MCP public base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("Quick MCP public base URL must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("Quick MCP public base URL requires a host and no query or fragment")
	}
	return configured, nil
}

func quickManualLawyerRoles(mode string) []string {
	roles := make([]string, 0, 2)
	for _, role := range []string{"plaintiff", "defendant"} {
		if !automaticLawyerEnabled(mode, role) {
			roles = append(roles, role)
		}
	}
	return roles
}

func writeQuickRemoteLawyerSkill(recordDir, role, caseID, mcpBase, capability string, webSearch bool, prompts launcherprompt.Sources) (string, error) {
	server := quickMCPServerName(caseID, role)
	mcpURL := quickMCPURL(mcpBase)
	mcpJSON, err := json.Marshal(map[string]any{
		"url":       mcpURL,
		"transport": "streamable-http",
		"headers":   map[string]string{"Authorization": "Bearer " + capability},
	})
	if err != nil {
		return "", err
	}
	instructions, err := prompts.Render("skill.openclaw", map[string]string{
		"{{CASE_ID}}":             caseID,
		"{{ROLE_ID}}":             role,
		"{{MCP_SERVER}}":          server,
		"{{MCP_URL}}":             mcpURL,
		"{{MCP_JSON}}":            string(mcpJSON),
		"{{SEARCH_INSTRUCTIONS}}": launcherprompt.RemoteSearchInstructions(webSearch),
	})
	if err != nil {
		return "", fmt.Errorf("prepare Quick %s remote-lawyer prompt: %w", role, err)
	}
	path := filepath.Join(recordDir, "openclaw-"+role+"-lawyer-skill.md")
	if err := writeAtomic(path, 0o600, func(file *os.File) error {
		written, err := io.WriteString(file, instructions)
		if written != len(instructions) {
			err = errors.Join(err, io.ErrShortWrite)
		}
		return err
	}); err != nil {
		return "", fmt.Errorf("write Quick %s remote-lawyer skill: %w", role, err)
	}
	return path, nil
}

func quickMCPURL(base string) string {
	return strings.TrimRight(base, "/") + "/mcp"
}

func quickMCPServerName(caseID, role string) string {
	value := "quick-" + caseID + "-" + role
	var result strings.Builder
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			result.WriteRune(character)
		} else {
			result.WriteByte('-')
		}
	}
	return strings.Trim(result.String(), "-")
}

func quickParticipantPrompt(prompts launcherprompt.Sources, role, caseID, server, workspace, evidenceDir string, pi bool) (string, error) {
	promptID := "participant"
	if pi {
		promptID = "participant.pi"
	}
	return prompts.Render(promptID, map[string]string{
		"{{ROLE}}":         role,
		"{{CASE}}":         caseID,
		"{{SERVER}}":       server,
		"{{WORKSPACE}}":    workspace,
		"{{EVIDENCE_DIR}}": evidenceDir,
	})
}

func quickLauncherPaths(runner AgentRunner, hostWorkspace, hostEvidenceDir string) (workspace, evidenceDir string) {
	switch runner {
	case RunnerPi:
		return lawyerlaunch.PiWorkspacePath, lawyerlaunch.PiEvidencePath
	case RunnerOpenClaw:
		return lawyerlaunch.OpenClawWorkspacePath, lawyerlaunch.OpenClawEvidencePath
	default:
		return hostWorkspace, hostEvidenceDir
	}
}
