package adjudicate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/runtime/runstate"
)

const coreErrorTailBytes = 64 * 1024

type coreProcessRequest struct {
	Command    string
	Args       []string
	Dir        string
	Env        []string
	LogsDir    string
	LogPrefix  string
	ResultPath string
	Observer   runstate.Observer
}

type coreProcessOutcome struct {
	result []byte
	err    error
}

type coreProcessWaiter interface {
	Done() <-chan coreProcessOutcome
}

type coreProcess struct {
	done <-chan coreProcessOutcome
}

func (p *coreProcess) Done() <-chan coreProcessOutcome {
	return p.done
}

func runCoreProcess(ctx context.Context, request coreProcessRequest) ([]byte, error) {
	process, err := startCoreProcess(ctx, request)
	if err != nil {
		return nil, err
	}
	outcome := <-process.Done()
	return outcome.result, outcome.err
}

func startCoreProcess(ctx context.Context, request coreProcessRequest) (coreProcessWaiter, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	request.Command = strings.TrimSpace(request.Command)
	if request.Command == "" {
		return nil, fmt.Errorf("core command is empty")
	}
	if strings.TrimSpace(request.LogsDir) == "" || strings.TrimSpace(request.LogPrefix) == "" {
		return nil, fmt.Errorf("core log directory and prefix are required")
	}
	if strings.TrimSpace(request.ResultPath) == "" {
		return nil, fmt.Errorf("core result path is required")
	}
	stdoutPath := filepath.Join(request.LogsDir, request.LogPrefix+".stdout")
	stderrPath := filepath.Join(request.LogsDir, request.LogPrefix+".stderr")
	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("create core stdout log %q: %w", stdoutPath, err)
	}
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create core stderr log %q: %w", stderrPath, err), closeNamed(stdout, stdoutPath))
	}

	cmd := exec.CommandContext(ctx, request.Command, request.Args...)
	cmd.Dir = strings.TrimSpace(request.Dir)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if request.Env != nil {
		cmd.Env = append([]string(nil), request.Env...)
	}
	startErr := cmd.Start()
	if startErr != nil {
		return nil, errors.Join(
			fmt.Errorf("start core command %q: %w", request.Command, startErr),
			closeNamed(stdout, stdoutPath),
			closeNamed(stderr, stderrPath),
		)
	}
	started := time.Now().UTC()
	finishProcess, observeErr := runstate.Start(request.Observer, runstate.ProcessStart{
		Name:      request.LogPrefix + "-core",
		Kind:      "core",
		PID:       cmd.Process.Pid,
		StartedAt: started,
	})
	if observeErr != nil {
		return nil, errors.Join(
			fmt.Errorf("record core process start: %w", observeErr),
			cmd.Process.Kill(),
			cmd.Wait(),
			closeNamed(stdout, stdoutPath),
			closeNamed(stderr, stderrPath),
		)
	}
	done := make(chan coreProcessOutcome, 1)
	go func() {
		waitErr := cmd.Wait()
		closeErr := errors.Join(closeNamed(stdout, stdoutPath), closeNamed(stderr, stderrPath))
		result, readErr := os.ReadFile(request.ResultPath)
		if readErr != nil {
			readErr = fmt.Errorf("read core result %q: %w", request.ResultPath, readErr)
		}
		if waitErr != nil {
			waitErr = coreProcessError(ctx, waitErr, stderrPath)
		}
		processErr := errors.Join(waitErr, closeErr, readErr)
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
		done <- coreProcessOutcome{result: result, err: errors.Join(processErr, finishErr)}
	}()
	return &coreProcess{done: done}, nil
}

func coreProcessError(ctx context.Context, waitErr error, stderrPath string) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(ctxErr, fmt.Errorf("core process: %w", waitErr))
	}
	tail, readErr := readTail(stderrPath, coreErrorTailBytes)
	message := strings.TrimSpace(string(tail))
	if message == "" {
		return errors.Join(fmt.Errorf("core process: %w", waitErr), readErr)
	}
	return errors.Join(fmt.Errorf("core process: %w: %s", waitErr, message), readErr)
}

func readTail(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open log tail %q: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("stat log tail %q: %w", path, err), closeNamed(file, path))
	}
	if info.Size() > limit {
		if _, err := file.Seek(info.Size()-limit, io.SeekStart); err != nil {
			return nil, errors.Join(fmt.Errorf("seek log tail %q: %w", path, err), closeNamed(file, path))
		}
	}
	var buffer bytes.Buffer
	_, copyErr := io.Copy(&buffer, file)
	closeErr := closeNamed(file, path)
	if copyErr != nil {
		return nil, errors.Join(fmt.Errorf("read log tail %q: %w", path, copyErr), closeErr)
	}
	return buffer.Bytes(), closeErr
}

func closeNamed(file *os.File, path string) error {
	if file == nil {
		return nil
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %q: %w", path, err)
	}
	return nil
}
