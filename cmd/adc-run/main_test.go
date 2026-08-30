package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCommandHelpAndArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := runLocal(context.Background(), []string{"-h"}, &stdout, &stderr); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage: adc-run") {
		t.Fatalf("help output = %q", stderr.String())
	}
	for _, flagName := range []string{"-launcher-prompt-dir", "-launcher-prompt-file"} {
		if !strings.Contains(stderr.String(), flagName) {
			t.Fatalf("help output lacks %s: %q", flagName, stderr.String())
		}
	}
	if err := runLocal(context.Background(), []string{"extra"}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "no positional arguments") {
		t.Fatalf("argument error = %v", err)
	}
}
