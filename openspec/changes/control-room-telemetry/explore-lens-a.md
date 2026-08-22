# Explore Lens A — Problem & Candidates: Control Room Telemetry

## Problem Space

`lucind-ai` orchestrates multi-agent coding sessions headlessly across isolated git worktrees (`internal/worktree/worktree.go:179-238`, `internal/run/batch.go:81-89`). Currently, active lane execution operates as a telemetry black box:

1. **Buffered I/O in Headless Executors**: All child CLI executors (`agy`, `cursor-agent`, `opencode`) capture child `stdout` and `stderr` synchronously into in-memory `bytes.Buffer` instances (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:129-135`). During dispatches running up to default lane timeouts of 20 minutes (`cmd/lucind-ai/cli.go:42`), no live execution feedback, tool call activity, or incremental output is streamed or persisted until process termination (`internal/executor/executor.go:42-63`).
2. **Strict SQLite Schema Constraints & Lock Hazards**: The ledger's `events` table enforces a strict SQLite `CHECK` constraint admitting only specific lifecycle event types (`internal/ledger/schema.go:34-43`). Attempting to funnel high-frequency token streams or raw stdout chunks into the SQLite ledger risks lock contention during concurrent batch dispatches (`internal/run/batch.go:66-113`) and feature lease renewals (`internal/run/run.go:203-211`), and violates the storage bounded design that caps diagnosis details to 4096 bytes (`internal/run/run.go:89-102`).
3. **Static Polling in Serve Query Surface**: The localhost server query surface (`internal/serve/model.go:14-25`, `internal/serve/handlers.go:36-118`, `cmd/lucind-ai/cli.go:674-725`) only serves static JSON polling for approvals and ledger rows via `/api/state` (`internal/serve/handlers.go:79-85, 120-146`). It provides no Server-Sent Events (SSE), WebSockets, or live stream endpoints for operators monitoring execution in the control room.

## Candidate Approaches

### Candidate 1 — SQLite-Centric High-Frequency Telemetry Table

**Approach**: Add a dedicated `telemetry_events` table to the SQLite schema via a new migration (`internal/ledger/schema.go:224-306`). Executors flush batched stdout/stderr chunks and heartbeats into the ledger using new `AppendTelemetry` methods (`internal/ledger/ledger.go:366-381`), which `internal/serve/handlers.go` polls by sequence ID.
**Pros**: Unified query interface across all ledger data; durable storage of complete telemetry history surviving daemon restarts.
**Cons**: Severe SQLite write lock contention among parallel lanes (`internal/run/batch.go:81-89`) and lease renewals (`internal/run/run.go:203-211`); unbounded SQLite database growth violating the small-ledger principle (`internal/run/run.go:71-78`).
**Feasibility**: High implementation feasibility using existing `modernc.org/sqlite` bindings (`internal/ledger/ledger.go:30-34`), but introduces high operational risk for lane concurrency.

### Candidate 2 — Worktree-Local Log Streaming with In-Memory Pub/Sub SSE Hub

**Approach**: Replace in-memory `bytes.Buffer` in executors with `io.MultiWriter` targeting worktree-local log files (e.g. `.lucind/stream.log` inside `wt.Path`, `internal/run/run.go:311-316`) and an in-memory Pub/Sub event hub in `internal/serve/`. Discrete status transitions stay in SQLite (`internal/ledger/schema.go:38-39`), while raw log streaming and live progress broadcast directly to web clients via HTTP Server-Sent Events (`internal/serve/handlers.go:36-85`).
**Pros**: Zero database lock contention; log lifecycles naturally align with worktree preservation or cleanup (`internal/run/run.go:641-646`); supports standard Unix file tailing (`tail -f`) directly in worktrees; zero performance overhead on core ledger transactions.
**Cons**: Live log streams for integrated worktrees become unavailable after worktree deletion unless explicitly archived during `PersistEnvelope` (`internal/run/run.go:647-660`); in-memory pub/sub connections do not persist across server restarts.
**Feasibility**: Very high. Fits cleanly into `internal/executor/executor.go:65-80`, `internal/run/run.go:368-375`, and `internal/serve/server.go:1-55`.

### Candidate 3 — Structured NDJSON Stream Parsing via IPC Socket Gateway

**Approach**: Stream agent CLI stdout through specialized real-time JSON parsers tailored to each executor's structured output format (`--output-format json` in `internal/executor/agy.go:143`, `internal/executor/cursor_agent.go:70`, and `opencode.go:108`). Parsed events (tool calls, token metrics) are sent across a local Unix domain socket IPC gateway to a standalone aggregator service.
**Pros**: Normalized structured telemetry (tool execution times, token costs, diff hunks) across divergent agent CLI protocols.
**Cons**: High fragility against upstream CLI schema mutations across `agy`, `cursor-agent`, and `opencode`; substantial architectural complexity violating the single-user subscription constraint (`docs/prd.md:48-57`).
**Feasibility**: Moderate-low. Requires complex parser maintenance and custom IPC socket lifecycle management.

## Initial Recommendations

Candidate 2 (Worktree-Local Log Streaming with In-Memory Pub/Sub SSE Hub) is the recommended path. It preserves the clean separation of concerns in `lucind-ai`: SQLite manages authoritative, low-frequency state transitions (`internal/ledger/schema.go:34-43`, `internal/ledger/ledger.go:448-485`), while worktree files and lightweight in-memory channels handle high-throughput I/O streaming without SQLite lock contention.

## Open Questions

- [ ] Should completed lane stream logs be retained into `.lucind/results/<lane-id>.log` during `PersistEnvelope` (`internal/run/run.go:647-660`) before lane worktrees are removed?
- [ ] Should the SSE endpoint in `internal/serve/handlers.go` stream raw stdout text lines, structured lifecycle events, or a multiplexed JSON envelope?
