# Design: Control Room Telemetry

## Technical Approach

Dual-tier telemetry for Candidate 2 (`openspec/changes/control-room-telemetry/proposal.md:13-35`). Tier 1 tees child stdout/stderr through `io.MultiWriter` to `<wt.Path>/.lucind/lane.log` and an in-memory SSE hub — no SQLite ingest. Tier 2 keeps coarse lifecycle in `events` / `lanes`, with failure notes capped at `streamDetailCap` (4096 bytes/stream, `internal/run/run.go:71-89`). `serve.Model` adds shell-free DTOs over `Ledger.Events`. Schema stays v5: no new tables, columns, or `events.type` values (`internal/ledger/schema.go:9-10,34-43`). Call sites: `exec.Run` (`internal/run/run.go:368-375`), the three executor `Run` methods, `serve.NewHandler` (`internal/serve/handlers.go:36-118`), `serve.Model` (`internal/serve/model.go:17-24`), and `PersistEnvelope` / `RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:641-660`).

## Architecture Decisions

### Decision: Dual-tier storage (files + SSE; SQLite coarse only)

**Choice**: High-frequency chunks go to the worktree log and in-memory hub. SQLite records only `run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended` and capped `lane_note` tails.
**Alternatives considered**: A `telemetry_events` table; Unix-socket / NDJSON gateway.
**Rationale**: Concurrent `ExecuteBatch` goroutines (`internal/run/batch.go:81-89`) already share WAL SQLite (`busy_timeout=5000`, `internal/ledger/ledger.go:127-129,162-164`) with `SetStatus` and `RenewLease` (`internal/feature/feature.go:354-385`). Stream rows would contend and break the small-ledger cap (`internal/run/run.go:71-89`). New `events.type` values fail the CHECK (`internal/ledger/schema.go:38-39`).

### Decision: Optional `io.Writer` sinks on `executor.Request`

**Choice**: Keep `Executor.Run(ctx, Request) (Outcome, error)` (`internal/executor/executor.go:67`) and `Outcome` fields (`internal/executor/executor.go:42-63`). Add optional `StdoutWriter` / `StderrWriter` on `Request` (`internal/executor/executor.go:14-37`). Nil sinks keep today's buffer-only capture.
**Alternatives considered**: Streaming channels from `Run`; dropping `Outcome.Stdout`/`Stderr`.
**Rationale**: `fakeExecutor` (`internal/run/run_test.go:29-56`) and batch dispatch (`internal/run/batch.go:86-88`) stay valid. When sinks are set, `Agy`, `CursorAgent`, and `Opencode` tee with `io.MultiWriter` at the current buffer assignment (`internal/executor/agy.go:169-175`, `internal/executor/cursor_agent.go:91-97`, `internal/executor/opencode.go:130-136`) and keep `cmd.WaitDelay` / `exec.ErrWaitDelay` → `OutputTruncated` (`internal/executor/agy.go:160-197`, `internal/executor/cursor_agent.go:82-118`, `internal/executor/opencode.go:121-160`).

### Decision: Stdlib SSE on the existing loopback mux

**Choice**: `GET /api/telemetry/events` via `net/http` + `http.Flusher` on `NewHandler`'s mux (`internal/serve/handlers.go:36-118`). Bind stays loopback (`internal/serve/server.go:12-22,57-73`). Unregister on `r.Context().Done()`. Do not change per-item decide (`internal/serve/handlers.go:148-211`).
**Alternatives considered**: WebSocket libraries (`go.mod` has none); OTLP sidecars; polling-only chunk endpoints.
**Rationale**: Unidirectional localhost stream, no new dependencies, same `ListenAndServe` guard (`internal/serve/server.go:19-23`).

### Decision: Shell-free lifecycle DTOs on `serve.Model`

**Choice**: Additive `ListRunEvents(ctx, runID) ([]EventDTO, error)` reading `Ledger.Events` (`internal/ledger/ledger.go:490-526`). No `os/exec`, `os`, or git — `TestModelSourceDoesNotShellOut` forbids those imports (`internal/serve/model_test.go:595-627`). Duration is observed at `exec.Run`, not a column on `events`.
**Alternatives considered**: `git log` / `git diff` subprocesses; duration columns on `events`.
**Rationale**: `Events` already returns insertion-ordered lifecycle rows. `Model` is already the shell-free DTO layer (`internal/serve/model.go:14-24`).

### Decision: Flush streams before terminal persist; no seventh status

**Choice**: After `exec.Run` returns (`internal/run/run.go:368-375`), bound file/hub flush (<500ms) before `decideStatus` and terminal `SetStatus` (`internal/run/run.go:402-435,480-483`). `lane.Status` stays six values (`internal/lane/status.go:10-17`; CHECK at `internal/ledger/schema.go:24-25`). `barrier.Evaluate` still releases only when every observed status is `Terminal()` (`internal/barrier/barrier.go:36-47`).
**Alternatives considered**: A `streaming`/`flushing` status; Observe before flush completes.
**Rationale**: `runOneLane` Observes only after `Execute` returns (`internal/run/batch.go:128,147-153`), and Execute already persists terminal status first (`internal/run/run.go:480-483`). A seventh status would rewrite the CHECK and barrier.

