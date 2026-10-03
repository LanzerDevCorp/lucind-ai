package check

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCheckMissingScript(t *testing.T) {
	dir := t.TempDir()
	passed, out, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if passed {
		t.Fatalf("Check() passed = true, want false")
	}
	wantMsg := "no lucind-checks.sh found at the project root; this project has not defined its verification checks yet"
	if out != wantMsg {
		t.Fatalf("Check() output = %q, want %q", out, wantMsg)
	}
}

func TestCheckScriptPasses(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho CHECKS_PASSED_OK\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	passed, out, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !passed {
		t.Fatalf("Check() passed = false, want true; output = %s", out)
	}
	if !strings.Contains(out, "CHECKS_PASSED_OK") {
		t.Errorf("Check() output = %q, want to contain CHECKS_PASSED_OK", out)
	}
}

func TestCheckScriptFails(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho CHECKS_FAILED_ERR\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	passed, out, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if passed {
		t.Fatalf("Check() passed = true, want false")
	}
	if !strings.Contains(out, "CHECKS_FAILED_ERR") {
		t.Errorf("Check() output = %q, want to contain CHECKS_FAILED_ERR", out)
	}
}

func TestCheckEnvironmentScrubbing(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nenv\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	t.Setenv("SECRET_TOKEN_XYZ", "supersecret")
	t.Setenv("UNSAFE_CUSTOM_VAR", "unsafe_value")
	t.Setenv("USER", "testuser")

	passed, out, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !passed {
		t.Fatalf("Check() passed = false, want true; output = %s", out)
	}

	if strings.Contains(out, "SECRET_TOKEN_XYZ") {
		t.Errorf("Check() output contains unwhitelisted variable SECRET_TOKEN_XYZ")
	}
	if strings.Contains(out, "UNSAFE_CUSTOM_VAR") {
		t.Errorf("Check() output contains unwhitelisted variable UNSAFE_CUSTOM_VAR")
	}
	if !strings.Contains(out, "PATH=") {
		t.Errorf("Check() output missing PATH")
	}
}

func TestCheckMissingPathFails(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	t.Setenv("PATH", "")
	_, _, err := Check(context.Background(), dir)
	if err == nil {
		t.Fatalf("Check() error = nil, want error when PATH is empty")
	}
}

func TestCheckRelativeRootFails(t *testing.T) {
	_, _, err := Check(context.Background(), "relative/path/to/repo")
	if err == nil {
		t.Fatalf("Check() error = nil, want error when repo root is relative")
	}
}

func TestCheckSymlinkRoot(t *testing.T) {
	realDir := t.TempDir()
	symDir := filepath.Join(t.TempDir(), "symlink-target")
	if err := os.Symlink(realDir, symDir); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	script := "#!/bin/sh\npwd -P\nexit 0\n"
	if err := os.WriteFile(filepath.Join(realDir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	passed, out, err := Check(context.Background(), symDir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !passed {
		t.Fatalf("Check() passed = false, want true")
	}
	realCanonical, err := filepath.EvalSymlinks(realDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.TrimSpace(out), realCanonical) {
		t.Errorf("Check() output = %q, want canonical directory %q", out, realCanonical)
	}
}

func TestCheckTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow timeout test in short mode")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 30\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	passed, _, err := Check(ctx, dir)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Check() error = %v, want nil on timeout", err)
	}
	if passed {
		t.Fatalf("Check() passed = true, want false on timeout")
	}
	if duration > 3*time.Second {
		t.Fatalf("Check() took %v, want prompt termination", duration)
	}
}

func TestCheckProcessGroupTermination(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping process group test in short mode")
	}
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := `#!/bin/sh
sh -c 'trap "" TERM; echo $$ > "$1"; while :; do sleep 1; done' child "` + pidFile + `" &
wait
`
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	// Use short grace period for fast testing
	origGrace := terminationGrace
	terminationGrace = 200 * time.Millisecond
	defer func() { terminationGrace = origGrace }()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	passed, _, err := Check(ctx, dir)
	if passed || err != nil {
		t.Fatalf("Check() = %t, %v, want false, nil", passed, err)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	var pid int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid); err != nil {
		t.Fatalf("parse pid: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("descendant process %d still exists: %v", pid, err)
	}
}

