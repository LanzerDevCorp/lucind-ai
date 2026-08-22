# Tasks Lens A — Decomposition & Ordering: Control Room Telemetry

## Assumed decomposition

The change decomposes into four sequential phases: Foundation & Core Interfaces (defining executor sinks, SSE hub, and shell-free event DTOs), Executor MultiWriter Streaming (enabling concurrent multi-writer streaming across agy, cursor-agent, and opencode while honoring WaitDelay), Execution Runtime & Server Integration (wiring worktree logs, SSE HTTP endpoints, log archiving, and flush-before-persist barriers), and Testing & Verification (comprehensive unit, AST, schema CHECK, and barrier integration testing). The critical path runs through `internal/executor/executor.go` → executor multi-writer teeing (`agy.go`, `cursor_agent.go`, `opencode.go`) → `internal/run/run.go` stream lifecycle and bounded flush → CLI integration in `cmd/lucind-ai/cli.go`.

## Phase 1: Foundation & Core Interfaces

- [ ] 1.1 Modify `internal/executor/executor.go:15-37` to add optional `StdoutWriter` and `StderrWriter` (`io.Writer`) fields to `Request`.
- [ ] 1.2 Create `internal/serve/hub.go` (new file) implementing `Hub` with thread-safe client subscription, channel unregistration, and `Broadcast` methods.
- [ ] 1.3 Modify `internal/serve/model.go:17-24` to implement `ListRunEvents(ctx context.Context, runID string) ([]EventDTO, error)` querying `Ledger.Events` without spawning subprocesses.

## Phase 2: Executor MultiWriter Streaming

- [ ] 2.1 Modify `internal/executor/agy.go:169-175` to tee child stdout and stderr to `req.StdoutWriter` and `req.StderrWriter` via `io.MultiWriter`, preserving `cmd.WaitDelay` handling and `OutputTruncated` return (`:182-197`).
- [ ] 2.2 Modify `internal/executor/cursor_agent.go:91-97` to tee child stdout and stderr to `req.StdoutWriter` and `req.StderrWriter` via `io.MultiWriter`, preserving `cmd.WaitDelay` handling and `OutputTruncated` return (`:103-118`).
- [ ] 2.3 Modify `internal/executor/opencode.go:130-136` to tee child stdout and stderr to `req.StdoutWriter` and `req.StderrWriter` via `io.MultiWriter`, retaining fallback stderr scanning and `cmd.WaitDelay` handling (`:142-160`).

## Phase 3: Execution Runtime & Server Integration

- [ ] 3.1 Modify `internal/serve/handlers.go:36-118` to accept `*Hub` in `NewHandler` and mount `GET /api/telemetry/events` with loopback check, `text/event-stream` response headers, event flushing, and context cancellation cleanup.
- [ ] 3.2 Modify `internal/run/run.go:368-375,402-435,480-483` to create `<wt.Path>/.lucind/lane.log`, pass file and hub broadcast sinks into `executor.Request`, and bound stream flush (<500ms) before terminal `SetStatus` and capped failure note writing.
- [ ] 3.3 Modify `cmd/lucind-ai/cli.go:641-660,715-723` to construct `serve.Hub` in `serveDispatch` and archive `<wt.Path>/.lucind/lane.log` to `.lucind/results/<lane-id>.log` in `PersistEnvelope` before worktree cleanup.

## Phase 4: Testing & Verification

- [ ] 4.1 Modify `internal/executor/agy_test.go:18-26`, `internal/executor/cursor_agent_test.go:18-26`, and `internal/executor/opencode_test.go:18-26` to assert concurrent sink teeing, exit code preservation, and `OutputTruncated` handling during grandchild pipe holds.
- [ ] 4.2 Create `internal/serve/hub_test.go` (new file) and modify `internal/serve/server_test.go:17-40` to test `Hub` pub-sub broadcasting, SSE header compliance, subscriber cleanup on disconnect, and non-loopback bind rejection.
- [ ] 4.3 Modify `internal/serve/model_test.go:595-627` to test `ListRunEvents` event mapping and assert AST import bans prevent `os/exec` and `git` references in `model.go`.
- [ ] 4.4 Modify `internal/ledger/ledger_test.go:733` to verify SQLite schema v5 CHECK constraints reject unadmitted streaming event types in `events`.
- [ ] 4.5 Modify `internal/run/run_test.go:29-56,91` and `internal/run/batch_test.go:66-113` to test worktree log persistence, 4096-byte `lane_note` failure truncation, and batch barrier release only after terminal `SetStatus` commit.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Base interface and struct change with no internal dependencies |
| 1.2 | — | Standalone in-memory pub-sub primitive with no internal dependencies |
| 1.3 | — | DTO query method reading existing `Ledger.Events` with no internal dependencies |
| 2.1 | 1.1 | Requires `req.StdoutWriter` and `req.StderrWriter` fields on `executor.Request` |
| 2.2 | 1.1 | Requires `req.StdoutWriter` and `req.StderrWriter` fields on `executor.Request` |
| 2.3 | 1.1 | Requires `req.StdoutWriter` and `req.StderrWriter` fields on `executor.Request` |
| 3.1 | 1.2 | Handler mounting requires `*serve.Hub` parameter in `NewHandler` |
| 3.2 | 1.1, 1.2 | Requires `executor.Request` sink fields and `serve.Hub` broadcast integration |
| 3.3 | 1.2, 3.1, 3.2 | Requires `serve.Hub` construction for `serveDispatch` and log creation from `run.Execute` |
| 4.1 | 2.1, 2.2, 2.3 | Exercises teeing and process lifecycle across all three executor implementations |
| 4.2 | 1.2, 3.1 | Exercises `Hub` broadcast mechanism and `/api/telemetry/events` HTTP handler |
| 4.3 | 1.3 | Tests `ListRunEvents` queries and AST import assertions on `model.go` |
| 4.4 | — | Tests SQLite schema v5 CHECK constraints independently of code changes |
| 4.5 | 3.2 | Exercises worktree log creation, bounded flush, and barrier release logic |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `Worktree-Local Log Teeing and Process Invariants` (`specs/lane-telemetry-streaming/spec.md:5-27`) | 1.1, 2.1, 2.2, 2.3, 3.2, 3.3, 4.1, 4.5 |
| `High-Frequency SQLite Ledger Isolation` (`specs/lane-execution/spec.md:5-26`) | 3.2, 4.4, 4.5 |
| `Loopback Server-Sent Events Telemetry Stream` (`specs/approvals-web-ui/spec.md:5-26`) | 1.2, 3.1, 3.3, 4.2 |
| `Feature Attempt Audit Preservation` (`specs/parent-feature-integration/spec.md:5-8`) | 1.3, 4.3, 4.4 |
| `Shell-Free Run Lifecycle Query` (`specs/shell-free-telemetry-query/spec.md:5-26`) | 1.3, 3.2, 4.3, 4.5 |

## Open Questions

- [ ] None
