package executor

import "github.com/LanzerDevCorp/lucind-ai/internal/lane"

// Status maps a dispatch Outcome to a lane status. This mapping lives here
// and nowhere else: internal/barrier has no clock by design and must never
// learn about timeouts or exit codes directly.
//
// Exit 0 deliberately maps to lane.Running, not lane.Done: exit 0 only
// means the child process ran to completion, not that the requested work
// succeeded. That verdict is decided by the result envelope, read by a
// different package (internal/result), which is the only place allowed to
// promote a lane to Done.
func Status(o Outcome) lane.Status {
	if o.TimedOut {
		return lane.Blocked
	}
	if o.ExitCode != 0 {
		return lane.Blocked
	}
	return lane.Running
}
