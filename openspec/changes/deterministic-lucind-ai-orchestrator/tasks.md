# Tasks: Deterministic lucind-ai Orchestrator

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700–1400 (skill/reference edits copied to both trees; Go gaps + tests). OpenCode tree already exists and is byte-identical, so this is not a 2500–3500-line create. |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 skills → PR 2 packet/DAG → PR 3 acceptance → PR 4 attempts → PR 5 CLI; or one PR if the human picks `size-exception` under this session’s 5000-line review budget |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

No `apply-dag.yaml` sidecar. Ship as one sequential packet (one lane, RED then GREEN inside it). Five work units below are rollback/PR-split boundaries, not parallel waves. Strict-TDD split across waves fails `Integrate` (`internal/run/integrate.go:50-59`; `openspec/changes/archive/2026-08-21-sdd-fan-out-lens/apply-dag.yaml:8-13`). Precedent: `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27`.

### Suggested Work Units

| Unit | Goal | Likely PR | Executor | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------|----------------------|-----------------|-------------------|
| 1 | Canonical Claude skill/references; re-copy into existing OpenCode tree | PR 1 | `agy` | `go test -run TestSkillAssetContract ./internal/packet` | N/A: prompt text; parity is a tree comparator | revert `plugin/claude-code/skills/lucind-ai/` and matching OpenCode files |
| 2 | Keep target-free parse and omitted-`allowed_paths` skip; pin Split stdout waves | PR 2 | `cursor-agent` | `go test -run 'TestParseAllowedPathsFrontmatter\|TestSplit_TwoWaveDAGSuccess\|TestWaves_OrderingAndYAMLOrderPreserved' ./internal/packet ./internal/dag` | `lucind-ai split --dag <fixture> --out <dir>` (stdout only; no plan file) | revert `internal/packet/packet.go` and `internal/dag/split.go` plus their tests |
| 3 | RED then GREEN: `decideStatus` demotes fired hard stops; pin commit-state completion | PR 3 | `cursor-agent` | `go test -run 'TestDecideStatus_FiredHardStopDemotes\|TestExecuteWriteDone\|TestExecuteReadOnly' ./internal/run` | N/A: hermetic `Execute` / `decideStatus` | revert `internal/run/run.go`, `batch.go`, and matching `*_test.go` (exact files, not `internal/run/`) |
| 4 | Pin idempotent attempt replay, CAS recovery, homogeneous `FeatureTarget` | PR 4 | `cursor-agent` | `go test -run 'TestAttemptReplayTerminalReturnsStoredResultWithoutSpies\|TestAttemptInterruptionAndRecoveryRefMismatchFailsClosed\|TestFeatureTargetHomogeneousBatchNamesTheFeature\|TestFeatureTargetRejectsPacketWithNoDeclaredTarget' ./internal/run` | N/A: hermetic ledger/CAS doubles | revert `internal/run/attempt.go`, `integrate_feature.go`, and matching tests |
| 5 | RED then GREEN: skill-parity and schema-freshness preflight at existing CLI barriers | PR 5 | `cursor-agent` | `go test -run 'TestRunPreflight_SkillParity\|TestRunDispatch_RejectsLinkedWorktree\|TestRunDispatch_SiblingWorktreeRejected\|TestResolvePrimaryRoot_RelativeCwdResolvesToPrimary' ./cmd/lucind-ai` | `lucind-ai run` from a linked worktree must refuse before `worktree.Create` | revert `cmd/lucind-ai/cli.go` and `cli_test.go` |

Same-wave disjointness does not apply (single packet). If a DAG is authored later, list exact files under `internal/run/` — naming the directory collides units 3 and 4 (`internal/packet/disjoint.go:13-22,24-48`).

## Phase 1: Orchestrator Skill Parity

- [ ] 1.1 Update `plugin/claude-code/skills/lucind-ai/SKILL.md:14-28` and `references/` for cross-runtime preflight, late target bind, and wave-N+1-only-after-exit-0. Do not add `LUCIND_REQUIRED_SKILLS`, `required_skills`, `integrate retry`, or `defect *` (`design.md:119`).
- [ ] 1.2 After 1.1, overwrite `plugin/opencode/skills/lucind-ai/` with a byte-identical copy of the Claude tree (directory already exists and currently matches; this is re-sync, not create).

## Phase 2: Packet Authoring & DAG Splitting

- [ ] 2.1 Keep `packet.Parse` (`internal/packet/packet.go:118-238`) free of required target fields (`:84-90`) and keep omitted `allowed_paths` empty (`:74-77,187-194`). Pin `TestParseAllowedPathsFrontmatter` omitted case (`packet_test.go:490-498`) and undeclared skip (`disjoint.go:24-48`, `disjoint_test.go:146-157`). Do not loosen `validatePacketAdmission`.
- [ ] 2.2 Keep `dag.Split` printing `lucind-ai run` lines in Kahn order with no plan file (`internal/dag/split.go:18-51`). Pin `TestSplit_TwoWaveDAGSuccess` (`split_test.go:13-57`) and `TestWaves_OrderingAndYAMLOrderPreserved` (`waves_test.go:43-60`). Skill (1.1) owns halt-on-nonzero; Split does not schedule.

