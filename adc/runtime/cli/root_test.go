package cli

import (
	"bytes"
	"context"
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
