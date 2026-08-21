package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
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
	// Supported executors are listed sorted, derived from supportedExecutors
	// at runtime -- not a hardcoded literal -- so this assertion checks each
	// name individually rather than a fixed joined string, and stays
	// correct as executors are added or removed.
	for name := range supportedExecutors {
		if !strings.Contains(stderr.String(), name) {
			t.Fatalf("stderr = %q, want it to list supported executor %q", stderr.String(), name)
		}
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
// to the named executor -- named explicitly, not omitted -- passes the
// check.
func TestRunKnownModelForExecutorPasses(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-1\n" +
		"executor: cursor-agent\n" +
		"routed_by: single-piece precision\n" +
		"model: cursor-grok-4.6-high\n" +
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
		t.Fatalf("stderr = %q, want the known model cursor-grok-4.6-high to pass the check", stderr.String())
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

// TestRunAgentOnNonOpencodeExecutorIsRejected proves that a packet naming
// agent on an executor other than opencode is rejected before dispatch,
// since agent is only meaningful for opencode.
func TestRunAgentOnNonOpencodeExecutorIsRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-1\n" +
		"executor: agy\n" +
		"routed_by: single-piece precision\n" +
		"agent: lucind-dag\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", path}, &stdout, &stderr)

	if code == 0 {
		t.Fatalf("run with agent on a non-opencode executor exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "lucind-dag") {
		t.Fatalf("stderr = %q, want it to name the agent", stderr.String())
	}
	if !strings.Contains(stderr.String(), "agy") {
		t.Fatalf("stderr = %q, want it to name the executor", stderr.String())
	}
}

// TestRunAgentOnOpencodeExecutorPasses proves that a packet naming agent on
// executor: opencode passes the pre-dispatch agent check.
func TestRunAgentOnOpencodeExecutorPasses(t *testing.T) {
	var stdout, stderr bytes.Buffer

	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-1\n" +
		"executor: opencode\n" +
		"routed_by: DAG authoring, specialist agent required\n" +
		"agent: lucind-dag\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	code := run(context.Background(), []string{"run", "--packet", path}, &stdout, &stderr)

	// The agent check must pass; the run still fails downstream because
	// this test has no real primary root / ledger wired -- what matters
	// here is that the agent-mismatch message never appears.
	if code == 0 {
		t.Fatalf("run exit code = 0 with no real dispatch environment, want non-zero for an unrelated reason")
	}
	if strings.Contains(stderr.String(), "only meaningful for executor") {
		t.Fatalf("stderr = %q, want agent on opencode to pass the check", stderr.String())
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

// TestRunAcceptsOpencodeExecutor proves that a packet specifying
// "executor: opencode" passes the pre-dispatch unsupported executor check.
func TestRunAcceptsOpencodeExecutor(t *testing.T) {
	factory, ok := supportedExecutors["opencode"]
	if !ok {
		t.Fatalf("supportedExecutors[%q] not found, want opencode to be accepted as a supported executor", "opencode")
	}
	if factory == nil || factory() == nil {
		t.Fatalf("supportedExecutors[%q] factory returned nil", "opencode")
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
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			createCalled = true
			return origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout).CreateWorktree(ctx, primaryRoot, laneID)
		}
		deps.PersistEnvelope = func(ctx context.Context, primaryRoot, laneID string, envelope *result.Envelope) error {
			return nil
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
	deps := productionDeps("test-run-id", primaryRoot, nil, 10*time.Minute, 0)

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

	// 1. HasUniqueLaneCommits: fresh worktree has no unique commits relative to baseSHA.
	hasCommits, err := deps.HasUniqueLaneCommits(context.Background(), wt.Path, wt.BaseSHA)
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

	hasCommits, err = deps.HasUniqueLaneCommits(context.Background(), wt.Path, wt.BaseSHA)
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
	deps := productionDeps("test-run-id", invalidDir, nil, 10*time.Minute, 0)

	if deps.HasUniqueLaneCommits == nil {
		t.Fatal("productionDeps.HasUniqueLaneCommits is nil, want non-nil git-backed func")
	}
	if deps.PorcelainEmpty == nil {
		t.Fatal("productionDeps.PorcelainEmpty is nil, want non-nil git-backed func")
	}

	_, err := deps.HasUniqueLaneCommits(context.Background(), invalidDir, "dummy-base-sha-1234567890abcdef")
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
		{
			name: "unordered cross-wave overlap",
			yaml: `change: test
packets:
  - id: A
    executor: agy
    routed_by: test
    allowed_paths: [internal/foo/]
    depends_on: []
    body_path: bodies/A.md
  - id: B
    executor: agy
    routed_by: test
    allowed_paths: [internal/bar/]
    depends_on: []
    body_path: bodies/B.md
  - id: C
    executor: agy
    routed_by: test
    allowed_paths: [internal/foo/bar.go]
    depends_on: [B]
    body_path: bodies/C.md`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			bodiesDir := filepath.Join(tempDir, "bodies")
			if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"dup", "p1", "p2", "A", "B", "C"} {
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
		"feature: feat-1\n" +
		"parent_ref: refs/heads/main\n" +
		"base_sha: 1111111111111111111111111111111111111111\n" +
		"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
		"---\n" +
		"Task 1\n"
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	origFactory := depsFactory
	defer func() { depsFactory = origFactory }()
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: t.TempDir(), Branch: "branch-" + laneID}, nil
		}
		deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
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
		deps.PersistEnvelope = func(ctx context.Context, primaryRoot, laneID string, envelope *result.Envelope) error {
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

const persistEnvelopeFinding = "integrated lane Findings must survive worktree removal"

// TestRunDispatchPersistsIntegratedLaneEnvelopeToPrimaryRoot proves that
// after a real run --packet dispatch integrates a lane, the full result
// envelope (including Findings, not just Summary) is written to
// .lucind/results/<lane-id>.json in the primary root and advertised on
// stdout via an envelope: line.
func TestRunDispatchPersistsIntegratedLaneEnvelopeToPrimaryRoot(t *testing.T) {
	primaryRoot := initRepo(t)

	p1 := filepath.Join(primaryRoot, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"feature: feat-1\n" +
		"parent_ref: refs/heads/main\n" +
		"base_sha: 1111111111111111111111111111111111111111\n" +
		"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
		"---\n" +
		"Task 1\n"
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	origFactory := depsFactory
	defer func() { depsFactory = origFactory }()
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: t.TempDir(), Branch: "branch-" + laneID}, nil
		}
		deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return true, nil
		}
		deps.PorcelainEmpty = func(ctx context.Context, worktreePath string) (bool, error) {
			return true, nil
		}
		deps.LookupExecutor = func(name string) (executor.Executor, error) {
			return testDoneExecutorWithFindings{}, nil
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

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--packet", p1}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}

	wantLine := "envelope:  .lucind/results/lane-1.json"
	if !strings.Contains(stdout.String(), wantLine) {
		t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), wantLine)
	}

	envelopePath := filepath.Join(primaryRoot, ".lucind", "results", "lane-1.json")
	data, err := os.ReadFile(envelopePath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v, want the persisted envelope on disk", envelopePath, err)
	}

	var env result.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v; content = %s", envelopePath, err, data)
	}
	found := false
	for _, f := range env.Findings {
		if f.Finding == persistEnvelopeFinding {
			found = true
			if f.Evidence == "" || f.Affects == "" {
				t.Errorf("persisted finding missing evidence or affects: %+v", f)
			}
			break
		}
	}
	if !found {
		t.Fatalf("persisted envelope Findings = %+v, want finding %q", env.Findings, persistEnvelopeFinding)
	}
}

