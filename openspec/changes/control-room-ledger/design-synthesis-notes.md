# Synthesis Notes: Control Room Ledger

## Unresolved Contradictions

Lens A Decision 6 and Lens C's test plan name `serve.Model` methods `ListRuns`, `GetRun`, and `GetLaneProgress`. Lens B (surface owner) names `GetRunSummary`, `ListRunSummaries`, and `GetLaneProgress`, with DTO structs `RunSummary` and `ProgressChunk`.

Evidence: none of these symbols exist. `internal/serve/model.go:14-25` is `Model` / `NewModel` (feature-parent DTOs). `ListFeatures` starts at `:128`. Code does not settle the identifiers. Canonical `design.md` specifies typed run-summary and progress-tail methods on `Model` and does not pick names. `*Ledger` method names (`RegisterRun`, `UpdateRunStatus`, `GetRun`, `ListRuns`, `AppendProgress`, `GetProgressAfter`, `PruneProgress`) agree across A and B.

## Coverage Gaps

- `openspec/changes/control-room-ledger/specs/` is absent. Packet preconditions said accepted specs were present. Delta requirements used here are the proposal's `## Delta specifications` (run-lifecycle-ledger, lane dispatch metadata, progress ingest, isolated prune, shell-free DTOs, primary-root isolation).
- Proposal open question — emit run status via `AppendEvent` (`internal/ledger/ledger.go:366-381`) with `run_status_changed`, or dedicated helpers only — was not carried by any design lens. Schema widening is in; dual-write vs helper-only is not decided.
- Exact SQL column types (`TEXT` vs `INTEGER`) and a full `runs.status` enum beyond `running` / terminal were not sized by any design lens.
- gentle-ai `sdd-design` 800-word budget is superseded by this packet's 1800 (matches `openspec/changes/archive/`). Skill-required sections (Choice / Alternatives / Rationale, Interfaces / Contracts, threat-matrix applicability with explicit `N/A: reason`) are in `design.md`. Skill Step 4 Engram persistence ran; Skill Step 5 orchestrator return block is superseded by `.lucind/result.json`.

## Dropped Citations

Every item was opened in this worktree. The claim was removed from `design.md` or rewritten without the failed citation.

1. **`internal/run/run.go:422-435` as the progress writer (A Decision 3).** Those lines append a completion `lane_note` after `decideStatus`, once the executor has returned. Canonical doc cites them only as diagnostics that remain; mid-flight ingest is a new `AppendProgress` API for `control-room-capture`.

2. **`serve.Model.ListRuns` / `GetRun` at `internal/serve/model.go:14-25` (A Decision 2/6, C test plan).** `Model` / `NewModel` only. Shell-free contract kept at `:14-25`; query analog is `ListFeatures` `:128-149`.

3. **`internal/serve/handlers.go:120-146` as already querying those run methods (A).** `serveStateJSON` builds `ServerState` with pending approvals and `ApproverRate` only (`:16-21`). Kept as the handler to extend; `/api/state` today is approvals-only (`:79-85`).

4. **`internal/run/run.go:480-483` as `runs.status` becoming terminal (B Hop 4).** `deps.Ledger.SetStatus` for the lane. Canonical flow uses A's site: `UpdateRunStatus` after `ExecuteBatch` returns (`cmd/lucind-ai/cli.go:304-311`).

5. **`PruneProgress` at `internal/ledger/ledger.go:877-890` (B Hop 5).** That range is `PruneIntegrationEvents`. Canonical doc uses it only as the analog.

6. **AppendProgress "without blocking lease renewals" at `internal/run/attempt.go:434-441` (B Hop 3).** Those lines set `renewInterval` and `startLeaseRenewal` during feature checks. They are the contention neighbor, not proof that progress writes will not wait. WAL/`busy_timeout` facts kept (`internal/ledger/ledger.go:162-185`).

7. **`serve.Model.GetRunSummary` at `internal/serve/model.go:128` (B file changes).** `ListFeatures`.

8. **`internal/run/run.go:390-483` as the `RegisterLane` / `lanes_meta.go` consumer (B).** `:390` is `persistCtx` after the executor returns. `RegisterLane` is `internal/run/run.go:327` (and never-started `internal/run/batch.go:184`).

9. **`internal/run/run.go:420-500` as the progress ingest / capture store (B).** Completion `lane_note` plus lane `SetStatus`. Not `lane_progress`.

10. **`.lucind/result.schema.json:1-160` as the in-repo result schema (C).** File exists here as the dispatch envelope copy. In-repo source of truth is `internal/result/result.schema.json`. Unchanged-schema claim kept with the in-repo path.

11. **`depsFactory` defined at `cmd/lucind-ai/cli.go:292` (C).** That line *calls* it. Definition is `cli.go:58-60`. Injection in tests starts at `cli_test.go:1074`, not `:1078` (which is a `CreateWorktree` mock body).

12. **`internal/ledger/ledger_test.go:368` as the concurrency analog (C).** `TestConcurrentRegisterAndSetStatusAcrossDistinctLanes` starts at `:367`.

## Architecture Divergence

All three assumed Candidate 1 (relational schema expansion with modular domain files): schema 5→6 via transactional `migrate`, `runs` + `lane_progress`, nullable `lanes.model`/`agent`/`feature`, `events` CHECK widened for `run_status_changed`, domain files sharing `*Ledger`, shell-free `serve.Model` DTOs, `RegisterRun` at CLI dispatch, WAL / `busy_timeout=5000` / `MaxOpenConns=4`, primary-root isolation. Independent convergence; A's architecture is not refuted by verified code.

What B assumed that did not survive A (content omitted from `design.md` except as corrected citations):

- **Run-terminal write site.** B Hop 4 placed `runs.status` transition at lane `SetStatus` (`internal/run/run.go:480-483`). A Decision 2 places it after `ExecuteBatch` at `cmd/lucind-ai/cli.go:304-311`. Design follows A. Lane `SetStatus` remains the lane write.
- **Model method names.** B's `GetRunSummary` / `ListRunSummaries` vs A's `GetRun` / `ListRuns`. Escalated above rather than silently adopting A inside B's surface slice.
- **Progress writer location.** B treated `run.go:420-500` as the ingest store. A named `run.go:422-435` as terminal consumer for progress. Both citations are post-process notes; neither survives as a live stream writer.

Lens C did not own architecture. It corroborated A's constraints (STRICT rebuilds, WAL writer, worktree-vs-primary ledger, closed event types, domain-file split, git-revert rollback with no downgrade script) and is compatible with Candidate 1.
