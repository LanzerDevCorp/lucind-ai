package main

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

	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.name", "test")
	runGitCmd(t, dir, "config", "user.email", "test@example.com")
	filePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(filePath, []byte("hello stability\n"), 0o644); err != nil {
		t.Fatalf("write initial file: %v", err)
	}
	runGitCmd(t, dir, "add", "README.md")
	runGitCmd(t, dir, "commit", "-m", "initial commit")
	return dir
}

func runGitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command git %v failed in %s: %v: %s", args, dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func porcelainStatus(t *testing.T, dir string) string {
	t.Helper()
	return runGitCmd(t, dir, "status", "--porcelain")
}

// 2. RED: TestPreflightRejectsNonGitWorkingDir
func TestPreflightRejectsNonGitWorkingDir(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	cfg := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     "some-version",
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	res, err := Preflight(ctx, cfg)
	if err == nil && res.OK {
		t.Fatalf("Preflight on non-git dir = %+v, err = nil; want non-git rejection", res)
	}
	if res.OK {
		t.Fatalf("Preflight on non-git dir res.OK = true, want false")
	}
	if !strings.Contains(res.Reason, "not a git repository") && !strings.Contains(fmt.Sprintf("%v", err), "not a git repository") {
		t.Errorf("Preflight reason = %q (err = %v), want 'not a git repository'", res.Reason, err)
	}

	// Zero-write assertion
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("ReadDir(%s): %v", dir, readErr)
	}
	if len(entries) != 0 {
		t.Errorf("non-git dir has %d entries after failed preflight; want 0 writes", len(entries))
	}
}

// 3. RED: TestPreflightRejectsDirtyWorkingTreeStaged
func TestPreflightRejectsDirtyWorkingTreeStaged(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)

	// Stage a new file
	stagedFile := filepath.Join(dir, "staged.txt")
	if err := os.WriteFile(stagedFile, []byte("staged content\n"), 0o644); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	runGitCmd(t, dir, "add", "staged.txt")

	beforePorcelain := porcelainStatus(t, dir)
	if beforePorcelain == "" {
		t.Fatalf("expected dirty porcelain before test")
	}

	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")
	cfg := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	res, err := Preflight(ctx, cfg)
	if err == nil && res.OK {
		t.Fatalf("Preflight on staged dirty tree = %+v, err = nil; want dirty rejection", res)
	}
	if res.OK {
		t.Fatalf("Preflight on staged dirty tree res.OK = true, want false")
	}
	if !strings.Contains(res.Reason, "dirty") {
		t.Errorf("Preflight reason = %q, want 'dirty'", res.Reason)
	}

	// Assert zero writes: git status --porcelain unchanged
	afterPorcelain := porcelainStatus(t, dir)
	if afterPorcelain != beforePorcelain {
		t.Errorf("porcelain changed: before=%q, after=%q", beforePorcelain, afterPorcelain)
	}
}

// 3. RED: TestPreflightRejectsDirtyWorkingTreeUntracked
func TestPreflightRejectsDirtyWorkingTreeUntracked(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)

	// Create an untracked file
	untrackedFile := filepath.Join(dir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	beforePorcelain := porcelainStatus(t, dir)
	if beforePorcelain == "" {
		t.Fatalf("expected dirty porcelain before test")
	}

	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")
	cfg := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	res, err := Preflight(ctx, cfg)
	if err == nil && res.OK {
		t.Fatalf("Preflight on untracked dirty tree = %+v, err = nil; want dirty rejection", res)
	}
	if res.OK {
		t.Fatalf("Preflight on untracked dirty tree res.OK = true, want false")
	}
	if !strings.Contains(res.Reason, "dirty") {
		t.Errorf("Preflight reason = %q, want 'dirty'", res.Reason)
	}

	// Assert zero writes: git status --porcelain unchanged
	afterPorcelain := porcelainStatus(t, dir)
	if afterPorcelain != beforePorcelain {
		t.Errorf("porcelain changed: before=%q, after=%q", beforePorcelain, afterPorcelain)
	}
}

