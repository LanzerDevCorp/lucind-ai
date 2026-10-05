package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/skillselect"
)

func TestSkillsSelect_FlagValidation(t *testing.T) {
	repo := initRepo(t)

	promptFile := filepath.Join(repo, "prompt.md")
	if err := os.WriteFile(promptFile, []byte("Test prompt"), 0o644); err != nil {
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
			wantErrSub: "usage: lucind-ai skills select --prompt <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]",
		},
		{
			name:       "unknown subcommand",
			args:       []string{"skills", "unknown"},
			wantExit:   1,
			wantErrSub: `lucind-ai: unknown subcommand "unknown"`,
		},
		{
			name:       "missing prompt flag",
			args:       []string{"skills", "select", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: --prompt is required",
		},
		{
			name:       "threshold below zero",
			args:       []string{"skills", "select", "--prompt", promptFile, "--threshold", "0", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got 0",
		},
		{
			name:       "threshold negative",
			args:       []string{"skills", "select", "--prompt", promptFile, "--threshold", "-0.5", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got -0.5",
		},
		{
			name:       "threshold above one",
			args:       []string{"skills", "select", "--prompt", promptFile, "--threshold", "1.5", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got 1.5",
		},
		{
			name:       "threshold not a number",
			args:       []string{"skills", "select", "--prompt", promptFile, "--threshold", "NaN", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: threshold must be in (0, 1], got NaN",
		},
		{
			name:       "unexpected extra args",
			args:       []string{"skills", "select", "--prompt", promptFile, "extra-arg", "--cwd", repo},
			wantExit:   1,
			wantErrSub: "lucind-ai: unexpected argument(s): extra-arg",
		},
		{
			name:       "missing prompt file",
			args:       []string{"skills", "select", "--prompt", filepath.Join(repo, "nonexistent-prompt.md"), "--cwd", repo},
			wantExit:   1,
			wantErrSub: "nonexistent-prompt.md",
		},
		{
			name:       "missing registry file",
			args:       []string{"skills", "select", "--prompt", promptFile, "--registry", filepath.Join(repo, "nonexistent-registry.md"), "--cwd", repo},
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
	promptFile := filepath.Join(repo, "prompt.md")
	if err := os.WriteFile(promptFile, []byte("Test prompt"), 0o644); err != nil {
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

	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"skills", "select",
		"--prompt", promptFile,
		"--cwd", repo,
	}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 when TYPESAFE_API_KEY is missing, got %d", code)
	}
	wantMsg := "lucind-ai: TYPESAFE_API_KEY is not set; export it or run lucind-ai install to store it in ~/.config/lucind/env"
	if !strings.Contains(stderr.String(), wantMsg) {
		t.Errorf("stderr = %q, want %q", stderr.String(), wantMsg)
	}
}

func TestSkillsSelect_Help(t *testing.T) {
	wantUsage := "usage: lucind-ai skills select --prompt <file|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]"

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

func TestSkillsSelect_StdinPrompt(t *testing.T) {
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

	t.Setenv("TYPESAFE_API_KEY", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	origStdin := stdinReader
	stdinReader = strings.NewReader("prompt content from stdin")
	defer func() { stdinReader = origStdin }()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"skills", "select",
		"--prompt", "-",
		"--cwd", repo,
	}, &stdout, &stderr)

	if code != 1 {
		t.Errorf("expected exit code 1 when TYPESAFE_API_KEY is missing, got %d", code)
	}
	wantMsg := "lucind-ai: TYPESAFE_API_KEY is not set; export it or run lucind-ai install to store it in ~/.config/lucind/env"
	if !strings.Contains(stderr.String(), wantMsg) {
		t.Errorf("stderr = %q, want %q", stderr.String(), wantMsg)
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

func TestSkillsSelect_SymlinkResolution(t *testing.T) {
	repo := initRepo(t)

	promptFile := filepath.Join(repo, "prompt.md")
	if err := os.WriteFile(promptFile, []byte("Implement feature"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	skillsTargetDir := t.TempDir()
	realSkillFile := filepath.Join(skillsTargetDir, "real-SKILL.md")
	if err := os.WriteFile(realSkillFile, []byte("# Real Skill"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	canonicalRealSkill, err := filepath.EvalSymlinks(realSkillFile)
	if err != nil {
		t.Fatalf("EvalSymlinks failed: %v", err)
	}

	symlinksDir := t.TempDir()
	symlinkSkillFile := filepath.Join(symlinksDir, "symlink-SKILL.md")
	if err := os.Symlink(realSkillFile, symlinkSkillFile); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	regDir := filepath.Join(repo, ".atl")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	regFile := filepath.Join(regDir, "skill-registry.md")
	regContent := "# Registry\n## Skills\n\n| Skill | Trigger / description | Scope | Path |\n| --- | --- | --- | --- |\n| `my-skill` | desc | repo | `" + symlinkSkillFile + "` |\n"
	if err := os.WriteFile(regFile, []byte(regContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Setenv("TYPESAFE_API_KEY", "mock-key")

	origSelect := selectSkills
	defer func() { selectSkills = origSelect }()

	var capturedSkills []skillselect.Skill
	selectSkills = func(ctx context.Context, client *skillselect.Client, skills []skillselect.Skill, in skillselect.Input, threshold float64) (skillselect.Result, error) {
		capturedSkills = skills
		return skillselect.Result{
			Model:     "mock-model",
			Threshold: threshold,
			Decisions: []skillselect.Decision{},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"skills", "select",
		"--prompt", promptFile,
		"--cwd", repo,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}

	if len(capturedSkills) != 1 {
		t.Fatalf("captured %d skills, want 1", len(capturedSkills))
	}

	if capturedSkills[0].Path != canonicalRealSkill {
		t.Errorf("captured skill path = %q, want canonical real path %q", capturedSkills[0].Path, canonicalRealSkill)
	}
}

func TestSkillsSelect_BriefFlagRejected(t *testing.T) {
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"skills", "select", "--brief", "task.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(skills select --brief) exit code = %d, want 1", code)
	}
	errOut := stderr.String()
	if strings.Contains(errOut, "flag provided but not defined") {
		t.Errorf("expected clean custom rejection, got Go generic flag error: %q", errOut)
	}
	if !strings.Contains(errOut, "--brief") || !strings.Contains(errOut, "--prompt") {
		t.Errorf("expected error message pointing from --brief to --prompt, got: %q", errOut)
	}
}

func TestSkillsSelect_KeyFromUserConfig(t *testing.T) {
	repo := initRepo(t)
	promptFile := filepath.Join(repo, "prompt.md")
	if err := os.WriteFile(promptFile, []byte("Test prompt"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	regDir := filepath.Join(repo, ".atl")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	regFile := filepath.Join(regDir, "skill-registry.md")
	regContent := "# Registry\n## Skills\n\n| Skill | Trigger / description | Scope | Path |\n| --- | --- | --- | --- |\n| `s1` | desc1 | repo | `/p1` |\n"
	if err := os.WriteFile(regFile, []byte(regContent), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	t.Setenv("TYPESAFE_API_KEY", "")
	tempXDG := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tempXDG)
	lucindConfigDir := filepath.Join(tempXDG, "lucind")
	if err := os.MkdirAll(lucindConfigDir, 0700); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lucindConfigDir, "env"), []byte("TYPESAFE_API_KEY=cfg-key\n"), 0600); err != nil {
		t.Fatalf("write env file failed: %v", err)
	}

	origSelect := selectSkills
	defer func() { selectSkills = origSelect }()
	selectCalled := false
	selectSkills = func(ctx context.Context, client *skillselect.Client, skills []skillselect.Skill, in skillselect.Input, threshold float64) (skillselect.Result, error) {
		selectCalled = true
		return skillselect.Result{
			Model:     "mock",
			Threshold: threshold,
			Decisions: []skillselect.Decision{},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"skills", "select",
		"--prompt", promptFile,
		"--cwd", repo,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !selectCalled {
		t.Errorf("expected selectSkills to be called using key from user config")
	}
}

