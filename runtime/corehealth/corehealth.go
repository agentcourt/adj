package corehealth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const maxResponseBytes = 64 << 10

type response struct {
	OK     bool   `json:"ok"`
	CaseID string `json:"case_id"`
	RunID  string `json:"run_id"`
}

func Probe(ctx context.Context, client *http.Client, healthURL, caseID, runID string) (returnErr error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("create core health request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request core health: %w", err)
	}
	defer func() {
		returnErr = errors.Join(returnErr, resp.Body.Close())
	}()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read core health response: %w", err)
	}
	if len(raw) > maxResponseBytes {
		return fmt.Errorf("core health response exceeds %d bytes", maxResponseBytes)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("core health returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var health response
	if err := json.Unmarshal(raw, &health); err != nil {
		return fmt.Errorf("decode core health response: %w", err)
	}
	if !health.OK {
		return fmt.Errorf("core health reports ok=false")
	}
	if health.CaseID != caseID || health.RunID != runID {
		return fmt.Errorf("core health identifies case %q run %q, expected case %q run %q", health.CaseID, health.RunID, caseID, runID)
	}
	return nil
}

func Wait(ctx context.Context, client *http.Client, healthURL, caseID, runID string, timeout time.Duration) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		if err := Probe(deadlineCtx, client, healthURL, caseID, runID); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-deadlineCtx.Done():
			return errors.Join(fmt.Errorf("%s did not become healthy within %s", healthURL, timeout), lastErr, deadlineCtx.Err())
		case <-ticker.C:
		}
	}
}