### Decision: Archive the worktree log beside `PersistEnvelope`

**Choice**: Write `<wt.Path>/.lucind/lane.log` during the run (same `.lucind/` as `writeResultSchema`, `internal/run/run.go:313-316,697-705`). Before `RemoveLaneWorktree`, copy it to `.lucind/results/<lane-id>.log` next to the JSON envelope (`cmd/lucind-ai/cli.go:641-660`), called from `completeIntegration` (`internal/run/integrate.go:151-164`).
**Alternatives considered**: Delete logs with the worktree; write all concurrent logs into the primary tree during the run.
**Rationale**: Worktrees are isolated (`internal/worktree/worktree.go:184-238`). Primary-tree writes during execution would contend across lanes. `PersistEnvelope` today writes `<laneID>.json` only (`cmd/lucind-ai/cli.go:647-659`).

## Flow and Invariants

```
Child CLI stdout/stderr
    → Executor MultiWriter ─┬→ <wt.Path>/.lucind/lane.log
                            └→ serve.Hub → GET /api/telemetry/events (loopback SSE)
    → exec.Run returns → flush (<500ms) → decideStatus → terminal SetStatus
    → runOneLane Observe → barrier.Evaluate
```

1. **Stdio drain is WaitDelay-bounded.** Assignment at `agy.go:167`, `cursor_agent.go:89`, `opencode.go:128`. Without it, a grandchild holding pipes blocks `Wait` until the grandchild exits (`internal/executor/agy.go:15-23`). Failure: hang past the lane deadline; `OutputTruncated` must still report the real exit (`internal/executor/agy.go:182-197`).
2. **Chunks never touch SQLite.** File + hub only. Inserting stream types fails `events.type` CHECK (`internal/ledger/schema.go:38-39`) and would contend with `RenewLease` (`internal/feature/feature.go:354-385`).
3. **Flush before persist.** `streamDetailCap` still bounds `lane_note` (`internal/run/run.go:71-89,422-435`). Unbounded flush delays `SetStatus`; uncapped notes bloat the ledger.
4. **Barrier sees durable terminals only.** `SetStatus` commits, then `Observe` (`internal/run/run.go:480-483`, `internal/run/batch.go:147-153`). Observing first would let `Evaluate` release on uncommitted state (`internal/barrier/barrier.go:42-47`).
5. **SSE is loopback and cancel-clean.** Non-loopback listen returns `ErrNonLoopback` (`internal/serve/server.go:12-22,57-73`). Leaked subscribers exhaust goroutines.

## Interfaces / Contracts

