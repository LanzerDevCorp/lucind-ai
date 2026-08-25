package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/feature"
	"github.com/LanzerDevCorp/lucind-ai/internal/integrate"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
	lucindrun "github.com/LanzerDevCorp/lucind-ai/internal/run"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability"
	"github.com/LanzerDevCorp/lucind-ai/internal/stability/evidence"
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
	return runStabilityRunWithDir(ctx, args, os.Stdin, stdout, stderr, "", nil)
}

// ForecastCopy is the verbatim master-plan-mandated long-work forecast.
const ForecastCopy = "15 agy dispatches, three sequential Trials, up to 135 minutes, temporary refs/worktrees/processes, and final cleanup."

// promptConfirmation displays the forecast and reads single-line interactive confirmation.
// Only exact case-insensitive "y" or "yes" proceeds.
func promptConfirmation(stdin io.Reader, stdout, stderr io.Writer) bool {
	fmt.Fprintln(stdout, ForecastCopy)
	fmt.Fprint(stdout, "proceed with stability campaign? [y/N]: ")
	if stdin == nil {
		return false
	}
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return false
	}
	ans := strings.TrimSpace(scanner.Text())
	return strings.EqualFold(ans, "y") || strings.EqualFold(ans, "yes")
}

// runStabilityRunWithDir allows injecting stdin, directory and lookPath for testing.
func runStabilityRunWithDir(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, dir string, lookPath func(string) (string, error)) int {
	return runStabilityRunWithConfig(ctx, args, stdin, stdout, stderr, PreflightConfig{PrimaryRoot: dir, LookPath: lookPath}, CampaignConfig{})
}

// runStabilityRunWithConfig allows injecting preflight and campaign configs for testing.
func runStabilityRunWithConfig(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, preflightCfg PreflightConfig, campCfg CampaignConfig) int {
	fs := flag.NewFlagSet("stability run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: lucind-ai stability run")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}

	res, err := Preflight(ctx, preflightCfg)
	if err != nil {
		fmt.Fprintf(stderr, "lucind-ai: stability preflight error: %v\n", err)
		return 1
	}
	if !res.OK {
		fmt.Fprintf(stderr, "lucind-ai: stability preflight failed: %s\n", res.Reason)
		return 1
	}

	fmt.Fprintf(stdout, "preflight passed: ready for stability campaign (candidate %s)\n", res.CandidateSHA)

	if !promptConfirmation(stdin, stdout, stderr) {
		fmt.Fprintln(stderr, "stability run declined by operator")
		return 1
	}

	if campCfg.PrimaryRoot == "" {
		campCfg.PrimaryRoot = res.PrimaryRoot
	}
	if campCfg.CandidateSHA == "" {
		campCfg.CandidateSHA = res.CandidateSHA
	}
	if campCfg.BuildVersion == "" {
		campCfg.BuildVersion = version
	}
	if campCfg.Stdout == nil {
		campCfg.Stdout = stdout
	}
	if campCfg.Stderr == nil {
		campCfg.Stderr = stderr
	}

	code, err := RunCampaign(ctx, campCfg)
	if err != nil {
		return code
	}
	return code
}

// CampaignConfig holds dependencies and configuration for executing a 3-Trial Stability Campaign.
type CampaignConfig struct {
	PrimaryRoot     string
	CandidateSHA    string
	BuildVersion    string
	CampaignID      string
	ReceiptID       string
	PGIDB           int
	Now             func() time.Time
	StoreOpener     func(context.Context, worktree.GitRunner, string) (*store.Store, error)
	LedgerOpener    func(context.Context, string) (*ledger.Ledger, error)
	CheckRunner     func(context.Context, string) (bool, string, error)
	DepsFactory     func(runID, primaryRoot string, ledg *ledger.Ledger, timeout, approvalTimeout time.Duration) lucindrun.Deps
	FeatureSvcMaker func(ledg *ledger.Ledger) *feature.Service
	ExecuteJourney  func(context.Context, *stability.StateMachine, lucindrun.Deps, *feature.Service, stability.JourneyConfig, int, string) (*stability.TrialJourneyResult, error)
	ReceiptWriter   func(string, evidence.Receipt) error
	GitRunner       worktree.GitRunner
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
}

func resolveGitCommonDir(ctx context.Context, runner worktree.GitRunner, repoDir string) (string, error) {
	if runner == nil {
		runner = worktree.DefaultGitRunner
	}
	out, err := runner.Run(ctx, repoDir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve git common dir: %w", err)
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return "", errors.New("empty git common dir")
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(repoDir, commonDir)
	}
	return filepath.Clean(commonDir), nil
}

