package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/dispatch"
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
  "lane_id": %q,
  "status": %q,
  "summary": "Completed lane work.",
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

	wantUsage := `usage: lucind-ai dispatch --cwd <dir> --allow <glob>... --brief <file|-> [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]
       lucind-ai wait <lane> [--cwd <dir>] [--timeout D]
       lucind-ai check [--out <path>]
       lucind-ai accept --lane <id>
       lucind-ai attest run -- <command> [args...]
       lucind-ai attest verify --command "<exact command string>"
       lucind-ai hook pre-tool-use|stop   (agy plugin handlers; stdin JSON)
       lucind-ai plugin install [--dir <staging root>]   (registers via agy plugin install)
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

	if !strings.Contains(usage, "lucind-ai dispatch") {
		t.Errorf("usage string unexpectedly missing 'lucind-ai dispatch'")
	}
	if !strings.Contains(usage, "lucind-ai wait") {
		t.Errorf("usage string unexpectedly missing 'lucind-ai wait'")
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

func TestDispatchHelp(t *testing.T) {
	ctx := context.Background()
	wantUsageLine := "usage: lucind-ai dispatch --cwd <dir> --allow <glob>... --brief <file|-> [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]"
	for _, flag := range []string{"--help", "-help", "-h", "help"} {
		t.Run("flag_"+flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(ctx, []string{"dispatch", flag}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run(dispatch %s) exit code = %d, want 0; stderr = %q", flag, code, stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, wantUsageLine) {
				t.Errorf("stdout missing usage line; got %q", out)
			}
			for _, expectedFlag := range []string{"-cwd", "-allow", "-brief", "-model", "-timeout", "-detach", "-lane", "-min-quota"} {
				if !strings.Contains(out, expectedFlag) {
					t.Errorf("stdout missing flag %q; got %q", expectedFlag, out)
				}
			}
		})
	}
}

func TestWaitHelp(t *testing.T) {
	ctx := context.Background()
	wantUsageLine := "usage: lucind-ai wait <lane> [--cwd <dir>] [--timeout D]"
	for _, flag := range []string{"--help", "-help", "-h", "help"} {
		t.Run("flag_"+flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(ctx, []string{"wait", flag}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("run(wait %s) exit code = %d, want 0; stderr = %q", flag, code, stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, wantUsageLine) {
				t.Errorf("stdout missing usage line; got %q", out)
			}
			for _, expectedFlag := range []string{"-cwd", "-timeout"} {
				if !strings.Contains(out, expectedFlag) {
					t.Errorf("stdout missing flag %q; got %q", expectedFlag, out)
				}
			}
		})
	}
}

func TestDispatchMissingRequiredFlags(t *testing.T) {
	ctx := context.Background()
	wantUsageLine := "usage: lucind-ai dispatch --cwd <dir> --allow <glob>... --brief <file|-> [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no flags",
			args:    []string{"dispatch"},
			wantErr: "--cwd is required",
		},
		{
			name:    "missing allow and brief",
			args:    []string{"dispatch", "--cwd", "/tmp"},
			wantErr: "--allow is required",
		},
		{
			name:    "missing brief",
			args:    []string{"dispatch", "--cwd", "/tmp", "--allow", "src/**"},
			wantErr: "--brief is required",
		},
		{
			name:    "unexpected positional arg",
			args:    []string{"dispatch", "--cwd", "/tmp", "--allow", "src/**", "--brief", "b.md", "unexpected"},
			wantErr: "unexpected argument(s)",
		},
		{
			name:    "invalid timeout",
			args:    []string{"dispatch", "--cwd", "/tmp", "--allow", "src/**", "--brief", "b.md", "--timeout", "xyz"},
			wantErr: "invalid --timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(ctx, tc.args, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("run(%v) exit code = %d, want 1", tc.args, code)
			}
			errOut := stderr.String()
			if !strings.Contains(errOut, tc.wantErr) {
				t.Errorf("stderr missing %q; got %q", tc.wantErr, errOut)
			}
			if !strings.Contains(errOut, wantUsageLine) {
				t.Errorf("stderr missing usage line; got %q", errOut)
			}
		})
	}
}

func TestWaitMissingLane(t *testing.T) {
	ctx := context.Background()
	wantUsageLine := "usage: lucind-ai wait <lane> [--cwd <dir>] [--timeout D]"

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no args",
			args:    []string{"wait"},
			wantErr: "lane is required",
		},
		{
			name:    "only cwd flag",
			args:    []string{"wait", "--cwd", "/tmp"},
			wantErr: "lane is required",
		},
		{
			name:    "unexpected extra positional arg",
			args:    []string{"wait", "lane-1", "extra"},
			wantErr: "unexpected argument(s)",
		},
		{
			name:    "invalid timeout",
			args:    []string{"wait", "lane-1", "--timeout", "invalid"},
			wantErr: "invalid --timeout",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(ctx, tc.args, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("run(%v) exit code = %d, want 1", tc.args, code)
			}
			errOut := stderr.String()
			if !strings.Contains(errOut, tc.wantErr) {
				t.Errorf("stderr missing %q; got %q", tc.wantErr, errOut)
			}
			if !strings.Contains(errOut, wantUsageLine) {
				t.Errorf("stderr missing usage line; got %q", errOut)
			}
		})
	}
}

func TestDispatchExecution(t *testing.T) {
	ctx := context.Background()
	repoDir := initRepo(t)

	briefFile := filepath.Join(repoDir, "task_brief.md")
	if err := os.WriteFile(briefFile, []byte("Implement feature X"), 0644); err != nil {
		t.Fatal(err)
	}

	origDispatchRun := dispatchRun
	defer func() { dispatchRun = origDispatchRun }()

	var capturedOpts dispatch.Options
	mockOutput := dispatch.Output{
		Lane:       "20261003-120000-abcd",
		PaneID:     "w1:p1",
		Cwd:        repoDir,
		Status:     "running",
		ResultPath: filepath.Join(repoDir, ".lucind/lanes/20261003-120000-abcd/result.json"),
	}

	// 1. Success with file brief and multiple allows
	dispatchRun = func(c context.Context, opts dispatch.Options, runner dispatch.HerdrRunner) (dispatch.Output, int, error) {
		capturedOpts = opts
		return mockOutput, 0, nil
	}

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{
		"dispatch",
		"--cwd", repoDir,
		"--allow", "src/**,pkg/**",
		"--allow", "cmd/**",
		"--brief", briefFile,
		"--model", "gemini-3.8-flash-high",
		"--timeout", "30m",
		"--detach",
		"--lane", "existing-lane",
		"--min-quota", "0.5",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(dispatch) exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	if capturedOpts.Cwd != repoDir {
		t.Errorf("captured Cwd = %q, want %q", capturedOpts.Cwd, repoDir)
	}
	wantAllow := []string{"src/**", "pkg/**", "cmd/**"}
	if len(capturedOpts.Allow) != len(wantAllow) {
		t.Errorf("captured Allow = %v, want %v", capturedOpts.Allow, wantAllow)
	}
	if capturedOpts.Brief != "Implement feature X" {
		t.Errorf("captured Brief = %q, want 'Implement feature X'", capturedOpts.Brief)
	}
	if capturedOpts.Model != "gemini-3.8-flash-high" {
		t.Errorf("captured Model = %q, want 'gemini-3.8-flash-high'", capturedOpts.Model)
	}
	if capturedOpts.Timeout != 30*time.Minute {
		t.Errorf("captured Timeout = %v, want 30m", capturedOpts.Timeout)
	}
	if !capturedOpts.Detach {
		t.Errorf("captured Detach = false, want true")
	}
	if capturedOpts.LaneID != "existing-lane" {
		t.Errorf("captured LaneID = %q, want 'existing-lane'", capturedOpts.LaneID)
	}
	if capturedOpts.MinQuota != 0.5 {
		t.Errorf("captured MinQuota = %v, want 0.5", capturedOpts.MinQuota)
	}

	var outJSON dispatch.Output
	if err := json.Unmarshal(stdout.Bytes(), &outJSON); err != nil {
		t.Fatalf("failed to parse stdout as JSON: %v; output = %q", err, stdout.String())
	}
	if outJSON.Lane != mockOutput.Lane {
		t.Errorf("outJSON.Lane = %q, want %q", outJSON.Lane, mockOutput.Lane)
	}

	// 2. Brief from stdin
	origStdin := stdinReader
	defer func() { stdinReader = origStdin }()
	stdinReader = strings.NewReader("stdin brief content")

	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{
		"dispatch",
		"--cwd", repoDir,
		"--allow", "src/**",
		"--brief", "-",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(dispatch --brief -) exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if capturedOpts.Brief != "stdin brief content" {
		t.Errorf("captured Brief = %q, want 'stdin brief content'", capturedOpts.Brief)
	}

	// 3. Exit code 3 (failed lane)
	dispatchRun = func(c context.Context, opts dispatch.Options, runner dispatch.HerdrRunner) (dispatch.Output, int, error) {
		out := mockOutput
		out.Status = "failed"
		return out, 3, nil
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{
		"dispatch",
		"--cwd", repoDir,
		"--allow", "src/**",
		"--brief", briefFile,
	}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if !strings.Contains(stdout.String(), `"status": "failed"`) {
		t.Errorf("stdout should contain failed status JSON; got %q", stdout.String())
	}

	// 4. Exit code 4 (timeout)
	dispatchRun = func(c context.Context, opts dispatch.Options, runner dispatch.HerdrRunner) (dispatch.Output, int, error) {
		out := mockOutput
		out.Status = "timeout"
		return out, 4, nil
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{
		"dispatch",
		"--cwd", repoDir,
		"--allow", "src/**",
		"--brief", briefFile,
	}, &stdout, &stderr)
	if code != 4 {
		t.Fatalf("exit code = %d, want 4", code)
	}
	if !strings.Contains(stdout.String(), `"status": "timeout"`) {
		t.Errorf("stdout should contain timeout status JSON; got %q", stdout.String())
	}
}

func TestWaitExecution(t *testing.T) {
	ctx := context.Background()
	repoDir := initRepo(t)

	origWaitRun := waitRun
	defer func() { waitRun = origWaitRun }()

	var capturedRepoRoot, capturedLaneID string
	var capturedTimeout time.Duration

	mockOutput := dispatch.Output{
		Lane:       "20261003-120000-abcd",
		PaneID:     "w1:pWait1",
		Cwd:        repoDir,
		Status:     "done",
		ResultPath: filepath.Join(repoDir, ".lucind/lanes/20261003-120000-abcd/result.json"),
	}

	waitRun = func(c context.Context, repoRoot, laneID string, timeout time.Duration, runner dispatch.HerdrRunner) (dispatch.Output, int, error) {
		capturedRepoRoot = repoRoot
		capturedLaneID = laneID
		capturedTimeout = timeout
		return mockOutput, 0, nil
	}

	// 1. Positional lane first: lucind-ai wait <lane> [--cwd <dir>] [--timeout D]
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{
		"wait", "20261003-120000-abcd",
		"--cwd", repoDir,
		"--timeout", "45m",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(wait) exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if capturedLaneID != "20261003-120000-abcd" {
		t.Errorf("captured LaneID = %q, want '20261003-120000-abcd'", capturedLaneID)
	}
	if capturedRepoRoot != repoDir {
		t.Errorf("captured RepoRoot = %q, want %q", capturedRepoRoot, repoDir)
	}
	if capturedTimeout != 45*time.Minute {
		t.Errorf("captured Timeout = %v, want 45m", capturedTimeout)
	}
	var outJSON dispatch.Output
	if err := json.Unmarshal(stdout.Bytes(), &outJSON); err != nil {
		t.Fatalf("failed to parse stdout as JSON: %v; output = %q", err, stdout.String())
	}
	if outJSON.Status != "done" {
		t.Errorf("outJSON.Status = %q, want 'done'", outJSON.Status)
	}

	// 2. Flags first: lucind-ai wait [--flags] <lane>
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{
		"wait",
		"--cwd", repoDir,
		"--timeout", "15m",
		"20261003-120000-abcd",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(wait flags first) exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if capturedLaneID != "20261003-120000-abcd" {
		t.Errorf("captured LaneID = %q, want '20261003-120000-abcd'", capturedLaneID)
	}

	// 3. Exit code 3 (failed lane)
	waitRun = func(c context.Context, repoRoot, laneID string, timeout time.Duration, runner dispatch.HerdrRunner) (dispatch.Output, int, error) {
		out := mockOutput
		out.Status = "failed"
		return out, 3, nil
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{
		"wait", "20261003-120000-abcd",
		"--cwd", repoDir,
	}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}

	// 4. Exit code 4 (timeout)
	waitRun = func(c context.Context, repoRoot, laneID string, timeout time.Duration, runner dispatch.HerdrRunner) (dispatch.Output, int, error) {
		out := mockOutput
		out.Status = "timeout"
		return out, 4, nil
	}
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{
		"wait", "20261003-120000-abcd",
		"--cwd", repoDir,
	}, &stdout, &stderr)
	if code != 4 {
		t.Fatalf("exit code = %d, want 4", code)
	}
}

