package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	headless "github.com/agentcourt/adj/runtime/agent"
	localrun "github.com/agentcourt/adj/runtime/localrun/adc"
)

type adcRunFunc func(context.Context, localrun.Options) (localrun.Result, error)

type ADCRunner struct {
	run adcRunFunc
}

func (r ADCRunner) Run(ctx context.Context, request ProcedureRequest) (ProcedureOutcome, error) {
	settings := request.Settings.Procedure.ADC
	if settings == nil {
		return ProcedureOutcome{}, fmt.Errorf("resolved adc settings are absent")
	}
	providers := configuredProviderNames(request.Settings.Common.ProviderCredentials)
	baseEnvironment := os.Environ()
	coreEnvironment, err := coreProviderEnvironmentFor(request.Settings, baseEnvironment, providers...)
	if err != nil {
		return ProcedureOutcome{}, err
	}
	mcpEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	participantEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	var plaintiff, defendant localrun.LawyerProfile
	if automaticLawyerEnabled(settings.AutoLawyers, "plaintiff") {
		plaintiff, err = resolvedADCLawyerProfile(request.Settings, settings.PlaintiffProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("resolve adc plaintiff lawyer: %w", err)
		}
		plaintiff.Environment, err = participantEnvironmentFor(baseEnvironment, request.Settings, settings.PlaintiffProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare adc plaintiff lawyer environment: %w", err)
		}
		plaintiff.StateDir = participantStateDir(request.SessionDir, settings.PlaintiffProfile, "plaintiff")
		plaintiff.WorkDir = participantWorkDir(request.RecordDir, "plaintiff")
	}
	if automaticLawyerEnabled(settings.AutoLawyers, "defendant") {
		defendant, err = resolvedADCLawyerProfile(request.Settings, settings.DefendantProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("resolve adc defendant lawyer: %w", err)
		}
		defendant.Environment, err = participantEnvironmentFor(baseEnvironment, request.Settings, settings.DefendantProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare adc defendant lawyer environment: %w", err)
		}
		defendant.StateDir = participantStateDir(request.SessionDir, settings.DefendantProfile, "defendant")
		defendant.WorkDir = participantWorkDir(request.RecordDir, "defendant")
	}
	timeoutSeconds := 0
	if settings.Timeout != 0 {
		seconds, err := wholeSeconds(settings.Timeout)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("adc timeout: %w", err)
		}
		timeoutSeconds = int(seconds)
	}
	opts := localrun.Options{
		CourtSubmissionErrorLimit: settings.CourtSubmissionErrorLimit,
		CoreCommand:               settings.CoreCommand,
		CoreWorkingDir:            settings.CoreWorkingDir,
		MCPCommand:                settings.MCPCommand,
		MCPWorkingDir:             settings.MCPWorkingDir,
		MCPListenAddr:             settings.MCPListenAddr,
		MCPPublicBaseURL:          settings.MCPPublicBaseURL,
		Proposition:               request.Request.Proposition,
		DocumentsDir:              filepath.Join(request.RecordDir, "inputs", "documents"),
		EvidenceStandard:          request.Settings.Common.EvidenceStandard,
		PromptDir:                 settings.PromptDir,
		PromptFiles:               map[string]string(settings.PromptFiles),
		LauncherPromptDir:         settings.LauncherPromptDir,
		LauncherPromptFiles:       map[string]string(settings.LauncherPromptFiles),
		MaxDocumentFiles:          request.Settings.Common.DocumentLimits.Count,
		MaxDocumentFileBytes:      request.Settings.Common.DocumentLimits.PerFile,
		MaxDocumentsTotalBytes:    request.Settings.Common.DocumentLimits.Total,
		OutputDir:                 request.RecordDir,
		CoreOutputDir:             request.CoreDir,
		LogsDir:                   request.LogsDir,
		TrialMode:                 settings.TrialMode,
		JurorTimeoutSeconds:       timeoutSeconds,
		CouncilAllowedEndpoints:   append([]string(nil), request.Settings.Common.CouncilAllowedEndpoints...),
		CouncilMinEndpoints:       request.Settings.Common.CouncilMinEndpoints,
		LawyerTimeoutSeconds:      timeoutSeconds,
		TimeoutSeconds:            timeoutSeconds,
		RunID:                     request.Request.RunID,
		CaseID:                    request.Request.CaseID,
		LawyerWebSearch:           settings.WebSearch,
		AutoLawyers:               settings.AutoLawyers,
		PlaintiffLawyer:           plaintiff,
		DefendantLawyer:           defendant,
		CoreEnvironment:           coreEnvironment,
		MCPEnvironment:            mcpEnvironment,
		ParticipantEnvironment:    participantEnvironment,
		ProcessObserver:           request.Observer,
	}
	if settings.TrialMode != "bench" {
		opts.JurorPersonasPath = request.Settings.Common.CouncilPool
		opts.JurorCount = request.Settings.Common.CouncilSize
		opts.MinimumConcurring = request.Settings.Common.RequiredVotes
		if request.Settings.Common.RequiredVotes == request.Settings.Common.CouncilSize {
			opts.UnanimousRequired = "true"
		} else {
			opts.UnanimousRequired = "false"
		}
	}
	run := r.run
	if run == nil {
		run = localrun.Run
	}
	result, runErr := run(ctx, opts)
	return mapADCResult(result, runErr)
}

