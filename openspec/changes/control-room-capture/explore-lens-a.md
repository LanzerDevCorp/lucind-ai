# Explore Lens A — Problem & Candidates: Control Room Capture

## Problem Space

`lucind-ai` coordinates parallel execution across subscription-backed agent CLIs (`agy`, `cursor-agent`, `opencode`) using isolated git worktrees (`docs/prd.md:12-18,61-70`, `internal/worktree/worktree.go:150-171`). While the core dispatch loop, batch barrier, and feature ledger operate reliably, observability into in-flight and completed executions suffers from three architectural limitations:

1. **In-Flight Blindness During Dispatch**: `run.Execute` dispatches agent CLI subprocesses headlessly via `exec.CommandContext` with in-memory `bytes.Buffer` capture (`internal/executor/agy.go:133-138`, `internal/executor/cursor_agent.go:102-106`, `internal/executor/opencode.go:117-121`). During long-running dispatches (up to `defaultTimeout = 20 * time.Minute`, `cmd/lucind-ai/cli.go:42`), the operator has no live streaming visibility into agent terminal output, progress, reasoning, or hang states.
2. **Terminal Stream Discard on Success**: When a lane finishes with `lane.Done` (`internal/run/run.go:408-415`), its captured `outcome.Stdout` and `outcome.Stderr` in `executor.Outcome` (`internal/executor/executor.go:42-63`) are discarded entirely from memory. On non-terminal failure or timeout, only a clipped tail bounded by `streamDetailCap = 4096` bytes per stream (`internal/run/run.go:89,103-120`) is recorded as an `EventLaneNote` in SQLite table `events` (`internal/ledger/schema.go:34-43`, `internal/run/run.go:423-435`). No persistent full transcript or execution stream survives for post-hoc debugging, auditing, or prompt verification.
3. **Disconnected Localhost Control Room UI**: The `lucind-ai serve` command (`cmd/lucind-ai/cli.go:112-113`, `internal/serve/server.go:19-53`) serves a loopback HTTP UI (`internal/serve/handlers.go:36-118`). However, `ServerState` (`internal/serve/handlers.go:15-21`) only exposes pending human approvals (`ledger.Approval`, `internal/ledger/schema.go:45-56`). The rich query surface implemented in `internal/serve/model.go:14-126` (features, attempts, leases, overlap evidence, reconciliation requests, candidates, and audit events) remains unrouted to HTTP endpoints, and there is zero support for viewing active runs, lane execution progress, or streaming logs.

The goal of `control-room-capture` is to establish real-time stream capture during dispatch, persist complete execution logs durably without bloating the SQLite database, and wire live stream and run inspection into the `lucind-ai serve` Control Room interface.

## Candidate Approaches

### Candidate 1 — File-Backed Stream Spooling with MultiWriter Tee & SSE Control Room Streaming

**Approach**: Modify `executor.Request` (`internal/executor/executor.go:14-37`) to accept an output stream or target directory. When `run.Execute` (`internal/run/run.go:368-375`) starts a lane, it creates dedicated log files under `.lucind/runs/<run-id>/<lane-id>.{stdout,stderr}.log` and uses `io.MultiWriter` in `internal/executor/agy.go:133-138` (and sibling executors) to spool stdout/stderr concurrently to disk while buffering in memory for `executor.Outcome`. `internal/serve/handlers.go:36-118` adds HTTP endpoints for static log download (`/api/runs/{runID}/lanes/{laneID}/logs`) and Server-Sent Events (`/api/runs/{runID}/lanes/{laneID}/stream`) that tail the active log file using standard `http.Flusher`.
**Pros**: Zero SQLite database bloat from large CLI output streams; crash-resilient as OS flushes log files incrementally; stdlib-only implementation (`os.File`, `io.MultiWriter`, `net/http`); clean decoupled architecture where `lucind-ai run` and `lucind-ai serve` interact through the filesystem and SQLite ledger.
**Cons**: Requires managing log file paths and lifecycle retention in `.lucind/`; log tailing requires handling file truncations or rotations if lanes restart.
**Feasibility**: High. Leverages standard Go library primitives, directly fits existing `internal/executor/` and `internal/serve/` packages without external dependencies or schema churn.

### Candidate 2 — SQLite Chunked Event Stream Storage with Long-Polling

