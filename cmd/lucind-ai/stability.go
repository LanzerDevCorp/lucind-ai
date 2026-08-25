package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/reconcile"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/store"
	"github.com/LanzerDevCorp/lucind-ai/internal/worktree"
)

// PreflightConfig configures the preflight admission validation pass.
type PreflightConfig struct {
	GOOS        string
	PrimaryRoot string
	Version     string
	LookPath    func(string) (string, error)
	CheckRunner func(context.Context, string) (bool, string, error)
	StoreOpener func(context.Context, worktree.GitRunner, string) (*store.Store, error)
	GitRunner   worktree.GitRunner
	KnownModels []string
}

// PreflightResult holds the structured outcome of a preflight pass.
type PreflightResult struct {
	OK           bool   `json:"ok"`
	Reason       string `json:"reason,omitempty"`
	PrimaryRoot  string `json:"primary_root,omitempty"`
	CandidateSHA string `json:"candidate_sha,omitempty"`
}

// StabilityStatusOutput represents the JSON payload emitted by stability status --json.
type StabilityStatusOutput struct {
	Active       bool            `json:"active"`
	CampaignID   string          `json:"campaign_id,omitempty"`
	CandidateSHA string          `json:"candidate_sha,omitempty"`
	Status       string          `json:"status,omitempty"`
	CreatedAt    *time.Time      `json:"created_at,omitempty"`
	UpdatedAt    *time.Time      `json:"updated_at,omitempty"`
	ClosedAt     *time.Time      `json:"closed_at,omitempty"`
	Campaign     *store.Campaign `json:"campaign,omitempty"`
}

// CheckOS validates that the operating system is Linux.
func CheckOS(goos string) error {
	if goos != "linux" {
		return fmt.Errorf("stability preflight: linux OS required (got %s)", goos)
	}
	return nil
}

// CheckGitRepo verifies that dir is a primary git repository and not a linked worktree.
func CheckGitRepo(ctx context.Context, runner worktree.GitRunner, dir string) (string, error) {
	if runner == nil {
		runner = worktree.DefaultGitRunner
	}
	out, err := runner.Run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("stability preflight: %q is not a git repository: %w", dir, err)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("stability preflight: %q is not a git repository", dir)
	}

	if worktree.IsLinkedWorktree(root) {
		return "", fmt.Errorf("stability preflight: refusing to run from inside a linked worktree (%s); run from primary repository instead", root)
	}
	return root, nil
}

// CheckCleanWorkingTree verifies that git status --porcelain in dir produces no output.
func CheckCleanWorkingTree(ctx context.Context, runner worktree.GitRunner, dir string) error {
	if runner == nil {
		runner = worktree.DefaultGitRunner
	}
	out, err := runner.Run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("stability preflight: git status failed: %w", err)
	}
	if len(strings.TrimSpace(string(out))) > 0 {
		return fmt.Errorf("stability preflight: working tree at %s is dirty (uncommitted modifications present)", dir)
	}
	return nil
}

// matchVersionAndHEAD checks if the binary compile-time version matches the git candidate HEAD commit.
func matchVersionAndHEAD(versionStr, headSHA string) bool {
	v := strings.TrimSpace(versionStr)
	head := strings.TrimSpace(headSHA)
	if v == "" || head == "" || v == "dev" {
		return false
	}
	if v == head {
		return true
	}
	// Prefix match if v is an abbreviated commit SHA (at least 7 hex characters)
	if len(v) >= 7 && len(v) <= len(head) && strings.HasPrefix(head, v) {
		return true
	}
	// Tag describe format: e.g. "v1.2.3-4-g8668e9f"
	if idx := strings.LastIndex(v, "-g"); idx != -1 {
		shaPart := v[idx+2:]
		shaPart = strings.TrimSuffix(shaPart, "-dirty")
		if len(shaPart) >= 7 && strings.HasPrefix(head, shaPart) {
			return true
		}
	}
	// Abbreviated dirty SHA: e.g. "8668e9f-dirty"
	if strings.HasSuffix(v, "-dirty") {
		shaPart := strings.TrimSuffix(v, "-dirty")
		if len(shaPart) >= 7 && strings.HasPrefix(head, shaPart) {
			return true
		}
	}
	return false
}

