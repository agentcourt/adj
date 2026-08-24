package lean

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCallContextTerminatesLeanProcessAtDeadline(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := New([]string{"sh", "-c", "exec sleep 5"}).CallContext(ctx, map[string]any{"request_type": "test"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("CallContext error = %v, want context deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("CallContext returned after %s, want less than one second", elapsed)
	}
}
