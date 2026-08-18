// Package lane is the zero-dependency vocabulary shared by internal/barrier
// and internal/ledger. It imports nothing outside the standard library, so
// that "the barrier never imports the ledger" is a compile-time fact rather
// than a review convention.
package lane

// Status is a lane's execution state.
type Status string

const (
	Pending  Status = "pending"
	Running  Status = "running"
	Done     Status = "done"
	Blocked  Status = "blocked"
	Deviated Status = "deviated"
	Failed   Status = "failed"
)

// Terminal reports whether s is one of the four terminal states
// (done, blocked, deviated, failed). Pending and running are not terminal.
func (s Status) Terminal() bool {
	switch s {
	case Done, Blocked, Deviated, Failed:
		return true
	default:
		return false
	}
}

// Valid reports whether s is one of the six defined statuses.
func (s Status) Valid() bool {
	switch s {
	case Pending, Running, Done, Blocked, Deviated, Failed:
		return true
	default:
		return false
	}
}

// State is the only type that crosses the ledger -> barrier boundary.
type State struct {
	LaneID string
	Status Status
}
