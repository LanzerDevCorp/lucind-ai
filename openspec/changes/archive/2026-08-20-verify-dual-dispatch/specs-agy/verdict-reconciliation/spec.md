# Verdict Reconciliation Specification

## Purpose

Specify orchestrator synthesis of dual judgment envelopes and the frozen mechanical check log into the canonical `openspec/changes/<change-id>/verify.md` report, and define the arbitration rules for consensus, disagreements, false-positive refutations, execution failures, and human escalations.

## Requirements

### Requirement: Canonical Verification Report Generation

The orchestrator MUST synthesize a single canonical verification report at `openspec/changes/<change-id>/verify.md` combining the frozen mechanical check log from `verify-mechanical.log` and the qualitative findings from both `agy` and `cursor-agent` result envelopes. (Design Decision 4.)

#### Scenario: Canonical report synthesis path and content
- GIVEN a completed `verify-mechanical.log` and two judgment envelopes for change `add-oauth`
- WHEN the orchestrator reconciles verdicts
- THEN `openspec/changes/add-oauth/verify.md` MUST be created containing mechanical check status, spec compliance evaluation, and consolidated findings

#### Scenario: Terminal consumer of canonical verify report
- GIVEN a synthesized `verify.md` report
- WHEN evaluated by the human reviewer, `state.yaml` updater, or `openspec archive`
- THEN it MUST serve as the authoritative gate record for the verified change

### Requirement: Unanimous Pass Reconciliation

When both judgment envelopes report `status: done` and neither reports blocking spec violations, the orchestrator MUST mark the overall verification outcome `PASSED` in `verify.md`, consolidate non-blocking suggestions or complementary findings, and update `state.yaml` to mark `verify` completed. (Design Decision 4.)

#### Scenario: Both lanes pass cleanly
- GIVEN `agy` reports `status: done` and `cursor-agent` reports `status: done` with no blocking defects
- WHEN the orchestrator reconciles verdicts
- THEN `verify.md` overall outcome MUST be `PASSED` and `state.yaml` MUST transition `verify` to done

#### Scenario: Complementary non-blocking findings consolidated
- GIVEN `agy` reports a non-blocking documentation gap and `cursor-agent` reports a non-blocking test coverage suggestion
- WHEN the orchestrator synthesizes `verify.md`
- THEN `verify.md` MUST consolidate both findings under non-blocking observations with overall status `PASSED`

### Requirement: Disagreement and False-Positive Adjudication

When one or both judgment envelopes report `blocked`, `deviated`, or raise findings, the orchestrator MUST independently verify each disputed finding against the codebase and specifications per `SKILL.md:102`. A confirmed spec violation MUST mark the overall outcome `BLOCKED` in `verify.md` and queue corrective tasks in `state.yaml`. A demonstrable false positive MUST be explicitly refuted in `verify.md` with exact `file:line` citations and marked resolved without blocking the change. (Design Decision 4.)

#### Scenario: Confirmed defect blocks verify
- GIVEN `agy` reports `status: blocked` citing missing error handling at `internal/auth/auth.go:45`, and the orchestrator verifies the gap against `spec.md`
- WHEN the orchestrator synthesizes `verify.md`
- THEN overall outcome MUST be `BLOCKED`, the finding MUST be recorded with remediation guidance, and `state.yaml` MUST reflect blocked status

#### Scenario: Demonstrable false positive refuted with evidence
- GIVEN `cursor-agent` reports `status: blocked` claiming a missing boundary check, but the orchestrator verifies the check exists at `internal/auth/validator.go:28`
- WHEN the orchestrator synthesizes `verify.md`
- THEN `verify.md` MUST record the finding as refuted with `internal/auth/validator.go:28` evidence, and the false positive MUST NOT block the overall `PASSED` verdict

### Requirement: Lane Execution Failure Handling

When a judgment lane reports `status: failed` due to an infrastructure error, timeout, or context exhaustion, the orchestrator MUST NOT treat the failure as a codebase defect. The orchestrator MAY re-dispatch the single failing judgment lane before final synthesis. (Design Decision 4.)

#### Scenario: Re-dispatching a single failed lane
- GIVEN `agy` finishes with `status: done` and `cursor-agent` fails due to timeout
- WHEN the orchestrator evaluates the batch
- THEN it MAY re-run only the `cursor-agent` judgment packet before synthesizing `verify.md`

### Requirement: Irreconcilable Ambiguity Escalation

When the two judgment executors produce contradictory interpretations of an underspecified requirement or architectural conflict that cannot be resolved from accepted design or spec documents, the orchestrator MUST set the overall outcome to `BLOCKED` and escalate the decision to the human operator. (Design Decision 4.)

#### Scenario: Ambiguity escalation to human
- GIVEN `agy` and `cursor-agent` provide mutually contradictory interpretations of an ambiguous spec requirement and neither is refuted by the codebase or design document
- WHEN the orchestrator attempts reconciliation
- THEN `verify.md` MUST record the unresolved ambiguity, set overall outcome to `BLOCKED`, and present the decision options to the human

### Requirement: Additive Rollback and Migration-Free Operation

Verification dual dispatch MUST NOT introduce SQLite ledger migrations, envelope schema version bumps, or state database schema changes. Reverting the CLI subcommand in `cmd/lucind-ai/cli.go` and template assets in `plugin/claude-code/skills/lucind-ai/assets/` MUST completely restore prior single-model verify behavior without breaking existing ledger records. (Design Decision 5.)

#### Scenario: Revert leaves ledger intact
- GIVEN historical runs on the SQLite ledger
- WHEN verify-dual-dispatch commits are reverted
- THEN existing ledger tables and run histories MUST remain valid and queryable
