# Synthesis Notes: Control Room Ledger

## Unresolved Contradictions

Lens A rates Candidate 1 **high feasibility** because WAL, `busy_timeout=5000`, and the connection pool already exist (`internal/ledger/ledger.go:127-130,162-184`). Lens C rates **SQLite write-lock contention during high-throughput telemetry appends** as **High** severity, including against lease renewals (`internal/run/attempt.go:434-441`) and status updates (`internal/ledger/ledger.go:452-486`), and treats unified SQLite vs a split telemetry store as an open trade-off.

The code confirms WAL and the pool; it does not measure ingest-vs-lease latency. Spike 1 is the experiment. This synthesis keeps Candidate 1 as the recommended *shape* (A is primary on approaches) and keeps C's contention risk and spike; it does **not** declare high-Hz ingest cheap.

## Coverage Gaps

None of the packet's eight exploration-spine items were missing from the drafts.

Not a spine gap, recorded so the orchestrator does not expect them: no draft sized v6 column types, indexes, or Go APIs (that is design). No draft cited a Control Room PRD or sibling change specs as binding requirements. `internal/serve/handlers.go` `/api/state` does not use `internal/serve/model.go`; that split was not named as a product decision.

The gentle-ai `sdd-explore` headings Current State / Affected Areas / Approaches / Recommendation / Risks / Ready for Proposal are covered inside the packet spine in `explore.md`. The lucind-ai skill's archive-derived explore list ("Built versus convention", "Prior art", "The deciding question") was not used as a second spine: the packet's eight-item list is the done-criterion, and those archive headings describe a different change (`sdd-fan-out-lens`).

## Dropped Citations

Every item below was opened in this worktree. The claim was removed from `explore.md` (or rewritten without the failed citation).

1. **`internal/worktree/worktree.go:79-115` (A)** — cited for isolated worktree execution. Those lines are `BranchFor`, `CanonicalizeRef`, and `ValidateParentRef`. Isolation is `pathFor`/`Create` at `internal/worktree/worktree.go:150-171`. Canonical doc cites the latter.

2. **`sdd_phase`, `fanout_group`, `change_id`, DAG `wave` on `dag.Node` / `packet.Packet` (A, `internal/dag/parse.go:22-37`, `internal/packet/packet.go:33-75`)** — those fields are not on either struct. Present and unused by `RegisterLane`: `Model`, `Agent`, `Feature`. `DAG.Change` is on `dag.DAG` (`internal/dag/parse.go:41`), not `Node`. Wave is computed, not a node column. Canonical doc only persists fields that exist.

3. **`migrateV5ToV6DDL` at `internal/ledger/schema.go:10,224-306` (A)** — no such constant. `schemaVersion` is 5 at line 10. Lines 224-307 are existing `migrate` through v5. Canonical doc cites `migrate` as the extension point, not a v6 DDL that does not exist.

4. **`internal/ledger/ledger.go:1-1436` (A)** — file ends at 1435. Migrations live in `schema.go`. Claim that "all ledger operations" are that one range is false. Canonical doc cites `Ledger`/`Open` at 131-192.

5. **`internal/serve/model.go:446-516` as JSON serdes cost for Candidate 2 (A)** — those functions scan RFC3339 timestamps (`scanFeature`, `scanAttempt`, `scanLease`, `scanOverlap`), not a `metadata_json` column. JSON cost kept as a design argument without that citation.

6. **`Model.ListRunSummaries` at `internal/serve/model.go:128-149` (B)** — that range is `ListFeatures`. No `ListRunSummaries`. Scenario 3 rewritten against `ListFeatures` as a non-run analog plus `PendingApprovals` / `ListLeases`.

7. **WAL "without lock contention" at `internal/ledger/ledger.go:163` (B scenario 1)** — line 163 sets `journal_mode(WAL)` and `busy_timeout(5000)`. It does not eliminate the SQLite write lock. Outcome dropped; WAL facts kept.

