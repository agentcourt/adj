package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/adc/runtime/courts"
	adceval "github.com/agentcourt/adj/adc/runtime/eval"
	"github.com/agentcourt/adj/adc/runtime/lean"
)

func RunEval(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.Join(fmt.Errorf("eval subcommand is required"), printEvalUsage(stderr))
	}
	switch args[0] {
	case "judge-voir-dire":
		return RunEvalJudgeVoirDire(ctx, args[1:], stdout, stderr)
	case "judge-for-cause":
		return RunEvalJudgeForCause(ctx, args[1:], stdout, stderr)
	case "judge-rule56":
		return RunEvalJudgeRule56(ctx, args[1:], stdout, stderr)
	case "judge-rule12":
		return RunEvalJudgeRule12(ctx, args[1:], stdout, stderr)
	case "judge-rule51":
		return RunEvalJudgeRule51(ctx, args[1:], stdout, stderr)
	case "judge-rule37":
		return RunEvalJudgeRule37(ctx, args[1:], stdout, stderr)
	case "judge-rule11":
		return RunEvalJudgeRule11(ctx, args[1:], stdout, stderr)
	case "judge-rule52":
		return RunEvalJudgeRule52(ctx, args[1:], stdout, stderr)
	case "judge-rule58":
		return RunEvalJudgeRule58(ctx, args[1:], stdout, stderr)
	case "judge-rule60":
		return RunEvalJudgeRule60(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			return printEvalUsage(stdout)
		}
		switch args[1] {
		case "judge-voir-dire":
			return RunEvalJudgeVoirDire(ctx, []string{"-h"}, stdout, stderr)
		case "judge-for-cause":
			return RunEvalJudgeForCause(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule56":
			return RunEvalJudgeRule56(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule12":
			return RunEvalJudgeRule12(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule51":
			return RunEvalJudgeRule51(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule37":
			return RunEvalJudgeRule37(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule11":
			return RunEvalJudgeRule11(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule52":
			return RunEvalJudgeRule52(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule58":
			return RunEvalJudgeRule58(ctx, []string{"-h"}, stdout, stderr)
		case "judge-rule60":
			return RunEvalJudgeRule60(ctx, []string{"-h"}, stdout, stderr)
		default:
			return errors.Join(fmt.Errorf("unknown eval help topic %q", args[1]), printEvalUsage(stderr))
		}
	default:
		return errors.Join(fmt.Errorf("unknown eval subcommand %q", args[0]), printEvalUsage(stderr))
	}
}

func defaultJudgeEvalPath(parts ...string) string {
	return defaultADCPath(append([]string{"evals", "adc", "judge"}, parts...)...)
}

func defaultJudgeEvalOutPath(parts ...string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(append([]string{"evals", "out", "adc", "judge"}, parts...)...)
	}
	return defaultJudgeEvalOutPathFrom(cwd, parts...)
}

func defaultJudgeEvalOutPathFrom(start string, parts ...string) string {
	rel := filepath.Join(append([]string{"evals", "out", "adc", "judge"}, parts...)...)
	moduleRoot := nearestGoModuleRoot(absoluteCleanPath(start))
	if moduleRoot == "" || !directoryExists(filepath.Join(moduleRoot, "evals", "out")) {
		return rel
	}
	return filepath.Join(moduleRoot, rel)
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func RunEvalJudgeVoirDire(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-voir-dire", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-voir-dire [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule47", "voir-dire-question", "fixtures.jsonl"), "Judge voir dire fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected rulings as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeVoirDire(adceval.JudgeVoirDireRescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeVoirDire(ctx, adceval.JudgeVoirDireOptions{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeForCause(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-for-cause", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-for-cause [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule47", "for-cause-challenge", "fixtures.jsonl"), "Judge for-cause fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("for-cause-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected for-cause rulings as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeForCause(adceval.JudgeForCauseRescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeForCause(ctx, adceval.JudgeForCauseOptions{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule56(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule56", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule56 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule56", "summary-judgment", "fixtures.jsonl"), "Judge Rule 56 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule56-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	dryRun := fs.Bool("dry-run", false, "Use expected dispositions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule56(ctx, adceval.JudgeRule56Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule12(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule12", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule12 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule12", "dismissal-jurisdiction", "fixtures.jsonl"), "Judge Rule 12 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule12-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected dispositions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule12(adceval.JudgeRule12RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule12(ctx, adceval.JudgeRule12Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule51(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule51", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule51 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule51", "jury-instructions", "fixtures.jsonl"), "Judge Rule 51 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule51-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected instruction summary as a synthetic model response")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule51(adceval.JudgeRule51RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule51(ctx, adceval.JudgeRule51Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule37(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule37", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule37 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule37", "discovery-sanctions", "fixtures.jsonl"), "Judge Rule 37 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule37-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 37 decisions as synthetic model responses")
	counterfactualModel := fs.Bool("counterfactual-model", false, "Use a model instead of the production deterministic action")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule37(adceval.JudgeRule37RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule37(ctx, adceval.JudgeRule37Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		CounterfactualModel:   *counterfactualModel,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule11(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule11", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule11 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule11", "sanctions", "fixtures.jsonl"), "Judge Rule 11 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule11-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 11 decisions as synthetic model responses")
	counterfactualModel := fs.Bool("counterfactual-model", false, "Use a model instead of the production deterministic action")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule11(adceval.JudgeRule11RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule11(ctx, adceval.JudgeRule11Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		CounterfactualModel:   *counterfactualModel,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule52(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule52", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule52 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule52", "bench-opinion", "fixtures.jsonl"), "Judge Rule 52 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule52-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 52 bench opinions as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule52(adceval.JudgeRule52RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule52(ctx, adceval.JudgeRule52Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule58(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule58", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule58 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule58", "judgment-entry", "fixtures.jsonl"), "Judge Rule 58 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule58-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 58 judgment entries as synthetic model responses")
	counterfactualModel := fs.Bool("counterfactual-model", false, "Use a model instead of the production deterministic action")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule58(ctx, adceval.JudgeRule58Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		CounterfactualModel:   *counterfactualModel,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func RunEvalJudgeRule60(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("eval judge-rule60", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc eval judge-rule60 [options]\n\n")
		fs.PrintDefaults()
	})
	fixtures := fs.String("fixtures", defaultJudgeEvalPath("rules", "rule60", "relief-from-judgment", "fixtures.jsonl"), "Judge Rule 60 fixture JSONL file")
	outDir := fs.String("out-dir", defaultJudgeEvalOutPath("rule60-latest"), "Directory for eval results and summary")
	opportunityPromptFile := fs.String("opportunity-prompt-file", "", "Eval-local opportunity prompt template file")
	opportunityPromptName := fs.String("opportunity-prompt-name", "", "Name to record for the eval-local opportunity prompt")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles deferredPromptFileFlag
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	model := fs.String("model", "openrouter://openai/gpt-5", "Judge model in endpoint://model form")
	rescoreResults := fs.String("rescore-results", "", "Existing results JSONL to rescore without model calls")
	dryRun := fs.Bool("dry-run", false, "Use expected Rule 60 rulings as synthetic model responses")
	online := fs.Bool("online", false, "Enable online model tool conversion behavior")
	limit := fs.Int("limit", 0, "Maximum number of fixtures to run; 0 means all")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM and fixture timeout in seconds")
	temperature := fs.String("temperature", "", "Override judge model temperature")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc %s accepts no positional arguments", fs.Name())
	}
	if strings.TrimSpace(*rescoreResults) != "" {
		summary, err := adceval.RescoreJudgeRule60(adceval.JudgeRule60RescoreOptions{
			ResultsPath: strings.TrimSpace(*rescoreResults),
			OutputDir:   strings.TrimSpace(*outDir),
		})
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal eval summary: %w", err)
		}
		_, err = fmt.Fprintln(stdout, string(raw))
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return fmt.Errorf("resolve --court: %w", err)
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolveDeferredPromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}
	tempPtr, err := parseOptionalFloat(*temperature)
	if err != nil {
		return fmt.Errorf("parse --temperature: %w", err)
	}
	summary, err := adceval.RunJudgeRule60(ctx, adceval.JudgeRule60Options{
		Court:                 court,
		PromptDir:             resolvedPromptDir,
		PromptFiles:           resolvedPromptFiles,
		FixturesPath:          strings.TrimSpace(*fixtures),
		OutputDir:             strings.TrimSpace(*outDir),
		OpportunityPromptPath: strings.TrimSpace(*opportunityPromptFile),
		OpportunityPromptName: strings.TrimSpace(*opportunityPromptName),
		Engine:                lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
		Model:                 strings.TrimSpace(*model),
		Online:                *online,
		DryRun:                *dryRun,
		Limit:                 *limit,
		Timeout:               time.Duration(*timeoutSeconds) * time.Second,
		Temperature:           tempPtr,
	})
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal eval summary: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(raw))
	return err
}

func printEvalUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `Usage: adc eval <eval> [options]

Evals:
  judge-for-cause  Evaluate judge rulings on for-cause juror challenges
  judge-voir-dire  Evaluate judge rulings on proposed voir dire questions
  judge-rule11     Evaluate judge dispositions of Rule 11 motions
  judge-rule12     Evaluate judge dispositions of Rule 12 motions
  judge-rule37     Evaluate judge dispositions of Rule 37 motions
  judge-rule51     Evaluate judge settlement of jury instructions
  judge-rule52     Evaluate judge Rule 52 bench opinions
  judge-rule58     Evaluate judge Rule 58 judgment entry
  judge-rule60     Evaluate judge Rule 60 relief from judgment
  judge-rule56     Evaluate judge dispositions of Rule 56 motions

Use 'adc eval help <eval>' for eval flags.
`)
	return err
}
