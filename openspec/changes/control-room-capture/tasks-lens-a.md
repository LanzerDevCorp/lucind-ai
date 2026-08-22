# Tasks Lens A — Decomposition & Ordering: Control Room Capture

## Assumed decomposition

This decomposition breaks the change into 4 sequential phases: Phase 1 establishes foundational request types and pure path resolution for primary-root log files; Phase 2 implements `io.MultiWriter` stdout/stderr teeing and non-interfering `WaitDelay` handling across all three child executors (`agy`, `cursor-agent`, `opencode`); Phase 3 integrates continuous file spooling in `internal/run.Execute`, exposes `Report.LogPath`, and enforces 4096-byte bounds on ledger failure diagnostics; Phase 4 exposes read-only loopback HTTP endpoints for live SSE tailing, transcript downloads, and model queries, wiring `primaryRoot` through `lucind-ai serve`. The critical path runs from foundation types (1.1) and path resolution (1.2) through executor multi-writers (2.1–2.3) to runner spool integration (3.1) and verification (3.3).

## Phase 1: Foundation & Path Resolution

- [ ] 1.1 Add stdout and stderr destination `io.Writer` fields to `executor.Request` in `internal/executor/executor.go:15-37`.
- [ ] 1.2 Add `ResolveLog(primaryRoot, runID, laneID string) string` helper and update validation to ensure logs reside under `<primaryRoot>/.lucind/` in `internal/ledgerpath/ledgerpath.go:34-58`.
- [ ] 1.3 Add unit tests verifying `ResolveLog` path structure and `Validate` rejection of worktree-shaped destinations in `internal/ledgerpath/ledgerpath_test.go:9-84`.

## Phase 2: Executor MultiWriter & Stdio Capture

- [ ] 2.1 Wrap stdout and stderr with `io.MultiWriter` when destination writers on `Request` are non-nil in `internal/executor/agy.go:169-173`.
- [ ] 2.2 Wrap stdout and stderr with `io.MultiWriter` when destination writers on `Request` are non-nil in `internal/executor/cursor_agent.go:91-95`.
- [ ] 2.3 Wrap stdout and stderr with `io.MultiWriter` when destination writers on `Request` are non-nil in `internal/executor/opencode.go:130-134`.
- [ ] 2.4 Add unit tests for subprocess stub teeing to destination writers and `WaitDelay` pipe drain timeout preservation (`OutputTruncated: true`, exit code 0 preserved) in `internal/executor/agy_test.go:28-65`, `internal/executor/cursor_agent_test.go:18-50`, and `internal/executor/opencode_test.go:18-80`.

## Phase 3: Primary-Root Spooling & Bounded Diagnostics

- [ ] 3.1 In `internal/run/run.go:220-258,368-374`, resolve log destination via `ledgerpath.ResolveLog`, create log file under `<primaryRoot>/.lucind/` before child spawn, wire destination writers into `executor.Request`, and expose `LogPath` on `Report`.
- [ ] 3.2 In `internal/run/run.go:416-435,488-508`, cap failure notes at 4096 bytes per stream (`streamDetailCap`), write no failure note on clean `Done`, and append `EventLaneNote` on `OutputTruncated` without failing `Done`.
- [ ] 3.3 Add unit and integration tests verifying continuous log creation, worktree cleanup survival, bounded ledger notes, and unclipped log retention in `internal/run/run_test.go:850-900,1100-1128` and `internal/run/integrate_test.go:390-435`.

## Phase 4: Loopback HTTP Endpoints & CLI Wiring

- [ ] 4.1 Update `NewHandler` signature in `internal/serve/handlers.go:36-118` to accept `primaryRoot string` and wire read-only `/api/model/...` routes over `serve.NewModel`.
- [ ] 4.2 Implement live SSE tail endpoint `GET /api/runs/{runID}/lanes/{laneID}/tail` and transcript download endpoint `GET /api/runs/{runID}/lanes/{laneID}/log` (returning 404 on missing log) in `internal/serve/handlers.go:36-118`.
- [ ] 4.3 Update `serveDispatch` in `cmd/lucind-ai/cli.go:674-725` to pass resolved `primaryRoot` to `serve.NewHandler`.
- [ ] 4.4 Add HTTP tests for SSE live tail streaming, client disconnect handling, transcript download 200/404 responses, and model routes in `internal/serve/server_test.go:17-60`.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Independent foundational struct field addition on `executor.Request`. |
| 1.2 | — | Independent pure path calculation addition in `ledgerpath`. |
| 1.3 | 1.2 | Verifies path resolution and validation logic introduced in task 1.2. |
| 2.1 | 1.1 | Requires destination `io.Writer` fields on `executor.Request` to compile. |
| 2.2 | 1.1 | Requires destination `io.Writer` fields on `executor.Request` to compile. |
| 2.3 | 1.1 | Requires destination `io.Writer` fields on `executor.Request` to compile. |
| 2.4 | 2.1, 2.2, 2.3 | Tests verify `io.MultiWriter` teeing across all three executor implementations. |
| 3.1 | 1.1, 1.2, 2.1, 2.2, 2.3 | `Execute` resolves log path via `ledgerpath.ResolveLog` and supplies destination writers to `executor.Request`. |
| 3.2 | 1.1 | Depends on `executor.Outcome.OutputTruncated` and ledger event append contracts. |
| 3.3 | 3.1, 3.2 | Verifies end-to-end execution, primary-root spool survival, and bounded ledger notes. |
| 4.1 | — | Modifies `serve.NewHandler` signature and wires existing `serve.NewModel` independently. |
| 4.2 | 1.2, 4.1 | Routes require `ledgerpath.ResolveLog` to locate log files and mount onto `serve.NewHandler` mux. |
| 4.3 | 4.1 | Requires updated `serve.NewHandler` signature taking `primaryRoot` to compile. |
| 4.4 | 4.1, 4.2 | Verifies SSE tailing, transcript downloads, and model endpoints registered on `serve.NewHandler`. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| Continuous primary-root stream spooling | 1.1, 1.2, 1.3, 2.1, 2.2, 2.3, 2.4, 3.1, 3.3 |
| Bounded SQLite diagnostics | 3.2, 3.3 |
| Non-interfering WaitDelay drain | 1.1, 2.1, 2.2, 2.3, 2.4, 3.2, 3.3 |
| Loopback HTTP stream access | 4.1, 4.2, 4.3, 4.4 |

## Open Questions

- [ ] Exact spool path convention (`.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`) and stream interleaving (single combined log vs separate stdout/stderr files) remain open design details to be finalized during task 1.2 implementation.
- [ ] Log retention policy (archiving with packets/results vs automatic pruning via worktree cleanup vs leaving gitignored) remains open for operational policy decision.
- [ ] Precedence note: `sdd-tasks/SKILL.md` specifies a unified task artifact with workload forecast and work-unit partition tables; this lens strictly focuses on decomposition and ordering per the 3-lens parallel architecture.
