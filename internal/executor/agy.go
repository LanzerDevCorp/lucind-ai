package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// defaultBinary is the name Agy execs when Binary is left empty. It relies
// on PATH lookup, matching how the CLI is normally invoked.
const defaultBinary = "agy"

// waitDelay bounds how long Run waits for the child's stdio pipes to drain
// after the context deadline kills the direct child process. Without this,
// a grandchild process the child spawned (e.g. a shell script's own "sleep"
// or similar) can keep holding the inherited stderr pipe open and Wait
// would block until that grandchild exits on its own, defeating the whole
// point of the context deadline. See os/exec.Cmd.WaitDelay.
const waitDelay = 250 * time.Millisecond

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
}

// Run execs agy with req.Prompt, in req.WorktreePath, bounded by ctx.
//
// agy exposes no --cwd flag, so the worktree is selected by setting the
// child process's working directory directly rather than by a flag.
//
// The flag set this method always sends -- --output-format json, --mode
// accept-edits, and --dangerously-skip-permissions -- is not a stylistic
// choice: it is the non-interactive invocation documented in
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
// it opt-in per packet was considered and rejected in favour of dispatch that
// never stalls. If that tradeoff is ever revisited, the natural home for the
// switch is the packet frontmatter, so the authorization travels with the
// work it authorizes.
func (a Agy) Run(ctx context.Context, req Request) (Outcome, error) {
	binary := a.Binary
	if binary == "" {
		binary = defaultBinary
	}

	args := []string{
		"--print", req.Prompt,
		"--output-format", "json",
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

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		outcome.ExitCode = exitErr.ExitCode()
		return outcome, nil
	}
	if err != nil {
		// Not an ExitError: the process never ran at all (binary missing,
		// permission denied, etc). This is a real error, not an Outcome.
		return Outcome{}, err
	}

	outcome.ExitCode = 0
	return outcome, nil
}
