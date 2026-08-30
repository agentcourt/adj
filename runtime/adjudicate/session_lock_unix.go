//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package adjudicate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

var errSessionInUse = errors.New("agent session is in use")

type sessionLease struct {
	file *os.File
}

func acquireSessionLease(sessionDir string) (*sessionLease, error) {
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return nil, fmt.Errorf("create agent session directory %q: %w", sessionDir, err)
	}
	path := filepath.Join(sessionDir, ".lease")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open agent session lease %q: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		closeErr := file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errors.Join(errSessionInUse, fmt.Errorf("agent session %q is in use", sessionDir), closeErr)
		}
		return nil, errors.Join(fmt.Errorf("lock agent session %q: %w", sessionDir, err), closeErr)
	}
	return &sessionLease{file: file}, nil
}

func (l *sessionLease) close() error {
	if l == nil || l.file == nil {
		return nil
	}
	file := l.file
	l.file = nil
	unlockErr := syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	closeErr := file.Close()
	if unlockErr != nil {
		unlockErr = fmt.Errorf("unlock agent session: %w", unlockErr)
	}
	if closeErr != nil {
		closeErr = fmt.Errorf("close agent session lease: %w", closeErr)
	}
	return errors.Join(unlockErr, closeErr)
}
