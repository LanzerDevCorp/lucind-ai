# Tasks Synthesis Notes: Control Room Capture

## Unresolved Contradictions

**Destination `io.Writer` field identifiers on `executor.Request`.** Lens A task 1.1 says “stdout and stderr destination `io.Writer` fields” with no Go names. Lens B’s wave plan calls them `Request.Stdout` / `Request.Stderr`. Canonical `design.md` left names unset; design-lens B used `StdoutDest` / `StderrDest` and design-lens C used `StdoutWriter` / `StderrWriter`. `Request` today has no dest fields (`internal/executor/executor.go:15-37`); `Outcome` already has `Stdout` / `Stderr` (`:42-63`). The code does not settle the identifier. `tasks.md` 1.1 adds the fields and forbids reusing the Outcome names; it does not pick Dest vs Writer.

## Coverage Gaps

- **Waves merged for Integrate viability: none.** Lens B’s hypothetical Wave 1 (`internal/ledgerpath` ∥ `internal/executor`) and Wave 2 (`internal/run` ∥ `internal/serve` + `cmd/lucind-ai`) are component-prefix disjoint under `packet.PathInScope` and would be green after the prior wave if each unit keeps its own RED+GREEN. B recommended no sidecar; `tasks.md` ships a single packet. Unit 4’s `NewHandler` signature change, `cli.go:715`, and `server_test.go` call sites stay one unit so `go build` is green.
- **Design E2E CLI log test** (`design.md` Testing Strategy: new CLI tests; existing `cli_test.go:37-48` is usage-only). No Lens A task. Not invented.
- **Live vendor binaries and high-frequency SSE contention** (Lens C Verification Gaps). Not invented as tasks.
- **Skill size budget is 530 words; this packet’s budget is 1800.** Packet wins. Skill Engram persist / return block superseded by the two files plus `.lucind/result.json`.
- Forecast plain-text guard lines are present. Skill work-unit columns are populated; Executor is extra (packet spine 7; no DAG, names kept from B for a later split).

## Dropped Citations

- **Lens A 1.2 “update validation”.** `Validate` already rejects worktree-shaped paths and accepts nested `.lucind/` (`ledgerpath.go:40-58`; `ledgerpath_test.go:51-58`). Kept: add `ResolveLog` beside `Resolve` (`:34-38`).
- **Lens A 2.4 `agy_test.go:28-65` as tee/WaitDelay.** Those lines are `TestRunExitZero` / `TestRunNonZeroExitCode`. WaitDelay is `TestRunGrandchildHoldingPipesExitZeroReportsOutputTruncated` (`:166`). `TestRunCapturesStdout` is `:479` (in-memory capture, not dest tee).
- **Lens A 2.4 `cursor_agent_test.go:18-50`.** Stub helper + `TestCursorAgentRunExitZero`. WaitDelay is `:178`; stdout capture `:95`.
- **Lens A 2.4 `opencode_test.go:18-80`.** Stub + exit-code tests (`CapturesStderr` starts `:73`). WaitDelay is `:178`; stdout capture `:95`.
- **Lens A 3.2 as production work to cap notes.** `streamDetailCap = 4096`, diagnosis tails, no note on clean Done, and `OutputTruncated` `EventLaneNote` already exist (`run.go:89,132-144,416-435,488-508`). Kept as a do-not-regress constraint (task 3.3), not a rewrite.
- **Lens A 3.3 `integrate_test.go:390-435` as log survival.** `TestCompleteIntegrationPersistsEnvelopeForEveryIntegratedLane` (`:392`) persists envelopes; it does not assert log files. Survival seam is `completeIntegration` (`integrate.go:159-163`).
- **Lens A 4.4 `server_test.go:17-60` as SSE/download/model tests.** `:17` is `TestNonLoopbackListenFails`; `:42` is `TestBulkRequestBodyReturns400`. Mux is approvals-only (`handlers.go:36-117`).
- **Lens C proving `go test -run 'TestResolve|TestValidate' ./internal/ledgerpath`.** `TestResolve` (`:9`) and `TestValidate` (`:37`) pass today without `ResolveLog`.
- **Lens C proving `go test -run 'Test(Agy|CursorAgent|Opencode)Run(CapturesStdout|GrandchildHoldingPipes)' ./internal/executor`.** Agy tests are `TestRun*` not `TestAgyRun*` — the regex matches zero agy tests. Cursor/Opencode matches are existing capture/WaitDelay tests and would pass without dest teeing.
- **Lens C proving `go test -run 'TestExecute(OversizedStderr|SuccessfulLane|TruncatedOutcome)' ./internal/run` as spooling proof.** Hits `run_test.go:651,856,1106` (truncation / 4096 cap / no note on Done). Those pass today with no log file. `integrate_test.go:392` is not an `TestExecute*` name.
- **Lens C proving `go test -run 'Test(NonLoopbackListen|SingleApproval)' ./internal/serve` as tail/download/Model.** `TestNonLoopbackListenFails` (`server_test.go:17`) and `TestSingleApprovalAndDefectEndpoints` (`:136`) would pass today. `:42` is bulk-approval 400.
- **Lens C `internal/serve/model_test.go:1` as Model HTTP routes.** Line 1 is `package serve_test`. `NewModel` is `model.go:22`; `model_test.go` is SQL status/audit, not mux routes.

## Decomposition Divergence

Lens A’s four sequential phases are authoritative and are what `tasks.md` uses.

**Independent convergence:** Lens C assumed the same four sequential deliverables (Request fields + `ResolveLog` → MultiWriter tee → `Execute` spool + bounded notes → HTTP/CLI). Corroboration.

**Lens B regrouped, then declined to dispatch the regrouping.** B’s Unit 1 is A 1.2–1.3 only; B’s Unit 2 folds A 1.1 into the executor tee so Wave 1 can run `ledgerpath` ∥ `executor`. Compatible with A’s dependency table (1.1 ∥ 1.2). B then recommended no sidecar; sequential Units 1–4 in one packet follow A’s phases (1.1 stays in Phase 1). B’s `allowed_paths` extras with no A task: `internal/run/batch_test.go`, `cmd/lucind-ai/cli_test.go` (no `NewHandler` callers). New `handlers_test.go` is allowed as the placement for A 4.4. B’s line estimate (~400–500, “under review budget”) is superseded by Lens C’s forecast (C owns it).
