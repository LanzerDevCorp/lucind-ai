package accept_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	l, err := lane.Load(repoDir, laneID)
	if err != nil {
		t.Fatalf("load lane %s: %v", laneID, err)
	}
	env := fmt.Sprintf(`{
  "lane_id": %q,
  "status": %q,
  "summary": "Completed lane work.",
  "hard_stops": []
}`, laneID, status)
	resPath := lane.ResultFilePath(repoDir, l)
	if err := os.MkdirAll(filepath.Dir(resPath), 0755); err != nil {
		t.Fatalf("mkdir lane dir: %v", err)
	}
	if err := os.WriteFile(resPath, []byte(env), 0644); err != nil {
		t.Fatalf("write result file: %v", err)
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
		t.Fatal("expected rejection reasons for missing result file")
	}
	expectedFile := lane.ResultFileName(l.Turn)
	found := false
	for _, r := range reasons {
		if strings.Contains(r, expectedFile) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason mentioning %s, got: %v", expectedFile, reasons)
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

	resPath := lane.ResultFilePath(repoDir, l)
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
	expectedFile := lane.ResultFileName(l.Turn)
	found := false
	for _, r := range reasons {
		if strings.Contains(r, expectedFile) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason mentioning %s, got: %v", expectedFile, reasons)
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

	checkCmd := "sh lucind-checks.sh"
	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", checkCmd)
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
	if len(receipt.Evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %d", len(receipt.Evidence))
	}
	if receipt.Evidence[0].Check != checkCmd {
		t.Errorf("evidence[0].Check = %q, want %q", receipt.Evidence[0].Check, checkCmd)
	}
	if receipt.Evidence[0].Attestation != nil {
		t.Fatal("expected evidence.Attestation == nil when running check")
	}
	if receipt.Evidence[0].CheckLog == nil {
		t.Fatal("expected evidence.CheckLog != nil")
	}
	logPath := *receipt.Evidence[0].CheckLog
	if !strings.HasSuffix(filepath.ToSlash(logPath), "check-0.log") {
		t.Errorf("expected check log to end with check-0.log, got %q", logPath)
	}
	checkLogData, err := os.ReadFile(logPath)
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

	checkCmd := "sh lucind-checks.sh"
	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", checkCmd)
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
	if len(receipt.Evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %d", len(receipt.Evidence))
	}
	if receipt.Evidence[0].CheckLog == nil {
		t.Fatal("expected evidence.CheckLog != nil on check failure")
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, checkCmd) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason naming check %q, got: %v", checkCmd, reasons)
	}
}

