package executor_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
)

func TestConcreteExecutorsExposeRequiredSkillsToChild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	wantSkills := []string{"lucind-executor", "lucind-apply"}
	wantJSON, err := json.Marshal(wantSkills)
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
			if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s' \"$LUCIND_REQUIRED_SKILLS\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			outcome, err := tt.make(stub).Run(context.Background(), executor.Request{
				Prompt:         "do the thing",
				WorktreePath:   t.TempDir(),
				RequiredSkills: wantSkills,
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if outcome.Stdout != string(wantJSON) {
				t.Errorf("child required skills = %q, want %q", outcome.Stdout, wantJSON)
			}
		})
	}
}

func TestConcreteExecutorsStripInheritedRequiredSkills(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
	}

	t.Setenv("LUCIND_REQUIRED_SKILLS", `["inherited-skill-value"]`)

	wantSkills := []string{"lucind-verify"}
	wantJSON, err := json.Marshal(wantSkills)
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
		t.Run(tt.name+"/with_override", func(t *testing.T) {
			stub := filepath.Join(t.TempDir(), "executor-stub.sh")
			if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s' \"$LUCIND_REQUIRED_SKILLS\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			outcome, err := tt.make(stub).Run(context.Background(), executor.Request{
				Prompt:         "do the thing",
				WorktreePath:   t.TempDir(),
				RequiredSkills: wantSkills,
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if outcome.Stdout != string(wantJSON) {
				t.Errorf("child required skills = %q, want %q", outcome.Stdout, wantJSON)
			}
		})

		t.Run(tt.name+"/with_empty_declaration", func(t *testing.T) {
			stub := filepath.Join(t.TempDir(), "executor-stub.sh")
			if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s' \"$LUCIND_REQUIRED_SKILLS\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			outcome, err := tt.make(stub).Run(context.Background(), executor.Request{
				Prompt:         "do the thing",
				WorktreePath:   t.TempDir(),
				RequiredSkills: nil,
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if outcome.Stdout != "" {
				t.Errorf("child required skills = %q, want empty string", outcome.Stdout)
			}
		})
	}
}

func TestConcreteExecutorsEmptyRequiredSkillsDeclaration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in -short mode")
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
			if err := os.WriteFile(stub, []byte("#!/bin/sh\nprintf '%s' \"$LUCIND_REQUIRED_SKILLS\"\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			outcome, err := tt.make(stub).Run(context.Background(), executor.Request{
				Prompt:         "do the thing",
				WorktreePath:   t.TempDir(),
				RequiredSkills: []string{},
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if outcome.Stdout != "" {
				t.Errorf("child required skills = %q, want empty string", outcome.Stdout)
			}
		})
	}
}
