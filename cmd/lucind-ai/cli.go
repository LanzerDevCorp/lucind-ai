package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LanzerDevCorp/lucind-ai/internal/dag"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/serve"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// defaultTimeout is the wall clock the binary grants each lane, absent an
// explicit --timeout. It matches the invocation documented in
// plugin/claude-code/skills/lucind-ai/references/runtime.md, which is the
// authoritative source for a headless agy dispatch's shape. It is applied
// per lane (via run.Deps.LaneTimeout, which run.ExecuteBatch derives each
// lane's own deadline from independently), never once for the whole batch
// -- see run.ExecuteBatch's doc comment for why.
const defaultTimeout = 20 * time.Minute

// usage is printed on stderr for a missing/unknown subcommand or a usage
// error, so a person driving the binary from a terminal always sees the one
// invocation that works rather than a stack trace. --packet is repeatable:
// each occurrence adds one more lane to the batch.
const usage = "usage: lucind-ai run --packet <path> [--packet <path> ...] [--timeout <duration>] [--approval-timeout <duration>] [--legacy-main] [--expected-parent-sha <sha>]\n       lucind-ai split --dag <path> --out <dir>\n       lucind-ai check [--out <path>]\n       lucind-ai serve [--addr <addr>] [--approver <name>] [--approval-timeout <duration>]\n       lucind-ai feature create --id <id> --parent <ref> --base-sha <sha> [--expected-parent-sha <sha>]\n       lucind-ai feature status [--id <id>]\n       lucind-ai feature recover --attempt <id>\n       lucind-ai reconcile approve --request <id> --source <feature> --target <feature> [--actor <name>]\n       lucind-ai reconcile decline --request <id> [--actor <name>] [--reason <reason>]\n       lucind-ai reconcile cancel --request <id> [--actor <name>] [--reason <reason>]\n       lucind-ai reconcile renew --request <id> [--base-sha <sha>] [--source-sha <sha>] [--target-sha <sha>]\n       lucind-ai --version"

// depsFactory constructs run.Deps for runDispatch. In production it is
// productionDeps; tests may override it to inject test doubles or observe dependency calls.
var depsFactory = productionDeps

// supportedExecutors names every packet.Executor value this binary knows
// how to dispatch. Unlisted values are a routing error, never a silent
// fallback to agy — see internal/run's Deps.LookupExecutor field.
var supportedExecutors = map[string]func() executor.Executor{
	"agy":          func() executor.Executor { return executor.Agy{} },
	"cursor-agent": func() executor.Executor { return executor.CursorAgent{} },
	"opencode":     func() executor.Executor { return executor.Opencode{} },
}

// packetPaths collects every --packet flag value, in the order given, so a
// single invocation can name a batch of lanes rather than just one. A
// single --packet keeps working exactly as before: len(packetPaths) == 1
// is simply the one-lane case of the same flow.
type packetPaths []string

// String satisfies flag.Value. It is never used to reconstruct a working
// command line (flag.Value's String is mainly for printing defaults), so a
// simple comma join is enough.
func (p *packetPaths) String() string {
	if p == nil {
		return ""
	}
	return strings.Join(*p, ",")
}

// Set satisfies flag.Value and is what makes --packet repeatable: the flag
// package calls Set once per occurrence instead of overwriting a single
// value.
func (p *packetPaths) Set(v string) error {
	*p = append(*p, v)
	return nil
}