func TestAccept_AttestationReuse(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	// Failing script that would fail if run
	writeChecksScript(t, repoDir, 1, "script should not be run when attestation is reused")

	checkCmd := "sh lucind-checks.sh"
	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", checkCmd)
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

	// Compute tree hash and write valid attestation for lane.CheckCommand(checkCmd)
	finalTree, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("tree hash: %v", err)
	}

	attestPath := writeCheckAttestation(t, repoDir, lane.CheckCommand(checkCmd), finalTree)

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(receipt.Evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %d", len(receipt.Evidence))
	}
	if receipt.Evidence[0].Check != checkCmd {
		t.Errorf("evidence[0].Check = %q, want %q", receipt.Evidence[0].Check, checkCmd)
	}
	if receipt.Evidence[0].Attestation == nil {
		t.Fatal("expected evidence.Attestation != nil")
	}
	if *receipt.Evidence[0].Attestation != attestPath {
		t.Fatalf("attestation path = %q; want %q", *receipt.Evidence[0].Attestation, attestPath)
	}
	if receipt.Evidence[0].CheckLog != nil {
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

func writeCheckAttestation(t *testing.T, repoDir, command, treeHash string) string {
	t.Helper()
	commonDir, err := attest.RepoCommonDir(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("repo common dir: %v", err)
	}
	repoID := attest.RepoID(commonDir)
	logDir, err := attest.ResolveStateDir(repoID)
	if err != nil {
		t.Fatalf("resolve state dir: %v", err)
	}
	key, err := attest.LoadOrCreateKey("")
	if err != nil {
		t.Fatalf("load or create key: %v", err)
	}

	attestEntry := attest.Entry{
		Version:    1,
		RepoID:     repoID,
		Command:    command,
		ExitCode:   0,
		TreeHash:   treeHash,
		StartedAt:  time.Now().Add(-1 * time.Second).Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
	}
	attestEntry.MAC = attest.ComputeMAC(attestEntry, key)
	attestPath, err := attest.WriteEntry(logDir, attestEntry)
	if err != nil {
		t.Fatalf("write attestation: %v", err)
	}
	return attestPath
}

func TestAccept_ZeroChecksAccepted(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	// Lane with zero checks
	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "app.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package app\n"), 0644); err != nil {
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
		t.Fatalf("expected 0 reasons, got: %v", reasons)
	}
	if len(receipt.Evidence) != 0 {
		t.Fatalf("expected empty evidence for 0 checks, got: %v", receipt.Evidence)
	}

	savedLane, err := lane.Load(repoDir, l.ID)
	if err != nil {
		t.Fatalf("load lane: %v", err)
	}
	if savedLane.Status != lane.StatusAccepted {
		t.Fatalf("lane.Status = %q; want %q", savedLane.Status, lane.StatusAccepted)
	}
}

func TestAccept_OneCheckValidAttestationAccepted(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	// Command that would fail if executed
	checkCmd := "exit 1"
	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", checkCmd)
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "app.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	finalTree, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("tree hash: %v", err)
	}

	attestPath := writeCheckAttestation(t, repoDir, lane.CheckCommand(checkCmd), finalTree)

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(receipt.Evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %d", len(receipt.Evidence))
	}
	if receipt.Evidence[0].Check != checkCmd {
		t.Errorf("evidence[0].Check = %q, want %q", receipt.Evidence[0].Check, checkCmd)
	}
	if receipt.Evidence[0].Attestation == nil {
		t.Fatal("expected evidence[0].Attestation != nil")
	}
	if *receipt.Evidence[0].Attestation != attestPath {
		t.Errorf("evidence[0].Attestation = %q, want %q", *receipt.Evidence[0].Attestation, attestPath)
	}
	if receipt.Evidence[0].CheckLog != nil {
		t.Errorf("expected evidence[0].CheckLog == nil, got %v", *receipt.Evidence[0].CheckLog)
	}
}

func TestAccept_OneCheckWithoutAttestationRunAndAccepted(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	checkCmd := "echo 'pass check 1'"
	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", checkCmd)
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "app.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(receipt.Evidence) != 1 {
		t.Fatalf("expected 1 evidence entry, got %d", len(receipt.Evidence))
	}
	if receipt.Evidence[0].Check != checkCmd {
		t.Errorf("evidence[0].Check = %q, want %q", receipt.Evidence[0].Check, checkCmd)
	}
	if receipt.Evidence[0].Attestation != nil {
		t.Errorf("expected evidence[0].Attestation == nil, got %v", *receipt.Evidence[0].Attestation)
	}
	if receipt.Evidence[0].CheckLog == nil {
		t.Fatal("expected evidence[0].CheckLog != nil")
	}
	logPath := *receipt.Evidence[0].CheckLog
	logContent, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read check log: %v", err)
	}
	if !strings.Contains(string(logContent), "pass check 1") {
		t.Errorf("log content does not contain %q: %s", "pass check 1", string(logContent))
	}
	laneDir := lane.LaneDir(repoDir, l.ID)
	if !strings.HasPrefix(filepath.Clean(logPath), filepath.Clean(laneDir)) {
		t.Errorf("log path %q not under lane dir %q", logPath, laneDir)
	}
}