// 3. RED: TestPreflightRejectsDirtyWorkingTreeModified
func TestPreflightRejectsDirtyWorkingTreeModified(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)

	// Modify tracked file
	trackedFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(trackedFile, []byte("modified content\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}

	beforePorcelain := porcelainStatus(t, dir)
	if beforePorcelain == "" {
		t.Fatalf("expected dirty porcelain before test")
	}

	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")
	cfg := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	res, err := Preflight(ctx, cfg)
	if err == nil && res.OK {
		t.Fatalf("Preflight on modified dirty tree = %+v, err = nil; want dirty rejection", res)
	}
	if res.OK {
		t.Fatalf("Preflight on modified dirty tree res.OK = true, want false")
	}
	if !strings.Contains(res.Reason, "dirty") {
		t.Errorf("Preflight reason = %q, want 'dirty'", res.Reason)
	}

	// Assert zero writes: git status --porcelain unchanged
	afterPorcelain := porcelainStatus(t, dir)
	if afterPorcelain != beforePorcelain {
		t.Errorf("porcelain changed: before=%q, after=%q", beforePorcelain, afterPorcelain)
	}
}

// 4. Linux-only gate
func TestPreflightRejectsNonLinuxOS(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir() // non-git directory that would also fail downstream checks

	cfg := PreflightConfig{
		GOOS:        "darwin",
		PrimaryRoot: dir,
		Version:     "dev",
	}

	res, err := Preflight(ctx, cfg)
	if err == nil && res.OK {
		t.Fatalf("Preflight on non-Linux OS = %+v, want Linux rejection", res)
	}
	if res.OK {
		t.Fatalf("Preflight on non-Linux OS res.OK = true, want false")
	}
	if !strings.Contains(res.Reason, "linux") {
		t.Errorf("Preflight reason = %q, want it to specify 'linux' OS requirement", res.Reason)
	}
	// Assert it did NOT fail for downstream reason (non-git repo)
	if strings.Contains(res.Reason, "git") {
		t.Errorf("Preflight reason = %q, should have failed on OS check before reaching git check", res.Reason)
	}
}

// 5. Candidate HEAD build match
func TestPreflightRejectsVersionMismatch(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	cfg := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     "dev",
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	res, err := Preflight(ctx, cfg)
	if err == nil && res.OK {
		t.Fatalf("Preflight with dev version = %+v, want mismatch rejection", res)
	}
	if res.OK {
		t.Fatalf("Preflight with dev version res.OK = true, want false")
	}
	if !strings.Contains(res.Reason, "make install") {
		t.Errorf("Preflight reason = %q, want mention of 'make install'", res.Reason)
	}
	if !strings.Contains(res.Reason, headSHA) {
		t.Errorf("Preflight reason = %q, want mention of headSHA %q", res.Reason, headSHA)
	}
	if !strings.Contains(res.Reason, "dev") {
		t.Errorf("Preflight reason = %q, want mention of version 'dev'", res.Reason)
	}
}

func TestPreflightAcceptsMatchingVersion(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	// Test with prefix
	cfg := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA[:7],
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	res, err := Preflight(ctx, cfg)
	if err != nil || !res.OK {
		t.Fatalf("Preflight with matching prefix version = %+v, err = %v; want OK", res, err)
	}
}

// 6. Baseline check
func TestPreflightBaselineCheck(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	// Failed baseline check
	cfgFail := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) {
			return false, "tests failed on baseline", nil
		},
	}

	resFail, _ := Preflight(ctx, cfgFail)
	if resFail.OK {
		t.Fatalf("Preflight with failing baseline = %+v, want rejection", resFail)
	}
	if !strings.Contains(resFail.Reason, "baseline") {
		t.Errorf("Preflight reason = %q, want 'baseline' mention", resFail.Reason)
	}

	// Passing baseline check
	cfgPass := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) {
			return true, "all checks passed", nil
		},
	}

	resPass, errPass := Preflight(ctx, cfgPass)
	if errPass != nil || !resPass.OK {
		t.Fatalf("Preflight with passing baseline = %+v, err = %v; want OK", resPass, errPass)
	}
}

