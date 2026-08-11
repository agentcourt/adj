package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsmorph/adj/adc/runtime/casegen"
	"github.com/jsmorph/adj/adc/runtime/courts"
	"github.com/jsmorph/adj/adc/runtime/lean"
	"github.com/jsmorph/adj/adc/runtime/report"
	"github.com/jsmorph/adj/adc/runtime/runner"
	"github.com/jsmorph/adj/adc/runtime/store"
	"github.com/jsmorph/adj/common/openai"
)

func RunCase(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("case", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc case --complaint <markdown> [options]\n\n")
		fs.PrintDefaults()
	})
	complaintPath := fs.String("complaint", "", "Path to complaint markdown")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	outDir := fs.String("out-dir", "out/case", "Output directory for staged inputs and run evidence")
	model := fs.String("model", casegen.DefaultRuntimeModel(), "Runtime model for litigation agents")
	nonJurorModel := fs.String("non-juror-model", casegen.DefaultNonJurorModel(), "Runtime model for judge, lawyers, and clerk")
	plaintiffModel := fs.String("plaintiff-model", "", "Runtime model for plaintiff counsel. Default: --non-juror-model")
	defendantModel := fs.String("defendant-model", "", "Runtime model for defense counsel. Default: --non-juror-model")
	judgeModel := fs.String("judge-model", "", "Runtime model for the judge. Default: --non-juror-model")
	clerkModel := fs.String("clerk-model", "", "Runtime model for the clerk. Default: --non-juror-model")
	plannerModel := fs.String("planner-model", casegen.DefaultPlannerModel(), "Model for neutral intake and strategy planning")
	reportModel := fs.String("report-model", casegen.DefaultRuntimeModel(), "Model for digest generation")
	temperature := fs.String("temperature", "", "Override runtime temperature")
	nonJurorTemperature := fs.String("non-juror-temperature", "", "Override runtime temperature for judge, lawyers, and clerk")
	jurorTemperature := fs.String("juror-temperature", "", "Override runtime temperature for jurors only")
	jurorPersonas := fs.String("juror-personas", defaultPersonaRecordsPath(), "Path to juror model/persona pairs file")
	trialMode := fs.String("trial-mode", "auto", "Trial mode override: auto, jury, or bench")
	skipVoirDire := fs.Bool("skip-voir-dire", false, "Skip questionnaires and voir dire, then empanel randomly from the candidate panel")
	jurorCount := fs.Int("juror-count", 0, "Jury size for jury trials, 6 through 12. Omit to use the scenario or court default")
	minimumConcurring := fs.Int("minimum-concurring", 0, "Minimum concurring jurors needed for a verdict. Omit to use the scenario or court default")
	unanimousRequired := fs.String("unanimous-required", "", "Whether the jury verdict must be unanimous: true or false. Omit to use the scenario or court default")
	online := fs.Bool("online", false, "Enable web search tool for planning and litigation agents")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout in seconds")
	maxResponseBytes := fs.Int("max-response-bytes", runner.DefaultMaxResponseBytes, "Maximum bytes allowed in one direct-runtime model response")
	var externalRoles stringListFlag
	caseID := fs.String("case-id", "", "Case ID for role API clients. Default: run id")
	caseAPIAddr := fs.String("caseapi-addr", "", "Listen address for the role API, for example 127.0.0.1:9001")
	roleAPITimeoutSeconds := fs.Int("roleapi-timeout-seconds", defaultRoleAPITimeoutSeconds, "Timeout in seconds for each external role opportunity")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", runner.DefaultInvalidAttemptLimit, "Maximum invalid model responses before a turn fails")
	runID := fs.String("run-id", "", "Run ID override")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	jsonSummary := fs.Bool("json-summary", true, "Emit JSON summary to stdout")
	fs.Var(&externalRoles, "external-role", "Role to serve through the role API during opportunity turns; repeat as needed")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*complaintPath) == "" {
		return fmt.Errorf("--complaint is required")
	}
	if strings.TrimSpace(*outDir) == "" {
		return fmt.Errorf("--out-dir is required")
	}
	resolvedReportModel := resolveDefault(*reportModel, casegen.DefaultRuntimeModel())
	timeout := time.Duration(*timeoutSeconds) * time.Second
	var client *openai.Client
	var jurorClient *openai.Client
	client, err := openai.NewFromEnv(*online, timeout)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*jurorPersonas) != "" {
		jurorClient, err = openai.NewFromEnv(*online, timeout)
		if err != nil {
			return err
		}
	}

	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	nonJurorTempPtr, err := parseOptionalFloat(*nonJurorTemperature)
	if err != nil {
		return fmt.Errorf("parse --non-juror-temperature: %w", err)
	}
	jurorTempPtr, err := parseOptionalFloat(*jurorTemperature)
	if err != nil {
		return fmt.Errorf("parse --juror-temperature: %w", err)
	}
	unanimousRequiredPtr, err := parseOptionalBool(*unanimousRequired)
	if err != nil {
		return fmt.Errorf("parse --unanimous-required: %w", err)
	}
	policyOverrides, err := juryPolicyOverrides(*jurorCount, *minimumConcurring, unanimousRequiredPtr)
	if err != nil {
		return err
	}

	setup, err := prepareComplaintScenario(ctx, client, complaintSetupOptions{
		ComplaintPath:       *complaintPath,
		CourtRef:            *courtRef,
		OutDir:              *outDir,
		RuntimeModel:        *model,
		PlannerModel:        *plannerModel,
		NonJurorModel:       *nonJurorModel,
		PlaintiffModel:      *plaintiffModel,
		DefendantModel:      *defendantModel,
		JudgeModel:          *judgeModel,
		ClerkModel:          *clerkModel,
		Temperature:         tempPtr,
		NonJurorTemperature: nonJurorTempPtr,
		TrialModeOverride:   *trialMode,
		SkipVoirDire:        *skipVoirDire,
		JurorCount:          *jurorCount,
		MinimumConcurring:   *minimumConcurring,
		UnanimousRequired:   unanimousRequiredPtr,
	})
	if err != nil {
		return err
	}

	normalizedCasePath := setup.NormalizedCasePath
	plaintiffStrategyPath := setup.PlaintiffStrategyPath
	defenseStrategyPath := setup.DefenseStrategyPath
	scenarioPath := setup.ScenarioPath
	outputPath := filepath.Join(*outDir, "run.json")
	runtimePath := filepath.Join(*outDir, "runtime.json")
	eventsPath := filepath.Join(*outDir, "events.ndjson")
	dbPath := filepath.Join(*outDir, "run.db")
	transcriptPath := filepath.Join(*outDir, "transcript.md")
	digestPath := filepath.Join(*outDir, "digest.md")

	runtimeLimits := runner.RuntimeLimits{
		LLMTimeoutSeconds:     *timeoutSeconds,
		RoleAPITimeoutSeconds: *roleAPITimeoutSeconds,
		MaxResponseBytes:      *maxResponseBytes,
		InvalidAttemptLimit:   *invalidAttemptLimit,
	}.Normalized()
	if err := writeJSONFile(runtimePath, runtimeLimits); err != nil {
		return err
	}

	effectiveRunID := strings.TrimSpace(*runID)
	if effectiveRunID == "" {
		effectiveRunID = fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	closeStore := func(err error) error {
		if closeErr := st.Close(); closeErr != nil {
			return errors.Join(err, fmt.Errorf("close sqlite: %w", closeErr))
		}
		return err
	}

	engine := lean.New(strings.Fields(strings.TrimSpace(*engineCommand)))

	r, err := runner.New(st, engine, client, jurorClient, runner.Config{
		ScenarioPath:      scenarioPath,
		OutputPath:        outputPath,
		EventsPath:        eventsPath,
		RunID:             effectiveRunID,
		CaseID:            resolveDefault(*caseID, effectiveRunID),
		CaseAPIAddr:       strings.TrimSpace(*caseAPIAddr),
		ExternalRoles:     []string(externalRoles),
		Model:             setup.RuntimeModel,
		Temperature:       tempPtr,
		JurorTemperature:  jurorTempPtr,
		JurorPersonasPath: strings.TrimSpace(*jurorPersonas),
		Runtime:           runtimeLimits,
		PolicyOverrides:   policyOverrides,
	})
	if err != nil {
		return closeStore(err)
	}
	result, err := r.Run(ctx)
	if closeErr := closeStore(err); closeErr != nil {
		return closeErr
	}
	if err := report.WriteTranscript(transcriptPath, result); err != nil {
		return err
	}
	if err := report.WriteDigestWithClient(digestPath, result, resolvedReportModel, client); err != nil {
		return err
	}

	summary := map[string]any{
		"run_id":             effectiveRunID,
		"complaint":          setup.Complaint.StagedRelPath,
		"normalized_case":    normalizedCasePath,
		"plaintiff_strategy": plaintiffStrategyPath,
		"defense_strategy":   defenseStrategyPath,
		"generated_scenario": scenarioPath,
		"output":             outputPath,
		"runtime":            runtimePath,
		"events":             eventsPath,
		"db":                 dbPath,
		"transcript":         transcriptPath,
		"digest":             digestPath,
	}
	if *jsonSummary {
		payload, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, string(payload))
		return err
	}
	_, err = fmt.Fprintf(stdout, "run_id=%s out_dir=%s scenario=%s output=%s runtime=%s digest=%s transcript=%s\n", effectiveRunID, *outDir, scenarioPath, outputPath, runtimePath, digestPath, transcriptPath)
	return err
}
