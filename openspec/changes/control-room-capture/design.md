# Design: Control Room Capture

## Technical Approach

Candidate 4: spool child stdio continuously to append-only files under `<primaryRoot>/.lucind/`, keep SQLite `events.detail` at `streamDetailCap` (4096 bytes/stream), and expose loopback SSE tail, transcript download, and `serve.NewModel` JSON from existing `lucind-ai serve`. No daemon, no stream blobs, no WaitDelay change. Maps to the proposal’s four requirements (primary-root spooling, non-interfering WaitDelay, bounded notes, loopback HTTP). Change-local specs are not in this worktree; modified live capabilities are `lane-execution` and `approvals-web-ui`.

## Architecture Decisions

### Decision: Primary-root file spooling, not SQLite chunks

**Choice**: Tee stdout/stderr with `io.MultiWriter` into files under `<primaryRoot>/.lucind/` (working layout `runs/<run_id>/lanes/<lane_id>.log`; prefix and split-vs-interleaved remain open).
**Alternatives considered**: SQLite `lane_stream_chunks`; worktree-local logs; buffers until exit.
**Rationale**: Concurrent lanes already write events and statuses under a 4-conn pool and `busy_timeout(5000)` (`internal/ledger/ledger.go:128,162-163,180-184`; `internal/run/batch.go:80-89`; `internal/run/run.go:425-434`). `completeIntegration` persists the envelope then removes the worktree (`internal/run/integrate.go:159-163`; `cmd/lucind-ai/cli.go:641-646`). `.lucind/` is gitignored (`.gitignore:2`).
**Terminal consumer**: `Execute` building `executor.Request` (`internal/run/run.go:368-374`); stdio assignment in `Agy.Run` / `CursorAgent.Run` / `Opencode.Run` (`internal/executor/agy.go:169-171`, `cursor_agent.go:91-93`, `opencode.go:130-132`).

### Decision: Destination writers on Request; Outcome stays diagnosis-only

**Choice**: Add stdout/stderr destination `io.Writer` fields on `Request` (`internal/executor/executor.go:15-37`). `Outcome` keeps `ExitCode`, `TimedOut`, `Stderr`, `Stdout`, `OutputTruncated` (`internal/executor/executor.go:42-63`).
**Alternatives considered**: Megabyte transcripts on `Outcome`; dropping string captures; channels into `Run`.
**Rationale**: `decideStatus` and `diagnosisDetail` stay synchronous and disk-independent (`internal/run/run.go:125-144,402,549-572`).
**Terminal consumer**: `Execute` at `internal/run/run.go:368-374`.

### Decision: Preserve WaitDelay drain

**Choice**: Keep default 5s `WaitDelay` and `errors.Is(err, exec.ErrWaitDelay)` → `OutputTruncated` with `ProcessState` exit (`internal/executor/agy.go:160-168,182-197`; `cursor_agent.go:82-90,104-118`; `opencode.go:121-129,143-160`).
**Alternatives considered**: `WaitDelay = 0`; `Setpgid`; treat `ErrWaitDelay` as `lane.Failed`.
**Rationale**: MCP grandchildren hold pipes. Indefinite wait hangs the batch (`internal/run/batch.go:80-89`). Fatal drain would reject valid `lane.Done` (`internal/run/run.go:402,488-499,506`; `internal/result/result.go:117-135`).
**Terminal consumer**: `Execute` recording `OutputCaptureIncomplete` (`internal/run/run.go:488-499,506`).

### Decision: Bounded SQLite notes; full bytes on disk

**Choice**: Keep `streamDetailCap = 4096` (`internal/run/run.go:89,132-144`) on `EventLaneNote` for non-zero exit, timeout, or unreadable envelope (`internal/run/run.go:416-435`). Clean `Done` writes no failure note.
**Alternatives considered**: Unclipped blobs; 64KB+ cap; disk-only (no notes).
**Rationale**: The ledger coordinates status, not transcripts (`internal/ledger/ledger.go:452-475`; `internal/run/run.go:425-434`). Diagnosis reaches stdout via `printReport` (`cmd/lucind-ai/cli.go:512-536`).
**Terminal consumer**: `diagnosisDetail` → `Ledger.AppendEvent` (`internal/run/run.go:125-144,425-434`).

### Decision: Loopback SSE tail and file download

