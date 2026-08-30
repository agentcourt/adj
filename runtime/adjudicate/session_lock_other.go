//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package adjudicate

import (
	"errors"
	"fmt"
	"runtime"
)

var errSessionInUse = errors.New("agent session is in use")

type sessionLease struct{}

func acquireSessionLease(string) (*sessionLease, error) {
	return nil, fmt.Errorf("agent session leases are unsupported on %s", runtime.GOOS)
}

func (l *sessionLease) close() error { return nil }
