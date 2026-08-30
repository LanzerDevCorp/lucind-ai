# Tasks: Agentic Phase Specialist

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 250–350 (gating in `accept.go:84-137` ~55; SDDPhase tests in `accept_test.go` ~100; `attempt.go:431-448` ~20; spy tests ~70; six skill-tree files ~90) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | auto-chain |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

Session review budget is 1500 lines (stricter than `openspec/config.yaml:7`). The estimate sits under both 400 and 1500.

**No `apply-dag.yaml` sidecar.** Ship one sequential packet with three in-repo work-unit commits (skills, `internal/accept` RED+GREEN, `internal/run` RED+GREEN). Executor for every unit: `cursor-agent`. A three-node same-wave DAG would be path-disjoint on exact files (`plugin/…` vs `internal/accept/accept.go` vs `internal/run/attempt.go`; `internal/packet/disjoint.go:8-22,24-47`) and each unit is green alone, but sidecar authoring costs more than the change (precedent `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27`). Keep both skill trees in unit 1 so `TestSkillTreesByteIdentical` (`internal/packet/packet_test.go:943-967`) is not a two-lane race. `strict_tdd: true` (`openspec/config.yaml:8`): RED and GREEN for one unit stay in one lane — `Integrate` reverts a red combined tree (`internal/run/integrate.go:50-59`).

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Hard Rule carve-out, Specialist synthesis review, decision-bearing Acceptance and `sdd_phase` checklist caveat in both skill trees | PR 1 | `go test ./internal/packet -run 'TestSkillTreesByteIdentical\|TestSkillAssetContract' -count=1` | N/A: prompt text; parity is a tree comparator | revert the six files under `plugin/claude-code/skills/lucind-ai/` and `plugin/opencode/skills/lucind-ai/` named in 1.1–1.3 |
| 2 | Unconditional `GetLaneMetadata`; gate `CheckPolicySnapshot` and `v.check` on `SDDPhase` | PR 1 | `go test ./internal/accept -race -count=1` | N/A: hermetic `newVerifierFixture` (`internal/accept/accept_test.go:26-67`) | revert `internal/accept/accept.go` and `accept_test.go` |
| 3 | Gate `checkFunc` in `ExecuteAttempt` CHECKING from combined-lane `SDDPhase` | PR 1 | `go test ./internal/run -run 'TestExecuteAttempt' -count=1` | N/A: hermetic `attemptSpies` (`internal/run/attempt_test.go:24-44,83-92`) | revert `internal/run/attempt.go` and `attempt_test.go` |

Same-wave disjointness does not apply (single packet). If a DAG is authored later, list exact files — a directory `plugin/` or `internal/run/` collides everything beneath it.

## Phase 1: Operational Contracts & Skill Tree Synchronization

- [ ] 1.1 Replace the Hard Rule at `plugin/claude-code/skills/lucind-ai/SKILL.md:19` and `plugin/opencode/skills/lucind-ai/SKILL.md:19` with Decision 3 New: text (`design.md:39-47`): a named `sdd-*` Specialist may Accept its own phase’s Lanes; Promotion stays forbidden to every Agent.
- [ ] 1.2 In both trees’ `references/strategies/fan-out.md:47-48`, move synthesis-note review and contradiction arbitration from Orchestrator to Specialist.
- [ ] 1.3 In both trees’ `references/contracts/acceptance-promotion.md:18-30,31-36`, add the Decision 2 `sdd_phase` caveat to checklist steps 1 and 8, and upgrade subagent delegation to decision-bearing Specialist Acceptance. Dual-Judge (`:38-43`) stays.
- [ ] 1.4 After 1.1–1.3, confirm `TestSkillTreesByteIdentical` (`packet_test.go:943-967`) and glossary lockstep in `TestSkillAssetContract` (`packet_test.go:778,924-941`). Edit both mirrors in the same unit.

## Phase 2: Acceptance Verifier SDD-Phase Gating

