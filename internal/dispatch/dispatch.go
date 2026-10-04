package dispatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/executor"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

// Options holds parameters for dispatching a lane.
type Options struct {
	Cwd      string
	LaneID   string
	Allow    []string
	Checks   []string
	Model    string
	Brief    string
	MinQuota float64
	Detach   bool
	Timeout  time.Duration
}

// Output is the structured outcome of dispatch or wait.
type Output struct {
	Lane       string `json:"lane"`
	PaneID     string `json:"pane_id"`
	Cwd        string `json:"cwd"`
	Status     string `json:"status"`
	ResultPath string `json:"result_path"`
}

type quotaEnsurer func(ctx context.Context, minQuota float64) error

var defaultQuotaEnsurer quotaEnsurer = func(ctx context.Context, minQuota float64) error {
	return executor.AgyQuota{}.Ensure(ctx, minQuota)
}

var ensureAgyQuota = defaultQuotaEnsurer

// SetEnsureAgyQuotaForTesting overrides the quota check function for testing.
func SetEnsureAgyQuotaForTesting(fn func(ctx context.Context, minQuota float64) error) {
	ensureAgyQuota = fn
}

// ResetEnsureAgyQuotaForTesting restores the default quota check function.
func ResetEnsureAgyQuotaForTesting() {
	ensureAgyQuota = defaultQuotaEnsurer
}

func constructBrief(userBrief, laneID string, allow []string, absResultPath string, checks []string) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimRight(userBrief, "\n"))
	sb.WriteString("\n\n---\n## Lane Contract\n")
	sb.WriteString(fmt.Sprintf("- Lane ID: %s\n", laneID))
	sb.WriteString("- Allowed globs:\n")
	for _, g := range allow {
		sb.WriteString(fmt.Sprintf("  - %s\n", g))
	}
	sb.WriteString(fmt.Sprintf("- Write your result envelope to `%s` following the result schema.\n", absResultPath))
	if len(checks) == 0 {
		sb.WriteString("- This lane requires no verification command.\n")
	} else {
		for _, c := range checks {
			quoted := "'" + strings.ReplaceAll(c, "'", `'\''`) + "'"
			sb.WriteString(fmt.Sprintf("- As the final verification run exactly `lucind-ai attest run -- sh -c %s` and do not edit files afterwards.\n", quoted))
		}
	}
	sb.WriteString("- Do not edit outside the allowed globs.\n")
	return sb.String()
}

// ConstructBrief formats the lane contract brief markdown.
func ConstructBrief(userBrief, laneID string, allow []string, absResultPath string, checks []string) string {
	return constructBrief(userBrief, laneID, allow, absResultPath, checks)
}