func resolveReceiptPath(ctx context.Context, runner worktree.GitRunner, repoDir, receiptID string) (string, error) {
	commonDir, err := resolveGitCommonDir(ctx, runner, repoDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "lucind-ai", "stability", "v1", "receipts", receiptID+".json"), nil
}

// RunCampaign coordinates the 3-Trial stability campaign loop and writes the canonical receipt upon 3 consecutive passes.
func RunCampaign(ctx context.Context, cfg CampaignConfig) (int, error) {
	if cfg.PrimaryRoot == "" {
		return 1, errors.New("primary root is required")
	}
	if cfg.CandidateSHA == "" {
		return 1, errors.New("candidate SHA is required")
	}
	if cfg.BuildVersion == "" {
		cfg.BuildVersion = version
	}
	if cfg.CampaignID == "" {
		cfg.CampaignID = fmt.Sprintf("camp-%s", uuid.NewString())
	}
	if cfg.ReceiptID == "" {
		cfg.ReceiptID = fmt.Sprintf("rcpt-%s", uuid.NewString())
	}
	if cfg.PGIDB == 0 {
		cfg.PGIDB = 99999999
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.StoreOpener == nil {
		cfg.StoreOpener = store.Open
	}
	if cfg.LedgerOpener == nil {
		cfg.LedgerOpener = ledger.Open
	}
	if cfg.CheckRunner == nil {
		cfg.CheckRunner = integrate.Check
	}
	if cfg.DepsFactory == nil {
		cfg.DepsFactory = productionDeps
	}
	if cfg.FeatureSvcMaker == nil {
		cfg.FeatureSvcMaker = feature.NewService
	}
	if cfg.ExecuteJourney == nil {
		cfg.ExecuteJourney = func(
			ctx context.Context, sm *stability.StateMachine, deps lucindrun.Deps,
			featSvc *feature.Service, journeyCfg stability.JourneyConfig, _ int, _ string,
		) (*stability.TrialJourneyResult, error) {
			return stability.ExecuteTrialJourneyLive(ctx, sm, deps, featSvc, journeyCfg)
		}
	}
	if cfg.ReceiptWriter == nil {
		cfg.ReceiptWriter = evidence.WriteReceipt
	}
	if cfg.GitRunner == nil {
		cfg.GitRunner = worktree.DefaultGitRunner
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}

	st, err := cfg.StoreOpener(ctx, cfg.GitRunner, cfg.PrimaryRoot)
	if err != nil {
		fmt.Fprintf(cfg.Stderr, "stability campaign error: open store: %v\n", err)
		return 1, fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	if _, err := st.CreateCampaign(ctx, cfg.CampaignID, cfg.CandidateSHA); err != nil {
		fmt.Fprintf(cfg.Stderr, "stability campaign error: create campaign row: %v\n", err)
		return 1, fmt.Errorf("create campaign row: %w", err)
	}

	sm := stability.NewStateMachine()
	if err := sm.Start(); err != nil {
		_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
		fmt.Fprintf(cfg.Stderr, "stability campaign error: start state machine: %v\n", err)
		return 1, fmt.Errorf("start state machine: %w", err)
	}

	ledg, err := cfg.LedgerOpener(ctx, cfg.PrimaryRoot)
	if err != nil {
		_ = sm.TransitionCampaign(stability.CampaignFailed)
		_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
		fmt.Fprintf(cfg.Stderr, "stability campaign error: open ledger: %v\n", err)
		return 1, fmt.Errorf("open ledger: %w", err)
	}
	defer ledg.Close()

	featSvc := cfg.FeatureSvcMaker(ledg)

	var trialRecords []evidence.TrialRecord
	for trialNum := 1; trialNum <= stability.TargetConsecutivePasses; trialNum++ {
		tNum, err := sm.StartNextTrial()
		if err != nil {
			_ = sm.TransitionCampaign(stability.CampaignFailed)
			_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
			fmt.Fprintf(cfg.Stderr, "stability campaign error: start trial %d: %v\n", trialNum, err)
			return 1, fmt.Errorf("start trial %d: %w", trialNum, err)
		}

		journeyCfg := stability.DefaultJourneyConfig(tNum, cfg.CandidateSHA)
		runID := fmt.Sprintf("stability-%s-trial-%d", cfg.CampaignID, tNum)
		deps := cfg.DepsFactory(runID, cfg.PrimaryRoot, ledg, stability.DispatchTimeout, 0)

		_, pB := stability.BuildJourneyPackets(journeyCfg)
		wtPathB := reconcile.WorktreePathFor(cfg.PrimaryRoot, pB.ID)

		res, err := cfg.ExecuteJourney(ctx, sm, deps, featSvc, journeyCfg, cfg.PGIDB, wtPathB)
		if err != nil {
			if sm.HasActiveTrial() {
				_ = sm.RecordTrialOutcome(stability.TrialFailed)
			}
			_ = sm.TransitionCampaign(stability.CampaignFailed)
			_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
			fmt.Fprintf(cfg.Stderr, "stability trial %d failed: %v\n", tNum, err)
			return 1, fmt.Errorf("trial %d failed: %w", tNum, err)
		}
		_ = res

		if err := sm.AdvanceTrial(stability.TrialCleanedUp); err != nil {
			_ = sm.RecordTrialOutcome(stability.TrialFailed)
			_ = sm.TransitionCampaign(stability.CampaignFailed)
			_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
			fmt.Fprintf(cfg.Stderr, "stability trial %d advance cleanup error: %v\n", tNum, err)
			return 1, fmt.Errorf("advance trial %d cleanup: %w", tNum, err)
		}

		if err := sm.RecordTrialOutcome(stability.TrialPassed); err != nil {
			_ = sm.TransitionCampaign(stability.CampaignFailed)
			_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
			fmt.Fprintf(cfg.Stderr, "stability trial %d record outcome error: %v\n", tNum, err)
			return 1, fmt.Errorf("record trial %d outcome: %w", tNum, err)
		}

		trialRecords = append(trialRecords, evidence.TrialRecord{
			TrialNumber: tNum,
			Verdict:     string(stability.TrialPassed),
		})

		// Zero-retry stop condition: halt immediately if not running
		if sm.State() != stability.CampaignRunning && !sm.IsEligibleForTerminalVerification() {
			break
		}
	}

	if !sm.IsEligibleForTerminalVerification() {
		finalStatus, _ := sm.State().StoreStatus()
		_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, finalStatus)
		fmt.Fprintf(cfg.Stderr, "stability campaign failed: eligible for verification = false (state %s)\n", sm.State())
		return 1, fmt.Errorf("campaign not eligible for verification (state %s)", sm.State())
	}

	// Final post-cleanup baseline check
	passed, output, err := cfg.CheckRunner(ctx, cfg.PrimaryRoot)
	if err != nil || !passed {
		_ = sm.TransitionCampaign(stability.CampaignFailed)
		_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
		fmt.Fprintf(cfg.Stderr, "stability campaign final baseline check failed: %s (%v)\n", strings.TrimSpace(output), err)
		return 1, fmt.Errorf("final baseline check failed: %w", err)
	}

	receiptPath, err := resolveReceiptPath(ctx, cfg.GitRunner, cfg.PrimaryRoot, cfg.ReceiptID)
	if err != nil {
		_ = sm.TransitionCampaign(stability.CampaignFailed)
		_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
		fmt.Fprintf(cfg.Stderr, "stability campaign resolve receipt path error: %v\n", err)
		return 1, fmt.Errorf("resolve receipt path: %w", err)
	}

	rcpt := evidence.Receipt{
		ReceiptID:     cfg.ReceiptID,
		CandidateSHA:  cfg.CandidateSHA,
		BuildVersion:  cfg.BuildVersion,
		Verdict:       string(store.StatusPassed),
		CreatedAt:     cfg.Now().UTC().Format(time.RFC3339),
		BaselineCheck: strings.TrimSpace(output),
		Trials:        trialRecords,
	}

	if err := cfg.ReceiptWriter(receiptPath, rcpt); err != nil {
		_ = sm.TransitionCampaign(stability.CampaignFailed)
		_ = st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusFailed)
		fmt.Fprintf(cfg.Stderr, "stability campaign write receipt error: %v\n", err)
		return 1, fmt.Errorf("write stability receipt: %w", err)
	}

	if err := st.UpdateCampaignStatus(ctx, cfg.CampaignID, store.StatusPassed); err != nil {
		fmt.Fprintf(cfg.Stderr, "stability campaign update status error: %v\n", err)
		return 1, fmt.Errorf("update campaign status to passed: %w", err)
	}

	fmt.Fprintf(cfg.Stdout, "stability campaign %s passed (3/3 trials passed)\n", cfg.CampaignID)
	fmt.Fprintf(cfg.Stdout, "receipt written: %s\n", receiptPath)
	return 0, nil
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
