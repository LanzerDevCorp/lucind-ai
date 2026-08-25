package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/evidence"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/fixture"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/reconcile"
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

// fakeJourneyExecutor is a test double for running trial journey packets.
type fakeJourneyExecutor struct {
	mu         sync.Mutex
	calls      map[string]int
	outcomes   map[string]executor.Outcome
	runFuncFor map[string]func(ctx context.Context, req executor.Request) (executor.Outcome, error)
}

func newFakeJourneyExecutor() *fakeJourneyExecutor {
	return &fakeJourneyExecutor{
		calls:      make(map[string]int),
		outcomes:   make(map[string]executor.Outcome),
		runFuncFor: make(map[string]func(ctx context.Context, req executor.Request) (executor.Outcome, error)),
	}
}

func (f *fakeJourneyExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	f.mu.Lock()
	f.calls[req.WorktreePath]++
	customFn := f.runFuncFor[req.WorktreePath]
	outcome, ok := f.outcomes[req.WorktreePath]
	f.mu.Unlock()

	if customFn != nil {
		return customFn(ctx, req)
	}
	if ok {
		return outcome, nil
	}
	return executor.Outcome{ExitCode: 0}, nil
}

func (f *fakeJourneyExecutor) DefaultModel() string {
	return stability.PinnedModel
}

func (f *fakeJourneyExecutor) KnownModels() []string {
	return []string{stability.PinnedModel}
}

func (f *fakeJourneyExecutor) CallCount(worktreePath string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[worktreePath]
}

// laneEnvelopeJSON returns a minimal envelope satisfying result.schema.json.
func laneEnvelopeJSON(id, status string) string {
	if status == "blocked" {
		return fmt.Sprintf(`{
			"packet_id": %q,
			"status": "blocked",
			"summary": "hit a hard stop",
			"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": true, "note": "would have had to edit it"}]
		}`, id)
	}
	return fmt.Sprintf(`{
		"packet_id": %q,
		"status": "done",
		"summary": "completed work cleanly",
		"hard_stops": [{"hard_stop": "do not touch internal/barrier", "fired": false}]
	}`, id)
}

func setupCleanTestRepoForStability(t *testing.T) (string, string) {
	t.Helper()
	dir := initTestGitRepo(t)
	checksScript := filepath.Join(dir, "lucind-checks.sh")
	if err := os.WriteFile(checksScript, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write checks script: %v", err)
	}
	runGitCmd(t, dir, "add", "lucind-checks.sh")
	runGitCmd(t, dir, "commit", "-m", "add checks script")
	headSHA := runGitCmd(t, dir, "rev-parse", "HEAD")
	return dir, headSHA
}

func newSimulatedTestDeps(t *testing.T, primaryRoot string, execEnv executor.Executor) (lucindrun.Deps, *ledger.Ledger) {
	t.Helper()
	l, err := ledger.Open(context.Background(), primaryRoot)
	if err != nil {
		t.Fatalf("ledger.Open error = %v", err)
	}
	t.Cleanup(func() { l.Close() })

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)

	deps := lucindrun.Deps{
		RunID:       "run-stability-simulated",
		PrimaryRoot: primaryRoot,
		Ledger:      l,
		LookupExecutor: func(name string) (executor.Executor, error) {
			return execEnv, nil
		},
		CreateWorktree: func(_ context.Context, _, laneID, _, baseSHA string) (worktree.Worktree, error) {
			wtPath := reconcile.WorktreePathFor(primaryRoot, laneID)
			lucindPath := filepath.Join(wtPath, ".lucind")
			if err := os.MkdirAll(lucindPath, 0o755); err != nil {
				return worktree.Worktree{}, err
			}

			runGit := func(args ...string) {
				cmd := exec.Command("git", append([]string{"-C", wtPath}, args...)...)
				_ = cmd.Run()
			}
			runGit("init")
			runGit("config", "user.name", "Test User")
			runGit("config", "user.email", "test@example.com")
			_ = fixture.MaterializeFixtures(wtPath)
			runGit("add", ".")
			runGit("commit", "-m", "initial baseline")

			cmd := exec.Command("git", "-C", wtPath, "rev-parse", "HEAD")
			out, _ := cmd.Output()
			actualBase := strings.TrimSpace(string(out))
			if baseSHA == "" {
				baseSHA = actualBase
			}

			if laneID == "stability-change-a" {
				_ = os.WriteFile(filepath.Join(wtPath, "fixture", "change_a.txt"), []byte("CHANGE_A=DONE\n"), 0o644)
			} else if laneID == "stability-change-b" {
				_ = os.WriteFile(filepath.Join(wtPath, "fixture", "change_b.txt"), []byte("CHANGE_B=DONE\n"), 0o644)
			} else if laneID == "stability-fix-a" {
				_ = os.WriteFile(filepath.Join(wtPath, "fixture", "defect.txt"), []byte("STATUS=FIXED\n"), 0o644)
			}
			runGit("add", ".")
			runGit("commit", "-m", "lane change for "+laneID)

			envJSON := laneEnvelopeJSON(laneID, "done")
			if err := os.WriteFile(filepath.Join(lucindPath, "result.json"), []byte(envJSON), 0o644); err != nil {
				return worktree.Worktree{}, err
			}
			return worktree.Worktree{
				Path:    wtPath,
				Branch:  "lucind/" + laneID,
				BaseSHA: actualBase,
			}, nil
		},
		WorktreeFS: func(path string) fs.FS {
			return os.DirFS(path)
		},
		Now: func() time.Time {
			return now
		},
		HasUniqueLaneCommits: func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return true, nil
		},
		PorcelainEmpty: func(ctx context.Context, worktreePath string) (bool, error) {
			return true, nil
		},
	}

	return deps, l
}

