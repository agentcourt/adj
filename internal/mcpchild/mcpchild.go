package mcpchild

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/agentcourt/adj/internal/mcpcap"
	"github.com/agentcourt/adj/runtime/runstate"
)

type Request struct {
	Command            string
	CommandArgs        []string
	WorkingDir         string
	Environment        []string
	ListenAddr         string
	CaseAPIBase        string
	SigningKey         []byte
	CaseAPIBearerToken []byte
	PromptDir          string
	PromptFiles        map[string]string
	LogsDir            string
	LogPrefix          string
	Name               string
	Observer           runstate.Observer
	StartupTimeout     time.Duration
}

type Process struct {
	address string
	done    <-chan error
}

func (p *Process) Address() string {
	if p == nil {
		return ""
	}
	return p.address
}

func (p *Process) Done() <-chan error {
	if p == nil {
		return nil
	}
	return p.done
}

type readyRecord struct {
	Address string `json:"address"`
}

func Start(ctx context.Context, request Request) (*Process, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request.Command = strings.TrimSpace(request.Command)
	request.WorkingDir = strings.TrimSpace(request.WorkingDir)
	request.ListenAddr = strings.TrimSpace(request.ListenAddr)
	request.CaseAPIBase = strings.TrimSpace(request.CaseAPIBase)
	request.PromptDir = strings.TrimSpace(request.PromptDir)
	request.LogsDir = strings.TrimSpace(request.LogsDir)
	request.LogPrefix = strings.TrimSpace(request.LogPrefix)
	request.Name = strings.TrimSpace(request.Name)
	if request.Command == "" {
		return nil, fmt.Errorf("MCP command is empty")
	}
	if request.ListenAddr == "" || request.CaseAPIBase == "" {
		return nil, fmt.Errorf("MCP listen address and case API base are required")
	}
	keyFile, err := mcpcap.EncodeKeyFile(request.SigningKey)
	if err != nil {
		return nil, err
	}
	if request.LogsDir == "" || request.LogPrefix == "" || request.Name == "" {
		return nil, fmt.Errorf("MCP log directory, log prefix, and process name are required")
	}
	if request.StartupTimeout <= 0 {
		return nil, fmt.Errorf("MCP startup timeout must be positive")
	}
	corePromptFiles, mcpPromptFiles, err := SplitPromptFiles(request.PromptFiles)
	if err != nil {
		return nil, fmt.Errorf("validate MCP child prompt files: %w", err)
	}
	if len(corePromptFiles) != 0 {
		return nil, fmt.Errorf("MCP child prompt IDs must begin with %q", "mcp.")
	}
	request.PromptFiles = mcpPromptFiles

	privateDir, err := os.MkdirTemp("", "adj-mcp-")
	if err != nil {
		return nil, fmt.Errorf("create private MCP directory: %w", err)
	}
	cleanupBeforeStart := func() error {
		if err := os.RemoveAll(privateDir); err != nil {
			return fmt.Errorf("remove private MCP directory %q: %w", privateDir, err)
		}
		return nil
	}
	keyPath := filepath.Join(privateDir, "signing-key")
	bearerTokenPath := filepath.Join(privateDir, "caseapi-bearer-token")
	readyPath := filepath.Join(privateDir, "ready.json")
	if err := writeSigningKey(keyPath, keyFile); err != nil {
		return nil, errors.Join(err, cleanupBeforeStart())
	}
	if len(request.CaseAPIBearerToken) > 0 {
		if err := writePrivateFile(bearerTokenPath, request.CaseAPIBearerToken, "case API bearer-token"); err != nil {
			return nil, errors.Join(err, cleanupBeforeStart())
		}
	}

	stdoutPath := filepath.Join(request.LogsDir, request.LogPrefix+".stdout")
	stderrPath := filepath.Join(request.LogsDir, request.LogPrefix+".stderr")
	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create MCP stdout log %q: %w", stdoutPath, err), cleanupBeforeStart())
	}
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create MCP stderr log %q: %w", stderrPath, err), closeFile(stdout, stdoutPath), cleanupBeforeStart())
	}

	args := append([]string(nil), request.CommandArgs...)
	args = append(args,
		"--listen", request.ListenAddr,
		"--caseapi-base", request.CaseAPIBase,
		"--signing-key-file", keyPath,
		"--session-ttl", "0",
		"--ready-file", readyPath,
	)
	if len(request.CaseAPIBearerToken) > 0 {
		args = append(args, "--caseapi-bearer-token-file", bearerTokenPath)
	}
	if request.PromptDir != "" {
		args = append(args, "--prompt-dir", request.PromptDir)
	}
	for _, id := range sortedPromptIDs(request.PromptFiles) {
		args = append(args, "--prompt-file", id+"="+request.PromptFiles[id])
	}

	cmd := exec.CommandContext(ctx, request.Command, args...)
	cmd.Dir = request.WorkingDir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if request.Environment != nil {
		cmd.Env = append([]string(nil), request.Environment...)
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("start MCP command %q: %w", request.Command, err),
			closeFile(stdout, stdoutPath),
			closeFile(stderr, stderrPath),
			cleanupBeforeStart(),
		)
	}
	finishProcess, observeErr := runstate.Start(request.Observer, runstate.ProcessStart{
		Name:      request.Name,
		Kind:      "mcp",
		PID:       cmd.Process.Pid,
		StartedAt: time.Now().UTC(),
	})
	if observeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("record MCP process %q start: %w", request.Name, observeErr),
			cmd.Process.Kill(),
			cmd.Wait(),
			closeFile(stdout, stdoutPath),
			closeFile(stderr, stderrPath),
			cleanupBeforeStart(),
		)
	}

	done := make(chan error, 1)
	go func() {
		waitErr := cmd.Wait()
		if waitErr != nil && ctx.Err() == nil {
			waitErr = processError(request.Name, waitErr, stderrPath)
		} else if ctx.Err() != nil {
			waitErr = nil
		}
		processErr := errors.Join(waitErr, closeFile(stdout, stdoutPath), closeFile(stderr, stderrPath))
		state := runstate.Completed
		if ctx.Err() != nil {
			state = runstate.Canceled
		} else if processErr != nil {
			state = runstate.Failed
		}
		var exitCode *int
		if cmd.ProcessState != nil {
			value := cmd.ProcessState.ExitCode()
			exitCode = &value
		}
		finishErr := runstate.Finish(finishProcess, runstate.ProcessFinish{
			State:      state,
			FinishedAt: time.Now().UTC(),
			ExitCode:   exitCode,
			Error:      runstate.ErrorText(processErr),
		})
		cleanupErr := cleanupBeforeStart()
		done <- errors.Join(processErr, finishErr, cleanupErr)
	}()

	startupCtx, cancelStartup := context.WithTimeout(ctx, request.StartupTimeout)
	defer cancelStartup()
	address, readyConsumed, err := waitReady(startupCtx, readyPath, done)
	if err != nil {
		if readyConsumed {
			return nil, err
		}
		return nil, errors.Join(err, cmd.Process.Kill(), <-done)
	}
	if err := removeFile(keyPath); err != nil {
		return nil, errors.Join(err, cmd.Process.Kill(), <-done)
	}
	if err := removeFile(bearerTokenPath); err != nil {
		return nil, errors.Join(err, cmd.Process.Kill(), <-done)
	}
	return &Process{address: address, done: done}, nil
}

