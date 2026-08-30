//go:build linux

package adjudicate

import "syscall"

func makeNamedPipe(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
