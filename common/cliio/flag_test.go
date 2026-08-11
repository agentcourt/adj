package cliio

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestParseReturnsHelpOutputFailure(t *testing.T) {
	output := NewErrorWriter(errorWriter{})
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(output)
	help, err := Parse(fs, []string{"-h"}, output)
	if !help || err == nil || !strings.Contains(err.Error(), "write command output") {
		t.Fatalf("help = %t, error = %v", help, err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
