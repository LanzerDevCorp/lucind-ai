package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// These tests cover the CLI's own wiring and failure paths only. Every
// scenario here fails before internal/run.Execute would ever touch git,
// the ledger, or an executor, so none of them shell out to real git or
// invoke real agy — see the package task's hard constraint against that.
//
// printReport itself is unexported but lives in this same package (main),
// so the two tests below call it directly with a hand-built
// lucindrun.Report rather than driving it indirectly through run() --
// there is no other seam that reaches it without a real dispatch.

func TestRunNoArgsPrintsUsageAndFails(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), nil, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run(nil args) exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q, want it to contain usage text", stderr.String())
	}
}

func TestRunUnknownSubcommandPrintsUsageAndFails(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"bogus"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run(bogus) exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q, want it to contain usage text", stderr.String())
	}
	if !strings.Contains(stderr.String(), "bogus") {
		t.Fatalf("stderr = %q, want it to name the unknown subcommand", stderr.String())
	}
}

func TestRunVersionFlagPrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(--version) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "lucind-ai") {
		t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), "lucind-ai")
	}
	if !strings.Contains(stdout.String(), version) {
		t.Fatalf("stdout = %q, want it to contain the build version %q", stdout.String(), version)
	}
}

func TestRunShortVersionFlagPrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"-v"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run(-v) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "lucind-ai") {
		t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), "lucind-ai")
	}
}

func TestRunMissingPacketFlagIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"run"}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with no --packet exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--packet") {
		t.Fatalf("stderr = %q, want it to mention --packet", stderr.String())
	}
}

func TestRunPacketFileDoesNotExist(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "does-not-exist.md")

	code := run(context.Background(), []string{"run", "--packet", missing}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with missing packet file exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), missing) {
		t.Fatalf("stderr = %q, want it to name the missing path %q", stderr.String(), missing)
	}
}

func TestRunMalformedPacketSurfacesParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	if err := os.WriteFile(path, []byte("no frontmatter here, just text\n"), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", path}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with malformed packet exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "frontmatter") {
		t.Fatalf("stderr = %q, want the packet parse error to surface", stderr.String())
	}
}

func TestRunUnsupportedExecutorNamesIt(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-1\n" +
		"executor: bogus-executor\n" +
		"routed_by: single-piece precision\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", path}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with unsupported executor exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "bogus-executor") {
		t.Fatalf("stderr = %q, want it to name the unsupported executor %q", stderr.String(), "bogus-executor")
	}
	if !strings.Contains(stderr.String(), "(supported: agy, cursor-agent)") {
		t.Fatalf("stderr = %q, want it to list supported executors (supported: agy, cursor-agent)", stderr.String())
	}
}

// TestRunModelMismatchedToExecutorIsRejected proves the exact regression
// this check exists for: a packet naming a model from a different
// provider family than its executor (here, a gemini- model on
// cursor-agent) is rejected before any worktree is created, rather than
// silently dispatching and billing against the wrong quota tier.
func TestRunModelMismatchedToExecutorIsRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-1\n" +
		"executor: cursor-agent\n" +
		"routed_by: single-piece precision\n" +
		"model: gemini-3.7-flash-high\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", path}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with mismatched model exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "gemini-3.7-flash-high") {
		t.Fatalf("stderr = %q, want it to name the mismatched model", stderr.String())
	}
	if !strings.Contains(stderr.String(), "cursor-agent") {
		t.Fatalf("stderr = %q, want it to name the executor", stderr.String())
	}
}

// TestRunKnownModelForExecutorPasses proves a model that genuinely belongs
// to the named executor -- including a deliberate escalation away from
// that executor's own default -- passes the check.
func TestRunKnownModelForExecutorPasses(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-1\n" +
		"executor: cursor-agent\n" +
		"routed_by: single-piece precision\n" +
		"model: claude-opus-4-8-high\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", path}, &stdout, &stderr)

	// The model check must pass; the run still fails downstream because
	// this test has no real primary root / ledger wired -- what matters
	// here is that the model mismatch message never appears.
	if code == 0 {
		t.Fatalf("run exit code = 0 with no real dispatch environment, want non-zero for an unrelated reason")
	}
	if strings.Contains(stderr.String(), "not a known model") {
		t.Fatalf("stderr = %q, want the known model claude-opus-4-8-high to pass the check", stderr.String())
	}
}

// TestRunOmittedModelSkipsModelCheck proves a packet that omits model
// entirely is never subject to the known-model check, for any executor.
func TestRunOmittedModelSkipsModelCheck(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-1\n" +
		"executor: cursor-agent\n" +
		"routed_by: single-piece precision\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	run(context.Background(), []string{"run", "--packet", path}, &stdout, &stderr)

	if strings.Contains(stderr.String(), "not a known model") {
		t.Fatalf("stderr = %q, want an omitted model to never trigger the known-model check", stderr.String())
	}
}

