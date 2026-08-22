# Explore: Control Room Telemetry

Recommended path: **worktree-local log files plus an in-memory SSE hub**. SQLite keeps coarse lifecycle transitions. High-frequency stdout/stderr does not go through the ledger.

## Problem

`lucind-ai` runs parallel lanes in isolated git worktrees (`internal/worktree/worktree.go:179-238`, `internal/run/batch.go:81-89`). During a dispatch, operators have no live view of process liveness, tool activity, or incremental output.

1. **Buffered child I/O.** `agy`, `cursor-agent`, and `opencode` assign `stdout`/`stderr` to in-memory `bytes.Buffer` and call `cmd.Run()` (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-132`). Capture is available only after the child exits, as `executor.Outcome` (`internal/executor/executor.go:42-63`). Default lane timeout is 20 minutes (`cmd/lucind-ai/cli.go:42`). The CLI prints a report only after the lane ends (`cmd/lucind-ai/cli.go:512-540`); the barrier waits for every lane to be terminal (`internal/barrier/barrier.go:34-52`).
2. **Ledger is a small, closed event log.** `events.type` admits only `run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended` (`internal/ledger/schema.go:34-43`). Diagnosis notes are capped at 4096 bytes per stream so the ledger does not become a log store (`internal/run/run.go:71-89`). Concurrent batch writes (`internal/run/batch.go:66-113`) already share SQLite with status transitions (`internal/run/run.go:348-350`) and feature lease renewals (`internal/run/run.go:203-211`, `internal/feature/feature.go:354-385`) on WAL with `busy_timeout=5000` (`internal/ledger/ledger.go:127-129,162-164`).
3. **Serve is static polling.** `lucind-ai serve` binds loopback (`cmd/lucind-ai/cli.go:674-725`, `internal/serve/server.go:12-22`). `/api/state` returns approver identity, rate, and pending approvals (`internal/serve/handlers.go:15-22,79-85,120-146`). The UI polls every 2s (`internal/serve/static/app.js:1-9,97`). There is no SSE, WebSocket, or live stream route. `serve.Model` is a shell-free SQLite DTO layer for feature/attempt/reconciliation state (`internal/serve/model.go:14-25`), not run telemetry.

## Candidate approaches

### 1. SQLite-centric `telemetry_events` table

Add a migration beside `migrate` (`internal/ledger/schema.go:224-306`) and a write path modeled on `AppendEvent` (`internal/ledger/ledger.go:366-381`). Serve polls by sequence.

- **Pros:** One query surface; history survives daemon restart.
- **Cons:** Write-lock contention with parallel lanes and lease renewals; unbounded growth against the small-ledger rule (`internal/run/run.go:71-78`). New `events.type` values fail the existing CHECK until a schema change (`internal/ledger/schema.go:38-39`).
- **Feasibility:** High to implement via `modernc.org/sqlite` (`internal/ledger/ledger.go:30-34`). High operational risk. **Unviable for high-frequency streams** given WAL serialization and lease-renewal writes.

### 2. Worktree-local logs + in-memory SSE hub (recommended)

Replace `bytes.Buffer` with `io.MultiWriter` teeing into a file under `wt.Path` (Execute already writes `.lucind/` there, `internal/run/run.go:311-316`) and an in-memory hub in `internal/serve`. Discrete status stays in SQLite via `SetStatus` (`internal/ledger/ledger.go:448-485`). Serve grows an SSE route on the existing mux (`internal/serve/handlers.go:36-85`) using stdlib `net/http` (`internal/serve/server.go:16-53`). Fits `Executor.Run` (`internal/executor/executor.go:65-80`, `internal/run/run.go:368-375`).

- **Pros:** No extra ledger write load; logs live with the worktree; `tail -f` works; Unix files, no new query language.
- **Cons:** Logs vanish when the worktree is removed (`RemoveLaneWorktree` at `cmd/lucind-ai/cli.go:641-646`, `worktree cleanup` at `cmd/lucind-ai/cli.go:1460-1474`) unless archived. `PersistEnvelope` today writes `.lucind/results/<laneID>.json` only (`cmd/lucind-ai/cli.go:647-660`). In-memory subscribers die with the serve process.
- **Feasibility:** Very high.

### 3. Per-executor NDJSON parsers + Unix-socket aggregator

All three CLIs already request JSON (`--output-format json` in `internal/executor/agy.go:143`, `internal/executor/cursor_agent.go:70`; `--format json` in `internal/executor/opencode.go:108`). Parse tool calls and metrics, forward over a local socket.

- **Pros:** Normalized structured events across CLIs.
- **Cons:** Fragile against upstream schema churn; extra process/IPC complexity vs the single-user constraint (`docs/prd.md:48-57`).
- **Feasibility:** Moderate-low. Not required to give operators a live tail.

## User and capability impact

**Operator / dispatcher** running `lucind-ai run` batches (`cmd/lucind-ai/cli.go:95-127,261-311`) or feature recover/renew (`cmd/lucind-ai/cli.go:739-744`) currently waits until printReport and barrier release. Live heartbeats, progress, and stall warnings before the 20-minute ceiling are new.

**Control Room serve** extends beyond approval listings (`internal/serve/handlers.go:15-22,79-86`) without weakening loopback (`internal/serve/server.go:16-53`) or per-lane decide invariants (`internal/serve/handlers.go:148-211`). `serve.Model` stays shell-free (`internal/serve/model_test.go:595-627`).

**Executors and run** emit a live stream from the `Run` adapters (`internal/executor/agy.go:67-82`, `internal/executor/cursor_agent.go:49-65`, `internal/executor/opencode.go:33-48`). Coarse milestones may still use `AppendEvent` / `WriteWithAudit` (`internal/ledger/ledger.go:360-381,832-873`) only if they stay inside admitted types or land behind an additive migration. `integration_events` already has unconstrained `type` (`internal/ledger/schema.go:171-180`).

New capabilities: live lane tail and health; optional post-run token/duration parse from captured JSON stdout; integration-phase audit already written through `WriteWithAudit`; chronological replay of existing ledger events via `Ledger.Events` (`internal/ledger/ledger.go:488-526`). `serve.Model` has no `ListRunEvents` / `GetLaneTelemetry` today.

## Scenarios

1. **Live batch progress.** Two parallel lanes (`internal/run/batch.go:19-68`). Executors tee output to a worktree log and SSE. The Control Room shows turns/elapsed instead of a silent pending row.
2. **Duration and optional token parse.** After `CursorAgent.Run`, `executor.Outcome` holds `ExitCode`, `TimedOut`, `Stdout`, `Stderr` (`internal/executor/executor.go:42-63`) — not `inputTokens`. Duration is observable at the `exec.Run` seam (`internal/run/run.go:368-375`). Token fields, if wanted, must be parsed from JSON stdout; they are not in the Go result type.
3. **Integration checks.** `ExecuteAttempt` is `recorded → leased → combining → checking → cas_pending → promoted` (`internal/run/attempt.go:213-214`). In `checking`, `integrate.Check` runs `lucind-checks.sh` via `CombinedOutput()` (`internal/integrate/integrate.go:90-109`) and audits via `WriteWithAudit` (`internal/run/attempt.go:408-443`). Output is not incrementally streamed today; live check tails would use the same file/SSE path as lane logs. `lucind-ai check` is the CLI wrapper (`cmd/lucind-ai/cli.go:409-478`).
4. **Stall / quota.** PRD: a dry subscription blocks that lane; a thrashing executor burns until the wall clock (`docs/prd.md:143-166`). Heartbeat absence is new telemetry. Diagnosis text today appears only after the lane is terminal (`cmd/lucind-ai/cli.go:523-539`).
5. **Post-mortem.** Mixed `done`/`blocked`/`deviated`/`failed` (`internal/lane/status.go:11-30`, `cmd/lucind-ai/cli.go:361-369`). `Ledger.Events` returns insertion-ordered lifecycle rows. `lane_progress` is not an admitted type.

## Risks and trade-offs

| Risk | Sev | Mitigation | Seam |
|---|---|---|---|
| SQLite `SQLITE_BUSY` / lease starvation if streams hit the ledger | High | Files + SSE; SQLite for coarse events only | `internal/ledger/ledger.go:127-129,162-164`, `internal/feature/feature.go:354-385` |
| New event types fail `events.type` CHECK | High | Migration v6, or keep high-frequency data off `events` | `internal/ledger/schema.go:34-43` |
| Async pipes + grandchild MCP servers hang `Wait` | High | Keep `cmd.WaitDelay`; close pipes on cancel; `ProcessState.ExitCode` on `ErrWaitDelay` | `internal/executor/agy.go:160-168,182-197`, `internal/executor/cursor_agent.go:82-90,104-118`, `internal/executor/opencode.go:121-129,143-159` |
| Unbounded RAM from `bytes.Buffer` | Med | `MultiWriter` to file + bounded ring; keep 4096-byte diagnosis tails | `internal/run/run.go:71-89` |
| SSE on a non-loopback bind | Med | `IsLoopback` on listen; no new auth | `internal/serve/server.go:12-22,55-73` |
| Stream flush delaying barrier Observe | Low | Bounded flush (under 500ms); Observe already waits for terminal status, including approval (`internal/run/run.go:437-450`) | `internal/run/batch.go:29-65` |

| Choice | For | Against | Cost |
|---|---|---|---|
| Worktree files + SSE vs SQLite ingest | No lock contention; OS cache | Lost after worktree delete without archive | Low |
| SSE vs WebSockets | Stdlib `net/http`; browser reconnect | Unidirectional | Low vs new WS dependency (`go.mod` has no websocket lib; `internal/serve` is stdlib HTTP) |
| Stdio intercept vs OTLP | Works with current CLIs | Parsing JSON/text | Low vs sidecars |

## Spikes

1. **Pipe tee + WaitDelay.** Replace `bytes.Buffer` with `MultiWriter` in `agy`/`opencode`. Prove grandchild-open-pipe behavior still maps to `OutputTruncated` (existing coverage starts at `internal/executor/agy_test.go:158-174`).
2. **WAL contention.** Benchmark parallel `AppendEvent` vs `RenewLease` to confirm isolating raw logs from SQLite.
3. **SSE hub.** Prototype `http.Flusher` on the serve mux (`internal/serve/handlers.go:36-85`); test disconnect and goroutine cleanup. Current `server_test.go` covers bulk-approval 400s, not streams.

## Success criteria

- [ ] Every dispatched lane tees live stdout/stderr to a worktree log (and SSE while serve is up) from `internal/run` / `Executor.Run`.
- [ ] Exit code, timeout, duration, and (if parsed) JSON token fields are queryable without `os/exec` in `internal/serve/model.go` (`internal/serve/model_test.go:595-627`).
- [ ] New routes stay loopback-only (`internal/serve/server.go:16-53`) and do not change approval decide rules (`internal/serve/handlers.go:148-211`).
- [ ] Feature attempts still record phase transitions into `integration_events` (`internal/ledger/schema.go:171-180`, `internal/ledger/ledger.go:832-873`).
- [ ] High-frequency chunks never depend on extending `events.type` without an explicit migration.

## Out of scope

- Patching `agy` / `cursor-agent` / `opencode` or forcing OTLP.
- Non-stdlib frontend bundlers or WebSocket libraries in `internal/serve`.
- Remote bind, tokens, multi-tenant auth (`internal/serve/server.go:12-22`).
- Changing the six-value `lane.Status` (`internal/lane/status.go:10-17`).
- UI layout and serve topology owned by sibling Control Room changes.

## Open questions

- Archive worktree logs before removal? If yes: beside `PersistEnvelope` as `.lucind/results/<lane-id>.log`, or under `.lucind/logs/<run_id>/` before `worktree cleanup`?
- SSE payload: raw lines, structured lifecycle, or a multiplexed JSON envelope?
- Sample child CPU/RSS, or only agent events, tokens, and phase durations?
- Keep 2s `/api/state` polling for approvals and add SSE only for logs, or move live state to SSE?
- For coarse milestones only: dedicated `telemetry_events` (v6) vs extending `events.type` CHECK?

## Affected areas

`internal/executor/*`, `internal/run/run.go`, `internal/serve/handlers.go` + `server.go` + `static/`, optionally `internal/ledger/schema.go` (v6), `cmd/lucind-ai/cli.go` (archive + serve wiring).

## Ready for proposal

Yes. Proposal should lock Candidate 2, the archive path, SSE envelope shape, and whether coarse milestones need schema v6.
