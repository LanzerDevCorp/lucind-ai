package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/LanzerDevCorp/lucind-ai/internal/accept"
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

// attemptOwner is the lease owner every feature-targeted dispatch claims. The
// value is fixed rather than per-process on purpose: feature.AcquireLease
// grants a lease only when the existing one has expired, with no same-owner
// exception (internal/feature/feature.go:307), so one concurrent dispatch on a
// feature blocks the other regardless of who they say they are. A fixed string
// keeps the audit trail readable without weakening that.
const attemptOwner = "lucind-ai run"

// usage is printed on stderr for a missing/unknown subcommand or a usage
// error, so a person driving the binary from a terminal always sees the one
// invocation that works rather than a stack trace. --packet is repeatable:
// each occurrence adds one more lane to the batch.
const usage = "usage: lucind-ai run --packet <path> [--packet <path> ...] [--timeout <duration>] [--approval-timeout <duration>] [--legacy-main] [--expected-parent-sha <sha>] [--min-quota <fraction>]\n       lucind-ai split --dag <path> --out <dir>\n       lucind-ai check [--out <path>]\n       lucind-ai accept --run <run-id> --lane <lane-id>\n       lucind-ai feature create --id <id> --parent <ref> --base-sha <sha> [--expected-parent-sha <sha>]\n       lucind-ai feature status [--id <id>]\n       lucind-ai feature recover --attempt <id>\n       lucind-ai feature renew --id <id> --owner <owner> --fence <fence> [--ttl <duration>]\n       lucind-ai feature lease release --id <id> [--owner <owner>] [--fence <fence>] [--pid <pid>] [--force]\n       lucind-ai feature lease status --id <id>\n       lucind-ai feature disable --id <id>\n       lucind-ai reconcile approve --request <id> --source <feature> --target <feature> [--actor <name>]\n       lucind-ai reconcile decline --request <id> [--actor <name>] [--reason <reason>]\n       lucind-ai reconcile cancel --request <id> [--actor <name>] [--reason <reason>]\n       lucind-ai reconcile renew --request <id> [--base-sha <sha>] [--source-sha <sha>] [--target-sha <sha>] [--wait-stable <duration>]\n       lucind-ai reconcile resolve --candidate <id> --sha <sha> [--actor <name>] [--wait-stable <duration>]\n       lucind-ai defect record --id <id> --feature <id> --signature <sig> [--evidence <ev>] [--disposition <disp>] [--run <run-id>] [--lane <lane-id>]\n       lucind-ai defect list --feature <id>\n       lucind-ai defect decline --id <id>\n       lucind-ai worktree cleanup --lane <id> [--force]\n       lucind-ai integrate retry --run <run-id> [--lane <id> ...] [--timeout <duration>] [--approval-timeout <duration>]\n       lucind-ai --version"

// depsFactory constructs run.Deps for runDispatch. In production it is
// productionDeps; tests may override it to inject test doubles or observe dependency calls.
var depsFactory = productionDeps

// defaultMinQuota is --min-quota's default: the minimum fraction of the
// active agy-pool account's remaining 5-hour Gemini quota required before a
// wave (one runDispatch invocation) is allowed to dispatch an agy-executed
// batch. Below it, ensureAgyQuota rotates to the pooled account with the
// most quota left; see internal/executor.AgyQuota.Ensure's doc comment for
// why the 5-hour window specifically, and why the check runs once per wave
// rather than once per lane.
const defaultMinQuota = 0.10

// ensureAgyQuota gates a batch dispatch on the active Antigravity account's
// remaining 5-hour Gemini quota, rotating to a higher-quota pooled account
// via agy-pool when needed. Only called when the batch includes an
// agy-executed packet (see runDispatch) and --min-quota is non-zero. Tests
// override this to avoid shelling out to the real agy/agy-pool binaries --
// see this package's hard constraint against that in cli_test.go.
var ensureAgyQuota = executor.AgyQuota{}.Ensure

