# Verify Dual Dispatch Specification

## Purpose

Orchestrate the two-stage SDD verify workflow, dispatching parallel read-only qualitative judgment lanes to `agy` and `cursor-agent`, reconciling dual verdict envelopes, cross-checking evidence against the codebase, synthesizing the canonical `verify.md` report, and updating change state.

## Requirements

### Requirement: Two-Stage SDD Verify Protocol

`plugin/claude-code/skills/lucind-ai/SKILL.md:80` MUST be updated from "Target direction, not yet built" to an operational two-stage verification workflow. Stage 1 MUST execute deterministic mechanical checks (`lucind-ai check`) once and commit `verify-mechanical.log`. Stage 2 MUST construct and dispatch dual read-only judgment packets to `agy` and `cursor-agent` in parallel. (Design Decision 1, Decision 4.)

#### Scenario: SKILL.md documents operational verify protocol
- GIVEN `plugin/claude-code/skills/lucind-ai/SKILL.md`
- WHEN row 80 and the verify workflow instructions are inspected
- THEN they MUST define the operational two-stage verify protocol and MUST NOT list verify as unbuilt or blocked

#### Scenario: Sequential stage ordering
- GIVEN an SDD change ready for verification
- WHEN `sdd-verify` is executed
- THEN stage 1 (`lucind-ai check`) MUST complete and succeed before stage 2 (dual packet dispatch) is initiated

### Requirement: Dual Parallel Judgment Dispatch and Barrier Join

The orchestrator MUST author two independent judgment packets (`packets/verify-<change-id>-agy.md` and `packets/verify-<change-id>-cursor-agent.md`) and dispatch them concurrently using a single `lucind-ai run` command. `lucind-ai run` MUST execute both lanes in parallel in isolated worktrees and hold the barrier until both lanes reach a terminal status (`Done`, `Blocked`, `Deviated`, or `Failed`). (Design Decision 1, Decision 2.)

#### Scenario: Single command dispatches both judgment lanes
- GIVEN prepared verify packets for `agy` and `cursor-agent`
- WHEN the orchestrator executes stage 2
- THEN it MUST invoke `lucind-ai run --packet packets/verify-<change-id>-agy.md --packet packets/verify-<change-id>-cursor-agent.md` in a single command

#### Scenario: Isolated worktrees created from candidate HEAD
- GIVEN the dual dispatch command
- WHEN `worktree.Create` provisions lane workspaces
- THEN `agy` and `cursor-agent` MUST each receive an independent, isolated worktree branched from candidate `HEAD`

#### Scenario: Barrier join waits for both lanes
- GIVEN both judgment lanes executing concurrently
- WHEN `lucind-ai run` reaches the barrier
- THEN it MUST block until both lanes have completed their execution and completion-mode enforcement

### Requirement: Mechanical Log Context Embedding

The orchestrator MUST embed the summary status, execution duration, git commit SHA, and any failure transcripts from `verify-mechanical.log` into the `## Context` section of both judgment packets prior to dispatch. (Design Decision 1.)

#### Scenario: Mechanical summary embedded in packet context
- GIVEN `verify-mechanical.log` generated from stage 1
- WHEN the orchestrator authors `packets/verify-<change-id>-agy.md` and `packets/verify-<change-id>-cursor-agent.md`
- THEN the `## Context` section of both packets MUST contain the frozen mechanical check summary

#### Scenario: Executors evaluate in-prompt context without file re-reads
- GIVEN an LLM executor running in a judgment worktree
- WHEN assessing test suite outcomes
- THEN the executor MUST find the mechanical outcome directly in its prompt context without needing additional file-read tool calls

### Requirement: Independent Evidence Cross-Checking

The orchestrator MUST independently verify every `file:line` citation, test assertion, and defect claim in both judgment envelopes against the actual codebase before accepting findings into `verify.md`. Green criteria alone MUST NOT be treated as proof of complete work (`SKILL.md:102`). (Design Decision 4.)

