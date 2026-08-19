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
}

// Outcome is what actually happened when the child process ran, nothing
// more. It intentionally carries no judgment about whether the requested
// work succeeded -- see Status for why exit 0 is not the same as done.
type Outcome struct {
	ExitCode int
	TimedOut bool
	// Stderr is captured for diagnosis when a dispatch fails or times out.
	Stderr string
}

// Executor runs one Request, bounded by ctx.
type Executor interface {
	Run(ctx context.Context, req Request) (Outcome, error)
}