// supportedExecutors names every packet.Executor value this binary knows
// how to dispatch. Unlisted values are a routing error, never a silent
// fallback to agy — see internal/run's Deps.LookupExecutor field.
var supportedExecutors = map[string]func() executor.Executor{
	"agy":          func() executor.Executor { return executor.Agy{} },
	"claude":       func() executor.Executor { return executor.Claude{} },
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

// loadPacket opens path, parses it as a dispatch packet, and stamps
// Packet.Path with the filesystem path that produced it. Parse itself never
// invents a path; only this CLI load step does.
func loadPacket(path string) (packet.Packet, error) {
	f, err := os.Open(path)
	if err != nil {
		return packet.Packet{}, fmt.Errorf("open packet %q: %w", path, err)
	}
	defer f.Close()
	p, err := packet.Parse(f)
	if err != nil {
		return packet.Packet{}, fmt.Errorf("parse packet %q: %w", path, err)
	}
	p.Path = path
	return p, nil
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
	case "accept":
		return runAccept(ctx, args[1:], stdout, stderr)
	case "feature":
		return featureDispatch(ctx, args[1:], stdout, stderr)
	case "reconcile":
		return reconcileDispatch(ctx, args[1:], stdout, stderr)
	case "defect":
		return defectDispatch(ctx, args[1:], stdout, stderr)
	case "worktree":
		return worktreeDispatch(ctx, args[1:], stdout, stderr)
	case "integrate":
		return integrateDispatch(ctx, args[1:], stdout, stderr)
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
	minQuota := fs.Float64("min-quota", defaultMinQuota, "minimum fraction of the active agy-pool account's 5h gemini quota required before dispatching an agy-executed batch; below it, auto-rotates to the pooled account with the most quota (0 disables the check)")

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
		p, err := loadPacket(path)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
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

	inputs := make([]dispatchAuthoringInput, len(ps))
	for i := range ps {
		inputs[i].Packet = ps[i]
	}
	ps, err = admitDispatchBatch(ctx, primaryRoot, inputs)
	if err != nil {
		printAdmissionError(stderr, err)
		return 1
	}

	// One batch produces one combined tree and promotes it once. This target
	// agreement check is still admission and therefore precedes quota and all
	// allocation side effects.
	attemptTarget, featureTargeted, err := lucindrun.FeatureTarget(ps)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	// Wave-level agy quota gate: one runDispatch invocation is one wave (the
	// orchestrator invokes "run" once per wave, with one --packet per lane in
	// it), and this fires exactly once per invocation -- never once per lane
	// -- so it can never rotate the shared active credential out from under a
	// lane that is already dispatching. Must stay before ExecuteBatch and
	// worktree.Create, same as the disjointness check above, so a blocked
	// wave leaves zero side effects. Skipped for a batch with no
	// agy-executed packet: the pooled account's quota has nothing to do with
	// another executor's billing.
	if *minQuota > 0 {
		usesAgy := false
		for _, p := range ps {
			if p.Executor == "agy" {
				usesAgy = true
				break
			}
		}
		if usesAgy {
			if err := ensureAgyQuota(ctx, *minQuota); err != nil {
				fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
				return 1
			}
		}
	}

	// A lane's own worktree is not a place to dispatch from: it would
	// nest a second worktree tree inside a linked worktree and put the
	// ledger somewhere other than the primary repository's .lucind/,
	// which internal/ledger.Open assumes is always the case.
	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
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

	// Register this run before anything else touches the ledger: every lane
	// and event ExecuteBatch is about to write carries this runID, and the
	// ledger derives run/lane/event/progress records from the runs table.
	// Without this row, that data is detached even though the lane and event
	// rows themselves are written with a valid run_id.
	//
	// runID is a fresh uuid.NewString() minted a few lines above and never
	// reused across invocations or retries (recovery of a crashed feature
	// attempt is keyed by attempt id, not run id -- see the "attempt:" line
	// below), so a duplicate-primary-key error here would mean something is
	// genuinely wrong (e.g. a UUID collision or manual ledger tampering),
	// not a benign retry to tolerate: fail the dispatch outright rather than
	// silently proceed with unreliable run bookkeeping.
	runFeatureID, runTargetRef := "", ""
	if featureTargeted {
		runFeatureID = attemptTarget.FeatureID
		runTargetRef = attemptTarget.ParentRef
	}
	if err := ledg.RegisterRun(ctx, ledger.Run{
		RunID:     runID,
		FeatureID: runFeatureID,
		Status:    string(lane.Running),
		TargetRef: runTargetRef,
		LaneCount: len(ps),
		StartedAt: deps.Now(),
		PID:       os.Getpid(),
	}); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: register run: %v\n", err)
		return 1
	}

	// finalStatus is recorded via the deferred UpdateRunStatus below no
	// matter which return statement this function takes from here on --
	// every early "return 1" leaves it at this pessimistic default, and only
	// the success path at the end of this function overwrites it with
	// lane.Done. This is deliberate: a run row must never linger at
	// "running" just because a later step returned early.
	finalStatus := string(lane.Failed)
	defer func() {
		if err := ledg.UpdateRunStatus(ctx, runID, finalStatus, deps.Now()); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: update run status: %v\n", err)
		}
	}()

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

	// A feature-targeted batch promotes by compare-and-swap on its named
	// parent ref, through the durable attempt state machine: lease held for
	// the whole attempt, cross-feature overlap gate before promotion, and a
	// recoverable row if this process dies mid-flight. A legacy batch keeps
	// the ff-merge into the primary checkout it has always used.
	var (
		integrateReport lucindrun.IntegrateReport
		attempt         lucindrun.Attempt
	)
	if featureTargeted {
		attemptTarget.ID = runID
		attemptTarget.IdempotencyKey = runID
		attemptTarget.Owner = attemptOwner
		integrateReport, attempt, err = lucindrun.IntegrateFeature(ctx, deps, batch, attemptTarget)
	} else {
		integrateReport, err = lucindrun.Integrate(ctx, deps, batch)
	}
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

	// The attempt id is what `lucind-ai feature recover --attempt <id>`
	// takes, so it has to reach the operator's terminal even on success.
	if attempt.ID != "" {
		fmt.Fprintf(stdout, "attempt:   %s (%s)\n", attempt.ID, attempt.Status)
	}

	printIntegrateReport(stdout, integrateReport)

	reverted := make(map[string]bool, len(integrateReport.Reverted))
	for _, id := range integrateReport.Reverted {
		reverted[id] = true
	}

	// Exit 0 requires every lane to have actually reached lane.Done and not
	// have been reverted by integration. A blocked/deviated/failed/reverted lane
	// is a real, non-crashing outcome (see printReport), but it must never be
	// mistaken for success by anything reading the exit code. The same
	// condition drives the run row's terminal status recorded by the
	// deferred UpdateRunStatus above.
	for _, r := range batch.Lanes {
		if r.Status != lane.Done || reverted[r.LaneID] {
			return 1
		}
	}
	finalStatus = string(lane.Done)
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

	if err := dag.Split(*dagPath, *outDir, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}
	return 0
}

