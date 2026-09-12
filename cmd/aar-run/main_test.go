package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	headless "github.com/agentcourt/adj/runtime/agent"
)

func TestCommandHelpAndArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := runLocal(context.Background(), []string{"-h"}, &stdout, &stderr); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(stderr.String(), "Usage: aar-run") {
		t.Fatalf("help output = %q", stderr.String())
	}
	for _, flagName := range []string{"-plaintiff-lawyer", "-defendant-lawyer", "-launcher-prompt-dir", "-launcher-prompt-file", "-plaintiff-lawyer-api-key-env", "-defendant-lawyer-resume", "-required-votes", "-council-endpoint", "-minimum-distinct-council-endpoints"} {
		if !strings.Contains(stderr.String(), flagName) {
			t.Fatalf("help output lacks %s: %q", flagName, stderr.String())
		}
	}
	for _, removed := range []string{"-lawyer-instructions", "-headless-lawyer-instructions", "-pi-lawyer-instructions", "-remote-lawyer-skill", "-council-instructions"} {
		if strings.Contains(stderr.String(), removed) {
			t.Fatalf("help output retains %s: %q", removed, stderr.String())
		}
	}
	if err := runLocal(context.Background(), []string{"one", "two"}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "at most one example") {
		t.Fatalf("argument error = %v", err)
	}
}

func TestAgentAuthModeAcceptsCommandSpelling(t *testing.T) {
	if got := agentAuthMode("api-key"); got != headless.AuthAPIKey {
		t.Fatalf("api-key auth mode = %q", got)
	}
	if got := agentAuthMode("subscription"); got != headless.AuthSubscription {
		t.Fatalf("subscription auth mode = %q", got)
	}
}

func TestOptionalBoolFlagDistinguishesOmission(t *testing.T) {
	var flag optionalBoolFlag
	if flag.Pointer() != nil {
		t.Fatal("omitted flag returned a value")
	}
	if err := flag.Set("false"); err != nil {
		t.Fatal(err)
	}
	if value := flag.Pointer(); value == nil || *value {
		t.Fatalf("false flag value = %#v", value)
	}
	if err := flag.Set("true"); err != nil {
		t.Fatal(err)
	}
	if value := flag.Pointer(); value == nil || !*value {
		t.Fatalf("true flag value = %#v", value)
	}
}
