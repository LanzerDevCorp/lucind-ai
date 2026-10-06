package lane

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MaxRetries        = 2
	RetryQuietWindow  = 60 * time.Second
	MaxTotalContinues = 10
)

type StopAction string

const (
	StopActionContinue StopAction = "continue"
	StopActionStop     StopAction = "stop"
)

type StopDecision struct {
	Action           StopAction
	Status           Status
	Retries          int
	Continues        int
	QuietWindowReset bool
	QuietWindowDur   time.Duration
	LimitExhausted   string
	Reason           string
	ReadErr          error
}

// CanContinue reports whether a lane with status s can begin a new turn.
// Returns false for StatusAccepted, StatusRejected, or !s.Valid().
// Returns true otherwise (StatusRunning, StatusDone, StatusFailed, StatusTimeout).
func (s Status) CanContinue() bool {
	if !s.Valid() || s == StatusAccepted || s == StatusRejected {
		return false
	}
	return true
}

// CanContinue reports whether the lane can begin a new turn.
func (l Lane) CanContinue() bool {
	return l.Status.CanContinue()
}

// BeginTurn transitions a lane into a new execution turn.
// It validates that the lane can continue, rotates legacy result files if necessary,
// increments turn, resets stop and retry counters, optionally updates allow and checks,
// and atomically persists the lane.
func (l *Lane) BeginTurn(root string, allow []string, checks []string) error {
	if !l.CanContinue() {
		return fmt.Errorf("cannot continue lane %s with status %s", l.ID, l.Status)
	}

	if l.Turn == 0 {
		laneDir := LaneDir(root, l.ID)
		src := filepath.Join(laneDir, "result.json")
		dst := filepath.Join(laneDir, "result.prev.json")
		if err := os.Rename(src, dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("rename previous result: %w", err)
		}
		l.Turn = 1
	} else {
		l.Turn++
	}

	l.Status = StatusRunning
	l.Retries = 0
	l.Continues = 0
	l.LastStopAt = nil

	if len(allow) > 0 {
		l.Allow = allow
	}
	if len(checks) > 0 {
		l.Checks = checks
	}

	l.UpdatedAt = time.Now().UTC()
	return l.Save(root)
}

// RecordStop evaluates a stop event for a running lane.
func (l *Lane) RecordStop(root string, at time.Time) (StopDecision, error) {
	if l.Status != StatusRunning {
		return StopDecision{}, fmt.Errorf("cannot record stop on lane %s with status %s", l.ID, l.Status)
	}

	if at.IsZero() {
		at = time.Now().UTC()
	} else {
		at = at.UTC()
	}

	_, outcome, readErr := l.CurrentResult(root)

	if readErr != nil {
		var quietWindowReset bool
		var quietWindowDur time.Duration
		if l.LastStopAt != nil && at.Sub(*l.LastStopAt) >= RetryQuietWindow {
			quietWindowReset = true
			quietWindowDur = at.Sub(*l.LastStopAt)
			l.Retries = 0
		}

		if l.Retries < MaxRetries && l.Continues < MaxTotalContinues {
			l.Retries++
			l.Continues++
			stopTime := at
			l.LastStopAt = &stopTime
			l.UpdatedAt = at
			if err := l.Save(root); err != nil {
				return StopDecision{}, err
			}

			msg := readErr.Error()
			if len(msg) > 1024 {
				msg = strings.ToValidUTF8(msg[:1024], "")
			}
			if errors.Is(readErr, fs.ErrNotExist) {
				msg = "the file does not exist"
			}

			return StopDecision{
				Action:           StopActionContinue,
				Status:           l.Status,
				Retries:          l.Retries,
				Continues:        l.Continues,
				QuietWindowReset: quietWindowReset,
				QuietWindowDur:   quietWindowDur,
				Reason:           msg,
				ReadErr:          readErr,
			}, nil
		}

		l.Status = StatusFailed
		l.UpdatedAt = at
		if err := l.Save(root); err != nil {
			return StopDecision{}, err
		}

		limitExhausted := "retries"
		if l.Continues >= MaxTotalContinues {
			limitExhausted = "total continues"
		}

		return StopDecision{
			Action:         StopActionStop,
			Status:         StatusFailed,
			Retries:        l.Retries,
			Continues:      l.Continues,
			LimitExhausted: limitExhausted,
			ReadErr:        readErr,
		}, nil
	}

	finalStatus := StatusDone
	if outcome != ResultOutcomeDone {
		finalStatus = StatusFailed
	}
	l.Status = finalStatus
	l.UpdatedAt = at
	if err := l.Save(root); err != nil {
		return StopDecision{}, err
	}

	return StopDecision{
		Action:    StopActionStop,
		Status:    finalStatus,
		Retries:   l.Retries,
		Continues: l.Continues,
		ReadErr:   nil,
	}, nil
}

// MarkTimeout marks a running or failed lane as timed out.
func (l *Lane) MarkTimeout(root string) error {
	if l.Status != StatusRunning && l.Status != StatusFailed {
		return fmt.Errorf("cannot mark lane %s timed out with status %s", l.ID, l.Status)
	}

	l.Status = StatusTimeout
	l.UpdatedAt = time.Now().UTC()
	return l.Save(root)
}

// RecordVerdict records a review verdict (accepted or rejected) on a lane.
func (l *Lane) RecordVerdict(root string, verdict string) error {
	if verdict != VerdictAccepted && verdict != VerdictRejected {
		return fmt.Errorf("invalid verdict %q: must be %q or %q", verdict, VerdictAccepted, VerdictRejected)
	}

	if !l.Status.Valid() {
		return fmt.Errorf("cannot record verdict on lane %s with status %s", l.ID, l.Status)
	}

	if verdict == VerdictAccepted {
		l.Status = StatusAccepted
	} else {
		l.Status = StatusRejected
	}

	l.UpdatedAt = time.Now().UTC()
	return l.Save(root)
}

// MarkStopped marks a stopped lane as done or failed based on current turn result validation.
func (l *Lane) MarkStopped(root string) (Status, error) {
	if l.Status != StatusRunning && l.Status != StatusFailed {
		return Status(""), fmt.Errorf("cannot mark stopped on lane %s with status %s", l.ID, l.Status)
	}

	_, outcome, _ := l.CurrentResult(root)
	finalStatus := StatusDone
	if outcome != ResultOutcomeDone {
		finalStatus = StatusFailed
	}

	l.Status = finalStatus
	l.UpdatedAt = time.Now().UTC()

	if err := l.Save(root); err != nil {
		return Status(""), err
	}

	return finalStatus, nil
}
