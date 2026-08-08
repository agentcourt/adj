package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jsmorph/adj/adc/runtime/runner"
)

func RunValidate(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("validate", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc validate --scenario <json>\n\n")
		fs.PrintDefaults()
	})
	scenarioPath := fs.String("scenario", "", "Path to scenario JSON")
	jsonSummary := fs.Bool("json", true, "Emit JSON summary")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*scenarioPath) == "" {
		return fmt.Errorf("--scenario is required")
	}

	report, err := runner.ValidateScenarioFile(strings.TrimSpace(*scenarioPath))
	if err != nil {
		return err
	}
	result := map[string]any{
		"scenario_name":             report.ScenarioName,
		"valid":                     report.Valid(),
		"requires_llm":              report.RequiresLLM,
		"unknown_roles":             report.UnknownRoles,
		"missing_action_type_turns": report.MissingActionTypes,
		"unsupported_actions":       report.UnsupportedActions,
	}
	if *jsonSummary {
		wire, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(stdout, string(wire)); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(
			stdout,
			"scenario=%s valid=%t requires_llm=%t unknown_roles=%d missing_action_type_turns=%d unsupported_actions=%d\n",
			report.ScenarioName,
			report.Valid(),
			report.RequiresLLM,
			len(report.UnknownRoles),
			len(report.MissingActionTypes),
			len(report.UnsupportedActions),
		); err != nil {
			return err
		}
	}
	if !report.Valid() {
		return fmt.Errorf("scenario validation failed")
	}
	return nil
}
