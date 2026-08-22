# Tasks Synthesis Notes: Control Room Telemetry

## Unresolved Contradictions

None.

## Coverage Gaps

- **Wave merge (step 5).** Lens B Wave 1 ran Unit 1 ∥ Unit 2 with `NewHandler(*Hub)` inside Unit 2. Combined-tree `lucind-checks.sh` is `CGO_ENABLED=0 go build ./...` then `go test ./... -race -count=1`. Production caller `serve.NewHandler(ledg, *approver, opencodeCmd)` is `cmd/lucind-ai/cli.go:715`; tests at `internal/serve/server_test.go:70` are in Unit 2, but `cli.go` is not. Wave 1 would fail `go build`. `handlers.go` / SSE / `cli.go:715` moved into Unit 3 with `PersistEnvelope`. After the merge, a hypothetical Wave 1 of Unit 1 ∥ Unit 2 (Hub + `ListRunEvents` only, no signature change) would build: extra `Request` fields are nil-safe, `NewHandler` stays three-arg. Sidecar still not recommended; the shipped shape is one packet, three sequential commits.

- **sdd-tasks High-path not taken.** Skill maps `single-pr` + likely-over-400 to `Chained PRs recommended: Yes` and `Decision needed before apply: Yes` (`size:exception`). Lens C owns the forecast and marked Medium / Chained No / Decision No on a 350–550 range that only straddles 400. Canonical `tasks.md` keeps C’s values. Drift: if apply lands above 400 lines, the orchestrator still needs a size exception the forecast did not pre-ask.

- **Lens B `allowed_paths` omitted test files A required.** Unit 3 now includes `internal/ledger/ledger_test.go` (task 4.4) and `internal/run/batch_test.go` (task 4.5). Production `batch.go` is unchanged (`Observe` already follows `Execute`, `batch.go:147-153`).

## Dropped Citations

- **`internal/ledger/ledger_test.go:733` as CHECK-rejects-stream-types (A 4.4, C row 4).** That function is `TestMigrateIsIdempotent` (reopen, preserve `schema_migrations.applied_at`). Design already says it is migration-only (`design.md:100`). CHECK is `schema.go:38-39`. No stream-type rejection test exists. Task 4.4 is a new test; `TestLedgerSchemaRejectsStreamEventTypes` is a name to write, not a command that matches today. A `-run` of only that name would pass with zero tests.

- **`internal/run/batch_test.go:66-113` (A 4.5) and `:61-120` (C row 6) as barrier-after-SetStatus tests.** Those lines are `batchFakeExecutor`. Production `ExecuteBatch` is `internal/run/batch.go:66-113`. Barrier tests start at `TestExecuteBatchBarrierStaysIdleWhileOneLaneWaitsForApproval` (`batch_test.go:1038`). `barrier_test.go:31-60` is `TestEvaluate` — kept as the Evaluate seam, not as the missing batch test.

- **`cmd/lucind-ai/cli_test.go:37-60` as PersistEnvelope archive / `serveDispatch` Hub wiring (C row 7).** Those tests are `TestRunNoArgsPrintsUsageAndFails` and `TestRunUnknownSubcommandPrintsUsageAndFails`. Closest persist coverage is `TestRunDispatchPersistsIntegratedLaneEnvelopeToPrimaryRoot` (`cli_test.go:1154`), JSON envelope only.

- **C `-run 'TestExecute(WritesWorktreeLog|TruncatesDiagnosticNote|FlushesBeforeTerminalStatus)'`.** Matches zero tests (`go test -run` would still exit 0). Closest existing: `TestExecuteWritesResultSchemaIntoWorktree` (`run_test.go:316`), `TestExecuteOversizedStderrLedgerNoteIsBoundedAndKeepsTail` (`:856`). New names belong in 4.5, not in a proving command that is already green.

- **C `-run 'TestExecuteBatch(BarrierObservesPersistedTerminal|ConcurrentExecution)'`.** Matches zero tests. Closest: `TestExecuteBatchRunsLanesConcurrentlyNotSequentially` (`batch_test.go:530`), `TestExecuteBatchBarrierStaysIdleWhileOneLaneWaitsForApproval` (`:1038`).

- **C `-run 'Test(PersistEnvelopeArchivesLog|ServeDispatchWired)'`.** Matches zero tests.

- **A 4.3: `model_test.go:595-627` tests `ListRunEvents` mapping.** That range is only `TestModelSourceDoesNotShellOut` (AST ban on `os/exec` / `os` / git). `ListRunEvents` does not exist. Keep the AST assertion; mapping is a new `TestModelListRunEvents`.

## Decomposition Divergence

Lens A (authoritative): four phases — foundation (Request writers, Hub, `ListRunEvents`), executor teeing, run/server/CLI wiring, tests. Critical path `executor.go` → three runners → `run.go` → `cli.go`.

Lens B and C independently converged on the same three file-groups A already implied (executor / serve / run+CLI), with Units 1 and 2 parallel and Unit 3 integrating both. That corroborates the partition, not a rival checklist. Content not copied from B/C as extra tasks: C’s seven acceptance *rows* map onto A’s 1.x–4.x; they are not a fourth phase. C’s named tests that do not exist were not added as phantom proving commands (see Dropped Citations).

B placed `handlers.go` in Unit 2; A’s dependency table already had 3.1 after 1.2 and 3.3 after 3.1 — A never claimed that signature change could compile without `cli.go`. After the wave merge, Unit 2 is Hub + `ListRunEvents` only, which is a subset of A’s Phase 1, not a different decomposition.

None of A’s production files is missing or ordered backwards. The only A task rewritten for code (not decomposition) is 4.4: new CHECK test at `schema.go:38-39` instead of modifying `:733`.