#### Scenario: Cited defect confirmed against codebase
- GIVEN an envelope reporting a defect with citation `internal/auth/session.go:84`
- WHEN the orchestrator reconciles verdicts
- THEN it MUST inspect `internal/auth/session.go:84` in the codebase to confirm the defect before marking the overall verdict blocked

#### Scenario: False positive refuted with code evidence
- GIVEN an envelope reporting a missing boundary check
- WHEN the orchestrator inspects the codebase and finds the check implemented at an upstream layer
- THEN it MUST record the finding as refuted in `verify.md` with concrete `file:line` citations and MUST NOT block on it

### Requirement: Four-Case Verdict Reconciliation Logic

The orchestrator MUST synthesize `openspec/changes/<change-id>/verify.md` by applying the 4-case reconciliation rules to the returned envelopes:
1. **Unanimous approval** (`done` / `done`): overall status `PASSED`, combining spec compliance matrices and non-blocking findings.
2. **Disagreement / Defect** (`done` vs `blocked`/`deviated`, or `blocked`/`blocked`): orchestrator adjudicates against code. Confirmed defect marks overall status `BLOCKED` with remediation findings; refuted finding is recorded as resolved with evidence.
3. **Execution failure** (`failed` on either lane): orchestrator evaluates for transient causes (timeout/quota) and MAY re-dispatch the single failing lane.
4. **Genuine ambiguity escalation**: contradictory interpretations of underspecified requirements that cannot be resolved from specs or design MUST set overall status `blocked` and escalate to the human via `AskQuestion`. (Design Decision 4.)

#### Scenario: Unanimous approval synthesizes PASSED verify.md
- GIVEN both `agy` and `cursor-agent` envelopes return `status: done` with no blocking defects
- WHEN the orchestrator reconciles verdicts
- THEN the overall verdict MUST be `PASSED` and `verify.md` MUST synthesize both compliance matrices

#### Scenario: Confirmed defect synthesizes BLOCKED verify.md
- GIVEN one or both envelopes return `status: blocked` with a confirmed defect
- WHEN the orchestrator reconciles verdicts
- THEN the overall verdict MUST be `BLOCKED` and `verify.md` MUST specify the required remediation tasks

#### Scenario: Refuted finding does not block verification
- GIVEN one envelope returns `status: blocked` on a finding that the orchestrator refutes with code evidence
- WHEN the orchestrator reconciles verdicts
- THEN the overall verdict MUST be `PASSED` and `verify.md` MUST document the refutation evidence

#### Scenario: Genuine spec ambiguity escalates to human
- GIVEN executors disagree on an underspecified requirement and documentation cannot arbitrate
- WHEN the orchestrator reconciles verdicts
- THEN it MUST escalate the decision to the human via `AskQuestion` and mark status `blocked`

### Requirement: Canonical Verification Report and State Update

The orchestrator MUST write the canonical verification synthesis to `openspec/changes/<change-id>/verify.md` and MUST update `openspec/changes/<change-id>/state.yaml`. (Design Decision 4.)

#### Scenario: Successful verification marks state done
- GIVEN `verify.md` synthesized with overall verdict `PASSED`
- WHEN the orchestrator updates change state
- THEN `openspec/changes/<change-id>/state.yaml` MUST update `verify: { status: done }` and mark the change ready for archiving

#### Scenario: Blocked verification marks state blocked
- GIVEN `verify.md` synthesized with overall verdict `BLOCKED`
- WHEN the orchestrator updates change state
- THEN `openspec/changes/<change-id>/state.yaml` MUST update `verify: { status: blocked }` and link to remediation tasks

### Requirement: Additive Rollback Without Ledger Migration

The verify dual-dispatch capability MUST be purely additive, requiring zero SQLite schema modifications and zero new ledger event types. Reverting the apply commit(s) MUST restore manual verification without orphaned database state. (Design Decision 5.)

#### Scenario: Rollback leaves database schema intact
- GIVEN the apply commit(s) for verify dual-dispatch are reverted
- WHEN the repository is inspected
- THEN existing ledger tables and historical run records MUST remain valid and readable with zero migration rollback