## Phase 3: Runtime Acceptance

- [ ] 3.0-RED Write `TestDecideStatus_FiredHardStopDemotes`: schema-valid `status=done` with `HardStop.Fired=true` must yield `lane.Blocked`. Must fail today: `decideStatus` returns `envelope.LaneStatus()` 1:1 (`internal/run/run.go:868-893`).
- [ ] 3.0b-RED Pin commit-state (threat matrix Applicable) via existing `TestExecuteWriteDoneWithoutUniqueCommitsFails`, `TestExecuteWriteDoneWithDirtyWorktreeFails`, `TestExecuteReadOnlyDoneWithUniqueCommitsFails` (`run.go:969-980`). Add git-level staged leftover vs untracked leftover if the dirty stub does not distinguish them. No RED for Documentation-like paths, Push state, or PR commands (`design.md:101-109`).
- [ ] 3.1 GREEN: after schema-valid `result.Read`, if any `HardStop.Fired` is true, demote to `lane.Blocked` regardless of `envelope.Status`. Do not change `enforceRequiredSkills` (`run.go:496-498`).
- [ ] 3.2 Keep `ExecuteBatch` non-cancelling WaitGroup join (`internal/run/batch.go:29-89`) so a hard-stop-blocked lane is not integrated. Pin `TestExecuteBatchAllDoneReleasesAndIntegratesAll` (`batch_test.go:219-250`).

## Phase 4: Feature Integration & Idempotent Attempts

- [ ] 4.1 Keep `ExecuteAttempt` terminal replay without redispatch (`internal/run/attempt.go:217-256`) and `RecoverAttempt` fail-closed on parent SHA mismatch while preserving worktrees (`:592-701`). Pin `TestAttemptReplayTerminalReturnsStoredResultWithoutSpies` (`attempt_test.go:128-150`) and `TestAttemptInterruptionAndRecoveryRefMismatchFailsClosed` (`:626-650`). No `integrate retry` work.
- [ ] 4.2 Keep `FeatureTarget` homogeneous bound targets (`internal/run/integrate_feature.go:26-78`) and `IntegrateFeature` revert-on-failed-promotion (`:100-140`). Pin `TestFeatureTargetHomogeneousBatchNamesTheFeature` (`integrate_feature_test.go:55-80`) and `TestFeatureTargetRejectsPacketWithNoDeclaredTarget` (`:142-152`).

## Phase 5: CLI Preflight

- [ ] 5.0-RED Write failing tests: skill-tree mismatch or stale embedded schema halts before `worktree.Create`; sibling worktree rejected; relative cwd resolves via `resolvePrimaryRoot` (`cmd/lucind-ai/cli.go:802-827`). Pin linked-worktree refusal already in `runDispatch` (`:353-361`) and `runFeatureCreate` (`:958-1007`). Pattern: `cli_test.go:45-88`; `worktree.IsLinkedWorktree` (`internal/worktree/worktree.go:292-313`).
- [ ] 5.1 GREEN: add skill-parity and embedded-schema freshness at those same barriers (Decision 2), before allocation. Do not add a preflight subcommand. Keep `printIntegrateReport` `integrated_ids`/`reverted_ids` (`cli.go:752-782`); do not retask that printer or `integrate retry`.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Canonical skill text. |
| 1.2 | 1.1 | OpenCode copy must match the edited Claude tree. |
| 2.1 | — | Parse rules are independent of skills. |
| 2.2 | 2.1 | Split consumes packet/DAG nodes. |
| 3.0-RED, 3.0b-RED | — | RED before GREEN in this lane. |
| 3.1 | 3.0-RED | Demotion implements the failing test. |
| 3.2 | 3.1 | Barrier outcomes include demoted lanes. |
| 4.1 | — | Attempt machine is independent of 3.1. |
| 4.2 | 2.1, 4.1 | Homogeneous targets + `ExecuteAttempt`. |
| 5.0-RED | — | RED before GREEN in this lane. |
| 5.1 | 1.2, 2.1, 3.2, 4.2, 5.0-RED | Preflight checks parity and calls existing run/packet APIs. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `specs/deterministic-orchestrator-contract/spec.md:5-25` | 1.1, 1.2, 5.0-RED, 5.1 |
| `specs/packet-authoring-contract/spec.md:5-29` | 2.1, 4.2, 5.1 |
| `specs/sdd-apply/spec.md:5-18` | 1.1, 2.2, 3.2 |
| `specs/acceptance-verifier/spec.md:5-39` | 3.0-RED, 3.1, 3.2 |
| `specs/parent-feature-integration/spec.md:5-23` | 4.1, 4.2 |

## Open Questions

- [ ] None. Chain strategy is `pending` until the ask-on-risk decision (stacked-to-main, feature-branch-chain, or size-exception).