// runCheck implements the "check" subcommand: executes lucind-checks.sh
// via internal/integrate.Check deterministically and reports results.
//
// Deliberately does NOT use resolvePrimaryRoot: "check" tests the code at
// wherever the caller is actually standing -- worktree or not -- never the
// ledger's primary root. It never touches the ledger, only calls
// integrate.Check(ctx, root) against root's own working tree. Using
// resolvePrimaryRoot here would silently redirect a linked-worktree
// invocation to test the PRIMARY checkout's code while still reporting it
// under the worktree's own identity: a false pass/fail with no error, no
// warning (see the regression this fixes -- resolvePrimaryRoot's
// git-common-dir-based rewrite was correct for the 18 ledger-touching call
// sites, but "check" was never one of them). gitShowToplevel preserves the
// original, correct "wherever you're standing" contract.
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

	root, err := gitShowToplevel(ctx)
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
	fmt.Fprintf(stdout, "status:        passed\nduration:      %v\ncommit:        %s\nresolved root: %s\n", duration, commitSHA, root)

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

type acceptanceVerifier interface {
	Verify(context.Context, accept.AcceptanceRequest) (accept.AcceptanceReceipt, error)
}

var acceptVerifierFactory = func(primaryRoot string, ledg *ledger.Ledger) acceptanceVerifier {
	return accept.NewVerifier(primaryRoot, ledg)
}

// runAccept is a receipt-gated adapter. All authority remains inside accept.Verifier.
func runAccept(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("accept", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai accept --run <run-id> --lane <lane-id>")
		fs.PrintDefaults()
	}

	runID := fs.String("run", "", "persisted run identifier")
	laneID := fs.String("lane", "", "lane identifier to accept")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "lucind-ai: unexpected argument(s): %s\n", strings.Join(fs.Args(), " "))
		fs.Usage()
		return 1
	}

	if strings.TrimSpace(*runID) == "" || strings.TrimSpace(*laneID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --run and --lane are required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: accept: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	receipt, err := acceptVerifierFactory(primaryRoot, ledg).Verify(ctx, accept.AcceptanceRequest{RunID: *runID, LaneID: *laneID})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: accept: %v\n", err)
		return 1
	}
	renderAcceptanceReceipt(stdout, receipt)
	return 0
}

func renderAcceptanceReceipt(w io.Writer, receipt accept.AcceptanceReceipt) {
	fmt.Fprintf(w, "acceptance receipt: %s\n", receipt.ReceiptID)
	fmt.Fprintf(w, "binding: %s\n", receipt.BindingHash)
	fmt.Fprintf(w, "candidate: %s\n", receipt.Binding.CandidateCommit)
	fmt.Fprintln(w, "meaning: mechanical evidence only; qualitative approval remains separate")
	fmt.Fprintln(w, "\nReminder: Complete qualitative review checklist steps 2–10 before promotion.")
	fmt.Fprintln(w, "See plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md for review steps.")
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

		fmt.Fprintf(w, "Inspect changes:\n  git -C %s status\n  git -C %s diff\n", r.Worktree, r.Worktree)
		fmt.Fprintln(w, "See plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md for recovery steps.")
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
	if len(rep.Reverted) > 0 {
		runIDStr := rep.RunID
		if runIDStr == "" {
			runIDStr = "<run-id>"
		}
		fmt.Fprintf(w, "\nReverted lanes preserved. To retry integration without redispatching:\n  lucind-ai integrate retry --run %s\n", runIDStr)
		fmt.Fprintln(w, "See plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md for recovery steps.")
	}
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

// gitShowToplevel runs "git rev-parse --show-toplevel" in the process's
// working directory and returns its trimmed, absolute output.
func gitShowToplevel(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	root := strings.TrimRight(stdout.String(), "\r\n")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("git rev-parse --show-toplevel returned a non-absolute path: %q", root)
	}
	return root, nil
}