func TestAccept_FailingCheckRejectedNamingCheck(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	checkCmd := "echo 'failing check' && exit 2"
	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", checkCmd)
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "app.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package app\n"), 0644); err != nil {
		t.Fatal(err)
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
		if strings.Contains(r, checkCmd) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason naming check %q, got: %v", checkCmd, reasons)
	}
}

func TestAccept_TwoChecksOneAttestedRunsOnlyMissing(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	check1 := "exit 42"                         // attested, would fail if executed
	check2 := "echo 'check 2 ran successfully'" // not attested, runs and passes

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", check1, check2)
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "app.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	finalTree, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("tree hash: %v", err)
	}

	attestPath := writeCheckAttestation(t, repoDir, lane.CheckCommand(check1), finalTree)

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(receipt.Evidence) != 2 {
		t.Fatalf("expected 2 evidence entries, got %d", len(receipt.Evidence))
	}

	// Check 1: attested, not run
	if receipt.Evidence[0].Check != check1 {
		t.Errorf("evidence[0].Check = %q, want %q", receipt.Evidence[0].Check, check1)
	}
	if receipt.Evidence[0].Attestation == nil || *receipt.Evidence[0].Attestation != attestPath {
		t.Errorf("evidence[0].Attestation = %v, want %q", receipt.Evidence[0].Attestation, attestPath)
	}
	if receipt.Evidence[0].CheckLog != nil {
		t.Errorf("expected evidence[0].CheckLog == nil, got %v", *receipt.Evidence[0].CheckLog)
	}

	// Check 2: run, not attested
	if receipt.Evidence[1].Check != check2 {
		t.Errorf("evidence[1].Check = %q, want %q", receipt.Evidence[1].Check, check2)
	}
	if receipt.Evidence[1].Attestation != nil {
		t.Errorf("expected evidence[1].Attestation == nil, got %v", *receipt.Evidence[1].Attestation)
	}
	if receipt.Evidence[1].CheckLog == nil {
		t.Fatal("expected evidence[1].CheckLog != nil")
	}
	logContent, err := os.ReadFile(*receipt.Evidence[1].CheckLog)
	if err != nil {
		t.Fatalf("read check 2 log: %v", err)
	}
	if !strings.Contains(string(logContent), "check 2 ran successfully") {
		t.Errorf("log content does not contain expected output: %s", string(logContent))
	}
}