// run parses args, wires internal/run.Deps from the real world, dispatches
// every named packet as its own lane through internal/run.ExecuteBatch, and
// reports the outcome to stdout/stderr. It returns a process exit code
// rather than calling os.Exit itself so it is testable in-process.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "run":
		return runDispatch(ctx, args[1:], stdout, stderr)
	case "split":
		return runSplit(ctx, args[1:], stdout, stderr)
	case "check":
		return runCheck(ctx, args[1:], stdout, stderr)
	case "serve":
		return serveDispatch(ctx, args[1:], stdout, stderr)
	case "feature":
		return featureDispatch(ctx, args[1:], stdout, stderr)
	case "reconcile":
		return reconcileDispatch(ctx, args[1:], stdout, stderr)
	case "renew":
		return runReconcileRenew(ctx, args[1:], stdout, stderr)
	case "--version", "-v":
		fmt.Fprintf(stdout, "lucind-ai %s (%s, %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return 0
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runDispatch implements the "run" subcommand: every --packet becomes one
// lane, and every lane in the batch is dispatched end to end through
// internal/run.ExecuteBatch.
func runDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usage)
		fs.PrintDefaults()
	}

	var packetFlags packetPaths
	fs.Var(&packetFlags, "packet", "path to a dispatch packet (repeatable: one lane per occurrence)")
	timeout := fs.Duration("timeout", defaultTimeout, "wall clock budget granted to each lane")
	approvalTimeout := fs.Duration("approval-timeout", 0, "approval timeout budget granted to lane gates (0 = no wait / bypass)")
	legacyMain := fs.Bool("legacy-main", false, "declare legacy mode (dispatches against main)")
	expectedParentSHA := fs.String("expected-parent-sha", "", "expected parent commit SHA for legacy mode")

	if err := fs.Parse(args); err != nil {
		// flag.ContinueOnError already invoked fs.Usage() on a parse
		// error; nothing more to print here.
		return 1
	}

	if len(packetFlags) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: --packet is required")
		fs.Usage()
		return 1
	}

	ps := make([]packet.Packet, 0, len(packetFlags))
	for _, path := range packetFlags {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: open packet %q: %v\n", path, err)
			return 1
		}
		p, err := packet.Parse(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: parse packet %q: %v\n", path, err)
			return 1
		}
		ps = append(ps, p)
	}

	if *legacyMain && *expectedParentSHA == "" {
		for i, p := range ps {
			if p.ExpectedParentSHA == "" {
				fmt.Fprintf(stderr, "lucind-ai: packet %q in legacy mode requires --expected-parent-sha or frontmatter expected_parent_sha\n", packetFlags[i])
				return 1
			}
		}
	}

	for i := range ps {
		if *legacyMain {
			ps[i].LegacyMain = true
		}
		if *expectedParentSHA != "" && ps[i].ExpectedParentSHA == "" {
			ps[i].ExpectedParentSHA = *expectedParentSHA
		}
	}

	// Every packet in this batch must name a supported executor --
	// checked for every packet before any of them dispatches.
	for i, p := range ps {
		if _, ok := supportedExecutors[p.Executor]; !ok {
			names := make([]string, 0, len(supportedExecutors))
			for name := range supportedExecutors {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Fprintf(stderr, "lucind-ai: unsupported executor %q in packet %q (supported: %s)\n", p.Executor, packetFlags[i], strings.Join(names, ", "))
			return 1
		}
	}

	// A named agent is only meaningful for the opencode executor -- checked
	// for every packet before any of them dispatches, exactly like the
	// executor-support and model checks above. Other executors ignore
	// Request.Agent silently at the Run level, but rejecting it here catches
	// a packet author's mistake (or copy-paste from an opencode packet)
	// before it dispatches instead of it just being a no-op.
	for i, p := range ps {
		if p.Agent == "" {
			continue
		}
		if p.Executor != "opencode" {
			fmt.Fprintf(stderr, "lucind-ai: packet %q names agent %q, but agent is only meaningful for executor \"opencode\" (got executor %q)\n", packetFlags[i], p.Agent, p.Executor)
			return 1
		}
	}

	// A named model must be one this executor actually knows -- checked
	// for every packet before any of them dispatches, exactly like the
	// executor-support check above. This is what stops a copy-pasted or
	// mistaken model string from a different provider family (e.g. a
	// gemini- model named for cursor-agent) from silently running -- and
	// billing -- as if it belonged to that executor. An omitted model is
	// always fine: the executor supplies its own DefaultModel.
	for i, p := range ps {
		if p.Model == "" {
			continue
		}
		factory := supportedExecutors[p.Executor] // already validated to exist above
		known := factory().KnownModels()
		ok := false
		for _, m := range known {
			if m == p.Model {
				ok = true
				break
			}
		}
		if !ok {
			fmt.Fprintf(stderr, "lucind-ai: packet %q names model %q, not a known model for executor %q (known: %s)\n", packetFlags[i], p.Model, p.Executor, strings.Join(known, ", "))
			return 1
		}
	}

	// Upfront batch-disjointness check: must stay before ExecuteBatch and
	// worktree.Create so Create is not the first overlap-failure side effect.
	if err := packet.DisjointAllowedPaths(ps); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
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

	deps := depsFactory(runID, primaryRoot, ledg, *timeout, *approvalTimeout)

	// N lanes is N simultaneous subscription quota burns. The forecast
	// only prints for an actual batch (more than one lane): a single
	// packet is not a quota-multiplying decision worth flagging.
	if len(ps) > 1 {
		fmt.Fprintf(stdout, "about to dispatch %d lanes concurrently -- each lane burns subscription quota concurrently (%d simultaneous quota burns)\n", len(ps), len(ps))
	}

	// ctx itself carries no deadline here: run.ExecuteBatch derives each
	// lane's own deadline independently from deps.LaneTimeout, so a slow
	// lane never consumes another lane's clock.
	batch, err := lucindrun.ExecuteBatch(ctx, deps, ps)
	if err != nil {
		// internal/run.ExecuteBatch's own errors already start with
		// "run: ", so no second "run: " prefix is added here -- otherwise
		// a user sees a doubled "lucind-ai: run: run: ..." on stderr.
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	integrateReport, err := lucindrun.Integrate(ctx, deps, batch)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	integrated := make(map[string]bool, len(integrateReport.Integrated))
	for _, id := range integrateReport.Integrated {
		integrated[id] = true
	}

	for _, r := range batch.Lanes {
		printReport(stdout, r)
		if integrated[r.LaneID] {
			fmt.Fprintf(stdout, "envelope:  .lucind/results/%s.json\n", r.LaneID)
		}
	}
	fmt.Fprintf(stdout, "released:  %t\n", batch.Released)

	printIntegrateReport(stdout, integrateReport)

	reverted := make(map[string]bool, len(integrateReport.Reverted))
	for _, id := range integrateReport.Reverted {
		reverted[id] = true
	}

	// Exit 0 requires every lane to have actually reached lane.Done and not
	// have been reverted by integration. A blocked/deviated/failed/reverted lane
	// is a real, non-crashing outcome (see printReport), but it must never be
	// mistaken for success by anything reading the exit code.
	for _, r := range batch.Lanes {
		if r.Status != lane.Done || reverted[r.LaneID] {
			return 1
		}
	}
	return 0
}

// runSplit implements the "split" subcommand: parses an apply-dag.yaml sidecar,
// validates DAG structure and disjointness, emits generated packet files under --out,
// and prints copy-pasteable wave commands to stdout in dependency order.
func runSplit(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("split", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usage)
		fs.PrintDefaults()
	}

	dagPath := fs.String("dag", "", "path to apply-dag.yaml sidecar")
	outDir := fs.String("out", "", "output directory for emitted packet markdown files")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *dagPath == "" {
		fmt.Fprintln(stderr, "lucind-ai: --dag is required")
		fs.Usage()
		return 1
	}
	if *outDir == "" {
		fmt.Fprintln(stderr, "lucind-ai: --out is required")
		fs.Usage()
		return 1
	}

	if err := dag.Split(*dagPath, *outDir, stdout); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}
	return 0
}

