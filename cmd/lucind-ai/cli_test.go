package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/accept"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/overlap"
	"github.com/LanzerDevCorp/lucind-ai/internal/packet"
	"github.com/LanzerDevCorp/lucind-ai/internal/packetauthor"
	"github.com/LanzerDevCorp/lucind-ai/internal/phasespec"
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

// TestRunAcceptsClaudeExecutor proves that a packet specifying
// "executor: claude" passes the pre-dispatch unsupported executor check.
func TestRunAcceptsClaudeExecutor(t *testing.T) {
	factory, ok := supportedExecutors["claude"]
	if !ok {
		t.Fatalf("supportedExecutors[%q] not found, want claude to be accepted as a supported executor", "claude")
	}
	if factory == nil || factory() == nil {
		t.Fatalf("supportedExecutors[%q] factory returned nil", "claude")
	}
}

// TestEveryExecutorOwnsExactlyOneProviderFamily pins the invariant that made
// adding a fourth executor safe: each registered executor may run on its own
// models only, so a model string copied from a sibling packet can never
// silently dispatch -- and bill -- against a different provider. Adding an
// executor whose KnownModels overlaps another's would break this.
func TestEveryExecutorOwnsExactlyOneProviderFamily(t *testing.T) {
	owner := map[string]string{}
	for name, factory := range supportedExecutors {
		for _, model := range factory().KnownModels() {
			if prior, clash := owner[model]; clash {
				t.Errorf("model %q is claimed by both %q and %q; every model must belong to exactly one executor", model, prior, name)
				continue
			}
			owner[model] = name
		}
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

// TestRunPacketFlagPopulatesPacketPath proves the load loop stamps each
// successfully parsed packet with the --packet path that produced it.
// Parse itself never invents a filesystem path; only this CLI wiring does.
func TestRunPacketFlagPopulatesPacketPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "packet.md")
	content := "---\n" +
		"id: lane-path\n" +
		"executor: agy\n" +
		"routed_by: path capture\n" +
		"---\n" +
		"Do the thing.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet fixture: %v", err)
	}

	got, err := loadPacket(path)
	if err != nil {
		t.Fatalf("loadPacket(%q) error = %v", path, err)
	}
	if got.Path != path {
		t.Fatalf("Packet.Path = %q, want %q", got.Path, path)
	}
	if got.ID != "lane-path" {
		t.Fatalf("Packet.ID = %q, want lane-path (parse still reflects frontmatter)", got.ID)
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
		deps.ResolveCandidateIdentity = stubCandidateIdentity
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
			createCalled = true
			return origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout).CreateWorktree(ctx, primaryRoot, laneID, parentRef, baseSHA)
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