**Choice**: SSE (`http.Flusher`) tails the spool file; finished download uses `http.ServeContent`. End on `r.Context().Done()`. No daemon, no WebSockets.
**Alternatives considered**: Required serve daemon; WebSockets; polling `/api/state`.
**Rationale**: `run` and `serve` are independent subcommands (`cmd/lucind-ai/cli.go:99-127,674-725`). Listen is loopback-only (`internal/serve/server.go:19-53`). Tailing the file avoids backpressuring the child.
**Terminal consumer**: `NewHandler` (`internal/serve/handlers.go:36-118`) from `serveDispatch` (`cmd/lucind-ai/cli.go:715-719`).

### Decision: Wire Model JSON into this change’s handler

**Choice**: Register read-only `/api/model/...` routes on `serve.NewHandler` over `serve.NewModel` (`internal/serve/model.go:21-24,14-125,128-343`).
**Alternatives considered**: Keep the mux approvals-only (`internal/serve/handlers.go:79-115`); defer Model routes to `control-room-serve`; a separate HTTP subcommand.
**Rationale**: `NewModel` already maps features, attempts, leases, and audit; only `lucind-ai feature status` calls it (`cmd/lucind-ai/cli.go:820-852`). `serveDispatch` never constructs it (`cmd/lucind-ai/cli.go:715`).
**Terminal consumer**: `serveDispatch` (`cmd/lucind-ai/cli.go:715-719`).

## Flow and Invariants

```
CLI run → Execute → executor.Run (MultiWriter) → .lucind/…/<lane>.log
              │                 │                         │
         EventLaneNote     Outcome (diagnosis)      serve SSE/download
```

1. **Spool before spawn.** Open the primary-root log before `exec.Run` (`internal/run/run.go:368-374`). Files inside `wt.Path` vanish when `completeIntegration` removes the worktree (`internal/run/integrate.go:159-163`). Candidates must live under `<primaryRoot>/.lucind/`; `ledgerpath.Validate` rejects worktree-shaped paths (`internal/ledgerpath/ledgerpath.go:40-58`). `Resolve` today returns only `lucind.db` (`internal/ledgerpath/ledgerpath.go:34-38`).
2. **Tee concurrently.** All three executors wrap buffers with dest writers when non-nil (`internal/executor/agy.go:169-173`; `cursor_agent.go:91-95`; `opencode.go:130-134`). Nil dest preserves today’s capture.
3. **WaitDelay isolation.** `ErrWaitDelay` sets `OutputTruncated`, keeps the exit code, and does not fail `Done` (`internal/executor/agy.go:182-197` and siblings; `internal/run/run.go:506`).
4. **Bounded notes.** Failure notes cap at 4096 bytes/stream; `Done` with empty reason writes none (`internal/run/run.go:89,125-144,416-435`).
5. **HTTP tail is file-based.** SSE reads the spool; disconnect ends the handler (`internal/serve/handlers.go:36-118`; `internal/serve/server.go:19-53`). Must not write back into the child’s pipes.

## Interfaces / Contracts

```go
// Request gains stdout/stderr destination io.Writer fields (nil = today's capture).
type Report struct { /* existing */; LogPath string }
func ResolveLog(primaryRoot, runID, laneID string) string
func NewHandler(l *ledger.Ledger, defaultApprover, opencodeCmd, primaryRoot string) http.Handler
```

Additive routes: `GET /api/runs/{runID}/lanes/{laneID}/tail` (`text/event-stream`), `GET /api/runs/{runID}/lanes/{laneID}/log` (`ServeContent`), plus `/api/model/...`. Existing `/`, `/api/state`, `/approvals/` stay (`internal/serve/handlers.go:36-118`).

## File Changes

| File | Action | Terminal consumer |
|---|---|---|
| `internal/executor/executor.go` | Destination writers on `Request` | `Execute` (`internal/run/run.go:368-374`) |
| `internal/executor/agy.go` | `MultiWriter` when dest non-nil (`:169-173`) | `cmd.Run` in `Agy.Run` (`agy.go:173`) |
| `internal/executor/cursor_agent.go` | Same (`:91-95`) | `CursorAgent.Run` (`cursor_agent.go:95`) |
| `internal/executor/opencode.go` | Same (`:130-134`) | `Opencode.Run` (`opencode.go:134`) |
| `internal/ledgerpath/ledgerpath.go` | Add `ResolveLog` beside `Resolve` (`:34-38`) | `Execute` spool create; serve tail/download |
| `internal/run/run.go` | Open log in `Execute`; set dests; `Report.LogPath` (`:149-258,368-374`) | `ExecuteBatch` from `runDispatch` (`cmd/lucind-ai/cli.go:304`) |
| `internal/serve/handlers.go` | SSE, download, Model routes (`:36-118`) | `serveDispatch` (`cmd/lucind-ai/cli.go:715-719`) |
| `cmd/lucind-ai/cli.go` | Pass `primaryRoot` into `NewHandler` (`:674-725`) | `run` switch `case "serve"` (`cli.go:112-113`) |

