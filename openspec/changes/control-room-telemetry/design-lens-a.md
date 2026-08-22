# Design Lens A — Decisions: Control Room Telemetry

## Assumed architecture

We extend `internal/executor` (`Request`, `Agy`, `CursorAgent`, `Opencode`) with optional `io.Writer` sinks while preserving `Outcome` and `Executor.Run` signatures. We extend `internal/run` (`run.Execute`) to tee child stdio to a worktree-local log file and an in-memory broadcast hub without modifying SQLite `events` schema or the six-value `lane.Status` enum. In `internal/serve`, we introduce an in-memory SSE telemetry hub, add the `/api/telemetry/events` route to the existing loopback HTTP handler, and extend `serve.Model` with shell-free lifecycle event DTO queries backed by `Ledger.Events`.

## Technical Approach

We implement a dual-tier telemetry model mapping to the accepted proposal (`openspec/changes/control-room-telemetry/proposal.md:13-35`). Tier 1 handles high-frequency raw stdout/stderr by streaming child processes concurrently to worktree-local logs and an in-memory SSE hub, satisfying `lane-telemetry-streaming` and `approvals-web-ui` loopback streaming without touching SQLite. Tier 2 preserves coarse lifecycle audit in SQLite (`events` and `lanes`), bounding failure diagnostics to `streamDetailCap` (4096 bytes/stream) and enforcing `shell-free-telemetry-query` through `serve.Model` without `os/exec`. Invariants from `lane-execution` and `parent-feature-integration` remain intact: stream flushing completes before status persistence, batch barrier evaluation (`barrier.Evaluate`) requires persisted terminal statuses, and feature integration checks continue through `WriteWithAudit`.

## Decision 1 — Dual-Tier Telemetry Storage Architecture