// runCheck implements the "check" subcommand: executes lucind-checks.sh
// via internal/integrate.Check deterministically and reports results.
func runCheck(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai check [--out <path>]")
		fs.PrintDefaults()
	}

	outPath := fs.String("out", "", "path to write execution record")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	root, err := resolvePrimaryRoot(ctx)
	if err != nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			root = wd
		} else {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
	}

	start := time.Now()
	passed, checkOutput, checkErr := integrate.Check(ctx, root)
	duration := time.Since(start)
	if checkErr != nil {
		fmt.Fprintf(stderr, "lucind-ai: check: %v\n", checkErr)
		return 1
	}

	commitSHA := resolveCommitSHA(ctx, root)

	exitCode := 0
	if !passed {
		exitCode = 1
	}

	if *outPath != "" {
		content := formatMechanicalLog(commitSHA, exitCode, duration, checkOutput)
		if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: create log directory: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*outPath, []byte(content), 0o644); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: write log file: %v\n", err)
			return 1
		}
	}

	if !passed {
		fmt.Fprintln(stderr, strings.TrimRight(checkOutput, "\n"))
		return 1
	}

	fmt.Fprint(stdout, checkOutput)
	if !strings.HasSuffix(checkOutput, "\n") {
		fmt.Fprintln(stdout)
	}
	fmt.Fprintf(stdout, "status:   passed\nduration: %v\ncommit:   %s\n", duration, commitSHA)

	return 0
}

