package check

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// DefaultTimeout is the maximum duration for running verification checks
	// when the caller's context does not specify a deadline.
	DefaultTimeout = 10 * time.Minute

	// MaxOutputBytes is the maximum combined output captured from the script.
	MaxOutputBytes = 1 << 20 // 1 MB

	// MissingScriptOutput is returned when lucind-checks.sh is absent.
	MissingScriptOutput = "no lucind-checks.sh found at the project root; this project has not defined its verification checks yet"
)

var (
	// ErrCheckFailed is returned by Run when verification checks fail.
	ErrCheckFailed = errors.New("check: verification checks failed")

	// SafeEnvKeys defines the allowed environment variables passed to the check subprocess.
	SafeEnvKeys = []string{
		"PATH",
		"HOME",
		"USER",
		"LOGNAME",
		"SHELL",
		"LANG",
		"LC_ALL",
		"TMPDIR",
	}

	// terminationGrace is the duration to wait after sending SIGTERM before SIGKILL.
	terminationGrace = 2 * time.Second
)

// Check runs the project verification suite by executing lucind-checks.sh
// at the root of repoRoot with a scrubbed environment and bounded execution time.
//
// Deprecated: lane checks are now configured per lane with dispatch --check.
//
// If lucind-checks.sh does not exist, Check returns passed = false, an explanatory
// message, and err = nil.
// If the script exits 0, Check returns passed = true, combined output, and err = nil.
// If the script exits non-zero, Check returns passed = false, combined output, and err = nil.
// If the execution times out, SIGTERM and then SIGKILL are sent to the process group,
// and Check returns passed = false, combined output, and err = nil.
// Non-nil err is reserved for setup or start errors.
func Check(ctx context.Context, repoRoot string) (passed bool, output string, err error) {
	if !filepath.IsAbs(repoRoot) {
		return false, "", errors.New("check: repo root must be absolute")
	}

	ownedRoot, err := filepath.EvalSymlinks(repoRoot)
	if err != nil {
		return false, "", fmt.Errorf("check: resolve repo root: %w", err)
	}
	ownedRoot, err = filepath.Abs(ownedRoot)
	if err != nil {
		return false, "", fmt.Errorf("check: canonicalize repo root: %w", err)
	}

	scriptPath := filepath.Join(ownedRoot, "lucind-checks.sh")
	if _, statErr := os.Stat(scriptPath); errors.Is(statErr, os.ErrNotExist) || os.IsNotExist(statErr) {
		return false, MissingScriptOutput, nil
	} else if statErr != nil {
		return false, "", fmt.Errorf("check: stat checks script: %w", statErr)
	}

	env, envErr := scrubEnvironment()
	if envErr != nil {
		return false, "", envErr
	}

	if ctx.Err() != nil {
		return false, "", fmt.Errorf("check: context ended before start: %w", ctx.Err())
	}

	checkCtx := ctx
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok {
		checkCtx, cancel = context.WithTimeout(ctx, DefaultTimeout)
	}
	defer cancel()

	cmd := exec.Command("sh", scriptPath)
	cmd.Dir = ownedRoot
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var out limitedBuffer
	out.max = MaxOutputBytes
	cmd.Stdout = &out
	cmd.Stderr = &out

	if startErr := cmd.Start(); startErr != nil {
		return false, out.String(), fmt.Errorf("check: start checks script: %w", startErr)
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()

	var cmdErr error
	select {
	case cmdErr = <-wait:
	case <-checkCtx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		timer := time.NewTimer(terminationGrace)
		select {
		case <-wait:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-wait
		}
		return false, out.String(), nil
	}

	if cmdErr != nil {
		var exitErr *exec.ExitError
		if errors.As(cmdErr, &exitErr) {
			return false, out.String(), nil
		}
		return false, out.String(), fmt.Errorf("check: wait checks script: %w", cmdErr)
	}

	return true, out.String(), nil
}

// Run executes verification checks against the repository at repoRoot.
// Output is written to stdout when checks pass, or to stderr when checks fail.
// If checks fail (exit non-zero or missing script), ErrCheckFailed is returned.
// Any setup or system errors are returned directly.
func Run(ctx context.Context, repoRoot string, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	passed, out, err := Check(ctx, repoRoot)
	if err != nil {
		return err
	}

	if !passed {
		if out != "" {
			if !strings.HasSuffix(out, "\n") {
				out += "\n"
			}
			_, _ = io.WriteString(stderr, out)
		}
		return ErrCheckFailed
	}

	if out != "" {
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		_, _ = io.WriteString(stdout, out)
	}

	return nil
}

func scrubEnvironment() ([]string, error) {
	pathVal, ok := os.LookupEnv("PATH")
	if !ok || pathVal == "" {
		return nil, errors.New("check: required environment variable PATH is missing or empty")
	}

	env := make([]string, 0, len(SafeEnvKeys))
	for _, key := range SafeEnvKeys {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+val)
		}
	}
	return env, nil
}

type limitedBuffer struct {
	mu        sync.Mutex
	b         bytes.Buffer
	truncated bool
	max       int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	written := len(p)
	remaining := b.max - b.b.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
			b.truncated = true
		}
		_, _ = b.b.Write(p)
	} else {
		b.truncated = true
	}
	return written, nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := b.b.String()
	if b.truncated {
		out += "\n[output truncated]\n"
	}
	return out
}
