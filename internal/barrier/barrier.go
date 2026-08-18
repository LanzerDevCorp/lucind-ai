// Package barrier is a pure, in-memory join over lane states. It has no
// clock, no timer, and no I/O: it releases based solely on the lane states
// it is handed, and it never imports internal/ledger.
package barrier

import (
	"errors"
	"fmt"
	"sync"

	"github.com/LanzerDevCorp/lucind-ai/internal/lane"
)

// ErrNoLanes is returned by New when the lane ID list is empty.
var ErrNoLanes = errors.New("barrier: at least one lane id is required")

// ErrUnexpectedLane is returned by Observe when a lane ID was not part of
// the batch the Barrier was constructed with.
var ErrUnexpectedLane = errors.New("barrier: observed lane is not part of this batch")

// Outcome is the result of evaluating a batch of lanes.
type Outcome struct {
	Released bool
	// Integrate holds the Done lanes, eligible for the combined tree.
	Integrate []string
	// Preserve holds every other terminal lane (blocked/deviated/failed);
	// their worktrees are preserved and they are excluded from integration.
	Preserve []string
}

// Evaluate is pure: no receiver, no shared state, no I/O. It releases the
// batch if and only if every lane in expected has a terminal observed
// status. A lane with no observed entry at all is treated as non-terminal,
// so a missing observation never releases the batch and never fabricates a
// timeout-driven state.
func Evaluate(expected []string, observed []lane.State) Outcome {
	byLane := make(map[string]lane.Status, len(observed))
	for _, s := range observed {
		byLane[s.LaneID] = s.Status
	}

	for _, id := range expected {
		st, ok := byLane[id]
		if !ok || !st.Terminal() {
			return Outcome{Released: false}
		}
	}

	integrate := make([]string, 0, len(expected))
	preserve := make([]string, 0, len(expected))
	for _, id := range expected {
		if byLane[id] == lane.Done {
			integrate = append(integrate, id)
		} else {
			preserve = append(preserve, id)
		}
	}

	return Outcome{Released: true, Integrate: integrate, Preserve: preserve}
}

// Barrier joins a fixed batch of lanes and releases exactly once, when
// every lane has reached a terminal state. It holds no clock, timer, or
// ledger reference: it only reacts to states handed to it via Observe.
type Barrier struct {
	mu       sync.Mutex
	expected []string
	observed map[string]lane.Status
	released bool
	done     chan struct{}
}

// New constructs a Barrier for exactly the given lane IDs. It rejects an
// empty list and duplicate lane IDs.
func New(laneIDs []string) (*Barrier, error) {
	if len(laneIDs) == 0 {
		return nil, ErrNoLanes
	}

	seen := make(map[string]struct{}, len(laneIDs))
	expected := make([]string, 0, len(laneIDs))
	for _, id := range laneIDs {
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("barrier: duplicate lane id %q", id)
		}
		seen[id] = struct{}{}
		expected = append(expected, id)
	}

	return &Barrier{
		expected: expected,
		observed: make(map[string]lane.Status, len(expected)),
		done:     make(chan struct{}),
	}, nil
}

// Observe records a lane's latest state. It is safe for concurrent use.
// It errors if the lane was not part of the batch New was called with.
// A lane's terminal status is write-once: once a lane is terminal, later
// Observe calls for that lane are accepted (no error) but ignored, so a
// late or duplicate update can never re-fire release or change Outcome.
func (b *Barrier) Observe(s lane.State) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	inBatch := false
	for _, id := range b.expected {
		if id == s.LaneID {
			inBatch = true
			break
		}
	}
	if !inBatch {
		return ErrUnexpectedLane
	}

	if b.released {
		return nil
	}

	if existing, ok := b.observed[s.LaneID]; ok && existing.Terminal() {
		return nil
	}
	b.observed[s.LaneID] = s.Status

	outcome := b.evaluateLocked()
	if outcome.Released {
		b.released = true
		close(b.done)
	}

	return nil
}

// Outcome reports the current release/eligibility result for this batch,
// computed deterministically from the states observed so far.
func (b *Barrier) Outcome() Outcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.evaluateLocked()
}

// Done returns a channel that is closed exactly once, when every lane in
// the batch has reached a terminal state.
func (b *Barrier) Done() <-chan struct{} {
	return b.done
}

func (b *Barrier) evaluateLocked() Outcome {
	states := make([]lane.State, 0, len(b.observed))
	for id, st := range b.observed {
		states = append(states, lane.State{LaneID: id, Status: st})
	}
	return Evaluate(b.expected, states)
}
