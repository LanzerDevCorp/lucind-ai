# Tasks Synthesis Notes: Control Room Ledger

## Unresolved Contradictions

None. The three `## Assumed decomposition` blocks agree on schema/domain → CLI+run writes ∥ serve Model reads. Lens B's optional Wave 2 parallel (Units 2∥3) does not conflict with lens A's "Phase 3 can proceed in parallel with Phase 2 once Phase 1 lands"; this checklist ships them sequentially inside one packet. Chained PRs (lens C) and "no apply-dag.yaml" (lens B) are different layers. `serve.Model` method identifiers remain unset in `design.md`; no tasks lens re-asserted incompatible names, so that design-phase item is not re-escalated here.

## Coverage Gaps

- **No waves merged.** Wave 1 (Unit 1) is green alone: additive schema v6 and `*Ledger` methods; existing `RegisterLane` callers compile with zero-value metadata fields. Wave 2 Unit 2 and Unit 3 are each green after Wave 1 (`NewHandler` at `cmd/lucind-ai/cli.go:715` unchanged). Same-wave Unit 2∥3 would be disjoint under `PathInScope` (`internal/packet/disjoint.go:8-22`): neither `cmd/lucind-ai/` nor `internal/run/` is a prefix of `internal/serve/` or vice versa. Shipped as a single packet anyway (sidecar overhead).
- **Skill `sdd-tasks` 530-word budget not applied.** Packet 1800-word budget wins. Skill forecast fields, Suggested Work Units columns (Likely PR, Focused test command, Runtime harness, Rollback boundary), specific/actionable/verifiable/small, and threat-matrix RED-before-production are in `tasks.md`. Skill Engram Step 4/5 return block is superseded by the two files plus `.lucind/result.json`.
- **Linked-worktree production already exists** at `cmd/lucind-ai/cli.go:277-280,702-705`. Task 2.2-RED may pass on HEAD (characterization). Design also listed `TestResolve` (`internal/ledgerpath/ledgerpath_test.go:9`) as a planned RED; it already exists — no new task.
- **No prune-trigger or message-shape tasks.** All three lenses left those as open questions; they stay in `tasks.md` Open questions.
- **Lens C verification gaps not tasked:** unclean `SIGKILL` leaving `runs.status=running`; end-to-end UI poll (`internal/serve/static/app.js:96-97`). Out of scope (`control-room-serve`, `control-room-ui-views`).
- **`handlers.go` / `/api/state` not tasked.** Lens A Phase 3 is `model.go` only. HTTP wiring is `control-room-serve`. `serveStateJSON` today is approvals-only (`internal/serve/handlers.go:79-85,120-146`).
- **No task to emit `run_status_changed` at CLI.** Spec requires the CHECK to admit the type (1.1, 1.6), not a dual-write. Proposal still asks AppendEvent vs helpers.

## Dropped Citations

Every item below was opened in this worktree. The claim is not in `tasks.md`.