// resolvePrimaryRoot resolves the primary repository root directory, even when
// invoked from within a linked worktree or subdirectory.
func resolvePrimaryRoot(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-common-dir")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	commonDir := strings.TrimRight(stdout.String(), "\r\n")
	if !filepath.IsAbs(commonDir) {
		abs, err := filepath.Abs(commonDir)
		if err != nil {
			return "", fmt.Errorf("resolve git common dir %q: %w", commonDir, err)
		}
		commonDir = abs
	}
	commonDir = filepath.Clean(commonDir)
	primaryRoot := filepath.Dir(commonDir)
	if !filepath.IsAbs(primaryRoot) {
		return "", fmt.Errorf("git rev-parse --git-common-dir returned a non-absolute path: %q", primaryRoot)
	}
	return primaryRoot, nil
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
		CreateWorktree: func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
			if parentRef == "" && baseSHA == "" {
				return worktree.Create(ctx, primaryRoot, laneID)
			}
			return worktree.CreateWithParent(ctx, primaryRoot, laneID, parentRef, baseSHA)
		},
		WorktreeFS:               os.DirFS,
		Now:                      time.Now,
		LaneTimeout:              timeout,
		ApprovalTimeout:          approvalTimeout,
		ResolveCandidateIdentity: lucindrun.ResolveCandidateIdentityFromGit,
		HasUniqueLaneCommits: func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return worktree.HasUniqueCommits(ctx, worktreePath, baseSHA)
		},
		PorcelainEmpty: worktree.PorcelainEmpty,
		CombineTree:    integrate.Combine,
		RunChecks:      integrate.Check,
		PromoteTarget:  integrate.Promote,
		PromoteCAS:     integrate.PromoteCAS,
		ResolveRefSHA: func(ctx context.Context, primaryRoot, ref string) (string, error) {
			return worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, primaryRoot, worktree.CanonicalizeRef(ref))
		},
		// The candidate is the tip of the combined worktree, resolved there
		// rather than in primaryRoot. Left unset, ExecuteAttempt falls back to
		// the integration *branch name* and would compare-and-swap the parent
		// ref to a string that is not a commit SHA.
		ResolveCandidateSHA: func(ctx context.Context, primaryRoot, worktreePath, branch string) (string, error) {
			return worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, worktreePath, "HEAD")
		},
		// Guards reuse of a previously approved+integrated reconciliation
		// candidate against reapplying a resolution that predates this
		// attempt's own real content -- see run.Deps.IsAncestorSHA's doc
		// comment. "git merge-base --is-ancestor" exits non-zero both for a
		// genuine "false" and for a real resolution error; either way, not
		// provably an ancestor means reuse is not safe, so both map to false.
		IsAncestorSHA: func(ctx context.Context, primaryRoot, ancestorSHA, descendantSHA string) (bool, error) {
			if _, err := worktree.DefaultGitRunner.Run(ctx, primaryRoot, "merge-base", "--is-ancestor", ancestorSHA, descendantSHA); err != nil {
				return false, nil
			}
			return true, nil
		},
		// The lease is held across combine and the full check run, and nothing
		// renews it mid-attempt. The 30s package default would expire during
		// lucind-checks.sh on any real repository and land the attempt in
		// `stale` after the checks had already passed, so it is pinned to the
		// same clock a lane gets.
		FeatureLeaseTTL: timeout,
		DiscardCombined: func(ctx context.Context, primaryRoot, path, branch string) error {
			if err := worktree.Remove(ctx, primaryRoot, path, true); err != nil {
				return err
			}
			return worktree.DeleteBranch(ctx, primaryRoot, branch)
		},
		RemoveLaneWorktree: func(ctx context.Context, primaryRoot, path, branch string) error {
			if err := worktree.Remove(ctx, primaryRoot, path, true); err != nil {
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

// featureDispatch dispatches feature subcommands (create, status, recover, renew, disable, lease).
func featureDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	_ = reconcile.ErrInvalidDirection
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: feature subcommand requires an action (create, status, recover, renew, disable, lease)")
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
	case "renew":
		return runFeatureRenew(ctx, args[1:], stdout, stderr)
	case "disable":
		return runFeatureDisable(ctx, args[1:], stdout, stderr)
	case "lease":
		return featureLeaseDispatch(ctx, args[1:], stdout, stderr)
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

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
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

// runFeatureStatus implements "lucind-ai feature status": queries feature, attempt, and lease state via feature.Service.
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

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	featSvc := feature.NewService(ledg)

	if *id != "" {
		feat, err := featSvc.Get(ctx, *id)
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

		lease, err := featSvc.GetLease(ctx, *id)
		if err == nil {
			fmt.Fprintf(stdout, "lease:                owner=%s fence=%d expires_at=%s\n", lease.Owner, lease.Fence, lease.ExpiresAt.Format(time.RFC3339))
		} else {
			fmt.Fprintf(stdout, "lease:                none\n")
		}

		attempts, err := featSvc.ListAttempts(ctx, *id)
		if err == nil && len(attempts) > 0 {
			fmt.Fprintln(stdout, "attempts:")
			for _, att := range attempts {
				fmt.Fprintf(stdout, "  - id=%s status=%s owner=%s fence=%d candidate_sha=%s failure_reason=%s\n",
					att.ID, att.Status, att.Owner, att.Fence, att.CandidateSHA, att.FailureReason)
			}
		}

		return 0
	}

	features, err := featSvc.List(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: list features: %v\n", err)
		return 1
	}

	leases, _ := featSvc.ListLeases(ctx)
	leaseByFeature := make(map[string]feature.Lease, len(leases))
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

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
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

// runFeatureRenew implements "lucind-ai feature renew": extends a feature
// lane's held lease (internal/feature.Service.RenewLease), given the exact
// (owner, fence) that currently holds it. This is distinct from "lucind-ai
// reconcile renew", which renews a reconciliation request's evidence/TTL
// instead -- a different concept entirely (see feature.go's Lease vs.
// reconcile.go's Request).
func runFeatureRenew(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature renew", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai feature renew --id <id> --owner <owner> --fence <fence> [--ttl <duration>]")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "feature identifier")
	owner := fs.String("owner", "", "current lease owner")
	fence := fs.Int64("fence", 0, "current lease fencing token")
	ttl := fs.Duration("ttl", 0, "renewed lease time-to-live (0 = RenewLease's own default)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --id is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(*owner) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --owner is required")
		fs.Usage()
		return 1
	}
	if *fence <= 0 {
		fmt.Fprintln(stderr, "lucind-ai: --fence is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	featSvc := feature.NewService(ledg)
	lease, err := featSvc.RenewLease(ctx, *id, *owner, *fence, *ttl)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "feature:    %s\nowner:      %s\nfence:      %d\nexpires_at: %s\n", lease.FeatureID, lease.Owner, lease.Fence, lease.ExpiresAt.Format(time.RFC3339))
	return 0
}

