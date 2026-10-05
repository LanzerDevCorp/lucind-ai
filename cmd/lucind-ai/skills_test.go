package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsSelect_FlagValidation(t *testing.T) {
	repo := initRepo(t)

	briefFile := filepath.Join(repo, "brief.md")
	if err := os.WriteFile(briefFile, []byte("Test brief"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantErrSub string
	}{
		{
			name:       "missing subcommand",
			args:       []string{"skills"},
			wantExit:   1,
			wantErrSub: "usage: lucind-ai skills select --brief <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]",
		},
		{
			name:       "unknown subcommand",
			args:       []string{"skills", "unknown"},
			wantExit:   1,
			wantErrSub: `lucind-ai: unknown subcommand "unknown"`,
		},
		{
			name:       "missing brief flag",
			args:       []string{"skills", "select", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: --brief is required",
		},
		{
			name:       "threshold below zero",
			args:       []string{"skills", "select", "--brief", briefFile, "--threshold", "0", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got 0",
		},
		{
			name:       "threshold negative",
			args:       []string{"skills", "select", "--brief", briefFile, "--threshold", "-0.5", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got -0.5",
		},
		{
			name:       "threshold above one",
			args:       []string{"skills", "select", "--brief", briefFile, "--threshold", "1.5", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got 1.5",
		},
		{
			name:       "threshold not a number",
			args:       []string{"skills", "select", "--brief", briefFile, "--threshold", "NaN", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got NaN",
		},
		{
			name:       "unexpected extra args",
			args:       []string{"skills", "select", "--brief", briefFile, "extra-arg", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: unexpected argument(s): extra-arg",
		},
		{
			name:       "missing brief file",
			args:       []string{"skills", "select", "--brief", filepath.Join(repo, "nonexistent-brief.md"), "--cwd", repo},
			wantExit:   1,
			wantErrSub: "nonexistent-brief.md",
		},
		{
			name:       "missing registry file",
			args:       []string{"skills", "select", "--brief", briefFile, "--registry", filepath.Join(repo, "nonexistent-registry.md"), "--cwd", repo},
			wantExit:   1,
			wantErrSub: "nonexistent-registry.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tt.args, &stdout, &stderr)
			if code != tt.wantExit {
				t.Errorf("run(%v) exit code = %d, want %d (stderr: %s)", tt.args, code, tt.wantExit, stderr.String())
			}
			if tt.wantErrSub != "" && !strings.Contains(stderr.String(), tt.wantErrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tt.wantErrSub)
			}
		})
	}
}

func TestSkillsSelect_MissingAPIKey(t *testing.T) {
	repo := initRepo(t)
	briefFile := filepath.Join(repo, "brief.md")
	if err := os.WriteFile(briefFile, []byte("Test brief"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	regDir := filepath.Join(repo, ".atl")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	regFile := filepath.Join(regDir, "skill-registry.md")
	regContent := `# Registry
## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| ` + "`s1`" + ` | desc1 | repo | ` + "`/p1`" + ` |
`
	if err := os.WriteFile(regFile, []byte(regContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	origKey := os.Getenv("TYPESAFE_API_KEY")
	_ = os.Unsetenv("TYPESAFE_API_KEY")
	defer func() {
		if origKey != "" {
			_ = os.Setenv("TYPESAFE_API_KEY", origKey)
		}
	}()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"skills", "select",
		"--brief", briefFile,
		"--cwd", repo,
	}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 when TYPESAFE_API_KEY is missing, got %d", code)
	}
	if !strings.Contains(stderr.String(), "lucind-ai: TYPESAFE_API_KEY environment variable is required") {
		t.Errorf("stderr = %q, want TYPESAFE_API_KEY missing error", stderr.String())
	}
}

func TestSkillsSelect_Help(t *testing.T) {
	wantUsage := "usage: lucind-ai skills select --brief <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]"

	t.Run("skills_help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"skills", "--help"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), wantUsage) {
			t.Errorf("stdout = %q, want usage info", stdout.String())
		}
	})

	t.Run("skills_select_help", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"skills", "select", "--help"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
		}
	})
}

func TestSkillsSelect_StdinBrief(t *testing.T) {
	repo := initRepo(t)
	regDir := filepath.Join(repo, ".atl")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	regFile := filepath.Join(regDir, "skill-registry.md")
	regContent := `# Registry
## Skills

| Skill | Trigger / description | Scope | Path |
| --- | --- | --- | --- |
| ` + "`s1`" + ` | desc1 | repo | ` + "`/p1`" + ` |
`
	if err := os.WriteFile(regFile, []byte(regContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	origKey := os.Getenv("TYPESAFE_API_KEY")
	_ = os.Unsetenv("TYPESAFE_API_KEY")
	defer func() {
		if origKey != "" {
			_ = os.Setenv("TYPESAFE_API_KEY", origKey)
		}
	}()

	origStdin := stdinReader
	stdinReader = strings.NewReader("brief content from stdin")
	defer func() { stdinReader = origStdin }()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"skills", "select",
		"--brief", "-",
		"--cwd", repo,
	}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 when TYPESAFE_API_KEY is missing, got %d", code)
	}
	if !strings.Contains(stderr.String(), "lucind-ai: TYPESAFE_API_KEY environment variable is required") {
		t.Errorf("stderr = %q, want TYPESAFE_API_KEY missing error (indicating brief was read from stdin)", stderr.String())
	}
}

func TestSkillsDispatch_Direct(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := skillsDispatch(context.Background(), []string{}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("skillsDispatch() empty args code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lucind-ai skills select") {
		t.Errorf("stderr = %q, want skillsUsage", stderr.String())
	}
}