// 7. Zero active campaigns
func TestPreflightChecksZeroActiveCampaigns(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	// Open real SQLite store under repo's git common dir
	st, err := store.Open(ctx, worktree.DefaultGitRunner, dir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	cfg := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	// 1. Empty store (0 active campaigns): passes
	res, err := Preflight(ctx, cfg)
	if err != nil || !res.OK {
		t.Fatalf("Preflight with empty store = %+v, err = %v; want OK", res, err)
	}

	// 2. Create an active campaign: fails
	camp, err := st.CreateCampaign(ctx, "camp-123", headSHA)
	if err != nil {
		t.Fatalf("CreateCampaign failed: %v", err)
	}

	resActive, _ := Preflight(ctx, cfg)
	if resActive.OK {
		t.Fatalf("Preflight with active campaign = %+v, want rejection", resActive)
	}
	if !strings.Contains(resActive.Reason, "active campaign") {
		t.Errorf("Preflight reason = %q, want 'active campaign' mention", resActive.Reason)
	}

	// 3. Mark campaign as closed (failed): passes again
	if err := st.UpdateCampaignStatus(ctx, camp.ID, store.StatusFailed); err != nil {
		t.Fatalf("UpdateCampaignStatus failed: %v", err)
	}

	resClosed, errClosed := Preflight(ctx, cfg)
	if errClosed != nil || !resClosed.OK {
		t.Fatalf("Preflight with closed campaign = %+v, err = %v; want OK", resClosed, errClosed)
	}
}

// 8. agy / model availability
func TestPreflightAgyAvailability(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	// Missing agy binary
	cfgNoAgy := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "", exec.ErrNotFound },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
	}

	resNoAgy, _ := Preflight(ctx, cfgNoAgy)
	if resNoAgy.OK {
		t.Fatalf("Preflight with missing agy = %+v, want rejection", resNoAgy)
	}
	if !strings.Contains(resNoAgy.Reason, "agy") {
		t.Errorf("Preflight reason = %q, want 'agy' mention", resNoAgy.Reason)
	}

	// Missing model in executor.Agy{}.KnownModels()
	cfgNoModel := PreflightConfig{
		GOOS:        "linux",
		PrimaryRoot: dir,
		Version:     headSHA,
		LookPath:    func(string) (string, error) { return "/bin/agy", nil },
		CheckRunner: func(context.Context, string) (bool, string, error) { return true, "ok", nil },
		KnownModels: []string{"some-other-model"},
	}

	resNoModel, _ := Preflight(ctx, cfgNoModel)
	if resNoModel.OK {
		t.Fatalf("Preflight with missing model = %+v, want rejection", resNoModel)
	}
	if !strings.Contains(resNoModel.Reason, "gemini-3.7-flash-high") {
		t.Errorf("Preflight reason = %q, want 'gemini-3.7-flash-high' mention", resNoModel.Reason)
	}
}