// runFeatureDisable implements "lucind-ai feature disable": retires a feature
// (feature.Service.Disable). A disabled feature stops appearing in the active-feature
// set that the overlap gate consults (internal/ledger.Ledger.ActiveFeatures filters on
// status = 'active'), and its ID becomes eligible for reuse: a later "lucind-ai feature
// create" against the same --id re-anchors and reactivates it, even with a different
// --parent/--base-sha than its original registration. This is the supported path to
// retire a feature registered against a base that turned out to be unusable, without
// permanently losing the ID to ErrFeatureImmutable.
func runFeatureDisable(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature disable", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai feature disable --id <id>")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "feature identifier to disable")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --id is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	featSvc := feature.NewService(ledg)
	if err := featSvc.Disable(ctx, *id); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "feature:  %s\nstatus:   %s\n", *id, feature.StatusDisabled)
	return 0
}

// featureLeaseDispatch dispatches lease subcommands (release, status).
func featureLeaseDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: feature lease subcommand requires an action (release, status)")
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "release":
		return runFeatureLeaseRelease(ctx, args[1:], stdout, stderr)
	case "status":
		return runFeatureLeaseStatus(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown feature lease action %q\n%s\n", args[0], usage)
		return 1
	}
}

// runFeatureLeaseRelease implements "lucind-ai feature lease release": releases an active or orphaned lease.
func runFeatureLeaseRelease(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature lease release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai feature lease release --id <id> [--owner <owner>] [--fence <fence>] [--pid <pid>] [--force]")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "feature identifier")
	owner := fs.String("owner", "", "lease owner")
	fence := fs.Int64("fence", 0, "lease fence token")
	pid := fs.Int("pid", 0, "process id to check for liveness before release")
	force := fs.Bool("force", false, "force release active lease regardless of owner/fence/process")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --id is required")
		fs.Usage()
		return 1
	}

	if *pid > 0 && !*force {
		alive, err := processAlive(*pid)
		if err == nil && alive {
			fmt.Fprintf(stderr, "lucind-ai: refusing to release lease: process %d is still alive\n", *pid)
			return 1
		}
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	svc := feature.NewService(ledg)

	if *owner != "" && *fence > 0 && !*force {
		if err := svc.ReleaseLease(ctx, *id, *owner, *fence); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: release lease: %v\n", err)
			return 1
		}
	} else {
		if err := svc.ForceReleaseLease(ctx, *id); err != nil {
			fmt.Fprintf(stderr, "lucind-ai: release lease: %v\n", err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "feature: %s\nlease:   released\n", *id)
	return 0
}

// runFeatureLeaseStatus implements "lucind-ai feature lease status": checks the current lease state for a feature.
func runFeatureLeaseStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("feature lease status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai feature lease status --id <id>")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "feature identifier")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --id is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	svc := feature.NewService(ledg)
	lease, err := svc.GetLease(ctx, *id)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: get lease: %v\n", err)
		return 1
	}

	now := time.Now().UTC()
	valid := lease.Valid(now)

	fmt.Fprintf(stdout, "feature:    %s\nowner:      %s\nfence:      %d\nexpires_at: %s\nvalid:      %v\n",
		lease.FeatureID, lease.Owner, lease.Fence, lease.ExpiresAt.Format(time.RFC3339), valid)
	return 0
}

func processAlive(pid int) (bool, error) {
	if pid <= 0 {
		return true, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.EPERM):
		return true, nil
	case errors.Is(err, syscall.ESRCH), errors.Is(err, os.ErrProcessDone):
		return false, nil
	default:
		return false, err
	}
}

// reconcileDispatch dispatches reconcile subcommands (approve, decline, cancel, renew).
func reconcileDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: reconcile subcommand requires an action (approve, decline, cancel, renew, resolve)")
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
	case "resolve":
		return runReconcileResolve(ctx, args[1:], stdout, stderr)
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

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
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

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
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

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
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