8. **Dashboard as "a single efficient query" citing `model.go:128-149`, `schema.go:24-25`, `ledger.go:705-717`, `model.go:207-227` (B)** — those are three separate APIs (`ListFeatures`, `PendingApprovals`, `ListLeases`). Aggregate-in-one-query claim dropped.

9. **`PruneLogs` / `PruneTelemetry` at `internal/ledger/ledger.go:877-890` (B scenario 5 and success criteria)** — that range is `PruneIntegrationEvents`. Canonical doc uses it only as the prune analog (as lens A did).

10. **`internal/ledger/schema.go:224-307` as proof v6 already applies (B success criteria)** — `migrate` is idempotent through v5 only. Kept as the function v6 must hook, not as existing v6.

11. **`internal/ledger/schema.go:179` as the `events(run_id, id)` index (C)** — line 179 is `idx_integration_events_feature` on `integration_events`. Events index is `internal/ledger/schema.go:43`.

12. **`internal/ledger/ledger.go:893-925` as `Events(ctx, runID)` (C)** — that range is `IntegrationEvents` (feature-scoped). Unpaginated run events are `internal/ledger/ledger.go:490-520`.

13. **`internal/run/attempt.go:113-135` for operator overrides / fence monotonicity (C)** — `GetAttempt` / `getAttempt` SELECT. Fence on `feature_leases` is `internal/ledger/schema.go:122-129`. CAS lease check is `internal/run/attempt.go:482-488`. Operator-override API is not at 113-135.

14. **`internal/ledger/schema.go:224-255` as copy-and-rename DDL (C)** — those lines are `migrate` control flow (begin tx, bootstrap v1–v3). Rebuild DDL is `internal/ledger/schema.go:59-78` and `191-219`.

15. **`internal/ledger/ledger_test.go:300-350` as STRICT-migration-under-concurrency seam (C spike 4)** — `TestSetStatusUpdatesLaneAndAppendsEventsInOrder`. Spike 4 in canonical doc cites schema/migrate only.

16. **`internal/ledger/ledger.go:835-873` as a run-event / live-UI seam (C spike 2)** — `WriteWithAudit` requires `FeatureID` and inserts `integration_events`, not `events`. Spike 2 retargeted to `AppendEvent`.

## Approach Divergence

**Lens B** treated future APIs (`AppendLogChunk`, `GetLogsAfter`, `ListRunSummaries`, `PruneLogs`, `PruneTelemetry`) as if they already sat at current line numbers. That cost a run-dashboard scenario and a prune scenario that cited the wrong functions. Independently B still wanted schema v6, WAL ingest, shell-free DTOs, prune-without-erasing-history, and primary-root ledger isolation — same problem boundary as A.

**Lens B** indexed chunks by `(run_id, lane_id, offset)` and added telemetry spans/tokens; **lens A** proposed `lane_progress` with `seq`. Same store, different column names and a wider telemetry payload. Canonical doc keeps sequenced SQLite progress; span/token columns stay a design question (also out of scope as `control-room-telemetry`).

**Lens C** assumed live Control Room delivery might replace SQLite polling with in-process pub/sub, and that stdout might live in event rows *or* worktree files. A's Candidate 3 was in-memory *instead of* SQLite for progress — C's pub/sub is a cache/push layer *on* SQLite. Treating them as the same candidate would have killed C's spike; they are not the same. Canonical doc rejects Candidate 3 as the store and keeps Spike 2 as complementary.

**Lens C** did not own problem/candidates; it still corroborated A's constraints: STRICT rebuilds, WAL writer, worktree-vs-primary ledger, closed event types (via pagination/index pressure on `events`).

**Convergence:** all three treat `internal/ledger` SQLite as the durable cross-process seam, schema v6 as the vehicle, high-frequency progress as something to isolate from lifecycle rows, prune as required, and UI/HTTP/capture/telemetry algorithm work as other changes.
