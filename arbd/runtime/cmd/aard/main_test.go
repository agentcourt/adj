package main

import (
	"bytes"
	"context"
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
