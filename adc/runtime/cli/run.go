package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jsmorph/adj/adc/runtime/lean"
	"github.com/jsmorph/adj/adc/runtime/report"
	"github.com/jsmorph/adj/adc/runtime/runner"
	"github.com/jsmorph/adj/adc/runtime/store"
	"github.com/jsmorph/adj/common/openai"
)

func RunScenarioCase(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("scenario", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc scenario --scenario <json> [options]\n\n")
		fs.PrintDefaults()
	})
	scenarioPath := fs.String("scenario", "", "Path to scenario JSON")
	outputPath := fs.String("output", "out/adc-run.json", "Run evidence output path")
	runtimePath := fs.String("runtime", "out/adc-runtime.json", "Runtime limits evidence output path")
	eventsPath := fs.String("events", "out/adc-actions.ndjson", "Event log output path")
	dbPath := fs.String("db", "out/adc-run.db", "SQLite path")
	model := fs.String("model", "", "Override the scenario default model for roles without their own model")
	temperature := fs.String("temperature", "", "Override the scenario default temperature for roles without their own temperature")
	jurorTemperature := fs.String("juror-temperature", "", "Override runtime temperature for jurors only")
	jurorPersonas := fs.String("juror-personas", defaultPersonaRecordsPath(), "Path to juror model/persona pairs file")
	jurorCount := fs.Int("juror-count", 0, "Jury size for jury trials, 6 through 12. Omit to use the scenario or court default")
	minimumConcurring := fs.Int("minimum-concurring", 0, "Minimum concurring jurors needed for a verdict. Omit to use the scenario or court default")
	unanimousRequired := fs.String("unanimous-required", "", "Whether the jury verdict must be unanimous: true or false. Omit to use the scenario or court default")
	online := fs.Bool("online", false, "Enable web search tool")
	offline := fs.Bool("offline", false, "Disable LLM calls; only deterministic turns")
	var externalRoles stringListFlag
	caseID := fs.String("case-id", "", "Case ID for role API clients. Default: run id")
	caseAPIAddr := fs.String("caseapi-addr", "", "Listen address for the role API, for example 127.0.0.1:9001")
	roleAPITimeoutSeconds := fs.Int("roleapi-timeout-seconds", defaultRoleAPITimeoutSeconds, "Timeout in seconds for each external role opportunity")
	maxResponseBytes := fs.Int("max-response-bytes", runner.DefaultMaxResponseBytes, "Maximum bytes allowed in one direct-runtime model response")
	runID := fs.String("run-id", "", "Run ID override")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", runner.DefaultInvalidAttemptLimit, "Maximum invalid model responses before a turn fails")
	jsonSummary := fs.Bool("json-summary", true, "Emit JSON summary to stdout")
	transcriptPath := fs.String("transcript", "", "Optional transcript markdown output path")
	digestPath := fs.String("digest", "", "Optional digest/report markdown output path")
	reportModel := fs.String("report-model", "", "Model for digest generation")
	allowAssertionFailures := fs.Bool("allow-assertion-failures", false, "Return success after recording failed scenario assertions")
	fs.Var(&externalRoles, "external-role", "Role to serve through the role API during opportunity turns; repeat as needed")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc scenario accepts no positional arguments")
	}
	if strings.TrimSpace(*scenarioPath) == "" {
		return fmt.Errorf("--scenario is required")
	}
	for _, path := range []string{*outputPath, *runtimePath, *eventsPath, *dbPath, *transcriptPath, *digestPath} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := ensureParentDir(path); err != nil {
			return err
		}
	}
	effectiveRunID := strings.TrimSpace(*runID)
	if effectiveRunID == "" {
		effectiveRunID = fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	}
	st, err := store.Open(*dbPath)
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
	var client *openai.Client
	var jurorClient *openai.Client
	resolvedModel := strings.TrimSpace(*model)
	if !*offline {
		client, err = openai.NewFromEnv(*online, time.Duration(*timeoutSeconds)*time.Second)
		if err != nil {
			return closeStore(err)
		}
		if strings.TrimSpace(*jurorPersonas) != "" {
			jurorClient, err = openai.NewFromEnv(*online, time.Duration(*timeoutSeconds)*time.Second)
			if err != nil {
				return closeStore(err)
			}
		}
	}

	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return closeStore(fmt.Errorf("parse --temperature: %w", err))
	}
	jurorTempPtr, err := parseOptionalFloat(*jurorTemperature)
	if err != nil {
		return closeStore(fmt.Errorf("parse --juror-temperature: %w", err))
	}
	unanimousRequiredPtr, err := parseOptionalBool(*unanimousRequired)
	if err != nil {
		return closeStore(fmt.Errorf("parse --unanimous-required: %w", err))
	}
	policyOverrides, err := juryPolicyOverrides(*jurorCount, *minimumConcurring, unanimousRequiredPtr)
	if err != nil {
		return closeStore(err)
	}

	runtimeLimits := runner.RuntimeLimits{
		LLMTimeoutSeconds:     *timeoutSeconds,
		RoleAPITimeoutSeconds: *roleAPITimeoutSeconds,
		MaxResponseBytes:      *maxResponseBytes,
		InvalidAttemptLimit:   *invalidAttemptLimit,
	}.Normalized()
	if err := writeJSONFile(*runtimePath, runtimeLimits); err != nil {
		return closeStore(err)
	}

	r, err := runner.New(st, engine, client, jurorClient, runner.Config{
		ScenarioPath:      *scenarioPath,
		OutputPath:        *outputPath,
		EventsPath:        *eventsPath,
		RunID:             effectiveRunID,
		CaseID:            resolveDefault(*caseID, effectiveRunID),
		CaseAPIAddr:       strings.TrimSpace(*caseAPIAddr),
		ExternalRoles:     []string(externalRoles),
		Model:             resolvedModel,
		Temperature:       tempPtr,
		JurorTemperature:  jurorTempPtr,
		JurorPersonasPath: strings.TrimSpace(*jurorPersonas),
		Runtime:           runtimeLimits,
		Offline:           *offline,
		PolicyOverrides:   policyOverrides,
	})
	if err != nil {
		return closeStore(err)
	}
	if *offline && r.RequiresLLMTurns() {
		if _, err := fmt.Fprintln(stderr, "warning: --offline is set, but scenario includes non-deterministic turns that require an LLM"); err != nil {
			return closeStore(fmt.Errorf("write offline warning: %w", err))
		}
	}
	result, err := r.Run(ctx)
	if closeErr := closeStore(err); closeErr != nil {
		return closeErr
	}
	failed := 0
	for _, a := range result.Assertions {
		if passed, _ := a["passed"].(bool); !passed {
			failed++
		}
	}
	summary := map[string]any{
		"scenario":           result.Scenario,
		"assertion_failures": failed,
		"output":             *outputPath,
		"runtime":            *runtimePath,
		"events":             *eventsPath,
		"db":                 *dbPath,
		"run_id":             effectiveRunID,
	}
	if *jsonSummary {
		payload, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(stdout, string(payload)); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(
			stdout,
			"scenario=%s run_id=%s assertion_failures=%d output=%s runtime=%s events=%s db=%s\n",
			result.Scenario,
			effectiveRunID,
			failed,
			*outputPath,
			*runtimePath,
			*eventsPath,
			*dbPath,
		); err != nil {
			return err
		}
	}
	if err := report.WriteTranscript(strings.TrimSpace(*transcriptPath), result); err != nil {
		return err
	}
	if err := report.WriteDigestWithClient(strings.TrimSpace(*digestPath), result, strings.TrimSpace(*reportModel), client); err != nil {
		return err
	}
	if failed > 0 && !*allowAssertionFailures {
		return fmt.Errorf("assertions failed: %d", failed)
	}
	return nil
}