1. **`internal/ledger/schema.go:308` (A 1.1).** File ends at 307; `migrate` returns `tx.Commit()` at 306–307. Kept as `schema.go:293-307`.
2. **`internal/ledger/ledger.go:230-330` as Open/WAL/pool (A 1.2).** That range is sqlite error codes, `Lane`, `RegisterLane`, `Lanes`. Open/WAL/pool/Close are `:127-130,:146-191,:162-184,:217-218`.
3. **A 2.2 as adding linked-worktree refusal.** Those returns already exist at `cli.go:277-280,702-705`. Task 2.2 is pin-via-test, not a new gate.
4. **C RED production task labeled 2.1 for worktree.** A's 2.1 is `RegisterRun`; worktree is 2.2. Citation `cli.go:277-280,702-705` maps to 2.2.
5. **C `go test -run` names** (`TestMigrateV5ToV6Database`, `TestRegisterAndGetRun`, `TestUpdateRunStatus`, `TestRegisterLanePersistsMetadata`, `TestEventsAdmitRunStatusChanged`, `TestAppendProgressAndCursorTail`, `TestPruneProgressIsolated`, `TestConcurrentProgressAndLeaseContention`, `TestRunLifecycleRegistration`, `TestRunRefusesLinkedWorktree`, `TestServeRefusesLinkedWorktree`, `TestModelRunAndProgressQueries`). None exist. A missing `-run` name exits 0. Kept as tests to write; unit focused commands are package-level `go test`.
6. **`ledger_test.go:43` as run persistence (C).** `TestOpenPlacesDatabaseUnderPrimaryRootLucindDir`.
7. **`ledger_test.go:579` / `:579-630` as v6 migrate or progress cursor (C).** `TestMigrateV1DatabaseAcceptsLaneNoteAndPreservesRows` and its v1 fixture.
8. **`ledger_test.go:608-616` as metadata persist (C).** v1 `INSERT` without `model`/`agent`/`feature`.
9. **`ledger_test.go:571-575` / `:618-630` as `run_status_changed` CHECK (C).** v1 events DDL/INSERT (five original types, no `lane_note` in the CHECK at 571–572).
10. **`ledger_test.go:24` as progress analog (C).** `openTestLedger` helper.
11. **`ledger_test.go:1584-1600` as `PruneProgress` (C).** `TestPruneIntegrationEventsRetention` deletes `integration_events`. Analog only.
12. **`ledger_test.go:360-367` as progress+lease contention (C).** `TestConcurrentRegisterAndSetStatusAcrossDistinctLanes` (lanes only). Analog at `:367`.
13. **`internal/run/attempt.go:434-441` as progress contention (C).** `renewInterval` / `startLeaseRenewal` during feature checks. Neighbor, not a progress API.
14. **`cli_test.go:212-255` as worktree/lifecycle analog (C).** `TestRunKnownModelForExecutorPasses` / `TestRunOmittedModelSkipsModelCheck`.
15. **`cli_test.go:1060-1100` as lifecycle analog (C).** `TestRunSequentialInvocationsProduceDistinctRunIDs`; `depsFactory` inject starts at `:1074`.
16. **`model_test.go:585-594` as run/progress queries (C).** Reconciliation DTO field asserts. `TestModelSourceDoesNotShellOut` starts at `:595`.
17. **`internal/serve/static/app.js:96-97` as Unit 3 proof (C).** 2s `fetchState` poll. Out of scope.
18. **Archive "1,500 lines / Unit 1 = 65%" sidecar rule (B).** `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` recommends no sidecar because that change was ~650–1200 lines and Unit 1 was too small for orchestration overhead. No 1500/65% rule. Single-packet recommendation kept on that analog plus Units 2+3 size.
19. **C `TestConcurrentProgressAndLeaseContention` as a threat-matrix RED.** Threat matrix Applicable row is only Git repository selection. Concurrency is a Unit 1 acceptance test (`TestConcurrentProgressAndSetStatus`), not a threat RED.

## Decomposition Divergence

All three independently converged on three phases: (1) transactional schema v6 and `*Ledger` domain files, (2) CLI run lifecycle plus `RegisterLane` metadata at `run.go:327` and `batch.go:184`, (3) shell-free DTOs on `serve.Model`. Critical path through Unit 1, then 2 and 3 unblocked. That convergence is corroboration.

What B assumed that did not survive A:

- **`internal/serve/handlers.go` in Unit 3.** A's Phase 3 is `model.go` only. Design File Changes lists `model.go`, not handlers. Dropped from `tasks.md`; `/api/state` stays approvals-only this change.
- **DAG Wave 2 (Units 2∥3) as the shipped shape.** B itself recommended no sidecar. Canonical file follows that recommendation and names the sequential single-packet shape.

What C assumed that did not survive A:

- **Worktree RED attached to "Task 2.1".** Remapped to 2.2.
- **Named `-run` filters as current proving commands.** Tests-to-write only.
- **Concurrency / prune / migrate tests cited at existing line numbers as if they already covered v6 APIs.** Those lines are analogs (v1 migrate, v4 migrate at `:934`, `PruneIntegrationEvents`, lane contention at `:367`).

Lens C's Review Workload Forecast (1,200–1,600; High; ask-on-risk; feature-branch-chain) and threat-matrix N/A rows match the design and are in `tasks.md`. B's executor assignment (agy / cursor-agent / cursor-agent) and rollback boundaries are in the Suggested Work Units table.
