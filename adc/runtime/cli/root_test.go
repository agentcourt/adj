package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunHelpWritesUsageToStdout(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run(context.Background(), []string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage: adc <subcommand> [options]") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestSubcommandsRejectPositionalArguments(t *testing.T) {
	for _, subcommand := range []string{"case", "case-packet", "complain", "scenario", "pacer", "validate", "verify-certificate"} {
		t.Run(subcommand, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := Run(context.Background(), []string{subcommand, "extra"}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), "no positional arguments") {
				t.Fatalf("Run error = %v", err)
			}
		})
	}
}

func TestRootHelpReturnsOutputFailure(t *testing.T) {
	err := Run(context.Background(), []string{"--help"}, errorWriter{}, errorWriter{})
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("Run error = %v", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestRunRequiresSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), nil, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "subcommand is required") {
		t.Fatalf("Run error = %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage: adc <subcommand> [options]") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