## Testing Strategy and Test Seams

| Layer | What | Approach | Seam |
|---|---|---|---|
| Unit | Tee dest writers; WaitDelay `OutputTruncated` | `writeStub` / sibling stubs + `bytes.Buffer` dests; grandchild `sleep` with small `WaitDelay` | `internal/executor/agy_test.go:18-26,28-65,158-218`; `cursor_agent_test.go:18-50`; `opencode_test.go:18-80` |
| Unit | Execute keeps `Done` + `OutputCaptureIncomplete` | `fakeExecutor` with `OutputTruncated` | `internal/run/run_test.go:25-56,651-669` |
| Unit | 4096-byte tail notes; none on `Done` | Oversized streams on fake executor | `internal/run/run.go:89,132-144,416-435`; `run_test.go:856-896,1106-1128` |
| Unit | Log path under primary `.lucind/`; reject worktrees | Table-driven `Validate` | `internal/ledgerpath/ledgerpath.go:40-58`; `ledgerpath_test.go:37-60` |
| HTTP | SSE live tail, 404 missing, disconnect, `ErrNonLoopback` | `httptest`; new tail pump | `internal/serve/server_test.go:17-40`; new handler tests (mux today is approvals-only) |
| Integration | Files persist across Done/Blocked/Failed/Deviated; survive worktree delete | Injected `Deps`, temp primary root | `internal/run/run.go:149-212,368-435`; `batch_test.go:530-573`; `integrate_test.go:392-435` |
| E2E | `lucind-ai run` with a stub child creates primary `.lucind/` logs | New CLI tests | `cmd/lucind-ai/cli.go:99-173,304`; existing `cli_test.go:37-48` is usage-only |

**Existing seams**: subprocess stubs; `run.Deps`; `fakeExecutor`; `ledgerpath.Validate`; `httptest` loopback.
**New seams**: destination writers on `Request`; `NewHandler` takes `primaryRoot` (or a log reader).

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: does not classify or execute documentation files | — | None |
| Git repository selection | Applicable | Logs live under `<primaryRoot>/.lucind/` via a new `ResolveLog` plus `Validate`. Worktree-shaped or `../` candidates return `ErrLedgerOutsidePrimaryRepo` and abort. | `ledgerpath_test.go` rejects worktree/traversal; `run_test.go` asserts `Execute` creates logs under primary `.lucind/`, never in worktrees |
| Commit state | N/A: does not create commits or touch the index | — | None |
| Push state | N/A: no push | — | None |
| PR commands | N/A: no PR argv | — | None |

Safe: logs on primary `.lucind/`. Failure: non-primary candidate errors; dispatch does not start.

## Rollback and Additivity

**Choice**: `git revert <sha>` restores in-memory `bytes.Buffer` capture and diagnosis-only notes (`internal/run/run.go:416-435`).
**Alternatives considered**: DB migration rollback; log-cleanup utilities. Rejected: no schema move (`schemaVersion = 5` at `internal/ledger/schema.go:9-10`); gitignored `.lucind/` files (`.gitignore:2`) are working state.
**Rationale**: Additive. Logs sit beside `.lucind/results/` (`cmd/lucind-ai/cli.go:655-660`) and `.lucind/lucind.db` (`internal/ledgerpath/ledgerpath.go:30-38`). `events.type` stays the six lifecycle values (`internal/ledger/schema.go:38-39`). Envelope types (`internal/result/result.go:102-135`) and the embedded schema (`internal/result/schema.go:10-28`) stay. Existing HTTP routes stay.

## Open Questions and Out of Scope

- [ ] Prefix: `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`?
- [ ] One interleaved file vs `.stdout.log` / `.stderr.log`?
- [ ] Retention: copy run logs at archive (today only `.lucind/packets/` and `.lucind/results/` — `plugin/claude-code/skills/lucind-ai/SKILL.md:280-283`), prune via `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:56`), or leave gitignored?

Out of scope: UI chrome and xterm (`control-room-ui-shell`, `control-room-ui-views`); listener/mux architecture (`control-room-serve`); schema migrations (`control-room-ledger`); token/timeline telemetry (`control-room-telemetry`); PTY / `Setpgid`.
