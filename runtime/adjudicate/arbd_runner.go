package adjudicate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	headless "github.com/agentcourt/adj/runtime/agent"
	localrun "github.com/agentcourt/adj/runtime/localrun/arbd"
)

type arbdRunFunc func(context.Context, localrun.Options) (localrun.Result, error)

type ARBDRunner struct {
	run arbdRunFunc
}

func (r ARBDRunner) Run(ctx context.Context, request ProcedureRequest) (ProcedureOutcome, error) {
	settings := request.Settings.Procedure.ARBD
	if settings == nil {
		return ProcedureOutcome{}, fmt.Errorf("resolved arbd settings are absent")
	}
	baseEnvironment := os.Environ()
	coreEnvironment, err := coreProviderEnvironmentFor(request.Settings, baseEnvironment, configuredProviderNames(request.Settings.Common.ProviderCredentials)...)
	if err != nil {
		return ProcedureOutcome{}, err
	}
	mcpEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	participantEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	var plaintiff, defendant localrun.LawyerProfile
	if automaticLawyerEnabled(settings.AutoLawyers, "plaintiff") {
		plaintiff, err = resolvedAARDLawyerProfile(request.Settings, settings.PlaintiffProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("resolve arbd plaintiff lawyer: %w", err)
		}
		plaintiff.Environment, err = participantEnvironmentFor(baseEnvironment, request.Settings, settings.PlaintiffProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare arbd plaintiff lawyer environment: %w", err)
		}
		plaintiff.StateDir = participantStateDir(request.SessionDir, settings.PlaintiffProfile, "plaintiff")
		plaintiff.WorkDir = participantWorkDir(request.RecordDir, "plaintiff")
	}
	if automaticLawyerEnabled(settings.AutoLawyers, "defendant") {
		defendant, err = resolvedAARDLawyerProfile(request.Settings, settings.DefendantProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("resolve arbd defendant lawyer: %w", err)
		}
		defendant.Environment, err = participantEnvironmentFor(baseEnvironment, request.Settings, settings.DefendantProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare arbd defendant lawyer environment: %w", err)
		}
		defendant.StateDir = participantStateDir(request.SessionDir, settings.DefendantProfile, "defendant")
		defendant.WorkDir = participantWorkDir(request.RecordDir, "defendant")
	}

	timeoutSeconds := 0
	if settings.Timeout != 0 {
		seconds, err := wholeSeconds(settings.Timeout)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("arbd timeout: %w", err)
		}
		timeoutSeconds = int(seconds)
	}
	complaintPath := filepath.Join(request.RecordDir, "inputs", "arbd-complaint.md")
	if err := writeARBDComplaint(complaintPath, request.Request.Proposition); err != nil {
		return ProcedureOutcome{}, err
	}
	caseFiles, err := formalCaseFilePaths(request.RecordDir, request.Documents)
	if err != nil {
		return ProcedureOutcome{}, err
	}
	run := r.run
	if run == nil {
		run = localrun.Run
	}
	opts := localrun.Options{
		CoreCommand:             settings.CoreCommand,
		CoreWorkingDir:          settings.CoreWorkingDir,
		MCPCommand:              settings.MCPCommand,
		MCPWorkingDir:           settings.MCPWorkingDir,
		MCPListenAddr:           settings.MCPListenAddr,
		MCPPublicBaseURL:        settings.MCPPublicBaseURL,
		ComplaintPath:           complaintPath,
		CaseFiles:               caseFiles,
		OutputDir:               request.RecordDir,
		CoreOutputDir:           request.CoreDir,
		LogsDir:                 request.LogsDir,
		CouncilSize:             request.Settings.Common.CouncilSize,
		JudgmentStandard:        settings.JudgmentStandard,
		PromptDir:               settings.PromptDir,
		PromptFiles:             map[string]string(settings.PromptFiles),
		LauncherPromptDir:       settings.LauncherPromptDir,
		LauncherPromptFiles:     map[string]string(settings.LauncherPromptFiles),
		CouncilPoolPath:         request.Settings.Common.CouncilPool,
		CouncilAllowedEndpoints: append([]string(nil), request.Settings.Common.CouncilAllowedEndpoints...),
		CouncilMinEndpoints:     request.Settings.Common.CouncilMinEndpoints,
		CouncilTimeoutSeconds:   timeoutSeconds,
		LawyerTimeoutSeconds:    timeoutSeconds,
		RunID:                   request.Request.RunID,
		CaseID:                  request.Request.CaseID,
		AutoLawyers:             settings.AutoLawyers,
		LawyerWebSearch:         settings.WebSearch,
		PlaintiffLawyer:         plaintiff,
		DefendantLawyer:         defendant,
		CoreEnvironment:         coreEnvironment,
		MCPEnvironment:          mcpEnvironment,
		ParticipantEnvironment:  participantEnvironment,
		ProcessObserver:         request.Observer,
	}
	result, runErr := run(ctx, opts)
	return mapARBDResult(request.Request.CaseID, request.Request.RunID, result, runErr)
}

func resolvedAARDLawyerProfile(settings ResolvedSettings, name string) (localrun.LawyerProfile, error) {
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

func writeARBDComplaint(path, proposition string) error {
	content := "# Question\n\n" + strings.TrimSpace(proposition) + "\n"
	return writeAtomic(path, 0o644, func(file *os.File) error {
		written, err := io.WriteString(file, content)
		if written != len(content) {
			err = errors.Join(err, io.ErrShortWrite)
		}
		return err
	})
}

func mapARBDResult(caseID, runID string, result localrun.Result, runErr error) (ProcedureOutcome, error) {
	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("encode arbd core result: %w", marshalErr))}
	}
	if result.CaseID != caseID || result.RunID != runID {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("arbd core returned case/run identity %q/%q, want %q/%q", result.CaseID, result.RunID, caseID, runID))}
	}
	phase := strings.TrimSpace(result.Phase)
	if phase == "" {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("arbd core returned an empty phase"))}
	}
	if runErr != nil {
		message := strings.TrimSpace(result.Error)
		if message == "" {
			message = "arbd runner failed"
		}
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(errors.New(message), runErr)}
	}

	switch strings.TrimSpace(result.Status) {
	case "ok":
		if phase != "closed" {
			return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("arbd core returned successful status in phase %q", result.Phase)}
		}
		answers := result.Answers
		if answers == nil {
			answers = map[string]int{}
		}
		for member, score := range answers {
			if strings.TrimSpace(member) == "" {
				return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("arbd core returned an answer with an empty council member ID")}
			}
			if score < 0 || score > 100 {
				return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("arbd core returned score %d for %s outside 0 through 100", score, member)}
			}
		}
		value, err := json.Marshal(answers)
		if err != nil {
			return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("encode arbd council answers: %w", err)}
		}
		return ProcedureOutcome{
			Status:          StatusOK,
			Phase:           phase,
			Decision:        &Decision{Kind: "council_answers", Value: string(value)},
			ProcedureResult: raw,
		}, nil
	case "failed":
		if len(result.Failure) == 0 {
			return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("arbd core returned failed status without failure data")}
		}
		failure, err := json.Marshal(result.Failure)
		if err != nil {
			return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("encode arbd failure: %w", err)}
		}
		return ProcedureOutcome{
			Status:          StatusFailed,
			Phase:           phase,
			ProcedureResult: raw,
			Failure:         failure,
		}, nil
	default:
		return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("arbd core returned invalid status %q", result.Status)}
	}
}
