package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunScenarioAcceptsServiceLauncherFlags(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunScenarioCase([]string{
		"--report-model", "report-model",
		"--allow-assertion-failures",
	}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--scenario is required") {
		t.Fatalf("error = %v, stderr = %q", err, stderr.String())
	}
}
