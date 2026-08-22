package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jsmorph/adj/common/mcpcli"
	quickmcp "github.com/jsmorph/adj/quick/mcp"
)

func TestCommandArguments(t *testing.T) {
	previous := runServer
	t.Cleanup(func() { runServer = previous })
	var received quickmcp.Options
	temporaryDirectory := t.TempDir()
	keyPath := filepath.Join(temporaryDirectory, "mcp.key")
	keyFile := "adjmcpkey1.AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8\n"
	if err := os.WriteFile(keyPath, []byte(keyFile), 0o600); err != nil {
		t.Fatal(err)
	}
	bearerTokenPath := filepath.Join(temporaryDirectory, "caseapi.token")
	if err := os.WriteFile(bearerTokenPath, []byte("api-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	readyPath := filepath.Join(temporaryDirectory, "ready.json")
	runServer = func(_ context.Context, opts quickmcp.Options) error {
		received = opts
		if err := opts.ListenerReady("127.0.0.1:24567"); err != nil {
			return err
		}
		raw, err := os.ReadFile(readyPath)
		if err != nil {
			return err
		}
		var record mcpcli.ReadyRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		if record.Address != "127.0.0.1:24567" {
			return fmt.Errorf("ready address = %q", record.Address)
		}
		return nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{
		"serve",
		"--listen", "127.0.0.1:24567",
		"--caseapi-base", "http://127.0.0.1:24568",
		"--signing-key-file", keyPath,
		"--caseapi-bearer-token-file", bearerTokenPath,
		"--session-ttl", "0",
		"--session-cleanup-interval", "2m",
		"--allow-origin", "https://one.example",
		"--allow-origin", "https://two.example",
		"--prompt-dir", "prompt-set",
		"--prompt-file", "mcp.tool.get_case=get-case.md",
		"--ready-file", readyPath,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if received.ListenAddr != "127.0.0.1:24567" || received.CaseAPIBase != "http://127.0.0.1:24568" {
		t.Fatalf("addresses = %#v", received)
	}
	if !reflect.DeepEqual(received.SigningKey, goldenTestKey()) || received.CaseAPIBearerToken != "api-token" {
		t.Fatalf("tokens = %#v", received)
	}
	if received.SessionTTL != 0 || !received.DisableSessionExpiry || received.SessionCleanupInterval != 2*time.Minute {
		t.Fatalf("session settings = %#v", received)
	}
	if !reflect.DeepEqual(received.AllowedOrigins, []string{"https://one.example", "https://two.example"}) {
		t.Fatalf("allowed origins = %#v", received.AllowedOrigins)
	}
	if received.PromptDir != "prompt-set" || !reflect.DeepEqual(received.PromptFiles, map[string]string{"mcp.tool.get_case": "get-case.md"}) {
		t.Fatalf("prompt mapping = %#v", received)
	}
	if received.Log != &stderr {
		t.Fatalf("log mapping = %#v", received)
	}
	if _, err := os.Stat(readyPath); !os.IsNotExist(err) {
		t.Fatalf("ready file remains after shutdown: %v", err)
	}
	stdout.Reset()
	err = run(context.Background(), []string{"issue", "--signing-key-file", keyPath, "--case-id", "case-alpha", "--role-id", "plaintiff"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	wantToken := "adjmcp1.eyJ2ZXJzaW9uIjoiYWRqLm1jcC5jYXBhYmlsaXR5LnYxIiwiYXVkaWVuY2UiOiJxdWljayIsImNhc2VfaWQiOiJjYXNlLWFscGhhIiwiYXNzaWdubWVudF90eXBlIjoibGF3eWVyIiwicHJpbmNpcGFsX2lkIjoicGxhaW50aWZmIn0.9W98_OD1jQN6xF0fcfUKmTWOgr2QFLYa2tQyxkalUCg\n"
	if stdout.String() != wantToken {
		t.Fatalf("issued token = %q", stdout.String())
	}
	generatedKeyPath := filepath.Join(temporaryDirectory, "generated.key")
	if err := run(context.Background(), []string{"keygen", "--signing-key-file", generatedKeyPath}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(generatedKeyPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("generated key info = %#v, error = %v", info, err)
	}
	failedReadyPath := filepath.Join(temporaryDirectory, "failed-ready.json")
	serverError := errors.New("server failed")
	runServer = func(_ context.Context, opts quickmcp.Options) error {
		if err := opts.ListenerReady("127.0.0.1:24569"); err != nil {
			return err
		}
		return serverError
	}
	err = run(context.Background(), []string{
		"serve",
		"--caseapi-base", "http://127.0.0.1:24568",
		"--signing-key-file", keyPath,
		"--caseapi-bearer-token-file", bearerTokenPath,
		"--ready-file", failedReadyPath,
	}, &stdout, &stderr)
	if !errors.Is(err, serverError) {
		t.Fatalf("server error = %v", err)
	}
	if _, err := os.Stat(failedReadyPath); !os.IsNotExist(err) {
		t.Fatalf("ready file remains after server error: %v", err)
	}
	if err := run(context.Background(), []string{"extra"}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "unknown quick-mcp mode") {
		t.Fatalf("mode error = %v", err)
	}
}

func goldenTestKey() []byte {
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index)
	}
	return key
}