// 9. stability status [--json]
func TestStabilityStatusOutput(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	st, err := store.Open(ctx, worktree.DefaultGitRunner, dir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	// 1. Status with no active campaign (human-readable)
	var stdout, stderr bytes.Buffer
	code := runStabilityStatusWithDir(ctx, []string{}, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("runStabilityStatus exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no active stability campaign") {
		t.Errorf("stdout = %q, want 'no active stability campaign'", stdout.String())
	}

	// 2. Status with no active campaign (--json)
	stdout.Reset()
	stderr.Reset()
	code = runStabilityStatusWithDir(ctx, []string{"--json"}, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("runStabilityStatus --json exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	var jsonOut StabilityStatusOutput
	if err := json.Unmarshal(stdout.Bytes(), &jsonOut); err != nil {
		t.Fatalf("unmarshal json status: %v (raw: %s)", err, stdout.String())
	}
	if jsonOut.Active {
		t.Errorf("jsonOut.Active = true, want false")
	}

	// 3. Create active campaign and check status (--json)
	camp, err := st.CreateCampaign(ctx, "camp-abc-123", headSHA)
	if err != nil {
		t.Fatalf("CreateCampaign failed: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = runStabilityStatusWithDir(ctx, []string{"--json"}, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("runStabilityStatus --json exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &jsonOut); err != nil {
		t.Fatalf("unmarshal json status with active campaign: %v (raw: %s)", err, stdout.String())
	}
	if !jsonOut.Active {
		t.Errorf("jsonOut.Active = false, want true")
	}
	if jsonOut.CampaignID != camp.ID {
		t.Errorf("jsonOut.CampaignID = %q, want %q", jsonOut.CampaignID, camp.ID)
	}
	if jsonOut.CandidateSHA != headSHA {
		t.Errorf("jsonOut.CandidateSHA = %q, want %q", jsonOut.CandidateSHA, headSHA)
	}
	if jsonOut.Status != string(store.StatusRunning) {
		t.Errorf("jsonOut.Status = %q, want %q", jsonOut.Status, store.StatusRunning)
	}

	// 4. Status with active campaign (human-readable)
	stdout.Reset()
	stderr.Reset()
	code = runStabilityStatusWithDir(ctx, []string{}, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("runStabilityStatus human exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), camp.ID) || !strings.Contains(stdout.String(), headSHA) {
		t.Errorf("stdout = %q, want to contain campaign ID and candidate SHA", stdout.String())
	}
}

// 10. stability resume / stability abort
func TestStabilityResumeAndAbort(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	st, err := store.Open(ctx, worktree.DefaultGitRunner, dir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	camp, err := st.CreateCampaign(ctx, "camp-resume-test", headSHA)
	if err != nil {
		t.Fatalf("CreateCampaign failed: %v", err)
	}

	// Resume on clean state -> safe -> exit 0
	var stdout, stderr bytes.Buffer
	code := runStabilityResumeWithDir(ctx, []string{}, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("runStabilityResume exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "safe") {
		t.Errorf("resume stdout = %q, want 'safe'", stdout.String())
	}

	// Abort on active campaign -> cleans resources -> status becomes failed -> exit 0
	stdout.Reset()
	stderr.Reset()
	code = runStabilityAbortWithDir(ctx, []string{}, &stdout, &stderr, dir)
	if code != 0 {
		t.Fatalf("runStabilityAbort exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "failed") && !strings.Contains(stdout.String(), "aborted") {
		t.Errorf("abort stdout = %q, want status failed/aborted", stdout.String())
	}

	// Confirm campaign is marked failed in store
	updatedCamp, err := st.GetCampaign(ctx, camp.ID)
	if err != nil {
		t.Fatalf("GetCampaign failed: %v", err)
	}
	if updatedCamp.Status != store.StatusFailed {
		t.Errorf("updatedCamp.Status = %q, want %q", updatedCamp.Status, store.StatusFailed)
	}
}

func TestStabilityResumeFailClosed(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	st, err := store.Open(ctx, worktree.DefaultGitRunner, dir)
	if err != nil {
		t.Fatalf("store.Open failed: %v", err)
	}
	defer st.Close()

	// Create campaign already in non-running / terminal status
	camp, err := st.CreateCampaign(ctx, "camp-fail-closed", headSHA)
	if err != nil {
		t.Fatalf("CreateCampaign failed: %v", err)
	}
	if err := st.UpdateCampaignStatus(ctx, camp.ID, store.StatusFailed); err != nil {
		t.Fatalf("UpdateCampaignStatus failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runStabilityResumeWithDir(ctx, []string{}, &stdout, &stderr, dir)
	if code == 0 {
		t.Fatalf("runStabilityResume on failed campaign exit = 0, want non-zero (1)")
	}
	if !strings.Contains(stdout.String(), "fail_closed") && !strings.Contains(stderr.String(), "fail_closed") && !strings.Contains(stderr.String(), "not found") {
		t.Errorf("stdout = %q, stderr = %q, want fail_closed or not found", stdout.String(), stderr.String())
	}
}

// 11. Routing in cli.go
func TestRunStabilityRouting(t *testing.T) {
	ctx := context.Background()

	// Missing stability action
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"stability"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run(stability) exit = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "requires an action") {
		t.Errorf("stderr = %q, want 'requires an action'", stderr.String())
	}

	// Unknown stability action
	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"stability", "unknown-action"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run(stability unknown-action) exit = 0, want non-zero")
	}
	if !strings.Contains(stderr.String(), "unknown stability subcommand") {
		t.Errorf("stderr = %q, want 'unknown stability subcommand'", stderr.String())
	}

	// Reachable via run() directly from a primary git repository
	dir := initTestGitRepo(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	stdout.Reset()
	stderr.Reset()
	code = run(ctx, []string{"stability", "status", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(stability status --json) exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	var statusOut StabilityStatusOutput
	if err := json.Unmarshal(stdout.Bytes(), &statusOut); err != nil {
		t.Fatalf("failed to unmarshal JSON from run(stability status --json): %v (raw: %s)", err, stdout.String())
	}
}

// stability run stub / placeholder
func TestStabilityRunPreflightStub(t *testing.T) {
	ctx := context.Background()
	dir := initTestGitRepo(t)

	// Write passing lucind-checks.sh and commit it so working tree is clean
	checksScript := filepath.Join(dir, "lucind-checks.sh")
	if err := os.WriteFile(checksScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write checks script: %v", err)
	}
	runGitCmd(t, dir, "add", "lucind-checks.sh")
	runGitCmd(t, dir, "commit", "-m", "add checks script")

	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")

	// Inject matching version
	oldVersion := version
	version = headSHA
	defer func() { version = oldVersion }()

	var stdout, stderr bytes.Buffer
	code := runStabilityRunWithDir(ctx, []string{}, &stdout, &stderr, dir, func(string) (string, error) {
		return "/bin/agy", nil
	})
	if code != 0 {
		t.Fatalf("runStabilityRunWithDir exit = %d, want 0; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "preflight passed") {
		t.Errorf("stdout = %q, want 'preflight passed'", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Wave 4b") && !strings.Contains(stdout.String(), "not yet implemented") {
		t.Errorf("stdout = %q, want stub notice for Wave 4b", stdout.String())
	}
}