func TestCheckOutputBoundedBuffer(t *testing.T) {
	dir := t.TempDir()
	// Generate > 1MB output
	script := "#!/bin/sh\nhead -c 1200000 /dev/zero | tr '\\000' 'A'\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	passed, out, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !passed {
		t.Fatalf("Check() passed = false, want true")
	}

	if !strings.Contains(out, "[output truncated]") {
		t.Errorf("Check() output should contain '[output truncated]'")
	}
	// The bounded buffer max is 1MB + truncation notice
	if len(out) > (1<<20)+200 {
		t.Errorf("Check() output length %d exceeded expected bound", len(out))
	}
}

func TestRun(t *testing.T) {
	t.Run("passing script writes stdout and returns nil", func(t *testing.T) {
		dir := t.TempDir()
		script := "#!/bin/sh\necho RUN_PASS\nexit 0\n"
		if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), dir, &stdout, &stderr)
		if err != nil {
			t.Fatalf("Run() error = %v, want nil", err)
		}
		if !strings.Contains(stdout.String(), "RUN_PASS") {
			t.Errorf("stdout = %q, want RUN_PASS", stdout.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("stderr = %q, want empty", stderr.String())
		}
	})

	t.Run("failing script writes stderr and returns ErrCheckFailed", func(t *testing.T) {
		dir := t.TempDir()
		script := "#!/bin/sh\necho RUN_FAIL\nexit 1\n"
		if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), dir, &stdout, &stderr)
		if !errors.Is(err, ErrCheckFailed) {
			t.Fatalf("Run() error = %v, want ErrCheckFailed", err)
		}
		if !strings.Contains(stderr.String(), "RUN_FAIL") {
			t.Errorf("stderr = %q, want RUN_FAIL", stderr.String())
		}
	})

	t.Run("missing script writes stderr and returns ErrCheckFailed", func(t *testing.T) {
		dir := t.TempDir()
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), dir, &stdout, &stderr)
		if !errors.Is(err, ErrCheckFailed) {
			t.Fatalf("Run() error = %v, want ErrCheckFailed", err)
		}
		if !strings.Contains(stderr.String(), "no lucind-checks.sh found") {
			t.Errorf("stderr = %q, want missing script message", stderr.String())
		}
	})

	t.Run("invalid root returns error directly", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), "not-absolute", &stdout, &stderr)
		if err == nil {
			t.Fatalf("Run() error = nil, want error for relative root")
		}
		if errors.Is(err, ErrCheckFailed) {
			t.Fatalf("Run() error = ErrCheckFailed, want setup error")
		}
	})
}

func TestCheckContextPreCancelled(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := Check(ctx, dir)
	if err == nil {
		t.Fatal("Check() error = nil, want error for pre-cancelled context")
	}
}

func TestCheckSignalsAndExitCodes(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{name: "exit 42", script: "#!/bin/sh\necho error-code-42\nexit 42\n"},
		{name: "self SIGTERM", script: "#!/bin/sh\necho self-term\nkill -TERM $$\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(tt.script), 0o755); err != nil {
				t.Fatal(err)
			}
			passed, out, err := Check(context.Background(), dir)
			if err != nil {
				t.Fatalf("Check() error = %v, want nil", err)
			}
			if passed {
				t.Fatalf("Check() passed = true, want false")
			}
			if out == "" {
				t.Errorf("Check() output is empty, want captured output")
			}
		})
	}
}

func TestCheckCapturesCombinedStdoutStderr(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho stdout_message\necho stderr_message >&2\nexit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "lucind-checks.sh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	passed, out, err := Check(context.Background(), dir)
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !passed {
		t.Fatalf("Check() passed = false, want true")
	}
	if !strings.Contains(out, "stdout_message") || !strings.Contains(out, "stderr_message") {
		t.Errorf("Check() output = %q, want both stdout and stderr messages", out)
	}
}

func TestCheckNonExistentDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	_, _, err := Check(context.Background(), dir)
	if err == nil {
		t.Fatal("Check() error = nil, want error for non-existent directory")
	}
}
