package accept_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/accept"
	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "user.email", "test@example.com")
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "initial commit")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v: %s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func writeResultJSON(t *testing.T, repoDir, laneID, status string) {
	t.Helper()
	env := fmt.Sprintf(`{
  "lane_id": %q,
  "status": %q,
  "summary": "Completed lane work.",
  "hard_stops": []
}`, laneID, status)
	resPath := lane.ResultPath(repoDir, laneID)
	if err := os.MkdirAll(filepath.Dir(resPath), 0755); err != nil {
		t.Fatalf("mkdir lane dir: %v", err)
	}
	if err := os.WriteFile(resPath, []byte(env), 0644); err != nil {
		t.Fatalf("write result.json: %v", err)
	}
}

func writeChecksScript(t *testing.T, repoDir string, exitCode int, output string) {
	t.Helper()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q\nexit %d\n", output, exitCode)
	path := filepath.Join(repoDir, "lucind-checks.sh")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write lucind-checks.sh: %v", err)
	}
}

func TestAccept_LaneNotFound(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	_, _, _, err := accept.Accept(ctx, repoDir, "20261003-120000-abcd")
	if err == nil {
		t.Fatal("expected error when lane does not exist")
	}
}

func TestAccept_ResultJsonMissing(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictRejected {
		t.Fatalf("verdict = %q; want %q", verdict, lane.VerdictRejected)
	}
	if receipt.Verdict != lane.VerdictRejected {
		t.Fatalf("receipt.Verdict = %q; want %q", receipt.Verdict, lane.VerdictRejected)
	}
	if len(reasons) == 0 {
		t.Fatal("expected rejection reasons for missing result.json")
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "result.json") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason mentioning result.json, got: %v", reasons)
	}

	// Verify lane on disk updated to rejected
	savedLane, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("load lane: %v", err)
	}
	if savedLane.Status != lane.StatusRejected {
		t.Fatalf("lane.Status = %q; want %q", savedLane.Status, lane.StatusRejected)
	}

	// Verify receipt on disk
	savedReceipt, err := lane.LoadReceipt(repoDir, l.ID)
	if err != nil {
		t.Fatalf("load receipt: %v", err)
	}
	if savedReceipt.Verdict != lane.VerdictRejected {
		t.Fatalf("savedReceipt.Verdict = %q; want %q", savedReceipt.Verdict, lane.VerdictRejected)
	}
}

func TestAccept_ResultJsonSchemaInvalid(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	resPath := lane.ResultPath(repoDir, l.ID)
	if err := os.WriteFile(resPath, []byte(`{"invalid": true}`), 0644); err != nil {
		t.Fatalf("write invalid result: %v", err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictRejected {
		t.Fatalf("verdict = %q; want %q", verdict, lane.VerdictRejected)
	}
	if receipt.Verdict != lane.VerdictRejected {
		t.Fatalf("receipt.Verdict = %q; want %q", receipt.Verdict, lane.VerdictRejected)
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "result.json") || strings.Contains(r, "schema") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason mentioning result.json or schema, got: %v", reasons)
	}
}

func TestAccept_ResultStatusNotDone(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "failed")

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictRejected {
		t.Fatalf("verdict = %q; want %q", verdict, lane.VerdictRejected)
	}
	if receipt.Verdict != lane.VerdictRejected {
		t.Fatalf("receipt.Verdict = %q; want %q", receipt.Verdict, lane.VerdictRejected)
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "failed") || strings.Contains(r, "status") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason mentioning status, got: %v", reasons)
	}
}

