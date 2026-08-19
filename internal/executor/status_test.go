package executor_test

import (
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func TestStatusBlocksOnTimeout(t *testing.T) {
	got := executor.Status(executor.Outcome{TimedOut: true})
	if got != lane.Blocked {
		t.Errorf("Status(TimedOut) = %v, want %v", got, lane.Blocked)
	}
}

// TestStatusExitZeroIsRunningNotDone is the important branch: exit 0 only
// means the process ran. Whether the requested work succeeded is decided
// by the result envelope, read by a different package (internal/result).
// Status must never promote exit 0 straight to lane.Done.
func TestStatusExitZeroIsRunningNotDone(t *testing.T) {
	got := executor.Status(executor.Outcome{ExitCode: 0, TimedOut: false})
	if got != lane.Running {
		t.Errorf("Status(exit 0) = %v, want %v", got, lane.Running)
	}
}

func TestStatusTable(t *testing.T) {
	tests := []struct {
		name string
		o    executor.Outcome
		want lane.Status
	}{
		{
			name: "timed out, silent executor",
			o:    executor.Outcome{TimedOut: true, ExitCode: 0},
			want: lane.Blocked,
		},
		{
			name: "timed out even with a nonzero exit code",
			o:    executor.Outcome{TimedOut: true, ExitCode: 1},
			want: lane.Blocked,
		},
		{
			name: "non-zero exit, crashed",
			o:    executor.Outcome{ExitCode: 1, TimedOut: false},
			want: lane.Blocked,
		},
		{
			name: "exit 0 is running, never done",
			o:    executor.Outcome{ExitCode: 0, TimedOut: false},
			want: lane.Running,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.Status(tt.o)
			if got != tt.want {
				t.Errorf("Status(%+v) = %v, want %v", tt.o, got, tt.want)
			}
		})
	}
}