// Part 1: Prohibited flag rejection tests
func TestStabilityRunRejectsYesFlag(t *testing.T) {
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := runStabilityRunWithDir(ctx, []string{"--yes"}, strings.NewReader(""), &stdout, &stderr, "", nil)
	if code != 1 {
		t.Fatalf("runStabilityRunWithDir(--yes) code = %d, want 1; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("stderr = %q, want unrecognized flag error for --yes", stderr.String())
	}
}

func TestStabilityRunRejectsTagFlag(t *testing.T) {
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := runStabilityRunWithDir(ctx, []string{"--tag"}, strings.NewReader(""), &stdout, &stderr, "", nil)
	if code != 1 {
		t.Fatalf("runStabilityRunWithDir(--tag) code = %d, want 1; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("stderr = %q, want unrecognized flag error for --tag", stderr.String())
	}
}

func TestStabilityRunRejectsPushFlag(t *testing.T) {
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := runStabilityRunWithDir(ctx, []string{"--push"}, strings.NewReader(""), &stdout, &stderr, "", nil)
	if code != 1 {
		t.Fatalf("runStabilityRunWithDir(--push) code = %d, want 1; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("stderr = %q, want unrecognized flag error for --push", stderr.String())
	}
}

func TestStabilityRunRejectsReleaseFlag(t *testing.T) {
	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	code := runStabilityRunWithDir(ctx, []string{"--release"}, strings.NewReader(""), &stdout, &stderr, "", nil)
	if code != 1 {
		t.Fatalf("runStabilityRunWithDir(--release) code = %d, want 1; stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "flag provided but not defined") {
		t.Errorf("stderr = %q, want unrecognized flag error for --release", stderr.String())
	}
}

// Part 1: Interactive confirmation tests
func TestStabilityRunConfirmation(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name         string
		stdinInput   string
		wantExitCode int
		wantDeclined bool
	}{
		{
			name:         "declines on empty stdin",
			stdinInput:   "",
			wantExitCode: 1,
			wantDeclined: true,
		},
		{
			name:         "declines on garbage input maybe",
			stdinInput:   "maybe\n",
			wantExitCode: 1,
			wantDeclined: true,
		},
		{
			name:         "declines on explicit no",
			stdinInput:   "no\n",
			wantExitCode: 1,
			wantDeclined: true,
		},
		{
			name:         "proceeds on lowercase y",
			stdinInput:   "y\n",
			wantExitCode: 0,
			wantDeclined: false,
		},
		{
			name:         "proceeds on uppercase YES",
			stdinInput:   "YES\n",
			wantExitCode: 0,
			wantDeclined: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir, headSHA := setupCleanTestRepoForStability(t)
			oldVersion := version
			version = headSHA
			defer func() { version = oldVersion }()

			var stdout, stderr bytes.Buffer
			code := runStabilityRunWithConfig(ctx, []string{}, strings.NewReader(tc.stdinInput), &stdout, &stderr, PreflightConfig{
				PrimaryRoot: dir,
				LookPath: func(string) (string, error) {
					return "/bin/agy", nil
				},
			}, CampaignConfig{
				CheckRunner: func(ctx context.Context, dir string) (bool, string, error) {
					return true, "PASS", nil
				},
				ExecuteJourney: func(ctx context.Context, sm *stability.StateMachine, deps lucindrun.Deps, featSvc *feature.Service, cfg stability.JourneyConfig, pgidB int, wtBPath string) (*stability.TrialJourneyResult, error) {
					_ = os.MkdirAll(filepath.Join(wtBPath, ".lucind"), 0o755)
					_ = os.WriteFile(filepath.Join(wtBPath, ".lucind", "result.json"), []byte(laneEnvelopeJSON("stability-change-b", "done")), 0o644)
					cfg.LeaseTTL = 50 * time.Millisecond
					return stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, pgidB, wtBPath)
				},
				DepsFactory: func(runID, primaryRoot string, l *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
					d, _ := newSimulatedTestDeps(t, primaryRoot, newFakeJourneyExecutor())
					d.RunID = runID
					d.Ledger = l
					return d
				},
			})
			if code != tc.wantExitCode {
				t.Fatalf("exit code = %d, want %d; stderr = %s", code, tc.wantExitCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), "15 agy dispatches, three sequential Trials, up to 135 minutes, temporary refs/worktrees/processes, and final cleanup.") {
				t.Errorf("stdout = %q, want verbatim forecast copy", stdout.String())
			}
			if tc.wantDeclined && !strings.Contains(stderr.String(), "declined") {
				t.Errorf("stderr = %q, want 'declined' message", stderr.String())
			}
		})
	}
}

// Part 3: Task 5.1 Simulated 3-Trial Campaign Verification
func TestStabilityRunPreflightAndSimulatedThreeTrialRun(t *testing.T) {
	ctx := context.Background()

	t.Run("ThreePassingTrialsReceiptAndBaseline", func(t *testing.T) {
		dir, headSHA := setupCleanTestRepoForStability(t)

		fakeExec := newFakeJourneyExecutor()
		deps, ledg := newSimulatedTestDeps(t, dir, fakeExec)
		_ = deps
		_ = ledg

		// Pre-populate Change B result envelope in target dir
		wtPathB := reconcile.WorktreePathFor(dir, "stability-change-b")
		_ = os.MkdirAll(filepath.Join(wtPathB, ".lucind"), 0o755)
		_ = os.WriteFile(filepath.Join(wtPathB, ".lucind", "result.json"), []byte(laneEnvelopeJSON("stability-change-b", "done")), 0o644)

		baselineChecksCount := 0
		var stdout, stderr bytes.Buffer

		campCfg := CampaignConfig{
			PrimaryRoot:  dir,
			CandidateSHA: headSHA,
			BuildVersion: headSHA,
			CampaignID:   "camp-sim-3trial-pass",
			ReceiptID:    "rcpt-sim-3trial-pass",
			PGIDB:        99999999,
			LedgerOpener: func(ctx context.Context, primaryRoot string) (*ledger.Ledger, error) {
				return ledger.Open(ctx, primaryRoot)
			},
			ExecuteJourney: func(ctx context.Context, sm *stability.StateMachine, deps lucindrun.Deps, featSvc *feature.Service, cfg stability.JourneyConfig, pgidB int, wtBPath string) (*stability.TrialJourneyResult, error) {
				_ = os.MkdirAll(filepath.Join(wtBPath, ".lucind"), 0o755)
				_ = os.WriteFile(filepath.Join(wtBPath, ".lucind", "result.json"), []byte(laneEnvelopeJSON("stability-change-b", "done")), 0o644)
				cfg.LeaseTTL = 50 * time.Millisecond
				return stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, pgidB, wtBPath)
			},
			DepsFactory: func(runID, primaryRoot string, l *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
				d, _ := newSimulatedTestDeps(t, primaryRoot, fakeExec)
				d.RunID = runID
				d.Ledger = l
				return d
			},
			CheckRunner: func(ctx context.Context, dir string) (bool, string, error) {
				baselineChecksCount++
				return true, "PASS: all baseline checks passed", nil
			},
			Stdout: &stdout,
			Stderr: &stderr,
		}

		code, err := RunCampaign(ctx, campCfg)
		if err != nil || code != 0 {
			t.Fatalf("RunCampaign failed: code = %d, err = %v, stderr = %s", code, err, stderr.String())
		}

		if baselineChecksCount != 1 {
			t.Errorf("baselineChecksCount = %d, want 1 (final baseline check)", baselineChecksCount)
		}

		// Verify store row is store.StatusPassed
		st, err := store.Open(ctx, worktree.DefaultGitRunner, dir)
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		defer st.Close()

		camp, err := st.GetCampaign(ctx, "camp-sim-3trial-pass")
		if err != nil {
			t.Fatalf("GetCampaign: %v", err)
		}
		if camp.Status != store.StatusPassed {
			t.Errorf("camp.Status = %q, want %q", camp.Status, store.StatusPassed)
		}

		// Verify receipt file exists and parses correctly
		receiptPath, err := resolveReceiptPath(ctx, worktree.DefaultGitRunner, dir, "rcpt-sim-3trial-pass")
		if err != nil {
			t.Fatalf("resolveReceiptPath: %v", err)
		}
		receiptData, err := os.ReadFile(receiptPath)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", receiptPath, err)
		}
		var rcpt evidence.Receipt
		if err := json.Unmarshal(receiptData, &rcpt); err != nil {
			t.Fatalf("Unmarshal receipt: %v", err)
		}
		if rcpt.ReceiptID != "rcpt-sim-3trial-pass" {
			t.Errorf("rcpt.ReceiptID = %q, want rcpt-sim-3trial-pass", rcpt.ReceiptID)
		}
		if rcpt.CandidateSHA != headSHA {
			t.Errorf("rcpt.CandidateSHA = %q, want %q", rcpt.CandidateSHA, headSHA)
		}
		if rcpt.Verdict != "passed" {
			t.Errorf("rcpt.Verdict = %q, want passed", rcpt.Verdict)
		}
		if len(rcpt.Trials) != 3 {
			t.Errorf("len(rcpt.Trials) = %d, want 3", len(rcpt.Trials))
		}
		for i, tr := range rcpt.Trials {
			if tr.TrialNumber != i+1 {
				t.Errorf("Trials[%d].TrialNumber = %d, want %d", i, tr.TrialNumber, i+1)
			}
			if tr.Verdict != "passed" {
				t.Errorf("Trials[%d].Verdict = %q, want passed", i, tr.Verdict)
			}
		}
	})

	t.Run("TrialFailureStopsImmediatelyZeroRetry", func(t *testing.T) {
		dir, headSHA := setupCleanTestRepoForStability(t)

		fakeExec := newFakeJourneyExecutor()
		trialsAttempted := 0

		var stdout, stderr bytes.Buffer
		campCfg := CampaignConfig{
			PrimaryRoot:  dir,
			CandidateSHA: headSHA,
			BuildVersion: headSHA,
			CampaignID:   "camp-sim-fail-stop",
			ReceiptID:    "rcpt-sim-fail-stop",
			PGIDB:        99999999,
			LedgerOpener: func(ctx context.Context, primaryRoot string) (*ledger.Ledger, error) {
				return ledger.Open(ctx, primaryRoot)
			},
			ExecuteJourney: func(ctx context.Context, sm *stability.StateMachine, deps lucindrun.Deps, featSvc *feature.Service, cfg stability.JourneyConfig, pgidB int, wtBPath string) (*stability.TrialJourneyResult, error) {
				trialsAttempted++
				if trialsAttempted == 1 {
					// Trial 1 succeeds
					_ = os.MkdirAll(filepath.Join(wtBPath, ".lucind"), 0o755)
					_ = os.WriteFile(filepath.Join(wtBPath, ".lucind", "result.json"), []byte(laneEnvelopeJSON("stability-change-b", "done")), 0o644)
					cfg.LeaseTTL = 50 * time.Millisecond
					return stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, pgidB, wtBPath)
				}
				// Trial 2 fails
				return nil, fmt.Errorf("injected failure on trial 2")
			},
			DepsFactory: func(runID, primaryRoot string, l *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
				d, _ := newSimulatedTestDeps(t, primaryRoot, fakeExec)
				d.RunID = runID
				d.Ledger = l
				return d
			},
			CheckRunner: func(ctx context.Context, dir string) (bool, string, error) {
				return true, "PASS", nil
			},
			Stdout: &stdout,
			Stderr: &stderr,
		}

		code, err := RunCampaign(ctx, campCfg)
		if code == 0 || err == nil {
			t.Fatalf("RunCampaign succeeded, want failure; code = %d, err = %v", code, err)
		}

		if trialsAttempted != 2 {
			t.Errorf("trialsAttempted = %d, want exactly 2 (stopped immediately on failure)", trialsAttempted)
		}

		// Verify store status is store.StatusFailed
		st, err := store.Open(ctx, worktree.DefaultGitRunner, dir)
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		defer st.Close()

		camp, err := st.GetCampaign(ctx, "camp-sim-fail-stop")
		if err != nil {
			t.Fatalf("GetCampaign: %v", err)
		}
		if camp.Status != store.StatusFailed {
			t.Errorf("camp.Status = %q, want %q", camp.Status, store.StatusFailed)
		}

		// Verify no receipt was written
		receiptPath, err := resolveReceiptPath(ctx, worktree.DefaultGitRunner, dir, "rcpt-sim-fail-stop")
		if err != nil {
			t.Fatalf("resolveReceiptPath: %v", err)
		}
		if _, err := os.Stat(receiptPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("receipt file %s exists, want not exist on failed campaign", receiptPath)
		}
	})

	t.Run("CrashAndReclaimMaintainsBookkeeping", func(t *testing.T) {
		dir, headSHA := setupCleanTestRepoForStability(t)

		fakeExec := newFakeJourneyExecutor()
		trialsAttempted := 0

		var stdout, stderr bytes.Buffer
		campCfg := CampaignConfig{
			PrimaryRoot:  dir,
			CandidateSHA: headSHA,
			BuildVersion: headSHA,
			CampaignID:   "camp-sim-crash-reclaim",
			ReceiptID:    "rcpt-sim-crash-reclaim",
			PGIDB:        99999999,
			LedgerOpener: func(ctx context.Context, primaryRoot string) (*ledger.Ledger, error) {
				return ledger.Open(ctx, primaryRoot)
			},
			ExecuteJourney: func(ctx context.Context, sm *stability.StateMachine, deps lucindrun.Deps, featSvc *feature.Service, cfg stability.JourneyConfig, pgidB int, wtBPath string) (*stability.TrialJourneyResult, error) {
				trialsAttempted++
				// Simulate Change B write result envelope in target dir
				_ = os.MkdirAll(filepath.Join(wtBPath, ".lucind"), 0o755)
				_ = os.WriteFile(filepath.Join(wtBPath, ".lucind", "result.json"), []byte(laneEnvelopeJSON("stability-change-b", "done")), 0o644)
				cfg.LeaseTTL = 50 * time.Millisecond
				return stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, pgidB, wtBPath)
			},
			DepsFactory: func(runID, primaryRoot string, l *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
				d, _ := newSimulatedTestDeps(t, primaryRoot, fakeExec)
				d.RunID = runID
				d.Ledger = l
				return d
			},
			CheckRunner: func(ctx context.Context, dir string) (bool, string, error) {
				return true, "PASS", nil
			},
			Stdout: &stdout,
			Stderr: &stderr,
		}

		code, err := RunCampaign(ctx, campCfg)
		if err != nil || code != 0 {
			t.Fatalf("RunCampaign failed: code = %d, err = %v, stderr = %s", code, err, stderr.String())
		}
		if trialsAttempted != 3 {
			t.Errorf("trialsAttempted = %d, want 3", trialsAttempted)
		}
	})

	t.Run("RemediationBranchSurfacesDefect", func(t *testing.T) {
		dir, headSHA := setupCleanTestRepoForStability(t)

		fakeExec := newFakeJourneyExecutor()
		trialsAttempted := 0

		var stdout, stderr bytes.Buffer
		campCfg := CampaignConfig{
			PrimaryRoot:  dir,
			CandidateSHA: headSHA,
			BuildVersion: headSHA,
			CampaignID:   "camp-sim-remediation",
			ReceiptID:    "rcpt-sim-remediation",
			PGIDB:        99999999,
			LedgerOpener: func(ctx context.Context, primaryRoot string) (*ledger.Ledger, error) {
				return ledger.Open(ctx, primaryRoot)
			},
			ExecuteJourney: func(ctx context.Context, sm *stability.StateMachine, deps lucindrun.Deps, featSvc *feature.Service, cfg stability.JourneyConfig, pgidB int, wtBPath string) (*stability.TrialJourneyResult, error) {
				trialsAttempted++
				_ = os.MkdirAll(filepath.Join(wtBPath, ".lucind"), 0o755)
				_ = os.WriteFile(filepath.Join(wtBPath, ".lucind", "result.json"), []byte(laneEnvelopeJSON("stability-change-b", "done")), 0o644)
				cfg.LeaseTTL = 50 * time.Millisecond
				res, err := stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, pgidB, wtBPath)
				if err != nil {
					return nil, err
				}
				if res.Approval == nil || !res.Approval.Approved {
					t.Errorf("res.Approval = %+v, want Approved: true", res.Approval)
				}
				return res, nil
			},
			DepsFactory: func(runID, primaryRoot string, l *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
				d, _ := newSimulatedTestDeps(t, primaryRoot, fakeExec)
				d.RunID = runID
				d.Ledger = l
				origCreate := d.CreateWorktree
				d.CreateWorktree = func(ctx context.Context, root, laneID, parentRef, baseSHA string) (worktree.Worktree, error) {
					wt, err := origCreate(ctx, root, laneID, parentRef, baseSHA)
					if err != nil {
						return wt, err
					}
					if laneID == "stability-change-a" {
						envJSON := laneEnvelopeJSON(laneID, "blocked")
						_ = os.WriteFile(filepath.Join(wt.Path, ".lucind", "result.json"), []byte(envJSON), 0o644)
					}
					return wt, nil
				}
				return d
			},
			CheckRunner: func(ctx context.Context, dir string) (bool, string, error) {
				return true, "PASS", nil
			},
			Stdout: &stdout,
			Stderr: &stderr,
		}

		code, err := RunCampaign(ctx, campCfg)
		if err != nil || code != 0 {
			t.Fatalf("RunCampaign failed: code = %d, err = %v, stderr = %s", code, err, stderr.String())
		}
		if trialsAttempted != 3 {
			t.Errorf("trialsAttempted = %d, want 3", trialsAttempted)
		}
	})

	t.Run("ReceiptContentVerification", func(t *testing.T) {
		dir, headSHA := setupCleanTestRepoForStability(t)

		fakeExec := newFakeJourneyExecutor()
		var stdout, stderr bytes.Buffer
		campCfg := CampaignConfig{
			PrimaryRoot:  dir,
			CandidateSHA: headSHA,
			BuildVersion: headSHA,
			CampaignID:   "camp-sim-receipt-verify",
			ReceiptID:    "rcpt-sim-receipt-verify",
			PGIDB:        99999999,
			LedgerOpener: func(ctx context.Context, primaryRoot string) (*ledger.Ledger, error) {
				return ledger.Open(ctx, primaryRoot)
			},
			DepsFactory: func(runID, primaryRoot string, l *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
				d, _ := newSimulatedTestDeps(t, primaryRoot, fakeExec)
				d.RunID = runID
				d.Ledger = l
				return d
			},
			ExecuteJourney: func(ctx context.Context, sm *stability.StateMachine, deps lucindrun.Deps, featSvc *feature.Service, cfg stability.JourneyConfig, pgidB int, wtBPath string) (*stability.TrialJourneyResult, error) {
				_ = os.MkdirAll(filepath.Join(wtBPath, ".lucind"), 0o755)
				_ = os.WriteFile(filepath.Join(wtBPath, ".lucind", "result.json"), []byte(laneEnvelopeJSON("stability-change-b", "done")), 0o644)
				cfg.LeaseTTL = 50 * time.Millisecond
				return stability.ExecuteTrialJourney(ctx, sm, deps, featSvc, cfg, pgidB, wtBPath)
			},
			CheckRunner: func(ctx context.Context, dir string) (bool, string, error) {
				return true, "PASS: all baseline checks passed", nil
			},
			Stdout: &stdout,
			Stderr: &stderr,
		}

		code, err := RunCampaign(ctx, campCfg)
		if err != nil || code != 0 {
			t.Fatalf("RunCampaign failed: code = %d, err = %v, stderr = %s", code, err, stderr.String())
		}

		receiptPath, err := resolveReceiptPath(ctx, worktree.DefaultGitRunner, dir, "rcpt-sim-receipt-verify")
		if err != nil {
			t.Fatalf("resolveReceiptPath: %v", err)
		}
		receiptData, err := os.ReadFile(receiptPath)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", receiptPath, err)
		}
		var rcpt evidence.Receipt
		if err := json.Unmarshal(receiptData, &rcpt); err != nil {
			t.Fatalf("Unmarshal receipt: %v", err)
		}
		if rcpt.ReceiptID != "rcpt-sim-receipt-verify" {
			t.Errorf("rcpt.ReceiptID = %q, want rcpt-sim-receipt-verify", rcpt.ReceiptID)
		}
		if rcpt.CandidateSHA != headSHA {
			t.Errorf("rcpt.CandidateSHA = %q, want %q", rcpt.CandidateSHA, headSHA)
		}
		if rcpt.BuildVersion != headSHA {
			t.Errorf("rcpt.BuildVersion = %q, want %q", rcpt.BuildVersion, headSHA)
		}
		if rcpt.Verdict != "passed" {
			t.Errorf("rcpt.Verdict = %q, want passed", rcpt.Verdict)
		}
		if rcpt.BaselineCheck != "PASS: all baseline checks passed" {
			t.Errorf("rcpt.BaselineCheck = %q, want 'PASS: all baseline checks passed'", rcpt.BaselineCheck)
		}
		if len(rcpt.Trials) != 3 {
			t.Errorf("len(rcpt.Trials) = %d, want 3", len(rcpt.Trials))
		}
		for i, tr := range rcpt.Trials {
			if tr.TrialNumber != i+1 {
				t.Errorf("Trials[%d].TrialNumber = %d, want %d", i, tr.TrialNumber, i+1)
			}
			if tr.Verdict != "passed" {
				t.Errorf("Trials[%d].Verdict = %q, want passed", i, tr.Verdict)
			}
		}
	})
}

