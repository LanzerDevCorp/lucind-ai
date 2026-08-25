package executor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// defaultBinary is the name Agy execs when Binary is left empty. It relies
// on PATH lookup, matching how the CLI is normally invoked.
const defaultBinary = "agy"

// defaultWaitDelay is the production value applied when Agy.WaitDelay is
// left zero. It bounds how long Run waits for the child's stdio pipes to
// drain after the direct child process has exited (or the context deadline
// killed it), before force-closing them. Without a non-zero WaitDelay, a
// grandchild process the child spawned -- real agy dispatches spawn MCP
// server subprocesses that inherit stdout/stderr this way -- can keep
// holding those pipes open indefinitely, and Wait would block until that
// grandchild exits on its own, defeating the whole point of the caller's
// context deadline. See os/exec.Cmd.WaitDelay.
//
// 5 seconds is a tradeoff, not a measured constant: long enough that a
// well-behaved grandchild's own shutdown (flushing output, closing its
// pipes) has a realistic chance to finish before Run gives up on capturing
// it, short enough that a grandchild which never exits on its own does not
// meaningfully delay how quickly a lane's result becomes available. Getting
// this exactly right is inherently a guess about real-world MCP server
// shutdown latency; the failure mode on either side is bounded and
// asymmetric with what it protects: too short only costs a truncated
// capture (see Outcome.OutputTruncated) with the exit code still correct,
// never a lost or misreported result, while too long only delays surfacing
// a dispatch whose grandchild is stuck. Tests must set Agy.WaitDelay
// explicitly to something small rather than inheriting this value, so a
// stub that intentionally leaves a grandchild alive doesn't slow the suite
// down.
const defaultWaitDelay = 5 * time.Second

// printTimeoutMargin is added on top of the context's remaining time so
// that the value passed as --print-timeout is always strictly greater than
// the context deadline. agy has no way to know the caller's context, so
// this margin is what keeps the Go side authoritative: agy's own
// --print-timeout must never be able to fire before ctx does.
const printTimeoutMargin = 1 * time.Minute

// printTimeoutFor computes the duration to pass as --print-timeout so it
// is strictly greater than ctx's remaining time until its deadline. It
// reports false when ctx carries no deadline at all -- in that case there
// is no context-side bound for agy's own timeout to undercut, so Run omits
// the --print-timeout flag entirely and lets agy fall back to its own
// default.
func printTimeoutFor(ctx context.Context) (time.Duration, bool) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}

	remaining := time.Until(deadline)
	if remaining < 0 {
		remaining = 0
	}
	return remaining + printTimeoutMargin, true
}