// TestRunAcceptsCursorAgentExecutor proves that a packet specifying
// "executor: cursor-agent" passes the pre-dispatch unsupported executor check.
func TestRunAcceptsCursorAgentExecutor(t *testing.T) {
	factory, ok := supportedExecutors["cursor-agent"]
	if !ok {
		t.Fatalf("supportedExecutors[%q] not found, want cursor-agent to be accepted as a supported executor", "cursor-agent")
	}
	if factory == nil || factory() == nil {
		t.Fatalf("supportedExecutors[%q] factory returned nil", "cursor-agent")
	}
}

// TestRunRepeatablePacketFlagPreservesOrderAndProcessesEachOne proves
// --packet is genuinely repeatable, not a last-value-wins flag: the FIRST
// packet given is malformed, so its parse error must surface (naming its
// own path), never the second (well-formed) packet's path. If --packet
// only kept the last occurrence -- a common bug for a naively hand-rolled
// flag.Value -- the malformed first packet would be silently dropped, the
// well-formed second packet would parse fine, and this test would instead
// fail all the way down at unsupported-executor or beyond.
func TestRunRepeatablePacketFlagPreservesOrderAndProcessesEachOne(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "packet-a.md")
	if err := os.WriteFile(firstPath, []byte("no frontmatter here, just text\n"), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}
	secondPath := filepath.Join(dir, "packet-b.md")
	secondContent := "---\n" +
		"id: lane-b\n" +
		"executor: agy\n" +
		"routed_by: single-piece precision\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(secondPath, []byte(secondContent), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", firstPath, "--packet", secondPath}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with a malformed first packet exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "frontmatter") {
		t.Fatalf("stderr = %q, want the first packet's parse error to surface", stderr.String())
	}
	if !strings.Contains(stderr.String(), firstPath) {
		t.Fatalf("stderr = %q, want it to name the first packet's path %q -- if it names the second packet instead, --packet is not actually repeatable", stderr.String(), firstPath)
	}
}

// TestRunMultiplePacketsSecondUnsupportedExecutorIsCaught proves every
// packet in a batch is checked for a supported executor, not just the
// first: a valid first packet must not mask an unsupported executor named
// by a later one.
func TestRunMultiplePacketsSecondUnsupportedExecutorIsCaught(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	firstPath := filepath.Join(dir, "packet-a.md")
	firstContent := "---\n" +
		"id: lane-a\n" +
		"executor: agy\n" +
		"routed_by: single-piece precision\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(firstPath, []byte(firstContent), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}
	secondPath := filepath.Join(dir, "packet-b.md")
	secondContent := "---\n" +
		"id: lane-b\n" +
		"executor: bogus-executor\n" +
		"routed_by: single-piece precision\n" +
		"---\n" +
		"Do another thing.\n"
	if err := os.WriteFile(secondPath, []byte(secondContent), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", firstPath, "--packet", secondPath}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with a batch whose second packet names an unsupported executor exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "bogus-executor") {
		t.Fatalf("stderr = %q, want it to name the unsupported executor %q", stderr.String(), "bogus-executor")
	}
	if !strings.Contains(stderr.String(), secondPath) {
		t.Fatalf("stderr = %q, want it to name the offending packet path %q", stderr.String(), secondPath)
	}
}

// TestRunOverlappingAllowedPathsFailsBeforeCreateWorktree (Task 5.4) proves that
// two packets whose declared AllowedPaths overlap fail the upfront disjointness check,
// returning exit code 1 and never invoking CreateWorktree.
func TestRunOverlappingAllowedPathsFailsBeforeCreateWorktree(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	p1 := filepath.Join(dir, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"allowed_paths: [\"internal/foo/\"]\n" +
		"---\n" +
		"Task 1\n"
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	p2 := filepath.Join(dir, "packet-2.md")
	p2Content := "---\n" +
		"id: lane-2\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"allowed_paths: [\"internal/foo/bar.go\"]\n" +
		"---\n" +
		"Task 2\n"
	if err := os.WriteFile(p2, []byte(p2Content), 0o644); err != nil {
		t.Fatalf("write packet 2: %v", err)
	}

	createCalled := false
	origFactory := depsFactory
	defer func() { depsFactory = origFactory }()
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout)
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			createCalled = true
			return origFactory(runID, primaryRoot, ledg, timeout).CreateWorktree(ctx, primaryRoot, laneID)
		}
		return deps
	}

	code := run(context.Background(), []string{"run", "--packet", p1, "--packet", p2}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run with overlapping allowed_paths exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "overlapping allowed_paths") {
		t.Fatalf("stderr = %q, want it to report overlapping allowed_paths error", stderr.String())
	}
	if !strings.Contains(stderr.String(), "lane-1") || !strings.Contains(stderr.String(), "lane-2") {
		t.Fatalf("stderr = %q, want it to name the overlapping packet IDs", stderr.String())
	}
	if createCalled {
		t.Fatalf("CreateWorktree was invoked, want it never to be called when allowed_paths overlap")
	}
}

