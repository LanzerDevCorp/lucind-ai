package dispatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/attest"
	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/result"
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
	absResultPath, _ := filepath.Abs(lane.ResultPath(repoRoot, laneID))
	var graceDeadline time.Time

	for {
		l, err := lane.Load(repoRoot, laneID)
		if err != nil {
			return Output{}, 1, fmt.Errorf("load lane %s: %w", laneID, err)
		}

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
			return makeOut(string(lane.StatusDone)), 0, nil
		}
		if l.Status == lane.StatusFailed {
			env, err := result.Read(os.DirFS(lane.LaneDir(repoRoot, laneID)), "result.json")
			if err == nil {
				if env.Status == "done" {
					_, _ = lane.MarkStopped(repoRoot, laneID)
					return makeOut(string(lane.StatusDone)), 0, nil
				}
				return makeOut(string(lane.StatusFailed)), 3, nil
			}

			// Trade-off: a genuine failure is reported up to exhaustionGrace later.
			if graceDeadline.IsZero() {
				graceDeadline = time.Now().Add(exhaustionGrace)
			}

			select {
			case <-ctx.Done():
				l.Status = lane.StatusTimeout
				_ = l.Save(repoRoot)
				return makeOut(string(lane.StatusTimeout)), 4, nil
			default:
			}

			if time.Now().After(deadline) && !graceDeadline.Before(deadline) {
				l.Status = lane.StatusTimeout
				_ = l.Save(repoRoot)
				return makeOut(string(lane.StatusTimeout)), 4, nil
			}

			if !time.Now().Before(graceDeadline) {
				return makeOut(string(lane.StatusFailed)), 3, nil
			}
		}
		if l.Status == lane.StatusTimeout {
			return makeOut(string(lane.StatusTimeout)), 4, nil
		}

		select {
		case <-ctx.Done():
			l.Status = lane.StatusTimeout
			_ = l.Save(repoRoot)
			return makeOut(string(lane.StatusTimeout)), 4, nil
		default:
		}

		if time.Now().After(deadline) {
			l.Status = lane.StatusTimeout
			_ = l.Save(repoRoot)
			return makeOut(string(lane.StatusTimeout)), 4, nil
		}

		sleepFunc(pollInterval)
	}
}

// Wait polls lane status until done, failed, or timeout.
func Wait(ctx context.Context, repoRoot, laneID string, timeout time.Duration, runner HerdrRunner) (Output, int, error) {
	if os.Getenv("HERDR_ENV") != "1" {
		return Output{}, 1, errors.New("herdr is the only supported runtime: HERDR_ENV must be set to 1")
	}

	if repoRoot == "" {
		root, err := attest.RepoToplevel(ctx, ".")
		if err != nil {
			return Output{}, 1, fmt.Errorf("resolve repo root: %w", err)
		}
		repoRoot = root
	}

	l, err := lane.Load(repoRoot, laneID)
	if err != nil {
		return Output{}, 1, fmt.Errorf("load lane %s: %w", laneID, err)
	}

	return waitLoop(ctx, repoRoot, laneID, l.Cwd, timeout)
}