func TestPrintReportEmitsTroubleshootingBannerOnNonDone(t *testing.T) {
	var stdout bytes.Buffer
	r := lucindrun.Report{
		LaneID:   "lane-a",
		Status:   lane.Blocked,
		Worktree: "/tmp/worktrees/lane-a",
	}

	printReport(&stdout, r)

	out := stdout.String()
	if !strings.Contains(out, "troubleshooting.md") {
		t.Errorf("printReport output = %q, want reference to troubleshooting.md", out)
	}
	if !strings.Contains(out, "git -C /tmp/worktrees/lane-a status") || !strings.Contains(out, "git -C /tmp/worktrees/lane-a diff") {
		t.Errorf("printReport output = %q, want git diff and status inspection commands", out)
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
	if strings.Contains(out, "troubleshooting.md") {
		t.Errorf("printReport output = %q, want no troubleshooting banner for a lane.Done report", out)
	}
}

// TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs (Task 5.1) proves that
// given an IntegrateReport with both integrated and reverted lanes, printIntegrateReport
// writes the integrate summary line, the integrated_ids and reverted_ids lines, and
// the integrate retry banner citing recovery-reconciliation.md.
func TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs(t *testing.T) {
	var stdout bytes.Buffer
	rep := lucindrun.IntegrateReport{
		RunID:      "run-456",
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
	if !strings.Contains(out, "lucind-ai integrate retry --run run-456") {
		t.Errorf("printIntegrateReport output = %q, want integrate retry command banner", out)
	}
	if !strings.Contains(out, "recovery-reconciliation.md") {
		t.Errorf("printIntegrateReport output = %q, want reference to recovery-reconciliation.md", out)
	}
}

// TestPrintIntegrateReportAllIntegratedExplicitlyEmptyRevertedIDs (Task 5.2) proves that
// when all lanes integrate and none are reverted, printIntegrateReport writes integrated_ids
// and an explicitly empty reverted_ids: line (not omitted), and omits the retry guidance banner.
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
	if strings.Contains(out, "integrate retry") || strings.Contains(out, "recovery-reconciliation.md") {
		t.Errorf("printIntegrateReport output = %q, want retry banner omitted when no reverted lanes", out)
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
	setupTestSkills(t, root)
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "seed commit")

	return root
}

func setupTestSkills(t *testing.T, root string) {
	t.Helper()
	skillsDir := filepath.Join(root, ".agents", "skills")
	skills := []string{
		"lucind-executor", "lucind-apply", "lucind-verify", "lucind-fan-out-lens",
		"sdd-explore", "sdd-propose", "sdd-spec", "sdd-design", "sdd-tasks",
		"sdd-apply", "sdd-verify", "sdd-archive",
	}
	for _, s := range skills {
		sDir := filepath.Join(skillsDir, s)
		if err := os.MkdirAll(sDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sDir, "SKILL.md"), []byte("# "+s+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lucindDir := filepath.Join(root, ".lucind")
	if err := os.MkdirAll(lucindDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rootsYaml := "roots:\n  - " + skillsDir + "\n"
	if err := os.WriteFile(filepath.Join(lucindDir, "skill-roots.yaml"), []byte(rootsYaml), 0o644); err != nil {
		t.Fatal(err)
	}
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
		"legacy_main: true\n" +
		"expected_parent_sha: " + currentHead(t, primaryRoot) + "\n" +
		"---\n" +
		admittedWriteBody
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	origFactory := depsFactory
	defer func() { depsFactory = origFactory }()
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.ResolveCandidateIdentity = stubCandidateIdentity
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
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
		deps.CombineTree = func(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (string, string, error) {
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

// TestRunDispatchRegistersRunRowInLedger proves that `lucind-ai run` inserts
// a runs table row for its own dispatch. Before this fix, ledger.RegisterRun
// had zero production callers -- the runs table was written only by tests --
// so ledger.ListRuns always returned zero rows despite a ledger full of
// lanes and events carrying valid run_id values.
func TestRunDispatchRegistersRunRowInLedger(t *testing.T) {
	primaryRoot := initRepo(t)

	p1 := filepath.Join(primaryRoot, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"legacy_main: true\n" +
		"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
		"---\n" +
		admittedWriteBody
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	overrideDispatchDeps(t, testDoneExecutor{})

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	before := time.Now().UTC().Add(-time.Second)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--packet", p1}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	after := time.Now().UTC().Add(time.Second)

	runID := extractRunID(stdout.String())
	if runID == "" {
		t.Fatalf("stdout has no run id line; stdout = %q", stdout.String())
	}

	l, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	defer l.Close()

	got, err := l.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetRun(%q): %v -- lucind-ai run must register its own run row", runID, err)
	}
	if got.Status != string(lane.Done) {
		t.Errorf("run.Status = %q, want %q after every lane finished done", got.Status, lane.Done)
	}
	if got.LaneCount != 1 {
		t.Errorf("run.LaneCount = %d, want 1", got.LaneCount)
	}
	if got.StartedAt.Before(before) || got.StartedAt.After(after) {
		t.Errorf("run.StartedAt = %v, want within [%v, %v]", got.StartedAt, before, after)
	}
	if got.EndedAt == nil {
		t.Fatal("run.EndedAt is nil, want a terminal timestamp once dispatch concluded")
	}
	if got.EndedAt.Before(before) || got.EndedAt.After(after) {
		t.Errorf("run.EndedAt = %v, want within [%v, %v]", *got.EndedAt, before, after)
	}
	if got.PID != os.Getpid() {
		t.Errorf("run.PID = %d, want os.Getpid() %d", got.PID, os.Getpid())
	}
}

// writeAgyPacket writes a minimal legacy-mode packet naming the given
// executor, for tests exercising runDispatch's agy quota gate.
func writeAgyPacket(t *testing.T, dir, laneID, executorName string) string {
	t.Helper()
	path := filepath.Join(dir, laneID+".md")
	content := "---\n" +
		"id: " + laneID + "\n" +
		"executor: " + executorName + "\n" +
		"routed_by: test\n" +
		"legacy_main: true\n" +
		"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
		"---\n" +
		admittedWriteBody
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write packet: %v", err)
	}
	return path
}

const admittedWriteBody = "## Done criteria\n- Complete the requested work.\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nValidate it against .lucind/result.schema.json before writing.\nCommit the completed work.\n"

func currentHead(t *testing.T, repo string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestAdmitDispatchBatchPreservesManualAndCompilesTypedContract(t *testing.T) {
	repo := initRepo(t)
	head := currentHead(t, repo)
	manualBody := strings.ReplaceAll(admittedWriteBody, "\n", "\r\n")
	contract := &packetauthor.Contract{
		Version: packetauthor.ContractVersion, RouteIntent: "compiled route", Mode: packetauthor.ModeWrite,
		WritePaths: []string{"internal/compiled"}, ReadOnlyPaths: []string{"docs/input.md"}, Goal: "Compile safely",
		DoneCriteria: []string{"compiled criterion"}, HardStops: []string{"stop on ambiguity"},
		Result: packetauthor.ResultObligations{Path: ".lucind/result.json", Schema: ".lucind/result.schema.json"},
	}
	inputs := []dispatchAuthoringInput{
		{Packet: packet.Packet{ID: "manual", Executor: "agy", RoutedBy: "manual route", LegacyMain: true, ExpectedParentSHA: head, AllowedPaths: []string{"internal/manual"}, Body: manualBody}},
		{Packet: packet.Packet{ID: "compiled", Executor: "agy", RoutedBy: "compiled route", LegacyMain: true, ExpectedParentSHA: head}, Contract: contract},
	}

	got, err := admitDispatchBatch(context.Background(), repo, inputs)
	if err != nil {
		t.Fatalf("admitDispatchBatch() error = %v", err)
	}
	if got[0].Body != manualBody {
		t.Fatalf("manual body changed:\n got %q\nwant %q", got[0].Body, manualBody)
	}
	if got[1].Body == "" || got[1].Authoring == nil || len(got[1].Authoring.ContractJSON) == 0 {
		t.Fatalf("compiled packet lacks rendered body or frozen authoring input: %+v", got[1])
	}
	if !reflect.DeepEqual(got[1].AllowedPaths, []string{"internal/compiled"}) || !reflect.DeepEqual(got[1].ReadOnlyPaths, []string{"docs/input.md"}) {
		t.Fatalf("compiled scopes = write:%v read:%v", got[1].AllowedPaths, got[1].ReadOnlyPaths)
	}
}

func TestAdmitDispatchBatchReturnsDeterministicMixedDiagnostics(t *testing.T) {
	repo := initRepo(t)
	head := currentHead(t, repo)
	missingObligations := "## Done criteria\n- done\n\n## Return\nNo result instructions.\nCommit the work.\n"
	readOnlyConflict := "## Done criteria\n- inspect\n\n## Return\nWrite the result envelope to .lucind/result.json in this worktree.\nValidate it against .lucind/result.schema.json before writing.\nAfter you commit, report success.\n"
	inputs := []dispatchAuthoringInput{
		{Packet: packet.Packet{ID: "first", Executor: "agy", RoutedBy: "first", LegacyMain: true, ExpectedParentSHA: head, Body: missingObligations}},
		{Packet: packet.Packet{ID: "second", Executor: "agy", RoutedBy: "second", LegacyMain: true, ExpectedParentSHA: head, ReadOnly: true, ReadOnlyPaths: []string{"../secret"}, Body: readOnlyConflict}},
		{Packet: packet.Packet{ID: "third", Executor: "agy", RoutedBy: "third", Feature: "missing", ParentRef: "refs/heads/missing", BaseSHA: head, ExpectedParentSHA: head, Body: admittedWriteBody}},
	}

	_, err := admitDispatchBatch(context.Background(), repo, inputs)
	var diagnostics packetauthor.Diagnostics
	if !errors.As(err, &diagnostics) {
		t.Fatalf("error = %T %v, want packetauthor.Diagnostics", err, err)
	}
	want := []string{"0:PA_RESULT_PATH_MISSING", "0:PA_RESULT_SCHEMA_MISSING", "1:PA_MODE_COMMIT_CONFLICT", "1:PA_PATH_INVALID", "2:PA_TARGET_INCOMPLETE"}
	got := make([]string, len(diagnostics))
	for i, diagnostic := range diagnostics {
		got[i] = fmt.Sprintf("%d:%s", diagnostic.PacketIndex, diagnostic.Code)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics = %v, want %v", got, want)
	}
}

// TestWU6TypedAuthoringReachesAcceptanceWithShadowIsolation proves the full
// typed path without making the specialist authoritative: admission produces
// the packet consumed by Execute, Execute freezes evidence, Acceptance
// independently verifies the result and candidate diff, and shadow evidence
// is persisted beside (never inside) the canonical receipt.
func TestWU6TypedAuthoringReachesAcceptanceWithShadowIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("uses real Git worktrees and SQLite")
	}

	ctx := context.Background()
	repo := initRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".lucind/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "lucind-checks.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".gitignore", "lucind-checks.sh")
	runGit(t, repo, "commit", "-m", "add test check harness")
	head := currentHead(t, repo)

	contract := packetauthor.Contract{
		Version: packetauthor.ContractVersion, RouteIntent: "typed integration proof", Mode: packetauthor.ModeWrite,
		WritePaths: []string{"internal/typed-proof.txt"}, ReadOnlyPaths: []string{"docs/input.md"},
		Goal: "Prove typed authoring reaches acceptance.", DoneCriteria: []string{"typed criterion"},
		HardStops: []string{"stop on unsafe output"}, Result: packetauthor.ResultObligations{
			Path: ".lucind/result.json", Schema: ".lucind/result.schema.json",
		},
	}
	binding := packetauthor.TargetBinding{LegacyMain: &packetauthor.LegacyMainTarget{ExpectedParentSHA: head, LiveParentSHA: head}}
	compiled, err := packetauthor.Compile(contract, binding)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	admitted, err := admitDispatchBatch(ctx, repo, []dispatchAuthoringInput{{
		Packet:   packet.Packet{ID: "typed-lane", Executor: "agy", RoutedBy: "source route", LegacyMain: true, ExpectedParentSHA: head},
		Contract: &contract,
	}})
	if err != nil {
		t.Fatalf("admitDispatchBatch() error = %v", err)
	}
	if len(admitted) != 1 || admitted[0].Authoring == nil || admitted[0].Body != string(compiled.Body) || admitted[0].Authoring.Digest != compiled.Digest {
		t.Fatalf("admitted typed packet = %+v, want deterministic compiled artifact", admitted)
	}
	if !reflect.DeepEqual(admitted[0].ReadOnlyPaths, contract.ReadOnlyPaths) || !reflect.DeepEqual(admitted[0].AllowedPaths, contract.WritePaths) {
		t.Fatalf("admitted scopes = write:%v read:%v", admitted[0].AllowedPaths, admitted[0].ReadOnlyPaths)
	}

	l, err := ledger.Open(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	const runID = "wu6-typed-run"
	if err := l.RegisterRun(ctx, ledger.Run{RunID: runID, Status: string(lane.Running), LaneCount: 1, StartedAt: time.Now().UTC(), PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}

	dispatcher := &typedIntegrationExecutor{t: t, path: "internal/typed-proof.txt", criterion: contract.DoneCriteria[0], stop: contract.HardStops[0]}
	deps := productionDeps(runID, repo, l, time.Minute, 0)
	deps.LookupExecutor = func(string) (executor.Executor, error) { return dispatcher, nil }
	report, err := lucindrun.Execute(ctx, deps, admitted[0])
	if err != nil || report.Status != lane.Done {
		t.Fatalf("Execute() = %+v, %v", report, err)
	}
	if dispatcher.request.Prompt != string(compiled.Body) || !reflect.DeepEqual(dispatcher.request.ReadOnlyPaths, contract.ReadOnlyPaths) {
		t.Fatalf("executor request lost typed packet facts: prompt=%q read_only=%v", dispatcher.request.Prompt, dispatcher.request.ReadOnlyPaths)
	}

	candidate, err := l.GetLaneCandidate(ctx, runID, admitted[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.AuthoringEvidenceVersion != ledger.AuthoringEvidenceVersion || candidate.AuthoringEvidenceHash == "" || candidate.ResultHash == "" {
		t.Fatalf("candidate lacks frozen versioned evidence: %+v", candidate)
	}
	evidence, err := ledger.DecodeAuthoringEvidence(candidate.AuthoringEvidenceVersion, candidate.AuthoringEvidenceJSON, candidate.AuthoringEvidenceHash)
	if err != nil {
		t.Fatalf("DecodeAuthoringEvidence() error = %v", err)
	}
	if !reflect.DeepEqual(evidence.DoneCriteria, contract.DoneCriteria) || !reflect.DeepEqual(evidence.HardStops, contract.HardStops) || len(evidence.Changes) != 1 || evidence.Changes[0].Path != "internal/typed-proof.txt" || evidence.ResultHash != candidate.ResultHash {
		t.Fatalf("frozen correspondence = %+v", evidence)
	}
	packetPath := filepath.Join(repo, ".lucind", "packets", "typed-lane.md")
	if err := os.MkdirAll(filepath.Dir(packetPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, compiled.Body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(packetPath, []byte("edited after dispatch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(packetPath); err != nil {
		t.Fatal(err)
	} else if string(contents) == string(compiled.Body) {
		t.Fatal("source packet was not mutated after dispatch")
	}
	unchanged, err := l.GetLaneCandidate(ctx, runID, admitted[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.AuthoringEvidenceJSON != candidate.AuthoringEvidenceJSON || unchanged.AuthoringEvidenceHash != candidate.AuthoringEvidenceHash || unchanged.ResultHash != candidate.ResultHash {
		t.Fatal("frozen candidate evidence changed after source packet mutation")
	}

	// A result mutation with a recomputed result hash still cannot replace the
	// frozen evidence. Use a second immutable candidate row rather than
	// bypassing the ledger's append-only trigger.
	const tamperedRun, tamperedLane = "wu6-tampered-run", "tampered-lane"
	if err := l.RegisterRun(ctx, ledger.Run{RunID: tamperedRun, Status: string(lane.Running), LaneCount: 1, StartedAt: time.Now().UTC(), PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	if err := l.RegisterLane(ctx, ledger.Lane{RunID: tamperedRun, LaneID: tamperedLane, PacketID: tamperedLane, Executor: "agy", RoutingCondition: "mutation", Status: lane.Running}); err != nil {
		t.Fatal(err)
	}
	tampered := candidate
	tampered.RunID, tampered.LaneID, tampered.PacketID = tamperedRun, tamperedLane, tamperedLane
	mutatedResult := strings.Replace(candidate.ResultJSON, "typed-lane", tamperedLane, 1)
	mutatedResult = strings.Replace(mutatedResult, contract.DoneCriteria[0], "tampered criterion", 1)
	tampered.ResultJSON, tampered.ResultHash = mutatedResult, integrationHash("result:v1", mutatedResult)
	tamperedEvidence, err := ledger.DecodeAuthoringEvidence(candidate.AuthoringEvidenceVersion, candidate.AuthoringEvidenceJSON, candidate.AuthoringEvidenceHash)
	if err != nil {
		t.Fatal(err)
	}
	tamperedEvidence.ResultHash = tampered.ResultHash
	tampered.AuthoringEvidenceJSON, tampered.AuthoringEvidenceHash, err = ledger.FreezeAuthoringEvidence(tamperedEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.SetDoneCandidate(ctx, tampered); err != nil {
		t.Fatal(err)
	}
	if _, err := (accept.NewVerifier(repo, l)).Verify(ctx, accept.AcceptanceRequest{RunID: tamperedRun, LaneID: tamperedLane}); err == nil {
		t.Fatal("Acceptance accepted a result whose declaration diverges from frozen evidence")
	}

	receipt, err := (accept.NewVerifier(repo, l)).Verify(ctx, accept.AcceptanceRequest{RunID: runID, LaneID: admitted[0].ID})
	if err != nil {
		t.Fatalf("Acceptance.Verify() error = %v", err)
	}
	if receipt.Binding.BindingVersion != "binding:v2" || receipt.Binding.AuthoringEvidenceHash != candidate.AuthoringEvidenceHash || receipt.ResultHash != candidate.ResultHash {
		t.Fatalf("acceptance binding = %+v", receipt.Binding)
	}

	manual, err := packetauthor.Compile(contract, binding)
	if err != nil {
		t.Fatal(err)
	}
	runner := &wu6SpecialistRunner{contract: contract}
	shadow := packetauthor.Observe(ctx, manual, contract, runner, binding)
	if !shadow.Valid || !shadow.Equivalent || !shadow.DigestEqual || !shadow.ReplayStable || !shadow.ManualSelected || shadow.Warning != nil {
		t.Fatalf("shadow comparison = %+v", shadow)
	}
	diffJSON, err := json.Marshal(shadow.Differences)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.PersistShadowAttempt(ctx, ledger.ShadowAttempt{
		ID: "wu6-shadow", RunID: runID, LaneID: admitted[0].ID, InputHash: "typed-input", SpecialistIdentity: packetauthor.SpecialistAgentName,
		FailureClass: string(shadow.FailureClass), Valid: shadow.Valid, Equivalent: shadow.Equivalent, ReplayStable: shadow.ReplayStable,
		DiffJSON: string(diffJSON), ManualDigest: shadow.ManualDigest, SpecialistDigest: shadow.SpecialistDigest, LatencyMS: shadow.LatencyMS, CreatedAt: time.Now().UTC(),
	}, &ledger.ShadowReview{AttemptID: "wu6-shadow", Reviewer: "wu6", ReviewMS: 1, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	var shadowRows, receiptRows int
	if err := l.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM packet_author_shadow_attempts WHERE id = 'wu6-shadow'`).Scan(&shadowRows); err != nil {
		t.Fatal(err)
	}
	if err := l.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM acceptance_receipts WHERE receipt_id = ?`, receipt.ReceiptID).Scan(&receiptRows); err != nil {
		t.Fatal(err)
	}
	if shadowRows != 1 || receiptRows != 1 {
		t.Fatalf("isolated persistence counts = shadow:%d receipt:%d", shadowRows, receiptRows)
	}
	if string(manual.Body) != string(compiled.Body) || shadow.ManualDigest != manual.Digest {
		t.Fatal("shadow observation changed the canonical manual artifact")
	}
}

func TestWU6ShadowFallbackAndDisablePreserveManualCanonicality(t *testing.T) {
	contract := validPacketAuthorSource()
	binding := packetauthor.TargetBinding{LegacyMain: &packetauthor.LegacyMainTarget{
		ExpectedParentSHA: strings.Repeat("a", 40), LiveParentSHA: strings.Repeat("a", 40),
	}}
	manual, err := packetauthor.Compile(contract, binding)
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte(nil), manual.Body...)
	fallback := packetauthor.Observe(context.Background(), manual, contract, specialistRunnerFunc(func(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
		return packetauthor.SpecialistResponse{Identity: "default", Output: []byte(`{}`)}, nil
	}), binding)
	if fallback.FailureClass != packetauthor.ShadowFailureFallbackAgent || !fallback.ManualSelected || string(manual.Body) != string(body) {
		t.Fatalf("fallback shadow result = %+v; manual body changed", fallback)
	}
	disabled := packetauthor.Disabled(manual)
	if disabled.FailureClass != packetauthor.ShadowFailureDisabled || !disabled.ManualSelected || disabled.ManualDigest != manual.Digest || string(manual.Body) != string(body) {
		t.Fatalf("disabled shadow result = %+v; manual body changed", disabled)
	}
}

func TestWU6AdmissionRejectsInvalidReadOnlyInput(t *testing.T) {
	repo := initRepo(t)
	head := currentHead(t, repo)
	_, err := admitDispatchBatch(context.Background(), repo, []dispatchAuthoringInput{{
		Packet: packet.Packet{
			ID: "wu6-invalid-input", Executor: "agy", RoutedBy: "source route",
			LegacyMain: true, ExpectedParentSHA: head, ReadOnlyPaths: []string{"../secret"},
			Body: admittedWriteBody,
		},
	}})
	var diagnostics packetauthor.Diagnostics
	if !errors.As(err, &diagnostics) || len(diagnostics) != 1 || diagnostics[0].Code != packetauthor.CodePathInvalid {
		t.Fatalf("admission error = %T %v, want one %s diagnostic", err, err, packetauthor.CodePathInvalid)
	}
}

type typedIntegrationExecutor struct {
	t         *testing.T
	path      string
	criterion string
	stop      string
	request   executor.Request
}

func (e *typedIntegrationExecutor) Run(_ context.Context, request executor.Request) (executor.Outcome, error) {
	e.request = request
	proof := filepath.Join(request.WorktreePath, e.path)
	if err := os.MkdirAll(filepath.Dir(proof), 0o755); err != nil {
		return executor.Outcome{}, err
	}
	if err := os.WriteFile(proof, []byte("typed proof\n"), 0o644); err != nil {
		return executor.Outcome{}, err
	}
	runGit(e.t, request.WorktreePath, "add", e.path)
	runGit(e.t, request.WorktreePath, "commit", "-m", "typed authoring proof")
	candidate := currentHead(e.t, request.WorktreePath)
	envelope := result.Envelope{
		PacketID: "typed-lane", Status: "done", Summary: "typed result",
		HardStops:    []result.HardStop{{HardStop: e.stop, Fired: false}},
		FilesChanged: []result.FileChange{{Change: "created", Path: e.path}},
		DoneCriteria: []result.DoneCriterion{{Criterion: e.criterion, Met: true}}, Commit: candidate,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return executor.Outcome{}, err
	}
	if err := os.WriteFile(filepath.Join(request.WorktreePath, ".lucind", "result.json"), data, 0o644); err != nil {
		return executor.Outcome{}, err
	}
	return executor.Outcome{ExitCode: 0}, nil
}

func (*typedIntegrationExecutor) DefaultModel() string  { return "typed-test-model" }
func (*typedIntegrationExecutor) KnownModels() []string { return []string{"typed-test-model"} }

type wu6SpecialistRunner struct {
	contract packetauthor.Contract
}

func (r *wu6SpecialistRunner) Run(context.Context, packetauthor.SpecialistInvocation) (packetauthor.SpecialistResponse, error) {
	data, err := json.Marshal(struct {
		Version  string                `json:"version"`
		Contract packetauthor.Contract `json:"contract"`
	}{packetauthor.SpecialistOutputV1, r.contract})
	if err != nil {
		return packetauthor.SpecialistResponse{}, err
	}
	return packetauthor.SpecialistResponse{Identity: packetauthor.SpecialistAgentName, Output: data}, nil
}

func integrationHash(domain, value string) string {
	h := sha256.New()
	var size [8]byte
	for _, field := range []string{domain, value} {
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		_, _ = h.Write(size[:])
		_, _ = h.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func TestPrintAdmissionErrorOffersExplicitTargetRemediation(t *testing.T) {
	var stderr bytes.Buffer
	printAdmissionError(&stderr, packetauthor.Diagnostics{{
		PacketIndex: 0, ItemIndex: -1, Code: packetauthor.CodeTargetIncomplete,
		Field: "target", Message: "exactly one typed target is required",
	}})
	for _, want := range []string{"complete feature target", "--legacy-main", "--expected-parent-sha"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want remediation containing %q", stderr.String(), want)
		}
	}
}

type countingDispatchExecutor struct{ calls *int }

func (e countingDispatchExecutor) Run(context.Context, executor.Request) (executor.Outcome, error) {
	*e.calls++
	return executor.Outcome{}, errors.New("executor should not run for a rejected batch")
}
func (countingDispatchExecutor) DefaultModel() string  { return "test-model" }
func (countingDispatchExecutor) KnownModels() []string { return []string{"test-model"} }

func TestRunDispatchRejectsWholeBatchBeforeQuotaAllocationAndExecutor(t *testing.T) {
	repo := initRepo(t)
	head := currentHead(t, repo)
	writePacket := func(name, id, readOnlyPaths, body string) string {
		t.Helper()
		path := filepath.Join(repo, name)
		content := fmt.Sprintf("---\nid: %s\nexecutor: agy\nrouted_by: test\nlegacy_main: true\nexpected_parent_sha: %s\n%s---\n%s", id, head, readOnlyPaths, body)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	safe := writePacket("safe.md", "safe", "", admittedWriteBody)
	unsafe := writePacket("unsafe.md", "unsafe", "read_only_paths: [\"../secret\"]\n", "## Done criteria\n- done\n\n## Return\nCommit the work.\n")

	quotaCalls, allocationCalls, executorCalls := 0, 0, 0
	origGate, origFactory := ensureAgyQuota, depsFactory
	t.Cleanup(func() { ensureAgyQuota, depsFactory = origGate, origFactory })
	ensureAgyQuota = func(context.Context, float64) error { quotaCalls++; return nil }
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.CreateWorktree = func(context.Context, string, string, string, string) (worktree.Worktree, error) {
			allocationCalls++
			return worktree.Worktree{Path: t.TempDir(), Branch: "lucind/test", BaseSHA: head}, nil
		}
		deps.LookupExecutor = func(string) (executor.Executor, error) {
			return countingDispatchExecutor{calls: &executorCalls}, nil
		}
		return deps
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--packet", safe, "--packet", unsafe}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if quotaCalls != 0 || allocationCalls != 0 || executorCalls != 0 {
		t.Fatalf("side effects = quota:%d allocation:%d executor:%d, want all zero", quotaCalls, allocationCalls, executorCalls)
	}
	if !strings.Contains(stderr.String(), "packet[1] PA_RESULT_PATH_MISSING") || !strings.Contains(stderr.String(), "packet[1] PA_RESULT_SCHEMA_MISSING") {
		t.Fatalf("stderr = %q, want deterministic packet-indexed diagnostics", stderr.String())
	}
	if !strings.Contains(stderr.String(), "packet[1] PA_PATH_INVALID read_only_paths[0]") {
		t.Fatalf("stderr = %q, want indexed malformed input diagnostic", stderr.String())
	}
}

func TestRunDispatchRejectsStaleCompiledBodyBeforeAllocation(t *testing.T) {
	repo := initRepo(t)
	head := currentHead(t, repo)
	stale := strings.Repeat("f", 40)
	if stale == head {
		stale = strings.Repeat("e", 40)
	}
	artifact, err := packetauthor.Compile(packetauthor.Contract{
		Version: packetauthor.ContractVersion, RouteIntent: "compiled route", Mode: packetauthor.ModeWrite,
		WritePaths: []string{"internal/compiled"}, Goal: "Compiled goal", DoneCriteria: []string{"done"}, HardStops: []string{"stop"},
		Result: packetauthor.ResultObligations{Path: ".lucind/result.json", Schema: ".lucind/result.schema.json"},
	}, packetauthor.TargetBinding{LegacyMain: &packetauthor.LegacyMainTarget{ExpectedParentSHA: head, LiveParentSHA: head}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "compiled.md")
	content := fmt.Sprintf("---\nid: compiled\nexecutor: agy\nrouted_by: compiled route\nlegacy_main: true\nexpected_parent_sha: %s\nallowed_paths: [\"internal/compiled\"]\n---\n%s", stale, artifact.Body)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	allocations := 0
	origFactory := depsFactory
	t.Cleanup(func() { depsFactory = origFactory })
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		allocations++
		return origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
	}
	cwd, _ := os.Getwd()
	_ = os.Chdir(repo)
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--packet", path, "--min-quota", "0"}, &stdout, &stderr)
	if code != 1 || allocations != 0 || !strings.Contains(stderr.String(), packetauthor.CodeTargetStale) {
		t.Fatalf("run = exit:%d allocations:%d stderr:%q, want stale rejection before allocation", code, allocations, stderr.String())
	}
}

// TestRunDispatchGatesOnAgyQuotaForAgyExecutorBatch proves runDispatch calls
// the agy quota gate (ensureAgyQuota), with the --min-quota flag's value,
// before dispatching a batch that includes an agy-executed packet. This is
// the wave-level gate: --packet is repeated once per lane in the same
// invocation, so this hook fires once per wave, never once per lane.
func TestRunDispatchGatesOnAgyQuotaForAgyExecutorBatch(t *testing.T) {
	primaryRoot := initRepo(t)
	p1 := writeAgyPacket(t, primaryRoot, "lane-1", "agy")
	overrideDispatchDeps(t, testDoneExecutor{})

	var gateCalled bool
	var gateMinFraction float64
	origGate := ensureAgyQuota
	ensureAgyQuota = func(ctx context.Context, minFraction float64) error {
		gateCalled = true
		gateMinFraction = minFraction
		return nil
	}
	t.Cleanup(func() { ensureAgyQuota = origGate })

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
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !gateCalled {
		t.Fatal("ensureAgyQuota was not called for a batch containing an agy-executed packet")
	}
	if gateMinFraction != defaultMinQuota {
		t.Errorf("ensureAgyQuota minFraction = %v, want default %v", gateMinFraction, defaultMinQuota)
	}
}

// TestRunDispatchSkipsAgyQuotaGateForNonAgyBatch proves the gate is skipped
// entirely when no packet in the batch names the agy executor, since the
// pooled-account quota it checks is meaningless for other executors' billing.
func TestRunDispatchSkipsAgyQuotaGateForNonAgyBatch(t *testing.T) {
	primaryRoot := initRepo(t)
	p1 := writeAgyPacket(t, primaryRoot, "lane-1", "cursor-agent")
	overrideDispatchDeps(t, testDoneExecutor{})

	var gateCalled bool
	origGate := ensureAgyQuota
	ensureAgyQuota = func(ctx context.Context, minFraction float64) error {
		gateCalled = true
		return nil
	}
	t.Cleanup(func() { ensureAgyQuota = origGate })

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
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if gateCalled {
		t.Error("ensureAgyQuota was called for a batch with no agy-executed packet, want skipped")
	}
}

// TestRunDispatchBlocksWaveWhenAgyQuotaGateFails proves a failing quota gate
// blocks the whole wave before any ledger or worktree side effect -- no lane
// dispatches, and no run row is registered, matching the same "fail before
// ExecuteBatch" contract as the batch-disjointness check above it.
func TestRunDispatchBlocksWaveWhenAgyQuotaGateFails(t *testing.T) {
	primaryRoot := initRepo(t)
	p1 := writeAgyPacket(t, primaryRoot, "lane-1", "agy")
	overrideDispatchDeps(t, testDoneExecutor{})

	const gateErr = "all pooled accounts below the quota minimum"
	origGate := ensureAgyQuota
	ensureAgyQuota = func(ctx context.Context, minFraction float64) error {
		return errors.New(gateErr)
	}
	t.Cleanup(func() { ensureAgyQuota = origGate })

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
	if code != 1 {
		t.Fatalf("run exit code = %d, want 1 when the quota gate fails", code)
	}
	if !strings.Contains(stderr.String(), gateErr) {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), gateErr)
	}
	if extractRunID(stdout.String()) != "" {
		t.Errorf("stdout = %q, want no run id line: the gate must block before RegisterRun", stdout.String())
	}
}

// TestRunDispatchMinQuotaFlagOverridesDefault proves --min-quota reaches the
// gate as-given.
func TestRunDispatchMinQuotaFlagOverridesDefault(t *testing.T) {
	primaryRoot := initRepo(t)
	p1 := writeAgyPacket(t, primaryRoot, "lane-1", "agy")
	overrideDispatchDeps(t, testDoneExecutor{})

	var gateMinFraction float64
	origGate := ensureAgyQuota
	ensureAgyQuota = func(ctx context.Context, minFraction float64) error {
		gateMinFraction = minFraction
		return nil
	}
	t.Cleanup(func() { ensureAgyQuota = origGate })

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--packet", p1, "--min-quota", "0.25"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if gateMinFraction != 0.25 {
		t.Errorf("ensureAgyQuota minFraction = %v, want 0.25", gateMinFraction)
	}
}

// TestRunDispatchMinQuotaZeroDisablesGate proves --min-quota 0 is an
// explicit escape hatch: the gate is skipped even for an agy-executor batch.
func TestRunDispatchMinQuotaZeroDisablesGate(t *testing.T) {
	primaryRoot := initRepo(t)
	p1 := writeAgyPacket(t, primaryRoot, "lane-1", "agy")
	overrideDispatchDeps(t, testDoneExecutor{})

	var gateCalled bool
	origGate := ensureAgyQuota
	ensureAgyQuota = func(ctx context.Context, minFraction float64) error {
		gateCalled = true
		return nil
	}
	t.Cleanup(func() { ensureAgyQuota = origGate })

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--packet", p1, "--min-quota", "0"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if gateCalled {
		t.Error("ensureAgyQuota was called with --min-quota 0, want the gate disabled")
	}
}

// TestRunDispatchRecordsFailedStatusWhenLaneDoesNotFinishDone proves the run
// row's terminal status tracks the actual outcome instead of being left at
// "running" forever when a lane does not finish done. A run row stuck at
// "running" would overcount serve.Model.GetOverview's ActiveRunCount
// indefinitely -- the exact "phantom active dispatch" state the console
// must never show.
func TestRunDispatchRecordsFailedStatusWhenLaneDoesNotFinishDone(t *testing.T) {
	primaryRoot := initRepo(t)

	p1 := filepath.Join(primaryRoot, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"legacy_main: true\n" +
		"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
		"---\n" +
		admittedWriteBody
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	overrideDispatchDeps(t, testDoneExecutor{
		exitCode: 1,
		envelope: `{"packet_id": "lane-1", "status": "blocked", "summary": "blocked", "hard_stops": [{"hard_stop": "stop", "fired": true, "note": "test block"}]}`,
	})

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
	if code == 0 {
		t.Fatalf("run exit code = 0, want non-zero after a blocked lane; stdout = %q", stdout.String())
	}

	runID := extractRunID(stdout.String())
	if runID == "" {
		t.Fatalf("stdout has no run id line; stdout = %q", stdout.String())
	}

	l, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open: %v", err)
	}
	defer l.Close()

	got, err := l.GetRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("GetRun(%q): %v", runID, err)
	}
	if got.Status == string(lane.Running) {
		t.Error(`run.Status is still "running" after the dispatch concluded, want a terminal status`)
	}
	if got.EndedAt == nil {
		t.Error("run.EndedAt is nil, want a terminal timestamp even on a non-done outcome")
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
		"legacy_main: true\n" +
		"expected_parent_sha: " + currentHead(t, primaryRoot) + "\n" +
		"---\n" +
		admittedWriteBody
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	origFactory := depsFactory
	defer func() { depsFactory = origFactory }()
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.ResolveCandidateIdentity = stubCandidateIdentity
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
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
		deps.CombineTree = func(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (string, string, error) {
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
		skills := req.RequiredSkills
		if skills == nil {
			skills = []string{}
		}
		skillsJSON, _ := json.Marshal(skills)
		pid := filepath.Base(req.WorktreePath)
		if pid == "" || pid == "." {
			pid = "lane-1"
		}
		envelope = fmt.Sprintf(`{"packet_id": %q, "status": "done", "summary": "done", "hard_stops": [], "skills_loaded": %s}`, pid, string(skillsJSON))
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
	rootBody := "## Goal\n\nRoot packet work that the leaf depends on.\n\n" + admittedWriteBody
	leafBody := "## Goal\n\nLeaf packet work that runs after the root.\n\n" + admittedWriteBody
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
    legacy_main: true
    expected_parent_sha: 1111111111111111111111111111111111111111
    allowed_paths:
      - internal/root
    depends_on: []
    body_path: bodies/apply-root.md
  - id: apply-leaf
    executor: agy
    routed_by: leaf depends on root
    legacy_main: true
    expected_parent_sha: 1111111111111111111111111111111111111111
    allowed_paths:
      - internal/leaf
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
	origResolver := resolveAdmissionRefSHA
	t.Cleanup(func() { resolveAdmissionRefSHA = origResolver })
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return "1111111111111111111111111111111111111111", nil
	}
	origFactory := depsFactory
	t.Cleanup(func() { depsFactory = origFactory })
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.ResolveCandidateIdentity = stubCandidateIdentity
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
		deps.CombineTree = func(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (string, string, error) {
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

func stubCandidateIdentity(context.Context, string, string, string) (lucindrun.CandidateIdentity, error) {
	return lucindrun.CandidateIdentity{
		BaseCommit: "base-commit", BaseTree: "base-tree", CandidateCommit: "candidate-commit", CandidateTree: "candidate-tree",
	}, nil
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
	if !strings.Contains(outStr, "resolved root:") {
		t.Errorf("stdout = %q, want it to contain 'resolved root:'", outStr)
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
			admittedWriteBody
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
		// This used to surface as a per-lane "status: failed" after every
		// worktree already existed, with no reason printed anywhere. Deriving
		// the batch's integration target before dispatch moves the same
		// rejection to stderr, before any quota is spent, and names the exit.
		if !strings.Contains(stderr.String(), "--legacy-main") {
			t.Errorf("expected stderr to name the --legacy-main exit, got:\n%s", stderr.String())
		}
		if strings.Contains(stdout.String(), "status:") {
			t.Errorf("expected no lane to dispatch at all, got:\n%s", stdout.String())
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
			admittedWriteBody
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

func TestFeatureRenewCLI(t *testing.T) {
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
	_, err = featSvc.Create(context.Background(), "feat-renew", "refs/heads/feature-renew", "1111111111111111111111111111111111111111")
	if err != nil {
		t.Fatalf("featSvc.Create error = %v", err)
	}

	lease, err := featSvc.AcquireLease(context.Background(), "feat-renew", "renew-owner", 5*time.Minute)
	if err != nil {
		t.Fatalf("featSvc.AcquireLease error = %v", err)
	}
	ledg.Close()

	t.Run("renews an existing lease and reports a later expiry", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"feature", "renew",
			"--id", "feat-renew",
			"--owner", "renew-owner",
			"--fence", fmt.Sprintf("%d", lease.Fence),
			"--ttl", "10m",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("feature renew exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "feat-renew") {
			t.Errorf("stdout = %q, want feature id feat-renew", stdout.String())
		}
		if !strings.Contains(stdout.String(), "renew-owner") {
			t.Errorf("stdout = %q, want owner renew-owner", stdout.String())
		}

		ledg2, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg2.Close()

		featSvc := feature.NewService(ledg2)
		newLease, err := featSvc.GetLease(context.Background(), "feat-renew")
		if err != nil {
			t.Fatalf("featSvc.GetLease error = %v", err)
		}
		if !newLease.ExpiresAt.After(lease.ExpiresAt) {
			t.Errorf("renewed expires_at = %v, want it after original %v", newLease.ExpiresAt, lease.ExpiresAt)
		}
	})

	t.Run("wrong owner is rejected as stale with non-zero exit", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"feature", "renew",
			"--id", "feat-renew",
			"--owner", "someone-else",
			"--fence", fmt.Sprintf("%d", lease.Fence),
		}, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("feature renew with wrong owner exit code = 0, want non-zero")
		}
		if stderr.String() == "" {
			t.Errorf("stderr is empty, want an error surfaced")
		}
	})

	t.Run("wrong fence is rejected as stale with non-zero exit", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"feature", "renew",
			"--id", "feat-renew",
			"--owner", "renew-owner",
			"--fence", fmt.Sprintf("%d", lease.Fence+99),
		}, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("feature renew with wrong fence exit code = 0, want non-zero")
		}
		if stderr.String() == "" {
			t.Errorf("stderr is empty, want an error surfaced")
		}
	})

	t.Run("missing required flags fails with usage/error", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
			want string
		}{
			{"missing id", []string{"feature", "renew", "--owner", "o", "--fence", "1"}, "--id"},
			{"missing owner", []string{"feature", "renew", "--id", "feat-renew", "--fence", "1"}, "--owner"},
			{"missing fence", []string{"feature", "renew", "--id", "feat-renew", "--owner", "o"}, "--fence"},
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

// TestFeatureDisableCLI covers "lucind-ai feature disable": the supported way
// to retire a feature registered against a base that turned out to be
// unusable, so it stops counting as active and its ID can be reused with a
// corrected anchor -- see internal/feature.Service.reactivateDisabled.
func TestFeatureDisableCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	redBaseSHA := "1111111111111111111111111111111111111111"
	greenBaseSHA := "4444444444444444444444444444444444444444"

	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}
	featSvc := feature.NewService(ledg)
	if _, err := featSvc.Create(context.Background(), "feat-stale", "refs/heads/feature-stale", redBaseSHA); err != nil {
		t.Fatalf("featSvc.Create error = %v", err)
	}
	ledg.Close()

	t.Run("disables an active feature", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "disable", "--id", "feat-stale"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("feature disable exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "feat-stale") || !strings.Contains(stdout.String(), "disabled") {
			t.Errorf("stdout = %q, want it to report feat-stale disabled", stdout.String())
		}

		ledg2, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg2.Close()

		f, err := feature.NewService(ledg2).Get(context.Background(), "feat-stale")
		if err != nil {
			t.Fatalf("Get after disable error = %v", err)
		}
		if f.Status != feature.StatusDisabled {
			t.Errorf("f.Status = %v, want disabled", f.Status)
		}

		active, err := ledg2.ActiveFeatures(context.Background())
		if err != nil {
			t.Fatalf("ActiveFeatures error = %v", err)
		}
		for _, af := range active {
			if af.ID == "feat-stale" {
				t.Errorf("ActiveFeatures still lists disabled feature %q", af.ID)
			}
		}
	})

	t.Run("feature create re-anchors and reactivates the disabled id with a new base", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"feature", "create",
			"--id", "feat-stale",
			"--parent", "refs/heads/feature-stale-corrected",
			"--base-sha", greenBaseSHA,
		}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("feature create (re-anchor) exit code = %d, want 0; stderr = %q", code, stderr.String())
		}

		ledg2, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg2.Close()

		f, err := feature.NewService(ledg2).Get(context.Background(), "feat-stale")
		if err != nil {
			t.Fatalf("Get after re-anchor error = %v", err)
		}
		if f.Status != feature.StatusActive || f.BaseSHA != greenBaseSHA || f.ParentRef != "refs/heads/feature-stale-corrected" {
			t.Errorf("reanchored feature = %+v, want active with base_sha %s", f, greenBaseSHA)
		}
	})

	t.Run("disabling an unknown id fails with non-zero exit", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "disable", "--id", "feat-does-not-exist"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("feature disable on unknown id exit code = 0, want non-zero")
		}
		if stderr.String() == "" {
			t.Errorf("stderr is empty, want an error surfaced")
		}
	})

	t.Run("missing --id fails with usage/error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"feature", "disable"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("feature disable without --id exit code = 0, want non-zero")
		}
		if !strings.Contains(stderr.String(), "--id") {
			t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "--id")
		}
	})
}

func TestWorktreeCleanupCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}

	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	t.Run("cleans up an existing stale worktree", func(t *testing.T) {
		wt, err := worktree.Create(context.Background(), primaryRoot, "stale-lane")
		if err != nil {
			t.Fatalf("worktree.Create error = %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"worktree", "cleanup", "--lane", "stale-lane"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("worktree cleanup exit code = %d, want 0; stderr = %q", code, stderr.String())
		}

		if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
			t.Errorf("os.Stat(%q) err = %v, want os.IsNotExist", wt.Path, err)
		}
	})

	t.Run("unforced dirty cleanup exits 1 with diagnostics and preserves files", func(t *testing.T) {
		wt, err := worktree.Create(context.Background(), primaryRoot, "dirty-lane")
		if err != nil {
			t.Fatalf("worktree.Create error = %v", err)
		}

		dirtyFile := filepath.Join(wt.Path, "wip.txt")
		if err := os.WriteFile(dirtyFile, []byte("uncommitted work in progress\n"), 0o644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"worktree", "cleanup", "--lane", "dirty-lane"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("worktree cleanup without --force on dirty tree exit code = %d, want 1; stderr = %q", code, stderr.String())
		}

		stderrStr := stderr.String()
		if !strings.Contains(stderrStr, "troubleshooting.md") {
			t.Errorf("stderr = %q, want diagnostic pointer to troubleshooting.md", stderrStr)
		}
		if !strings.Contains(stderrStr, "--force") {
			t.Errorf("stderr = %q, want mention of --force flag", stderrStr)
		}
		if !strings.Contains(stderrStr, "git") || !strings.Contains(stderrStr, "diff") {
			t.Errorf("stderr = %q, want git diff / status inspection commands", stderrStr)
		}

		if _, err := os.Stat(dirtyFile); err != nil {
			t.Errorf("dirty file was deleted: %v", err)
		}
		if !worktree.IsLinkedWorktree(wt.Path) {
			t.Errorf("worktree was removed despite being dirty")
		}
	})

	t.Run("dirty cleanup with --force exits 0 and removes files", func(t *testing.T) {
		wtPath := filepath.Join(filepath.Dir(primaryRoot), filepath.Base(primaryRoot)+"-worktrees", "dirty-lane")
		if !worktree.IsLinkedWorktree(wtPath) {
			t.Fatalf("dirty-lane worktree does not exist before --force cleanup")
		}

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"worktree", "cleanup", "--lane", "dirty-lane", "--force"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("worktree cleanup with --force exit code = %d, want 0; stderr = %q", code, stderr.String())
		}

		if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
			t.Errorf("os.Stat(%q) err = %v, want os.IsNotExist after forced cleanup", wtPath, err)
		}
	})

	t.Run("dirty cleanup with -f exits 0 and removes files", func(t *testing.T) {
		wt, err := worktree.Create(context.Background(), primaryRoot, "dirty-lane-short")
		if err != nil {
			t.Fatalf("worktree.Create error = %v", err)
		}

		dirtyFile := filepath.Join(wt.Path, "wip.txt")
		if err := os.WriteFile(dirtyFile, []byte("uncommitted work\n"), 0o644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"worktree", "cleanup", "--lane", "dirty-lane-short", "-f"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("worktree cleanup with -f exit code = %d, want 0; stderr = %q", code, stderr.String())
		}

		if _, err := os.Stat(wt.Path); !os.IsNotExist(err) {
			t.Errorf("os.Stat(%q) err = %v, want os.IsNotExist after -f cleanup", wt.Path, err)
		}
	})

	t.Run("nonexistent lane is idempotent and still exits 0", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"worktree", "cleanup", "--lane", "never-existed"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("worktree cleanup on nonexistent lane exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
	})

	t.Run("missing --lane flag is usage error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"worktree", "cleanup"}, &stdout, &stderr)
		if code == 0 {
			t.Fatalf("worktree cleanup without --lane exit code = 0, want non-zero")
		}
		if !strings.Contains(stderr.String(), "--lane") {
			t.Errorf("stderr = %q, want --lane mentioned", stderr.String())
		}
	})

	t.Run("refuses invocation from inside linked worktree", func(t *testing.T) {
		wt, err := worktree.Create(context.Background(), primaryRoot, "inner-wt")
		if err != nil {
			t.Fatalf("worktree.Create error = %v", err)
		}
		defer func() {
			_ = os.Chdir(primaryRoot)
			_ = worktree.Remove(context.Background(), primaryRoot, wt.Path, true)
		}()

		if err := os.Chdir(wt.Path); err != nil {
			t.Fatalf("Chdir error = %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{"worktree", "cleanup", "--lane", "inner-wt"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("worktree cleanup inside linked worktree exit code = %d, want 1; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "refusing to run from inside a linked worktree") {
			t.Errorf("stderr = %q, want linked worktree refusal error", stderr.String())
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

// TestReconcileResolveCLI covers "lucind-ai reconcile candidate resolve": the human-in-the-loop
// path that closes the reconciliation loop -- see internal/run/gate_test.go's
// TestApprovedIntegratedCandidateUnblocksPromotion for how a registered resolution actually
// clears a blocked feature's promotion.
func TestReconcileResolveCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	// A second real commit to stand in for the human's resolved commit -- reconcile resolve
	// must verify --sha resolves to a real commit in the primary repo before accepting it.
	if err := os.WriteFile(filepath.Join(primaryRoot, "resolved.txt"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(resolved.txt) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "resolved.txt")
	runGit(t, primaryRoot, "commit", "-m", "resolved commit")
	resolvedSHA := strings.TrimSpace(string(func() []byte {
		out, err := exec.Command("git", "-C", primaryRoot, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatalf("git rev-parse HEAD error = %v", err)
		}
		return out
	}()))

	setupRequest := func(t *testing.T, id string) reconcile.Candidate {
		t.Helper()
		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg.Close()

		featSvc := feature.NewService(ledg)
		_, _ = featSvc.Create(context.Background(), "feat-resolve-src-"+id, "refs/heads/source-"+id, "1111111111111111111111111111111111111111")
		_, _ = featSvc.Create(context.Background(), "feat-resolve-tgt-"+id, "refs/heads/target-"+id, "2222222222222222222222222222222222222222")

		reconcileSvc := reconcile.NewService(ledg)
		req, err := reconcileSvc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
			ID:            "req-resolve-" + id,
			FeatureID:     "feat-resolve-tgt-" + id,
			SourceFeature: "feat-resolve-src-" + id,
			SourceParent:  "refs/heads/source-" + id,
			TargetFeature: "feat-resolve-tgt-" + id,
			TargetParent:  "refs/heads/target-" + id,
			SourceSHA:     "1111111111111111111111111111111111111111",
			TargetSHA:     "2222222222222222222222222222222222222222",
			Evidence: &overlap.Evidence{
				Version:     "1.0",
				BaseSHA:     "0000000000000000000000000000000000000000",
				FeatureASHA: "1111111111111111111111111111111111111111",
				FeatureBSHA: "2222222222222222222222222222222222222222",
				Class:       overlap.ClassRequired,
				Signals:     overlap.Signals{ConflictPaths: []string{"pkg/conflict.go"}},
			},
			TTL: 15 * time.Minute,
		})
		if err != nil {
			t.Fatalf("CreateRequest error = %v", err)
		}
		_, cand, err := reconcileSvc.Approve(context.Background(), reconcile.ApproveParams{
			RequestID:     req.ID,
			SourceFeature: "feat-resolve-src-" + id,
			TargetFeature: "feat-resolve-tgt-" + id,
			Actor:         "setup",
		})
		if err != nil {
			t.Fatalf("Approve error = %v", err)
		}
		return cand
	}

	t.Run("resolving a running candidate with a real sha marks it integrated", func(t *testing.T) {
		cand := setupRequest(t, "ok")

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "resolve",
			"--candidate", cand.ID,
			"--sha", resolvedSHA,
			"--actor", "test-actor",
		}, &stdout, &stderr)

		if code != 0 {
			t.Fatalf("reconcile resolve exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), resolvedSHA) {
			t.Errorf("stdout = %q, want it to contain the resolved sha %q", stdout.String(), resolvedSHA)
		}

		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg.Close()

		got, err := ledg.ReconciliationCandidate(context.Background(), cand.ID)
		if err != nil {
			t.Fatalf("ReconciliationCandidate error = %v", err)
		}
		if got.Status != string(reconcile.CandidateStatusIntegrated) {
			t.Errorf("candidate status = %q, want %q", got.Status, reconcile.CandidateStatusIntegrated)
		}
		if got.CandidateSHA != resolvedSHA {
			t.Errorf("candidate_sha = %q, want %q", got.CandidateSHA, resolvedSHA)
		}
	})

	t.Run("resolving an already-integrated candidate again is rejected", func(t *testing.T) {
		cand := setupRequest(t, "twice")

		var stdout1, stderr1 bytes.Buffer
		if code := run(context.Background(), []string{
			"reconcile", "resolve", "--candidate", cand.ID, "--sha", resolvedSHA, "--actor", "test-actor",
		}, &stdout1, &stderr1); code != 0 {
			t.Fatalf("first reconcile resolve exit code = %d, want 0; stderr = %q", code, stderr1.String())
		}

		var stdout2, stderr2 bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "resolve", "--candidate", cand.ID, "--sha", resolvedSHA, "--actor", "test-actor",
		}, &stdout2, &stderr2)
		if code == 0 {
			t.Fatalf("second reconcile resolve exit code = 0, want non-zero (candidate no longer running)")
		}
	})

	t.Run("a sha that does not resolve to a real commit is rejected and the candidate stays running", func(t *testing.T) {
		cand := setupRequest(t, "badsha")

		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"reconcile", "resolve",
			"--candidate", cand.ID,
			"--sha", "0000000000000000000000000000000000000000",
			"--actor", "test-actor",
		}, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("reconcile resolve with a nonexistent sha exit code = 0, want non-zero")
		}

		ledg, err := ledger.Open(context.Background(), primaryRoot)
		if err != nil {
			t.Fatalf("ledger.Open error = %v", err)
		}
		defer ledg.Close()

		got, err := ledg.ReconciliationCandidate(context.Background(), cand.ID)
		if err != nil {
			t.Fatalf("ReconciliationCandidate error = %v", err)
		}
		if got.Status != string(reconcile.CandidateStatusRunning) {
			t.Errorf("candidate status = %q, want still %q after a rejected sha", got.Status, reconcile.CandidateStatusRunning)
		}
	})

	t.Run("missing required flags fails with usage/error", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
			want string
		}{
			{"missing candidate", []string{"reconcile", "resolve", "--sha", resolvedSHA}, "--candidate"},
			{"missing sha", []string{"reconcile", "resolve", "--candidate", "cand-1"}, "--sha"},
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

func TestReconcileResolve_RejectsLinkedWorktree(t *testing.T) {
	primaryRoot := initRepo(t)
	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, primaryRoot, "worktree", "add", linked, "-b", "linked-resolve")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(linked); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{
		"reconcile", "resolve",
		"--candidate", "cand-linked",
		"--sha", "HEAD",
		"--actor", "test-actor",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("reconcile resolve from linked worktree exit code = 0, want non-zero; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "linked worktree") {
		t.Errorf("stderr = %q, want linked worktree refusal", stderr.String())
	}
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

	t.Run("top-level renew is no longer a valid subcommand", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), []string{
			"renew",
			"--request", "does-not-matter",
		}, &stdout, &stderr)

		if code == 0 {
			t.Fatalf("top-level renew exit code = %d, want non-zero (unknown subcommand)", code)
		}
		if !strings.Contains(stderr.String(), "unknown subcommand") {
			t.Errorf("stderr = %q, want it to report an unknown subcommand", stderr.String())
		}
	})
}