// formatMechanicalLog formats the structured metadata header followed by the
// check transcript for verify-mechanical.log.
func formatMechanicalLog(commitSHA string, exitCode int, duration time.Duration, output string) string {
	var sb strings.Builder
	sb.WriteString("=== lucind-ai mechanical check ===\n")
	sb.WriteString(fmt.Sprintf("Git Commit SHA: %s\n", commitSHA))
	sb.WriteString("Command: lucind-checks.sh\n")
	sb.WriteString(fmt.Sprintf("Duration: %v\n", duration))
	sb.WriteString(fmt.Sprintf("Exit Code: %d\n", exitCode))
	sb.WriteString("==================================\n")
	sb.WriteString(output)
	return sb.String()
}

// resolveCommitSHA runs "git rev-parse HEAD" in dir and returns the trimmed commit SHA.
func resolveCommitSHA(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// printReport prints a short, human-readable summary of one lane's run.
// A lane that did not reach lane.Done gets a visually unmissable banner so
// a person skimming a terminal cannot mistake it for a successful run. It
// does not print the batch's barrier release: that is a property of the
// whole batch, not of any one lane, so runDispatch prints it once, after
// every lane's report.
func printReport(w io.Writer, r lucindrun.Report) {
	fmt.Fprintf(w, "lane:      %s\n", r.LaneID)
	fmt.Fprintf(w, "status:    %s\n", r.Status)
	fmt.Fprintf(w, "worktree:  %s\n", r.Worktree)
	if r.Envelope != nil {
		fmt.Fprintf(w, "summary:   %s\n", r.Envelope.Summary)
	}
	if r.OutputCaptureIncomplete {
		fmt.Fprintln(w, "note:      captured output may be incomplete (dispatch pipes did not fully drain); this does not by itself mean the lane failed")
	}

	if r.Status != lane.Done {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "!!! LANE DID NOT COMPLETE (status=%s) — worktree preserved at %s !!!\n", r.Status, r.Worktree)

		// r.Diagnosis is empty for a lane whose terminal status came from
		// a readable envelope (see run.Report.Diagnosis's doc comment),
		// so nothing is printed here in that case -- only the three
		// non-success dispatch paths (non-zero exit, timeout, unreadable
		// envelope) ever populate it. Printed under the banner, never
		// above it: the banner is the headline, this is the detail a
		// person reaches for next.
		if r.Diagnosis != "" {
			fmt.Fprintln(w, "--- captured diagnosis ---")
			fmt.Fprintln(w, r.Diagnosis)
			fmt.Fprintln(w, "---------------------------")
		}
	}
}