| Surface | Today | Delta | Compatible? |
|---|---|---|---|
| `executor.Request` | `internal/executor/executor.go:14-37` | Optional `StdoutWriter`, `StderrWriter` | Yes; nil = buffer only |
| `Outcome`, `Executor` | `internal/executor/executor.go:42-80` | Unchanged | Yes |
| `Agy`/`CursorAgent`/`Opencode`.Run | buffers at `:169-175`, `:91-97`, `:130-136` | `io.MultiWriter` to buffer + sinks | Yes; WaitDelay unchanged |
| `run.Deps` | `internal/run/run.go:149-212` | Optional hub/broadcast sink | Yes; nil skips broadcast |
| `run.Execute` | `internal/run/run.go:292-510` | Open lane.log; pass sinks at `:368-374` | Yes; `Report` unchanged |
| `serve.NewHandler` | `internal/serve/handlers.go:36-118` | Accept hub; mount `/api/telemetry/events` | Yes; `/`, `/api/state`, `/approvals/` unchanged |
| `serve.Hub` | (none) | New pub-sub in `internal/serve/hub.go` | Yes; new type |
| `serve.Model` | `internal/serve/model.go:17-24` | `ListRunEvents` via `Ledger.Events` | Yes; additive |
| Schema v5 | `internal/ledger/schema.go:9-10,18-56` | Unchanged | Yes |
| CLI flags | `cmd/lucind-ai/cli.go:142-145,683-685` | Unchanged | Yes |
| Envelope | `internal/result/result.go:102-115` | Unchanged | Yes |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/executor/executor.go` | Modify | Optional writer fields on `Request` | `Execute` at `internal/run/run.go:368-375` |
| `internal/executor/agy.go` | Modify | Tee via `MultiWriter` | `Agy.Run` from `Execute` `:368-375` |
| `internal/executor/cursor_agent.go` | Modify | Same tee | `CursorAgent.Run` from `Execute` `:368-375` |
| `internal/executor/opencode.go` | Modify | Same tee; keep agent-fallback stderr scan | `Opencode.Run` from `Execute` `:368-375` |
| `internal/run/run.go` | Modify | Create lane.log; attach sinks; flush before terminal `SetStatus` | `runOneLane` `internal/run/batch.go:128`; `runDispatch` `cmd/lucind-ai/cli.go:304` |
| `internal/serve/hub.go` | Create | In-memory subscribe/broadcast | `NewHandler` `internal/serve/handlers.go:36-118`; `Execute` `:368` |
| `internal/serve/handlers.go` | Modify | Mount SSE; pass hub into `NewHandler` | `serveDispatch` `cmd/lucind-ai/cli.go:715-723` |
| `internal/serve/model.go` | Modify | `ListRunEvents` | `TestModelSourceDoesNotShellOut` `internal/serve/model_test.go:595-627` |
| `cmd/lucind-ai/cli.go` | Modify | Construct hub in `serveDispatch`; archive `.log` in `PersistEnvelope` | `ListenAndServe` `cli.go:719`; `completeIntegration` `internal/run/integrate.go:161-162` |

## Testing Strategy and Test Seams

| Layer | What | Approach | Seam |
|---|---|---|---|
| Unit | Tee + `Outcome` match | Stub scripts on `Binary`; assert sinks and buffers | `writeStub` `internal/executor/agy_test.go:18-26`; `writeCursorStub` `:18-26`; `writeOpencodeStub` `:18-26` |
| Unit | Grandchild pipes | Child exits 0, grandchild holds stdio; `OutputTruncated`, `ExitCode=0`, no hang | `agy_test.go:158-191`, `cursor_agent_test.go:178-205`, `opencode_test.go:178-205` |
| Unit | SSE headers/flush/disconnect | Drive `/api/telemetry/events`; 200, `text/event-stream`, unregister on cancel | New tests beside `internal/serve/server_test.go:17-40`; new `Hub` seam |
| Unit | Loopback guard | Non-loopback `ListenAndServe` → `ErrNonLoopback` | `internal/serve/server.go:57-73`, `internal/serve/server_test.go:17-40` |
| Unit | Shell-free Model | Keep AST import ban on new DTOs | `internal/serve/model_test.go:595-627` |
| Unit | CHECK rejects stream types | Insert unadmitted `events.type`; expect constraint error | `internal/ledger/schema.go:38-39` (new test; `TestMigrateIsIdempotent` at `ledger_test.go:733` is migration-only) |
| Integration | Log vs capped note | High-volume output; full file log; `lane_note` ≤4096 | `run.Deps` `internal/run/run.go:149-212`; `fakeExecutor` `internal/run/run_test.go:29-56`; ledger via `t.TempDir()` `:91` |
| Integration | Barrier vs flush | Continuous output; release only after every terminal `SetStatus` | `ExecuteBatch` `internal/run/batch.go:66-113`; `Evaluate` `internal/barrier/barrier.go:36-59` |
| E2E | SSE during dispatch | `lucind-ai serve` + concurrent lanes + loopback client | `serveDispatch` `cmd/lucind-ai/cli.go:674-725`; `ListenAndServe` `internal/serve/server.go:19-53` |

Existing seams to reuse: `WaitDelay` fields (`agy.go:74-81`, `cursor_agent.go:22-29`, `opencode.go:40-47`); `Deps` injection; `fstest.MapFS` worktree FS (e.g. `internal/run/run_test.go:132`). New seams: Request writers, `serve.Hub`, log open in `Execute`.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: telemetry tees stdio; no path-classification boundary | N/A | N/A |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: log path is under existing `wt.Path` | N/A | N/A |
| Commit state | staged, `commit -a`, empty index | N/A: no commit/index changes | N/A | N/A |
| Push state | tracking branch, first push, refspec | N/A: no push | N/A | N/A |
| PR commands | `--head`, env prefix, composed commands | N/A: no PR argv | N/A | N/A |

Loopback bind remains the existing serve control (`internal/serve/server.go:12-22,57-73`); RED coverage stays `TestNonLoopbackListenFails` (`internal/serve/server_test.go:17-40`) plus new SSE tests. Not a new matrix row.

## Rollback and Additivity

**Choice**: `git revert` of the telemetry commits.
**Alternatives considered**: Schema downgrade v6→v5; feature flags. Rejected: version stays 5 (`internal/ledger/schema.go:9-10`); no table or CHECK changes.
**Rationale**: Additive only. Envelope types untouched (`internal/result/result.go:102-115`). SSE is a new mux route; decide is unchanged (`internal/serve/handlers.go:148-211`). Request writers are nil-safe. Revert restores `bytes.Buffer` and drops the SSE handler; no ledger rollback; worktree logs are not git history.

## Open Questions

- [ ] SSE payload: raw stdout/stderr chunks, or a multiplexed JSON envelope (`lane_id`, `stream`, `chunk`)?

## Out of Scope

- Patching `agy` / `cursor-agent` / `opencode`, or requiring OTLP.
- WebSocket libraries or non-stdlib frameworks in `internal/serve`.
- Remote bind, TLS, or multi-tenant auth (`internal/serve/server.go:12-22,55-73`).
- A seventh `lane.Status` (`internal/lane/status.go:8-28`).
- Persisting untruncated streams into `events`.
- Parsing token usage from JSON stdout (`Outcome` has no token fields, `internal/executor/executor.go:42-63`).