// CheckCandidateBuild compares compile-time version against candidate HEAD SHA at dir.
func CheckCandidateBuild(ctx context.Context, runner worktree.GitRunner, dir, versionStr string) (string, error) {
	if runner == nil {
		runner = worktree.DefaultGitRunner
	}
	out, err := runner.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("stability preflight: resolve HEAD commit SHA: %w", err)
	}
	headSHA := strings.TrimSpace(string(out))
	if !matchVersionAndHEAD(versionStr, headSHA) {
		return headSHA, fmt.Errorf("stability preflight: running version %q does not match candidate HEAD %s; run 'make install'", versionStr, headSHA)
	}
	return headSHA, nil
}

// CheckBaseline executes baseline checks script via integrate.Check.
func CheckBaseline(ctx context.Context, dir string, checkFn func(context.Context, string) (bool, string, error)) error {
	if checkFn == nil {
		checkFn = integrate.Check
	}
	passed, output, err := checkFn(ctx, dir)
	if err != nil {
		return fmt.Errorf("stability preflight: baseline check error: %w", err)
	}
	if !passed {
		return fmt.Errorf("stability preflight: baseline check failed: %s", strings.TrimSpace(output))
	}
	return nil
}

// CheckZeroActiveCampaigns verifies that no active stability campaign exists in the SQLite store.
func CheckZeroActiveCampaigns(ctx context.Context, st *store.Store) error {
	if st == nil {
		return errors.New("stability preflight: nil stability store")
	}
	camp, err := st.GetActiveCampaign(ctx)
	if err == nil {
		return fmt.Errorf("stability preflight: active campaign %q already in progress (candidate %s)", camp.ID, camp.CandidateSHA)
	}
	if errors.Is(err, store.ErrCampaignNotFound) {
		return nil
	}
	return fmt.Errorf("stability preflight: check active campaigns: %w", err)
}

// CheckAgyAvailability verifies agy binary presence on PATH and pinned model support.
func CheckAgyAvailability(lookPath func(string) (string, error), knownModels []string) error {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if len(knownModels) == 0 {
		knownModels = executor.Agy{}.KnownModels()
	}

	modelPinned := false
	for _, m := range knownModels {
		if m == "gemini-3.7-flash-high" {
			modelPinned = true
			break
		}
	}
	if !modelPinned {
		return fmt.Errorf("stability preflight: pinned model %q not supported by agy executor", "gemini-3.7-flash-high")
	}

	if _, err := lookPath("agy"); err != nil {
		return fmt.Errorf("stability preflight: 'agy' executable not found on PATH: %w", err)
	}
	return nil
}

// Preflight runs the pure, read-only admission validation pass.
func Preflight(ctx context.Context, cfg PreflightConfig) (PreflightResult, error) {
	goos := cfg.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	// 1. Linux OS gate (must fire before other checks)
	if err := CheckOS(goos); err != nil {
		return PreflightResult{OK: false, Reason: err.Error()}, nil
	}

	// 2. Git repository selection & linked worktree rejection
	primaryRoot := cfg.PrimaryRoot
	if primaryRoot == "" {
		resolved, err := resolvePrimaryRoot(ctx)
		if err != nil {
			return PreflightResult{OK: false, Reason: fmt.Sprintf("stability preflight: resolve primary root: %v", err)}, nil
		}
		primaryRoot = resolved
	}
	root, err := CheckGitRepo(ctx, cfg.GitRunner, primaryRoot)
	if err != nil {
		return PreflightResult{OK: false, Reason: err.Error()}, nil
	}

	// 3. Clean working tree check
	if err := CheckCleanWorkingTree(ctx, cfg.GitRunner, root); err != nil {
		return PreflightResult{OK: false, Reason: err.Error(), PrimaryRoot: root}, nil
	}

	// 4. Candidate HEAD build match check
	ver := cfg.Version
	if ver == "" {
		ver = version
	}
	headSHA, err := CheckCandidateBuild(ctx, cfg.GitRunner, root, ver)
	if err != nil {
		return PreflightResult{OK: false, Reason: err.Error(), PrimaryRoot: root, CandidateSHA: headSHA}, nil
	}

	// 5. Baseline check
	checkRunner := cfg.CheckRunner
	if checkRunner == nil {
		checkRunner = integrate.Check
	}
	if err := CheckBaseline(ctx, root, checkRunner); err != nil {
		return PreflightResult{OK: false, Reason: err.Error(), PrimaryRoot: root, CandidateSHA: headSHA}, nil
	}

	// 6. Zero active campaigns check
	storeOpener := cfg.StoreOpener
	if storeOpener == nil {
		storeOpener = store.Open
	}
	st, err := storeOpener(ctx, cfg.GitRunner, root)
	if err != nil {
		return PreflightResult{OK: false, Reason: fmt.Sprintf("stability preflight: open store: %v", err), PrimaryRoot: root, CandidateSHA: headSHA}, nil
	}
	defer st.Close()

	if err := CheckZeroActiveCampaigns(ctx, st); err != nil {
		return PreflightResult{OK: false, Reason: err.Error(), PrimaryRoot: root, CandidateSHA: headSHA}, nil
	}

	// 7. agy & model availability check
	if err := CheckAgyAvailability(cfg.LookPath, cfg.KnownModels); err != nil {
		return PreflightResult{OK: false, Reason: err.Error(), PrimaryRoot: root, CandidateSHA: headSHA}, nil
	}

	return PreflightResult{
		OK:           true,
		PrimaryRoot:  root,
		CandidateSHA: headSHA,
	}, nil
}

