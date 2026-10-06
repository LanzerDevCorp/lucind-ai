package dispatch

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	Cwd        string
	LaneID     string
	Allow      []string
	Checks     []string
	Model      string
	Prompt     string
	Brief      string
	MinQuota   float64
	Detach     bool
	Timeout    time.Duration
	AutoSkills bool
	Stderr     io.Writer
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

func extractSkillsSection(prompt string) (string, string) {
	lines := strings.Split(prompt, "\n")
	skillsStart := -1
	for i, line := range lines {
		if strings.TrimRight(line, " \t\r") == "## Skills to load before work" {
			skillsStart = i
			break
		}
	}
	if skillsStart == -1 {
		return "", strings.TrimSpace(prompt)
	}

	skillsEnd := len(lines)
	seenContent := false
	for j := skillsStart + 1; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") {
			skillsEnd = j
			break
		}
		if trimmed == "" {
			if seenContent {
				skillsEnd = j
				break
			}
		} else {
			seenContent = true
		}
	}

	skillsSection := strings.TrimSpace(strings.Join(lines[skillsStart:skillsEnd], "\n"))
	before := strings.TrimSpace(strings.Join(lines[:skillsStart], "\n"))
	after := strings.TrimSpace(strings.Join(lines[skillsEnd:], "\n"))

	var rest string
	switch {
	case before != "" && after != "":
		rest = before + "\n\n" + after
	case before != "":
		rest = before
	default:
		rest = after
	}

	return skillsSection, rest
}

func constructPrompt(userPrompt, laneID string, turn int, allow []string, absResultPath string, checks []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "lucind-lane: %s turn: %d", laneID, turn)

	skillsSec, rest := extractSkillsSection(userPrompt)
	if skillsSec != "" {
		sb.WriteString("\n\n")
		sb.WriteString(skillsSec)
	}
	if rest != "" {
		sb.WriteString("\n\n")
		sb.WriteString(rest)
	}

	sb.WriteString("\n\n---\n## Lane Contract\n")
	fmt.Fprintf(&sb, "- Lane ID: %s\n", laneID)
	sb.WriteString("- Allowed globs:\n")
	for _, g := range allow {
		fmt.Fprintf(&sb, "  - %s\n", g)
	}
	fmt.Fprintf(&sb, "- Write your result envelope to `%s` following the result schema.\n", absResultPath)
	if len(checks) == 0 {
		sb.WriteString("- This lane requires no verification command.\n")
	} else {
		sb.WriteString("- As the final verification, after your last edit, run these commands exactly as listed and do not edit files afterwards:\n")
		for _, c := range checks {
			quoted := "'" + strings.ReplaceAll(c, "'", `'\''`) + "'"
			fmt.Fprintf(&sb, "  - `lucind-ai attest run -- sh -c %s`\n", quoted)
		}
	}
	sb.WriteString("- Do not edit outside the allowed globs.\n")
	return sb.String()
}

// ConstructPrompt formats the lane contract prompt markdown.
func ConstructPrompt(userPrompt, laneID string, turn int, allow []string, absResultPath string, checks []string) string {
	return constructPrompt(userPrompt, laneID, turn, allow, absResultPath, checks)
}