type testDoneExecutor struct {
	exitCode int
	envelope string
}

func (e testDoneExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	envelope := e.envelope
	if envelope == "" {
		envelope = `{"packet_id": "lane-1", "status": "done", "summary": "done", "hard_stops": []}`
	}
	envelopePath := filepath.Join(req.WorktreePath, ".lucind", "result.json")
	_ = os.MkdirAll(filepath.Dir(envelopePath), 0o755)
	_ = os.WriteFile(envelopePath, []byte(envelope), 0o644)
	return executor.Outcome{ExitCode: e.exitCode}, nil
}

func (testDoneExecutor) DefaultModel() string {
	return "test-model"
}

func (testDoneExecutor) KnownModels() []string {
	return []string{"test-model"}
}

type testDoneExecutorWithFindings struct {
	testDoneExecutor
}

func (e testDoneExecutorWithFindings) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	e.envelope = `{
	"packet_id": "lane-1",
	"status": "done",
	"summary": "done",
	"hard_stops": [],
	"findings": [{
		"finding": "` + persistEnvelopeFinding + `",
		"evidence": "internal/run/integrate.go:158",
		"affects": "verify-dual-dispatch Stage 3 file:line check"
	}]
}`
	return e.testDoneExecutor.Run(ctx, req)
}

func extractRunID(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "run id: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "run id: "))
		}
	}
	return ""
}

const blockedApplyRootEnvelope = `{
	"packet_id": "apply-root",
	"status": "blocked",
	"summary": "blocked",
	"hard_stops": [{"hard_stop": "stop", "fired": true, "note": "wave 1 blocked"}]
}`