// Dispatch executes the agy lane dispatch workflow in a herdr pane.
func Dispatch(ctx context.Context, opts Options, runner HerdrRunner) (Output, int, error) {
	// 1. HERDR_ENV=1 check
	if os.Getenv("HERDR_ENV") != "1" {
		return Output{}, 1, errors.New("herdr is the only supported runtime: HERDR_ENV must be set to 1")
	}

	for _, c := range opts.Checks {
		if strings.TrimSpace(c) == "" {
			return Output{}, 1, fmt.Errorf("check command cannot be empty")
		}
	}

	// 2. Quota gate
	if opts.MinQuota > 0 {
		if err := ensureAgyQuota(ctx, opts.MinQuota); err != nil {
			return Output{}, 1, fmt.Errorf("quota gate: %w", err)
		}
	}

	if runner == nil {
		runner = DefaultHerdrRunner{}
	}

	cwd := opts.Cwd
	if cwd == "" {
		cwd = "."
	}
	repoRoot, err := attest.RepoToplevel(ctx, cwd)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve repo root: %w", err)
	}

	// 3. Model resolution
	model, err := ResolveModel(opts.Model)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve model: %w", err)
	}

	// 4. Lane creation or continuation
	var l lane.Lane
	isContinuation := opts.LaneID != ""
	if isContinuation {
		loadedLane, err := lane.Load(repoRoot, opts.LaneID)
		if err != nil {
			return Output{}, 1, fmt.Errorf("load lane %s: %w", opts.LaneID, err)
		}
		if loadedLane.Status == lane.StatusAccepted || loadedLane.Status == lane.StatusRejected {
			return Output{}, 1, fmt.Errorf("cannot continue lane %s with status %s", opts.LaneID, loadedLane.Status)
		}
		loadedLane.Status = lane.StatusRunning
		loadedLane.Retries = 0
		if len(opts.Checks) > 0 {
			loadedLane.Checks = opts.Checks
		}
		if err := loadedLane.Save(repoRoot); err != nil {
			return Output{}, 1, fmt.Errorf("save lane %s: %w", opts.LaneID, err)
		}
		l = loadedLane
	} else {
		createdLane, err := lane.Create(ctx, cwd, opts.Allow, model, opts.Checks...)
		if err != nil {
			return Output{}, 1, fmt.Errorf("create lane: %w", err)
		}
		l = createdLane
	}

	// 5. Full prompt construction
	resultRelPath := lane.ResultPath(repoRoot, l.ID)
	absResultPath, err := filepath.Abs(resultRelPath)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve abs result path: %w", err)
	}

	allow := l.Allow
	if isContinuation && len(opts.Allow) > 0 {
		allow = opts.Allow
	}
	briefText := constructBrief(opts.Brief, l.ID, allow, absResultPath, l.Checks)
	briefPath := filepath.Join(lane.LaneDir(repoRoot, l.ID), "brief.md")
	if err := os.WriteFile(briefPath, []byte(briefText), 0644); err != nil {
		return Output{}, 1, fmt.Errorf("write brief.md: %w", err)
	}
	absBriefPath, err := filepath.Abs(briefPath)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve abs brief path: %w", err)
	}

	// 6. Pane management
	if !isContinuation {
		direction := DetermineSplitDirection(ctx, runner)
		paneID, err := SplitPane(ctx, runner, direction, cwd, l.ID)
		if err != nil {
			return Output{}, 1, fmt.Errorf("split pane: %w", err)
		}
		l.PaneID = paneID
		if err := l.Save(repoRoot); err != nil {
			return Output{}, 1, fmt.Errorf("save lane with pane id: %w", err)
		}

		last4 := l.ID
		if len(l.ID) > 4 {
			last4 = l.ID[len(l.ID)-4:]
		}
		agentName := "lane-" + last4
		startArgs := []string{
			"agent", "start", agentName,
			"--kind", "agy",
			"--pane", paneID,
			"--",
			"--dangerously-skip-permissions",
			"--model", model,
		}
		if out, err := runner.Run(ctx, startArgs...); err != nil {
			return Output{}, 1, fmt.Errorf("herdr agent start: %w (output: %s)", err, string(out))
		}
		// The agy TUI drops input sent before it is idle, so wait for readiness.
		waitArgs := []string{"agent", "wait", l.PaneID, "--until", "idle", "--timeout", "60000"}
		if out, err := runner.Run(ctx, waitArgs...); err != nil {
			return Output{}, 1, fmt.Errorf("herdr agent wait: %w (output: %s)", err, string(out))
		}
	}

	promptText := fmt.Sprintf("Read and follow %s", absBriefPath)
	if err := sendPrompt(ctx, runner, l.PaneID, promptText, absBriefPath); err != nil {
		return Output{}, 1, err
	}

	// 7. Wait loop or detach
	if opts.Detach {
		out := Output{
			Lane:       l.ID,
			PaneID:     l.PaneID,
			Cwd:        cwd,
			Status:     string(lane.StatusRunning),
			ResultPath: absResultPath,
		}
		return out, 0, nil
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Minute
	}
	return waitLoop(ctx, repoRoot, l.ID, cwd, timeout)
}

// sendPrompt submits the prompt and has herdr confirm that agy started a turn
// (working) or is blocked. When herdr reports an error (for example
// agent_prompt_stalled) it does not resend blindly: it reads the pane and
// resends once only if the brief path is absent, otherwise it treats the
// prompt as delivered.
func sendPrompt(ctx context.Context, runner HerdrRunner, paneID, text, marker string) error {
	args := []string{
		"agent", "prompt", paneID, text,
		"--wait", "--until", "working", "--until", "blocked", "--timeout", "30000",
	}
	out, err := runner.Run(ctx, args...)
	if err == nil {
		return nil
	}
	firstErr := fmt.Errorf("herdr agent prompt: %w (output: %s)", err, string(out))

	readArgs := []string{"pane", "read", paneID, "--source", "recent-unwrapped", "--lines", "40"}
	pane, rerr := runner.Run(ctx, readArgs...)
	if rerr != nil {
		return fmt.Errorf("%w; pane read failed: %v", firstErr, rerr)
	}
	if strings.Contains(string(pane), marker) {
		return nil
	}
	if out, err := runner.Run(ctx, args...); err != nil {
		return fmt.Errorf("herdr agent prompt (resend): %w (output: %s)", err, string(out))
	}
	return nil
}
