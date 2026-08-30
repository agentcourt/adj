package corehealth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProbeRequiresMatchingIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(w, `{"ok":true,"case_id":"case-1","run_id":"run-1"}`); err != nil {
			t.Errorf("write health response: %v", err)
		}
	}))
	defer server.Close()
	if err := Probe(context.Background(), server.Client(), server.URL, "case-1", "run-1"); err != nil {
		t.Fatalf("probe matching identity: %v", err)
	}
	if err := Probe(context.Background(), server.Client(), server.URL, "case-1", "other-run"); err == nil || !strings.Contains(err.Error(), "other-run") {
		t.Fatalf("mismatched probe error = %v", err)
	}
}

func TestWaitReturnsLastProbeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "starting failed", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	err := Wait(context.Background(), server.Client(), server.URL, "case-1", "run-1", 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "starting failed") || !strings.Contains(err.Error(), "did not become healthy") {
		t.Fatalf("wait error = %v", err)
	}
}
