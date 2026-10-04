package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.name", "Test User")
	runGit(t, root, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "seed commit")
	return root
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-c", "user.email=cli-test@example.com",
		"-c", "user.name=cli-test",
		"-C", dir,
	}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeResultJSON(t *testing.T, repoDir, laneID, status string) {
	t.Helper()
	env := fmt.Sprintf(`{
  "packet_id": %q,
  "status": %q,
  "summary": "Completed packet work.",
  "hard_stops": []
}`, laneID, status)
	resPath := lane.ResultPath(repoDir, laneID)
	if err := os.MkdirAll(filepath.Dir(resPath), 0o755); err != nil {
		t.Fatalf("mkdir lane dir: %v", err)
	}
	if err := os.WriteFile(resPath, []byte(env), 0o644); err != nil {
		t.Fatalf("write result.json: %v", err)
	}
}

func writeChecksScript(t *testing.T, repoDir string, exitCode int, output string) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q\nexit %d\n", output, exitCode)
	path := filepath.Join(repoDir, "lucind-checks.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write lucind-checks.sh: %v", err)
	}
}

func TestUsageAndHelp(t *testing.T) {
	ctx := context.Background()

	wantUsage := `usage: lucind-ai check [--out <path>]
       lucind-ai accept --lane <id>
       lucind-ai attest run -- <command> [args...]
       lucind-ai attest verify --command "<exact command string>"
       lucind-ai hook stop --state-dir <dir> --result <path> [--max-continues <n>]
       lucind-ai --version`

	// 1. Missing args prints usage to stderr and exits 1
	{
		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run() exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), wantUsage) {
			t.Fatalf("stderr = %q, want it to contain usage:\n%s", stderr.String(), wantUsage)
		}
	}

	// 2. Help flags print usage to stdout and exit 0
	for _, flag := range []string{"--help", "-help", "-h", "help"} {
		t.Run("flag_"+flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(ctx, []string{flag}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run(%s) exit code = %d, want 0", flag, code)
			}
			if !strings.Contains(stdout.String(), wantUsage) {
				t.Fatalf("stdout = %q, want it to contain usage:\n%s", stdout.String(), wantUsage)
			}
		})
	}

	// 3. Unknown subcommand prints error and usage to stderr and exits 1
	{
		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{"nonexistent"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run(nonexistent) exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), `unknown subcommand "nonexistent"`) {
			t.Errorf("stderr = %q, want unknown subcommand message", stderr.String())
		}
		if !strings.Contains(stderr.String(), wantUsage) {
			t.Errorf("stderr = %q, want usage output", stderr.String())
		}
	}

	// 4. Usage output contains only surviving commands and no legacy/orchestration commands
	deletedCommands := []string{
		"lucind-ai run",
		"lucind-ai explore",
		"lucind-ai split",
		"lucind-ai feature",
		"lucind-ai reconcile",
		"lucind-ai defect",
		"lucind-ai worktree",
		"lucind-ai integrate",
		"lucind-ai usage",
		"lucind-ai rules",
	}
	for _, deleted := range deletedCommands {
		if strings.Contains(usage, deleted) {
			t.Errorf("usage string unexpectedly contains deleted command %q", deleted)
		}
	}
}

func TestVersion(t *testing.T) {
	ctx := context.Background()
	for _, flag := range []string{"--version", "-version", "-v", "version"} {
		t.Run("flag_"+flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(ctx, []string{flag}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run(%s) exit code = %d, want 0; stderr = %q", flag, code, stderr.String())
			}
			out := stdout.String()
			if !strings.HasPrefix(out, "lucind-ai ") {
				t.Errorf("stdout = %q, want prefix 'lucind-ai '", out)
			}
			if !strings.Contains(out, version) {
				t.Errorf("stdout = %q, want it to contain version %q", out, version)
			}
		})
	}
}

func TestCheckMissingScript(t *testing.T) {
	repoDir := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(check) exit code = %d, want 1; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no lucind-checks.sh found at the project root") {
		t.Fatalf("stderr = %q, want it to contain missing script message", stderr.String())
	}
}

