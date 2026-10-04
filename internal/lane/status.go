package lane

// Status represents a lane's execution and review lifecycle state.
type Status string

const (
	StatusRunning  Status = "running"
	StatusDone     Status = "done"
	StatusFailed   Status = "failed"
	StatusTimeout  Status = "timeout"
	StatusAccepted Status = "accepted"
	StatusRejected Status = "rejected"

	// Deprecated aliases kept for compatibility during migration.
	Running         = StatusRunning
	Done            = StatusDone
	Failed          = StatusFailed
	Pending  Status = "pending"
	Blocked  Status = "blocked"
	Deviated Status = "deviated"
)

// Terminal reports whether s is a terminal state.
// Execution outcomes (done, failed, timeout) and review outcomes
// (accepted, rejected) are terminal. Running is not terminal.
func (s Status) Terminal() bool {
	switch s {
	case StatusDone, StatusFailed, StatusTimeout, StatusAccepted, StatusRejected:
		return true
	default:
		return false
	}
}

// Valid reports whether s is one of the valid statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusRunning, StatusDone, StatusFailed, StatusTimeout, StatusAccepted, StatusRejected:
		return true
	default:
		return false
	}
}

// State represents a lane identifier and its status.
type State struct {
	LaneID string
	Status Status
}