// resolveFeatureTipSHA resolves featureID's current real tip SHA using the
// same fallback chain internal/run/attempt.go's evaluateOverlapGate uses for
// the opposing feature in an overlap comparison: featureID's own declared
// ParentRef, live-resolved in primaryRoot; falling back to its recorded
// ExpectedParentSHA; falling back to its recorded BaseSHA. Returns an error
// only when the feature itself cannot be found -- an unresolvable ParentRef
// is not fatal here, since the ExpectedParentSHA/BaseSHA fallbacks may still
// produce a usable value.
func resolveFeatureTipSHA(ctx context.Context, ledg *ledger.Ledger, primaryRoot, featureID string) (string, error) {
	feat, err := feature.NewService(ledg).Get(ctx, featureID)
	if err != nil {
		return "", err
	}
	if feat.ParentRef != "" {
		if sha, err := worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, primaryRoot, worktree.CanonicalizeRef(feat.ParentRef)); err == nil && sha != "" {
			return sha, nil
		}
	}
	if feat.ExpectedParentSHA != "" {
		return feat.ExpectedParentSHA, nil
	}
	return feat.BaseSHA, nil
}

// runReconcileRenew implements "lucind-ai reconcile renew": renews an expired or awaiting reconciliation request with fresh evidence.
//
// --source-sha/--target-sha are optional overrides. Left unset, each defaults to that feature's
// own current real tip (resolveFeatureTipSHA) rather than silently carrying forward whatever SHA
// the request being renewed already had stored -- which is what a caller who only ever passes one
// of the two flags (or neither) would otherwise get, and which can permanently pin a stale seed
// value (e.g. from the very first, automatically-created request) across every renew/approve/resolve
// cycle, defeating the whole point of renewing.
func runReconcileRenew(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile renew", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai reconcile renew --request <id> [--base-sha <sha>] [--source-sha <sha>] [--target-sha <sha>] [--wait-stable <duration>] [--ttl <duration>]")
		fmt.Fprintln(stderr, "  --source-sha/--target-sha default to each feature's current real ParentRef tip when omitted.")
		fs.PrintDefaults()
	}

	requestID := fs.String("request", "", "reconciliation request identifier to renew")
	baseSHA := fs.String("base-sha", "", "base commit SHA override")
	sourceSHA := fs.String("source-sha", "", "source commit SHA override")
	targetSHA := fs.String("target-sha", "", "target commit SHA override")
	waitStable := fs.Duration("wait-stable", 0, "duration to wait for target branch stability before renewing")
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

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
		return 1
	}

	if *waitStable > 0 && *targetSHA != "" {
		stableSHA, err := waitBranchStable(ctx, primaryRoot, *targetSHA, *waitStable)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: wait stable: %v\n", err)
			return 1
		}
		*targetSHA = stableSHA
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
		ResolveFeatureTipSHA: func(ctx context.Context, featureID string) (string, error) {
			return resolveFeatureTipSHA(ctx, ledg, primaryRoot, featureID)
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "request:   %s\nstatus:    %s\nrenewed:   from %s\ndirection: %s\nexpires:   %s\n",
		newReq.ID, newReq.Status, *requestID, newReq.Direction, newReq.ExpiresAt.Format(time.RFC3339))
	return 0
}

// runReconcileResolve implements "lucind-ai reconcile resolve": registers a human-produced
// resolution commit against an approved reconciliation request's candidate, closing the
// reconciliation loop.
func runReconcileResolve(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("reconcile resolve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai reconcile resolve --candidate <id> --sha <sha> [--actor <name>] [--wait-stable <duration>]")
		fs.PrintDefaults()
	}

	candidateID := fs.String("candidate", "", "reconciliation candidate identifier")
	sha := fs.String("sha", "", "commit sha of the human-produced resolution")
	actor := fs.String("actor", defaultApprover(), "actor identity recording the resolution")
	waitStable := fs.Duration("wait-stable", 0, "duration to wait for resolution commit stability before resolving")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*candidateID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --candidate is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(*sha) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --sha is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
		return 1
	}

	if *waitStable > 0 {
		stableSHA, err := waitBranchStable(ctx, primaryRoot, *sha, *waitStable)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: wait stable: %v\n", err)
			return 1
		}
		*sha = stableSHA
	}

	resolvedSHA, err := worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, primaryRoot, *sha)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: --sha %q does not resolve to a commit in this repository: %v\n", *sha, err)
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
	cand, err := reconcileSvc.UpdateCandidateStatus(ctx, *candidateID, reconcile.CandidateStatusIntegrated, resolvedSHA, "")
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "candidate: %s\nrequest:   %s\nstatus:    %s\nsha:       %s\nactor:     %s\n",
		cand.ID, cand.RequestID, cand.Status, cand.CandidateSHA, act)
	return 0
}

func waitBranchStable(ctx context.Context, dir, ref string, waitStable time.Duration) (string, error) {
	if waitStable <= 0 {
		return worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, dir, ref)
	}

	timer := time.NewTimer(waitStable)
	defer timer.Stop()

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	lastSHA, err := worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, dir, ref)
	if err != nil {
		return "", fmt.Errorf("resolve initial ref %s: %w", ref, err)
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-timer.C:
			return lastSHA, nil
		case <-ticker.C:
			currentSHA, err := worktree.ResolveCommitSHA(ctx, worktree.DefaultGitRunner, dir, ref)
			if err != nil {
				return "", fmt.Errorf("resolve ref %s: %w", ref, err)
			}
			if currentSHA != lastSHA {
				lastSHA = currentSHA
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(waitStable)
			}
		}
	}
}

