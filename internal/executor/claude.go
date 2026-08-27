package executor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"time"
)

// defaultClaudeBinary is the name Claude execs when Binary is left empty.
// It relies on PATH lookup, matching how the CLI is normally invoked.
const defaultClaudeBinary = "claude"

// Claude dispatches the claude CLI (Claude Code) headlessly via --print.
type Claude struct {
	// Binary is the executable to run. Defaults to "claude" when empty,
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

// DefaultModel returns the model Claude runs on when a packet names none.
func (c Claude) DefaultModel() string {
	return "claude-opus-5"
}

// KnownModels returns every model a packet may deliberately request for
// Claude. claude-opus-5, Claude's own default, is the only entry: this
// executor exists specifically to reach Anthropic's top-tier reasoning
// model for adversarial judgment work, and it must run strictly on it --
// never escalate to another provider's model -- so provider identity and
// billing stay unambiguous. The full model name is used rather than the
// CLI's short "opus" alias on purpose: the alias silently re-points at
// whatever the latest Opus is, which would make a dispatch's actual billed
// model unreproducible from the packet alone. Extend this list only for a
// model actually verified against the real claude CLI, and never with
// another provider's model -- see internal/executor.Executor.KnownModels's
// doc comment for why this is not free text.
func (c Claude) KnownModels() []string {
	return []string{"claude-opus-5"}
}

// Run execs claude with req.Prompt, in req.WorktreePath, bounded by ctx.
//
// claude's --print is a boolean ("print response and exit"), not a flag
// that carries the prompt the way agy's --print does. The prompt is
// therefore a positional argument, and it is always sent behind a "--"
// separator. That separator is load-bearing, not defensive style: a packet
// body is Markdown and may legitimately begin with "-" (a bullet list),
// and claude rejects unknown options outright rather than ignoring them,
// so a bare positional would make such a packet exit 1 before dispatching.
// Verified against the real CLI: `claude --definitely-not-a-flag` answers
// "error: unknown option", while the same string behind "--" is accepted
// as the prompt.
//
// --permission-mode acceptEdits is claude's spelling of agy's --mode
// accept-edits, and --dangerously-skip-permissions is spelled identically
// on both CLIs. Both are sent unconditionally for the same reason agy sends
// them: without them a headless run stalls on an interactive permission
// prompt and the lane dies on the wall clock for no real failure. See
// Agy.Run's doc comment for the full worktree-boundary rationale for why
// that cost is accepted; it applies identically here.
//
// Unlike agy, claude exposes no --print-timeout (verified against `claude
// --help`), so ctx is the only clock -- exactly as for cursor-agent and
// opencode. Sending agy's flag here would not merely be redundant: claude
// rejects unknown options, so every dispatch would exit 1 without running.
//
// Request.Agent is deliberately ignored even though claude does expose an
// --agent flag. cmd/lucind-ai/cli.go's pre-dispatch validation rejects a
// packet-declared agent on any non-opencode executor, so honoring it here
// would create a flag that no packet can ever legally reach. Wiring it is a
// separate, deliberate change to that validation, not something to smuggle
// in with a new executor.
//
// When req.Progress is set the dispatch switches to --output-format
// stream-json so lane progress is persisted to the ledger incrementally as
// it happens, rather than only once at exit. That switch drags one
// mandatory companion flag with it: claude refuses
// stream-json without --verbose outright rather than degrading. Verified in
// the real CLI (2.1.241), which guards the pair with `if (outputFormat ===
// "stream-json" && !verbose)`, prints `Error: When using --print,
// --output-format=stream-json requires --verbose` and exits 1 without ever
// dispatching. Same failure family as --print-timeout, opposite direction:
// there a flag must never be sent, here one must never be omitted.
//
// Unlike Agy.Run, a stream that carries no terminal result record is NOT
// replayed as a blocking JSON run. That replay is affordable for agy and is
// not affordable here: this executor is pinned to Opus, so a replay bills a
// second full run, and the first run's edits are already on disk -- the
// replay would re-apply work on top of itself. Nothing downstream needs the
// replay either: Outcome.Stdout is diagnosis-only (internal/run/run.go
// reads the real envelope from the worktree's .lucind/result.json), and the
// complete raw stream is captured in Stdout regardless.
func (c Claude) Run(ctx context.Context, req Request) (Outcome, error) {
	if req.Progress == nil {
		return c.runFormat(ctx, req, "json", nil)
	}

	decoder := newClaudeStreamDecoder(req.Progress)
	outcome, err := c.runFormat(ctx, req, "stream-json", decoder)
	decoder.finish()
	if err != nil {
		return Outcome{}, err
	}
	if decoder.terminal {
		// The terminal record is byte-identical to what --output-format json
		// would have printed, so a progress dispatch's Stdout reads exactly
		// like a blocking one's. Without it, Stdout stays the raw stream.
		outcome.Stdout = string(decoder.result)
	}
	return outcome, nil
}

func (c Claude) runFormat(ctx context.Context, req Request, format string, stdoutTap io.Writer) (Outcome, error) {
	binary := c.Binary
	if binary == "" {
		binary = defaultClaudeBinary
	}

	args := []string{
		"--print",
		"--output-format", format,
		"--permission-mode", "acceptEdits",
		"--dangerously-skip-permissions",
	}
	if format == "stream-json" {
		args = append(args, "--verbose")
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
	// The separator and the prompt stay last, in this order: everything
	// after "--" is positional, so no flag may be appended past this point.
	args = append(args, "--", req.Prompt)

	waitDelay := c.WaitDelay
	if waitDelay == 0 {
		waitDelay = defaultWaitDelay
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = req.WorktreePath
	cmd.WaitDelay = waitDelay

	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	if stdoutTap == nil {
		cmd.Stdout = &stdout
	} else {
		cmd.Stdout = io.MultiWriter(&stdout, stdoutTap)
	}

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
		// because a grandchild process, such as an MCP server claude
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