func SplitPromptFiles(paths map[string]string) (map[string]string, map[string]string, error) {
	core := make(map[string]string)
	mcp := make(map[string]string)
	for id, path := range paths {
		trimmedID := strings.TrimSpace(id)
		trimmedPath := strings.TrimSpace(path)
		if trimmedID == "" || trimmedPath == "" {
			return nil, nil, fmt.Errorf("prompt ID and path must not be empty")
		}
		if trimmedID != id {
			return nil, nil, fmt.Errorf("prompt ID %q has surrounding whitespace", id)
		}
		if id == "mcp." {
			return nil, nil, fmt.Errorf("MCP prompt ID is empty")
		}
		if strings.HasPrefix(id, "mcp.") {
			mcp[id] = trimmedPath
		} else {
			core[id] = trimmedPath
		}
	}
	return core, mcp, nil
}

func SelectedListenAddress(requested, published string) (string, string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = "0.0.0.0:0"
	}
	host, _, err := net.SplitHostPort(requested)
	if err != nil {
		return "", "", fmt.Errorf("parse requested MCP listen address %q: %w", requested, err)
	}
	_, port, err := net.SplitHostPort(strings.TrimSpace(published))
	if err != nil {
		return "", "", fmt.Errorf("parse published MCP listen address %q: %w", published, err)
	}
	return net.JoinHostPort(host, port), port, nil
}

func writeSigningKey(path string, contents []byte) error {
	return writePrivateFile(path, contents, "MCP signing-key")
}

func writePrivateFile(path string, contents []byte, description string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create %s file %q: %w", description, path, err)
	}
	written, writeErr := file.Write(contents)
	if written != len(contents) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return errors.Join(fmt.Errorf("write %s file %q: %w", description, path, err), removeFile(path))
	}
	return nil
}

func waitReady(ctx context.Context, path string, done <-chan error) (string, bool, error) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		address, err := readReady(path)
		if err == nil {
			return address, false, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
		select {
		case processErr := <-done:
			if processErr == nil {
				return "", true, fmt.Errorf("MCP process exited before publishing readiness")
			}
			return "", true, fmt.Errorf("MCP process exited before publishing readiness: %w", processErr)
		case <-ctx.Done():
			return "", false, fmt.Errorf("wait for MCP ready file: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func readReady(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var record readyRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return "", fmt.Errorf("decode MCP ready file %q: %w", path, err)
	}
	address := strings.TrimSpace(record.Address)
	_, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("MCP ready file %q has invalid address %q: %w", path, address, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("MCP ready file %q has invalid port %q", path, portText)
	}
	return address, nil
}

func sortedPromptIDs(paths map[string]string) []string {
	ids := make([]string, 0, len(paths))
	for id := range paths {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func processError(name string, waitErr error, stderrPath string) error {
	tail, readErr := readTail(stderrPath, 64<<10)
	message := strings.TrimSpace(string(tail))
	if message == "" {
		return errors.Join(fmt.Errorf("MCP process %q: %w", name, waitErr), readErr)
	}
	return errors.Join(fmt.Errorf("MCP process %q: %w: %s", name, waitErr, message), readErr)
}

func readTail(path string, limit int64) (raw []byte, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, file.Close())
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	offset := info.Size() - limit
	if offset < 0 {
		offset = 0
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	raw, err = io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		if index := bytes.IndexByte(raw, '\n'); index >= 0 {
			raw = raw[index+1:]
		}
	}
	return raw, nil
}

func closeFile(file *os.File, path string) error {
	if file == nil {
		return nil
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %q: %w", path, err)
	}
	return nil
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %q: %w", path, err)
	}
	return nil
}
