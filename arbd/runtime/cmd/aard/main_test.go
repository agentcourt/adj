package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHelpTopicsSucceed(t *testing.T) {
	t.Parallel()

	for _, topic := range []string{"case", "case-packet", "complain", "validate", "verify-certificate"} {
		t.Run(topic, func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := dispatch(context.Background(), []string{"help", topic}, &stdout, &stderr); err != nil {
				t.Fatalf("dispatch help %s: %v", topic, err)
			}
			if !strings.Contains(stdout.String()+stderr.String(), "Usage: aard "+topic) {
				t.Fatalf("help output missing usage:\nstdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
			}
		})
	}
}

func TestSubcommandsRejectPositionalArguments(t *testing.T) {
	for _, subcommand := range []string{"case", "case-packet", "complain", "validate", "verify-certificate"} {
		t.Run(subcommand, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := dispatch(context.Background(), []string{subcommand, "extra"}, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), "no positional arguments") {
				t.Fatalf("dispatch error = %v", err)
			}
		})
	}
}

func TestRootHelpReturnsOutputFailure(t *testing.T) {
	err := dispatch(context.Background(), []string{"--help"}, errorWriter{}, errorWriter{})
	if err == nil || !strings.Contains(err.Error(), "write failed") {
		t.Fatalf("dispatch error = %v", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