// stabilityDispatch dispatches stability subcommands (run, status, resume, abort).
func stabilityDispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "lucind-ai: stability subcommand requires an action (run, status, resume, abort)")
		fmt.Fprintln(stderr, usage)
		return 1
	}

	switch args[0] {
	case "run":
		return runStabilityRun(ctx, args[1:], stdout, stderr)
	case "status":
		return runStabilityStatus(ctx, args[1:], stdout, stderr)
	case "resume":
		return runStabilityResume(ctx, args[1:], stdout, stderr)
	case "abort":
		return runStabilityAbort(ctx, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "lucind-ai: unknown stability subcommand %q\n%s\n", args[0], usage)
		return 1
	}
}

// runStabilityRun implements "lucind-ai stability run": validates preflight admission
// and stops short of starting Campaign orchestration (Wave 4b).
func runStabilityRun(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runStabilityRunWithDir(ctx, args, stdout, stderr, "", nil)
}

// runStabilityRunWithDir allows injecting directory and lookPath for testing.
func runStabilityRunWithDir(ctx context.Context, args []string, stdout, stderr io.Writer, dir string, lookPath func(string) (string, error)) int {
	fs := flag.NewFlagSet("stability run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai stability run")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg := PreflightConfig{
		PrimaryRoot: dir,
		LookPath:    lookPath,
	}

	res, err := Preflight(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: stability preflight error: %v\n", err)
		return 1
	}
	if !res.OK {
		fmt.Fprintf(stderr, "lucind-ai: stability preflight failed: %s\n", res.Reason)
		return 1
	}

	fmt.Fprintf(stdout, "preflight passed: ready for stability campaign (candidate %s)\n", res.CandidateSHA)
	fmt.Fprintln(stdout, "note: stability run dispatch orchestration is not yet implemented (Wave 4b stub)")
	return 0
}

// runStabilityStatus implements "lucind-ai stability status": queries active stability campaign state.
func runStabilityStatus(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runStabilityStatusWithDir(ctx, args, stdout, stderr, "")
}

