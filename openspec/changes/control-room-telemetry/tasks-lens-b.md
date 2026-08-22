# Tasks Lens B — Partition & Dispatch Shape: Control Room Telemetry

## Assumed decomposition

The work decomposes into three dispatchable units: (1) executor stdio teeing with optional writer sinks across `Agy`, `CursorAgent`, and `Opencode`; (2) loopback SSE telemetry `Hub` and shell-free lifecycle event DTOs on `serve.Model`; and (3) run engine integration (`lane.log` creation, stream flushes before terminal `SetStatus`, log archiving in `PersistEnvelope`, and CLI wiring). Units 1 and 2 are independent and parallelizable; Unit 3 integrates both and forms the critical path.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Optional writer sinks on `executor.Request` and stdio teeing via `io.MultiWriter` across `Agy`, `CursorAgent`, and `Opencode` runners while preserving `WaitDelay` | `internal/executor/executor.go`<br>`internal/executor/agy.go`<br>`internal/executor/cursor_agent.go`<br>`internal/executor/opencode.go`<br>`internal/executor/agy_test.go`<br>`internal/executor/cursor_agent_test.go`<br>`internal/executor/opencode_test.go` | `agy` | `internal/executor/`: reverts writer fields on `Request` and restores single-buffer command execution |
| 2 | In-memory SSE telemetry `Hub`, `GET /api/telemetry/events` loopback endpoint on `serve.NewHandler`, and shell-free `ListRunEvents` on `serve.Model` | `internal/serve/hub.go` (new file)<br>`internal/serve/hub_test.go` (new file)<br>`internal/serve/handlers.go`<br>`internal/serve/server_test.go`<br>`internal/serve/model.go`<br>`internal/serve/model_test.go` | `cursor-agent` | `internal/serve/`: deletes `hub*`, reverts `ListRunEvents`, and removes SSE route from mux |
| 3 | Worktree `.lucind/lane.log` writing, stream flushes before terminal `SetStatus`, log archive in `PersistEnvelope`, and CLI dispatch wiring | `internal/run/run.go`<br>`internal/run/run_test.go`<br>`cmd/lucind-ai/cli.go`<br>`cmd/lucind-ai/cli_test.go` | `agy` | `internal/run/` and `cmd/lucind-ai/`: restores pre-telemetry run loop and CLI defaults |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1, Unit 2 | Yes (2 lanes) | Yes: Unit 1 adds nil-safe optional sinks and passes executor tests; Unit 2 adds additive hub and shell-free DTOs passing serve tests; neither modifies callers in `run.go` or `cli.go`. |
| 2 | Unit 3 | No (1 lane) | Yes: Integrates executor sinks and serve hub into `run.Execute` and `cmd/lucind-ai`; full `lucind-checks.sh` and integration tests pass. |

## Disjointness Check

- **Wave 1 (Unit 1 ⟷ Unit 2)**: Unit 1 (`internal/executor/*`) vs Unit 2 (`internal/serve/*`). No path in Unit 1 shares a prefix with any path in Unit 2 under `packet.PathInScope` (`internal/executor/` ∩ `internal/serve/` = ∅). **Verdict: Pairwise disjoint.**
- **Wave 2 (Unit 3)**: Wave of one unit needs no check.

## Sidecar Recommendation

**Recommendation**: Single packet, no sidecar
**Rationale**: While Units 1 and 2 are structurally independent and pairwise disjoint, the entire change spans ~350–500 lines across 9 production files and is strictly additive. Splitting into an `apply-dag.yaml` sidecar with multi-wave dispatch overhead is not warranted for a change of this scale, following the precedent in `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` where a multi-unit change was applied as a single packet with sequential work-unit commits.

## Open Questions

- [ ] None.