// featureDispatchDeps stubs the compare-and-swap promotion path so a
// feature-targeted dispatch can be driven end to end without a second real
// feature branch in the fixture repository.
func featureDispatchDeps(t *testing.T, promoted *[]string) func(string, string, *ledger.Ledger, time.Duration, time.Duration) lucindrun.Deps {
	t.Helper()
	origResolver := resolveAdmissionRefSHA
	t.Cleanup(func() { resolveAdmissionRefSHA = origResolver })
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return "1111111111111111111111111111111111111111", nil
	}
	origFactory := depsFactory
	return func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.ResolveCandidateIdentity = stubCandidateIdentity
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
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
		deps.CombineTree = func(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (string, string, error) {
			return t.TempDir(), "integration-branch", nil
		}
		deps.RunChecks = func(ctx context.Context, worktreePath string) (bool, string, error) {
			return true, "", nil
		}
		deps.ResolveRefSHA = func(ctx context.Context, primaryRoot, ref string) (string, error) {
			return "1111111111111111111111111111111111111111", nil
		}
		deps.ResolveCandidateSHA = func(ctx context.Context, primaryRoot, worktreePath, branch string) (string, error) {
			return "2222222222222222222222222222222222222222", nil
		}
		deps.PromoteCAS = func(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
			*promoted = append(*promoted, parentRef+" "+expectedSHA+"->"+candidateSHA)
			return nil
		}
		deps.PromoteTarget = func(ctx context.Context, primaryRoot, integrationBranch string) error {
			t.Errorf("PromoteTarget called on a feature-targeted batch; promotion must go through the compare-and-swap attempt path, never an ff-merge into the primary checkout")
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
}

// A feature-targeted dispatch must drive the durable attempt state machine:
// it records an integration_attempts row, promotes by compare-and-swap on the
// named parent ref, and never ff-merges into whatever the primary checkout
// happens to have checked out.
func TestRunDispatchFeatureBatchRecordsIntegrationAttempt(t *testing.T) {
	primaryRoot := initRepo(t)

	p1 := filepath.Join(primaryRoot, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-feat-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"feature: feat-alpha\n" +
		"parent_ref: refs/heads/feature-alpha\n" +
		"base_sha: 1111111111111111111111111111111111111111\n" +
		"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
		"---\n" +
		admittedWriteBody
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	var promoted []string
	origFactory := depsFactory
	t.Cleanup(func() { depsFactory = origFactory })
	depsFactory = featureDispatchDeps(t, &promoted)

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

	if len(promoted) != 1 {
		t.Fatalf("PromoteCAS calls = %v, want exactly one compare-and-swap promotion", promoted)
	}
	want := "refs/heads/feature-alpha 1111111111111111111111111111111111111111->2222222222222222222222222222222222222222"
	if promoted[0] != want {
		t.Errorf("PromoteCAS call = %q, want %q", promoted[0], want)
	}

	if !strings.Contains(stdout.String(), "attempt:") {
		t.Errorf("stdout = %q, want it to name the integration attempt so `feature recover --attempt <id>` has an id to use", stdout.String())
	}

	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open() error = %v", err)
	}
	defer ledg.Close()

	var count int
	if err := ledg.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM integration_attempts WHERE feature_id = ? AND status = 'promoted'`, "feat-alpha").Scan(&count); err != nil {
		t.Fatalf("query integration_attempts: %v", err)
	}
	if count != 1 {
		t.Errorf("promoted integration_attempts rows = %d, want 1", count)
	}
}

// One batch promotes onto one parent. A batch naming two features is rejected
// before any lane dispatches, so no quota is burned on work that has nowhere
// coherent to land.
func TestRunDispatchRejectsMixedFeatureTargets(t *testing.T) {
	primaryRoot := initRepo(t)

	writePacket := func(name, laneID, featureID string) string {
		path := filepath.Join(primaryRoot, name)
		content := "---\n" +
			"id: " + laneID + "\n" +
			"executor: agy\n" +
			"routed_by: test\n" +
			"feature: " + featureID + "\n" +
			"parent_ref: refs/heads/" + featureID + "\n" +
			"base_sha: 1111111111111111111111111111111111111111\n" +
			"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
			"---\n" +
			admittedWriteBody
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write packet %s: %v", name, err)
		}
		return path
	}

	p1 := writePacket("packet-1.md", "lane-a", "feat-alpha")
	p2 := writePacket("packet-2.md", "lane-b", "feat-beta")
	origResolver := resolveAdmissionRefSHA
	t.Cleanup(func() { resolveAdmissionRefSHA = origResolver })
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return "1111111111111111111111111111111111111111", nil
	}

	origFactory := depsFactory
	t.Cleanup(func() { depsFactory = origFactory })
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.ResolveCandidateIdentity = stubCandidateIdentity
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
			t.Errorf("CreateWorktree called for lane %q; a mixed-target batch must be rejected before any lane dispatches", laneID)
			return worktree.Worktree{}, nil
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
	code := run(context.Background(), []string{"run", "--packet", p1, "--packet", p2}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run exit code = %d, want 1; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "same feature target") {
		t.Errorf("stderr = %q, want it to explain that one batch promotes onto one feature target", stderr.String())
	}
}

// TestIntegrateRetryCLI proves the Defect B fix end to end: a lane that
// reaches its own "done" but is reverted only because the batch-level
// checks fail (e.g. the base was red, unrelated to the lane's own work) can
// be re-integrated later, once the base is fixed, WITHOUT redispatching the
// lane -- "lucind-ai integrate retry" rebuilds the batch straight from the
// ledger and the lane's own preserved worktree/result envelope.
func TestIntegrateRetryCLI(t *testing.T) {
	primaryRoot := initRepo(t)

	p1 := filepath.Join(primaryRoot, "packet-1.md")
	p1Content := "---\n" +
		"id: lane-retry-1\n" +
		"executor: agy\n" +
		"routed_by: test\n" +
		"feature: feat-retry\n" +
		"parent_ref: refs/heads/feature-retry\n" +
		"base_sha: 1111111111111111111111111111111111111111\n" +
		"expected_parent_sha: 1111111111111111111111111111111111111111\n" +
		"---\n" +
		admittedWriteBody
	if err := os.WriteFile(p1, []byte(p1Content), 0o644); err != nil {
		t.Fatalf("write packet 1: %v", err)
	}

	var (
		checksPass bool
		promoted   []string
	)
	origResolver := resolveAdmissionRefSHA
	t.Cleanup(func() { resolveAdmissionRefSHA = origResolver })
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return "1111111111111111111111111111111111111111", nil
	}
	origFactory := depsFactory
	t.Cleanup(func() { depsFactory = origFactory })
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout, approvalTimeout)
		deps.ResolveCandidateIdentity = stubCandidateIdentity
		deps.CreateWorktree = func(ctx context.Context, primaryRoot, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
			return worktree.Worktree{Path: t.TempDir(), Branch: "branch-" + laneID}, nil
		}
		deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return true, nil
		}
		deps.PorcelainEmpty = func(ctx context.Context, worktreePath string) (bool, error) {
			return true, nil
		}
		deps.LookupExecutor = func(name string) (executor.Executor, error) {
			return testDoneExecutor{envelope: `{"packet_id": "lane-retry-1", "status": "done", "summary": "done", "hard_stops": []}`}, nil
		}
		deps.CombineTree = func(ctx context.Context, primaryRoot, runID, parentRef, baseSHA string, branches []string) (string, string, error) {
			return t.TempDir(), "integration-branch", nil
		}
		// checksPass simulates the base itself going from red to green
		// between the original dispatch and the later retry, with no
		// change at all to the lane's own work.
		deps.RunChecks = func(ctx context.Context, worktreePath string) (bool, string, error) {
			if checksPass {
				return true, "", nil
			}
			return false, "base is red", nil
		}
		deps.ResolveRefSHA = func(ctx context.Context, primaryRoot, ref string) (string, error) {
			return "1111111111111111111111111111111111111111", nil
		}
		deps.ResolveCandidateSHA = func(ctx context.Context, primaryRoot, worktreePath, branch string) (string, error) {
			return "2222222222222222222222222222222222222222", nil
		}
		deps.PromoteCAS = func(ctx context.Context, primaryRoot, parentRef, candidateSHA, expectedSHA string) error {
			promoted = append(promoted, parentRef+" "+expectedSHA+"->"+candidateSHA)
			return nil
		}
		deps.PromoteTarget = func(ctx context.Context, primaryRoot, integrationBranch string) error {
			t.Errorf("PromoteTarget called on a feature-targeted batch; promotion must go through the compare-and-swap attempt path")
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

	// Original dispatch: the lane itself reaches "done", but the base is
	// red, so the batch-level checks fail and the lane is reverted.
	checksPass = false
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--packet", p1}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("initial run exit code = %d, want 1 (reverted); stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "reverted_ids: lane-retry-1") {
		t.Fatalf("initial run stdout = %q, want lane-retry-1 in reverted_ids", stdout.String())
	}
	if len(promoted) != 0 {
		t.Fatalf("PromoteCAS calls = %v, want none from the reverted first attempt", promoted)
	}

	runID := extractRunID(stdout.String())
	if runID == "" {
		t.Fatalf("could not extract run id from stdout: %q", stdout.String())
	}

	// The base is fixed; retry integration for the SAME run, with no
	// redispatch of lane-retry-1.
	checksPass = true
	var retryStdout, retryStderr bytes.Buffer
	retryCode := run(context.Background(), []string{"integrate", "retry", "--run", runID}, &retryStdout, &retryStderr)
	if retryCode != 0 {
		t.Fatalf("integrate retry exit code = %d, want 0; stderr = %q stdout = %q", retryCode, retryStderr.String(), retryStdout.String())
	}
	if !strings.Contains(retryStdout.String(), "integrated_ids: lane-retry-1") {
		t.Errorf("retry stdout = %q, want lane-retry-1 in integrated_ids", retryStdout.String())
	}
	if len(promoted) != 1 {
		t.Fatalf("PromoteCAS calls = %v, want exactly one compare-and-swap promotion from the retry", promoted)
	}
	want := "refs/heads/feature-retry 1111111111111111111111111111111111111111->2222222222222222222222222222222222222222"
	if promoted[0] != want {
		t.Errorf("PromoteCAS call = %q, want %q", promoted[0], want)
	}
}

// TestIntegrateRetryCLIRequiresRun proves --run is validated before any
// ledger or git work happens.
func TestIntegrateRetryCLIRequiresRun(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"integrate", "retry"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("integrate retry without --run exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--run") {
		t.Errorf("stderr = %q, want it to contain %q", stderr.String(), "--run")
	}
}

// TestIntegrateRetryCLIUnknownRun proves an unknown run id is reported
// clearly rather than silently doing nothing.
func TestIntegrateRetryCLIUnknownRun(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"integrate", "retry", "--run", "run-does-not-exist"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("integrate retry with unknown run exit code = 0, want non-zero")
	}
	if stderr.String() == "" {
		t.Errorf("stderr is empty, want an error surfaced")
	}
}

func TestDefectSubcommandUnknownAction(t *testing.T) {
	var stdout, stderr bytes.Buffer
	codeNoAction := run(context.Background(), []string{"defect"}, &stdout, &stderr)
	if codeNoAction == 0 {
		t.Fatalf("lucind-ai defect without action exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "action") {
		t.Errorf("stderr = %q, want it to mention action requirement", stderr.String())
	}

	stderr.Reset()
	codeUnknown := run(context.Background(), []string{"defect", "bogus"}, &stdout, &stderr)
	if codeUnknown == 0 {
		t.Fatalf("lucind-ai defect bogus exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "unknown defect subcommand") {
		t.Errorf("stderr = %q, want it to mention unknown defect subcommand", stderr.String())
	}
}

func TestDefectListCLIRequiresFeature(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"defect", "list"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("lucind-ai defect list without --feature exit code = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "--feature") {
		t.Errorf("stderr = %q, want it to contain --feature", stderr.String())
	}
}

func TestDefectRecordCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	baseSHA := strings.Repeat("1", 40)
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	// Create feature first
	createCode := run(ctx, []string{"feature", "create", "--id", "feat-defect-1", "--parent", "refs/heads/feature-defect-1", "--base-sha", baseSHA}, &stdout, &stderr)
	if createCode != 0 {
		t.Fatalf("feature create exit code = %d, want 0; stderr = %q", createCode, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	recordCode := run(ctx, []string{
		"defect", "record",
		"--id", "defect-rec-1",
		"--feature", "feat-defect-1",
		"--signature", "TestAuthFailed",
		"--evidence", "stack trace: nil pointer",
		"--disposition", "recorded",
	}, &stdout, &stderr)
	if recordCode != 0 {
		t.Fatalf("defect record exit code = %d, want 0; stderr = %q", recordCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "recorded defect defect-rec-1 for feature feat-defect-1") {
		t.Errorf("stdout = %q, want recorded defect confirmation", stdout.String())
	}

	// Verify defect is persisted in ledger
	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer ledg.Close()

	rec, err := ledg.GetDefect(ctx, "defect-rec-1")
	if err != nil {
		t.Fatalf("GetDefect(defect-rec-1): %v", err)
	}
	if rec.ID != "defect-rec-1" || rec.FeatureID != "feat-defect-1" ||
		rec.ErrorSignature != "TestAuthFailed" || rec.Evidence != "stack trace: nil pointer" ||
		rec.Disposition != ledger.DefectDispositionRecorded {
		t.Errorf("persisted DefectRecord = %+v", rec)
	}
}

func TestDefectRecordCLIRequiresFlags(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	ctx := context.Background()

	// Missing --id
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"defect", "record", "--feature", "f1", "--signature", "sig1"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "--id") {
		t.Fatalf("defect record without --id code = %d, want non-zero; stderr = %q", code, stderr.String())
	}

	// Missing --feature
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"defect", "record", "--id", "id1", "--signature", "sig1"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "--feature") {
		t.Fatalf("defect record without --feature code = %d, want non-zero; stderr = %q", code, stderr.String())
	}

	// Missing --signature
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"defect", "record", "--id", "id1", "--feature", "f1"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "--signature") {
		t.Fatalf("defect record without --signature code = %d, want non-zero; stderr = %q", code, stderr.String())
	}
}

func TestDefectListCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	baseSHA := strings.Repeat("2", 40)
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	// Create feature first
	createCode := run(ctx, []string{"feature", "create", "--id", "feat-list-1", "--parent", "refs/heads/feature-list-1", "--base-sha", baseSHA}, &stdout, &stderr)
	if createCode != 0 {
		t.Fatalf("feature create exit code = %d, want 0; stderr = %q", createCode, stderr.String())
	}

	// Record two defects
	run(ctx, []string{"defect", "record", "--id", "def-l-1", "--feature", "feat-list-1", "--signature", "sig-1", "--disposition", "recorded"}, &stdout, &stderr)
	run(ctx, []string{"defect", "record", "--id", "def-l-2", "--feature", "feat-list-1", "--signature", "sig-2", "--disposition", "repaired"}, &stdout, &stderr)

	stdout.Reset()
	stderr.Reset()
	listCode := run(ctx, []string{"defect", "list", "--feature", "feat-list-1"}, &stdout, &stderr)
	if listCode != 0 {
		t.Fatalf("defect list exit code = %d, want 0; stderr = %q", listCode, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "def-l-1") || !strings.Contains(out, "sig-1") || !strings.Contains(out, "recorded") {
		t.Errorf("defect list output missing def-l-1 details: %q", out)
	}
	if !strings.Contains(out, "def-l-2") || !strings.Contains(out, "sig-2") || !strings.Contains(out, "repaired") {
		t.Errorf("defect list output missing def-l-2 details: %q", out)
	}
}

func TestDefectRecordCLIRejectsInvalidDisposition(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	baseSHA := strings.Repeat("3", 40)
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	createCode := run(ctx, []string{"feature", "create", "--id", "feat-disp-test", "--parent", "refs/heads/feature-disp-test", "--base-sha", baseSHA}, &stdout, &stderr)
	if createCode != 0 {
		t.Fatalf("feature create exit code = %d, want 0; stderr = %q", createCode, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code := run(ctx, []string{
		"defect", "record",
		"--id", "def-bad-disp",
		"--feature", "feat-disp-test",
		"--signature", "sig",
		"--disposition", "not-a-valid-disposition",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("defect record with invalid disposition exit code = %d, want non-zero", code)
	}
	if !strings.Contains(stderr.String(), "invalid disposition") {
		t.Errorf("stderr = %q, want it to mention 'invalid disposition'", stderr.String())
	}
}

func TestDefectDeclineCLI(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	baseSHA := strings.Repeat("4", 40)
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	// Create feature first
	createCode := run(ctx, []string{"feature", "create", "--id", "feat-decline-1", "--parent", "refs/heads/feature-decline-1", "--base-sha", baseSHA}, &stdout, &stderr)
	if createCode != 0 {
		t.Fatalf("feature create exit code = %d, want 0; stderr = %q", createCode, stderr.String())
	}

	// Record a defect with disposition recorded
	recordCode := run(ctx, []string{
		"defect", "record",
		"--id", "defect-to-decline",
		"--feature", "feat-decline-1",
		"--signature", "TestFixMe",
		"--evidence", "broken test",
		"--disposition", "recorded",
	}, &stdout, &stderr)
	if recordCode != 0 {
		t.Fatalf("defect record exit code = %d; stderr = %q", recordCode, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	// Decline the defect
	declineCode := run(ctx, []string{"defect", "decline", "--id", "defect-to-decline"}, &stdout, &stderr)
	if declineCode != 0 {
		t.Fatalf("defect decline exit code = %d, want 0; stderr = %q", declineCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "declined defect defect-to-decline") {
		t.Errorf("stdout = %q, want confirmation 'declined defect defect-to-decline'", stdout.String())
	}

	// Verify in ledger that disposition is now declined
	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer ledg.Close()

	rec, err := ledg.GetDefect(ctx, "defect-to-decline")
	if err != nil {
		t.Fatalf("GetDefect: %v", err)
	}
	if rec.Disposition != ledger.DefectDispositionDeclined {
		t.Errorf("Disposition = %q, want %q", rec.Disposition, ledger.DefectDispositionDeclined)
	}
}

func TestDefectDeclineCLIRequiresFlags(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"defect", "decline"}, &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "--id") {
		t.Fatalf("defect decline without --id code = %d, want non-zero; stderr = %q", code, stderr.String())
	}
}

func TestDefectDeclineCLINotFound(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"defect", "decline", "--id", "nonexistent-id"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("defect decline nonexistent ID code = %d, want non-zero", code)
	}
}

func TestLinkedWorktreeCommands(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	baseSHA := strings.Repeat("5", 40)
	ctx := context.Background()
	var stdout, stderr bytes.Buffer

	// 1. Create feature in primary repo
	createCode := run(ctx, []string{"feature", "create", "--id", "feat-wt-1", "--parent", "refs/heads/feature-wt-1", "--base-sha", baseSHA}, &stdout, &stderr)
	if createCode != 0 {
		t.Fatalf("feature create exit code = %d, want 0; stderr = %q", createCode, stderr.String())
	}

	// 2. Record an initial defect in primary repo
	recCode := run(ctx, []string{
		"defect", "record",
		"--id", "def-wt-1",
		"--feature", "feat-wt-1",
		"--signature", "sig-initial",
		"--evidence", "stack-initial",
		"--disposition", "recorded",
	}, &stdout, &stderr)
	if recCode != 0 {
		t.Fatalf("defect record in primary root exit code = %d; stderr = %q", recCode, stderr.String())
	}

	// 3. Create a linked worktree
	wt, err := worktree.Create(ctx, primaryRoot, "lane-wt-test")
	if err != nil {
		t.Fatalf("worktree.Create: %v", err)
	}
	defer worktree.Remove(ctx, primaryRoot, wt.Path, true)

	if !worktree.IsLinkedWorktree(wt.Path) {
		t.Fatalf("IsLinkedWorktree(%q) = false, want true", wt.Path)
	}

	// 4. Switch working directory into the linked worktree
	if err := os.Chdir(wt.Path); err != nil {
		t.Fatalf("os.Chdir to worktree: %v", err)
	}

	// 5. Test feature status from inside linked worktree
	stdout.Reset()
	stderr.Reset()
	statusCode := run(ctx, []string{"feature", "status", "--id", "feat-wt-1"}, &stdout, &stderr)
	if statusCode != 0 {
		t.Errorf("feature status in linked worktree exit code = %d, want 0; stderr = %q", statusCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "feat-wt-1") {
		t.Errorf("feature status stdout = %q, want it to contain 'feat-wt-1'", stdout.String())
	}

	// 6. Test defect list from inside linked worktree
	stdout.Reset()
	stderr.Reset()
	listCode := run(ctx, []string{"defect", "list", "--feature", "feat-wt-1"}, &stdout, &stderr)
	if listCode != 0 {
		t.Errorf("defect list in linked worktree exit code = %d, want 0; stderr = %q", listCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "def-wt-1") {
		t.Errorf("defect list stdout = %q, want it to contain 'def-wt-1'", stdout.String())
	}

	// 7. Test defect record from inside linked worktree
	stdout.Reset()
	stderr.Reset()
	recordCode := run(ctx, []string{
		"defect", "record",
		"--id", "def-wt-2",
		"--feature", "feat-wt-1",
		"--signature", "sig-from-wt",
		"--evidence", "stack-from-wt",
		"--disposition", "recorded",
	}, &stdout, &stderr)
	if recordCode != 0 {
		t.Errorf("defect record in linked worktree exit code = %d, want 0; stderr = %q", recordCode, stderr.String())
	}

	// 8. Test defect decline from inside linked worktree
	stdout.Reset()
	stderr.Reset()
	declineCode := run(ctx, []string{"defect", "decline", "--id", "def-wt-1"}, &stdout, &stderr)
	if declineCode != 0 {
		t.Errorf("defect decline in linked worktree exit code = %d, want 0; stderr = %q", declineCode, stderr.String())
	}

	// 9. Verify in primary root's ledger that updates persisted
	ledg, err := ledger.Open(ctx, primaryRoot)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer ledg.Close()

	d1, err := ledg.GetDefect(ctx, "def-wt-1")
	if err != nil {
		t.Fatalf("GetDefect(def-wt-1): %v", err)
	}
	if d1.Disposition != ledger.DefectDispositionDeclined {
		t.Errorf("def-wt-1 disposition = %q, want %q", d1.Disposition, ledger.DefectDispositionDeclined)
	}

	d2, err := ledg.GetDefect(ctx, "def-wt-2")
	if err != nil {
		t.Fatalf("GetDefect(def-wt-2): %v", err)
	}
	if d2.ErrorSignature != "sig-from-wt" || d2.Disposition != ledger.DefectDispositionRecorded {
		t.Errorf("def-wt-2 = %+v, want signature 'sig-from-wt' and disposition 'recorded'", d2)
	}
}

// TestRunCheckFromLinkedWorktreeTestsWorktreeOwnCode proves the runCheck
// regression fix: "check" run from inside a linked worktree must test and
// report the WORKTREE's own commit and lucind-checks.sh -- never silently
// redirect to the primary checkout's. resolvePrimaryRoot's
// git-common-dir-based semantics are correct for the 18 ledger-touching call
// sites, but "check" never touches the ledger; it tests wherever the caller
// is actually standing, worktree or not. The two lucind-checks.sh scripts
// and commits below are deliberately never merged/visible to each other --
// a real, local, uncommitted-relative-to-primary divergence -- so a run
// against the wrong root is unambiguously detectable by its output.
func TestRunCheckFromLinkedWorktreeTestsWorktreeOwnCode(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to real git")
	}
	primaryRoot := initRepo(t)

	primaryScript := "#!/bin/sh\necho \"PASS: primary\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(primaryRoot, "lucind-checks.sh"), []byte(primaryScript), 0o755); err != nil {
		t.Fatalf("WriteFile(primary lucind-checks.sh) error = %v", err)
	}
	runGit(t, primaryRoot, "add", "lucind-checks.sh")
	runGit(t, primaryRoot, "commit", "-m", "add primary lucind-checks.sh")

	ctx := context.Background()
	wt, err := worktree.Create(ctx, primaryRoot, "lane-check-wt")
	if err != nil {
		t.Fatalf("worktree.Create: %v", err)
	}
	defer worktree.Remove(ctx, primaryRoot, wt.Path, true)

	if !worktree.IsLinkedWorktree(wt.Path) {
		t.Fatalf("IsLinkedWorktree(%q) = false, want true", wt.Path)
	}

	// A commit that exists ONLY in the worktree's own branch -- never merged
	// or visible from the primary checkout.
	worktreeScript := "#!/bin/sh\necho \"PASS: worktree own code\"\nexit 0\n"
	if err := os.WriteFile(filepath.Join(wt.Path, "lucind-checks.sh"), []byte(worktreeScript), 0o755); err != nil {
		t.Fatalf("WriteFile(worktree lucind-checks.sh) error = %v", err)
	}
	runGit(t, wt.Path, "add", "lucind-checks.sh")
	runGit(t, wt.Path, "commit", "-m", "worktree-only change")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = wt.Path
	worktreeHeadBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD (worktree): %v", err)
	}
	worktreeHead := strings.TrimSpace(string(worktreeHeadBytes))

	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = primaryRoot
	primaryHeadBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD (primary): %v", err)
	}
	primaryHead := strings.TrimSpace(string(primaryHeadBytes))

	if worktreeHead == primaryHead {
		t.Fatalf("worktree and primary HEAD unexpectedly identical (%s); test setup did not diverge them", worktreeHead)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(wt.Path); err != nil {
		t.Fatalf("os.Chdir to worktree: %v", err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("check in linked worktree exit code = %d, want 0; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}

	outStr := stdout.String()
	if !strings.Contains(outStr, "PASS: worktree own code") {
		t.Errorf("stdout = %q, want it to contain the WORKTREE's own script output %q -- got the primary checkout's script instead, meaning check tested the wrong code", outStr, "PASS: worktree own code")
	}
	if strings.Contains(outStr, "PASS: primary") {
		t.Errorf("stdout = %q, unexpectedly contains the PRIMARY checkout's script output -- check ran against primary instead of the worktree", outStr)
	}
	if !strings.Contains(outStr, worktreeHead) {
		t.Errorf("stdout = %q, want it to report the worktree's own commit %q", outStr, worktreeHead)
	}
	if strings.Contains(outStr, primaryHead) {
		t.Errorf("stdout = %q, unexpectedly reports the PRIMARY checkout's commit %q instead of the worktree's own %q", outStr, primaryHead, worktreeHead)
	}
}

func TestAcceptNoFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"accept"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run(accept with no flags) = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--run and --lane are required") {
		t.Fatalf("stderr = %q, want required persisted identity", stderr.String())
	}
}

// TestUsageAdvertisesAcceptRunAndLane guards the top-level usage string against
// drift: accept takes exactly --run and --lane, so the one invocation a person
// sees on a usage error must not describe flags the subcommand does not parse.
func TestUsageAdvertisesAcceptRunAndLane(t *testing.T) {
	if !strings.Contains(usage, "lucind-ai accept --run <run-id> --lane <lane-id>") {
		t.Fatalf("usage = %q, want it to advertise the real accept signature", usage)
	}
}

type fakeAcceptanceVerifier struct {
	receipt ledger.AcceptanceReceipt
	err     error
	request accept.AcceptanceRequest
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *fakeAcceptanceVerifier) Verify(_ context.Context, request accept.AcceptanceRequest) (ledger.AcceptanceReceipt, error) {
	f.request = request
	return f.receipt, f.err
}

func TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly(t *testing.T) {
	primaryRoot := t.TempDir()
	runGit(t, primaryRoot, "init", "-b", "main")
	runGit(t, primaryRoot, "config", "user.name", "lucind-test")
	runGit(t, primaryRoot, "config", "user.email", "test@lucind.ai")
	if err := os.WriteFile(filepath.Join(primaryRoot, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, primaryRoot, "add", "README.md")
	runGit(t, primaryRoot, "commit", "-m", "seed commit")
	refsBefore := runGitOutput(t, primaryRoot, "show-ref")

	fake := &fakeAcceptanceVerifier{receipt: ledger.AcceptanceReceipt{
		ReceiptID: "receipt-1", BindingHash: "binding-1", ResultHash: "result-1", ChecksHash: "checks-1",
		Binding: ledger.AcceptanceBinding{RunID: "run-1", LaneID: "lane-1", CandidateCommit: "candidate-1"}, Cleanup: "removed",
	}}
	originalFactory := acceptVerifierFactory
	acceptVerifierFactory = func(string, *ledger.Ledger) acceptanceVerifier { return fake }
	t.Cleanup(func() { acceptVerifierFactory = originalFactory })

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer os.Chdir(cwd)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"accept", "--run", "run-1", "--lane", "lane-1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("accept exit code = %d, want 0; stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{"receipt-1", "binding-1", "mechanical evidence", "qualitative approval remains separate", "acceptance-promotion.md", "steps 2–10"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout %q missing %q", out, want)
		}
	}
	if strings.Contains(strings.ToLower(out), "semantically approved") {
		t.Fatalf("output escalates authority: %q", out)
	}
	if fake.request != (accept.AcceptanceRequest{RunID: "run-1", LaneID: "lane-1"}) {
		t.Fatalf("request = %+v", fake.request)
	}
	if refsAfter := runGitOutput(t, primaryRoot, "show-ref"); refsAfter != refsBefore {
		t.Fatalf("CLI mutated refs")
	}

	fake.err = errors.New("exact receipt absent")
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"accept", "--run", "run-1", "--lane", "lane-1"}, &stdout, &stderr); code == 0 {
		t.Fatal("accept succeeded without receipt")
	}
}

func TestFeatureLeaseReleaseCLI(t *testing.T) {
	primaryRoot := t.TempDir()

	runGit(t, primaryRoot, "init", "-b", "main")
	runGit(t, primaryRoot, "config", "user.name", "lucind-test")
	runGit(t, primaryRoot, "config", "user.email", "test@lucind.ai")

	seedFile := filepath.Join(primaryRoot, "README.md")
	if err := os.WriteFile(seedFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}
	runGit(t, primaryRoot, "add", "README.md")
	runGit(t, primaryRoot, "commit", "-m", "initial commit")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = primaryRoot
	baseSHABytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	baseSHA := strings.TrimSpace(string(baseSHABytes))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer os.Chdir(cwd)

	// Create feature in ledger
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"feature", "create", "--id", "feat-lease-test", "--parent", "refs/heads/feature-test", "--base-sha", baseSHA}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("feature create exit code = %d: %s", code, stderr.String())
	}

	// Acquire lease via ledger
	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	featSvc := feature.NewService(ledg)
	l, err := featSvc.AcquireLease(context.Background(), "feat-lease-test", "test-worker", time.Hour)
	if err != nil {
		t.Fatalf("AcquireLease: %v", err)
	}
	ledg.Close()

	// Check status
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"feature", "lease", "status", "--id", "feat-lease-test"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("feature lease status exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid:      true") {
		t.Errorf("stdout = %q, want valid: true", stdout.String())
	}

	// Try release with live PID (should fail)
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"feature", "lease", "release", "--id", "feat-lease-test", "--pid", fmt.Sprintf("%d", os.Getpid())}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("feature lease release with live PID exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "is still alive") {
		t.Errorf("stderr = %q, want it to mention process is still alive", stderr.String())
	}

	// Release with matching owner and fence
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"feature", "lease", "release", "--id", "feat-lease-test", "--owner", l.Owner, "--fence", fmt.Sprintf("%d", l.Fence)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("feature lease release exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "lease:   released") {
		t.Errorf("stdout = %q, want lease: released", stdout.String())
	}

	// Check status after release
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"feature", "lease", "status", "--id", "feat-lease-test"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("feature lease status after release exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid:      false") {
		t.Errorf("stdout = %q, want valid: false", stdout.String())
	}
}

func TestReconcileWaitStableCLI(t *testing.T) {
	primaryRoot := t.TempDir()

	runGit(t, primaryRoot, "init", "-b", "main")
	runGit(t, primaryRoot, "config", "user.name", "lucind-test")
	runGit(t, primaryRoot, "config", "user.email", "test@lucind.ai")

	seedFile := filepath.Join(primaryRoot, "README.md")
	if err := os.WriteFile(seedFile, []byte("# Test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md): %v", err)
	}
	runGit(t, primaryRoot, "add", "README.md")
	runGit(t, primaryRoot, "commit", "-m", "initial commit")

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = primaryRoot
	baseSHABytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	baseSHA := strings.TrimSpace(string(baseSHABytes))

	// Source branch
	runGit(t, primaryRoot, "checkout", "-b", "feature-src")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("source content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "source commit")
	srcSHABytes, _ := exec.Command("git", "-C", primaryRoot, "rev-parse", "HEAD").Output()
	srcSHA := strings.TrimSpace(string(srcSHABytes))

	// Target branch
	runGit(t, primaryRoot, "checkout", "main")
	runGit(t, primaryRoot, "checkout", "-b", "feature-tgt")
	if err := os.WriteFile(filepath.Join(primaryRoot, "conflict.txt"), []byte("target content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, primaryRoot, "add", "conflict.txt")
	runGit(t, primaryRoot, "commit", "-m", "target commit")
	tgtSHABytes, _ := exec.Command("git", "-C", primaryRoot, "rev-parse", "HEAD").Output()
	tgtSHA := strings.TrimSpace(string(tgtSHABytes))

	runGit(t, primaryRoot, "checkout", "main")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatalf("os.Chdir: %v", err)
	}
	defer os.Chdir(cwd)

	ledg, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	featSvc := feature.NewService(ledg)
	_, _ = featSvc.Create(context.Background(), "feat-s", "refs/heads/feature-src", baseSHA)
	_, _ = featSvc.Create(context.Background(), "feat-t", "refs/heads/feature-tgt", baseSHA)

	reconcileSvc := reconcile.NewService(ledg)
	ev := &overlap.Evidence{
		Version:     "1.0",
		BaseSHA:     baseSHA,
		FeatureASHA: srcSHA,
		FeatureBSHA: tgtSHA,
		Class:       overlap.ClassRequired,
		Signals: overlap.Signals{
			ConflictPaths: []string{"conflict.txt"},
		},
	}

	req, err := reconcileSvc.CreateRequest(context.Background(), reconcile.CreateRequestParams{
		ID:            "req-ws-1",
		FeatureID:     "feat-t",
		SourceFeature: "feat-s",
		SourceParent:  "refs/heads/feature-src",
		TargetFeature: "feat-t",
		TargetParent:  "refs/heads/feature-tgt",
		SourceSHA:     srcSHA,
		TargetSHA:     tgtSHA,
		Evidence:      ev,
		TTL:           10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	ledg.Close()

	// Renew with --wait-stable
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"reconcile", "renew", "--request", req.ID, "--target-sha", "refs/heads/feature-tgt", "--wait-stable", "50ms"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("reconcile renew exit code = %d: %s", code, stderr.String())
	}
	renewOut := stdout.String()
	var newReqID string
	for _, line := range strings.Split(renewOut, "\n") {
		if strings.HasPrefix(line, "request:") {
			newReqID = strings.TrimSpace(strings.TrimPrefix(line, "request:"))
			break
		}
	}
	if newReqID == "" {
		t.Fatalf("could not parse new request ID from renew output: %s", renewOut)
	}

	// Read renewed request from ledger
	ledg, _ = ledger.Open(context.Background(), primaryRoot)
	renewedReq, err := ledg.ReconciliationRequest(context.Background(), newReqID)
	if err != nil {
		t.Fatalf("ReconciliationRequest failed: %v", err)
	}
	ledg.Close()

	sourceFeat, _, targetFeat, _ := reconcile.ParseDirection(renewedReq.Direction)

	// Approve renewed request to create candidate
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"reconcile", "approve", "--request", newReqID, "--source", sourceFeat, "--target", targetFeat}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("reconcile approve exit code = %d: %s", code, stderr.String())
	}

	// Read candidate ID from ledger
	ledg, _ = ledger.Open(context.Background(), primaryRoot)
	cand, err := ledg.ReconciliationCandidateByRequest(context.Background(), newReqID)
	if err != nil || cand.ID == "" {
		t.Fatalf("ReconciliationCandidateByRequest failed: %v", err)
	}
	candID := cand.ID
	ledg.Close()

	// Resolve candidate with --wait-stable
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"reconcile", "resolve", "--candidate", candID, "--sha", srcSHA, "--wait-stable", "50ms"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("reconcile resolve exit code = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status:    integrated") {
		t.Errorf("stdout = %q, want status: integrated", stdout.String())
	}
}

// TestEveryDefectDispositionIsReachableFromCLI is the guard for the class of
// defect that lets a durable record enter a state it can never leave. The
// defect_records CHECK constraint admits four dispositions, and
// UpdateDefectDisposition accepts any of them, but for a long time only
// "declined" had a CLI path: a genuinely repaired defect stayed "recorded"
// forever, indistinguishable from one nobody had looked at.
//
// This drives the real CLI for every value in the declared vocabulary rather
// than grepping for call sites, so a disposition added to the schema without a
// way to reach it fails here.
func TestEveryDefectDispositionIsReachableFromCLI(t *testing.T) {
	for _, tc := range []struct {
		disposition ledger.DefectDisposition
		args        func(id string) []string
	}{
		{ledger.DefectDispositionRecorded, func(id string) []string { return nil }},
		{ledger.DefectDispositionRepaired, func(id string) []string {
			return []string{"defect", "resolve", "--id", id}
		}},
		{ledger.DefectDispositionDeclined, func(id string) []string {
			return []string{"defect", "decline", "--id", id}
		}},
		{ledger.DefectDispositionDeferred, func(id string) []string {
			return []string{"defect", "defer", "--id", id}
		}},
	} {
		t.Run(string(tc.disposition), func(t *testing.T) {
			primaryRoot := initRepo(t)
			cwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(primaryRoot); err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(cwd)

			ctx := context.Background()
			var stdout, stderr bytes.Buffer

			featureID := "feat-reach-" + string(tc.disposition)
			if code := run(ctx, []string{
				"feature", "create", "--id", featureID,
				"--parent", "refs/heads/" + featureID,
				"--base-sha", strings.Repeat("7", 40),
			}, &stdout, &stderr); code != 0 {
				t.Fatalf("feature create exit = %d; stderr = %q", code, stderr.String())
			}

			defectID := "def-reach-" + string(tc.disposition)
			if code := run(ctx, []string{
				"defect", "record",
				"--id", defectID,
				"--feature", featureID,
				"--signature", "sig-" + string(tc.disposition),
			}, &stdout, &stderr); code != 0 {
				t.Fatalf("defect record exit = %d; stderr = %q", code, stderr.String())
			}

			if args := tc.args(defectID); args != nil {
				stdout.Reset()
				stderr.Reset()
				if code := run(ctx, args, &stdout, &stderr); code != 0 {
					t.Fatalf("%v exit = %d, want 0; stderr = %q", args, code, stderr.String())
				}
			}

			ledg, err := ledger.Open(ctx, primaryRoot)
			if err != nil {
				t.Fatalf("open ledger: %v", err)
			}
			defer ledg.Close()

			rec, err := ledg.GetDefect(ctx, defectID)
			if err != nil {
				t.Fatalf("GetDefect: %v", err)
			}
			if rec.Disposition != tc.disposition {
				t.Errorf("Disposition = %q, want %q", rec.Disposition, tc.disposition)
			}
		})
	}
}

func TestDefectResolveCLIRequiresID(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"defect", "resolve"}, &stdout, &stderr); code == 0 || !strings.Contains(stderr.String(), "--id") {
		t.Fatalf("defect resolve without --id code = %d, want non-zero naming --id; stderr = %q", code, stderr.String())
	}
}

func TestDefectResolveCLINotFound(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	if code := run(ctx, []string{"defect", "resolve", "--id", "nonexistent-id"}, &stdout, &stderr); code == 0 {
		t.Fatalf("defect resolve on unknown id code = %d, want non-zero", code)
	}
}

type mockCLIStatusQuerier struct {
	output []byte
	err    error
}

func (m *mockCLIStatusQuerier) QueryStatus(_ context.Context, _ string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

func TestPhaseSubcommandRequiresPhaseName(t *testing.T) {
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"phase"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run(phase) = %d, want non-zero", code)
	}
	if !strings.Contains(stderr.String(), "phase name is required") {
		t.Fatalf("expected stderr to say 'phase name is required', got %q", stderr.String())
	}
}

func TestPhaseSubcommandGatesPrematureSynthesis(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	change := "test-change"
	statusJSON := []byte(`{
  "schemaName": "gentle-ai.sdd-status",
  "schemaVersion": 1,
  "changeName": "` + change + `",
  "artifactPaths": {},
  "artifacts": {
    "proposal": "missing"
  },
  "dependencies": {
    "proposal": "ready"
  },
  "nextRecommended": "propose"
}`)

	origQuerier := defaultStatusQuerier
	defer func() { defaultStatusQuerier = origQuerier }()
	defaultStatusQuerier = func(string) phasespec.StatusQuerier {
		return &mockCLIStatusQuerier{output: statusJSON}
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	// propose synthesis requires lens-a, lens-b, lens-c merged; with mock querier (no merged lenses), it must fail
	code := run(ctx, []string{"phase", "propose", "--change", change}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected run(phase propose) to fail when lenses unmerged, got code 0; stdout=%q, stderr=%q", stdout.String(), stderr.String())
	}

	// Verify no artifact was created
	artifactPath := filepath.Join(primaryRoot, "openspec", "changes", change, "propose.md")
	if _, err := os.Stat(artifactPath); err == nil {
		t.Fatalf("expected artifact %s to NOT exist, but it was created", artifactPath)
	}
}

func TestPhaseSubcommandFailsClosedOnMalformedStatusJSON(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	origQuerier := defaultStatusQuerier
	defer func() { defaultStatusQuerier = origQuerier }()
	defaultStatusQuerier = func(string) phasespec.StatusQuerier {
		return &mockCLIStatusQuerier{output: []byte(`{ invalid json `)}
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"phase", "propose"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected run(phase) on malformed status to fail, got 0")
	}
}

func TestPhaseSubcommandPhaseAlreadyComplete(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	change := "test-change"
	statusJSON := []byte(`{
  "schemaName": "gentle-ai.sdd-status",
  "schemaVersion": 1,
  "changeName": "` + change + `",
  "artifactPaths": {},
  "artifacts": {
    "proposal": "done"
  },
  "dependencies": {
    "proposal": "all_done"
  },
  "nextRecommended": "spec"
}`)

	origQuerier := defaultStatusQuerier
	defer func() { defaultStatusQuerier = origQuerier }()
	defaultStatusQuerier = func(string) phasespec.StatusQuerier {
		return &mockCLIStatusQuerier{output: statusJSON}
	}

	// Create canonical artifact on disk
	artifactDir := filepath.Join(primaryRoot, "openspec", "changes", change)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(artifactDir, "propose.md")
	if err := os.WriteFile(artifactPath, []byte("# Propose\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"phase", "propose", "--change", change}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected run(phase propose) on complete phase to exit 0, got %d; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "already complete") {
		t.Fatalf("expected stdout to report already complete, got %q", stdout.String())
	}
}

func TestPhaseSubcommandPhaseNotCompleteIfFileMissingOnDisk(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	change := "test-change"
	// Status says proposal: done, but we do NOT create propose.md on disk, and lenses are unmerged -> must gate synthesis and fail
	statusJSON := []byte(`{
  "schemaName": "gentle-ai.sdd-status",
  "schemaVersion": 1,
  "changeName": "` + change + `",
  "artifacts": {
    "proposal": "done"
  },
  "dependencies": {
    "proposal": "all_done"
  },
  "nextRecommended": "spec"
}`)

	origQuerier := defaultStatusQuerier
	defer func() { defaultStatusQuerier = origQuerier }()
	defaultStatusQuerier = func(string) phasespec.StatusQuerier {
		return &mockCLIStatusQuerier{output: statusJSON}
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"phase", "propose", "--change", change}, &stdout, &stderr)
	// Because file is missing from disk, it treats phase as incomplete; and because lenses are unmerged, it returns 1 (cannot start synthesis)
	if code == 0 {
		t.Fatalf("expected run(phase propose) to fail when artifact missing from disk and lenses unmerged, got 0; stdout=%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "already complete") {
		t.Fatalf("expected stdout to NOT say already complete, got %q", stdout.String())
	}
}

func TestPhaseSubcommandDispatchesSynthesisWhenLensesMerged(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	change := "test-change"
	statusJSON := []byte(`{
  "schemaName": "gentle-ai.sdd-status",
  "schemaVersion": 1,
  "changeName": "` + change + `",
  "lenses": {
    "lens-a": {"id": "lens-a", "accepted": true, "merged": true},
    "lens-b": {"id": "lens-b", "accepted": true, "merged": true},
    "lens-c": {"id": "lens-c", "accepted": true, "merged": true}
  },
  "artifacts": {
    "proposal": "missing"
  },
  "dependencies": {
    "proposal": "ready"
  },
  "nextRecommended": "propose"
}`)

	origQuerier := defaultStatusQuerier
	defer func() { defaultStatusQuerier = origQuerier }()
	defaultStatusQuerier = func(string) phasespec.StatusQuerier {
		return &mockCLIStatusQuerier{output: statusJSON}
	}

	overrideDispatchDeps(t, testDoneExecutor{
		envelope: fmt.Sprintf(`{"packet_id": "propose-%s-synthesis", "status": "done", "summary": "done", "hard_stops": [], "skills_loaded": ["lucind-executor", "lucind-fan-out-lens", "sdd-propose"]}`, change),
	})

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"phase", "propose", "--change", change}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected run(phase propose) to exit 0, got %d; stderr=%s, stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "synthesis dispatched") {
		t.Fatalf("expected stdout to report synthesis dispatched, got %q", stdout.String())
	}
}

func TestPhaseSubcommandUnknownPhase(t *testing.T) {
	primaryRoot := initRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(primaryRoot); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	change := "test-change"
	statusJSON := []byte(`{
  "schemaName": "gentle-ai.sdd-status",
  "schemaVersion": 1,
  "changeName": "` + change + `",
  "artifactPaths": {},
  "artifacts": {},
  "dependencies": {}
}`)

	origQuerier := defaultStatusQuerier
	defer func() { defaultStatusQuerier = origQuerier }()
	defaultStatusQuerier = func(string) phasespec.StatusQuerier {
		return &mockCLIStatusQuerier{output: statusJSON}
	}

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"phase", "unrecognized-phase", "--change", change}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected run(phase unrecognized) to return non-zero, got 0")
	}
}