// printIntegrateReport prints the integration outcome summary, including
// the integrate count line, integrated_ids, and reverted_ids.
func printIntegrateReport(w io.Writer, rep lucindrun.IntegrateReport) {
	if rep.Reason != "" {
		fmt.Fprintf(w, "integrate: attempted=%t passed=%t integrated=%d reverted=%d reason=%s\n",
			rep.Attempted, rep.Passed, len(rep.Integrated), len(rep.Reverted), rep.Reason)
	} else {
		fmt.Fprintf(w, "integrate: attempted=%t passed=%t integrated=%d reverted=%d\n",
			rep.Attempted, rep.Passed, len(rep.Integrated), len(rep.Reverted))
	}
	printIDList(w, "integrated_ids", rep.Integrated)
	printIDList(w, "reverted_ids", rep.Reverted)
}

// printIDList formats an ID list on a single line. When ids is empty, it
// prints an explicitly empty list ("<label>:\n") rather than omitting the line.
func printIDList(w io.Writer, label string, ids []string) {
	if len(ids) == 0 {
		fmt.Fprintf(w, "%s:\n", label)
		return
	}
	fmt.Fprintf(w, "%s: %s\n", label, strings.Join(ids, " "))
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

// productionDeps constructs the production run.Deps wiring real-world
// dependencies (git, ledger, worktree, executors, clock).
func productionDeps(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
	return lucindrun.Deps{
		RunID:       runID,
		PrimaryRoot: primaryRoot,
		Ledger:      ledg,
		LookupExecutor: func(name string) (executor.Executor, error) {
			factory, ok := supportedExecutors[name]
			if !ok {
				return nil, fmt.Errorf("unsupported executor %q", name)
			}
			return factory(), nil
		},
		CreateWorktree:  worktree.Create,
		WorktreeFS:      os.DirFS,
		Now:             time.Now,
		LaneTimeout:     timeout,
		ApprovalTimeout: approvalTimeout,
		HasUniqueLaneCommits: func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return worktree.HasUniqueCommits(ctx, worktreePath, baseSHA)
		},
		PorcelainEmpty: worktree.PorcelainEmpty,
		CombineTree:    integrate.Combine,
		RunChecks:      integrate.Check,
		PromoteTarget:  integrate.Promote,
		DiscardCombined: func(ctx context.Context, primaryRoot, path, branch string) error {
			if err := worktree.Remove(ctx, primaryRoot, path); err != nil {
				return err
			}
			return worktree.DeleteBranch(ctx, primaryRoot, branch)
		},
		RemoveLaneWorktree: func(ctx context.Context, primaryRoot, path, branch string) error {
			if err := worktree.Remove(ctx, primaryRoot, path); err != nil {
				return err
			}
			return worktree.DeleteBranch(ctx, primaryRoot, branch)
		},
		PersistEnvelope: func(ctx context.Context, primaryRoot, laneID string, envelope *result.Envelope) error {
			if envelope == nil {
				return nil
			}
			data, err := json.MarshalIndent(envelope, "", "  ")
			if err != nil {
				return err
			}
			dir := filepath.Join(primaryRoot, ".lucind", "results")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dir, laneID+".json"), data, 0o644)
		},
	}
}

func defaultApprover() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "anonymous"
}

// serveDispatch implements the "serve" subcommand: localhost approvals web UI.
func serveDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usage)
		fs.PrintDefaults()
	}

	addr := fs.String("addr", "127.0.0.1:7433", "listen address (loopback only)")
	approver := fs.String("approver", defaultApprover(), "signed-in approver identity")
	approvalTimeout := fs.Duration("approval-timeout", 30*time.Minute, "informational only -- does not gate lanes; pass --approval-timeout to 'lucind-ai run' to actually enable the wait")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if !serve.IsLoopback(*addr) {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", fmt.Errorf("%w: %s", serve.ErrNonLoopback, *addr))
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	opencodeCmd := "opencode run --agent build -m openai/gpt-5.6-sol"
	handler := serve.NewHandler(ledg, *approver, opencodeCmd)

	fmt.Fprintf(stdout, "lucind-ai serve listening on http://%s (approver: %s, approval timeout: %s)\n", *addr, *approver, *approvalTimeout)

	if err := serve.ListenAndServe(ctx, *addr, handler); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	return 0
}

