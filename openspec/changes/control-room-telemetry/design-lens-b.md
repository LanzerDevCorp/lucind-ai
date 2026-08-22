# Design Lens B — Surface & Flow: Control Room Telemetry

## Assumed architecture

We assume Candidate 2: worktree-local log files and an in-memory loopback Server-Sent Events (SSE) broadcast hub. `internal/executor.Request` gains optional `io.Writer` sinks while `Outcome` and `Executor.Run` signatures remain unchanged for `Agy`, `CursorAgent`, and `Opencode`. `internal/run.Execute` tees child streams to `<wt.Path>/.lucind/lane.log` and the in-memory hub, leaving SQLite schema (v5) and `lane.Status` unchanged. `internal/serve` adds `/api/telemetry/events` on loopback and shell-free DTO queries in `serve.Model` backed by `Ledger.Events`.

## Flow and Invariants

```
Child Process (CLI)
       │ (stdout/stderr)
       ▼
Executor MultiWriter ──┬──→ Worktree Log (<wt.Path>/.lucind/lane.log)
(internal/executor)   └──→ In-Memory Hub (internal/serve/hub.go)
                                    │
                                    ├──→ Loopback SSE (/api/telemetry/events)
                                    ▼
                              run.Execute
                                    │ (flush & status persist)
                                    ▼
                              SQLite Ledger (lanes, events)
                                    │
                                    ▼
                              Batch Barrier (barrier.Barrier)
```