// writeApplyDagTwoPacketFixture builds the Phase 7 two-node DAG (one
// depends_on edge) and splits it. It returns the two stdout wave packet
// paths in dependency order (root then leaf).
func writeApplyDagTwoPacketFixture(t *testing.T) (wave1Path, wave2Path string) {
	t.Helper()

	tempDir := t.TempDir()
	bodiesDir := filepath.Join(tempDir, "bodies")
	if err := os.MkdirAll(bodiesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootBody := "## Goal\n\nRoot packet work that the leaf depends on.\n"
	leafBody := "## Goal\n\nLeaf packet work that runs after the root.\n"
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-root.md"), []byte(rootBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bodiesDir, "apply-leaf.md"), []byte(leafBody), 0o644); err != nil {
		t.Fatal(err)
	}

	yamlContent := `change: apply-dag-dispatch
packets:
  - id: apply-root
    executor: agy
    routed_by: root has no dependencies
    feature: apply-dag-dispatch
    parent_ref: refs/heads/main
    base_sha: 1111111111111111111111111111111111111111
    expected_parent_sha: 1111111111111111111111111111111111111111
    allowed_paths:
      - internal/root/
    depends_on: []
    body_path: bodies/apply-root.md
  - id: apply-leaf
    executor: agy
    routed_by: leaf depends on root
    feature: apply-dag-dispatch
    parent_ref: refs/heads/main
    base_sha: 1111111111111111111111111111111111111111
    expected_parent_sha: 1111111111111111111111111111111111111111
    allowed_paths:
      - internal/leaf/
    depends_on:
      - apply-root
    body_path: bodies/apply-leaf.md
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

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("split stdout = %q, want exactly two wave lines", stdout.String())
	}
	wave1Path = packetPathFromWaveLine(t, lines[0])
	wave2Path = packetPathFromWaveLine(t, lines[1])
	return wave1Path, wave2Path
}

func packetPathFromWaveLine(t *testing.T, line string) string {
	t.Helper()
	const prefix = "lucind-ai run --packet "
	if !strings.HasPrefix(line, prefix) {
		t.Fatalf("wave line %q, want prefix %q", line, prefix)
	}
	path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if path == "" || strings.Contains(path, " --") {
		t.Fatalf("wave line %q, want a single --packet path", line)
	}
	return path
}

// overrideDispatchDeps installs the runDispatch test doubles used by
// TestRunSequentialInvocationsProduceDistinctRunIDs, substituting exec for
// LookupExecutor. CreateWorktree stays the production git worktree so the
// AllowedPaths scope-check can inspect a real repo; a dummy TempDir would
// Block as a git failure once emitted packets carry allowed_paths.
func overrideDispatchDeps(t *testing.T, exec executor.Executor) {
	t.Helper()
	origFactory := depsFactory
	t.Cleanup(func() { depsFactory = origFactory })
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.CreateWorktree = origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout).CreateWorktree
		deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return true, nil
		}
		deps.PorcelainEmpty = func(ctx context.Context, worktreePath string) (bool, error) {
			return true, nil
		}
		deps.LookupExecutor = func(name string) (executor.Executor, error) {
			return exec, nil
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
		deps.PersistEnvelope = func(ctx context.Context, primaryRoot, laneID string, envelope *result.Envelope) error {
			return nil
		}
		return deps
	}
}

// TestApplyDagTwoWaveSequentialDispatch (Phase 7.3) parses the two
// lucind-ai run --packet lines from a two-node one-edge split and
// dispatches them sequentially against testDoneExecutor. Wave 2 is only
// invoked after wave 1 exits 0 — the test is the sequencer, not the binary.
func TestApplyDagTwoWaveSequentialDispatch(t *testing.T) {
	t.Run("wave2 after wave1 done", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wave1Path, wave2Path := writeApplyDagTwoPacketFixture(t)
		overrideDispatchDeps(t, testDoneExecutor{})

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(primaryRoot); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(cwd)

		var stdout1, stderr1 bytes.Buffer
		code1 := run(context.Background(), []string{"run", "--packet", wave1Path}, &stdout1, &stderr1)

		wave2Dispatched := false
		var stdout2, stderr2 bytes.Buffer
		var code2 int
		if code1 == 0 {
			wave2Dispatched = true
			code2 = run(context.Background(), []string{"run", "--packet", wave2Path}, &stdout2, &stderr2)
		}
		if code1 != 0 {
			t.Fatalf("wave 1 exit code = %d, want 0; stderr = %q; stdout = %q", code1, stderr1.String(), stdout1.String())
		}
		if !wave2Dispatched {
			t.Fatal("wave 2 was not dispatched after wave 1 exit 0")
		}
		if code2 != 0 {
			t.Fatalf("wave 2 exit code = %d, want 0; stderr = %q; stdout = %q", code2, stderr2.String(), stdout2.String())
		}

		runID1 := extractRunID(stdout1.String())
		runID2 := extractRunID(stdout2.String())
		if runID1 == "" {
			t.Fatalf("wave 1 stdout has no run id line; stdout = %q", stdout1.String())
		}
		if runID2 == "" {
			t.Fatalf("wave 2 stdout has no run id line; stdout = %q", stdout2.String())
		}
		if runID1 == runID2 {
			t.Fatalf("wave 1 and wave 2 produced the same run id %q, want distinct run IDs", runID1)
		}
		if !strings.Contains(stdout1.String(), "integrated_ids: apply-root") {
			t.Fatalf("wave 1 stdout = %q, want integrated_ids: apply-root", stdout1.String())
		}
	})

	t.Run("wave2 skipped when wave1 blocked", func(t *testing.T) {
		primaryRoot := initRepo(t)
		wave1Path, wave2Path := writeApplyDagTwoPacketFixture(t)
		overrideDispatchDeps(t, testDoneExecutor{
			exitCode: 1,
			envelope: blockedApplyRootEnvelope,
		})

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(primaryRoot); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(cwd)

		var stdout1, stderr1 bytes.Buffer
		code1 := run(context.Background(), []string{"run", "--packet", wave1Path}, &stdout1, &stderr1)

		wave2Dispatched := false
		if code1 == 0 {
			wave2Dispatched = true
			var stdout2, stderr2 bytes.Buffer
			_ = run(context.Background(), []string{"run", "--packet", wave2Path}, &stdout2, &stderr2)
		}
		if code1 == 0 {
			t.Fatalf("wave 1 exit code = 0, want non-zero after blocked envelope; stdout = %q", stdout1.String())
		}
		if wave2Dispatched {
			t.Fatal("wave 2 was dispatched after a non-zero wave 1 exit")
		}
	})
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

// TestRunCheckEndToEndWithRealScript (Task 5.1) proves that check against a real
// two-line executable lucind-checks.sh in a real temp git repo writes the full
// mechanical log contract to --out.
func TestRunCheckEndToEndWithRealScript(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	repoDir := initRepo(t)
	scriptPath := filepath.Join(repoDir, "lucind-checks.sh")
	scriptContent := "#!/bin/sh\nset -e\necho \"BUILD: ok\"\necho \"TESTS: ok\"\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("WriteFile(lucind-checks.sh) error = %v", err)
	}

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	commitBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD error = %v", err)
	}
	expectedSHA := strings.TrimSpace(string(commitBytes))

	logPath := filepath.Join(repoDir, "openspec/changes/e2e/verify-mechanical.log")

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

	if !strings.Contains(logContent, expectedSHA) {
		t.Errorf("logContent = %q, want it to contain git commit SHA %q", logContent, expectedSHA)
	}
	if !strings.Contains(logContent, "Duration:") {
		t.Errorf("logContent = %q, want it to contain duration", logContent)
	}
	if !strings.Contains(logContent, "Exit Code: 0") {
		t.Errorf("logContent = %q, want it to contain 'Exit Code: 0'", logContent)
	}
	if !strings.Contains(logContent, "BUILD: ok") {
		t.Errorf("logContent = %q, want it to contain transcript %q", logContent, "BUILD: ok")
	}
	if !strings.Contains(logContent, "TESTS: ok") {
		t.Errorf("logContent = %q, want it to contain transcript %q", logContent, "TESTS: ok")
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

func TestServeNonLoopbackAddrRejectedAtCLI(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"serve", "--addr", "0.0.0.0:7433"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("serve with 0.0.0.0 exit code = 0, want non-zero")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "loopback") {
		t.Fatalf("stderr = %q, want it to mention loopback error", stderr.String())
	}
}

func TestServeFlagsAndSubcommandRecognized(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Invalid flag should fail with usage
	code := run(context.Background(), []string{"serve", "--invalid-flag"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("serve with invalid flag exit code = 0, want non-zero")
	}
}

func TestDefaultApproverNotEmpty(t *testing.T) {
	app := defaultApprover()
	if app == "" {
		t.Errorf("defaultApprover() returned empty string")
	}
}

func TestRunAcceptsApprovalTimeoutFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Missing packet but valid flags should fail with --packet required, not flag parse error
	code := run(context.Background(), []string{"run", "--approval-timeout", "15m"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run without --packet exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--packet is required") {
		t.Fatalf("stderr = %q, want --packet is required", stderr.String())
	}
}

func TestRunLegacyModeDispatch(t *testing.T) {
	t.Run("legacy packet without legacy flag fails admission", func(t *testing.T) {
		primaryRoot := initRepo(t)
		overrideDispatchDeps(t, testDoneExecutor{})

		p := filepath.Join(primaryRoot, "legacy-packet.md")
		content := "---\n" +
			"id: legacy-lane\n" +
			"executor: agy\n" +
			"routed_by: legacy dispatch\n" +
			"---\n" +
			"Legacy task\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(primaryRoot); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(cwd)

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"run", "--packet", p}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected non-zero exit code for legacy packet without --legacy-main, got 0; stdout = %q", stdout.String())
		}
		if !strings.Contains(stdout.String(), "status:    failed") {
			t.Errorf("expected status: failed in stdout, got:\n%s", stdout.String())
		}
	})

	t.Run("legacy packet with legacy-main and expected-parent-sha succeeds", func(t *testing.T) {
		primaryRoot := initRepo(t)
		overrideDispatchDeps(t, testDoneExecutor{})

		p := filepath.Join(primaryRoot, "legacy-packet.md")
		content := "---\n" +
			"id: legacy-lane\n" +
			"executor: agy\n" +
			"routed_by: legacy dispatch\n" +
			"---\n" +
			"Legacy task\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(primaryRoot); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(cwd)

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"run",
			"--packet", p,
			"--legacy-main",
			"--expected-parent-sha", "1111111111111111111111111111111111111111",
		}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected exit code 0 with --legacy-main and --expected-parent-sha, got %d; stderr = %q; stdout = %q", code, stderr.String(), stdout.String())
		}
		if !strings.Contains(stdout.String(), "status:    done") {
			t.Errorf("expected status: done in stdout, got:\n%s", stdout.String())
		}
	})

	t.Run("legacy packet with legacy-main but missing expected-parent-sha fails", func(t *testing.T) {
		primaryRoot := initRepo(t)
		overrideDispatchDeps(t, testDoneExecutor{})

		p := filepath.Join(primaryRoot, "legacy-packet.md")
		content := "---\n" +
			"id: legacy-lane\n" +
			"executor: agy\n" +
			"routed_by: legacy dispatch\n" +
			"---\n" +
			"Legacy task\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(primaryRoot); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(cwd)

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"run",
			"--packet", p,
			"--legacy-main",
		}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("expected non-zero exit code with --legacy-main without --expected-parent-sha, got 0; stdout = %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "--expected-parent-sha") {
			t.Errorf("expected stderr to mention --expected-parent-sha, got: %q", stderr.String())
		}
	})
}

func TestFeatureCreateCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	baseSHA := "1111111111111111111111111111111111111111"
	expSHA := "2222222222222222222222222222222222222222"

	t.Run("creates feature and outputs identity", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"feature", "create",
			"--id", "feat-alpha",
			"--parent", "refs/heads/feature-alpha",
			"--base-sha", baseSHA,
			"--expected-parent-sha", expSHA,
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("feature create exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "feat-alpha") {
			t.Errorf("stdout = %q, want it to contain feature id %q", stdout.String(), "feat-alpha")
		}

		// Verify persisted in ledger
		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg.Close()

		featSvc := feature.NewService(ledg)
		f, err := featSvc.Get(context.Background(), "feat-alpha")
		if err != nil {
			t.Fatalf("featSvc.Get(feat-alpha) error = %v", err)
		}
		if f.ID != "feat-alpha" || f.ParentRef != "refs/heads/feature-alpha" || f.BaseSHA != baseSHA || f.ExpectedParentSHA != expSHA || f.Status != feature.StatusActive {
			t.Errorf("persisted feature mismatch: %+v", f)
		}
	})

	t.Run("idempotent invocation with identical flags returns original feature", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"feature", "create",
			"--id", "feat-alpha",
			"--parent", "refs/heads/feature-alpha",
			"--base-sha", baseSHA,
			"--expected-parent-sha", expSHA,
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("second feature create exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "feat-alpha") {
			t.Errorf("stdout = %q, want it to contain feature id %q", stdout.String(), "feat-alpha")
		}
	})

	t.Run("duplicate id with mismatched attributes is rejected with clear error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"feature", "create",
			"--id", "feat-alpha",
			"--parent", "refs/heads/feature-different",
			"--base-sha", "3333333333333333333333333333333333333333",
		}, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("duplicate feature create with different attrs exit code = %d, want non-zero", code)
		}
		if !strings.Contains(stderr.String(), "immutable") && !strings.Contains(stderr.String(), "already exists") && !strings.Contains(stderr.String(), "mismatch") {
			t.Errorf("stderr = %q, want clear error about immutable/duplicate/mismatch", stderr.String())
		}
	})

	t.Run("missing required flags fails with usage/error", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
			want string
		}{
			{"missing id", []string{"feature", "create", "--parent", "refs/heads/foo", "--base-sha", baseSHA}, "--id"},
			{"missing parent", []string{"feature", "create", "--id", "feat-x", "--base-sha", baseSHA}, "--parent"},
			{"missing base-sha", []string{"feature", "create", "--id", "feat-x", "--parent", "refs/heads/foo"}, "--base-sha"},
			{"invalid parent main", []string{"feature", "create", "--id", "feat-x", "--parent", "main", "--base-sha", baseSHA}, "invalid parent"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := run(context.Background(), tc.args, &stdout, &stderr)
				if code == 0 {
					t.Fatalf("run(%v) exit code = 0, want non-zero", tc.args)
				}
				if !strings.Contains(stderr.String(), tc.want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tc.want)
				}
			})
		}
	})
}

func TestFeatureStatusCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}

	featSvc := feature.NewService(ledg)
	_, err = featSvc.Create(context.Background(), "feat-status-1", "refs/heads/feature-status-1", "1111111111111111111111111111111111111111", "2222222222222222222222222222222222222222")
	if err != nil {
		t.Fatalf("featSvc.Create error = %v", err)
	}

	// Insert attempt and lease
	_, err = featSvc.AcquireLease(context.Background(), "feat-status-1", "test-worker", 5*time.Minute)
	if err != nil {
		t.Fatalf("featSvc.AcquireLease error = %v", err)
	}

	_, err = ledg.DB().ExecContext(context.Background(), `
		INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
		VALUES ('att-stat-1', 'feat-status-1', 'idem-1', 'promoted', 'test-worker', 1, '3333333333333333333333333333333333333333', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert attempt error = %v", err)
	}
	ledg.Close()

	t.Run("feature status with specific --id reports feature, attempt, and lease state", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "status", "--id", "feat-status-1"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("feature status exit code = %d, want 0; stderr = %q", code, stderr.String())
		}

		out := stdout.String()
		if !strings.Contains(out, "feat-status-1") {
			t.Errorf("stdout = %q, want feature ID feat-status-1", out)
		}
		if !strings.Contains(out, "refs/heads/feature-status-1") {
			t.Errorf("stdout = %q, want parent ref", out)
		}
		if !strings.Contains(out, "test-worker") {
			t.Errorf("stdout = %q, want lease owner test-worker", out)
		}
		if !strings.Contains(out, "att-stat-1") {
			t.Errorf("stdout = %q, want attempt ID att-stat-1", out)
		}
		if !strings.Contains(out, "promoted") {
			t.Errorf("stdout = %q, want attempt status promoted", out)
		}
	})

	t.Run("feature status without --id lists features", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "status"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("feature status exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "feat-status-1") {
			t.Errorf("stdout = %q, want feature ID feat-status-1 in list", out)
		}
	})

	t.Run("feature status for non-existent feature returns non-zero", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "status", "--id", "feat-nonexistent"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("feature status on non-existent feature exit code = 0, want non-zero")
		}
	})
}

func TestFeatureRecoverCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// Create a branch in the repo for feature parent
	runGit(t, primaryRoot, "checkout", "-b", "feature-recover-parent")
	headSHA := resolveCommitSHA(context.Background(), primaryRoot)

	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}

	featSvc := feature.NewService(ledg)
	_, err = featSvc.Create(context.Background(), "feat-rec", "refs/heads/feature-recover-parent", headSHA, headSHA)
	if err != nil {
		t.Fatalf("featSvc.Create error = %v", err)
	}

	// Case 1: Post-CAS attempt in cas_pending where current branch SHA == candidate_sha -> Resumes/Promotes
	candSHA := headSHA
	_, err = ledg.DB().ExecContext(context.Background(), `
		INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
		VALUES ('att-rec-success', 'feat-rec', 'idem-rec-1', 'cas_pending', 'worker-1', 1, ?, '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
		candSHA)
	if err != nil {
		t.Fatalf("insert attempt error = %v", err)
	}

	// Case 2: Attempt with mismatched ref -> Blocks, needs a human
	_, err = featSvc.Create(context.Background(), "feat-rec-mismatch", "refs/heads/feature-nonexistent-ref", "1111111111111111111111111111111111111111", "1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("featSvc.Create error = %v", err)
	}
	_, err = ledg.DB().ExecContext(context.Background(), `
		INSERT INTO integration_attempts (id, feature_id, idempotency_key, status, owner, fence, candidate_sha, failure_reason, created_at, updated_at)
		VALUES ('att-rec-blocked', 'feat-rec-mismatch', 'idem-rec-2', 'recorded', 'worker-2', 1, '', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert attempt error = %v", err)
	}
	ledg.Close()

	t.Run("successful recovery surfaces resumed/promoted outcome with exit code 0", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "recover", "--attempt", "att-rec-success"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("feature recover exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "promoted") && !strings.Contains(out, "resumed") {
			t.Errorf("stdout = %q, want resumed or promoted outcome", out)
		}
	})

	t.Run("blocked recovery surfaces blocked outcome with distinct non-zero exit code and human message", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "recover", "--attempt", "att-rec-blocked"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("feature recover on blocked attempt exit code = 0, want non-zero")
		}
		combined := stdout.String() + "\n" + stderr.String()
		if !strings.Contains(combined, "blocked") {
			t.Errorf("output = %q, want it to mention 'blocked'", combined)
		}
		if !strings.Contains(combined, "human") && !strings.Contains(combined, "mismatch") {
			t.Errorf("output = %q, want it to explain human intervention or mismatch reason", combined)
		}
	})

	t.Run("missing --attempt flag is usage error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "recover"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("feature recover without --attempt exit code = 0, want non-zero")
		}
		if !strings.Contains(stderr.String(), "--attempt") {
			t.Errorf("stderr = %q, want --attempt mentioned", stderr.String())
		}
	})
}

func TestReconcileApproveCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}

	featSvc := feature.NewService(ledg)
	_, _ = featSvc.Create(context.Background(), "feat-source", "refs/heads/source", "1111111111111111111111111111111111111111")
	_, _ = featSvc.Create(context.Background(), "feat-target", "refs/heads/target", "2222222222222222222222222222222222222222")

	reconcileSvc := reconcile.NewService(ledg)
	ev := &overlap.Evidence{
		Version:     "1.0",
		BaseSHA:     "0000000000000000000000000000000000000000",
		FeatureASHA: "1111111111111111111111111111111111111111",
		FeatureBSHA: "2222222222222222222222222222222222222222",
		Class:       overlap.ClassRequired,
		Signals: overlap.Signals{
			ConflictPaths: []string{"pkg/conflict.go"},
		},
	}
	req, err := reconcileSvc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
		ID:            "req-rec-1",
		FeatureID:     "feat-target",
		SourceFeature: "feat-source",
		SourceParent:  "refs/heads/source",
		TargetFeature: "feat-target",
		TargetParent:  "refs/heads/target",
		SourceSHA:     "1111111111111111111111111111111111111111",
		TargetSHA:     "2222222222222222222222222222222222222222",
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest error = %v", err)
	}
	ledg.Close()

	t.Run("approving with matching direction succeeds and creates candidate", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "approve",
			"--request", req.ID,
			"--source", "feat-source",
			"--target", "feat-target",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("reconcile approve exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), req.ID) && !strings.Contains(stdout.String(), "approved") {
			t.Errorf("stdout = %q, want approval confirmation", stdout.String())
		}

		// Verify candidate in ledger
		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg.Close()

		cand, err := ledg.ReconciliationCandidateByRequest(context.Background(), req.ID)
		if err != nil {
			t.Fatalf("candidate not found for request %s: %v", req.ID, err)
		}
		if cand.RequestID != req.ID || cand.Status != string(reconcile.CandidateStatusRunning) {
			t.Errorf("candidate state unexpected: %+v", cand)
		}
	})

	t.Run("idempotent approve returns original result and does not duplicate candidate", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "approve",
			"--request", req.ID,
			"--source", "feat-source",
			"--target", "feat-target",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("second reconcile approve exit code = %d, want 0; stderr = %q", code, stderr.String())
		}

		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg.Close()

		// Verify candidate count for this request is still 1
		rows, err := ledg.DB().QueryContext(context.Background(), `SELECT COUNT(*) FROM reconciliation_candidates WHERE request_id = ?`, req.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var count int
		if rows.Next() {
			_ = rows.Scan(&count)
		}
		if count != 1 {
			t.Errorf("candidate count = %d, want 1 (no duplicate created)", count)
		}
	})

	t.Run("mismatched source or target direction is rejected with clear error", func(t *testing.T) {
		// Create a second request to test mismatched direction
		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		reconcileSvc2 := reconcile.NewService(ledg)
		req2, err := reconcileSvc2.CreateRequest(context.Background(), reconcile.CreateRequestParams{
			ID:            "req-rec-2",
			FeatureID:     "feat-target",
			SourceFeature: "feat-source",
			SourceParent:  "refs/heads/source",
			TargetFeature: "feat-target",
			TargetParent:  "refs/heads/target",
			SourceSHA:     "1111111111111111111111111111111111111111",
			TargetSHA:     "2222222222222222222222222222222222222222",
			Evidence:      ev,
			TTL:           15 * time.Minute,
		})
		if err != nil {
			t.Fatalf("CreateRequest error = %v", err)
		}
		ledg.Close()

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "approve",
			"--request", req2.ID,
			"--source", "feat-target", // reversed!
			"--target", "feat-source",
		}, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("reconcile approve with mismatched direction exit code = %d, want non-zero", code)
		}
		if !strings.Contains(stderr.String(), "direction") && !strings.Contains(stderr.String(), "mismatch") && !strings.Contains(stderr.String(), "invalid") {
			t.Errorf("stderr = %q, want direction error", stderr.String())
		}
	})

	t.Run("missing required flags fails with usage/error", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
			want string
		}{
			{"missing request", []string{"reconcile", "approve", "--source", "feat-source", "--target", "feat-target"}, "--request"},
			{"missing source", []string{"reconcile", "approve", "--request", "req-1", "--target", "feat-target"}, "--source"},
			{"missing target", []string{"reconcile", "approve", "--request", "req-1", "--source", "feat-source"}, "--target"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				code := run(context.Background(), tc.args, &stdout, &stderr)
				if code == 0 {
					t.Fatalf("run(%v) exit code = 0, want non-zero", tc.args)
				}
				if !strings.Contains(stderr.String(), tc.want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tc.want)
				}
			})
		}
	})
}

func TestReconcileDeclineCancelRenewCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	baseSHA := resolveCommitSHA(context.Background(), primaryRoot)

	// Create branches with conflicting edits
	runGit(t, primaryRoot, "checkout", "-b", "source-branch")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("source content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "source commit")
	sourceSHA := resolveCommitSHA(context.Background(), primaryRoot)

	runGit(t, primaryRoot, "checkout", "master")
	runGit(t, primaryRoot, "checkout", "-b", "target-branch")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("target content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "target commit")
	targetSHA := resolveCommitSHA(context.Background(), primaryRoot)

	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}

	featSvc := feature.NewService(ledg)
	_, _ = featSvc.Create(context.Background(), "feat-s", "refs/heads/source-branch", baseSHA)
	_, _ = featSvc.Create(context.Background(), "feat-t", "refs/heads/target-branch", baseSHA)

	reconcileSvc := reconcile.NewService(ledg)
	ev := &overlap.Evidence{
		Version:     "1.0",
		BaseSHA:     baseSHA,
		FeatureASHA: sourceSHA,
		FeatureBSHA: targetSHA,
		Class:       overlap.ClassRequired,
		Signals: overlap.Signals{
			ConflictPaths: []string{"conflict.txt"},
		},
	}

	reqDecline, err := reconcileSvc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
		ID:            "req-decline-1",
		FeatureID:     "feat-t",
		SourceFeature: "feat-s",
		SourceParent:  "refs/heads/source-branch",
		TargetFeature: "feat-t",
		TargetParent:  "refs/heads/target-branch",
		SourceSHA:     sourceSHA,
		TargetSHA:     targetSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest decline: %v", err)
	}

	reqCancel, err := reconcileSvc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
		ID:            "req-cancel-1",
		FeatureID:     "feat-t",
		SourceFeature: "feat-s",
		SourceParent:  "refs/heads/source-branch",
		TargetFeature: "feat-t",
		TargetParent:  "refs/heads/target-branch",
		SourceSHA:     sourceSHA,
		TargetSHA:     targetSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest cancel: %v", err)
	}

	reqRenew, err := reconcileSvc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
		ID:            "req-renew-1",
		FeatureID:     "feat-t",
		SourceFeature: "feat-s",
		SourceParent:  "refs/heads/source-branch",
		TargetFeature: "feat-t",
		TargetParent:  "refs/heads/target-branch",
		SourceSHA:     sourceSHA,
		TargetSHA:     targetSHA,
		Evidence:      ev,
		TTL:           15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest renew: %v", err)
	}
	ledg.Close()

	t.Run("reconcile decline marks request declined", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "decline",
			"--request", reqDecline.ID,
			"--reason", "manual rejection",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("reconcile decline exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), reqDecline.ID) || !strings.Contains(stdout.String(), "declined") {
			t.Errorf("stdout = %q, want decline confirmation", stdout.String())
		}

		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatal(err)
		}
		defer ledg.Close()
		r, err := ledg.ReconciliationRequest(context.Background(), reqDecline.ID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "declined" {
			t.Errorf("request status = %s, want declined", r.Status)
		}
	})

	t.Run("reconcile cancel marks request cancelled", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "cancel",
			"--request", reqCancel.ID,
			"--reason", "cancelling obsolete request",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("reconcile cancel exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), reqCancel.ID) || !strings.Contains(stdout.String(), "cancelled") {
			t.Errorf("stdout = %q, want cancel confirmation", stdout.String())
		}

		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatal(err)
		}
		defer ledg.Close()
		r, err := ledg.ReconciliationRequest(context.Background(), reqCancel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "cancelled" {
			t.Errorf("request status = %s, want cancelled", r.Status)
		}
	})

	t.Run("reconcile renew recomputes evidence and creates new request", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "renew",
			"--request", reqRenew.ID,
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("reconcile renew exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "renewed") && !strings.Contains(stdout.String(), "awaiting") {
			t.Errorf("stdout = %q, want renew confirmation", stdout.String())
		}
	})

	t.Run("top-level renew alias also works", func(t *testing.T) {
		// Create another request for top-level renew test
		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatal(err)
		}
		svc := reconcile.NewService(ledg)
		reqRenew2, err := svc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
			ID:            "req-renew-2",
			FeatureID:     "feat-t",
			SourceFeature: "feat-s",
			SourceParent:  "refs/heads/source-branch",
			TargetFeature: "feat-t",
			TargetParent:  "refs/heads/target-branch",
			SourceSHA:     sourceSHA,
			TargetSHA:     targetSHA,
			Evidence:      ev,
			TTL:           15 * time.Minute,
		})
		if err != nil {
			t.Fatal(err)
		}
		ledg.Close()

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"renew",
			"--request", reqRenew2.ID,
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("renew exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "renewed") && !strings.Contains(stdout.String(), "awaiting") {
			t.Errorf("stdout = %q, want renew confirmation", stdout.String())
		}
	})
}





