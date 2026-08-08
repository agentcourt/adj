package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jsmorph/adj/adc/runtime/casegen"
	"github.com/jsmorph/adj/adc/runtime/courts"
	"github.com/jsmorph/adj/common/openai"
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
}

type complaintSetupResult struct {
	RuntimeModel          string
	Complaint             casegen.ComplaintInput
	NormalizedCasePath    string
	PlaintiffStrategyPath string
	DefenseStrategyPath   string
	ScenarioPath          string
}

func prepareComplaintScenario(ctx context.Context, client *openai.Client, opts complaintSetupOptions) (complaintSetupResult, error) {
	if strings.TrimSpace(opts.ComplaintPath) == "" {
		return complaintSetupResult{}, fmt.Errorf("complaint path is required")
	}
	if strings.TrimSpace(opts.OutDir) == "" {
		return complaintSetupResult{}, fmt.Errorf("out dir is required")
	}
	if client == nil {
		return complaintSetupResult{}, fmt.Errorf("OpenAI client is required")
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return complaintSetupResult{}, fmt.Errorf("create out dir: %w", err)
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
		return complaintSetupResult{}, err
	}
	court, err := courts.Resolve(opts.CourtRef)
	if err != nil {
		return complaintSetupResult{}, err
	}
	complaint, err = casegen.StageComplaintAssets(opts.OutDir, complaint)
	if err != nil {
		return complaintSetupResult{}, err
	}

	plan, err := casegen.CreatePlan(ctx, client, resolvedPlannerModel, complaint, court)
	if err != nil {
		return complaintSetupResult{}, err
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
	})
	if err != nil {
		return complaintSetupResult{}, err
	}

	result := complaintSetupResult{
		RuntimeModel:          resolvedRuntimeModel,
		Complaint:             complaint,
		NormalizedCasePath:    filepath.Join(opts.OutDir, "normalized-case.json"),
		PlaintiffStrategyPath: filepath.Join(opts.OutDir, "plaintiff-strategy.md"),
		DefenseStrategyPath:   filepath.Join(opts.OutDir, "defense-strategy.md"),
		ScenarioPath:          filepath.Join(opts.OutDir, "generated-scenario.json"),
	}
	if err := writeJSONFile(result.NormalizedCasePath, plan.Packet); err != nil {
		return complaintSetupResult{}, err
	}
	if err := os.WriteFile(result.PlaintiffStrategyPath, []byte(strings.TrimSpace(plan.PlaintiffStrategy)+"\n"), 0o644); err != nil {
		return complaintSetupResult{}, fmt.Errorf("write plaintiff strategy: %w", err)
	}
	if err := os.WriteFile(result.DefenseStrategyPath, []byte(strings.TrimSpace(plan.DefenseStrategy)+"\n"), 0o644); err != nil {
		return complaintSetupResult{}, fmt.Errorf("write defense strategy: %w", err)
	}
	if err := writeJSONFile(result.ScenarioPath, scenario); err != nil {
		return complaintSetupResult{}, err
	}
	return result, nil
}
