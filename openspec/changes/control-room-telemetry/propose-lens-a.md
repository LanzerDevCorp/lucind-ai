# Proposal Lens A — Candidate & Approach: Control Room Telemetry

## Selected Candidate & Approach

We select **Candidate 2: Worktree-Local Log Streaming with In-Memory Pub/Sub SSE Hub** from exploration (`openspec/changes/control-room-telemetry/explore.md:23-30`).

### Problem Summary & Existing Seams
`lucind-ai` executes parallel lanes across isolated git worktrees (`internal/worktree/worktree.go:179-238`, `internal/run/batch.go:81-89`). Currently, execution visibility is blocked by three architecture constraints:
1. **Synchronous In-Memory Buffering**: Agent executors (`agy`, `cursor-agent`, `opencode`) accumulate `stdout` and `stderr` into in-memory `bytes.Buffer` instances (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`), making output invisible during 20-minute dispatches (`cmd/lucind-ai/cli.go:42`) until child termination (`internal/executor/executor.go:42-63`).
2. **Strict SQLite Schema & Contention Hazards**: The `events` table enforces a closed `CHECK (type IN (...))` constraint admitting only six lifecycle event types (`internal/ledger/schema.go:34-43`). Funneling high-frequency streams into SQLite risks `SQLITE_BUSY` lock contention (`internal/ledger/ledger.go:127-129,162-164`) against concurrent batch dispatches (`internal/run/batch.go:66-113`) and feature lease renewals (`internal/run/run.go:203-211`, `internal/feature/feature.go:354-385`), violating the bounded diagnostic note policy (`internal/run/run.go:71-89`).
3. **Static Polling Server**: `lucind-ai serve` binds loopback (`internal/serve/server.go:16-23`, `cmd/lucind-ai/cli.go:674-725`) and provides only static polling for approvals (`internal/serve/handlers.go:36-85`), with no streaming transport.

### Core Technical Approach
1. **Real-Time Output Teeing**: In `internal/executor/*` and `internal/run/run.go:368-375`, replace `bytes.Buffer` with an `io.MultiWriter` that writes child process stdout and stderr concurrently to:
   - A worktree-local log file (e.g., `.lucind/stream.log` under `wt.Path`, `internal/run/run.go:311-316`).
   - An in-memory broadcast channel/hub in `internal/serve`.
2. **Process Pipe Lifecycle Integrity**: Preserve `cmd.WaitDelay` handling across executors (`internal/executor/agy.go:160-168`, `internal/executor/cursor_agent.go:82-90`, `internal/executor/opencode.go:121-129`). If inherited grandchild processes (such as agent MCP servers) keep stdio pipes open, map `exec.ErrWaitDelay` to `Outcome.OutputTruncated = true` with the real exit code from `cmd.ProcessState.ExitCode()` (`internal/executor/agy.go:182-197`, `internal/executor/cursor_agent.go:104-118`, `internal/executor/opencode.go:143-159`).
3. **Loopback Server-Sent Events (SSE)**: Expose a `/api/telemetry/events` SSE endpoint on the stdlib `http.ServeMux` in `internal/serve/handlers.go:36-85` using `http.Flusher`. Enforce strict loopback validation via `serve.IsLoopback` (`internal/serve/server.go:12-22,55-73`) and unregister channel subscribers on client disconnect.
4. **Preserved Ledger Invariants**: Discrete lifecycle events (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`) remain authoritative in SQLite via `AppendEvent` and `SetStatus` (`internal/ledger/ledger.go:366-381,448-485`). Failure diagnosis notes remain capped by `streamDetailCap` (4096 bytes per stream, `internal/run/run.go:71-89,422-435`). Integration check outputs continue auditing to `integration_events` (`internal/ledger/schema.go:171-180`, `internal/run/attempt.go:408-443`).

## Conceptual Changes & Architecture Rationale

### Conceptual Additions & Modifications
- **Stream Sink Abstraction**: `executor.Request` (`internal/executor/executor.go:14-37`) and `Executor.Run` (`internal/executor/executor.go:65-80`) transition from synchronous memory capture to streaming writer sinks (`io.Writer`).
- **Telemetry Tier Separation**: Establishes a dual-tier storage model:
  - *Tier 1 (High-frequency)*: Unbounded stdout/stderr chunks stream ephemerally through worktree log files and the in-memory SSE hub, incurring zero SQLite lock overhead.
  - *Tier 2 (Low-frequency)*: Canonical lane status (`internal/lane/status.go:8-28`), approval decisions (`internal/ledger/schema.go:45-56`), and capped diagnosis notes (`internal/run/run.go:71-89`) persist in SQLite.
- **Shell-Free Telemetry Queries**: Extends `serve.Model` (`internal/serve/model.go:14-25`) with read-only DTOs and queries for lane run events and execution durations backed directly by SQLite (`internal/ledger/ledger.go:488-526`), without spawning shell or git processes (`internal/serve/model_test.go:595-627`).

### Architecture Rationale
- **Zero Ledger Contention**: Isolating high-throughput agent logs from SQLite prevents `SQLITE_BUSY` lock starvation during concurrent lane execution (`internal/run/batch.go:66-113`) and 10-second lease renewal cycles (`internal/feature/feature.go:354-385`).
- **Standard Library Simplicity**: Leveraging stdlib `net/http` and `http.Flusher` (`internal/serve/server.go:16-53`) avoids heavy external dependencies (WebSockets, OTLP collectors) in `go.mod`.
- **Barrier Decoupling**: Stream flushes are bounded (<500ms) before lane completion, ensuring batch barrier synchronization (`internal/barrier/barrier.go:36-52`, `internal/run/batch.go:29-65`) depends solely on persisted terminal status (`internal/run/run.go:348-351,437-450`).

## Alternatives Considered & Rejected

### 1. SQLite-Centric Telemetry Table (`telemetry_events`)
- **Approach**: Insert streaming chunks and heartbeats into a dedicated SQLite table via schema migration (`internal/ledger/schema.go:224-306`), polled by sequence ID.
- **Rejection Rationale**: WAL-mode SQLite serializes writes. High-frequency writes from parallel lanes (`internal/run/batch.go:66-113`) cause write lock contention against lease renewals (`internal/feature/feature.go:354-385`, `internal/ledger/ledger.go:127-129`), while bloating database files against the small-ledger principle (`internal/run/run.go:71-78`).

### 2. Structured NDJSON Stream Parsing via IPC Socket Gateway
- **Approach**: Parse agent-specific CLI JSON formats (`--output-format json` in `internal/executor/agy.go:143`, `cursor_agent.go:70`, `opencode.go:108`) in real time and forward structured tool events over a local Unix domain socket.
- **Rejection Rationale**: Fragile against upstream CLI output schema drift and adds IPC process lifecycle complexity contrary to the single-user design (`docs/prd.md:48-57`). Live text streaming satisfies visibility needs without protocol coupling.

### 3. WebSocket / OTLP Sidecar Infrastructure
- **Approach**: Introduce bidirectional WebSockets or OpenTelemetry collector sidecars for telemetry ingest.
- **Rejection Rationale**: Introduces external dependencies and daemon overhead. Unidirectional SSE over stdlib HTTP satisfies all control room requirements cleanly within localhost security boundaries (`internal/serve/server.go:12-22`).

## Open Questions

- [ ] Should worktree-local telemetry log files be archived to `.lucind/results/<lane-id>.log` during `PersistEnvelope` (`cmd/lucind-ai/cli.go:647-660`) before `RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:641-646`) or `worktree cleanup` (`cmd/lucind-ai/cli.go:1460-1474`) deletes the worktree?
- [ ] Should the SSE telemetry payload in `internal/serve/handlers.go` be structured as raw stdout/stderr text chunks or as a multiplexed JSON envelope (containing lane ID, stream type, timestamp, and chunk payload)?
- [ ] For coarse progress tracking (such as turn index or elapsed duration), should milestone events be stored in SQLite via an additive migration v6 in `internal/ledger/schema.go:224-306`, or retained solely in-memory within the SSE broadcast hub?
