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
)

var pollInterval = 1 * time.Second
var sleepFunc = time.Sleep

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

func waitLoop(ctx context.Context, repoRoot, laneID, cwd string, timeout time.Duration) (Output, int, error) {
	if timeout <= 0 {
		timeout = 60 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	absResultPath, _ := filepath.Abs(lane.ResultPath(repoRoot, laneID))

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
			return makeOut(string(lane.StatusFailed)), 3, nil
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