// TestRunDisjointAllowedPathsPassesCheck (Task 5.5) proves that two packets declaring
// disjoint allowed_paths pass the upfront check and proceed past it.
func TestRunDisjointAllowedPathsPassesCheck(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	p1 := filepath.Join(dir, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"allowed_paths: [\"internal/foo/\"]\n" +
		"---\n" +
		"Task 1\n"
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	p2 := filepath.Join(dir, "packet-2.md")
	p2Content := "---\n" +
		"id: lane-2\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"allowed_paths: [\"internal/bar/\"]\n" +
		"---\n" +
		"Task 2\n"
	if err := os.WriteFile(p2, []byte(p2Content), 0o644); err != nil {
		t.Fatalf("write packet 2: %v", err)
	}

	run(context.Background(), []string{"run", "--packet", p1, "--packet", p2}, &stdout, &stderr)

	if strings.Contains(stderr.String(), "overlapping allowed_paths") {
		t.Fatalf("stderr = %q, want disjoint allowed_paths to pass the upfront check", stderr.String())
	}
}

// TestPrintReportNotesIncompleteOutputCaptureWhenTruncated proves a report
// carrying OutputCaptureIncomplete gets a diagnostic note, printed after
// the status line (subordinate to it, never a headline), and that the note
// explicitly says this does not by itself mean the lane failed.
func TestPrintReportNotesIncompleteOutputCaptureWhenTruncated(t *testing.T) {
	var stdout bytes.Buffer
	r := lucindrun.Report{
		LaneID:                  "lane-a",
		Status:                  lane.Done,
		Worktree:                "/tmp/worktree",
		OutputCaptureIncomplete: true,
	}

	printReport(&stdout, r)

	out := stdout.String()
	if !strings.Contains(out, "may be incomplete") {
		t.Errorf("printReport output = %q, want it to note the captured output may be incomplete", out)
	}
	if !strings.Contains(out, "does not by itself mean the lane failed") {
		t.Errorf("printReport output = %q, want it to say truncation alone does not mean the lane failed", out)
	}

	statusIdx := strings.Index(out, "status:")
	noteIdx := strings.Index(out, "note:")
	if statusIdx == -1 || noteIdx == -1 || noteIdx < statusIdx {
		t.Errorf("printReport output = %q, want the note line to come after (subordinate to) the status line", out)
	}
}

// TestPrintReportOmitsCaptureNoteWhenNotTruncated proves the note is
// conditional: an ordinary report with a fully-drained capture must not
// print it.
func TestPrintReportOmitsCaptureNoteWhenNotTruncated(t *testing.T) {
	var stdout bytes.Buffer
	r := lucindrun.Report{
		LaneID:                  "lane-a",
		Status:                  lane.Done,
		Worktree:                "/tmp/worktree",
		OutputCaptureIncomplete: false,
	}

	printReport(&stdout, r)

	out := stdout.String()
	if strings.Contains(out, "note:") {
		t.Errorf("printReport output = %q, want no capture note for a non-truncated report", out)
	}
}

// TestPrintReportShowsDiagnosisUnderBannerForFailedLane proves the gap
// this change closes: a lane that did not complete must have its captured
// diagnosis printed to the terminal, under the "LANE DID NOT COMPLETE"
// banner, so a person reading the run's output can see why without
// opening the ledger's SQLite file themselves. The Diagnosis text here
// deliberately mirrors the real incident that motivated this change:
// stderr empty, the actual failure reported as JSON on stdout (agy was
// observed reporting errors that way, not on stderr) -- printReport must
// surface stdout's content just as readily as stderr's.
func TestPrintReportShowsDiagnosisUnderBannerForFailedLane(t *testing.T) {
	var stdout bytes.Buffer
	r := lucindrun.Report{
		LaneID:   "lane-a",
		Status:   lane.Blocked,
		Worktree: "/tmp/worktree",
		Diagnosis: "dispatch exited 1\n" +
			"stderr: (none captured)\n" +
			`stdout: {"status":"ERROR","error":"timeout waiting for response","duration_seconds":84.5}`,
	}

	printReport(&stdout, r)

	out := stdout.String()
	if !strings.Contains(out, "timeout waiting for response") {
		t.Errorf("printReport output = %q, want it to contain the captured diagnosis from stdout", out)
	}
	if !strings.Contains(out, "stderr: (none captured)") {
		t.Errorf("printReport output = %q, want it to also show the (empty) stderr label", out)
	}

	bannerIdx := strings.Index(out, "LANE DID NOT COMPLETE")
	diagnosisIdx := strings.Index(out, "timeout waiting for response")
	if bannerIdx == -1 || diagnosisIdx == -1 || diagnosisIdx < bannerIdx {
		t.Errorf("printReport output = %q, want the diagnosis to appear after (subordinate to) the banner", out)
	}
}

// TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured proves the block is
// conditional: a failed lane whose Diagnosis is empty (e.g. a
// lane.Blocked reported cleanly by the envelope itself, which explains
// itself through Envelope, not Diagnosis) must print no empty diagnosis
// block at all.
func TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured(t *testing.T) {
	var stdout bytes.Buffer
	r := lucindrun.Report{
		LaneID:   "lane-a",
		Status:   lane.Blocked,
		Worktree: "/tmp/worktree",
	}

	printReport(&stdout, r)

	out := stdout.String()
	if strings.Contains(out, "captured diagnosis") {
		t.Errorf("printReport output = %q, want no diagnosis block when Diagnosis is empty", out)
	}
}

// TestPrintReportOmitsDiagnosisBlockForDoneLane proves a lane that
// reached lane.Done never prints a diagnosis block, even in the
// (currently impossible) case its Diagnosis field were somehow set --
// the block lives strictly under the "did not complete" banner, which a
// done lane never prints.
func TestPrintReportOmitsDiagnosisBlockForDoneLane(t *testing.T) {
	var stdout bytes.Buffer
	r := lucindrun.Report{
		LaneID:    "lane-a",
		Status:    lane.Done,
		Worktree:  "/tmp/worktree",
		Diagnosis: "should never print for a done lane",
	}

	printReport(&stdout, r)

	out := stdout.String()
	if strings.Contains(out, "should never print for a done lane") {
		t.Errorf("printReport output = %q, want no diagnosis printed for a lane.Done report", out)
	}
	if strings.Contains(out, "captured diagnosis") {
		t.Errorf("printReport output = %q, want no diagnosis block for a lane.Done report", out)
	}
}

// TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs (Task 5.1) proves that
// given an IntegrateReport with both integrated and reverted lanes, printIntegrateReport
// writes the integrate summary line and the integrated_ids and reverted_ids lines.
func TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs(t *testing.T) {
	var stdout bytes.Buffer
	rep := lucindrun.IntegrateReport{
		Attempted:  true,
		Passed:     true,
		Integrated: []string{"apply-ledger"},
		Reverted:   []string{"apply-serve"},
		Reason:     "bisected out of batch",
	}

	printIntegrateReport(&stdout, rep)

	out := stdout.String()
	if !strings.Contains(out, "integrate: attempted=true passed=true integrated=1 reverted=1 reason=bisected out of batch") {
		t.Errorf("printIntegrateReport output = %q, want it to contain integrate summary count line", out)
	}
	if !strings.Contains(out, "integrated_ids: apply-ledger") {
		t.Errorf("printIntegrateReport output = %q, want it to contain integrated_ids: apply-ledger", out)
	}
	if !strings.Contains(out, "reverted_ids: apply-serve") {
		t.Errorf("printIntegrateReport output = %q, want it to contain reverted_ids: apply-serve", out)
	}
}

// TestPrintIntegrateReportAllIntegratedExplicitlyEmptyRevertedIDs (Task 5.2) proves that
// when all lanes integrate and none are reverted, printIntegrateReport writes integrated_ids
// and an explicitly empty reverted_ids: line (not omitted).
func TestPrintIntegrateReportAllIntegratedExplicitlyEmptyRevertedIDs(t *testing.T) {
	var stdout bytes.Buffer
	rep := lucindrun.IntegrateReport{
		Attempted:  true,
		Passed:     true,
		Integrated: []string{"lane-1", "lane-2"},
		Reverted:   nil,
	}

	printIntegrateReport(&stdout, rep)

	out := stdout.String()
	if !strings.Contains(out, "integrate: attempted=true passed=true integrated=2 reverted=0") {
		t.Errorf("printIntegrateReport output = %q, want it to contain integrate summary count line", out)
	}
	if !strings.Contains(out, "integrated_ids: lane-1 lane-2") {
		t.Errorf("printIntegrateReport output = %q, want it to contain integrated_ids: lane-1 lane-2", out)
	}
	if !strings.Contains(out, "reverted_ids:\n") {
		t.Errorf("printIntegrateReport output = %q, want it to contain explicitly empty reverted_ids:", out)
	}
}

// initRepo creates a throwaway git repository in t.TempDir() with one
// commit, so "git worktree add" has a HEAD to branch from. It works on a
// machine with no configured git identity by passing user.email/user.name
// directly to each command.
func initRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "seed commit")

	return root
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{
		"-c", "user.email=cli-test@example.com",
		"-c", "user.name=cli-test",
	}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v, output = %s", args, err, out)
	}
}

