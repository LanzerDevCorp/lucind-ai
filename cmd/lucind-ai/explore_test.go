package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
)

type exploreTestExecutor struct {
	mu           sync.Mutex
	calls        []executor.Request
	lensFailures map[string]string // lens suffix ("structural", etc.) -> failure reason
	synthSummary string
}

func (e *exploreTestExecutor) Run(ctx context.Context, req executor.Request) (executor.Outcome, error) {
	e.mu.Lock()
	e.calls = append(e.calls, req)
	e.mu.Unlock()

	laneID := filepath.Base(req.WorktreePath)

	for suffix, failReason := range e.lensFailures {
		if strings.HasSuffix(laneID, "-"+suffix) {
			return executor.Outcome{ExitCode: 1, Stderr: failReason}, nil
		}
	}

	summary := "evidence from " + laneID
	if strings.HasSuffix(laneID, "-synthesis") {
		if e.synthSummary != "" {
			summary = e.synthSummary
		} else {
			summary = "synthesized handoff for " + laneID
		}
	}

	skills := req.RequiredSkills
	if skills == nil {
		skills = []string{}
	}
	var skillsParts []string
	for _, s := range skills {
		skillsParts = append(skillsParts, fmt.Sprintf("%q", s))
	}
	skillsJSON := "[" + strings.Join(skillsParts, ", ") + "]"

	envelope := fmt.Sprintf(`{"packet_id": %q, "status": "done", "summary": %q, "hard_stops": [], "skills_loaded": %s}`, laneID, summary, skillsJSON)
	envelopePath := filepath.Join(req.WorktreePath, ".lucind", "result.json")
	_ = os.MkdirAll(filepath.Dir(envelopePath), 0o755)
	_ = os.WriteFile(envelopePath, []byte(envelope), 0o644)
	return executor.Outcome{ExitCode: 0}, nil
}

func (e *exploreTestExecutor) DefaultModel() string {
	return "test-model"
}

func (e *exploreTestExecutor) KnownModels() []string {
	return []string{"test-model", "gemini-3.8-flash-medium"}
}

