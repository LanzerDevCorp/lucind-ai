# Tasks: Control Room Capture

Spool child stdio to append-only files under `<primaryRoot>/.lucind/`, keep SQLite notes at `streamDetailCap`, and expose loopback SSE tail, transcript download, and `serve.NewModel` JSON. Dest `io.Writer` field names on `Request` are unresolved — add the fields; do not invent identifiers.

**No `apply-dag.yaml` sidecar.** Single packet, four sequential work-unit commits. Chained PRs below are a review split, not an apply DAG.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 550–700 (~240 production across 8 files, ~390 test) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Units 1–2) → PR 2 (Unit 3) → PR 3 (Unit 4) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

`feature-branch-chain` bases: PR 1 → tracker; PR 2 → PR 1; PR 3 → PR 2.

### Suggested Work Units

| Unit | Goal | Likely PR | Executor | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------|----------------------|-----------------|-------------------|
| 1 | Dest writers on `Request`; `ResolveLog` + tests | PR 1 | agy | `go test -run TestResolveLog ./internal/ledgerpath` | N/A: pure path logic | `internal/executor/executor.go`; `internal/ledgerpath/ledgerpath.go`; `ledgerpath_test.go` |
| 2 | `io.MultiWriter` tee on agy, cursor-agent, opencode | PR 1 | agy | `go test -run TeeDestWriters ./internal/executor` | N/A: `writeStub` / sibling stubs, not vendor CLIs | `internal/executor/agy.go`, `cursor_agent.go`, `opencode.go` + their `_test.go` |
| 3 | Open primary log in `Execute`; `Report.LogPath`; keep 4096-byte notes | PR 2 | cursor-agent | `go test -run TestExecuteCreatesLogsUnderPrimaryRootOnly ./internal/run` | N/A: `fakeExecutor` + temp primary root | `internal/run/run.go`; new cases in `run_test.go` / `integrate_test.go` |
| 4 | SSE tail, log download, Model JSON, `primaryRoot` into `NewHandler` | PR 3 | cursor-agent | `go test -run TestLane ./internal/serve` | N/A: `httptest`, not a browser EventSource | `internal/serve/handlers.go`; `server_test.go` (and/or new `handlers_test.go`); `cmd/lucind-ai/cli.go:715` |

Same-wave pairs: none (sequential commits, one Integrate). Hypothetical B waves stay prefix-disjoint (`internal/ledgerpath` vs `internal/executor`; `internal/run` vs `internal/serve` + `cmd/lucind-ai`) and would be green after the prior wave if RED+GREEN stay in one unit. Not shipped.

## Dependency order

1.1 ∥ 1.2 → 1.3. 2.1–2.3 need 1.1. 2.4 with 2.1–2.3 in Unit 2. 3.1–3.3 need 1.1, 1.3, 2.1–2.3. 4.x need 1.3; 4.1 signature + 4.3 CLI + `server_test.go` call sites are one GREEN (`go build` otherwise fails). 4.2 needs 4.1.

## Threat-matrix RED tests

Applicable row only: **Git repository selection**. N/A (no RED tasks): Documentation-like paths, Commit state, Push state, PR commands.

- [ ] **RED before 1.3:** `TestResolveLog` — `ResolveLog` under `<primaryRoot>/.lucind/`; `Validate` rejects a worktree-shaped candidate (`ErrLedgerOutsidePrimaryRepo`). Fails today: `Resolve` returns `lucind.db` only (`ledgerpath.go:34-38`).
- [ ] **RED before 3.2:** `TestExecuteCreatesLogsUnderPrimaryRootOnly` — spool under `deps.PrimaryRoot/.lucind/`, zero files in `wt.Path`. Fails today: `Execute` builds `Request` with no dests (`run.go:368-374`).

## Phase 1: Foundation & path resolution

- [ ] 1.1 Add stdout/stderr destination `io.Writer` fields to `Request` (`internal/executor/executor.go:15-37`). Nil keeps today’s capture. Do not reuse `Outcome.Stdout` / `Stderr` names.
- [ ] 1.2 RED `TestResolveLog` in `internal/ledgerpath/ledgerpath_test.go` (extend `:9-84`). Existing `TestValidate` already rejects worktree `lucind.db` (`:56-58`) and accepts nested `.lucind/` (`:51-53`).
- [ ] 1.3 GREEN `ResolveLog(primaryRoot, runID, laneID string) string` beside `Resolve`. Do not change `Validate` (`:40-58`). Prefix `runs/` vs `logs/` is an open question — pick one and use it everywhere.