type customInstrumentedExecutor struct {
	inner executor.Executor
	onRun func(req executor.Request)
}

func (c *customInstrumentedExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	if c.onRun != nil {
		c.onRun(req)
	}
	return c.inner.Run(ctx, req)
}

func (c *customInstrumentedExecutor) DefaultModel() string {
	return c.inner.DefaultModel()
}

func (c *customInstrumentedExecutor) KnownModels() []string {
	return c.inner.KnownModels()
}

func TestStabilityRunProductionDefaultWiresLiveJourney(t *testing.T) {
	ctx := context.Background()
	dir, headSHA := setupCleanTestRepoForStability(t)

	var (
		mu              sync.Mutex
		receivedSetpgid []bool
		receivedOnStart []bool
	)

	fakeExec := newFakeJourneyExecutor()
	customExec := &customInstrumentedExecutor{
		inner: fakeExec,
		onRun: func(req executor.Request) {
			mu.Lock()
			defer mu.Unlock()
			receivedSetpgid = append(receivedSetpgid, req.Setpgid)
			receivedOnStart = append(receivedOnStart, req.OnStart != nil)
			if req.OnStart != nil {
				req.OnStart(99999)
			}
		},
	}

	var stdout, stderr bytes.Buffer
	campCfg := CampaignConfig{
		PrimaryRoot:  dir,
		CandidateSHA: headSHA,
		BuildVersion: headSHA,
		CampaignID:   "camp-prod-default-live",
		ReceiptID:    "rcpt-prod-default-live",
		LedgerOpener: func(ctx context.Context, primaryRoot string) (*ledger.Ledger, error) {
			return ledger.Open(ctx, primaryRoot)
		},
		DepsFactory: func(runID, primaryRoot string, l *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps {
			d, _ := newSimulatedTestDeps(t, primaryRoot, customExec)
			d.RunID = runID
			d.Ledger = l
			return d
		},
		CheckRunner: func(ctx context.Context, dir string) (bool, string, error) {
			return true, "PASS: baseline check passed", nil
		},
		Stdout: &stdout,
		Stderr: &stderr,
		// ExecuteJourney is intentionally omitted (nil) to test production default wiring
	}

	code, err := RunCampaign(ctx, campCfg)
	if err != nil || code != 0 {
		t.Fatalf("RunCampaign failed: code = %d, err = %v, stderr = %s", code, err, stderr.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(receivedSetpgid) == 0 {
		t.Fatalf("no executor dispatches were observed")
	}
	for i, spgid := range receivedSetpgid {
		if !spgid {
			t.Errorf("dispatch[%d] req.Setpgid = false, want true (proving ExecuteTrialJourneyLive was reached via production default wiring)", i)
		}
	}
	for i, onStart := range receivedOnStart {
		if !onStart {
			t.Errorf("dispatch[%d] req.OnStart is nil, want non-nil (proving ExecuteTrialJourneyLive was reached via production default wiring)", i)
		}
	}
}