// TestProductionDepsWiresGitBackedInspectionFuncs proves that productionDeps
// assigns non-nil, git-backed HasUniqueLaneCommits and PorcelainEmpty
// closures, and that HasUniqueLaneCommits closes over primaryRoot.
func TestProductionDepsWiresGitBackedInspectionFuncs(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	deps := productionDeps("test-run-id", primaryRoot, nil, 10*time.Minute)

	if deps.HasUniqueLaneCommits == nil {
		t.Fatal("productionDeps.HasUniqueLaneCommits is nil, want non-nil git-backed func")
	}
	if deps.PorcelainEmpty == nil {
		t.Fatal("productionDeps.PorcelainEmpty is nil, want non-nil git-backed func")
	}

	wt, err := worktree.Create(context.Background(), primaryRoot, "lane1")
	if err != nil {
		t.Fatalf("worktree.Create() error = %v, want nil", err)
	}

	// 1. HasUniqueLaneCommits: fresh worktree has no unique commits relative to primaryRoot.
	hasCommits, err := deps.HasUniqueLaneCommits(context.Background(), wt.Path)
	if err != nil {
		t.Fatalf("HasUniqueLaneCommits() error = %v, want nil", err)
	}
	if hasCommits {
		t.Errorf("HasUniqueLaneCommits() = true, want false for a fresh worktree")
	}

	// 2. PorcelainEmpty: fresh worktree is clean.
	clean, err := deps.PorcelainEmpty(context.Background(), wt.Path)
	if err != nil {
		t.Fatalf("PorcelainEmpty() error = %v, want nil", err)
	}
	if !clean {
		t.Errorf("PorcelainEmpty() = false, want true for a fresh worktree")
	}

	// 3. Adding a commit in the worktree causes HasUniqueLaneCommits to report true.
	if err := os.WriteFile(filepath.Join(wt.Path, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(feature.txt) error = %v", err)
	}
	runGit(t, wt.Path, "add", "feature.txt")
	runGit(t, wt.Path, "commit", "-m", "lane commit")

	hasCommits, err = deps.HasUniqueLaneCommits(context.Background(), wt.Path)
	if err != nil {
		t.Fatalf("HasUniqueLaneCommits() error = %v, want nil", err)
	}
	if !hasCommits {
		t.Errorf("HasUniqueLaneCommits() = false, want true after committing in worktree")
	}

	// 4. Adding an untracked file causes PorcelainEmpty to report false.
	if err := os.WriteFile(filepath.Join(wt.Path, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(dirty.txt) error = %v", err)
	}
	clean, err = deps.PorcelainEmpty(context.Background(), wt.Path)
	if err != nil {
		t.Fatalf("PorcelainEmpty() error = %v, want nil", err)
	}
	if clean {
		t.Errorf("PorcelainEmpty() = true, want false when untracked file exists")
	}
}

// TestProductionDepsGitInspectionErrorPropagation proves that git failure on
// invalid paths propagates as an error from both inspection funcs.
func TestProductionDepsGitInspectionErrorPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	invalidDir := t.TempDir()
	deps := productionDeps("test-run-id", invalidDir, nil, 10*time.Minute)

	if deps.HasUniqueLaneCommits == nil {
		t.Fatal("productionDeps.HasUniqueLaneCommits is nil, want non-nil git-backed func")
	}
	if deps.PorcelainEmpty == nil {
		t.Fatal("productionDeps.PorcelainEmpty is nil, want non-nil git-backed func")
	}

	_, err := deps.HasUniqueLaneCommits(context.Background(), invalidDir)
	if err == nil {
		t.Error("HasUniqueLaneCommits() error = nil, want non-nil for non-git directory")
	}

	_, err = deps.PorcelainEmpty(context.Background(), invalidDir)
	if err == nil {
		t.Error("PorcelainEmpty() error = nil, want non-nil for non-git directory")
	}
}

// TestRunSplitTwoWaveDAGSuccess (Task 5.8) proves that invoking `lucind-ai split`
// with --dag and --out on a valid two-wave fixture writes packet files, prints
// the copy-pasteable wave commands to stdout, and exits 0.
func TestRunSplitTwoWaveDAGSuccess(t *testing.T) {
	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"apply-ledger", "apply-serve", "apply-run"} {
		body := "# Goal\n\nGoal for " + name + "\n\n## Done criteria\n\n- [ ] Done\n"
		if err := os.WriteFile(filepath.Join(bodiesDir, name+".md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: apply-ledger
    executor: agy
    routed_by: schema and CRUD isolated from HTTP
    allowed_paths:
      - internal/ledger/
    depends_on: []
    body_path: bodies/apply-ledger.md
  - id: apply-serve
    executor: cursor-agent
    routed_by: HTTP isolated after ledger exists
    allowed_paths:
      - internal/serve/
    depends_on:
      - apply-ledger
    body_path: bodies/apply-serve.md
  - id: apply-run
    executor: agy
    routed_by: run logic isolated after ledger exists
    allowed_paths:
      - internal/run/
    depends_on:
      - apply-ledger
    body_path: bodies/apply-run.md
`
	dagPath := filepath.Join(tempDir, "apply-dag.yaml")
	if err := os.WriteFile(dagPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outDir := filepath.Join(tempDir, "packets")
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"split", "--dag", dagPath, "--out", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(split) exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	// Verify emitted packet files
	for _, expected := range []string{"apply-ledger.md", "apply-serve.md", "apply-run.md"} {
		path := filepath.Join(outDir, expected)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected emitted file %s not found: %v", expected, err)
		}
	}

	// Verify wave commands on stdout
	outStr := stdout.String()
	if !strings.Contains(outStr, "lucind-ai run --packet "+filepath.Join(outDir, "apply-ledger.md")) {
		t.Errorf("stdout = %q, want it to contain wave 1 command", outStr)
	}
	if !strings.Contains(outStr, "apply-serve.md") || !strings.Contains(outStr, "apply-run.md") {
		t.Errorf("stdout = %q, want it to contain wave 2 command with both packets", outStr)
	}
}

// TestRunSplitValidationFailuresExit1AndWriteNoFiles (Task 5.9) proves that
// invalid DAGs (cyclic, duplicate-id, empty-allowed_paths) exit 1 and write no files under --out.
func TestRunSplitValidationFailuresExit1AndWriteNoFiles(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "duplicate id",
			yaml: `change: test
packets:
  - id: dup
    executor: agy
    routed_by: test
    allowed_paths: [internal/a/]
    depends_on: []
    body_path: bodies/dup.md
  - id: dup
    executor: agy
    routed_by: test
    allowed_paths: [internal/b/]
    depends_on: []
    body_path: bodies/dup.md`,
		},
		{
			name: "cycle",
			yaml: `change: test
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths: [internal/a/]
    depends_on: [p2]
    body_path: bodies/p1.md
  - id: p2
    executor: agy
    routed_by: test
    allowed_paths: [internal/b/]
    depends_on: [p1]
    body_path: bodies/p2.md`,
		},
		{
			name: "empty allowed_paths",
			yaml: `change: test
packets:
  - id: p1
    executor: agy
    routed_by: test
    allowed_paths: []
    depends_on: []
    body_path: bodies/p1.md`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			bodiesDir := filepath.Join(tempDir, "bodies")
			if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"dup", "p1", "p2"} {
				if err := os.WriteFile(filepath.Join(bodiesDir, id+".md"), []byte("# Goal\nGoal\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			dagPath := filepath.Join(tempDir, "apply-dag.yaml")
			if err := os.WriteFile(dagPath, []byte(tc.yaml), 0o644); err != nil {
				t.Fatal(err)
			}

			outDir := filepath.Join(tempDir, "packets")
			var stdout, stderr bytes.Buffer

			code := run(context.Background(), []string{"split", "--dag", dagPath, "--out", outDir}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("run(split) on %s exit code = %d, want 1", tc.name, code)
			}
			if stderr.Len() == 0 {
				t.Fatalf("run(split) on %s stderr is empty, want error output", tc.name)
			}

			// OutDir should either not exist or have 0 files
			if entries, err := os.ReadDir(outDir); err == nil && len(entries) > 0 {
				t.Fatalf("outDir has %d entries, want 0 files written on validation failure", len(entries))
			}
		})
	}
}

// TestRunSplitMissingFlagsIsUsageError proves that split requires --dag and --out.
func TestRunSplitMissingFlagsIsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"split"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run(split with no flags) exit code = 0, want 1")
	}
	if !strings.Contains(stderr.String(), "--dag") {
		t.Fatalf("stderr = %q, want mention of --dag", stderr.String())
	}

	stderr.Reset()
	code = run(context.Background(), []string{"split", "--dag", "some/path.yaml"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run(split with no --out) exit code = 0, want 1")
	}
	if !strings.Contains(stderr.String(), "--out") {
		t.Fatalf("stderr = %q, want mention of --out", stderr.String())
	}
}

// TestRunSequentialInvocationsProduceDistinctRunIDs (Task 5.11 & 5.12) proves that
// two sequential runDispatch invocations produce two distinct run id lines on stdout.
func TestRunSequentialInvocationsProduceDistinctRunIDs(t *testing.T) {
	primaryRoot := initRepo(t)

	p1 := filepath.Join(primaryRoot, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"---\n" +
		"Task 1\n"
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	origFactory := depsFactory
	defer func() { depsFactory = origFactory }()
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout)
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: t.TempDir(), Branch: "branch-" + laneID}, nil
		}
		deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath string) (bool, error) {
			return true, nil
		}
		deps.PorcelainEmpty = func(ctx context.Context, worktreePath string) (bool, error) {
			return true, nil
		}
		deps.LookupExecutor = func(name string) (executor.Executor, error) {
			return testDoneExecutor{}, nil
		}
		deps.CombineTree = func(ctx context.Context, primaryRoot, runID string, branches []string) (string, string, error) {
			return t.TempDir(), "integration-branch", nil
		}
		deps.RunChecks = func(ctx context.Context, worktreePath string) (bool, string, error) {
			return true, "", nil
		}
		deps.PromoteTarget = func(ctx context.Context, primaryRoot, integrationBranch string) error {
			return nil
		}
		deps.DiscardCombined = func(ctx context.Context, primaryRoot, worktreePath, branchName string) error {
			return nil
		}
		deps.RemoveLaneWorktree = func(ctx context.Context, primaryRoot, worktreePath, branch string) error {
			return nil
		}
		return deps
	}

	// Change working directory to primaryRoot for resolvePrimaryRoot
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout1, stderr1 bytes.Buffer
	code1 := run(context.Background(), []string{"run", "--packet", p1}, &stdout1, &stderr1)
	if code1 != 0 {
		t.Fatalf("run 1 exit code = %d, want 0; stderr = %q", code1, stderr1.String())
	}

	var stdout2, stderr2 bytes.Buffer
	code2 := run(context.Background(), []string{"run", "--packet", p1}, &stdout2, &stderr2)
	if code2 != 0 {
		t.Fatalf("run 2 exit code = %d, want 0; stderr = %q", code2, stderr2.String())
	}

	runID1 := extractRunID(stdout1.String())
	runID2 := extractRunID(stdout2.String())

	if runID1 == "" {
		t.Fatalf("run 1 stdout has no run id line; stdout = %q", stdout1.String())
	}
	if runID2 == "" {
		t.Fatalf("run 2 stdout has no run id line; stdout = %q", stdout2.String())
	}
	if runID1 == runID2 {
		t.Fatalf("run 1 and run 2 produced the same run id %q, want distinct run IDs per wave", runID1)
	}
}

type testDoneExecutor struct{}

func (testDoneExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	envelope := `{"packet_id": "lane-1", "status": "done", "summary": "done", "hard_stops": []}`
	envelopePath := filepath.Join(req.WorktreePath, ".lucind", "result.json")
	_ = os.MkdirAll(filepath.Dir(envelopePath), 0o755)
	_ = os.WriteFile(envelopePath, []byte(envelope), 0o644)
	return executor.Outcome{ExitCode: 0}, nil
}

func (testDoneExecutor) DefaultModel() string {
	return "test-model"
}

func (testDoneExecutor) KnownModels() []string {
	return []string{"test-model"}
}

func extractRunID(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "run id: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "run id: "))
		}
	}
	return ""
}

// TestRunCheckMissingScriptFails (Task 1.1) proves that invoking check in a repository
// lacking lucind-checks.sh returns exit code 1 and writes the missing-script message to stderr.
func TestRunCheckMissingScriptFails(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	repoDir := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(check) exit code = %d, want 1; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "no lucind-checks.sh found at the project root") {
		t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), "no lucind-checks.sh found at the project root")
	}
}

// TestRunCheckScriptFails (Task 1.3 & 1.4) proves that a repository with a failing
// lucind-checks.sh exits 1 and prints the failure transcript.
func TestRunCheckScriptFails(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	repoDir := initRepo(t)
	scriptPath := filepath.Join(repoDir, "lucind-checks.sh")
	scriptContent := "#!/bin/sh\necho \"FAIL: linter error\"\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(check) exit code = %d, want 1; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(combined, "FAIL: linter error") {
		t.Fatalf("combined output = %q, want it to contain %q", combined, "FAIL: linter error")
	}
}

// TestRunCheckScriptPasses (Task 1.5 & 1.6) proves that a repository with a passing
// lucind-checks.sh exits 0 and prints status, elapsed duration, git commit SHA, and transcript.
func TestRunCheckScriptPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	repoDir := initRepo(t)
	scriptPath := filepath.Join(repoDir, "lucind-checks.sh")
	scriptContent := "#!/bin/sh\necho \"PASS: all suites clean\"\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	// Get commit SHA from HEAD
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	commitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD error = %v", err)
	}
	expectedSHA := strings.TrimSpace(string(commitBytes))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(check) exit code = %d, want 0; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	outStr := stdout.String()
	if !strings.Contains(outStr, "PASS: all suites clean") {
		t.Errorf("stdout = %q, want it to contain %q", outStr, "PASS: all suites clean")
	}
	if !strings.Contains(outStr, "passed") {
		t.Errorf("stdout = %q, want it to contain execution status %q", outStr, "passed")
	}
	if !strings.Contains(outStr, expectedSHA) {
		t.Errorf("stdout = %q, want it to contain git commit SHA %q", outStr, expectedSHA)
	}
	if !strings.Contains(strings.ToLower(outStr), "duration") {
		t.Errorf("stdout = %q, want it to contain execution duration", outStr)
	}
}

// TestRunCheckOutFlagWritesLogFile (Task 1.7 & 1.8) proves that passing --out <path>
// writes the complete record with structured header to the specified file, creating parent dirs.
func TestRunCheckOutFlagWritesLogFile(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	repoDir := initRepo(t)
	scriptPath := filepath.Join(repoDir, "lucind-checks.sh")
	scriptContent := "#!/bin/sh\necho \"PASS: all suites clean\"\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	logPath := filepath.Join(repoDir, "openspec", "changes", "test-change", "verify-mechanical.log")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "--out", logPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(check --out) exit code = %d, want 0; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(logPath) error = %v", err)
	}
	logContent := string(logBytes)

	if !strings.Contains(logContent, "=== lucind-ai mechanical check ===") {
		t.Errorf("logContent = %q, want it to contain header banner", logContent)
	}
	if !strings.Contains(logContent, "Git Commit SHA:") {
		t.Errorf("logContent = %q, want it to contain 'Git Commit SHA:'", logContent)
	}
	if !strings.Contains(logContent, "Duration:") {
		t.Errorf("logContent = %q, want it to contain 'Duration:'", logContent)
	}
	if !strings.Contains(logContent, "Exit Code: 0") {
		t.Errorf("logContent = %q, want it to contain 'Exit Code: 0'", logContent)
	}
	if !strings.Contains(logContent, "PASS: all suites clean") {
		t.Errorf("logContent = %q, want it to contain transcript %q", logContent, "PASS: all suites clean")
	}
}

// TestRunCheckOutFlagOverwritesExistingLog (Task 1.7 & 1.8) proves that re-running
// check with --out overwrites any existing log file cleanly.
func TestRunCheckOutFlagOverwritesExistingLog(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	repoDir := initRepo(t)
	scriptPath := filepath.Join(repoDir, "lucind-checks.sh")
	scriptContent := "#!/bin/sh\necho \"PASS: first run\"\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	logPath := filepath.Join(repoDir, "openspec", "changes", "test-change", "verify-mechanical.log")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "--out", logPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first run(check --out) exit code = %d, want 0", code)
	}

	// Update script for second run
	secondScript := "#!/bin/sh\necho \"PASS: second run\"\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(secondScript), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"check", "--out", logPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second run(check --out) exit code = %d, want 0", code)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(logPath) error = %v", err)
	}
	logContent := string(logBytes)

	if strings.Contains(logContent, "PASS: first run") {
		t.Errorf("logContent = %q, want old content to be cleanly overwritten", logContent)
	}
	if !strings.Contains(logContent, "PASS: second run") {
		t.Errorf("logContent = %q, want it to contain new content %q", logContent, "PASS: second run")
	}
}

// TestRunCheckOmitOutFlagCreatesNoFile (Task 1.5 & 1.6) proves that omitting --out
// writes to stdout/stderr only and does not create any log file.
func TestRunCheckOmitOutFlagCreatesNoFile(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	repoDir := initRepo(t)
	scriptPath := filepath.Join(repoDir, "lucind-checks.sh")
	scriptContent := "#!/bin/sh\necho \"PASS: all suites clean\"\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	logPath := filepath.Join(repoDir, "verify-mechanical.log")

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(check) exit code = %d, want 0", code)
	}

	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("expected no log file created at %s, but stat error = %v", logPath, err)
	}
}

// TestRunCheckUsageAndUnknownFlags (Task 1.9 & 1.10) proves that unknown flags and unexpected
// positional arguments to check exit 1 and print usage, and that the global usage string documents check.
func TestRunCheckUsageAndUnknownFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// 1. Unknown flag to check
	code := run(context.Background(), []string{"check", "--bogus"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(check --bogus) exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lucind-ai check [--out <path>]") {
		t.Fatalf("stderr = %q, want check usage text", stderr.String())
	}

	// 2. Unexpected positional argument to check
	stderr.Reset()
	code = run(context.Background(), []string{"check", "extra-arg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(check extra-arg) exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lucind-ai check [--out <path>]") {
		t.Fatalf("stderr = %q, want check usage text", stderr.String())
	}

	// 3. Global usage documents check subcommand alongside run, split, --version
	stderr.Reset()
	code = run(context.Background(), nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(nil) exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "lucind-ai check [--out <path>]") {
		t.Fatalf("global usage stderr = %q, want it to document 'lucind-ai check [--out <path>]'", stderr.String())
	}
}

// TestFormatMechanicalLogHeader (Task 2.1 & 2.2) proves that formatMechanicalLog prepends
// the structured header with 40-char commit SHA, exit code, duration, and command line to transcript.
func TestFormatMechanicalLogHeader(t *testing.T) {
	commitSHA := "0123456789abcdef0123456789abcdef01234567"
	exitCode := 0
	duration := 1234 * time.Millisecond
	transcript := "PASS: unit tests passed\n"

	got := formatMechanicalLog(commitSHA, exitCode, duration, transcript)

	if !strings.HasPrefix(got, "=== lucind-ai mechanical check ===\n") {
		t.Errorf("got = %q, want prefix '=== lucind-ai mechanical check ===\\n'", got)
	}
	if !strings.Contains(got, "Git Commit SHA: 0123456789abcdef0123456789abcdef01234567\n") {
		t.Errorf("got = %q, want it to contain 'Git Commit SHA: %s\\n'", got, commitSHA)
	}
	if !strings.Contains(got, "Command: lucind-checks.sh\n") {
		t.Errorf("got = %q, want it to contain 'Command: lucind-checks.sh\\n'", got)
	}
	if !strings.Contains(got, "Duration: 1.234s\n") {
		t.Errorf("got = %q, want it to contain 'Duration: 1.234s\\n'", got)
	}
	if !strings.Contains(got, "Exit Code: 0\n") {
		t.Errorf("got = %q, want it to contain 'Exit Code: 0\\n'", got)
	}
	if !strings.Contains(got, "==================================\n") {
		t.Errorf("got = %q, want it to contain header delimiter", got)
	}
	if !strings.HasSuffix(got, transcript) {
		t.Errorf("got = %q, want suffix %q", got, transcript)
	}

	// Test non-zero exit code
	failLog := formatMechanicalLog(commitSHA, 1, 500*time.Millisecond, "FAIL\n")
	if !strings.Contains(failLog, "Exit Code: 1\n") {
		t.Errorf("failLog = %q, want 'Exit Code: 1\\n'", failLog)
	}
}







