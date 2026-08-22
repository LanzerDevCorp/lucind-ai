# Synthesis Notes: Control Room Ledger

## Unresolved Contradictions

Lens A selects Candidate 1, which stores mid-flight progress in SQLite `lane_progress` on the same primary-root database as leases and `SetStatus`. Lens C rates **SQLite writer contention under progress appends and lease renewal** as High, citing WAL/`busy_timeout` (`internal/ledger/ledger.go:162-185`), lease renew (`internal/run/attempt.go:434-441`), and `ValidateLease` (`internal/run/attempt.go:482-488`).

The code confirms WAL, `busy_timeout=5000`, and `MaxOpenConns=4`. It does not measure ingest-versus-lease latency or prove the pool will absorb high-Hz `AppendProgress` without `SQLITE_BUSY`. This synthesis keeps Candidate 1 as the proposed *shape* (lens A owns approach) and keeps C's risk plus Spike 1. It does **not** declare high-Hz ingest cheap, and it does **not** switch to a split telemetry store.

## Coverage Gaps

None of the packet's nine proposal-spine items were missing from the drafts.

Not a spine gap: no draft sized exact v6 SQL types, Go method signatures, or run-status enum values beyond A's sketch (`pending`, `running`, `done`, `failed`, `released`). That is design. The gentle-ai `sdd-propose` skill's 450-word budget is superseded by this packet's 1800-word budget; required skill sections (capabilities, rollback, success criteria, lane-lifecycle call sites) are covered inside the spine.

## Dropped Citations

Every item below was opened in this worktree. The claim was removed from `proposal.md` (or rewritten without the failed citation).

1. **`internal/executor/executor.go:42-61` as mid-flight streaming from executors (A).** Those lines define `Outcome` stdout/stderr captured after the process ends, plus truncation. Canonical doc cites them only as end-of-run capture; `lane_progress` is new.

2. **`internal/serve/model.go:128-149` as Control Room run DTO querying (A).** That range is `ListFeatures`. Canonical doc cites it only as the analog to extend; shell-free contract is `internal/serve/model.go:14-25`.

3. **`~/.claude/skills/sdd-propose/SKILL.md:92-158` as a product open question (A, C).** Outside this repository; process topology, not ledger behavior. Omitted from proposal open questions.

4. **`internal/ledger/schema.go:171-180` as the progress-pruning seam (B).** That range is `integration_events` DDL and `idx_integration_events_feature`. Prune analog is `internal/ledger/ledger.go:877-890`.

5. **`PruneProgress` at `internal/ledger/ledger.go:877-890` (B scenario).** That range is `PruneIntegrationEvents`. Canonical doc uses it only as the analog.

6. **`Model.GetRunSummary` at `internal/serve/model.go:14-25` (B).** Those lines are `Model` / `NewModel` (feature-parent DTOs). No `GetRunSummary`. Requirement kept as new methods on `Model`.

7. **`internal/run/run.go:422-435` as incremental stdout/stderr ingest (B capability row).** Those lines append a completion `lane_note`. Canonical doc cites them as the diagnostic-only path being supplemented, not as a live stream.

8. **Concurrent progress appends "without writer starvation or deadlocks" at `internal/ledger/ledger.go:162-185` (B).** That range sets WAL, `busy_timeout=5000`, and `MaxOpenConns=4`. It does not eliminate the SQLite write lock. Outcome dropped; WAL facts kept.

9. **`.lucind/result.schema.json:1-160` unchanged (C).** That path is the dispatched envelope schema under gitignored `.lucind/`. The in-repo schema is `internal/result/result.schema.json`. Packet-unchanged claim kept via `internal/packet/packet.go:33-75` only.

10. **`internal/ledger/ledger_test.go:335-375` as concurrent `AppendProgress` + `ValidateLease` (C).** Lines 335–349 finish `TestSetStatusUpdatesLaneAndAppendsEventsInOrder`; 367 starts `TestConcurrentRegisterAndSetStatusAcrossDistinctLanes`. No progress API, no lease renew. Analog cited at 367 only.

11. **`internal/ledger/ledger_test.go:490-530` as `GetProgressAfter` (C).** `TestSetWorktreePreservedOnUnknownLaneErrors` and `TestLaneStatesFeedsBarrierEvaluate`. No progress cursor.

12. **`internal/serve/model_test.go:45-85` as progress tailing (C).** Overlap-evidence helper and start of `TestStatusRoundTripFromWriteAPIs` (feature DTOs).

13. **`internal/ledger/ledger_test.go:875-920` as `PruneProgress` (C).** `TestMigrateV2DatabaseCreatesApprovalsTableAndPreservesRows`. Real prune test: `TestPruneIntegrationEventsRetention` at `internal/ledger/ledger_test.go:1584`.

14. **`internal/serve/model_test.go:12-65` as `GetRunSummary` / `ListRuns` / `GetLaneProgress` (C).** Imports and helpers. Shell-free test is `TestModelSourceDoesNotShellOut` at `internal/serve/model_test.go:595`.

15. **`internal/serve/handlers_test.go:80-140` and `:160-210` (C).** File does not exist. Handler tests live in `internal/serve/server_test.go` (bulk-reject at line 42).

16. **`cmd/lucind-ai/cli_test.go:210-250` as linked-worktree refusal (C).** `TestRunKnownModelForExecutorPasses` / `TestRunOmittedModelSkipsModelCheck`. CLI worktree gates are `cmd/lucind-ai/cli.go:277-280,702-705`; no matching test at 210–250.

17. **`internal/ledgerpath/ledgerpath_test.go:15-55` as CLI exit-1 worktree refusal (C).** `TestResolve` and the start of `TestValidate` (path math). Does not exercise `lucind-ai run`/`serve`. Canonical doc cites `TestResolve` at line 9 for path derivation only.

## Scope Divergence

**Lens B** treated future APIs (`RegisterRun`, `AppendProgress`, `GetProgressAfter`, `PruneProgress`, `GetRunSummary`) as if they already sat at current line numbers. That cost several scenarios (see Dropped Citations). Independently B still wanted schema v6, WAL ingest, shell-free DTOs, prune-without-erasing-history, and primary-root isolation — same boundary as A.

**Lens B** listed `progress-stream-pruning` as a first-class added capability. **Lens A** left automated retention as an open question. Not a candidate contradiction: canonical proposal includes a prune API modeled on `PruneIntegrationEvents` (explore already required it; C tests it) and keeps trigger/cutoff as open questions.

**Lens B/C** asked whether chunks are raw strings or structured JSON. **Lens A** sketched `message` TEXT. Canonical doc keeps A's column sketch and leaves payload shape as an open question.

**Lens C** did not own candidate selection. It corroborated A's constraints: STRICT rebuilds, WAL writer, worktree-vs-primary ledger, closed event types, domain-file split for `DisjointAllowedPaths`. Its rollback (git revert; no downgrade script) is compatible with Candidate 1. In-process pub/sub is not in C's propose draft as a competing store; Candidate 3 (in-memory *instead of* SQLite) remains rejected.

**Convergence:** all three treat `internal/ledger` SQLite as the durable cross-process seam, schema v6 as the vehicle, high-frequency progress as isolated from lifecycle rows, primary-root isolation as preserved, governance APIs as unchanged, and UI/HTTP/capture/telemetry-algorithm work as other changes.