// worktreeDispatch dispatches worktree subcommands (cleanup).
func worktreeDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: worktree subcommand requires an action (cleanup)")
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "cleanup":
		return runWorktreeCleanup(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown worktree subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runWorktreeCleanup implements "lucind-ai worktree cleanup": removes a
// lane's stale linked worktree left behind after a block-for-inspection
// outcome, so the identical packet id can be retried without hitting
// worktree.ErrWorktreeExists. Idempotent by design: cleanup on a lane with
// no worktree on disk succeeds the same way as removing one that exists,
// so success is reported either way without distinguishing the two.
func runWorktreeCleanup(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("worktree cleanup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai worktree cleanup --lane <id> [--force]")
		fs.PrintDefaults()
	}

	laneID := fs.String("lane", "", "lane identifier whose worktree should be cleaned up")
	force := fs.Bool("force", false, "force removal of worktree even if it contains uncommitted changes")
	fs.BoolVar(force, "f", false, "alias for --force")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*laneID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --lane is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
		return 1
	}

	if err := worktree.Cleanup(ctx, primaryRoot, *laneID, *force); err != nil {
		if errors.Is(err, worktree.ErrWorktreeDirty) {
			wtPath := worktree.PathFor(primaryRoot, *laneID)
			fmt.Fprintf(stderr, "lucind-ai: worktree for lane %q has uncommitted changes\n", *laneID)
			fmt.Fprintf(stderr, "Worktree path: %s\n", wtPath)
			out, _ := worktree.DefaultGitRunner.Run(ctx, wtPath, "status", "--porcelain")
			if len(out) > 0 {
				fmt.Fprintf(stderr, "Uncommitted changes:\n%s", string(out))
			}
			fmt.Fprintf(stderr, "Inspect changes:\n  git -C %s status\n  git -C %s diff\n", wtPath, wtPath)
			fmt.Fprintln(stderr, "See plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md for recovery steps.")
			fmt.Fprintf(stderr, "To discard all uncommitted changes and remove the worktree, re-run with --force:\n  lucind-ai worktree cleanup --lane %s --force\n", *laneID)
			return 1
		}
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "worktree: cleaned up lane %s\n", *laneID)
	return 0
}

