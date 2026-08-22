# Tasks: Control Room Telemetry

No `apply-dag.yaml` sidecar. Single packet, three sequential work-unit commits (Unit 1 → 2 → 3). Lens B Wave 1 (executor ∥ serve including `NewHandler`) is not Integrate-green; `handlers.go` / SSE stay in Unit 3 with `cli.go:715`.

Threat matrix (`design.md:107-118`): all five rows `N/A`. No threat-matrix RED-test tasks. Loopback stays `TestNonLoopbackListenFails` (`internal/serve/server_test.go:17-40`).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 350–550 (250–350 production across 9 files, 200–300 test across ~8 files) |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (Units 1–3) |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary | Executor |
|------|------|-----------|----------------------|-----------------|-------------------|----------|
| 1 | Optional `StdoutWriter`/`StderrWriter` on `executor.Request`; `io.MultiWriter` tee in Agy, CursorAgent, Opencode; keep `WaitDelay` | PR 1 | `go test ./internal/executor -count=1` | N/A: `writeStub` / `writeCursorStub` / `writeOpencodeStub` (`agy_test.go:18-26`, `cursor_agent_test.go:18-26`, `opencode_test.go:18-26`) | `internal/executor/`: drop writer fields; restore buffer-only `cmd.Stdout`/`Stderr` | `agy` |
| 2 | `serve.Hub` pub-sub; shell-free `ListRunEvents` on `Model`. Do not change `NewHandler` | PR 1 | `go test ./internal/serve -count=1` | N/A: in-process Hub + AST on `model.go` (`model_test.go:595-627`) | `internal/serve/`: delete `hub*`; revert `ListRunEvents` | `cursor-agent` |
| 3 | `GET /api/telemetry/events`; `lane.log` + flush before `SetStatus`; archive in `PersistEnvelope`; CHECK test | PR 1 | `go test ./internal/serve ./internal/run ./internal/ledger ./cmd/lucind-ai -count=1` | N/A: `fakeExecutor` (`run_test.go:29-56`), `httptest`, TempDir ledger (`run_test.go:91`). Live `lucind-ai serve` + real agents is out of scope | `handlers.go` signature/route; `run.go` log/flush; `cli.go` Hub + archive; new ledger CHECK test | `agy` |

`allowed_paths`: Unit 1 = `internal/executor/executor.go`, `agy.go`, `cursor_agent.go`, `opencode.go`, `agy_test.go`, `cursor_agent_test.go`, `opencode_test.go`. Unit 2 = `internal/serve/hub.go` (new), `hub_test.go` (new), `model.go`, `model_test.go`. Unit 3 = `internal/serve/handlers.go`, `server_test.go`, `internal/run/run.go`, `run_test.go`, `batch_test.go`, `cmd/lucind-ai/cli.go`, `cli_test.go`, `internal/ledger/ledger_test.go`.

Same-wave pairs: none (sequential packet). File-level, Unit 1 (`internal/executor/`) vs Unit 2 (`hub.go`/`model.go`) is disjoint under `packet.PathInScope`. Unit 2 vs Unit 3 share package `serve` but different files; they still cannot run in parallel — Unit 3 needs `Hub`.

## Phase 1: Foundation & Core Interfaces

- [ ] 1.1 Add optional `StdoutWriter` and `StderrWriter` (`io.Writer`) to `Request` (`internal/executor/executor.go:15-37`). Nil = buffer-only.
- [ ] 1.2 Create `internal/serve/hub.go`: thread-safe subscribe, unregister, `Broadcast`.
- [ ] 1.3 Add `ListRunEvents(ctx context.Context, runID string) ([]EventDTO, error)` on `Model` (`internal/serve/model.go:17-24`) via `Ledger.Events` (`internal/ledger/ledger.go:490-526`). No `os/exec`, `os`, or git.

## Phase 2: Executor MultiWriter Streaming

- [ ] 2.1 Tee Agy stdout/stderr with `io.MultiWriter` when sinks are set (`internal/executor/agy.go:169-175`); keep `WaitDelay` / `OutputTruncated` (`:182-197`).
- [ ] 2.2 Same tee on CursorAgent (`internal/executor/cursor_agent.go:91-97`, `:104-118`).
- [ ] 2.3 Same tee on Opencode (`internal/executor/opencode.go:130-136`); keep agent-fallback stderr scan (`:142-160`).

## Phase 3: Execution Runtime & Server Integration

