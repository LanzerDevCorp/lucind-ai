# Design Lens C — Failure, Test & Rollback: Control Room Telemetry

## Assumed architecture

This design assumes Candidate 2: worktree-local log files and an in-memory loopback Server-Sent Events (SSE) broadcast hub. `internal/executor.Request` gains optional `io.Writer` sinks; `Agy`, `CursorAgent`, and `Opencode` use `io.MultiWriter` for stdout/stderr without altering `Outcome` or `WaitDelay`. `internal/run.Execute` creates a log under `wt.Path/.lucind/` and tees child streams to an in-memory hub in `internal/serve`, keeping `internal/ledger` schema v5 unchanged. `internal/serve` mounts `/api/telemetry/events` on loopback and exposes shell-free event DTOs via `serve.Model`.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit | Executor stream teeing & `MultiWriter` | Drive `Run` with stub scripts emitting stdout/stderr; assert chunks reach writer sinks and `Outcome` matches. | `internal/executor/agy_test.go:18-26` (`writeStub`), `internal/executor/cursor_agent_test.go:18-26`, `internal/executor/opencode_test.go:18-26` |
| Unit | Grandchild pipe retention & `WaitDelay` | Stub exits 0 while grandchild keeps stdio open; assert `OutputTruncated = true`, `ExitCode = 0`, and no hang. | `internal/executor/agy_test.go:158-191`, `internal/executor/cursor_agent_test.go:178-205`, `internal/executor/opencode_test.go:178-205` |
| Unit | SSE endpoint headers & chunk flushing | Connect `httptest.NewRecorder` with `http.Flusher` to `/api/telemetry/events`; assert 200 OK, `text/event-stream`, and live chunks. | `internal/serve/handlers_test.go:18-40`, new seam required in `internal/serve/handlers.go:36-85` |
| Unit | SSE subscriber disconnect cleanup | Cancel client request context; verify hub subscriber unregisters and goroutine exits without leaks. | New seam required on `serve.Hub` subscriber channel |
| Unit | Loopback bind guard on telemetry routes | Pass loopback and non-loopback addresses to `ListenAndServe`; assert non-loopback returns `ErrNonLoopback`. | `internal/serve/server.go:57-73` (`IsLoopback`), `internal/serve/server_test.go:17-40` |
| Unit | Shell-free query safety for telemetry DTOs | AST/import parse of `internal/serve/model.go`; verify no imports of `os/exec`, `os`, or references to `"git"`. | `internal/serve/model_test.go:595-627` (`TestModelSourceDoesNotShellOut`) |
| Unit | Ledger schema v5 integrity & CHECKs | Attempt inserting unadmitted event types; verify SQLite `events` CHECK rejects stream chunks at v5. | `internal/ledger/schema.go:9-10,34-43`, `internal/ledger/ledger_test.go:733` (`TestMigrateIsIdempotent`) |
| Integration | Worktree log teeing & status isolation | Execute lane with high-volume output; assert full log written to worktree, `lane_note` capped at 4096 bytes, status clean. | `internal/run/run.go:149-203` (`Deps`), `internal/run/run.go:71-102` (`streamDetailCap`), `internal/run/run_test.go:29-56` (`fakeExecutor`) |
| Integration | Barrier sync under stream flush | Run batch with continuous stream output; assert barrier releases only after every lane persists terminal status. | `internal/run/batch.go:29-112` (`ExecuteBatch`), `internal/run/batch.go:120-155` (`runOneLane`), `internal/barrier/barrier.go:36-59` (`Evaluate`) |
| E2E | Loopback SSE client during lane dispatch | Start `lucind-ai serve`, dispatch concurrent lanes, and stream SSE events to loopback client until batch completion. | `cmd/lucind-ai/cli.go:674-725` (`serveCmd`), `internal/serve/server.go:19-53` |

## Test Seams