// Dispatch executes the agy lane dispatch workflow in a herdr pane.
func Dispatch(ctx context.Context, opts Options, runner HerdrRunner) (Output, int, error) {
	// 1. HERDR_ENV=1 check
	if os.Getenv("HERDR_ENV") != "1" {
		return Output{}, 1, errors.New("herdr is the only supported runtime: HERDR_ENV must be set to 1")
	}

	cwd := opts.Cwd
	if cwd == "" {
		cwd = "."
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve abs cwd: %w", err)
	}
	cwd = absCwd

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

	repoRoot, err := attest.RepoToplevel(ctx, cwd)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve repo root: %w", err)
	}

	// 3. Model resolution
	model, err := ResolveModel(opts.Model)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve model: %w", err)
	}

	// 4. Continuation pre-validation and allow globs resolution
	var loadedLane lane.Lane
	isContinuation := opts.LaneID != ""
	if isContinuation {
		var err error
		loadedLane, err = lane.Load(repoRoot, opts.LaneID)
		if err != nil {
			return Output{}, 1, fmt.Errorf("load lane %s: %w", opts.LaneID, err)
		}
		if !loadedLane.CanContinue() {
			return Output{}, 1, fmt.Errorf("cannot continue lane %s with status %s", opts.LaneID, loadedLane.Status)
		}
	}

	allowGlobs := opts.Allow
	if isContinuation && len(allowGlobs) == 0 {
		allowGlobs = loadedLane.Allow
	}

	userPrompt := opts.Prompt
	if userPrompt == "" {
		userPrompt = opts.Brief
	}

	// 5. Auto-skills selection BEFORE lane creation or mutation
	var autoSkills autoSkillsOutcome
	if opts.AutoSkills {
		var err error
		autoSkills, err = selectAutoSkills(ctx, repoRoot, userPrompt, allowGlobs)
		if err != nil {
			return Output{}, ExitAutoSkillsUnavailable, err
		}
		if autoSkills.sectionToInsert != "" {
			userPrompt = insertSkillsSection(userPrompt, autoSkills.sectionToInsert)
		}
	}

	// 6. Lane creation or continuation
	var l lane.Lane
	if isContinuation {
		if err := loadedLane.BeginTurn(repoRoot, opts.Allow, opts.Checks); err != nil {
			return Output{}, 1, fmt.Errorf("begin continuation turn %s: %w", opts.LaneID, err)
		}
		l = loadedLane
	} else {
		createdLane, err := lane.Create(ctx, cwd, opts.Allow, model, opts.Checks...)
		if err != nil {
			return Output{}, 1, fmt.Errorf("create lane: %w", err)
		}
		l = createdLane
	}

	// 7. Record auto-skills if enabled
	if opts.AutoSkills {
		recordAutoSkills(repoRoot, l, autoSkills, opts.Stderr)
	}

	// 8. Full prompt construction
	resultRelPath := lane.ResultFilePath(repoRoot, l)
	absResultPath, err := filepath.Abs(resultRelPath)
	if err != nil {
		return Output{}, 1, fmt.Errorf("resolve abs result path: %w", err)
	}

	promptText := constructPrompt(userPrompt, l.ID, l.Turn, l.Allow, absResultPath, l.Checks)
	promptPath := filepath.Join(lane.LaneDir(repoRoot, l.ID), "prompt.md")
	if err := os.WriteFile(promptPath, []byte(promptText), 0644); err != nil {
		return Output{}, 1, fmt.Errorf("write prompt.md: %w", err)
	}

	// 6. Pane management
	if !isContinuation {
		direction := DetermineSplitDirection(ctx, runner)
		paneID, err := SplitPane(ctx, runner, direction, cwd, l.ID)
		if err != nil {
			return Output{}, 1, fmt.Errorf("split pane: %w", err)
		}

		paneCwd, err := GetPaneCwd(ctx, runner, paneID)
		if err != nil {
			_ = ClosePane(ctx, runner, paneID)
			return Output{}, 1, fmt.Errorf("verify pane cwd: %w", err)
		}
		if resolveSymlinks(cwd) != resolveSymlinks(paneCwd) {
			_ = ClosePane(ctx, runner, paneID)
			return Output{}, 1, fmt.Errorf("pane cwd mismatch: expected %s, got %s", cwd, paneCwd)
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

	marker := fmt.Sprintf("lucind-lane: %s turn: %d", l.ID, l.Turn)
	if err := sendPrompt(ctx, runner, l.PaneID, promptText, marker); err != nil {
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
// resends once only if the marker line is absent, otherwise it treats the
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

func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}
