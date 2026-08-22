# Proposal Lens B — Capability Impact & Specs: Control Room Telemetry

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `lane-telemetry-streaming` | Added | Tees child process stdout/stderr in real time to worktree-local log files and an in-memory broadcast channel during executor dispatch instead of unbounded memory buffering. | `internal/run/run.go:368-375`, `internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136` |
| `approvals-web-ui` | Modified | Adds a loopback-only Server-Sent Events (SSE) stream endpoint to `internal/serve` for live execution monitoring without weakening localhost binding or individual approval decisions. | `internal/serve/server.go:16-23,57-73`, `internal/serve/handlers.go:36-85,148-211`, `openspec/specs/approvals-web-ui/spec.md:10-25` |
| `shell-free-telemetry-query` | Added | Introduces read-only DTOs and query methods on `serve.Model` for run events, execution durations, and exit codes without shelling out or invoking `os/exec`. | `internal/serve/model.go:14-25`, `internal/ledger/ledger.go:488-526`, `internal/serve/model_test.go:595-627` |
| `lane-execution` | Modified | Integrates live output flushing into lane dispatch while keeping the six-value `lane.Status` enum unchanged and ensuring bounded stream teardown prior to barrier observation. | `internal/lane/status.go:8-28`, `internal/run/run.go:348-351`, `internal/barrier/barrier.go:36-47`, `openspec/specs/lane-execution/spec.md:10-48` |
| `parent-feature-integration` | Modified | Preserves phase duration and check output recording in `integration_events` via `WriteWithAudit` during attempt execution without high-frequency SQLite write contention. | `internal/run/attempt.go:213-214,408-443`, `internal/integrate/integrate.go:90-109`, `internal/ledger/ledger.go:832-873`, `openspec/specs/parent-feature-integration/spec.md:33-45` |

## Delta Specifications

### Requirement: Worktree-Local Log Teeing and Non-Blocking Output Capture

The executor dispatch loop MUST stream child process stdout and stderr concurrently to a worktree-local log file under the active worktree path and an in-memory broadcast channel using an `io.MultiWriter` instead of buffering process output solely in memory (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`, `internal/run/run.go:368-375`). The executor MUST honor `cmd.WaitDelay` and populate `Outcome.OutputTruncated` when stdio pipes remain open past process exit, preserving the true observed exit code (`internal/executor/agy.go:160-197`, `internal/executor/cursor_agent.go:82-118`, `internal/executor/opencode.go:121-159`).

#### Scenario: Live output streaming during lane execution

- GIVEN a running lane executing in an isolated worktree (`internal/run/run.go:368-375`)
- WHEN the child process writes output to stdout or stderr (`internal/executor/agy.go:169-175`)
- THEN the output MUST be written immediately to the worktree log file and broadcast to streaming subscribers without blocking child process execution.

#### Scenario: Grandchild process keeps stdio pipe open

- GIVEN a child process that has exited with code 0 while an inherited grandchild process keeps stdio pipes open (`internal/executor/agy.go:182-197`)
- WHEN `cmd.WaitDelay` elapses (`internal/executor/agy.go:160-168`)
- THEN the executor MUST return `Outcome.OutputTruncated = true` with exit code 0 and MUST NOT hang execution indefinitely.

### Requirement: Loopback Server-Sent Events Telemetry Stream

The HTTP server in `internal/serve` MUST expose a Server-Sent Events (SSE) endpoint (e.g., `/api/telemetry/events`) implemented via stdlib `net/http` and `http.Flusher` (`internal/serve/handlers.go:36-85`). The endpoint MUST enforce loopback binding through `serve.IsLoopback` and MUST reject non-loopback requests or remote network interfaces (`internal/serve/server.go:16-23,57-73`, `openspec/specs/approvals-web-ui/spec.md:10-25`). Telemetry streaming MUST NOT modify or bypass individual per-item approval controls (`internal/serve/handlers.go:148-211`, `openspec/specs/approvals-web-ui/spec.md:26-48`).

#### Scenario: Loopback client subscribes to SSE stream

- GIVEN `lucind-ai serve` listening on `127.0.0.1:7433` (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:16-23`)
- WHEN an HTTP client requests the SSE telemetry endpoint over loopback (`internal/serve/handlers.go:36-85`)
- THEN the server MUST return HTTP 200 with `Content-Type: text/event-stream` and flush live lane output events as they occur.