func setupExploreTestEnvironment(t *testing.T, exec executor.Executor) (repoDir string, stateDir string) {
	t.Helper()
	repoDir = initRepo(t)
	head := currentHead(t, repoDir)

	stateDir = t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)

	origQuota := ensureAgyQuota
	t.Cleanup(func() { ensureAgyQuota = origQuota })
	ensureAgyQuota = func(context.Context, float64) error { return nil }

	origResolver := resolveAdmissionRefSHA
	t.Cleanup(func() { resolveAdmissionRefSHA = origResolver })
	resolveAdmissionRefSHA = func(context.Context, string, string) (string, error) {
		return head, nil
	}

	origExecutors := supportedExecutors
	t.Cleanup(func() { supportedExecutors = origExecutors })
	supportedExecutors = map[string]func() executor.Executor{
		"agy": func() executor.Executor { return exec },
	}

	origFactory := depsFactory
	t.Cleanup(func() { depsFactory = origFactory })
	depsFactory = func(runID, primaryRoot string, ledg *ledger.Ledger, timeout time.Duration) lucindrun.Deps {
		deps := origFactory(runID, primaryRoot, ledg, timeout)
		deps.ResolveCandidateIdentity = stubCandidateIdentity
		deps.CreateWorktree = origFactory(runID, primaryRoot, ledg, timeout).CreateWorktree
		deps.HasUniqueLaneCommits = func(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
			return false, nil
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

	return repoDir, stateDir
}

func TestExploreAllThreeLensesSucceed(t *testing.T) {
	fakeExec := &exploreTestExecutor{
		synthSummary: "final unified synthesis handoff with path:line citations",
	}
	repoDir, stateDir := setupExploreTestEnvironment(t, fakeExec)

	var stdout, stderr bytes.Buffer
	origWD, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	code := run(context.Background(), []string{
		"explore",
		"--objective", "Map all dependencies in internal/explorefan",
		"--scope", "internal/explorefan",
		"--id", "explore-test1",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("explore failed with exit code %d\nStdout:\n%s\nStderr:\n%s", code, stdout.String(), stderr.String())
	}

	// Verify 4 total executions: 3 lenses + 1 synthesis
	fakeExec.mu.Lock()
	callCount := len(fakeExec.calls)
	fakeExec.mu.Unlock()
	if callCount != 4 {
		t.Fatalf("expected 4 lane executions (3 lens + 1 synthesis), got %d", callCount)
	}

	// Verify synthesis packet prompt contains all three fake summaries
	fakeExec.mu.Lock()
	synthCall := fakeExec.calls[3]
	fakeExec.mu.Unlock()
	if !strings.Contains(synthCall.Prompt, "evidence from explore-test1-structural") {
		t.Errorf("synthesis prompt missing structural evidence")
	}
	if !strings.Contains(synthCall.Prompt, "evidence from explore-test1-textual") {
		t.Errorf("synthesis prompt missing textual evidence")
	}
	if !strings.Contains(synthCall.Prompt, "evidence from explore-test1-historical") {
		t.Errorf("synthesis prompt missing historical evidence")
	}

	// Verify stdout contains the first-line banner and synthesis summary
	expectedBanner := "explore explore-test1: handoff from synthesis lane explore-test1-synthesis"
	if !strings.Contains(stdout.String(), expectedBanner) {
		t.Errorf("stdout missing expected banner %q\nGot:\n%s", expectedBanner, stdout.String())
	}
	if !strings.Contains(stdout.String(), "final unified synthesis handoff with path:line citations") {
		t.Errorf("stdout missing synthesis summary")
	}

	// Verify handoff.md was written
	runDir := filepath.Join(stateDir, "lucind-ai", "explore", "explore-test1")
	handoffFile := filepath.Join(runDir, "handoff.md")
	content, err := os.ReadFile(handoffFile)
	if err != nil {
		t.Fatalf("read handoff.md: %v", err)
	}
	if !strings.Contains(string(content), "final unified synthesis handoff with path:line citations") {
		t.Errorf("handoff.md content mismatch: %s", string(content))
	}

	// Verify file modes: dir 0700, files 0600
	info, err := os.Stat(runDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("runDir mode = %o, want 0700", info.Mode().Perm())
	}
	hInfo, err := os.Stat(handoffFile)
	if err != nil {
		t.Fatal(err)
	}
	if hInfo.Mode().Perm() != 0o600 {
		t.Errorf("handoff.md mode = %o, want 0600", hInfo.Mode().Perm())
	}
}

func TestExploreOneLensFailsStillRunsSynthesis(t *testing.T) {
	fakeExec := &exploreTestExecutor{
		lensFailures: map[string]string{
			"historical": "git log timed out after 30s",
		},
		synthSummary: "synthesis handoff acknowledging historical failure",
	}
	repoDir, _ := setupExploreTestEnvironment(t, fakeExec)

	var stdout, stderr bytes.Buffer
	origWD, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	code := run(context.Background(), []string{
		"explore",
		"--objective", "Map architecture",
		"--id", "explore-onefail",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("explore failed with exit code %d\nStdout:\n%s\nStderr:\n%s", code, stdout.String(), stderr.String())
	}

	fakeExec.mu.Lock()
	callCount := len(fakeExec.calls)
	synthCall := fakeExec.calls[3]
	fakeExec.mu.Unlock()

	if callCount != 4 {
		t.Fatalf("expected 4 lane executions, got %d", callCount)
	}

	// Synthesis prompt should mention the failure
	if !strings.Contains(synthCall.Prompt, "## Lens: historical (failed)") {
		t.Errorf("synthesis prompt missing failed historical section")
	}
	if !strings.Contains(synthCall.Prompt, "git log timed out after 30s") {
		t.Errorf("synthesis prompt missing failure diagnosis")
	}
}

func TestExploreAllLensesFailExitsOneWithoutSynthesis(t *testing.T) {
	fakeExec := &exploreTestExecutor{
		lensFailures: map[string]string{
			"structural": "codegraph failed",
			"textual":    "ripgrep failed",
			"historical": "git log failed",
		},
	}
	repoDir, _ := setupExploreTestEnvironment(t, fakeExec)

	var stdout, stderr bytes.Buffer
	origWD, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	code := run(context.Background(), []string{
		"explore",
		"--objective", "Map architecture",
		"--id", "explore-allfail",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("explore exit code = %d, want 1", code)
	}

	fakeExec.mu.Lock()
	callCount := len(fakeExec.calls)
	fakeExec.mu.Unlock()

	// Exactly 3 lenses ran, synthesis was NEVER invoked
	if callCount != 3 {
		t.Fatalf("expected 3 lens executions and no synthesis, got %d calls", callCount)
	}

	if !strings.Contains(stderr.String(), "all lens lanes failed") {
		t.Errorf("stderr missing all lens lanes failed notice: %s", stderr.String())
	}
}

func TestExploreFlagsValidationRejectsBeforeLanesStart(t *testing.T) {
	fakeExec := &exploreTestExecutor{}
	repoDir, _ := setupExploreTestEnvironment(t, fakeExec)

	origWD, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "missing objective",
			args: []string{"explore", "--id", "explore-valid"},
		},
		{
			name: "invalid id with uppercase",
			args: []string{"explore", "--objective", "valid", "--id", "Explore-Bad"},
		},
		{
			name: "invalid scope with leading slash",
			args: []string{"explore", "--objective", "valid", "--scope", "/etc/passwd"},
		},
		{
			name: "invalid scope with dotdot",
			args: []string{"explore", "--objective", "valid", "--scope", "../outside"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeExec.mu.Lock()
			fakeExec.calls = nil
			fakeExec.mu.Unlock()

			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tt.args, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}

			fakeExec.mu.Lock()
			callCount := len(fakeExec.calls)
			fakeExec.mu.Unlock()
			if callCount != 0 {
				t.Fatalf("lanes were started (%d calls), want 0 for validation failure", callCount)
			}
		})
	}
}
