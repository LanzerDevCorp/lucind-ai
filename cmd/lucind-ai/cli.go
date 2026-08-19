package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// defaultTimeout is the wall clock the binary owns for one dispatch, absent
// an explicit --timeout. It matches the invocation documented in
// plugin/claude-code/skills/lucind-ai/references/runtime.md, which is the
// authoritative source for a headless agy dispatch's shape.
const defaultTimeout = 20 * time.Minute

// usage is printed on stderr for a missing/unknown subcommand or a usage
// error, so a person driving the binary from a terminal always sees the one
// invocation that works rather than a stack trace.
const usage = "usage: lucind-ai run --packet <path> [--timeout <duration>]"

// supportedExecutors names every packet.Executor value this binary knows
// how to dispatch. Unlisted values are a routing error, never a silent
// fallback to agy — see internal/run's Deps.Executor field.
var supportedExecutors = map[string]func() executor.Executor{
	"agy": func() executor.Executor { return executor.Agy{} },
}

// run parses args, wires internal/run.Deps from the real world, executes
// exactly one packet, and reports the outcome to stdout/stderr. It returns
// a process exit code rather than calling os.Exit itself so it is testable
// in-process.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "run":
		return runDispatch(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runDispatch implements the "run" subcommand: one packet, dispatched
// end-to-end through internal/run.Execute.
func runDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usage)
		fs.PrintDefaults()
	}

	packetPath := fs.String("packet", "", "path to the dispatch packet (required)")
	timeout := fs.Duration("timeout", defaultTimeout, "wall clock budget for this dispatch")

	if err := fs.Parse(args); err != nil {
		// flag.ContinueOnError already invoked fs.Usage() on a parse
		// error; nothing more to print here.
		return 1
	}

	if *packetPath == "" {
		fmt.Fprintln(stderr, "lucind-ai: --packet is required")
		fs.Usage()
		return 1
	}

	f, err := os.Open(*packetPath)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open packet %q: %v\n", *packetPath, err)
		return 1
	}
	defer f.Close()

	p, err := packet.Parse(f)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: parse packet %q: %v\n", *packetPath, err)
		return 1
	}

	newExecutor, ok := supportedExecutors[p.Executor]
	if !ok {
		fmt.Fprintf(stderr, "lucind-ai: unsupported executor %q in packet %q (supported: agy)\n", p.Executor, *packetPath)
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	// A lane's own worktree is not a place to dispatch from: it would
	// nest a second worktree tree inside a linked worktree and put the
	// ledger somewhere other than the primary repository's .lucind/,
	// which internal/ledger.Open assumes is always the case.
	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	runID := uuid.NewString()
	fmt.Fprintf(stdout, "run id: %s\n", runID)

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	deps := lucindrun.Deps{
		RunID:          runID,
		PrimaryRoot:    primaryRoot,
		Ledger:         ledg,
		Executor:       newExecutor(),
		CreateWorktree: worktree.Create,
		WorktreeFS:     os.DirFS,
		Now:            time.Now,
	}

	dispatchCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	report, err := lucindrun.Execute(dispatchCtx, deps, p)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: run: %v\n", err)
		return 1
	}

	printReport(stdout, report)

	// Exit 0 requires both a clean Execute call and a lane that actually
	// reached lane.Done. A blocked/deviated/failed lane is a real,
	// non-crashing outcome (see printReport), but it must never be
	// mistaken for success by anything reading the exit code.
	if report.Status != lane.Done {
		return 1
	}
	return 0
}

// printReport prints a short, human-readable summary of one lane's run.
// A lane that did not reach lane.Done gets a visually unmissable banner so
// a person skimming a terminal cannot mistake it for a successful run.
func printReport(w io.Writer, r lucindrun.Report) {
	fmt.Fprintf(w, "lane:      %s\n", r.LaneID)
	fmt.Fprintf(w, "status:    %s\n", r.Status)
	fmt.Fprintf(w, "worktree:  %s\n", r.Worktree)
	fmt.Fprintf(w, "released:  %t\n", r.Released)
	if r.Envelope != nil {
		fmt.Fprintf(w, "summary:   %s\n", r.Envelope.Summary)
	}

	if r.Status != lane.Done {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "!!! LANE DID NOT COMPLETE (status=%s) — worktree preserved at %s !!!\n", r.Status, r.Worktree)
	}
}

// resolvePrimaryRoot runs "git rev-parse --show-toplevel" in the process's
// working directory and returns its trimmed, absolute output.
// worktree.Create derives every lane's worktree location from this value
// and worktree.Worktree.Path documents itself as absolute, so a relative
// result here is treated as a failure rather than passed downstream.
func resolvePrimaryRoot(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	root := strings.TrimRight(stdout.String(), "\n")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("git rev-parse --show-toplevel returned a non-absolute path: %q", root)
	}
	return root, nil
}
