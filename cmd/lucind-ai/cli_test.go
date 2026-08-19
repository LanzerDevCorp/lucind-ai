package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests cover the CLI's own wiring and failure paths only. Every
// scenario here fails before internal/run.Execute would ever touch git,
// the ledger, or an executor, so none of them shell out to real git or
// invoke real agy — see the package task's hard constraint against that.

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
