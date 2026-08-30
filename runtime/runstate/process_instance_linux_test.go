//go:build linux

package runstate

import (
	"os"
	"strings"
	"testing"
)

func TestLinuxProcessInstanceIDIncludesBootID(t *testing.T) {
	identity, err := ProcessInstanceID(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	bootID, startTicks, ok := strings.Cut(identity, ":")
	if !ok || bootID == "" || startTicks == "" {
		t.Fatalf("process identity = %q", identity)
	}
}
