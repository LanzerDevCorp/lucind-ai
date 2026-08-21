package executor

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// defaultOpencodeBinary is the name Opencode execs when Binary is left empty.
// It relies on PATH lookup, matching how the CLI is normally invoked.
const defaultOpencodeBinary = "opencode"

// Opencode dispatches the opencode CLI headlessly via its "run" subcommand.
type Opencode struct {
	// Binary is the executable to run. Defaults to "opencode" when empty,
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

// DefaultModel returns the model Opencode runs on when a packet names none.
func (o Opencode) DefaultModel() string {
	return "openai/gpt-5.6-sol"
}

// KnownModels returns every model a packet may deliberately request for
// Opencode. openai/gpt-5.6-sol, Opencode's own default, is the only entry:
// this executor exists specifically to reach that model, and it must run
// strictly on it -- never escalate to another provider's model -- so
// provider identity and billing stay unambiguous. Extend this list only for
// a model actually verified against the real opencode CLI, and never with
// another provider's model -- see internal/executor.Executor.KnownModels's
// doc comment for why this is not free text.
func (o Opencode) KnownModels() []string {
	return []string{"openai/gpt-5.6-sol"}
}

// Run execs opencode with req.Prompt, in req.WorktreePath, bounded by ctx.
//
// opencode's "run" subcommand takes the prompt as a positional argument,
// not behind a flag like agy's --print or cursor-agent's --print. The flag
// set this method always sends -- --format json and --auto -- is the
// minimal non-interactive invocation verified against the real opencode CLI
// (`opencode run --help`, v1.18.19): --auto is opencode's own label for
// "auto-approve permissions that are not explicitly denied (dangerous!)",
// the headless equivalent of agy's --dangerously-skip-permissions and
// cursor-agent's --trust/--force/--approve-mcps -- without it a headless run
// stalls on an interactive permission prompt. See Agy.Run's doc comment for
// the full worktree-boundary rationale for why this flag is still sent
// unconditionally despite that cost; it applies identically here.
//
// Agent is optional and passed through verbatim as --agent when set (e.g.
// the `lucind-dag` agent, purpose-built for authoring apply-dag.yaml
// sidecars -- see `opencode agent list`). Unlike Model, there is no
// known-agent allow-list here: opencode itself validates the agent name at
// dispatch time, and an unknown name is opencode's own error to surface,
// not a cross-provider billing risk the way a wrong Model would be.
//
// Like cursor-agent, opencode exposes no timeout flag and no --json-schema
// equivalent. Wall-clock bounding is enforced exclusively via ctx, and
// schema enforcement is handled by result envelope validation rather than
// flag-level constraints.
func (o Opencode) Run(ctx context.Context, req Request) (Outcome, error) {
	binary := o.Binary
	if binary == "" {
		binary = defaultOpencodeBinary
	}

	args := []string{
		req.Prompt,
		"--format", "json",
		"--auto",
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.Agent != "" {
		args = append(args, "--agent", req.Agent)
	}
	if req.WorktreePath != "" {
		args = append(args, "--dir", req.WorktreePath)
	}

	waitDelay := o.WaitDelay
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
