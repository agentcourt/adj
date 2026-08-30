package mcpchild

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentcourt/adj/internal/mcpcap"
	"github.com/agentcourt/adj/runtime/runstate"
)

type processObserver struct {
	mu     sync.Mutex
	start  runstate.ProcessStart
	finish runstate.ProcessFinish
}

func (o *processObserver) StartProcess(start runstate.ProcessStart) (runstate.FinishProcess, error) {
	o.mu.Lock()
	o.start = start
	o.mu.Unlock()
	return func(finish runstate.ProcessFinish) error {
		o.mu.Lock()
		o.finish = finish
		o.mu.Unlock()
		return nil
	}, nil
}

type helperObservation struct {
	PID       int      `json:"pid"`
	KeyPath   string   `json:"key_path"`
	KeyMode   uint32   `json:"key_mode"`
	TokenPath string   `json:"token_path"`
	TokenMode uint32   `json:"token_mode"`
	Args      []string `json:"args"`
}

func TestManagedMCPChildUsesPrivateSigningKeyAndPublishedAddress(t *testing.T) {
	logsDir := t.TempDir()
	observationPath := filepath.Join(t.TempDir(), "observation.json")
	observer := &processObserver{}
	ctx, cancel := context.WithCancel(context.Background())
	process, err := Start(ctx, Request{
		Command:     os.Args[0],
		CommandArgs: []string{"-test.run=TestMCPChildHelperProcess", "--"},
		Environment: append(os.Environ(),
			"ADJ_MCP_CHILD_HELPER=1",
			"ADJ_MCP_CHILD_OBSERVATION="+observationPath,
		),
		ListenAddr:         "0.0.0.0:0",
		CaseAPIBase:        "http://127.0.0.1:32124",
		SigningKey:         []byte("0123456789abcdef0123456789abcdef"),
		CaseAPIBearerToken: []byte("case-api-secret"),
		PromptDir:          "/prompts/arb",
		PromptFiles: map[string]string{
			"mcp.tool.wait":    "/prompts/wait.md",
			"mcp.session.help": "/prompts/session.md",
		},
		LogsDir:        logsDir,
		LogPrefix:      "mcp",
		Name:           "test-mcp",
		Observer:       observer,
		StartupTimeout: 5 * time.Second,
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if process.Address() != "127.0.0.1:32123" {
		cancel()
		t.Fatalf("published address = %q", process.Address())
	}
	raw, err := os.ReadFile(observationPath)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	var observation helperObservation
	if err := json.Unmarshal(raw, &observation); err != nil {
		cancel()
		t.Fatal(err)
	}
	if observation.PID == os.Getpid() || observation.PID <= 0 {
		cancel()
		t.Fatalf("child PID = %d, parent PID = %d", observation.PID, os.Getpid())
	}
	if observation.KeyMode != 0o600 {
		cancel()
		t.Fatalf("signing-key mode = %#o", observation.KeyMode)
	}
	if observation.TokenMode != 0o600 {
		cancel()
		t.Fatalf("case API bearer-token mode = %#o", observation.TokenMode)
	}
	if strings.Contains(strings.Join(observation.Args, "\x00"), "adjmcpkey1.") || strings.Contains(strings.Join(observation.Args, "\x00"), "case-api-secret") {
		cancel()
		t.Fatal("credential appeared in child argv")
	}
	if _, err := os.Stat(observation.KeyPath); !os.IsNotExist(err) {
		cancel()
		t.Fatalf("signing-key file remains after readiness: %v", err)
	}
	if _, err := os.Stat(observation.TokenPath); !os.IsNotExist(err) {
		cancel()
		t.Fatalf("case API bearer-token file remains after readiness: %v", err)
	}
	wantPromptArgs := []string{
		"--prompt-file", "mcp.session.help=/prompts/session.md",
		"--prompt-file", "mcp.tool.wait=/prompts/wait.md",
	}
	if !reflect.DeepEqual(observation.Args[len(observation.Args)-len(wantPromptArgs):], wantPromptArgs) {
		cancel()
		t.Fatalf("prompt arguments = %#v", observation.Args)
	}
	observer.mu.Lock()
	start := observer.start
	observer.mu.Unlock()
	if start.PID != observation.PID || start.Name != "test-mcp" || start.Kind != "mcp" {
		cancel()
		t.Fatalf("observed process start = %#v", start)
	}

	cancel()
	select {
	case err := <-process.Done():
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for MCP child to exit")
	}
	if _, err := os.Stat(filepath.Dir(observation.KeyPath)); !os.IsNotExist(err) {
		t.Fatalf("private MCP directory remains after exit: %v", err)
	}
	observer.mu.Lock()
	finish := observer.finish
	observer.mu.Unlock()
	if finish.State != runstate.Canceled {
		t.Fatalf("observed process finish = %#v", finish)
	}
}

func TestSplitPromptFilesPreservesMCPIDs(t *testing.T) {
	core, mcp, err := SplitPromptFiles(map[string]string{
		"attorney.arguments":       "/prompts/arguments.md",
		"mcp.session.instructions": "/prompts/session.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(core, map[string]string{"attorney.arguments": "/prompts/arguments.md"}) {
		t.Fatalf("core prompt files = %#v", core)
	}
	if !reflect.DeepEqual(mcp, map[string]string{"mcp.session.instructions": "/prompts/session.md"}) {
		t.Fatalf("MCP prompt files = %#v", mcp)
	}
	if _, _, err := SplitPromptFiles(map[string]string{"mcp.": "/prompts/empty.md"}); err == nil {
		t.Fatal("empty MCP prompt ID was accepted")
	}
}

func TestMCPChildHelperProcess(t *testing.T) {
	if os.Getenv("ADJ_MCP_CHILD_HELPER") != "1" {
		return
	}
	args := argumentsAfterSeparator(os.Args)
	values, err := parseHelperArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := values["--signing-key-file"]
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mcpcap.ParseKeyFile(raw); err != nil {
		t.Fatal(err)
	}
	for _, arg := range args {
		if arg == strings.TrimSpace(string(raw)) {
			t.Fatal("signing key in argv")
		}
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	tokenPath := values["--caseapi-bearer-token-file"]
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(token) != "case-api-secret" {
		t.Fatal("unexpected case API bearer token")
	}
	tokenInfo, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	observation := helperObservation{
		PID:       os.Getpid(),
		KeyPath:   keyPath,
		KeyMode:   uint32(info.Mode().Perm()),
		TokenPath: tokenPath,
		TokenMode: uint32(tokenInfo.Mode().Perm()),
		Args:      args,
	}
	encoded, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("ADJ_MCP_CHILD_OBSERVATION"), encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	readyPath := values["--ready-file"]
	temporaryPath := readyPath + ".temporary"
	if err := os.WriteFile(temporaryPath, []byte("{\"address\":\"127.0.0.1:32123\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(temporaryPath, readyPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		t.Fatal(err)
	}
	select {}
}

func argumentsAfterSeparator(args []string) []string {
	for index, arg := range args {
		if arg == "--" {
			return append([]string(nil), args[index+1:]...)
		}
	}
	return nil
}

func parseHelperArgs(args []string) (map[string]string, error) {
	values := make(map[string]string)
	for index := 0; index < len(args); index += 2 {
		if index+1 >= len(args) {
			return nil, fmt.Errorf("argument %q has no value", args[index])
		}
		values[args[index]] = args[index+1]
	}
	for _, name := range []string{"--listen", "--caseapi-base", "--signing-key-file", "--caseapi-bearer-token-file", "--ready-file"} {
		if strings.TrimSpace(values[name]) == "" {
			return nil, fmt.Errorf("missing %s", name)
		}
	}
	return values, nil
}