func TestCheckScriptPasses(t *testing.T) {
	repoDir := initRepo(t)
	writeChecksScript(t, repoDir, 0, "PASS: all checks passed")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(check) exit code = %d, want 0; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "PASS: all checks passed") {
		t.Errorf("stdout = %q, want it to contain script output", out)
	}
	if !strings.Contains(out, "status:        passed") {
		t.Errorf("stdout = %q, want status: passed", out)
	}
	if !strings.Contains(out, "duration:") {
		t.Errorf("stdout = %q, want duration:", out)
	}
	if !strings.Contains(out, "resolved root:") {
		t.Errorf("stdout = %q, want resolved root:", out)
	}
}

func TestCheckScriptFails(t *testing.T) {
	repoDir := initRepo(t)
	writeChecksScript(t, repoDir, 1, "FAIL: tests failed")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(check) exit code = %d, want 1; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "FAIL: tests failed") {
		t.Fatalf("stderr = %q, want script output in stderr", stderr.String())
	}
}

func TestCheckOutFlag(t *testing.T) {
	repoDir := initRepo(t)
	writeChecksScript(t, repoDir, 0, "PASS: check ran")

	logPath := filepath.Join(repoDir, "logs", "verify-mechanical.log")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "--out", logPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(check --out) exit code = %d, want 0; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file %s: %v", logPath, err)
	}
	logContent := string(data)
	if !strings.Contains(logContent, "=== lucind-ai mechanical check ===") {
		t.Errorf("logContent missing banner: %s", logContent)
	}
	if !strings.Contains(logContent, "Command: lucind-checks.sh") {
		t.Errorf("logContent missing Command line: %s", logContent)
	}
	if !strings.Contains(logContent, "Exit Code: 0") {
		t.Errorf("logContent missing Exit Code line: %s", logContent)
	}
	if !strings.Contains(logContent, "PASS: check ran") {
		t.Errorf("logContent missing transcript: %s", logContent)
	}
}

func TestCheckUnexpectedArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "extra-arg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(check extra-arg) exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lucind-ai check [--out <path>]") {
		t.Errorf("stderr = %q, want check usage", stderr.String())
	}
}

func TestAcceptSubcommand(t *testing.T) {
	ctx := context.Background()

	// 1. Missing --lane flag fails
	{
		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{"accept"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run(accept) exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "--lane is required") {
			t.Errorf("stderr = %q, want '--lane is required'", stderr.String())
		}
		if !strings.Contains(stderr.String(), "usage: lucind-ai accept --lane <id>") {
			t.Errorf("stderr = %q, want accept usage", stderr.String())
		}
	}

	// 2. Unexpected positional argument fails
	{
		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{"accept", "--lane", "lane-1", "extra"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run(accept --lane lane-1 extra) exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "unexpected argument(s)") {
			t.Errorf("stderr = %q, want 'unexpected argument(s)'", stderr.String())
		}
	}

	// 3. Acceptance flow: lane valid, result.json done, checks pass -> exit 0
	{
		repoDir := initRepo(t)
		writeChecksScript(t, repoDir, 0, "PASS: all checks passed")
		l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
		if err != nil {
			t.Fatalf("create lane: %v", err)
		}
		writeResultJSON(t, repoDir, l.ID, "done")

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(repoDir); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{"accept", "--lane", l.ID}, &stdout, &stderr)
		_ = os.Chdir(cwd)

		if code != 0 {
			t.Fatalf("run(accept --lane %s) exit code = %d, want 0; stderr = %q, stdout = %q", l.ID, code, stderr.String(), stdout.String())
		}
		if !strings.Contains(stdout.String(), fmt.Sprintf("lane %s accepted", l.ID)) {
			t.Errorf("stdout = %q, want 'lane %s accepted'", stdout.String(), l.ID)
		}
	}

	// 4. Rejection flow: lane with failed result.json -> exit 1
	{
		repoDir := initRepo(t)
		writeChecksScript(t, repoDir, 0, "PASS: all checks passed")
		l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
		if err != nil {
			t.Fatalf("create lane: %v", err)
		}
		writeResultJSON(t, repoDir, l.ID, "failed")

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(repoDir); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		code := run(ctx, []string{"accept", "--lane", l.ID}, &stdout, &stderr)
		_ = os.Chdir(cwd)

		if code != 1 {
			t.Fatalf("run(accept --lane %s) exit code = %d, want 1; stderr = %q, stdout = %q", l.ID, code, stderr.String(), stdout.String())
		}
		if !strings.Contains(stderr.String(), fmt.Sprintf("lane %s rejected", l.ID)) {
			t.Errorf("stderr = %q, want 'lane %s rejected'", stderr.String(), l.ID)
		}
	}
}