// featureDispatch dispatches feature subcommands (create, status, recover).
func featureDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = reconcile.ErrInvalidDirection
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: feature subcommand requires an action (create, status, recover)")
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "create":
		return runFeatureCreate(ctx, args[1:], stdout, stderr)
	case "status":
		return runFeatureStatus(ctx, args[1:], stdout, stderr)
	case "recover":
		return runFeatureRecover(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown feature subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runFeatureCreate implements "lucind-ai feature create": creates a new feature in the ledger.
func runFeatureCreate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai feature create --id <id> --parent <ref> --base-sha <sha> [--expected-parent-sha <sha>]")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "feature identifier")
	parent := fs.String("parent", "", "parent branch ref (e.g. refs/heads/feature-foo)")
	parentRef := fs.String("parent-ref", "", "alias for --parent")
	baseSHA := fs.String("base-sha", "", "immutable base commit SHA")
	expectedParentSHA := fs.String("expected-parent-sha", "", "expected parent commit SHA")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	parentVal := *parent
	if parentVal == "" && *parentRef != "" {
		parentVal = *parentRef
	}

	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --id is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(parentVal) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --parent is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(*baseSHA) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --base-sha is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	featSvc := feature.NewService(ledg)
	feat, err := featSvc.Create(ctx, *id, parentVal, *baseSHA, *expectedParentSHA)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "feature:  %s\nparent:   %s\nbase_sha: %s\nstatus:   %s\n", feat.ID, feat.ParentRef, feat.BaseSHA, feat.Status)
	return 0
}

// runFeatureStatus implements "lucind-ai feature status": queries feature, attempt, and lease state via serve.Model.
func runFeatureStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai feature status [--id <id>]")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "feature identifier to query")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	model := serve.NewModel(ledg)

	if *id != "" {
		feat, err := model.GetFeature(ctx, *id)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}

		fmt.Fprintf(stdout, "feature:              %s\n", feat.ID)
		fmt.Fprintf(stdout, "parent_ref:           %s\n", feat.ParentRef)
		fmt.Fprintf(stdout, "base_sha:             %s\n", feat.BaseSHA)
		if feat.ExpectedParentSHA != "" {
			fmt.Fprintf(stdout, "expected_parent_sha:  %s\n", feat.ExpectedParentSHA)
		}
		fmt.Fprintf(stdout, "status:               %s\n", feat.Status)
		fmt.Fprintf(stdout, "created_at:           %s\n", feat.CreatedAt.Format(time.RFC3339))
		fmt.Fprintf(stdout, "updated_at:           %s\n", feat.UpdatedAt.Format(time.RFC3339))

		lease, err := model.GetLease(ctx, *id)
		if err == nil {
			fmt.Fprintf(stdout, "lease:                owner=%s fence=%d expires_at=%s\n", lease.Owner, lease.Fence, lease.ExpiresAt.Format(time.RFC3339))
		} else {
			fmt.Fprintf(stdout, "lease:                none\n")
		}

		attempts, err := model.ListAttempts(ctx, *id)
		if err == nil && len(attempts) > 0 {
			fmt.Fprintln(stdout, "attempts:")
			for _, att := range attempts {
				fmt.Fprintf(stdout, "  - id=%s status=%s owner=%s fence=%d candidate_sha=%s failure_reason=%s\n",
					att.ID, att.Status, att.Owner, att.Fence, att.CandidateSHA, att.FailureReason)
			}
		}

		return 0
	}

	features, err := model.ListFeatures(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: list features: %v\n", err)
		return 1
	}

	leases, _ := model.ListLeases(ctx)
	leaseByFeature := make(map[string]serve.Lease, len(leases))
	for _, l := range leases {
		leaseByFeature[l.FeatureID] = l
	}

	if len(features) == 0 {
		fmt.Fprintln(stdout, "no features registered")
		return 0
	}

	for _, f := range features {
		fmt.Fprintf(stdout, "feature: %s  status: %s  parent_ref: %s  base_sha: %s\n", f.ID, f.Status, f.ParentRef, f.BaseSHA)
		if l, ok := leaseByFeature[f.ID]; ok {
			fmt.Fprintf(stdout, "  lease: owner=%s fence=%d expires_at=%s\n", l.Owner, l.Fence, l.ExpiresAt.Format(time.RFC3339))
		}
	}

	return 0
}