- [ ] 2.1-RED In `internal/accept/accept_test.go`, using `newVerifierFixture` (`:26-67`) and `UpdateLaneMetadata` (`internal/ledger/lanes_meta.go:49-60`), write failing `TestVerifier*` cases: `Verifier.Verify` (`accept.go:62`) runs `v.check` for `SDDPhase == "apply"`, `""`, or missing metadata; skips `CheckPolicySnapshot` and `v.check` for declared non-apply planning phases; still enforces schema, hard stops, done criteria, and `allowed_paths` (`:214-261`); failing checks reject apply/unlabeled (existing `:320`). Must fail today: checks always run (`:120-137`).
- [ ] 2.2 GREEN: Lift `GetLaneMetadata` out of the `AuthoringEvidenceVersion` branch (`accept.go:84-96`). Gate `CheckPolicySnapshot` and `v.check` (`:120-137`) per Decision 2 (`design.md:21-27`). Do not modify `integrate.Check` (`internal/integrate/integrate.go:159-200`).

## Phase 3: Attempt Execution SDD-Phase Gating

- [ ] 3.1-RED In `internal/run/attempt_test.go`, using `attemptSpies.checkCalls` (`:24-44,83-92`), write failing tests: `ExecuteAttempt` (`attempt.go:217-328`) invokes `checkFunc` (`:431-448`) when any combined lane is apply or empty/missing; skips only when every combined lane is a declared non-apply phase. Must fail today: `:448` always calls `checkFunc`.
- [ ] 3.2 GREEN: In CHECKING, resolve `SDDPhase` from combined lanes via `Deps.Ledger` (`internal/run/run.go:165,377-397`) and gate `checkFunc`. Leave lease renewal (`attempt.go:447-449`) and `integrate.Check` ungated.

## Phase 4: Out-of-Repository Specialist Prompt Handoff

- [ ] 4.1 After 1.1–1.3, hand the paste-ready prompt (`design.md:102-106`) to a human for `~/.claude/skills/sdd-*/SKILL.md`. Lanes cannot write it (`fan-out.md:43`).

Threat matrix (`design.md:121-127`): every row is N/A. No threat-matrix RED tasks.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1, 1.2, 1.3 | — | Distinct files; each task edits both mirrors of one pair. |
| 1.4 | 1.1, 1.2, 1.3 | Byte-identity after all three pairs land. |
| 2.1-RED | — | Fixture exists; no production dependency. |
| 2.2 | 2.1-RED | Same lane; turns 2.1 green. |
| 3.1-RED | — | Spies exist; independent of `accept.go`. |
| 3.2 | 3.1-RED | Same lane; turns 3.1 green. |
| 4.1 | 1.1, 1.2, 1.3 | External paste follows the in-repo contract. |

Phases 2 and 3 are independent of each other and of Phase 1 for compile/test greenness.

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `phase-verdict-reporting`: Structured Phase Verdict Reporting | 1.2, 1.3, 4.1 |
| `phase-specialist-dispatch`: Specialist sequencing and canonical artifact generation | 1.1, 1.2, 1.3, 1.4, 4.1 |
| `acceptance-verifier`: Fail-Closed Mechanical Criteria | 1.3, 2.1-RED, 2.2, 3.1-RED, 3.2 |
| `sdd-planning-fan-out`: Two-Wave Planning Fan-Out Protocol | 1.2, 4.1 |

## Open Questions

- [ ] What tool or CLI bridge, in a later Change, lets `sdd-*` Specialists invoke `lucind-ai run` without Orchestrator mediation? (`design.md:139`)
- [ ] Should `lucind-ai accept` expose `--force-checks`, or is packet-level exception metadata enough? (`design.md:140`) `LaneMetadata` has `SDDPhase` and no exception field (`lanes_meta.go:20-47`). Unlabeled empty/missing is covered by 2.1/3.1; do not add schema (`design.md:31`).