#### Scenario: SSE client disconnect cleans up channel subscriber

- GIVEN an active SSE connection receiving lane events (`internal/serve/handlers.go:36-85`)
- WHEN the client closes the connection (`internal/serve/server.go:41-53`)
- THEN the server MUST terminate the stream handler goroutine and unregister the subscriber channel without resource leaks.

### Requirement: Ledger Isolation from High-Frequency Output

High-frequency stdout and stderr chunks MUST NOT be inserted into the SQLite `events` table (`internal/ledger/schema.go:34-43`, `internal/ledger/ledger.go:366-381`). The SQLite ledger MUST record only coarse lifecycle transitions (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`) via `SetStatus` and `AppendEvent` (`internal/ledger/ledger.go:360-381,448-485`). Diagnostic details in `lane_note` events MUST remain bounded by `streamDetailCap` (4096 bytes per stream) to prevent database lock contention and database bloat (`internal/run/run.go:71-89,422-435`).

#### Scenario: High-volume output preserves ledger write bounds

- GIVEN a dispatched lane generating multi-megabyte stdout output (`internal/run/run.go:368-375`)
- WHEN the child process runs and completes (`internal/executor/executor.go:42-63`)
- THEN the raw stdout stream MUST be stored in the worktree log file while the SQLite `events` table contains only coarse lifecycle rows (`internal/ledger/ledger.go:366-381,448-485`).

#### Scenario: Terminal failure records capped diagnostic note

- GIVEN a lane terminating with a non-zero exit code and captured stderr output exceeding 4096 bytes (`internal/run/run.go:422-435`)
- WHEN `run.Execute` records the failure note (`internal/run/run.go:424-435`)
- THEN the stored `lane_note` detail MUST be truncated to 4096 bytes per stream with `streamTruncatedMarker` appended (`internal/run/run.go:71-100`).

### Requirement: Shell-Free Telemetry Queries and Status Invariants

`serve.Model` MUST provide read-only query methods for run lifecycle events and execution durations backed directly by SQLite queries without executing shell commands or importing `os/exec` (`internal/serve/model.go:14-25`, `internal/serve/model_test.go:595-627`). Telemetry streaming MUST NOT modify the six-value `lane.Status` enum (`internal/lane/status.go:8-28`) and MUST NOT delay batch barrier release beyond bounded stream flushing (`internal/barrier/barrier.go:36-47`, `internal/run/batch.go:29-65`).

#### Scenario: Shell-free telemetry query execution

- GIVEN completed lane executions recorded in the ledger (`internal/ledger/ledger.go:488-526`)
- WHEN `serve.Model` executes a telemetry query (`internal/serve/model.go:14-25`)
- THEN the query MUST return structured DTOs populated directly from SQLite without invoking external processes or git commands (`internal/serve/model_test.go:595-627`).

#### Scenario: Barrier releases only after terminal persistence

- GIVEN parallel executing lanes with active telemetry streams (`internal/run/batch.go:19-68`)
- WHEN a lane finishes child process execution and flushes its stream (`internal/run/run.go:368-375`)
- THEN the batch barrier MUST NOT release until every lane's terminal status is persisted in the ledger (`internal/barrier/barrier.go:36-47`, `internal/run/run.go:348-351`).

## Open Questions

- [ ] Should completed worktree stream logs be archived to `.lucind/results/<lane-id>.log` during `PersistEnvelope` (`cmd/lucind-ai/cli.go:647-660`) before lane worktree cleanup (`cmd/lucind-ai/cli.go:641-646,1460-1474`)?
- [ ] Should the SSE stream payload use raw stdout/stderr text chunks or a multiplexed JSON envelope containing lane ID, timestamp, stream type, and payload data?
- [ ] Should coarse progress milestones (turn index, elapsed seconds) be stored in a dedicated `telemetry_events` table via SQLite migration v6 or kept in-memory within the SSE hub?
