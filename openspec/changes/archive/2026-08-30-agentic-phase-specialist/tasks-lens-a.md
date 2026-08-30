# Tasks Lens A — Decomposition & Ordering: Agentic Phase Specialist

## Assumed decomposition

The implementation decomposes into four sequential phases: (1) skill tree synchronization and reference documentation updates for the Hard Rule carve-out, synthesis-note arbitration, and decision-bearing Acceptance contracts; (2) acceptance verifier test-driven gating for `SDDPhase` checks in `internal/accept`; (3) attempt execution test-driven gating for `checkFunc` in `internal/run`; (4) out-of-repository prompt handoff for `~/.claude/skills/sdd-*/SKILL.md`. The critical path requires Phase 1 to establish the contract and verify byte-identical skill trees, followed by Phase 2 and Phase 3 to implement scoped check gating in Go, with Phase 4 as an out-of-band post-Change handoff.

## Phase 1: Operational Contracts & Skill Tree Synchronization

- [ ] 1.1 Apply Hard Rule Decision 3 replacement text in `plugin/claude-code/skills/lucind-ai/SKILL.md:19` and `plugin/opencode/skills/lucind-ai/SKILL.md:19`, carving out phase-scoped Specialist Acceptance while preserving human-only Promotion.
- [ ] 1.2 Update synthesis-note review and contradiction arbitration in `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` and `plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47-48` from Orchestrator to Specialist.
- [ ] 1.3 Update 10-step checklist steps 1 and 8 and Acceptance delegation in `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30,31-36` and `plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30,31-36` to add `sdd_phase`-conditional check caveats and decision-bearing Specialist Acceptance.
- [ ] 1.4 Verify skill trees remain byte-identical and domain projections remain in lockstep via `TestSkillTreesByteIdentical` and `TestSkillAssetContract` in `internal/packet/packet_test.go:924-941,943-967`.

## Phase 2: Acceptance Verifier SDD-Phase Gating

- [ ] 2.1 Author RED unit tests in `internal/accept/accept_test.go:26-67,80-140` asserting `Verifier.Accept` executes `lucind-checks.sh` for `SDDPhase == "apply"`, empty `sdd_phase`, missing metadata, or explicit exception; skips `v.check` for non-apply planning phases; and fails closed on failing checks.
- [ ] 2.2 Modify `internal/accept/accept.go:84-96,120-137` to lift `GetLaneMetadata` out of the versioned authoring evidence branch, evaluate `metadata.SDDPhase`, and gate `CheckPolicySnapshot` and `v.check` execution.

## Phase 3: Attempt Execution SDD-Phase Gating

- [ ] 3.1 Author RED unit tests in `internal/run/attempt_test.go:24-44,83-92` asserting `ExecuteAttempt` in CHECKING phase executes `checkFunc` for apply, empty, missing, or exception lanes, and skips `checkFunc` when all combined lanes declare non-apply planning phases.
- [ ] 3.2 Modify `internal/run/attempt.go:217-328,431-448` in `ExecuteAttempt` to inspect combined lanes' `SDDPhase` via `deps.Ledger` and gate `checkFunc` execution.

## Phase 4: Out-of-Repository Specialist Prompt Handoff

