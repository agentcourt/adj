//go:build linux

package runstate

import (
	"os"
	"testing"
)

func TestProcessInstanceIDIsStableForCurrentProcess(t *testing.T) {
	first, err := ProcessInstanceID(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	second, err := ProcessInstanceID(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || first != second {
		t.Fatalf("process instance changed from %q to %q", first, second)
	}
}