**Approach**: Add an additive `lane_stream_chunks` table to SQLite schema (`internal/ledger/schema.go:10-57`) storing sequential chunks (e.g. 16KB text blocks) with timestamps and sequence numbers. A custom chunking writer in `internal/run/run.go:368-375` flushes subprocess output to `ledger.Ledger` (`internal/ledger/ledger.go:180-210`). `internal/serve/handlers.go:36-118` provides a chunk query API (`/api/runs/{runID}/lanes/{laneID}/chunks?since=<seq>`) polled by `internal/serve/static/app.js`.
**Pros**: Single atomic store for all runs, events, metadata, and logs; no orphan log files on disk; automatic cleanup via SQLite foreign key cascading or transaction rollback.
**Cons**: High write amplification and database file growth from multi-megabyte agent streams; potential SQLite lock contention with barrier state updates (`internal/run/run.go:425-434`, `internal/run/batch.go:159-209`) under `_busy_timeout=5000` (`internal/ledger/ledger.go:67-75`).
**Feasibility**: Medium. Requires additive schema v6 and careful chunking tuning to prevent database lock degradation during parallel 4-lane dispatches (`internal/run/batch.go:66-147`).

### Candidate 3 — Persistent Daemon Process with In-Memory Ring-Buffers & WebSockets

**Approach**: Make `lucind-ai serve` (`internal/serve/server.go:19-53`) a required persistent background daemon. `lucind-ai run` connects to the daemon over a local Unix domain socket, streaming execution events and stdio into in-memory ring-buffers. The UI connects over WebSockets (`/ws`) for bidirectional terminal streaming and interactive lane cancellation.
**Pros**: Sub-millisecond streaming latency; memory usage bounded by ring-buffer size; supports interactive terminal input if interactive lanes are added.
**Cons**: Breaks the standalone, daemonless CLI architecture (`cmd/lucind-ai/cli.go:99-127`, `docs/prd.md:188-193`); introduces Unix socket IPC complexity; requires non-stdlib WebSocket dependencies violating project constraints (`openspec/changes/archive/2026-08-20-approvals-web-ui/proposal.md:25`, `docs/prd.md:50`).
**Feasibility**: Low. Directly conflicts with the single-binary standalone execution design and stdlib-only constraints.

### Candidate 4 — Hybrid File Spooling with Ledger Milestones & Model Query Routing

**Approach**: Subprocesses stream stdout/stderr directly to file-backed logs under `.lucind/runs/<run-id>/lanes/<lane-id>.log` via `io.MultiWriter` (`internal/executor/executor.go:14-37`, `internal/executor/agy.go:133-138`). `internal/run/run.go:368-450` logs discrete lifecycle milestone events (dispatch start, heartbeat, envelope parse, approval wait, completion) to `internal/ledger/schema.go:34-43` `events`. `internal/serve/handlers.go:36-118` exposes SSE log tailing alongside JSON endpoints backed by `internal/serve/model.go:128-343` (`ListFeatures`, `GetFeature`, `ListAttempts`, `ListReconciliationRequests`, `ListAuditEvents`), turning `lucind-ai serve` into a comprehensive Control Room dashboard.
**Pros**: Combines file-backed log efficiency with atomic ledger queryability; activates existing `internal/serve/model.go` queries; zero non-stdlib dependencies; works whether `serve` runs before, during, or after `run`.
**Cons**: Requires updating archive routines (`plugin/claude-code/skills/lucind-ai/SKILL.md:280-285`) to preserve or prune run logs during change archival.
**Feasibility**: High. Directly unifies `internal/run`, `internal/executor`, `internal/ledger`, and `internal/serve` using existing codebase structures.

## Initial Recommendations

Candidate 4 (Hybrid File Spooling with Ledger Milestones & Model Query Routing) is recommended:
1. **Separation of Concerns**: Storing heavy text streams on the filesystem prevents SQLite bloat and lock contention while structured milestones in `internal/ledger/schema.go:34-43` maintain queryable run history.
2. **Reuses Existing Architecture**: Activates `internal/serve/model.go:14-126` queries that are already implemented and tested, expanding `lucind-ai serve` from a simple approval gate to a full Control Room.
3. **Stdlib-Only & Daemonless**: Preserves standalone dispatch in `cmd/lucind-ai/cli.go:99-127` and loopback HTTP in `internal/serve/server.go:19-53` without external runtime dependencies.

## Open Questions

- [ ] Should log files under `.lucind/runs/<run-id>/lanes/<lane-id>.log` be archived into `openspec/changes/<change-id>/logs/` upon phase completion, or retained only in `.lucind/` with a bounded retention policy?
- [ ] Should `internal/serve` stream combined stdout/stderr as a single interleaved stream or provide separate selectable tabs/endpoints for stdout and stderr?
