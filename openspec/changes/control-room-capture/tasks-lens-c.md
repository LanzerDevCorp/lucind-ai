# Tasks Lens C — Proof & Review Burden: Control Room Capture

## Assumed decomposition

The change is decomposed into 4 sequential deliverables: Unit 1 introduces destination `io.Writer` fields on `executor.Request` and `ledgerpath.ResolveLog` path resolution/validation under `<primaryRoot>/.lucind/`; Unit 2 implements `io.MultiWriter` stdout/stderr teeing across `agy`, `cursor-agent`, and `opencode` with non-interfering `WaitDelay` pipe draining; Unit 3 integrates continuous spooling in `run.Execute`, exposes `Report.LogPath`, and enforces 4096-byte bounds on ledger failure diagnostics; Unit 4 implements loopback HTTP SSE tailing (`/api/runs/{runID}/lanes/{laneID}/tail`), transcript download (`/api/runs/{runID}/lanes/{laneID}/log`), Model JSON route wiring, and CLI `primaryRoot` plumbing in `lucind-ai serve`. The critical path runs sequentially from Unit 1 types and path resolution through Unit 2 executor stdio teeing and Unit 3 runner spooling to Unit 4 loopback HTTP endpoints.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | 550–700 lines (~240 production across 8 files, ~390 test across 6 test files) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Foundation & Executors: Unit 1 + Unit 2) → PR 2 (Runner Spooling & Diagnostics: Unit 3) → PR 3 (Serve SSE Tail & HTTP Endpoints: Unit 4) |
| Delivery strategy | ask-on-risk |
| Chain strategy | feature-branch-chain |

Basis: Counted modified production files (`internal/executor/executor.go` ~10 lines, `internal/executor/agy.go` ~15 lines, `internal/executor/cursor_agent.go` ~15 lines, `internal/executor/opencode.go` ~15 lines, `internal/ledgerpath/ledgerpath.go` ~20 lines, `internal/run/run.go` ~35 lines, `internal/serve/handlers.go` ~120 lines, `cmd/lucind-ai/cli.go` ~10 lines = ~240 lines) and corresponding unit/integration test additions (`ledgerpath_test.go` ~40 lines, `agy_test.go` + `cursor_agent_test.go` + `opencode_test.go` ~120 lines, `run_test.go` + `integrate_test.go` ~80 lines, `server_test.go` ~150 lines = ~390 lines). Total 550–700 lines exceeds the 400-line budget, warranting a 3-PR feature branch chain.

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A: does not classify or execute documentation files | None | N/A: boundary not touched | None |
| Git repository selection | Applicable | `TestResolveLogAndValidateRejectWorktrees` & `TestExecuteCreatesLogsUnderPrimaryRootOnly` | Reproduces candidate log destination inside worktree path (`<repo>-worktrees/lane-1/.lucind/...`) or escaping via traversal (`../`); asserts `Validate` returns `ErrLedgerOutsidePrimaryRepo` and `Execute` creates spool file exclusively under `deps.PrimaryRoot/.lucind/`, with zero log files written inside `wt.Path`. | Unit 1 `ledgerpath.ResolveLog` & Unit 3 `run.Execute` log initialization |
| Commit state | N/A: does not create commits or touch the index | None | N/A: boundary not touched | None |
| Push state | N/A: no push | None | N/A: boundary not touched | None |
| PR commands | N/A: no PR argv | None | N/A: boundary not touched | None |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Unit 1: Foundation & Path Resolution (`internal/executor`, `internal/ledgerpath`) | `go test -run 'TestResolve|TestValidate' ./internal/ledgerpath` (derived from `internal/ledgerpath/ledgerpath_test.go:9,37`) | `ResolveLog` computes `<primaryRoot>/.lucind/runs/<runID>/lanes/<laneID>.log`; `Validate` rejects worktree-shaped or traversal paths with `ErrLedgerOutsidePrimaryRepo`. | Does not prove filesystem directory creation or disk write permissions. |
| Unit 2: Executor MultiWriter Teeing (`internal/executor`) | `go test -run 'Test(Agy|CursorAgent|Opencode)Run(CapturesStdout|GrandchildHoldingPipes)' ./internal/executor` (derived from `internal/executor/agy_test.go:166,479`, `internal/executor/cursor_agent_test.go:95,178`, `internal/executor/opencode_test.go:95,178`) | Child stdout and stderr tee concurrently to destination `io.Writer` when non-nil; `WaitDelay` pipe timeout sets `OutputTruncated: true` without failing exit code 0. | Does not prove live upstream binary execution without stubs (`agy`, `cursor-agent`, `opencode`). |
| Unit 3: Runner Spooling & Bounded Diagnostics (`internal/run`) | `go test -run 'TestExecute(OversizedStderr|SuccessfulLane|TruncatedOutcome)' ./internal/run` (derived from `internal/run/run_test.go:651,856,1106` and `internal/run/integrate_test.go:392`) | `Execute` opens continuous log under primary `.lucind/` before child spawn; log persists after worktree removal; ledger failure note capped at 4096 bytes per stream; clean `Done` writes no failure note. | Does not prove disk quota exhaustion handling or filesystem full failure recovery. |
| Unit 4: Serve SSE Tail & HTTP Endpoints (`internal/serve`, `cmd/lucind-ai`) | `go test -run 'Test(NonLoopbackListen|SingleApproval)' ./internal/serve` (derived from `internal/serve/server_test.go:17,42,136` and `internal/serve/model_test.go:1`) | Loopback enforcement on listener; `/api/runs/.../tail` streams SSE appends; transcript download returns 200 with full log or 404 when absent; client disconnect ends stream cleanly. | Does not prove browser EventSource reconnection semantics or external reverse proxy behavior. |

## Verification Gaps

- Live CLI execution with vendor binaries: Executor tests use shell subprocess stubs (`writeStub`, `writeCursorStub`, `writeOpencodeStub`) rather than real proprietary agent binaries (`agy`, `cursor-agent`, `opencode`) to avoid network quota usage in CI; end-to-end verification requires manual smoke tests with real CLI tools.
- Real-time SSE tailing during rapid continuous writes: Unit tests verify chunk delivery via `httptest`, but testing high-frequency write contention with concurrent client disconnects requires an active integration harness with a live streaming subprocess.

## Open Questions

- [ ] Path convention: confirm whether `ResolveLog` targets `.lucind/runs/<run_id>/lanes/<lane_id>.log` vs `.lucind/logs/<run_id>/<lane_id>.log`.
- [ ] Stream layout: confirm single interleaved log stream vs separate `.stdout.log` and `.stderr.log` files.
- [ ] Retention policy: confirm whether run logs are archived with results envelopes or left gitignored under `.lucind/`.