1. **Child Process ──→ Executor `io.MultiWriter` (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`):**
   - *Invariant*: Subprocess stdio draining is bounded by `cmd.WaitDelay` (`internal/executor/agy.go:39`, `internal/executor/cursor_agent.go:29`, `internal/executor/opencode.go:47`).
   - *Failure impact*: If grandchild MCP processes inherit stdio and `WaitDelay` or `exec.ErrWaitDelay` handling fails, `Run` hangs until context deadline (`internal/executor/executor.go:52-62`).

2. **Executor MultiWriter ──→ Worktree Log & In-Memory SSE Hub (`internal/run/run.go:368-375`, `internal/serve/hub.go`):**
   - *Invariant*: Raw stream chunks bypass SQLite, writing to files and memory without acquiring database locks.
   - *Failure impact*: Ingesting stream chunks into SQLite causes `SQLITE_BUSY` contention with lease renewals (`internal/ledger/ledger.go:127-129`, `internal/feature/feature.go:354-385`) and violates `events.type` CHECKs (`internal/ledger/schema.go:38-42`).

3. **Stream Flush ──→ `run.Execute` Status Decision (`internal/run/run.go:402-435`):**
   - *Invariant*: Stream buffers flush (<500ms) before status persistence; failure notes in SQLite remain capped at 4096 bytes per stream (`internal/run/run.go:89`).
   - *Failure impact*: Unbounded flush delays status recording; exceeding `streamDetailCap` bloats SQLite rows beyond the small-ledger invariant (`internal/run/run.go:71-89`).

4. **`run.Execute` ──→ SQLite Ledger (`internal/ledger/ledger.go:480-485`, `internal/run/batch.go:147-153`):**
   - *Invariant*: Terminal status is committed via `SetStatus` before `b.Observe` updates the batch barrier.
   - *Failure impact*: Observing before persistence causes `barrier.Evaluate` to release on uncommitted state (`internal/barrier/barrier.go:36-47`).

5. **SSE Hub ──→ Loopback HTTP Client (`internal/serve/server.go:19-23,57-73`, `internal/serve/handlers.go:37-85`):**
   - *Invariant*: Streaming is restricted to loopback via `serve.IsLoopback`; disconnect cancels registration cleanly via `r.Context().Done()`.
   - *Failure impact*: Non-loopback binding exposes unauthenticated data (`internal/serve/server.go:12-22`); leaked subscribers exhaust goroutines.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `executor.Request` | `internal/executor/executor.go:15-37` | Add optional `StdoutWriter io.Writer` and `StderrWriter io.Writer` fields. | Yes; nil writers preserve internal buffer capture only. |
| `executor.Outcome` | `internal/executor/executor.go:42-63` | Unchanged (`ExitCode`, `TimedOut`, `Stderr`, `Stdout`, `OutputTruncated`). | Yes; preserves existing struct fields and semantics. |
| `executor.Executor` | `internal/executor/executor.go:66-80` | Interface signature unchanged (`Run`, `DefaultModel`, `KnownModels`). | Yes; callers and test stubs satisfy interface unchanged. |
| `executor.Agy.Run` | `internal/executor/agy.go:135-213` | Replace buffer assignment (`:169-172`) with `io.MultiWriter` to buffer and `req.StdoutWriter`/`req.StderrWriter`. | Yes; existing CLI flags and outcome values unchanged. |
| `executor.CursorAgent.Run` | `internal/executor/cursor_agent.go:62-134` | Replace buffer assignment (`:91-94`) with `io.MultiWriter` to buffer and `req.StdoutWriter`/`req.StderrWriter`. | Yes; existing CLI flags and outcome values unchanged. |
| `executor.Opencode.Run` | `internal/executor/opencode.go:92-184` | Replace buffer assignment (`:130-133`) with `io.MultiWriter` to buffer and `req.StdoutWriter`/`req.StderrWriter`. | Yes; fallback warning detection and outcome values unchanged. |
| `run.Deps` | `internal/run/run.go:149-212` | Add optional `Hub *serve.Hub` field. | Yes; nil hub bypasses broadcast without breaking log creation. |
| `run.Execute` | `internal/run/run.go:292-500` | Open `<wt.Path>/.lucind/lane.log` and pass multi-writers into `executor.Request` (`:368-374`). | Yes; returned `Report` and error semantics unchanged. |
| `serve.NewHandler` | `internal/serve/handlers.go:36-118` | Accept `hub *Hub` parameter to mount SSE stream. | Yes; existing `/`, `/api/state`, and `/approvals/` routes unchanged. |
| `GET /api/telemetry/events` | `internal/serve/handlers.go:37-85` | New route streaming SSE chunks via `http.Flusher`. | Yes; additive route on loopback HTTP mux. |
| `serve.Hub` | `internal/serve/server.go:1-74` | New in-memory pub-sub hub type in `internal/serve/hub.go`. | Yes; new package type with no external surface break. |
| `serve.Model` | `internal/serve/model.go:17-24` | Add `ListRunEvents(ctx context.Context, runID string) ([]EventDTO, error)` calling `Ledger.Events` (`internal/ledger/ledger.go:490-526`). | Yes; additive read-only query method on existing model. |
| SQLite Schema v5 | `internal/ledger/schema.go:9-10,18-56` | Schema version 5 and DDL unchanged (`lanes`, `events`, `approvals`). | Yes; zero migration required and CHECK constraints satisfied. |
| Log Archive Artifact | `cmd/lucind-ai/cli.go:647-660` | Copy `<wt.Path>/.lucind/lane.log` to `.lucind/results/<laneID>.log` before worktree deletion. | Yes; additive artifact created alongside existing `result.json`. |
| CLI flags (`serve`, `run`) | `cmd/lucind-ai/cli.go:56,683-686` | Unchanged (`--addr`, `--approver`, `--approval-timeout`, `--timeout`, `--packet`). | Yes; existing flags and invocations unchanged. |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/executor/executor.go` | Modify | Add optional `StdoutWriter` and `StderrWriter` `io.Writer` fields to `Request`. | `run.Execute` in `internal/run/run.go:368-375` |
| `internal/executor/agy.go` | Modify | Tee stdout/stderr to `req.StdoutWriter`/`req.StderrWriter` via `io.MultiWriter`. | `Agy.Run` in `internal/run/run.go:368-375` |
| `internal/executor/cursor_agent.go` | Modify | Tee stdout/stderr to `req.StdoutWriter`/`req.StderrWriter` via `io.MultiWriter`. | `CursorAgent.Run` in `internal/run/run.go:368-375` |
| `internal/executor/opencode.go` | Modify | Tee stdout/stderr to `req.StdoutWriter`/`req.StderrWriter` via `io.MultiWriter`. | `Opencode.Run` in `internal/run/run.go:368-375` |
| `internal/run/run.go` | Modify | Create `<wt.Path>/.lucind/lane.log`, attach sinks to `Request`, and stream to hub. | `ExecuteBatch` in `internal/run/batch.go:128` and `cmd/lucind-ai/cli.go:304` |
| `internal/serve/hub.go` | Create | In-memory SSE pub-sub `Hub` with concurrent subscription and broadcast. | `serve.NewHandler` in `internal/serve/handlers.go:36-85` and `run.Execute` in `internal/run/run.go:368` |
| `internal/serve/handlers.go` | Modify | Mount `/api/telemetry/events` SSE handler on mux using `http.Flusher`. | `serveDispatch` in `cmd/lucind-ai/cli.go:715-723` |
| `internal/serve/model.go` | Modify | Add shell-free `ListRunEvents` query method backed by `Ledger.Events`. | `TestModelSourceDoesNotShellOut` in `internal/serve/model_test.go:595-627` |
| `cmd/lucind-ai/cli.go` | Modify | Wire `serve.Hub` in `serveDispatch` and archive log in `PersistEnvelope`/`RemoveLaneWorktree`. | `serveDispatch` in `cmd/lucind-ai/cli.go:675-725` and `completeIntegration` in `cmd/lucind-ai/cli.go:641-660` |

## Open Questions

- [ ] Whether the SSE event payload format should be raw string chunks or a multiplexed JSON envelope (`{"lane_id": "...", "stream": "stdout", "chunk": "..."}`).
- [ ] Whether log archive files should be placed at `.lucind/results/<lane-id>.log` or `.lucind/logs/<run-id>/<lane-id>.log` prior to worktree removal.
