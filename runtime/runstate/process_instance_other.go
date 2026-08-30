//go:build !linux

package runstate

func ProcessInstanceID(int) (string, error) {
	return "", nil
}
