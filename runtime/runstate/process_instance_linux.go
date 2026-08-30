//go:build linux

package runstate

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ProcessInstanceID(pid int) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("process PID must be positive")
	}
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", fmt.Errorf("read process %d start identity: %w", pid, err)
	}
	close := strings.LastIndexByte(string(raw), ')')
	if close < 0 {
		return "", fmt.Errorf("process %d stat has no command terminator", pid)
	}
	fields := strings.Fields(string(raw[close+1:]))
	if len(fields) <= 19 {
		return "", fmt.Errorf("process %d stat has %d fields after command, need at least 20", pid, len(fields))
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", fmt.Errorf("parse process %d start identity: %w", pid, err)
	}
	bootRaw, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("read system boot identity: %w", err)
	}
	bootID := strings.TrimSpace(string(bootRaw))
	if bootID == "" || len(strings.Fields(bootID)) != 1 {
		return "", fmt.Errorf("system boot identity is invalid")
	}
	return bootID + ":" + fields[19], nil
}