### Existing Seams
- **Subprocess stubbing**: `writeStub` creates shell scripts injected as `Binary` on `executor.Agy` (`internal/executor/agy.go:10-38`, `internal/executor/agy_test.go:18-26`), `executor.CursorAgent` (`internal/executor/cursor_agent.go:10-23`, `internal/executor/cursor_agent_test.go:18-26`), and `executor.Opencode` (`internal/executor/opencode.go:10-38`, `internal/executor/opencode_test.go:18-26`).
- **WaitDelay configuration**: `WaitDelay` field on `Agy` (`internal/executor/agy.go:12,160-163`), `CursorAgent` (`internal/executor/cursor_agent.go:12,82-85`), and `Opencode` (`internal/executor/opencode.go:12,121-124`).
- **Composition root injection**: `run.Deps` (`internal/run/run.go:149-203`) injects `LookupExecutor`, `CreateWorktree`, `WorktreeFS`, `Ledger`, `Now`, and `PersistEnvelope`.
- **Test doubles**: `fakeExecutor` (`internal/run/run_test.go:29-56`) implements `executor.Executor` with pre-programmed `Outcome` and `beforeReturn` hook.
- **In-memory SQLite & virtual FS**: `ledger.Open(":memory:")` and `fstest.MapFS` (`internal/run/run_test.go:95-110`).
- **Loopback predicate**: `serve.IsLoopback` (`internal/serve/server.go:57-73`).
- **AST import assertions**: `TestModelSourceDoesNotShellOut` (`internal/serve/model_test.go:595-627`).

### New Seams Required
- **`executor.Request` stream sinks**: Optional `StdoutWriter io.Writer` and `StderrWriter io.Writer` on `executor.Request` (`internal/executor/executor.go:15-37`), teed by `Agy.Run`, `CursorAgent.Run`, and `Opencode.Run` via `io.MultiWriter`.
- **`serve.Hub` broadcaster**: In-memory SSE hub in `internal/serve`, injectable into `run.Deps` and `serve.NewHandler`.
- **Worktree log creation hook**: Hook in `run.Execute` (`internal/run/run.go:368-375`) opening `.lucind/lane.log` under `wt.Path` as sink to `executor.Request`.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: telemetry streams stdio to logs; no file classification boundary exists. | N/A — no path classification boundary | N/A |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: log destination is rooted under established `wt.Path`; no repo selector is altered. | N/A — no repository routing boundary | N/A |
| Commit state | staged, `commit -a`, empty index | N/A: stdio teeing operates on live process streams; git staging and commits are untouched. | N/A — no commit or index boundary | N/A |
| Push state | tracking branch, first push, explicit refspec | N/A: telemetry does not manage remote push state, branch tracking, or refspecs. | N/A — no push or refspec boundary | N/A |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: telemetry does not construct or execute VCS PR automation commands. | N/A — no PR command boundary | N/A |

## Rollback and Additivity

**Choice**: Clean `git revert` of telemetry commits.
**Alternatives considered**: Schema downgrade migration v6 -> v5, or feature flags. Rejected because SQLite schema version stays 5 (`internal/ledger/schema.go:9-10`) and no database tables or CHECK constraints change.
**Rationale**: All format and API additions are strictly additive:
- **Ledger schema**: `schemaVersion = 5` is unchanged. High-frequency telemetry streams bypass SQLite to worktree files and in-memory SSE, avoiding schema migrations and `SQLITE_BUSY` lock contention with feature lease renewals (`internal/feature/feature.go:354-385`).
- **Packet envelope**: `.lucind/result.schema.json` and `internal/result.Envelope` types are untouched (`internal/result/result.go:10-40`).
- **HTTP API**: `/api/telemetry/events` is an additive route on `http.ServeMux` (`internal/serve/handlers.go:36-85`); `/`, `/api/state`, `/approvals/`, and approval handling (`internal/serve/handlers.go:148-211`) remain unchanged.
- **Executor contract**: `executor.Request` writer additions are optional and nil-safe; `Executor.Run` and `Outcome` signatures remain identical (`internal/executor/executor.go:42-80`).
- Reverting restores `bytes.Buffer` in executors and drops the SSE handler without leaving orphaned state or requiring ledger rollbacks.

## Out of Scope

- Modifying upstream agent CLIs (`agy`, `cursor-agent`, `opencode`) or requiring OTLP integration.
- Adding third-party WebSocket libraries or non-stdlib web frameworks to `internal/serve`.
- Remote network exposure, TLS termination, or multi-tenant token auth for the approvals server.
- Adding new `lane.Status` enum states (`internal/lane/status.go:8-28`).
- Persisting un-truncated stream logs into SQLite `events` table.

## Open Questions

- [ ] Should worktree-local logs (`.lucind/lane.log`) be archived to `.lucind/logs/<run-id>/<lane-id>.log` before `RemoveLaneWorktree` deletes the worktree (`cmd/lucind-ai/cli.go:641-646`)?
- [ ] Threat matrix reference table was read from `~/.claude/skills/sdd-design/references/threat-matrix.md` as the packet context contained a template placeholder.