## Phase 2: Executor MultiWriter

Strict TDD inside Unit 2: dest-writer tests fail until 2.1–2.3.

- [ ] 2.1 Wrap stdout/stderr with `io.MultiWriter` when dests are non-nil (`internal/executor/agy.go:169-173`). Keep WaitDelay (`:160-168,182-197`).
- [ ] 2.2 Same (`internal/executor/cursor_agent.go:91-95`; WaitDelay `:82-90,104-118`).
- [ ] 2.3 Same (`internal/executor/opencode.go:130-134`; WaitDelay `:121-129,143-160`).
- [ ] 2.4 Dest-writer tests named `TestRunTeeDestWriters`, `TestCursorAgentRunTeeDestWriters`, `TestOpencodeRunTeeDestWriters` via `writeStub` / `writeCursorStub` / `writeOpencodeStub` + `bytes.Buffer`. WaitDelay regression: `OutputTruncated: true`, exit 0 (`agy_test.go:166`, `cursor_agent_test.go:178`, `opencode_test.go:178`).

## Phase 3: Primary-root spooling

- [ ] 3.1 RED `TestExecuteCreatesLogsUnderPrimaryRootOnly` in `internal/run/run_test.go`.
- [ ] 3.2 GREEN: `ResolveLog`, create the file before spawn, set dests, set `Report.LogPath` (`run.go:220-258,368-374,501-508`).
- [ ] 3.3 Keep `streamDetailCap = 4096` (`run.go:89,132-144`); no failure note on clean Done (`:416-435`); `EventLaneNote` on `OutputTruncated` without failing Done (`:488-508`). Regression: `TestExecuteOversizedStderrLedgerNoteIsBoundedAndKeepsTail` (`run_test.go:856`), `TestExecuteSuccessfulLaneReportsNoDiagnosisOrLedgerNote` (`:1106`). Add log survival across `completeIntegration` worktree remove (`integrate.go:159-163`).

## Phase 4: Loopback HTTP & CLI

Strict TDD inside Unit 4: 4.4 tests fail until 4.1–4.2. `NewHandler` signature, `cli.go:715`, and `server_test.go` call sites are one GREEN.

- [ ] 4.1 GREEN `NewHandler(..., primaryRoot string)` and `/api/model/...` over `serve.NewModel` (`handlers.go:36-118`; `model.go:22`). Same commit: `serveDispatch` (`cli.go:715`) and every 3-arg `NewHandler` in `server_test.go`.
- [ ] 4.2 GREEN `GET /api/runs/{runID}/lanes/{laneID}/tail` (SSE/`http.Flusher`, end on `r.Context().Done()`) and `GET .../log` (`ServeContent`, 404 if missing). Locate files with `ResolveLog`. Do not write into the child’s pipes.
- [ ] 4.3 (same unit as 4.1) Pass resolved `primaryRoot` into `NewHandler` (`cli.go:674-725`).
- [ ] 4.4 RED then GREEN HTTP tests `TestLaneTail` and `TestLaneLog`: live tail, disconnect, 200/404 download, Model JSON. Mux today is `/`, `/api/state`, `/approvals/` only (`handlers.go:36-117`). Place in `server_test.go` or new `handlers_test.go`. `TestNonLoopbackListenFails` (`server_test.go:17`) stays loopback-only.

## Requirement traceability

| Requirement | Tasks |
|---|---|
| Continuous primary-root stream spooling | 1.1–1.3, 2.1–2.4, 3.1–3.3 |
| Bounded SQLite diagnostics | 3.3 (keep existing cap; do not re-implement) |
| Non-interfering WaitDelay drain | 2.1–2.4, 3.3 |
| Loopback HTTP stream access | 4.1–4.4 |

## Open questions (apply-time)

- Prefix: `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`?
- One interleaved file vs `.stdout.log` / `.stderr.log`?
- Retention: archive with packets/results, prune via worktree cleanup, or leave gitignored?