// Agy is the real Executor: it dispatches the agy CLI headlessly.
type Agy struct {
	// Binary is the executable to run. Defaults to "agy" when empty, but
	// tests override it to point at a stub script instead of spending
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

// DefaultModel returns the model Agy runs on when a packet names none.
func (a Agy) DefaultModel() string {
	return "gemini-3.7-flash-high"
}

// KnownModels returns every model a packet may deliberately request for
// Agy. Extend this list only for a model actually verified against the
// real agy CLI -- see internal/executor.Executor.KnownModels's doc comment
// for why this is not free text.
func (a Agy) KnownModels() []string {
	return []string{"gemini-3.7-flash-high"}
}

// Run execs agy with req.Prompt, in req.WorktreePath, bounded by ctx.
//
// agy exposes no --cwd flag, so the worktree is selected by setting the
// child process's working directory directly rather than by a flag.
//
// The flag set this method sends --mode accept-edits and --dangerously-skip-
// permissions on both the blocking JSON path and the optional stream-json
// progress path. This is not a stylistic choice: it is the non-interactive invocation documented in
// plugin/claude-code/skills/lucind-ai/references/runtime.md (see also
// docs/prd.md section 6, step 4), which is this project's authoritative
// source for which flags a headless agy dispatch requires and why.
// --dangerously-skip-permissions in particular is load-bearing: without it
// a headless run stalls on an interactive permission prompt and the lane
// dies on the wall clock for no reason, not because of any real failure.
// Anyone changing this flag set should update runtime.md first, then this
// method to match -- not the other way around.
//
// Sending it unconditionally is a deliberate decision with a real cost, and
// it is recorded here so it stays visible. A git worktree is not a sandbox:
// it shares the filesystem, the credentials and the network with the primary
// repository. With this flag the dispatched agent auto-approves every tool
// call it makes, including calls that reach outside its own worktree. Making
// it opt-in per packet was considered and rejected twice in favour of
// dispatch that never stalls. If that tradeoff is ever revisited, the natural
// home for the switch is the packet frontmatter, so the authorization travels
// with the work it authorizes.
//
// Neither cmd.Dir nor --add-dir bounds that authorization. This is observed,
// not inferred: on 2026-08-19 a dispatch whose working directory was an empty
// scratch directory, with --add-dir pointing at that same directory, followed
// the paths written in its packet instead. It created a git worktree in the
// real repository, checked out a branch, edited a file and committed. It kept
// faithfully to the allowed_paths its packet named -- the packet's prose was
// obeyed exactly -- but the process-level boundary was not a boundary at all.
//
// So the scope of a dispatched lane is bounded by the text of its packet and
// by nothing else. Write allowed_paths as the only fence there is, because it
// is.
func (a Agy) Run(ctx context.Context, req Request) (Outcome, error) {
	if req.Progress == nil {
		return a.runFormat(ctx, req, "json", nil)
	}

	decoder := newAgyStreamDecoder(req.Progress)
	outcome, err := a.runFormat(ctx, req, "stream-json", decoder)
	decoder.finish()
	if err != nil {
		return Outcome{}, err
	}
	if decoder.terminal {
		// The terminal result is the stream-json equivalent of blocking JSON's
		// stdout. Returning only its payload keeps Outcome semantics stable.
		outcome.Stdout = string(decoder.result)
		return outcome, nil
	}
	if outcome.TimedOut || outcome.OutputTruncated {
		// The turn may have completed while output was cut off. Never replay it
		// when the terminal record's absence is not trustworthy.
		return outcome, nil
	}

	emitAgyProgress(req.Progress, agyStreamFallbackMessage)
	return a.runFormat(ctx, req, "json", nil)
}

func (a Agy) runFormat(ctx context.Context, req Request, format string, stdoutTap io.Writer) (Outcome, error) {
	binary := a.Binary
	if binary == "" {
		binary = defaultBinary
	}

	args := []string{
		"--print", req.Prompt,
		"--output-format", format,
		"--mode", "accept-edits",
		"--dangerously-skip-permissions",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.SchemaPath != "" {
		args = append(args, "--json-schema", req.SchemaPath)
	}
	if req.WorktreePath != "" {
		args = append(args, "--add-dir", req.WorktreePath)
	}
	if pt, ok := printTimeoutFor(ctx); ok {
		args = append(args, "--print-timeout", pt.String())
	}

	waitDelay := a.WaitDelay
	if waitDelay == 0 {
		waitDelay = defaultWaitDelay
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = req.WorktreePath
	cmd.WaitDelay = waitDelay
	if req.Setpgid {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process == nil {
				return nil
			}
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	if stdoutTap == nil {
		cmd.Stdout = &stdout
	} else {
		cmd.Stdout = io.MultiWriter(&stdout, stdoutTap)
	}

	err := cmd.Start()
	if err == nil {
		if req.Setpgid && req.OnStart != nil && cmd.Process != nil {
			req.OnStart(cmd.Process.Pid)
		}
		err = cmd.Wait()
	}

	outcome := Outcome{Stderr: stderr.String(), Stdout: stdout.String()}
	if req.Setpgid && cmd.Process != nil {
		outcome.PGID = cmd.Process.Pid
	}

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
		// because a grandchild process, such as an MCP server agy
		// spawned, inherited them). cmd.ProcessState is populated once
		// the process itself has exited, so it is the reliable source
		// for the real exit code here, not a guess.
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
