# Verify Dual Dispatch Specification

## Purpose

Orchestrate the two-stage SDD `verify` workflow: dispatch parallel read-only qualitative
judgment lanes to `agy` and `cursor-agent`, reconcile the dual verdict envelopes against the
codebase, synthesize the canonical `verify.md` report, and update change state.

## Requirements

### Requirement: Two-Stage SDD Verify Protocol

`plugin/claude-code/skills/lucind-ai/SKILL.md:80` MUST be updated from "Target direction, not yet
built" to an operational two-stage verification workflow. Stage 1 MUST execute deterministic
mechanical checks (`lucind-ai check`) once and commit `verify-mechanical.log`. Stage 2 MUST
construct and dispatch dual read-only judgment packets to `agy` and `cursor-agent` in parallel.
Stage 1 MUST complete and succeed before stage 2 is initiated.

#### Scenario: SKILL.md documents the operational verify protocol
- GIVEN `plugin/claude-code/skills/lucind-ai/SKILL.md`
- WHEN row 80 and the verify workflow instructions are inspected
- THEN they MUST define the operational two-stage verify protocol and MUST NOT list `verify` as unbuilt or blocked

#### Scenario: Sequential stage ordering
- GIVEN an SDD change ready for verification
- WHEN `sdd-verify` is executed
- THEN stage 1 (`lucind-ai check`) MUST complete and succeed before stage 2 (dual packet dispatch) is initiated

### Requirement: Dual Parallel Judgment Dispatch and Barrier Join

The orchestrator MUST author two independent judgment packets
(`packets/verify-<change-id>-agy.md` and `packets/verify-<change-id>-cursor-agent.md`) and
dispatch them concurrently with a single `lucind-ai run` invocation. `lucind-ai run` MUST execute
both lanes in parallel, in isolated worktrees branched from the candidate HEAD, and MUST hold the
barrier until both lanes reach a terminal status (`Done`, `Blocked`, `Deviated`, or `Failed`).

#### Scenario: Single command dispatches both judgment lanes
- GIVEN prepared verify packets for `agy` and `cursor-agent`
- WHEN the orchestrator executes stage 2
- THEN it MUST invoke `lucind-ai run --packet packets/verify-<change-id>-agy.md --packet packets/verify-<change-id>-cursor-agent.md` in a single command

#### Scenario: Isolated worktrees created from candidate HEAD
- GIVEN the dual dispatch command
- WHEN `worktree.Create` provisions lane workspaces
- THEN `agy` and `cursor-agent` MUST each receive an independent, isolated worktree branched from candidate HEAD

#### Scenario: Barrier join waits for both lanes
- GIVEN both judgment lanes executing concurrently
- WHEN `lucind-ai run` reaches the barrier
- THEN it MUST block until both lanes have completed execution and completion-mode enforcement

### Requirement: Mechanical Log Context Embedding

The orchestrator MUST embed the summary status, execution duration, git commit SHA, and any
failure transcript from `verify-mechanical.log` into the `## Context` section of both judgment
packets prior to dispatch, so each executor evaluates the identical frozen mechanical outcome
without an extra file read.

#### Scenario: Mechanical summary embedded in packet context
- GIVEN `verify-mechanical.log` generated from stage 1
- WHEN the orchestrator authors `packets/verify-<change-id>-agy.md` and `packets/verify-<change-id>-cursor-agent.md`
- THEN the `## Context` section of both packets MUST contain the frozen mechanical check summary

#### Scenario: Executors evaluate in-prompt context without file re-reads
- GIVEN an LLM executor running in a judgment worktree
- WHEN assessing mechanical check outcomes
- THEN the executor MUST find the mechanical outcome directly in its prompt context without needing an additional file-read tool call

### Requirement: Independent Evidence Cross-Checking

The orchestrator MUST independently verify every `file:line` citation, test assertion, and
defect claim in both judgment envelopes against the actual codebase before accepting a finding
into `verify.md`. Green criteria alone MUST NOT be treated as proof of complete work
(`SKILL.md:102`).

#### Scenario: Cited defect confirmed against the codebase
- GIVEN an envelope reporting a defect with citation `internal/auth/session.go:84`
- WHEN the orchestrator reconciles verdicts
- THEN it MUST inspect `internal/auth/session.go:84` in the codebase to confirm the defect before marking the overall verdict blocked

#### Scenario: False positive refuted with code evidence
- GIVEN an envelope reporting a missing boundary check
- WHEN the orchestrator inspects the codebase and finds the check implemented at an upstream layer
- THEN it MUST record the finding as refuted in `verify.md` with concrete `file:line` citations and MUST NOT block on it

### Requirement: Unanimous Pass Reconciliation

When both judgment envelopes report `status: done` and neither reports a blocking spec
violation, the orchestrator MUST mark the overall verification outcome `PASSED` in `verify.md`,
consolidating non-blocking suggestions or complementary findings from both lanes.

#### Scenario: Both lanes pass cleanly
- GIVEN `agy` reports `status: done` and `cursor-agent` reports `status: done` with no blocking defects
- WHEN the orchestrator reconciles verdicts
- THEN the overall outcome MUST be `PASSED` and `verify.md` MUST synthesize both compliance matrices

