package executor

import (
	"context"
	"testing"
	"time"
)

// TestPrintTimeoutForExceedsContextDeadline asserts the core safety
// property: the value handed to agy's own --print-timeout must be strictly
// greater than the context deadline, so the Go side always decides and
// agy's own timeout can never fire first.
func TestPrintTimeoutForExceedsContextDeadline(t *testing.T) {
	const ctxBudget = 50 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), ctxBudget)
	defer cancel()

	got, ok := printTimeoutFor(ctx)
	if !ok {
		t.Fatalf("printTimeoutFor() ok = false, want true for a context with a deadline")
	}
	if got <= ctxBudget {
		t.Errorf("printTimeoutFor() = %v, want strictly greater than context budget %v", got, ctxBudget)
	}
}

func TestPrintTimeoutForNoDeadline(t *testing.T) {
	_, ok := printTimeoutFor(context.Background())
	if ok {
		t.Errorf("printTimeoutFor() ok = true, want false for a context with no deadline")
	}
}