// integrateDispatch dispatches integrate subcommands (retry).
func integrateDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: integrate subcommand requires an action (retry)")
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "retry":
		return runIntegrateRetry(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown integrate subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runIntegrateRetry implements "lucind-ai integrate retry": re-runs the
// combine/check/promote step over lane branches that already reached their
// own "done" in a previous "lucind-ai run" but were reverted because that
// batch's aggregate integration step failed (e.g. the base was red, unrelated
// to the lanes' own work) -- without redispatching any AI lane.
//
// It reconstructs the batch directly from durable state
// (run.RebuildBatchForRetry: the ledger's lane rows plus each lane's own
// preserved worktree and on-disk result envelope), then feeds it through the
// exact same run.Integrate / run.IntegrateFeature path "lucind-ai run" itself
// uses -- so a retry gets the identical bisection, checks, and (for a
// feature-targeted run) compare-and-swap promotion semantics. A
// feature-targeted run re-reads its feature's CURRENT parent_ref/base_sha
// from the ledger at retry time, so re-anchoring the feature first (see
// "lucind-ai feature disable" then "feature create") is enough to retry
// against a corrected base.
//
// --lane may be repeated to hand-pick specific lane IDs; every one named
// must qualify (preserved worktree, "done" envelope) or the retry fails
// closed naming which lane and why. Omitted, every qualifying lane in the
// run is included automatically.
func runIntegrateRetry(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("integrate retry", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai integrate retry --run <run-id> [--lane <id> ...] [--timeout <duration>] [--approval-timeout <duration>]")
		fs.PrintDefaults()
	}

	runID := fs.String("run", "", "run id whose completed, reverted lanes should be re-integrated")
	var laneFlags packetPaths
	fs.Var(&laneFlags, "lane", "lane id to retry (repeatable; omit to auto-select every done+preserved lane in the run)")
	timeout := fs.Duration("timeout", defaultTimeout, "wall clock budget granted to the combine/check step")
	approvalTimeout := fs.Duration("approval-timeout", 0, "approval timeout budget granted to lane gates (0 = no wait / bypass)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*runID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --run is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	toplevel, err := gitShowToplevel(ctx)
	if err == nil && worktree.IsLinkedWorktree(toplevel) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", toplevel)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	runRow, err := ledg.GetRun(ctx, *runID)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	deps := depsFactory(*runID, primaryRoot, ledg, *timeout, *approvalTimeout)

	batch, err := lucindrun.RebuildBatchForRetry(ctx, deps, *runID, []string(laneFlags))
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "retrying integration for run %s: %d lane(s): %s\n", *runID, len(batch.Outcome.Integrate), strings.Join(batch.Outcome.Integrate, ", "))

	var (
		integrateReport lucindrun.IntegrateReport
		attempt         lucindrun.Attempt
	)
	if runRow.FeatureID != "" {
		featSvc := feature.NewService(ledg)
		feat, ferr := featSvc.Get(ctx, runRow.FeatureID)
		if ferr != nil {
			fmt.Fprintf(stderr, "lucind-ai: get feature %q: %v\n", runRow.FeatureID, ferr)
			return 1
		}

		// The feature row's own ParentRef/BaseSHA/ExpectedParentSHA are set
		// once at feature.Service.Create and never updated again, so they
		// are only correct for a feature's first wave. RetryFeatureTarget
		// prefers the original packet's own dispatch-time target, recovered
		// from each included lane's LaneMetadata, which is what a later
		// wave's CAS promotion actually used -- see RetryFeatureTarget's doc
		// comment.
		target, terr := lucindrun.RetryFeatureTarget(ctx, deps, feat, batch.Outcome.Integrate)
		if terr != nil {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", terr)
			return 1
		}

		attemptID := uuid.NewString()
		target.ID = attemptID
		target.IdempotencyKey = attemptID
		target.Owner = attemptOwner
		integrateReport, attempt, err = lucindrun.IntegrateFeature(ctx, deps, batch, target)
	} else {
		integrateReport, err = lucindrun.Integrate(ctx, deps, batch)
	}
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	printIntegrateReport(stdout, integrateReport)
	if attempt.ID != "" {
		fmt.Fprintf(stdout, "attempt:   %s (%s)\n", attempt.ID, attempt.Status)
	}

	if !integrateReport.Passed {
		return 1
	}
	return 0
}

// defectDispatch dispatches defect subcommands (record, list, decline).
func defectDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: defect subcommand requires an action (record, list, decline)")
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "record":
		return runDefectRecord(ctx, args[1:], stdout, stderr)
	case "list":
		return runDefectList(ctx, args[1:], stdout, stderr)
	case "decline":
		return runDefectDecline(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown defect subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runDefectRecord implements "lucind-ai defect record": records a defect in the ledger.
func runDefectRecord(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("defect record", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai defect record --id <id> --feature <id> --signature <sig> [--evidence <ev>] [--disposition <disp>] [--run <run-id>] [--lane <lane-id>]")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "defect record identifier")
	featureID := fs.String("feature", "", "feature identifier")
	signature := fs.String("signature", "", "error signature")
	evidence := fs.String("evidence", "", "error evidence or stack trace")
	disposition := fs.String("disposition", string(ledger.DefectDispositionRecorded), "disposition (recorded, repaired, declined, deferred)")
	runID := fs.String("run", "", "associated run identifier")
	laneID := fs.String("lane", "", "associated lane identifier")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --id is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(*featureID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --feature is required")
		fs.Usage()
		return 1
	}
	if strings.TrimSpace(*signature) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --signature is required")
		fs.Usage()
		return 1
	}

	disp := strings.TrimSpace(*disposition)
	if disp == "" {
		disp = string(ledger.DefectDispositionRecorded)
	}
	if !ledger.DefectDisposition(disp).Valid() {
		fmt.Fprintf(stderr, "lucind-ai: invalid disposition %q (valid: recorded, repaired, declined, deferred)\n", disp)
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	rec := ledger.DefectRecord{
		ID:             *id,
		FeatureID:      *featureID,
		RunID:          *runID,
		LaneID:         *laneID,
		ErrorSignature: *signature,
		Evidence:       *evidence,
		Disposition:    ledger.DefectDisposition(disp),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := ledg.RecordDefect(ctx, rec); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "recorded defect %s for feature %s\n", *id, *featureID)
	return 0
}

// runDefectList implements "lucind-ai defect list": lists defects for a feature from the ledger.
func runDefectList(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("defect list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai defect list --feature <id>")
		fs.PrintDefaults()
	}

	featureID := fs.String("feature", "", "feature identifier")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*featureID) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --feature is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	defects, err := ledg.ListDefects(ctx, *featureID)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	if len(defects) == 0 {
		fmt.Fprintf(stdout, "no defects recorded for feature %s\n", *featureID)
		return 0
	}

	for _, d := range defects {
		fmt.Fprintf(stdout, "defect: %s  disposition: %s  signature: %s  created_at: %s\n",
			d.ID, d.Disposition, d.ErrorSignature, d.CreatedAt.Format(time.RFC3339))
	}
	return 0
}

// runDefectDecline implements "lucind-ai defect decline": transitions an existing defect record to declined.
func runDefectDecline(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("defect decline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai defect decline --id <id>")
		fs.PrintDefaults()
	}

	id := fs.String("id", "", "defect record identifier")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "lucind-ai: --id is required")
		fs.Usage()
		return 1
	}

	primaryRoot, err := resolvePrimaryRoot(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: resolve primary repository root: %v\n", err)
		return 1
	}

	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open ledger: %v\n", err)
		return 1
	}
	defer ledg.Close()

	if err := ledg.UpdateDefectDisposition(ctx, *id, ledger.DefectDispositionDeclined); err != nil {
		fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "declined defect %s\n", *id)
	return 0
}
