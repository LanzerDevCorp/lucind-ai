package dispatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/repo"
)

var pollInterval = 1 * time.Second
var sleepFunc = time.Sleep
var exhaustionGrace = 10 * time.Minute

// SetSleepForTesting allows tests to control polling delay.
func SetSleepForTesting(fn func(time.Duration), interval time.Duration) {
	sleepFunc = fn
	pollInterval = interval
}

// ResetSleepForTesting restores production sleep behavior.
func ResetSleepForTesting() {
	sleepFunc = time.Sleep
	pollInterval = 1 * time.Second
}

// SetExhaustionGraceForTesting allows tests to control the grace period for failed lanes.
func SetExhaustionGraceForTesting(d time.Duration) {
	exhaustionGrace = d
}

// ResetExhaustionGraceForTesting restores production exhaustion grace behavior.
func ResetExhaustionGraceForTesting() {
	exhaustionGrace = 10 * time.Minute
}

func waitLoop(ctx context.Context, repoRoot, laneID, cwd string, timeout time.Duration) (Output, int, error) {
	if timeout <= 0 {
		timeout = 60 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	var graceDeadline time.Time

	for {
		l, err := lane.Load(repoRoot, laneID)
		if err != nil {
			return Output{}, ExitError, fmt.Errorf("load lane %s: %w", laneID, err)
		}

		absResultPath, _ := filepath.Abs(lane.ResultFilePath(repoRoot, l))

		outCwd := cwd
		if outCwd == "" {
			outCwd = l.Cwd
		}

		makeOut := func(status string) Output {
			return Output{
				Lane:       l.ID,
				PaneID:     l.PaneID,
				Cwd:        outCwd,
				Status:     status,
				ResultPath: absResultPath,
			}
		}

		if l.Status == lane.StatusDone {
			_, outcome, _ := l.CurrentResult(repoRoot)
			if outcome == lane.ResultOutcomeDone {
				return makeOut(string(lane.StatusDone)), ExitDone, nil
			}
			return makeOut(string(lane.StatusFailed)), ExitFailed, nil
		}
		if l.Status == lane.StatusFailed {
			_, outcome, _ := l.CurrentResult(repoRoot)
			if outcome == lane.ResultOutcomeDone {
				_, _ = lane.MarkStopped(repoRoot, laneID)
				return makeOut(string(lane.StatusDone)), ExitDone, nil
			}
			if outcome == lane.ResultOutcomeNotDone {
				return makeOut(string(lane.StatusFailed)), ExitFailed, nil
			}

			// Trade-off: a genuine failure is reported up to exhaustionGrace later.
			if graceDeadline.IsZero() {
				graceDeadline = time.Now().Add(exhaustionGrace)
			}

			select {
			case <-ctx.Done():
				_ = l.MarkTimeout(repoRoot)
				return makeOut(string(lane.StatusTimeout)), ExitTimeout, nil
			default:
			}

			if time.Now().After(deadline) && !graceDeadline.Before(deadline) {
				_ = l.MarkTimeout(repoRoot)
				return makeOut(string(lane.StatusTimeout)), ExitTimeout, nil
			}

			if !time.Now().Before(graceDeadline) {
				return makeOut(string(lane.StatusFailed)), ExitFailed, nil
			}
		}
		if l.Status == lane.StatusTimeout {
			return makeOut(string(lane.StatusTimeout)), ExitTimeout, nil
		}

		select {
		case <-ctx.Done():
			_ = l.MarkTimeout(repoRoot)
			return makeOut(string(lane.StatusTimeout)), ExitTimeout, nil
		default:
		}

		if time.Now().After(deadline) {
			_ = l.MarkTimeout(repoRoot)
			return makeOut(string(lane.StatusTimeout)), ExitTimeout, nil
		}

		sleepFunc(pollInterval)
	}
}

// Wait polls lane status until done, failed, or timeout.
func Wait(ctx context.Context, repoRoot, laneID string, timeout time.Duration) (Output, int, error) {
	if os.Getenv("HERDR_ENV") != "1" {
		return Output{}, ExitError, errors.New("herdr is the only supported runtime: HERDR_ENV must be set to 1")
	}

	if repoRoot == "" {
		root, err := repo.Toplevel(ctx, ".")
		if err != nil {
			return Output{}, ExitError, fmt.Errorf("resolve repo root: %w", err)
		}
		repoRoot = root
	}

	l, err := lane.Load(repoRoot, laneID)
	if err != nil {
		return Output{}, ExitError, fmt.Errorf("load lane %s: %w", laneID, err)
	}

	return waitLoop(ctx, repoRoot, laneID, l.Cwd, timeout)
}
