package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/casegen"
	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/common/documents"
	"github.com/agentcourt/adj/common/openai"
)

type complaintSetupOptions struct {
	ComplaintPath       string
	CourtRef            string
	OutDir              string
	RuntimeModel        string
	PlannerModel        string
	NonJurorModel       string
	PlaintiffModel      string
	DefendantModel      string
	JudgeModel          string
	ClerkModel          string
	Temperature         *float64
	NonJurorTemperature *float64
	TrialModeOverride   string
	SkipVoirDire        bool
	JurorCount          int
	MinimumConcurring   int
	UnanimousRequired   *bool
	PromptDir           string
	PromptFiles         map[string]string
}

type caseSetupResult struct {
	RuntimeModel          string
	Complaint             casegen.ComplaintInput
	NormalizedCasePath    string
	PlaintiffStrategyPath string
	DefenseStrategyPath   string
	ScenarioPath          string
	DocumentManifestPath  string
	DocumentsPath         string
}

type propositionSetupOptions struct {
	Proposition         string
	EvidenceStandard    string
	DocumentsDir        string
	DocumentLimits      documents.Limits
	OutDir              string
	RuntimeModel        string
	NonJurorModel       string
	PlaintiffModel      string
	DefendantModel      string
	JudgeModel          string
	ClerkModel          string
	Temperature         *float64
	NonJurorTemperature *float64
	TrialModeOverride   string
	SkipVoirDire        bool
	JurorCount          int
	MinimumConcurring   int
	UnanimousRequired   *bool
	PromptDir           string
	PromptFiles         map[string]string
}

func prepareComplaintScenario(ctx context.Context, client *openai.Client, opts complaintSetupOptions) (caseSetupResult, error) {
	if strings.TrimSpace(opts.ComplaintPath) == "" {
		return caseSetupResult{}, fmt.Errorf("complaint path is required")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return caseSetupResult{}, fmt.Errorf("out dir is required")
	}
	if client == nil {
		return caseSetupResult{}, fmt.Errorf("OpenAI client is required")
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return caseSetupResult{}, fmt.Errorf("create out dir: %w", err)
	}

	resolvedRuntimeModel := resolveDefault(opts.RuntimeModel, casegen.DefaultRuntimeModel())
	resolvedPlannerModel := resolveDefault(opts.PlannerModel, casegen.DefaultPlannerModel())
	resolvedNonJurorModel := resolveDefault(opts.NonJurorModel, casegen.DefaultNonJurorModel())
	resolvedPlaintiffModel := resolveDefault(opts.PlaintiffModel, resolvedNonJurorModel)
	resolvedDefendantModel := resolveDefault(opts.DefendantModel, resolvedNonJurorModel)
	resolvedJudgeModel := resolveDefault(opts.JudgeModel, resolvedNonJurorModel)
	resolvedClerkModel := resolveDefault(opts.ClerkModel, resolvedNonJurorModel)

	complaint, err := casegen.LoadComplaint(opts.ComplaintPath)
	if err != nil {
		return caseSetupResult{}, err
	}
	court, err := courts.Resolve(opts.CourtRef)
	if err != nil {
		return caseSetupResult{}, err
	}
	complaint, err = casegen.StageComplaintAssets(opts.OutDir, complaint)
	if err != nil {
		return caseSetupResult{}, err
	}

	plan, err := casegen.CreatePlanWithOptions(ctx, client, resolvedPlannerModel, complaint, court, casegen.PlanningOptions{
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	})
	if err != nil {
		return caseSetupResult{}, err
	}
	scenario, err := casegen.BuildScenario(plan, complaint, casegen.ScenarioOptions{
		RuntimeModel:        resolvedRuntimeModel,
		Temperature:         opts.Temperature,
		NonJurorTemperature: opts.NonJurorTemperature,
		PlaintiffModel:      resolvedPlaintiffModel,
		DefendantModel:      resolvedDefendantModel,
		JudgeModel:          resolvedJudgeModel,
		ClerkModel:          resolvedClerkModel,
		Court:               court,
		TrialModeOverride:   strings.TrimSpace(opts.TrialModeOverride),
		SkipVoirDire:        opts.SkipVoirDire,
		JurorCount:          opts.JurorCount,
		MinimumConcurring:   opts.MinimumConcurring,
		UnanimousRequired:   opts.UnanimousRequired,
		PromptDir:           opts.PromptDir,
		PromptFiles:         opts.PromptFiles,
	})
	if err != nil {
		return caseSetupResult{}, err
	}

	result := caseSetupResult{
		RuntimeModel:          resolvedRuntimeModel,
		Complaint:             complaint,
		NormalizedCasePath:    filepath.Join(opts.OutDir, "normalized-case.json"),
		PlaintiffStrategyPath: filepath.Join(opts.OutDir, "plaintiff-strategy.md"),
		DefenseStrategyPath:   filepath.Join(opts.OutDir, "defense-strategy.md"),
		ScenarioPath:          filepath.Join(opts.OutDir, "generated-scenario.json"),
	}
	if err := writeJSONFile(result.NormalizedCasePath, plan.Packet); err != nil {
		return caseSetupResult{}, err
	}
	if err := os.WriteFile(result.PlaintiffStrategyPath, []byte(strings.TrimSpace(plan.PlaintiffStrategy)+"\n"), 0o644); err != nil {
		return caseSetupResult{}, fmt.Errorf("write plaintiff strategy: %w", err)
	}
	if err := os.WriteFile(result.DefenseStrategyPath, []byte(strings.TrimSpace(plan.DefenseStrategy)+"\n"), 0o644); err != nil {
		return caseSetupResult{}, fmt.Errorf("write defense strategy: %w", err)
	}
	if err := writeJSONFile(result.ScenarioPath, scenario); err != nil {
		return caseSetupResult{}, err
	}
	return result, nil
}

