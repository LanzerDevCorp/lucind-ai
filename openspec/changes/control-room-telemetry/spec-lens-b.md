# Spec Lens B — Scenarios & Coverage: Control Room Telemetry

## Assumed requirements

This specification defines behavioral proof scenarios across five capabilities (`lane-telemetry-streaming`, `shell-free-telemetry-query`, `approvals-web-ui`, `lane-execution`, and `parent-feature-integration`) introduced or modified by control room telemetry. The four assumed requirements assert: (1) worktree-local log teeing concurrently streams child stdout/stderr to disk and an in-memory hub while preserving `WaitDelay` truncation semantics; (2) loopback SSE endpoints stream live output over `net/http` without violating localhost binding or individual approval rules; (3) ledger isolation ensures high-frequency chunks bypass SQLite `events` and failure diagnostic notes stay capped at 4096 bytes; and (4) shell-free queries and status invariants allow `serve.Model` to read lifecycle events without `os/exec` while maintaining the six-value `lane.Status` enum and batch barrier guarantees.

## Scenarios

### Requirement: Worktree-local log teeing

#### Scenario: Live stdout and stderr streamed concurrently

- GIVEN a lane running in an isolated worktree with output capture configured (`internal/run/run.go:368-375`)
- WHEN the child process writes to stdout and stderr (`internal/executor/agy.go:169-175`)
- THEN output bytes MUST be written to the worktree log file and broadcast to subscribers without blocking the child
- AND `Outcome` MUST capture the final exit code and stream content upon child termination

#### Scenario: Grandchild holds stdio open past process exit

- GIVEN a child process that has exited with code 0 while a grandchild keeps stdio descriptors open (`internal/executor/agy.go:182-197`)
- WHEN `cmd.WaitDelay` elapses (`internal/executor/agy.go:160-168`)
- THEN the executor MUST return `Outcome.OutputTruncated = true` with exit code 0 and MUST NOT hang

#### Scenario: Non-zero exit code with partial capture

- GIVEN a lane execution where the child process exits with code 1 after partial execution
- WHEN `cmd.Run` completes with an exit error (`internal/executor/agy.go:199-205`)
- THEN `Outcome.ExitCode` MUST be 1 and the worktree log MUST retain all output produced prior to exit

### Requirement: Loopback SSE stream

#### Scenario: Loopback client receives live event stream

- GIVEN `lucind-ai serve` running on `127.0.0.1:7433` (`internal/serve/server.go:16-23`)
- WHEN a loopback HTTP client sends a `GET` request to `/api/telemetry/events`
- THEN the server MUST return HTTP 200 with `Content-Type: text/event-stream` and flush lane events as they occur

#### Scenario: Client disconnect cleans up subscription

- GIVEN an active loopback SSE subscriber connection receiving live stream events
- WHEN the client closes the connection or the request context cancels
- THEN the hub MUST unregister the subscriber channel and stop dispatching events without error or leaked goroutines

#### Scenario: Non-loopback address rejected

- GIVEN a non-loopback bind address `0.0.0.0:7433` (`internal/serve/server.go:20-22,57-73`)
- WHEN `serve.ListenAndServe` is called with this address
- THEN the server MUST reject binding, return `ErrNonLoopback`, and exit with an error

### Requirement: Ledger isolation

#### Scenario: High-volume output stays off SQLite ledger

- GIVEN a lane generating multi-megabyte stdout during dispatch (`internal/run/run.go:368-375`)
- WHEN the child process executes to completion
- THEN raw output MUST exist only in the worktree log and in-memory hub while `events` contains only coarse lifecycle rows

#### Scenario: Terminal failure truncates diagnostic note

- GIVEN a failed lane execution where stderr exceeds 4096 bytes (`internal/run/run.go:71-100`)
- WHEN `Execute` records the failure note in the ledger (`internal/run/run.go:422-435`)
- THEN the `lane_note` event detail MUST truncate stderr to 4096 bytes with `streamTruncatedMarker` while full output is preserved in the worktree log

#### Scenario: Invalid event type rejected by ledger schema

- GIVEN an attempt to write a streaming chunk directly as a ledger event
- WHEN `AppendEvent` executes with an unlisted event type (`internal/ledger/schema.go:38-39`)
- THEN SQLite MUST reject the insertion with a CHECK constraint error

### Requirement: Shell-free queries and status invariants

#### Scenario: Telemetry event history queried without shell out

- GIVEN completed lane lifecycle events recorded in SQLite (`internal/ledger/ledger.go:488-526`)
- WHEN `serve.Model` executes a telemetry query (`internal/serve/model.go:14-25`)
- THEN it MUST return populated DTOs from SQLite without invoking `os/exec` or git commands

#### Scenario: Batch barrier observes only persisted terminal status

- GIVEN parallel lanes executing with active telemetry streams (`internal/run/batch.go:29-65`)
- WHEN child processes exit and bounded stream flush completes
- THEN `barrier.Evaluate` MUST NOT release until every lane's terminal status is persisted in the ledger

#### Scenario: Unpersisted or non-terminal lane blocks barrier release

- GIVEN a batch with multiple lanes where one lane has finished execution but has not persisted its terminal status (`internal/barrier/barrier.go:36-47`)
- WHEN `barrier.Evaluate` is called
- THEN `Evaluate` MUST return `Outcome.Released = false` and prevent batch integration

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Worktree-local log teeing | covered | covered | covered | `internal/executor/agy_test.go:158-191`, `internal/run/run.go:368-375` |
| Loopback SSE stream | covered | covered | covered | `internal/serve/server_test.go:17-40`, `internal/serve/handlers.go:36-85` |
| Ledger isolation | covered | covered | covered | `internal/ledger/ledger_test.go:366-381`, `internal/run/run.go:71-100` |
| Shell-free queries and status invariants | covered | covered | covered | `internal/serve/model_test.go:595-627`, `internal/barrier/barrier_test.go:36-59` |

## Untestable Assertions

None

## Open Questions

- [ ] Whether worktree log files should be archived beside result envelopes (e.g. under `.lucind/logs/<run-id>/` or `.lucind/results/<lane-id>.log`) prior to worktree removal during cleanup (`proposal.md:154`).
- [ ] Whether the SSE event payload should stream raw chunk byte frames or multiplexed JSON envelopes containing lane ID, stream name, and timestamp (`proposal.md:155`).