// runFeatureRecover implements "lucind-ai feature recover": invokes attempt recovery.
func runFeatureRecover(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature recover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai feature recover --attempt <id>")
		fs.PrintDefaults()
	}

	attemptID := fs.String("attempt", "", "integration attempt identifier to recover")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*attemptID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --attempt is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	deps := depsFactory(uuid.NewString(), primaryRoot, ledg, defaultTimeout, 0)
	att, err := lucindrun.RecoverAttempt(ctx, deps, *attemptID)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: recover attempt %q: %v\n", *attemptID, err)
		return 1
	}

	switch att.Status {
	case lucindrun.AttemptStatusPromoted:
		fmt.Fprintf(stdout, "resumed: attempt %s promoted (candidate_sha=%s)\n", att.ID, att.CandidateSHA)
		return 0
	case lucindrun.AttemptStatusBlocked:
		fmt.Fprintf(stderr, "blocked: attempt %s blocked, needs human intervention (reason: %s)\n", att.ID, att.FailureReason)
		return 2
	default:
		fmt.Fprintf(stderr, "attempt %s ended with status: %s (reason: %s)\n", att.ID, att.Status, att.FailureReason)
		return 1
	}
}

// reconcileDispatch dispatches reconcile subcommands (approve, decline, cancel, renew).
func reconcileDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: reconcile subcommand requires an action (approve, decline, cancel, renew)")
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "approve":
		return runReconcileApprove(ctx, args[1:], stdout, stderr)
	case "decline":
		return runReconcileDecline(ctx, args[1:], stdout, stderr)
	case "cancel":
		return runReconcileCancel(ctx, args[1:], stdout, stderr)
	case "renew":
		return runReconcileRenew(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown reconcile subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runReconcileApprove implements "lucind-ai reconcile approve": binds exact direction and authorizes candidate.
func runReconcileApprove(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile approve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai reconcile approve --request <id> --source <feature> --target <feature> [--actor <name>] [--allowed-paths <paths>]")
		fs.PrintDefaults()
	}

	requestID := fs.String("request", "", "reconciliation request identifier")
	source := fs.String("source", "", "source feature identifier (direction source)")
	target := fs.String("target", "", "target feature identifier (direction target)")
	actor := fs.String("actor", defaultApprover(), "actor identity recording the decision")
	allowedPaths := fs.String("allowed-paths", "", "comma-separated allowed paths override")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*requestID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --request is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(*source) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --source is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(*target) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --target is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	reconcileSvc := reconcile.NewService(ledg)
	req, err := reconcileSvc.GetRequest(ctx, *requestID)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	// Idempotency: if already approved with matching direction, return original result
	if req.Status == reconcile.RequestStatusApproved {
		if req.SourceFeature == *source && req.TargetFeature == *target {
			cand, err := ledg.ReconciliationCandidateByRequest(ctx, req.ID)
			if err == nil {
				fmt.Fprintf(stdout, "request:   %s\nstatus:    approved\ndirection: %s\ncandidate: %s\nactor:     %s\n", req.ID, req.Direction, cand.ID, req.Actor)
				return 0
			}
		}
		fmt.Fprintf(stderr, "lucind-ai: %v\n", reconcile.ErrInvalidDirection)
		return 1
	}

	if req.Status != reconcile.RequestStatusAwaiting {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", reconcile.ErrRequestNotAwaiting)
		return 1
	}

	if req.SourceFeature != *source || req.TargetFeature != *target {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", reconcile.ErrInvalidDirection)
		return 1
	}

	var paths []string
	if strings.TrimSpace(*allowedPaths) != "" {
		for _, p := range strings.Split(*allowedPaths, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				paths = append(paths, p)
			}
		}
	}

	act := strings.TrimSpace(*actor)
	if act == "" {
		act = defaultApprover()
	}

	appReq, cand, err := reconcileSvc.Approve(ctx, reconcile.ApproveParams{
		RequestID:     *requestID,
		SourceFeature: *source,
		TargetFeature: *target,
		Actor:         act,
		AllowedPaths:  paths,
	})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "request:   %s\nstatus:    approved\ndirection: %s\ncandidate: %s\nactor:     %s\n", appReq.ID, appReq.Direction, cand.ID, appReq.Actor)
	return 0
}