- [ ] 4.1 Hand off paste-ready prompt text from `openspec/changes/agentic-phase-specialist/design.md:102-108` to human operator for out-of-repository `~/.claude/skills/sdd-*/SKILL.md` updates.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Hard Rule edit in `SKILL.md` has no code or doc dependencies and can proceed in parallel. |
| 1.2 | — | Synthesis-note arbitration edit in `fan-out.md` has no code or doc dependencies and can proceed in parallel. |
| 1.3 | — | Acceptance checklist and delegation edit in `acceptance-promotion.md` has no code dependencies and can proceed in parallel. |
| 1.4 | 1.1, 1.2, 1.3 | `TestSkillTreesByteIdentical` fails unless all mirrored files across Claude Code and OpenCode skill trees are updated in lockstep. |
| 2.1 | — | Verifier test harness `newVerifierFixture` exists; RED tests can be written independently before verifier gating changes. |
| 2.2 | 2.1 | `accept.go` gating implementation satisfies and turns green the failing RED tests in `accept_test.go`. |
| 3.1 | — | Attempt test harness `attemptSpies` exists; RED tests can be written independently before attempt gating changes. |
| 3.2 | 3.1 | `attempt.go` gating implementation satisfies and turns green the failing RED tests in `attempt_test.go`. |
| 4.1 | 1.1, 1.2, 1.3 | External skill prompt handoff requires in-repo contracts and reference documentation to land first. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `phase-verdict-reporting`: Structured Phase Verdict Reporting | 1.2, 1.3, 4.1 |
| `phase-specialist-dispatch`: Specialist sequencing and canonical artifact generation | 1.1, 1.2, 1.3, 1.4, 4.1 |
| `acceptance-verifier`: Fail-Closed Mechanical Criteria | 1.3, 2.1, 2.2, 3.1, 3.2 |
| `sdd-planning-fan-out`: Two-Wave Planning Fan-Out Protocol | 1.2, 4.1 |

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:91-93` | Definition of Promotion as human-confirmed integration into an Integration Target. |
| `CONTEXT.md:103-106` | Definition of Specialist as phase-scoped Agent holding independent phase-lane Acceptance authority. |
| `CONTEXT.md:107-109` | Definition of Phase Verdict as compressed markdown report containing outcome, canonical path, and unresolved divergence. |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | Architecture decision establishing phase-Specialist acceptance authority and scoped `lucind-checks.sh` execution. |
| `docs/sdd-phase-specialist.md:21-30` | Resolved design decisions for Specialist runtime substrate, Acceptance authority, Phase Verdict contents, bounded correction, and check gating. |
| `internal/accept/accept.go:84-96` | `GetLaneMetadata` currently invoked conditionally inside `AuthoringEvidenceVersion` branch before `validateResultAndScope`. |
| `internal/accept/accept.go:120-137` | `CheckPolicySnapshot` and `v.check` execution inside verifier isolation worktree. |
| `internal/accept/accept.go:214-261` | `validateResultAndScope` enforcing result hash, status, hard stops, done criteria, and `allowed_paths` scope. |
| `internal/accept/accept_test.go:26-67` | `newVerifierFixture` test fixture setting up candidate commits, ledger records, and verifier instances. |
| `internal/accept/accept_test.go:80-140` | Existing verifier test suite asserting acceptance receipt persistence, scope checking, and check failures. |
| `internal/integrate/integrate.go:159-200` | `integrate.Check` executing `lucind-checks.sh` script within target worktree. |
| `internal/ledger/lanes_meta.go:20-47` | `LaneMetadata` struct definition capturing `SDDPhase` and packet dispatch fields. |
| `internal/packet/packet_test.go:924-941` | `TestSkillAssetContract` asserting `references/core/domain.md` remains in lockstep with `CONTEXT.md`. |
| `internal/packet/packet_test.go:943-967` | `TestSkillTreesByteIdentical` asserting Claude Code and OpenCode skill trees are byte-identical. |
| `internal/run/attempt.go:217-328` | `ExecuteAttempt` orchestrating attempt lifecycle and state recovery. |
| `internal/run/attempt.go:431-448` | `checkFunc` execution during attempt CHECKING phase. |
| `internal/run/attempt_test.go:24-44` | `attemptSpies` struct tracking test invocations including `checkCalls` and custom `checkFunc`. |
| `internal/run/attempt_test.go:83-92` | Default `RunChecks` spy recording `checkCalls` and returning pass status. |
| `internal/run/run.go:377-397` | `UpdateLaneMetadata` recording packet `SDDPhase` in ledger at dispatch. |
| `openspec/changes/agentic-phase-specialist/design.md:14-37` | Architecture decisions 1, 2, and 3 governing Phase Verdict format, caller check gating, and Hard Rule carve-out. |
| `openspec/changes/agentic-phase-specialist/design.md:39-47` | Literal replacement text for Hard Rule in `SKILL.md` line 19. |
| `openspec/changes/agentic-phase-specialist/design.md:82-89` | Phase Verdict completion interface format specifying Outcome, Canonical Artifact, and Unresolved Divergence. |
| `openspec/changes/agentic-phase-specialist/design.md:92-108` | File changes table defining modified files, actions, terminal consumers, and out-of-repository handoff text. |
| `openspec/changes/agentic-phase-specialist/specs/acceptance-verifier/spec.md:5-57` | Fail-Closed Mechanical Criteria requirement and scenarios for apply suite execution, planning suite skipping, and unlabeled/exception handling. |
| `openspec/changes/agentic-phase-specialist/specs/phase-specialist-dispatch/spec.md:5-50` | Specialist sequencing requirement and scenarios for lens gating, independent acceptance authority, Tier A dual-judge, and promotion prohibition. |
| `openspec/changes/agentic-phase-specialist/specs/phase-verdict-reporting/spec.md:5-29` | Structured Phase Verdict Reporting requirement and scenarios for compressed verdict, bounded correction, and rejecting raw evidence. |
| `openspec/changes/agentic-phase-specialist/specs/sdd-planning-fan-out/spec.md:5-32` | Two-Wave Planning Fan-Out Protocol requirement and scenarios for dual-wave dispatch, unintegrated draft protection, and synthesis note arbitration. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19` | Existing Hard Rule denying acceptance authority to all agents before Specialist carve-out. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | 10-step acceptance checklist steps 1 and 8 stating full-suite execution prior to `sdd_phase` conditional caveat. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Acceptance subagent delegation section defining evidence-only gathering prior to decision-bearing Specialist upgrade. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25` | Dispatch sequencing rules requiring lens completion and integration before synthesis dispatch. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Synthesis-note review and contradiction arbitration currently assigned to Orchestrator before move to Specialist. |
| `plugin/opencode/skills/lucind-ai/SKILL.md:19` | OpenCode mirror Hard Rule denying acceptance authority to all agents before Specialist carve-out. |
| `plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | OpenCode mirror 10-step acceptance checklist steps 1 and 8 prior to `sdd_phase` conditional caveat. |
| `plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | OpenCode mirror Acceptance subagent delegation section prior to decision-bearing Specialist upgrade. |
| `plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47-48` | OpenCode mirror synthesis-note review currently assigned to Orchestrator before move to Specialist. |
