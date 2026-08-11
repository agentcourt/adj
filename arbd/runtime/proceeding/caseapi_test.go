package proceeding

import (
	"context"
	"strings"
	"testing"
)

func TestCaseAPIServerReportsServeFailure(t *testing.T) {
	api, err := startCaseAPIServer(&runContext{}, false)
	if err != nil {
		t.Fatalf("startCaseAPIServer: %v", err)
	}
	if err := api.ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	err = api.Close(context.Background())
	if err == nil || !strings.Contains(err.Error(), "case API server failed") {
		t.Fatalf("Close error = %v", err)
	}
}
