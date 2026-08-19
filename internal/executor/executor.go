// Package executor dispatches a coding-agent CLI headlessly into a git
// worktree, bounded by a wall clock the caller supplies via context. The
// target CLI (agy) exposes its own --print-timeout, but that flag alone is
// not trustworthy: it is not always present (cursor-agent has no timeout
// flag at all), and even when present it defaults far above what a bounded
// dispatch needs. The binary therefore owns the clock: every Run call is
// bounded by ctx, and when a child CLI does expose its own timeout flag,
// executor sets it strictly above the context deadline so the Go side is
// always the one that decides, never the child process.
package executor

import "context"

// Request is one dispatch: a prompt to run, in a worktree, on a model.
type Request struct {
	// Prompt is the packet body, passed to the agent verbatim.
	Prompt string
	// WorktreePath becomes the child process's working directory. There is
	// no --cwd flag on the target CLI, so this is the only way to select
	// which worktree it operates on.
	WorktreePath string
	// Model is optional; the flag is omitted entirely when this is empty.
	Model string
	// SchemaPath is the path to the result schema on disk; --json-schema is
	// omitted when empty (cursor-agent cannot be constrained at the
	// source, so this is a belt, not the braces).
	SchemaPath string
}

// Outcome is what actually happened when the child process ran, nothing
// more. It intentionally carries no judgment about whether the requested
// work succeeded -- see Status for why exit 0 is not the same as done.
type Outcome struct {
	ExitCode int
	TimedOut bool
	// Stderr is captured for diagnosis when a dispatch fails or times out.
	Stderr string
	// Stdout is captured for diagnosis too: this binary has never
	// completed a real dispatch yet, and a failed first run with no
	// captured stdout is undiagnosable. This is not speculative --
	// it is the minimum needed to debug the first real invocation.
	Stdout string
	// OutputTruncated is true when the child process itself ran to
	// completion but its stdout/stderr pipes did not finish draining
	// within the executor's WaitDelay afterward -- typically because a
	// grandchild process it spawned (e.g. an MCP server subprocess agy
	// starts) inherited those pipes and kept them open past the parent's
	// own exit. This is a warning about the completeness of Stdout/Stderr
	// capture only. It says nothing about whether the dispatched work
	// succeeded: ExitCode still reflects the process's real, observed
	// exit status, and a truncated capture must never be treated as a
	// failed or discarded run.
	OutputTruncated bool
}

// Executor runs one Request, bounded by ctx.
type Executor interface {
	Run(ctx context.Context, req Request) (Outcome, error)
	// DefaultModel returns the model this executor runs on when a packet
	// names none. Each executor owns its own default -- there is no
	// project-wide fallback.
	DefaultModel() string
}
