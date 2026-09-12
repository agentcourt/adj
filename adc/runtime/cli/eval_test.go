package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultJudgeEvalOutPathFromADCDirectory(t *testing.T) {
	root := t.TempDir()
	writePathTestFile(t, filepath.Join(root, "go.mod"))
	if err := os.MkdirAll(filepath.Join(root, "evals", "out"), 0o755); err != nil {
		t.Fatalf("MkdirAll eval output root: %v", err)
	}
	adcDir := filepath.Join(root, "adc")
	if err := os.MkdirAll(adcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll adc directory: %v", err)
	}

	got := defaultJudgeEvalOutPathFrom(adcDir, "rule56-latest")
	want := filepath.Join(root, "evals", "out", "adc", "judge", "rule56-latest")
	if got != want {
		t.Fatalf("defaultJudgeEvalOutPathFrom = %q, want %q", got, want)
	}
}

func TestRunEvalHelp(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := RunEval(context.Background(), []string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("RunEval: %v", err)
	}
	for _, subcommand := range []string{"judge-for-cause", "judge-voir-dire", "judge-rule11", "judge-rule12", "judge-rule37", "judge-rule51", "judge-rule52", "judge-rule56", "judge-rule58", "judge-rule60"} {
		if !strings.Contains(stdout.String(), subcommand) {
			t.Errorf("help omits %q", subcommand)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunEvalSuiteHelpIncludesSharedOptions(t *testing.T) {
	t.Parallel()

	for _, subcommand := range []string{"judge-for-cause", "judge-voir-dire", "judge-rule11", "judge-rule12", "judge-rule37", "judge-rule51", "judge-rule52", "judge-rule56", "judge-rule58", "judge-rule60"} {
		t.Run(subcommand, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := RunEval(context.Background(), []string{subcommand, "--help"}, &stdout, &stderr); err != nil {
				t.Fatalf("RunEval: %v", err)
			}
			for _, option := range []string{"-court", "-prompt-dir", "-prompt-file"} {
				if !strings.Contains(stderr.String(), option) {
					t.Errorf("help omits %q:\n%s", option, stderr.String())
				}
			}
		})
	}
}

func TestRunEvalCounterfactualModelOption(t *testing.T) {
	counterfactualSuites := map[string]bool{
		"judge-rule58": true,
	}
	for _, subcommand := range []string{"judge-for-cause", "judge-voir-dire", "judge-rule11", "judge-rule12", "judge-rule37", "judge-rule51", "judge-rule52", "judge-rule56", "judge-rule58", "judge-rule60"} {
		t.Run(subcommand, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := RunEval(context.Background(), []string{subcommand, "--help"}, &stdout, &stderr); err != nil {
				t.Fatalf("RunEval: %v", err)
			}
			hasOption := strings.Contains(stderr.String(), "-counterfactual-model")
			if hasOption != counterfactualSuites[subcommand] {
				t.Fatalf("counterfactual option present = %t\n%s", hasOption, stderr.String())
			}
		})
	}
}

func TestRunEvalSuiteRejectsUnknownPromptID(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunEval(context.Background(), []string{"judge-rule56", "--prompt-file", "unknown=prompt.md"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), `unknown ADC prompt id "unknown"`) {
		t.Fatalf("RunEval error = %v", err)
	}
}

func TestRunEvalRescoreSkipsRuntimeConfiguration(t *testing.T) {
	for _, subcommand := range []string{
		"judge-for-cause",
		"judge-voir-dire",
		"judge-rule11",
		"judge-rule12",
		"judge-rule37",
		"judge-rule51",
		"judge-rule52",
		"judge-rule60",
	} {
		t.Run(subcommand, func(t *testing.T) {
			missingResults := filepath.Join(t.TempDir(), "missing.jsonl")
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := RunEval(context.Background(), []string{
				subcommand,
				"--rescore-results", missingResults,
				"--out-dir", filepath.Join(t.TempDir(), "out"),
				"--court", "missing-court",
				"--prompt-file", "unknown=prompt.md",
				"--temperature", "invalid",
			}, &stdout, &stderr)
			if err == nil {
				t.Fatal("RunEval returned nil error")
			}
			if !strings.Contains(err.Error(), missingResults) {
				t.Fatalf("RunEval error = %v", err)
			}
			for _, unrelated := range []string{"resolve --court", "unknown ADC prompt id", "parse --temperature"} {
				if strings.Contains(err.Error(), unrelated) {
					t.Errorf("RunEval error contains unrelated runtime failure %q: %v", unrelated, err)
				}
			}
		})
	}
}
