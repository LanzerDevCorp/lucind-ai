package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// defaultCursorBinary is the name CursorAgent execs when Binary is left empty.
// It relies on PATH lookup, matching how the CLI is normally invoked.
const defaultCursorBinary = "cursor-agent"

// CursorAgent dispatches the cursor-agent CLI headlessly.
type CursorAgent struct {
	// Binary is the executable to run. Defaults to "cursor-agent" when empty,
	// but tests override it to point at a stub script instead of spending
	// real quota against the real CLI.
	Binary string

	// WaitDelay bounds how long Run waits for the child's stdio pipes to
	// drain once the direct child process has exited. Zero (the field's
	// natural default) is replaced with defaultWaitDelay in Run, so
	// production always dispatches with a non-zero value -- see
	// defaultWaitDelay's doc comment for why that value must never be
	// left at exec's own zero-value behavior (wait forever). Tests set
	// this explicitly to something small.
	WaitDelay time.Duration
}

// DefaultModel returns the model CursorAgent runs on when a packet names none.
func (c CursorAgent) DefaultModel() string {
	return "cursor-grok-4.6-high"
}

// KnownModels returns every model a packet may deliberately request for
// CursorAgent. cursor-grok-4.6-high is the default (billed against
// Cursor's included "Cursor Models" quota); claude-opus-4-8-high is a
// deliberately-tested escalation for non-trivial logic (billed against
// Cursor's separate "Other Models" quota -- a real cost tradeoff, not a
// mistake, when named on purpose). Extend this list only for a model
// actually verified against the real cursor-agent CLI -- see
// internal/executor.Executor.KnownModels's doc comment for why this is
// not free text. In particular, never add a gemini- model here: that is
// agy's provider family, and naming one for cursor-agent is exactly the
// copy-paste mistake this method exists to catch.
func (c CursorAgent) KnownModels() []string {
	return []string{"cursor-grok-4.6-high", "claude-opus-4-8-high"}
}

// Run execs cursor-agent with req.Prompt, in req.WorktreePath, bounded by ctx.
//
// The flag set this method always sends -- --print, --output-format json,
// --trust, --force, and --approve-mcps -- is the minimal non-interactive
// invocation verified empirically across both design proposals.
// --trust and --force (-f) in particular are load-bearing: without them a
// headless run stalls on an interactive workspace trust or tool-execution
// prompt and exits with an error or dies on the wall clock.
//
// Unlike agy, cursor-agent exposes no timeout flag and no --json-schema
// equivalent. Wall-clock bounding is enforced exclusively via ctx, and schema
// enforcement is handled by result envelope validation rather than flag-level
// constraints.
func (c CursorAgent) Run(ctx context.Context, req Request) (Outcome, error) {
	binary := c.Binary
	if binary == "" {
		binary = defaultCursorBinary
	}

	args := []string{
		"--print", req.Prompt,
		"--output-format", "json",
		"--trust",
		"--force",
		"--approve-mcps",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.WorktreePath != "" {
		args = append(args, "--workspace", req.WorktreePath)
	}

	waitDelay := c.WaitDelay
	if waitDelay == 0 {
		waitDelay = defaultWaitDelay
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = req.WorktreePath
	cmd.WaitDelay = waitDelay

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()

	outcome := Outcome{Stderr: stderr.String(), Stdout: stdout.String()}

	if ctx.Err() == context.DeadlineExceeded {
		outcome.TimedOut = true
		return outcome, nil
	}

	if errors.Is(err, exec.ErrWaitDelay) {
		// The process itself ran and exited with a successful status --
		// os/exec.Cmd.WaitDelay's doc comment is explicit that
		// ErrWaitDelay replaces a nil Wait error only when "the command
		// has otherwise exited with a successful status". Only its
		// stdio pipes stayed open past waitDelay afterward (typically
		// because a grandchild process inherited them). cmd.ProcessState
		// is populated once the process itself has exited, so it is the
		// reliable source for the real exit code here, not a guess.
		if cmd.ProcessState != nil {
			outcome.ExitCode = cmd.ProcessState.ExitCode()
		}
		outcome.OutputTruncated = true
		return outcome, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		outcome.ExitCode = exitErr.ExitCode()
		return outcome, nil
	}
	if err != nil {
		// Not an ExitError and not ErrWaitDelay: the process never ran
		// at all (binary missing, permission denied, etc). This is a
		// real error, not an Outcome.
		return Outcome{}, err
	}

	outcome.ExitCode = 0
	return outcome, nil
}
