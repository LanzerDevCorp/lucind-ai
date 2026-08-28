package executor_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

func TestConcreteExecutorsExposeReadOnlyPathsToChild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	wantPaths := []string{"docs/spec.md", "internal/input/config.go"}
	want, err := json.Marshal(wantPaths)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		make func(string) executor.Executor
	}{
		{name: "agy", make: func(binary string) executor.Executor { return executor.Agy{Binary: binary} }},
		{name: "opencode", make: func(binary string) executor.Executor { return executor.Opencode{Binary: binary} }},
		{name: "claude", make: func(binary string) executor.Executor { return executor.Claude{Binary: binary} }},
		{name: "cursor-agent", make: func(binary string) executor.Executor { return executor.CursorAgent{Binary: binary} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := filepath.Join(t.TempDir(), "executor-stub.sh")
			if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s' \"$LUCIND_READ_ONLY_PATHS\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			outcome, err := tt.make(stub).Run(context.Background(), executor.Request{
				Prompt: "do the thing", WorktreePath: t.TempDir(), ReadOnlyPaths: wantPaths,
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if outcome.Stdout != string(want) {
				t.Errorf("child read-only context = %q, want %q", outcome.Stdout, want)
			}
		})
	}
}
