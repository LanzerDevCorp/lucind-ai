# Spec Lens B — Scenarios & Coverage: Control Room Capture

## Assumed requirements

The control-room-capture change introduces continuous file-backed stream spooling under `<primaryRoot>/.lucind/` for all executors (`agy`, `cursor-agent`, `opencode`) across all terminal statuses (`Done`, `Blocked`, `Failed`, `Deviated`) surviving worktree deletion (`Continuous primary-root stream spooling`). It preserves non-interfering `WaitDelay` pipe draining so grandchild processes holding pipes truncate capture without altering exit codes or failing lanes (`Non-interfering WaitDelay drain`). It maintains bounded SQLite ledger notes capped at 4096 bytes per stream while retaining full streams on disk (`Bounded SQLite diagnostics`). Finally, it provides read-only loopback HTTP endpoints on `lucind-ai serve` for live SSE tailing and post-mortem transcript download without stalling child execution (`Loopback HTTP stream access`).

## Scenarios

### Requirement: Continuous primary-root stream spooling

#### Scenario: Successful lane retains complete log
- GIVEN a lane completes with exit code 0 and a valid Done envelope
- WHEN completeIntegration deletes the lane worktree
- THEN the primary-root log file under .lucind/ MUST retain full unclipped stdout and stderr

#### Scenario: Uniform tee across all executors
- GIVEN an invocation of agy, cursor-agent, or opencode executor
- WHEN the child process writes output to stdout or stderr
- THEN bytes MUST be incrementally tee'd to the primary-root log file and captured in Outcome

#### Scenario: Logs survive worktree cleanup on all terminal statuses
- GIVEN a lane ending in Done, Blocked, Failed, or Deviated
- WHEN worktree cleanup executes
- THEN the log file on primary root .lucind/ MUST remain intact on disk

#### Scenario: Log path outside primary root is rejected
- GIVEN a candidate log destination residing in a worktree path
- WHEN ledgerpath.Validate checks the destination path
- THEN Validate MUST return ErrLedgerOutsidePrimaryRepo and refuse the path

### Requirement: Non-interfering WaitDelay drain

#### Scenario: Clean exit with complete drain
- GIVEN a child process that exits 0 and closes pipes within WaitDelay
- WHEN the executor awaits process completion
- THEN OutputTruncated MUST be false and the lane status is Done

#### Scenario: Grandchild holds pipes past WaitDelay
- GIVEN a child process that exits 0 while a grandchild retains stdout or stderr pipes
- WHEN WaitDelay expires with exec.ErrWaitDelay
- THEN OutputTruncated MUST be true, the real exit code 0 is preserved, and status remains Done

#### Scenario: Truncation diagnostic recorded without failing lane
- GIVEN a completed lane with OutputTruncated set to true
- WHEN Execute persists terminal status
- THEN an EventLaneNote with outputTruncatedDetail MUST be appended to the ledger without altering lane status

### Requirement: Bounded SQLite diagnostics

#### Scenario: Clean Done appends no failure note
- GIVEN a lane with terminal status Done and empty failure reason
- WHEN Execute persists the lane run
- THEN no EventLaneNote failure record MUST be appended to the SQLite ledger

#### Scenario: Failure output clipped to 4096 bytes in ledger
- GIVEN a failed lane producing 50KiB of combined stdout and stderr
- WHEN Execute writes the failure diagnosis to the ledger
- THEN the EventLaneNote detail MUST contain at most a 4096-byte tail per stream plus truncation marker
- AND the primary-root log file MUST retain the full unclipped 50KiB

#### Scenario: Timed-out dispatch records bounded diagnostic
- GIVEN a lane dispatch that exceeds its context timeout
- WHEN Execute records the Blocked status
- THEN the appended EventLaneNote detail MUST be bounded to 4096 bytes per stream

### Requirement: Loopback HTTP stream access

#### Scenario: Live SSE tail of in-flight lane
- GIVEN an active running lane dispatch
- WHEN an HTTP client requests GET stream endpoint on the loopback server
- THEN the server MUST stream file appends via SSE with text/event-stream without blocking child execution

#### Scenario: Post-mortem download of finished lane transcript
- GIVEN a finished lane with a persisted log file on primary root
- WHEN an HTTP client requests the lane transcript endpoint
- THEN the server MUST return HTTP 200 with the full stored log content

#### Scenario: Client disconnect during live tail
- GIVEN an active SSE live tail connection
- WHEN the HTTP client disconnects closing request context
- THEN the SSE handler MUST terminate its tail loop cleanly without leaking goroutines or stalling the child

#### Scenario: Log request for non-existent lane returns 404
- GIVEN a request for a lane ID with no corresponding log file
- WHEN the client requests the log endpoint
- THEN the server MUST return HTTP 404 Not Found

#### Scenario: Non-loopback bind address rejected
- GIVEN a server startup configuration with address 0.0.0.0:7433
- WHEN lucind-ai serve initializes
- THEN the server MUST return ErrNonLoopback and refuse to start

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Continuous primary-root stream spooling | covered | covered | covered | internal/executor/agy_test.go:28, internal/run/integrate_test.go:392 |
| Non-interfering WaitDelay drain | covered | covered | covered | internal/run/run_test.go:820, internal/executor/agy_test.go:158 |
| Bounded SQLite diagnostics | covered | covered | covered | internal/run/run_test.go:645, internal/run/run.go:132 |
| Loopback HTTP stream access | covered | covered | covered | internal/serve/server_test.go:17, new seam required |

## Untestable Assertions

None

## Open Questions

- [ ] Directory layout: `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`, and interleaved file vs split `.stdout.log`/`.stderr.log`.
- [ ] Route registration ownership: whether log SSE, download, and Model JSON routes register in `approvals-web-ui` (`internal/serve/handlers.go:36-118`) or in downstream `control-room-serve`.
- [ ] Retention policy: whether run logs are archived/pruned via `lucind-ai worktree cleanup` / dedicated command, or retained as gitignored files indefinitely.
