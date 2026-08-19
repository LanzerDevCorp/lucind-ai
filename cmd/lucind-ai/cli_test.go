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
