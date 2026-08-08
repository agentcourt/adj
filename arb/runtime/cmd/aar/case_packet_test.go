package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCasePacketWritesPacket(t *testing.T) {
	dir := t.TempDir()
	complaint := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Proposition\n\nP\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	evidence := filepath.Join(dir, "evidence.txt")
	if err := os.WriteFile(evidence, []byte("evidence\n"), 0o644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	packet := filepath.Join(dir, "case.tar.gz")
	manifest := filepath.Join(dir, "case-packet.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := runCasePacket(context.Background(), []string{
		"--complaint", complaint,
		"--file", evidence,
		"--packet", packet,
		"--manifest", manifest,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runCasePacket returned error: %v\nstderr=%s", err, stderr.String())
	}
	if _, err := os.Stat(packet); err != nil {
		t.Fatalf("stat packet: %v", err)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("stat manifest: %v", err)
	}
	var summary map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &summary); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if summary["case_file_mode"] != "explicit" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestRunCasePacketRejectsMissingArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runCasePacket(context.Background(), nil, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "--complaint, --packet, and --manifest are required") {
		t.Fatalf("runCasePacket error = %v, want missing-args error", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}