- [ ] 3.1 Accept `*Hub` on `NewHandler` (`internal/serve/handlers.go:36-118`); mount `GET /api/telemetry/events` (loopback already `ListenAndServe`, `internal/serve/server.go:19-22`); `text/event-stream`, flush, unregister on `r.Context().Done()`. Do not change decide (`:148-211`). Update `cli.go:715` in 3.3 in the same unit.
- [ ] 3.2 In `Execute` (`internal/run/run.go:368-375`): create `<wt.Path>/.lucind/lane.log` beside `writeResultSchema` (`:313-316`); pass file + hub sinks on `Request`; optional hub on `Deps` (`:149-212`). Bound flush (<500ms) after `exec.Run` returns, before `decideStatus` / diagnosis (`:402-435`) and terminal `SetStatus` (`:480-483`). Keep `streamDetailCap` 4096 (`:71-89`).
- [ ] 3.3 Construct `serve.Hub` in `serveDispatch` (`cmd/lucind-ai/cli.go:715-723`). In `PersistEnvelope` (`:647-660`), copy `lane.log` to `.lucind/results/<lane-id>.log` before `RemoveLaneWorktree` (`:641-646`).

## Phase 4: Testing & Verification

Tests travel with the unit that implements the behavior (same Integrate lane).

- [ ] 4.1 Extend Agy/CursorAgent/Opencode grandchild and exit tests (`agy_test.go:28,166`, `cursor_agent_test.go:28`, `opencode_test.go:28`) to assert concurrent sink teeing, exit codes, and `OutputTruncated`.
- [ ] 4.2 Add `internal/serve/hub_test.go` (pub-sub, unregister). Beside `server_test.go:17-40`, add SSE tests: 200, `text/event-stream`, flush, disconnect cleanup. Keep `TestNonLoopbackListenFails`.
- [ ] 4.3 Add `TestModelListRunEvents` mapping DTOs from `Ledger.Events`. Keep `TestModelSourceDoesNotShellOut` (`model_test.go:595-627`) over the new method.
- [ ] 4.4 **New** test (not `TestMigrateIsIdempotent` at `ledger_test.go:733`): insert an unadmitted `events.type`; expect CHECK failure (`internal/ledger/schema.go:38-39`). Schema stays v5 (`:9-10`).
- [ ] 4.5 With `fakeExecutor` (`run_test.go:29-56`) and `newTestDeps` (`:91`): full raw output in `.lucind/lane.log`; `lane_note` still ≤4096 (`TestExecuteOversizedStderrLedgerNoteIsBoundedAndKeepsTail`). With `ExecuteBatch` (`batch.go:66-113`) and `Observe` after `Execute` (`:147-153`): barrier releases only after terminal `SetStatus`. Extend `TestExecuteBatchBarrierStaysIdleWhileOneLaneWaitsForApproval`. Archive: extend `TestRunDispatchPersistsIntegratedLaneEnvelopeToPrimaryRoot` (`cli_test.go:1154`).

## Dependency Order

1.1 → 2.1, 2.2, 2.3 → 4.1. 1.2 → 4.2 (Hub). 1.3 → 4.3. 1.2 + 3.3 (same unit as 3.1) → SSE in 4.2. 1.1 + 1.2 → 3.2 → 4.5. 3.1 + 3.2 → 3.3. 4.4 independent. Sequential units: 1 then 2 then 3 (3 needs `Hub` and writer fields).

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Worktree-Local Log Teeing and Process Invariants (`specs/lane-telemetry-streaming/spec.md`) | 1.1, 2.1–2.3, 3.2, 3.3, 4.1, 4.5 |
| High-Frequency SQLite Ledger Isolation (`specs/lane-execution/spec.md`) | 3.2, 4.4, 4.5 |
| Loopback Server-Sent Events Telemetry Stream (`specs/approvals-web-ui/spec.md`) | 1.2, 3.1, 3.3, 4.2 |
| Feature Attempt Audit Preservation (`specs/parent-feature-integration/spec.md`) | 4.4 (no stream types in `events`; `WriteWithAudit` unchanged) |
| Shell-Free Run Lifecycle Query (`specs/shell-free-telemetry-query/spec.md`) | 1.3, 3.2, 4.3, 4.5 |

## Open Questions

- [ ] SSE payload: raw stdout/stderr chunks vs multiplexed JSON (`lane_id`, `stream`, `chunk`) — `design.md:127`. Apply must not invent a format the design left open.