#### Scenario: Complementary non-blocking findings consolidated
- GIVEN `agy` reports a non-blocking documentation gap and `cursor-agent` reports a non-blocking test coverage suggestion
- WHEN the orchestrator synthesizes `verify.md`
- THEN `verify.md` MUST consolidate both findings under non-blocking observations with overall status `PASSED`

### Requirement: Disagreement and False-Positive Adjudication

When one or both judgment envelopes report `blocked` or `deviated`, the orchestrator MUST
independently verify each disputed finding against the codebase and specifications. A confirmed
spec violation MUST mark the overall outcome `BLOCKED` in `verify.md` with remediation guidance
and queue corrective tasks in `state.yaml`. A demonstrable false positive MUST be explicitly
refuted in `verify.md` with exact `file:line` citations and marked resolved without blocking the
change.

#### Scenario: Confirmed defect blocks verify
- GIVEN `agy` reports `status: blocked` citing missing error handling at `internal/auth/auth.go:45`, and the orchestrator verifies the gap against `spec.md`
- WHEN the orchestrator synthesizes `verify.md`
- THEN the overall outcome MUST be `BLOCKED`, the finding MUST be recorded with remediation guidance, and `state.yaml` MUST reflect blocked status

#### Scenario: Demonstrable false positive refuted with evidence
- GIVEN `cursor-agent` reports `status: blocked` claiming a missing boundary check, but the orchestrator verifies the check exists at `internal/auth/validator.go:28`
- WHEN the orchestrator synthesizes `verify.md`
- THEN `verify.md` MUST record the finding as refuted with `internal/auth/validator.go:28` evidence, and it MUST NOT block the overall `PASSED` verdict

### Requirement: Lane Execution Failure Handling

When a judgment lane reports `status: failed` due to an infrastructure error, timeout, or
context exhaustion, the orchestrator MUST NOT treat the failure as a codebase defect. The
orchestrator MAY re-dispatch the single failing judgment lane before final synthesis.

#### Scenario: Re-dispatching a single failed lane
- GIVEN `agy` finishes with `status: done` and `cursor-agent` fails due to timeout
- WHEN the orchestrator evaluates the batch
- THEN it MAY re-run only the `cursor-agent` judgment packet before synthesizing `verify.md`

### Requirement: Irreconcilable Ambiguity Escalation

When the two judgment executors produce contradictory interpretations of an underspecified
requirement that cannot be resolved from accepted design or spec documents — a case distinct
from an adjudicable disagreement, since here the orchestrator itself cannot determine which
interpretation is correct — the orchestrator MUST set the overall outcome to `BLOCKED` and
escalate the decision to the human operator.

#### Scenario: Ambiguity escalation to the human
- GIVEN `agy` and `cursor-agent` provide mutually contradictory interpretations of an ambiguous spec requirement and neither is refuted by the codebase or design document
- WHEN the orchestrator attempts reconciliation
- THEN `verify.md` MUST record the unresolved ambiguity, set overall outcome to `BLOCKED`, and present the decision options to the human

### Requirement: Canonical Verification Report and State Update

The orchestrator MUST synthesize a single canonical verification report at
`openspec/changes/<change-id>/verify.md`, combining the frozen mechanical log and the qualitative
findings from both envelopes, and MUST update `openspec/changes/<change-id>/state.yaml`
accordingly. `verify.md` serves as the authoritative gate record consumed by the human reviewer,
`state.yaml`, and `openspec archive`.

#### Scenario: Canonical report synthesis content
- GIVEN a completed `verify-mechanical.log` and two judgment envelopes for change `add-oauth`
- WHEN the orchestrator reconciles verdicts
- THEN `openspec/changes/add-oauth/verify.md` MUST be created containing mechanical check status, spec compliance evaluation, and consolidated findings

#### Scenario: Successful verification marks state done
- GIVEN `verify.md` synthesized with overall verdict `PASSED`
- WHEN the orchestrator updates change state
- THEN `state.yaml` MUST update `verify: { status: done }` and mark the change ready for archiving

#### Scenario: Blocked verification marks state blocked
- GIVEN `verify.md` synthesized with overall verdict `BLOCKED`
- WHEN the orchestrator updates change state
- THEN `state.yaml` MUST update `verify: { status: blocked }` and link to remediation tasks

### Requirement: Additive Rollback Without Ledger Migration

Verify dual-dispatch MUST be purely additive: zero SQLite schema modifications, zero new ledger
event types, zero envelope schema version bumps. Reverting the apply commit(s) touching
`cmd/lucind-ai/cli.go` and `plugin/claude-code/skills/lucind-ai/` MUST fully restore the prior
manual/single-model verification workflow without orphaning any database state.

#### Scenario: Rollback leaves the database schema intact
- GIVEN the apply commit(s) for verify-dual-dispatch are reverted
- WHEN the repository is inspected
- THEN existing ledger tables and historical run records MUST remain valid and readable with zero migration rollback
