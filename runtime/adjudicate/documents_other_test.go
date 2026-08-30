//go:build !linux

package adjudicate

import "errors"

func makeNamedPipe(string) error {
	return errors.New("named pipes unavailable")
}