**Choice**: Route high-frequency stdout/stderr streams to worktree-local log files (`<wt.Path>/.lucind/lane.log`) and an in-memory SSE hub in `internal/serve`. Store only coarse lifecycle milestones (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`) and capped failure notes in SQLite.
**Alternatives considered**: Ingesting high-frequency streaming chunks into a SQLite `telemetry_events` table; Unix domain socket / NDJSON gateway.
**Rationale**: Concurrent batch dispatches (`internal/run/batch.go:81-89`) share WAL-mode SQLite with `SetStatus` (`internal/run/run.go:348-351`) and feature lease renewals (`internal/feature/feature.go:354-385`). High-frequency writes cause `SQLITE_BUSY` contention and violate the small-ledger invariant (`internal/run/run.go:71-89`). File logs and in-memory SSE provide zero database lock contention and unbounded stream capture.
**Terminal consumer**: `run.Execute` in `internal/run/run.go:368-375` and `streamDetailCap` in `internal/run/run.go:89`.

## Decision 2 — Backward-Compatible Executor Sinks via Request

**Choice**: Maintain existing `Executor.Run(ctx context.Context, req Request) (Outcome, error)` signatures (`internal/executor/executor.go:67`) and `Outcome` fields (`internal/executor/executor.go:42-63`), adding optional `StdoutWriter` and `StderrWriter` `io.Writer` sinks to `executor.Request` (`internal/executor/executor.go:14-37`).
**Alternatives considered**: Changing `Executor.Run` to return streaming channels; removing `Outcome.Stdout`/`Outcome.Stderr` in favor of mandatory stream callbacks.
**Rationale**: Retaining `Executor.Run` and `Outcome` preserves compatibility with test stubs (`internal/run/run_test.go:45-60`) and batch dispatch (`internal/run/batch.go:86-88`). When sinks are present, executors tee using `io.MultiWriter` while preserving `cmd.WaitDelay` handling for grandchild processes (`internal/executor/agy.go:160-197`, `internal/executor/cursor_agent.go:82-118`, `internal/executor/opencode.go:121-160`).
**Terminal consumer**: `executor.Executor` in `internal/executor/executor.go:66-80` and `Agy.Run` in `internal/executor/agy.go:135-175`.

## Decision 3 — Stdlib SSE on Loopback Mux for Real-Time Streaming

**Choice**: Implement the SSE telemetry stream at `/api/telemetry/events` using stdlib `net/http` and `http.Flusher` on the existing `http.ServeMux` in `internal/serve/handlers.go:37-85`, enforced by loopback validation in `serve.IsLoopback` (`internal/serve/server.go:57-73`).
**Alternatives considered**: Third-party WebSocket packages; external telemetry sidecars (OTLP / OpenTelemetry collectors); polling-only `/api/telemetry` chunk endpoints.
**Rationale**: `go.mod` contains no WebSocket dependencies. Standard library SSE provides unidirectional, low-latency streaming to localhost browser clients with zero external dependencies, native HTTP connection lifecycle handling (`r.Context().Done()` for subscriber unregistration), and strict adherence to localhost-only security (`internal/serve/server.go:12-22`).
**Terminal consumer**: `serve.NewHandler` in `internal/serve/handlers.go:36-85` and `serve.ListenAndServe` in `internal/serve/server.go:19-23`.

## Decision 4 — Shell-Free Telemetry DTOs in serve.Model

**Choice**: Query lifecycle history via `serve.Model` methods (`internal/serve/model.go:17-24`) reading `Ledger.Events` (`internal/ledger/ledger.go:490-526`) directly from SQLite, returning typed DTO structs without executing shell commands or git subprocesses.
**Alternatives considered**: Invoking `git log` / `git diff` subprocesses via `os/exec`; adding duration columns to SQLite `events` schema.
**Rationale**: `serve.Model` enforces a strict shell-free architecture tested by `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`). Lifecycle events are already queryable in insertion order from `events` (`internal/ledger/schema.go:34-43`), and run duration is observed at the `exec.Run` call seam (`internal/run/run.go:368-375`).
**Terminal consumer**: `serve.Model` in `internal/serve/model.go:17-24` and `Ledger.Events` in `internal/ledger/ledger.go:490-526`.

## Decision 5 — Bounded Stream Flush and Barrier Release Decoupling

**Choice**: Bound stream flushing (<500ms) within `run.Execute` prior to status determination and ledger persistence (`internal/run/run.go:348-351,402-435`). Batch barrier evaluation (`internal/barrier/barrier.go:36-60`) continues to depend strictly on persisted terminal statuses (`internal/lane/status.go:21-28`) with no intermediate streaming status.
**Alternatives considered**: Introducing a `streaming` or `flushing` status to `lane.Status`; allowing the barrier to release before stream buffers flush.
**Rationale**: `lane.Status` is a fixed six-value enum checked by SQLite table constraints (`internal/ledger/schema.go:24-25`). Adding intermediate states complicates barrier logic. Flushing before status persistence guarantees log completeness while preserving the invariant that `barrier.Evaluate` releases only when all lanes reach durable terminal states (`internal/barrier/barrier.go:42-47`, `internal/run/batch.go:93-98`).
**Terminal consumer**: `barrier.Evaluate` in `internal/barrier/barrier.go:36-60` and `run.Execute` in `internal/run/run.go:348-351,402-435`.

## Decision 6 — Log Archiving on Worktree Removal

**Choice**: Write active logs to `<wt.Path>/.lucind/lane.log` during execution, and copy/archive the log to `.lucind/results/<lane-id>.log` adjacent to `PersistEnvelope` (`cmd/lucind-ai/cli.go:647-660`) when `RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:641-646`) is invoked.
**Alternatives considered**: Deleting worktree logs upon lane integration; writing all concurrent lane logs directly to a shared primary directory during execution.
**Rationale**: Worktrees operate in filesystem isolation (`internal/worktree/worktree.go:179-238`), matching where `writeResultSchema` writes (`internal/run/run.go:313-316`). Archiving beside `PersistEnvelope` preserves post-run diagnostics after worktrees are cleaned up without introducing write lock contention on the primary repository during execution.
**Terminal consumer**: `PersistEnvelope` and `RemoveLaneWorktree` in `cmd/lucind-ai/cli.go:641-660`.

## Open Questions

- [ ] Whether the SSE event payload format should be raw string chunks per stream or a structured JSON envelope (`{"lane_id": "...", "stream": "stdout", "chunk": "..."}`).
