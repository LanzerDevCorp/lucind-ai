package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
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
		"executor: cursor-agent\n" +
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
	if !strings.Contains(stderr.String(), "cursor-agent") {
		t.Fatalf("stderr = %q, want it to name the unsupported executor %q", stderr.String(), "cursor-agent")
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
		"executor: cursor-agent\n" +
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
	if !strings.Contains(stderr.String(), "cursor-agent") {
		t.Fatalf("stderr = %q, want it to name the unsupported executor %q", stderr.String(), "cursor-agent")
	}
	if !strings.Contains(stderr.String(), secondPath) {
		t.Fatalf("stderr = %q, want it to name the offending packet path %q", stderr.String(), secondPath)
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
