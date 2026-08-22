# Delta for control-room-capture

## ADDED Requirements

### Requirement: Continuous primary-root stream spooling

Executors MUST stream child stdout and stderr to durable files under `<primaryRoot>/.lucind/…` for `agy`, `cursor-agent`, and `opencode`. Spool files MUST be created at process spawn and MUST persist across `Done`, `Blocked`, `Failed`, and `Deviated` lane outcomes, remaining accessible after worktree deletion.

#### Scenario: Successful lane retains complete log

- GIVEN a lane completes with exit code 0 and a valid Done envelope
- WHEN integrate removes the lane worktree
- THEN the primary-root log under `.lucind/` MUST retain full unclipped stdout and stderr

#### Scenario: Uniform capture across all executors

- GIVEN an invocation of the agy, cursor-agent, or opencode executor
- WHEN the child process writes output to stdout or stderr
- THEN those bytes MUST appear incrementally in the primary-root log and remain available for diagnosis

#### Scenario: Logs survive worktree cleanup on all terminal statuses

- GIVEN a lane ending in Done, Blocked, Failed, or Deviated
- WHEN worktree cleanup executes
- THEN the log file on primary-root `.lucind/` MUST remain intact

#### Scenario: Log path outside primary root is rejected

- GIVEN a candidate log destination residing in a worktree path
- WHEN the system checks the destination path
- THEN it MUST refuse to write the log there

### Requirement: Bounded SQLite diagnostics

`events.detail` in the ledger MUST stay capped at 4096 bytes per stream for failed, timed-out, or unreadable-envelope dispatches, and the binary MUST NOT store unclipped stream blobs in SQLite. Completed lanes with `lane.Done` and an empty failure reason MUST NOT write a failure-detail `EventLaneNote`.

#### Scenario: Clean Done appends no failure note

- GIVEN a lane with terminal status Done and empty failure reason
- WHEN the lane run is persisted
- THEN a failure-detail EventLaneNote MUST NOT be appended to the ledger

#### Scenario: Failure output clipped to 4096 bytes in ledger

- GIVEN a failed lane producing 50KiB of combined stdout and stderr
- WHEN the failure diagnosis is written to the ledger
- THEN the EventLaneNote detail MUST contain at most a 4096-byte tail per stream plus truncation marker
- AND the primary-root log file MUST retain the full unclipped 50KiB

#### Scenario: Timed-out dispatch records bounded diagnostic

- GIVEN a lane dispatch that exceeds its context timeout
- WHEN Blocked status is recorded
- THEN the appended EventLaneNote detail MUST be bounded to 4096 bytes per stream