func preparePropositionScenario(opts propositionSetupOptions) (caseSetupResult, error) {
	if strings.TrimSpace(opts.OutDir) == "" {
		return caseSetupResult{}, fmt.Errorf("out dir is required")
	}
	if opts.DocumentLimits.MaxFiles <= 0 {
		return caseSetupResult{}, fmt.Errorf("maximum document files must be positive")
	}
	if opts.DocumentLimits.MaxFileBytes <= 0 {
		return caseSetupResult{}, fmt.Errorf("maximum document file bytes must be positive")
	}
	if opts.DocumentLimits.MaxTotalBytes <= 0 {
		return caseSetupResult{}, fmt.Errorf("maximum documents total bytes must be positive")
	}

	trialMode := strings.ToLower(strings.TrimSpace(opts.TrialModeOverride))
	if trialMode == "" || trialMode == "auto" {
		trialMode = "jury"
	}
	plan, err := casegen.CreatePropositionPlanWithOptions(opts.Proposition, opts.EvidenceStandard, trialMode, casegen.PlanningOptions{
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	})
	if err != nil {
		return caseSetupResult{}, err
	}

	resolvedRuntimeModel := resolveDefault(opts.RuntimeModel, casegen.DefaultRuntimeModel())
	resolvedNonJurorModel := resolveDefault(opts.NonJurorModel, casegen.DefaultNonJurorModel())
	resolvedPlaintiffModel := resolveDefault(opts.PlaintiffModel, resolvedNonJurorModel)
	resolvedDefendantModel := resolveDefault(opts.DefendantModel, resolvedNonJurorModel)
	resolvedJudgeModel := resolveDefault(opts.JudgeModel, resolvedNonJurorModel)
	resolvedClerkModel := resolveDefault(opts.ClerkModel, resolvedNonJurorModel)

	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return caseSetupResult{}, fmt.Errorf("create out dir: %w", err)
	}
	documentsPath := filepath.Join(opts.OutDir, "documents")
	var manifest documents.Manifest
	if strings.TrimSpace(opts.DocumentsDir) == "" {
		if err := os.Mkdir(documentsPath, 0o755); err != nil {
			return caseSetupResult{}, fmt.Errorf("create documents directory: %w", err)
		}
		manifest = documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}}
	} else {
		manifest, err = documents.Import(opts.DocumentsDir, documentsPath, opts.DocumentLimits)
		if err != nil {
			return caseSetupResult{}, err
		}
	}

	result := caseSetupResult{
		RuntimeModel:          resolvedRuntimeModel,
		NormalizedCasePath:    filepath.Join(opts.OutDir, "normalized-case.json"),
		PlaintiffStrategyPath: filepath.Join(opts.OutDir, "plaintiff-strategy.md"),
		DefenseStrategyPath:   filepath.Join(opts.OutDir, "defense-strategy.md"),
		ScenarioPath:          filepath.Join(opts.OutDir, "generated-scenario.json"),
		DocumentManifestPath:  filepath.Join(opts.OutDir, "documents.json"),
		DocumentsPath:         documentsPath,
	}
	if err := writeJSONFile(result.DocumentManifestPath, manifest); err != nil {
		return caseSetupResult{}, err
	}

	scenario, err := casegen.BuildPropositionScenario(plan, manifest, documentsPath, "documents", casegen.ScenarioOptions{
		RuntimeModel:        resolvedRuntimeModel,
		Temperature:         opts.Temperature,
		NonJurorTemperature: opts.NonJurorTemperature,
		PlaintiffModel:      resolvedPlaintiffModel,
		DefendantModel:      resolvedDefendantModel,
		JudgeModel:          resolvedJudgeModel,
		ClerkModel:          resolvedClerkModel,
		Court:               courts.PropositionTribunal(),
		TrialModeOverride:   trialMode,
		SkipVoirDire:        opts.SkipVoirDire,
		JurorCount:          opts.JurorCount,
		MinimumConcurring:   opts.MinimumConcurring,
		UnanimousRequired:   opts.UnanimousRequired,
		PromptDir:           opts.PromptDir,
		PromptFiles:         opts.PromptFiles,
	})
	if err != nil {
		return caseSetupResult{}, err
	}
	if err := writeJSONFile(result.NormalizedCasePath, plan.Packet); err != nil {
		return caseSetupResult{}, err
	}
	if err := os.WriteFile(result.PlaintiffStrategyPath, []byte(strings.TrimSpace(plan.PlaintiffStrategy)+"\n"), 0o644); err != nil {
		return caseSetupResult{}, fmt.Errorf("write plaintiff strategy: %w", err)
	}
	if err := os.WriteFile(result.DefenseStrategyPath, []byte(strings.TrimSpace(plan.DefenseStrategy)+"\n"), 0o644); err != nil {
		return caseSetupResult{}, fmt.Errorf("write defense strategy: %w", err)
	}
	if err := writeJSONFile(result.ScenarioPath, scenario); err != nil {
		return caseSetupResult{}, err
	}
	return result, nil
}