func TestAccept_ChangedFilesDisallowed(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	// Disallowed file outside src/**
	disallowedFile := filepath.Join(repoDir, "other", "file.txt")
	if err := os.MkdirAll(filepath.Dir(disallowedFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(disallowedFile, []byte("forbidden\n"), 0644); err != nil {
		t.Fatal(err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictRejected {
		t.Fatalf("verdict = %q; want %q", verdict, lane.VerdictRejected)
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "other/file.txt") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason listing offending file other/file.txt, got: %v", reasons)
	}
	foundInChanged := false
	for _, f := range receipt.ChangedFiles {
		if f == "other/file.txt" {
			foundInChanged = true
			break
		}
	}
	if !foundInChanged {
		t.Fatalf("expected changed_files to contain other/file.txt, got: %v", receipt.ChangedFiles)
	}
}

func TestAccept_MultipleDisallowedFiles(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	// Disallowed files
	fileA := filepath.Join(repoDir, "a", "file1.txt")
	fileB := filepath.Join(repoDir, "b", "file2.txt")
	_ = os.MkdirAll(filepath.Dir(fileA), 0755)
	_ = os.MkdirAll(filepath.Dir(fileB), 0755)
	_ = os.WriteFile(fileA, []byte("a\n"), 0644)
	_ = os.WriteFile(fileB, []byte("b\n"), 0644)

	verdict, _, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictRejected {
		t.Fatalf("verdict = %q; want %q", verdict, lane.VerdictRejected)
	}

	foundA, foundB := false, false
	for _, r := range reasons {
		if strings.Contains(r, "a/file1.txt") {
			foundA = true
		}
		if strings.Contains(r, "b/file2.txt") {
			foundB = true
		}
	}
	if !foundA || !foundB {
		t.Fatalf("expected reasons to list both offending files, got: %v", reasons)
	}
}

func TestAccept_MultipleAllowPatterns(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeChecksScript(t, repoDir, 0, "pass")

	l, err := lane.Create(ctx, repoDir, []string{"pkg/**", "docs/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	pkgFile := filepath.Join(repoDir, "pkg", "lib.go")
	docsFile := filepath.Join(repoDir, "docs", "readme.md")
	_ = os.MkdirAll(filepath.Dir(pkgFile), 0755)
	_ = os.MkdirAll(filepath.Dir(docsFile), 0755)
	_ = os.WriteFile(pkgFile, []byte("package pkg\n"), 0644)
	_ = os.WriteFile(docsFile, []byte("# Docs\n"), 0644)

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(reasons) != 0 {
		t.Fatalf("expected 0 reasons, got %v", reasons)
	}
	if receipt.Verdict != lane.VerdictAccepted {
		t.Fatalf("receipt.Verdict = %q; want %q", receipt.Verdict, lane.VerdictAccepted)
	}
}

func TestAccept_LucindFilesExemptFromAllow(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeChecksScript(t, repoDir, 0, "checks passed")

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	// Allowed change
	srcFile := filepath.Join(repoDir, "src", "code.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package src\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Change under .lucind/ which must be exempt
	lucindExtra := filepath.Join(repoDir, ".lucind", "extra.txt")
	if err := os.WriteFile(lucindExtra, []byte("extra metadata\n"), 0644); err != nil {
		t.Fatal(err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(reasons) != 0 {
		t.Fatalf("expected empty reasons, got %v", reasons)
	}
	if receipt.Verdict != lane.VerdictAccepted {
		t.Fatalf("receipt.Verdict = %q; want %q", receipt.Verdict, lane.VerdictAccepted)
	}

	savedLane, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("load lane: %v", err)
	}
	if savedLane.Status != lane.StatusAccepted {
		t.Fatalf("lane.Status = %q; want %q", savedLane.Status, lane.StatusAccepted)
	}
}

func TestAccept_FallbackChecksPass(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeChecksScript(t, repoDir, 0, "all tests passed successfully")

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "code.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package src\n"), 0644); err != nil {
		t.Fatal(err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if receipt.Evidence.Attestation != nil {
		t.Fatal("expected evidence.Attestation == nil when falling back to check.Check")
	}
	if receipt.Evidence.CheckLog == nil {
		t.Fatal("expected evidence.CheckLog != nil")
	}
	checkLogData, err := os.ReadFile(*receipt.Evidence.CheckLog)
	if err != nil {
		t.Fatalf("read check log: %v", err)
	}
	if !strings.Contains(string(checkLogData), "all tests passed successfully") {
		t.Fatalf("unexpected check log content: %q", string(checkLogData))
	}
}

func TestAccept_FallbackChecksFail(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeChecksScript(t, repoDir, 1, "test failed with error")

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "code.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package src\n"), 0644); err != nil {
		t.Fatal(err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictRejected {
		t.Fatalf("verdict = %q; want %q", verdict, lane.VerdictRejected)
	}
	if receipt.Evidence.CheckLog == nil {
		t.Fatal("expected evidence.CheckLog != nil on fallback check failure")
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "check failed:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason starting with 'check failed:', got: %v", reasons)
	}
}

func TestAccept_AttestationReuse(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	// Failing script that would fail if run
	writeChecksScript(t, repoDir, 1, "script should not be run when attestation is reused")

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "code.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package src\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Compute tree hash and write valid attestation
	finalTree, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("tree hash: %v", err)
	}

	commonDir, err := attest.RepoCommonDir(ctx, repoDir)
	if err != nil {
		t.Fatal(err)
	}
	repoID := attest.RepoID(commonDir)
	logDir, err := attest.ResolveStateDir(repoID)
	if err != nil {
		t.Fatal(err)
	}
	key, err := attest.LoadOrCreateKey("")
	if err != nil {
		t.Fatal(err)
	}

	attestEntry := attest.Entry{
		Version:    1,
		RepoID:     repoID,
		Command:    "sh lucind-checks.sh",
		ExitCode:   0,
		TreeHash:   finalTree,
		StartedAt:  time.Now().Add(-1 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
	}
	attestEntry.MAC = attest.ComputeMAC(attestEntry, key)
	attestPath, err := attest.WriteEntry(logDir, attestEntry)
	if err != nil {
		t.Fatalf("write attestation: %v", err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if receipt.Evidence.Attestation == nil {
		t.Fatal("expected evidence.Attestation != nil")
	}
	if *receipt.Evidence.Attestation != attestPath {
		t.Fatalf("attestation path = %q; want %q", *receipt.Evidence.Attestation, attestPath)
	}
	if receipt.Evidence.CheckLog != nil {
		t.Fatal("expected evidence.CheckLog == nil when attestation matched")
	}
}

func TestAccept_UnchangedRepoBaseEqualsFinalTree(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	// Ignore .lucind so lane files don't change tree hash
	gitignore := filepath.Join(repoDir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte(".lucind\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "add", ".gitignore")
	runGit(t, repoDir, "commit", "-m", "ignore .lucind")

	writeChecksScript(t, repoDir, 0, "pass")

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if receipt.ChangedFiles == nil || len(receipt.ChangedFiles) != 0 {
		t.Fatalf("expected empty changed_files slice, got: %#v", receipt.ChangedFiles)
	}
}

func TestRun_CliHelper(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	writeChecksScript(t, repoDir, 0, "pass")

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	// 1. Missing result.json -> Reject -> exit code 1
	var stdout, stderr bytes.Buffer
	code := accept.Run(ctx, repoDir, l.ID, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() = %d; want 1 on reject", code)
	}
	if !strings.Contains(stderr.String(), "rejected") {
		t.Fatalf("expected stderr to contain 'rejected', got: %q", stderr.String())
	}

	// 2. Add valid result.json -> Accept -> exit code 0
	writeResultJSON(t, repoDir, l.ID, "done")
	stdout.Reset()
	stderr.Reset()
	code = accept.Run(ctx, repoDir, l.ID, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() = %d; want 0 on accept (stderr: %q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "accepted") {
		t.Fatalf("expected stdout to contain 'accepted', got: %q", stdout.String())
	}

	// 3. Lane not found -> exit code 1
	stdout.Reset()
	stderr.Reset()
	code = accept.Run(ctx, repoDir, "20261003-120000-0000", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() = %d; want 1 on error", code)
	}
}