func resolvedADCLawyerProfile(settings ResolvedSettings, name string) (localrun.LawyerProfile, error) {
	profile, ok := settings.AgentProfiles[name]
	if !ok {
		return localrun.LawyerProfile{}, fmt.Errorf("agent profile %q is absent", name)
	}
	resolved := localrun.LawyerProfile{
		Name:            name,
		Runner:          localrun.LawyerRunner(profile.Runner),
		Provider:        headless.Provider(profile.Provider),
		Command:         profile.Command,
		Model:           profile.Model,
		ReasoningEffort: profile.ReasoningEffort,
	}
	resume := profile.Resume
	resolved.Resume = &resume
	switch profile.Authentication.Source {
	case AuthSubscription:
		resolved.AuthMode = headless.AuthSubscription
		resolved.CredentialsFile = profile.Authentication.Path
	case AuthAPIKey:
		resolved.AuthMode = headless.AuthAPIKey
		resolved.APIKeyEnv = profile.Authentication.EnvironmentVariable
	default:
		return localrun.LawyerProfile{}, fmt.Errorf("unsupported authentication source %q", profile.Authentication.Source)
	}
	return resolved, nil
}

func mapADCResult(result localrun.Result, runErr error) (ProcedureOutcome, error) {
	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("encode adc core result: %w", marshalErr)), Provider: result.Provider}
	}
	caseState, ok := result.FinalState["case"].(map[string]any)
	if !ok {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("adc core result has no case state")), Provider: result.Provider}
	}
	phase := strings.TrimSpace(adcResultString(caseState["phase"]))
	if phase == "" {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("adc core returned an empty phase")), Provider: result.Provider}
	}
	if runErr != nil {
		return ProcedureOutcome{}, &CoreRunError{Err: runErr, Provider: result.Provider}
	}
	status := strings.TrimSpace(adcResultString(caseState["status"]))
	if status != "judgment_entered" && status != "closed" {
		return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("adc core returned nonterminal case status %q", status), Provider: result.Provider}
	}
	resolution := strings.TrimSpace(adcResultString(caseState["resolution"]))
	if resolution != "demonstrated" && resolution != "not_demonstrated" && resolution != "no_decision" {
		return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("adc core returned invalid resolution %q", resolution), Provider: result.Provider}
	}
	return ProcedureOutcome{
		Status: StatusOK,
		Phase:  phase,
		Decision: &Decision{
			Kind:  "binary",
			Value: resolution,
		},
		ProcedureResult: raw,
		Provider:        result.Provider,
	}, nil
}

func adcResultString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