func TestAccept_ReceiptEvidencePerCheck(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	check1 := "echo 'check 1 result'"
	check2 := "echo 'check 2 result'"

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model", check1, check2)
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	writeResultJSON(t, repoDir, l.ID, "done")

	srcFile := filepath.Join(repoDir, "src", "app.go")
	if err := os.MkdirAll(filepath.Dir(srcFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcFile, []byte("package app\n"), 0644); err != nil {
		t.Fatal(err)
	}

	finalTree, err := attest.TreeHash(ctx, repoDir)
	if err != nil {
		t.Fatalf("tree hash: %v", err)
	}

	attestPath := writeCheckAttestation(t, repoDir, lane.CheckCommand(check1), finalTree)

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(receipt.Evidence) != 2 {
		t.Fatalf("expected 2 evidence entries matching checks, got %d", len(receipt.Evidence))
	}
	if receipt.Evidence[0].Check != check1 {
		t.Errorf("evidence[0].Check = %q, want %q", receipt.Evidence[0].Check, check1)
	}
	if receipt.Evidence[0].Attestation == nil || *receipt.Evidence[0].Attestation != attestPath {
		t.Errorf("evidence[0].Attestation = %v, want %q", receipt.Evidence[0].Attestation, attestPath)
	}
	if receipt.Evidence[1].Check != check2 {
		t.Errorf("evidence[1].Check = %q, want %q", receipt.Evidence[1].Check, check2)
	}
	if receipt.Evidence[1].CheckLog == nil {
		t.Fatal("expected evidence[1].CheckLog != nil")
	}

	// Verify receipt on disk matches via LoadReceipt
	loaded, err := lane.LoadReceipt(repoDir, l.ID)
	if err != nil {
		t.Fatalf("load receipt: %v", err)
	}
	if len(loaded.Evidence) != 2 {
		t.Fatalf("loaded receipt evidence len = %d, want 2", len(loaded.Evidence))
	}
	if loaded.Evidence[0].Check != check1 || loaded.Evidence[0].Attestation == nil || *loaded.Evidence[0].Attestation != attestPath {
		t.Errorf("loaded evidence[0] mismatch: %+v", loaded.Evidence[0])
	}
	if loaded.Evidence[1].Check != check2 || loaded.Evidence[1].CheckLog == nil || *loaded.Evidence[1].CheckLog != *receipt.Evidence[1].CheckLog {
		t.Errorf("loaded evidence[1] mismatch: %+v", loaded.Evidence[1])
	}

	// Verify raw receipt.json
	data, err := os.ReadFile(lane.ReceiptPath(repoDir, l.ID))
	if err != nil {
		t.Fatalf("read receipt.json: %v", err)
	}
	var raw struct {
		Evidence []map[string]any `json:"evidence"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal receipt.json: %v", err)
	}
	if len(raw.Evidence) != 2 {
		t.Fatalf("raw evidence array len = %d, want 2", len(raw.Evidence))
	}
}

func TestAccept_Turn2_MissingResult2Refused(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	// Write valid result for turn 1
	writeResultJSON(t, repoDir, l.ID, "done")

	// Advance lane to turn 2
	l.Turn = 2
	if err := lane.Save(repoDir, l); err != nil {
		t.Fatalf("save lane: %v", err)
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
	expectedReason := fmt.Sprintf("%s missing", lane.ResultFileName(2))
	found := false
	for _, r := range reasons {
		if strings.Contains(r, expectedReason) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reason containing %q, got: %v", expectedReason, reasons)
	}
}

func TestAccept_Turn2_ValidResultAccepted(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	// Write valid result for turn 1
	writeResultJSON(t, repoDir, l.ID, "done")

	// Advance lane to turn 2
	l.Turn = 2
	if err := lane.Save(repoDir, l); err != nil {
		t.Fatalf("save lane: %v", err)
	}

	// Write valid result for turn 2
	writeResultJSON(t, repoDir, l.ID, "done")

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(reasons) != 0 {
		t.Fatalf("expected 0 reasons, got: %v", reasons)
	}
	if receipt.Verdict != lane.VerdictAccepted {
		t.Fatalf("receipt.Verdict = %q; want %q", receipt.Verdict, lane.VerdictAccepted)
	}
}

func TestAccept_TurnLegacy_ValidResultAccepted(t *testing.T) {
	ctx := context.Background()
	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	l, err := lane.Create(ctx, repoDir, []string{"src/**"}, "test-model")
	if err != nil {
		t.Fatalf("create lane: %v", err)
	}

	// Legacy lane with Turn == 0
	l.Turn = 0
	if err := lane.Save(repoDir, l); err != nil {
		t.Fatalf("save lane: %v", err)
	}

	// Write valid result (for Turn == 0, ResultFilePath writes result.json)
	writeResultJSON(t, repoDir, l.ID, "done")

	verdict, receipt, reasons, err := accept.Accept(ctx, repoDir, l.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != lane.VerdictAccepted {
		t.Fatalf("verdict = %q; want %q (reasons: %v)", verdict, lane.VerdictAccepted, reasons)
	}
	if len(reasons) != 0 {
		t.Fatalf("expected 0 reasons, got: %v", reasons)
	}
	if receipt.Verdict != lane.VerdictAccepted {
		t.Fatalf("receipt.Verdict = %q; want %q", receipt.Verdict, lane.VerdictAccepted)
	}
}