// runReconcileDecline implements "lucind-ai reconcile decline": declines an awaiting reconciliation request.
func runReconcileDecline(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile decline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai reconcile decline --request <id> [--actor <name>] [--reason <reason>]")
		fs.PrintDefaults()
	}

	requestID := fs.String("request", "", "reconciliation request identifier")
	actor := fs.String("actor", defaultApprover(), "actor recording the decline")
	reason := fs.String("reason", "", "optional reason for decline")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*requestID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --request is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	act := strings.TrimSpace(*actor)
	if act == "" {
		act = defaultApprover()
	}

	reconcileSvc := reconcile.NewService(ledg)
	req, err := reconcileSvc.Decline(ctx, *requestID, act, *reason)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "request:   %s\nstatus:    %s\nactor:     %s\n", req.ID, req.Status, req.Actor)
	return 0
}

// runReconcileCancel implements "lucind-ai reconcile cancel": cancels an awaiting reconciliation request.
func runReconcileCancel(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile cancel", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai reconcile cancel --request <id> [--actor <name>] [--reason <reason>]")
		fs.PrintDefaults()
	}

	requestID := fs.String("request", "", "reconciliation request identifier")
	actor := fs.String("actor", defaultApprover(), "actor recording the cancellation")
	reason := fs.String("reason", "", "optional reason for cancellation")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*requestID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --request is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	act := strings.TrimSpace(*actor)
	if act == "" {
		act = defaultApprover()
	}

	reconcileSvc := reconcile.NewService(ledg)
	req, err := reconcileSvc.Cancel(ctx, *requestID, act, *reason)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "request:   %s\nstatus:    %s\nactor:     %s\n", req.ID, req.Status, req.Actor)
	return 0
}

// runReconcileRenew implements "lucind-ai reconcile renew" and top-level "lucind-ai renew": renews an expired or awaiting reconciliation request with fresh evidence.
func runReconcileRenew(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile renew", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai reconcile renew --request <id> [--base-sha <sha>] [--source-sha <sha>] [--target-sha <sha>] [--ttl <duration>]")
		fs.PrintDefaults()
	}

	requestID := fs.String("request", "", "reconciliation request identifier to renew")
	baseSHA := fs.String("base-sha", "", "base commit SHA override")
	sourceSHA := fs.String("source-sha", "", "source commit SHA override")
	targetSHA := fs.String("target-sha", "", "target commit SHA override")
	ttl := fs.Duration("ttl", 15*time.Minute, "time-to-live budget for new request")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*requestID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --request is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	deps := depsFactory(uuid.NewString(), primaryRoot, ledg, defaultTimeout, 0)
	var opts []reconcile.ServiceOption
	if deps.EvaluateOverlap != nil {
		opts = append(opts, reconcile.WithOverlapEvaluator(deps.EvaluateOverlap))
	}

	reconcileSvc := reconcile.NewService(ledg, opts...)
	newReq, err := reconcileSvc.Renew(ctx, reconcile.RenewParams{
		OldRequestID:     *requestID,
		RepoDir:          primaryRoot,
		BaseSHA:          *baseSHA,
		CurrentSourceSHA: *sourceSHA,
		CurrentTargetSHA: *targetSHA,
		TTL:              *ttl,
	})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "request:   %s\nstatus:    %s\nrenewed:   from %s\ndirection: %s\nexpires:   %s\n",
		newReq.ID, newReq.Status, *requestID, newReq.Direction, newReq.ExpiresAt.Format(time.RFC3339))
	return 0
}


