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
	localrun "github.com/agentcourt/adj/runtime/localrun/arb"
)

type arbRunFunc func(context.Context, localrun.Options) (localrun.Result, error)

type ARBRunner struct {
	run arbRunFunc
}

func (r ARBRunner) Run(ctx context.Context, request ProcedureRequest) (ProcedureOutcome, error) {
	settings := request.Settings.Procedure.ARB
	if settings == nil {
		return ProcedureOutcome{}, fmt.Errorf("resolved arb settings are absent")
	}
	baseEnvironment := os.Environ()
	coreEnvironment, err := coreProviderEnvironmentFor(request.Settings, baseEnvironment, "openrouter")
	if err != nil {
		return ProcedureOutcome{}, err
	}
	mcpEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	participantEnvironment := credentialFreeEnvironment(baseEnvironment, request.Settings)
	var plaintiff, defendant localrun.LawyerProfile
	if automaticLawyerEnabled(settings.AutoLawyers, "plaintiff") {
		plaintiff, err = resolvedAARLawyerProfile(request.Settings, settings.PlaintiffProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("resolve arb plaintiff lawyer: %w", err)
		}
		plaintiff.Environment, err = participantEnvironmentFor(baseEnvironment, request.Settings, settings.PlaintiffProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare arb plaintiff lawyer environment: %w", err)
		}
		plaintiff.StateDir = participantStateDir(request.SessionDir, settings.PlaintiffProfile, "plaintiff")
		plaintiff.WorkDir = participantWorkDir(request.RecordDir, "plaintiff")
	}
	if automaticLawyerEnabled(settings.AutoLawyers, "defendant") {
		defendant, err = resolvedAARLawyerProfile(request.Settings, settings.DefendantProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("resolve arb defendant lawyer: %w", err)
		}
		defendant.Environment, err = participantEnvironmentFor(baseEnvironment, request.Settings, settings.DefendantProfile)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("prepare arb defendant lawyer environment: %w", err)
		}
		defendant.StateDir = participantStateDir(request.SessionDir, settings.DefendantProfile, "defendant")
		defendant.WorkDir = participantWorkDir(request.RecordDir, "defendant")
	}
	timeoutSeconds := 0
	if settings.Timeout != 0 {
		seconds, err := wholeSeconds(settings.Timeout)
		if err != nil {
			return ProcedureOutcome{}, fmt.Errorf("arb timeout: %w", err)
		}
		timeoutSeconds = int(seconds)
	}

	complaintPath := filepath.Join(request.RecordDir, "inputs", "arb-complaint.md")
	if err := writeARBComplaint(complaintPath, request.Request.Proposition); err != nil {
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
		CoreCommand:            settings.CoreCommand,
		CoreWorkingDir:         settings.CoreWorkingDir,
		MCPCommand:             settings.MCPCommand,
		MCPWorkingDir:          settings.MCPWorkingDir,
		MCPListenAddr:          settings.MCPListenAddr,
		MCPPublicBaseURL:       settings.MCPPublicBaseURL,
		ComplaintPath:          complaintPath,
		CaseFiles:              caseFiles,
		OutputDir:              request.RecordDir,
		CoreOutputDir:          request.CoreDir,
		LogsDir:                request.LogsDir,
		CouncilSize:            request.Settings.Common.CouncilSize,
		RequiredVotes:          request.Settings.Common.RequiredVotes,
		EvidenceStandard:       request.Settings.Common.EvidenceStandard,
		PromptDir:              settings.PromptDir,
		PromptFiles:            map[string]string(settings.PromptFiles),
		LauncherPromptDir:      settings.LauncherPromptDir,
		LauncherPromptFiles:    map[string]string(settings.LauncherPromptFiles),
		CouncilPoolPath:        request.Settings.Common.CouncilPool,
		CouncilTimeoutSeconds:  timeoutSeconds,
		LawyerTimeoutSeconds:   timeoutSeconds,
		RunID:                  request.Request.RunID,
		CaseID:                 request.Request.CaseID,
		LawyerWebSearch:        settings.WebSearch,
		AutoLawyers:            settings.AutoLawyers,
		PlaintiffLawyer:        plaintiff,
		DefendantLawyer:        defendant,
		CoreEnvironment:        coreEnvironment,
		MCPEnvironment:         mcpEnvironment,
		ParticipantEnvironment: participantEnvironment,
		ProcessObserver:        request.Observer,
	}
	result, runErr := run(ctx, opts)
	return mapARBResult(result, runErr)
}

func resolvedAARLawyerProfile(settings ResolvedSettings, name string) (localrun.LawyerProfile, error) {
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

func writeARBComplaint(path, proposition string) error {
	content := "# Proposition\n\n" + strings.TrimSpace(proposition) + "\n"
	return writeAtomic(path, 0o644, func(file *os.File) error {
		written, err := io.WriteString(file, content)
		if written != len(content) {
			err = errors.Join(err, io.ErrShortWrite)
		}
		return err
	})
}

func formalCaseFilePaths(recordDir string, manifest DocumentManifest) ([]string, error) {
	root := filepath.Join(recordDir, "inputs", "documents")
	paths := make([]string, 0, len(manifest.Documents))
	for _, document := range manifest.Documents {
		if !confinedRelativePath(filepath.FromSlash(document.Path)) {
			return nil, fmt.Errorf("staged document path %q is invalid", document.Path)
		}
		path := filepath.Join(root, filepath.FromSlash(document.Path))
		if !pathWithin(root, path) {
			return nil, fmt.Errorf("staged document path %q escapes its root", document.Path)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func mapARBResult(result localrun.Result, runErr error) (ProcedureOutcome, error) {
	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("encode arb core result: %w", marshalErr)), Provider: result.Provider}
	}
	phase := strings.TrimSpace(result.Phase)
	if phase == "" {
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("arb core returned an empty phase")), Provider: result.Provider}
	}
	switch strings.TrimSpace(result.Status) {
	case "ok":
		if runErr != nil {
			return ProcedureOutcome{}, &CoreRunError{Err: runErr, Provider: result.Provider}
		}
		resolution := strings.TrimSpace(result.Resolution)
		if resolution != "demonstrated" && resolution != "not_demonstrated" && resolution != "no_majority" {
			return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("arb core returned invalid resolution %q", result.Resolution), Provider: result.Provider}
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
	case "failed":
		if runErr != nil {
			message := strings.TrimSpace(result.Error)
			if message == "" {
				message = "arb runner failed"
			}
			return ProcedureOutcome{}, &CoreRunError{Class: result.ErrorClass, Err: errors.Join(errors.New(message), runErr), Provider: result.Provider}
		}
		failure, err := json.Marshal(result.Failure)
		if err != nil {
			return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("encode arb failure: %w", err), Provider: result.Provider}
		}
		if len(result.Failure) == 0 {
			return ProcedureOutcome{}, &CoreRunError{Err: fmt.Errorf("arb core returned failed status without failure data"), Provider: result.Provider}
		}
		return ProcedureOutcome{
			Status:          StatusFailed,
			Phase:           phase,
			ProcedureResult: raw,
			Failure:         failure,
			Provider:        result.Provider,
		}, nil
	default:
		return ProcedureOutcome{}, &CoreRunError{Err: errors.Join(runErr, fmt.Errorf("arb core returned invalid status %q", result.Status)), Provider: result.Provider}
	}
}