// runStabilityStatusWithDir allows injecting directory for testing.
func runStabilityStatusWithDir(ctx context.Context, args []string, stdout, stderr io.Writer, dir string) int {
	fs := flag.NewFlagSet("stability status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai stability status [--json]")
		fs.PrintDefaults()
	}

	jsonFlag := fs.Bool("json", false, "emit status as JSON object")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	primaryRoot := dir
	if primaryRoot == "" {
		root, err := resolvePrimaryRoot(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
		primaryRoot = root
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	st, err := store.Open(ctx, worktree.DefaultGitRunner, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open stability store: %v\n", err)
		return 1
	}
	defer st.Close()

	camp, err := st.GetActiveCampaign(ctx)
	if err != nil && !errors.Is(err, store.ErrCampaignNotFound) {
		fmt.Fprintf(stderr, "lucind-ai: get active campaign: %v\n", err)
		return 1
	}

	if errors.Is(err, store.ErrCampaignNotFound) {
		if *jsonFlag {
			out := StabilityStatusOutput{
				Active: false,
				Status: "none",
			}
			data, _ := json.MarshalIndent(out, "", "  ")
			fmt.Fprintln(stdout, string(data))
			return 0
		}
		fmt.Fprintln(stdout, "no active stability campaign")
		return 0
	}

	if *jsonFlag {
		out := StabilityStatusOutput{
			Active:       true,
			CampaignID:   camp.ID,
			CandidateSHA: camp.CandidateSHA,
			Status:       string(camp.Status),
			CreatedAt:    &camp.CreatedAt,
			UpdatedAt:    &camp.UpdatedAt,
			ClosedAt:     camp.ClosedAt,
			Campaign:     &camp,
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: marshal json: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(data))
		return 0
	}

	fmt.Fprintf(stdout, "campaign:      %s\n", camp.ID)
	fmt.Fprintf(stdout, "candidate_sha: %s\n", camp.CandidateSHA)
	fmt.Fprintf(stdout, "status:        %s\n", camp.Status)
	fmt.Fprintf(stdout, "created_at:    %s\n", camp.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(stdout, "updated_at:    %s\n", camp.UpdatedAt.Format(time.RFC3339))
	if camp.ClosedAt != nil {
		fmt.Fprintf(stdout, "closed_at:     %s\n", camp.ClosedAt.Format(time.RFC3339))
	}
	return 0
}

// runStabilityResume implements "lucind-ai stability resume": delegates to reconcile.Inspect.
func runStabilityResume(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runStabilityResumeWithDir(ctx, args, stdout, stderr, "")
}

// runStabilityResumeWithDir allows injecting directory for testing.
func runStabilityResumeWithDir(ctx context.Context, args []string, stdout, stderr io.Writer, dir string) int {
	fs := flag.NewFlagSet("stability resume", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai stability resume")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	primaryRoot := dir
	if primaryRoot == "" {
		root, err := resolvePrimaryRoot(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
		primaryRoot = root
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	st, err := store.Open(ctx, worktree.DefaultGitRunner, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open stability store: %v\n", err)
		return 1
	}
	defer st.Close()

	report, err := reconcile.Inspect(ctx, reconcile.InspectParams{
		Store:       st,
		PrimaryRoot: primaryRoot,
	})
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: stability resume inspect: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "campaign: %s\n", report.Campaign.ID)
	fmt.Fprintf(stdout, "decision: %s\n", report.Decision)
	fmt.Fprintf(stdout, "reason:   %s\n", report.Reason)
	if len(report.Ambiguities) > 0 {
		fmt.Fprintln(stdout, "ambiguities:")
		for _, amb := range report.Ambiguities {
			fmt.Fprintf(stdout, "  - %s\n", amb)
		}
	}

	if report.Decision != reconcile.DecisionSafe {
		return 1
	}
	return 0
}

// runStabilityAbort implements "lucind-ai stability abort": delegates to reconcile.Abort.
func runStabilityAbort(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return runStabilityAbortWithDir(ctx, args, stdout, stderr, "")
}

// runStabilityAbortWithDir allows injecting directory for testing.
func runStabilityAbortWithDir(ctx context.Context, args []string, stdout, stderr io.Writer, dir string) int {
	fs := flag.NewFlagSet("stability abort", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai stability abort")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	primaryRoot := dir
	if primaryRoot == "" {
		root, err := resolvePrimaryRoot(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "lucind-ai: %v\n", err)
			return 1
		}
		primaryRoot = root
	}

	if worktree.IsLinkedWorktree(primaryRoot) {
		fmt.Fprintf(stderr, "lucind-ai: refusing to run from inside a linked worktree (%s); run from the primary repository instead\n", primaryRoot)
		return 1
	}

	st, err := store.Open(ctx, worktree.DefaultGitRunner, primaryRoot)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: open stability store: %v\n", err)
		return 1
	}
	defer st.Close()

	res, err := reconcile.Abort(ctx, reconcile.AbortParams{
		Store:       st,
		PrimaryRoot: primaryRoot,
	})
	if res != nil {
		fmt.Fprintf(stdout, "campaign: %s\n", res.CampaignID)
		fmt.Fprintf(stdout, "status:   %s\n", res.FinalStatus)
		if len(res.CleanedWorktrees) > 0 {
			fmt.Fprintf(stdout, "cleaned_worktrees: %s\n", strings.Join(res.CleanedWorktrees, ", "))
		}
		if len(res.CleanedBranches) > 0 {
			fmt.Fprintf(stdout, "cleaned_branches:  %s\n", strings.Join(res.CleanedBranches, ", "))
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: stability abort: %v\n", err)
		return 1
	}

	if res != nil && res.FinalStatus == store.StatusBlockedCleanup {
		return 1
	}
	return 0
}
