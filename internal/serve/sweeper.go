package serve

import (
	"context"
	"errors"
	"log"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
	"github.com/LanzerDevCorp/lucind-ai/internal/ledger"
)

const defaultSweepInterval = 10 * time.Second

const orphanNote = "orphaned: driving process no longer running"

// SweeperConfig controls how often serve rechecks driving-process liveness.
type SweeperConfig struct {
	Interval time.Duration
}

// Sweeper reconciles running lanes whose RegisterRun PID is no longer alive.
// Liveness probing is Linux-only; on other GOOS values Run waits on ctx only.
type Sweeper struct {
	ledger   *ledger.Ledger
	interval time.Duration
}

// NewSweeper constructs a Sweeper over l. Interval defaults to 10s.
func NewSweeper(l *ledger.Ledger, cfg SweeperConfig) *Sweeper {
	interval := cfg.Interval
	if interval <= 0 {
		interval = defaultSweepInterval
	}
	return &Sweeper{ledger: l, interval: interval}
}

// Run performs one immediate sweep, then ticks until ctx is canceled.
func (s *Sweeper) Run(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		<-ctx.Done()
		return nil
	}
	if err := s.SweepOnce(ctx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.SweepOnce(ctx); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return err
			}
		}
	}
}

// SweepOnce runs a single orphan-reconciliation pass.
func (s *Sweeper) SweepOnce(ctx context.Context) error {
	return s.sweep(ctx)
}

func (s *Sweeper) sweep(ctx context.Context) error {
	runs, err := s.ledger.ListRuns(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, run := range runs {
		if run.PID <= 0 {
			continue
		}
		alive, probeErr := processAlive(run.PID)
		if probeErr != nil {
			log.Printf("serve: sweeper: probe pid %d for run %s: %v", run.PID, run.RunID, probeErr)
			continue
		}
		if alive {
			continue
		}
		lanes, err := s.ledger.Lanes(ctx, run.RunID)
		if err != nil {
			return err
		}
		for _, ln := range lanes {
			if ln.Status != lane.Running {
				continue
			}
			if err := s.ledger.SetStatus(ctx, run.RunID, ln.LaneID, lane.Failed, now); err != nil {
				return err
			}
			if err := s.ledger.AppendEvent(ctx, ledger.Event{
				RunID: run.RunID, LaneID: ln.LaneID, Type: ledger.EventLaneNote,
				Detail: orphanNote, At: now,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func processAlive(pid int) (bool, error) {
	if pid <= 0 {
		return true, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = proc.Signal(syscall.Signal(0))
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.EPERM):
		return true, nil
	case errors.Is(err, syscall.ESRCH), errors.Is(err, os.ErrProcessDone):
		return false, nil
	default:
		return false, err
	}
}
