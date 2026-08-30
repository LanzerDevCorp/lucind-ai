# Phase Specialist Dispatch Specification

## Purpose

Specify phase specialist lifecycle state tracking, synthesis sequencing, and canonical artifact generation.

## Requirements

### Requirement: Specialist sequencing and canonical artifact generation

The phase Specialist MUST ingest `gentle-ai sdd-status` JSON, MUST NOT start synthesis until all required planning lenses are accepted and merged, and MUST land canonical phase artifacts at `openspec/changes/<change>/` under the canonical per-phase filename (`proposal.md`, `spec.md`, `design.md`, `tasks.md`, `apply.md`, `verify.md`, `remediate.md`, `archive.md`). Named `sdd-*` Specialists MUST independently decide Acceptance for Lanes in their assigned phase without human confirmation and MUST direct the Orchestrator to execute the corresponding `lucind-ai run` and `lucind-ai accept` invocations. Ordinary delegated workers MUST NOT decide Acceptance or direct acceptance execution. Promotion MUST remain human-confirmed and MUST NOT be decided or requested by any Specialist or delegated worker. For Tier A Changes, Specialist Acceptance decisions MUST enforce Dual-Judge evaluation before acceptance. The Hard Rule MUST carve out named `sdd-*` Specialists for Acceptance of their own phase's Lanes only. The existing status and dispatch adapter remains the mechanical inspection tool.
(Previously: The specialist was a deterministic sequencer that ingested status JSON and dispatched child lanes, without phase-scoped Acceptance authority.)

#### Scenario: Fan-out lenses merged before synthesis dispatch

- GIVEN an active propose phase with all required lenses (`lens-a`, `lens-b`, `lens-c`) accepted and merged
- WHEN the phase specialist checks `gentle-ai sdd-status`
- THEN the specialist MUST dispatch the propose synthesis lane for `openspec/changes/<change>/proposal.md`.

#### Scenario: Unchanged phase state generates no dispatches

- GIVEN `gentle-ai sdd-status` reporting phase complete with canonical artifact present
- WHEN the phase specialist inspects lifecycle state
- THEN the specialist MUST complete without dispatching redundant lanes.

#### Scenario: Synthesis blocked while lenses unmerged

- GIVEN an active propose phase with an unmerged lens
- WHEN the phase specialist evaluates next action
- THEN the specialist MUST NOT dispatch synthesis and MUST wait.

#### Scenario: Specialist independently accepts phase planning lanes

- GIVEN a completed planning lane with a schema-valid result envelope, passing qualitative checks, and clean declared scope
- WHEN the assigned phase Specialist decides Acceptance
- THEN the Orchestrator mechanically records the acceptance receipt and the lane is accepted without requesting human confirmation

#### Scenario: Tier A change requires dual-judge evaluation for specialist acceptance

- GIVEN a completed lane in a Tier A Change evaluated independently by two distinct model architectures
- WHEN the two judges disagree on compliance or safety during acceptance evaluation
- THEN acceptance is blocked and the lane is not accepted until the evaluation divergence is resolved

#### Scenario: Ordinary worker agent denied acceptance authority

- GIVEN an ordinary delegated worker executing a lane
- WHEN the worker completes lane execution and attempts to issue an acceptance decision
- THEN acceptance authority is denied and the worker is prohibited from accepting the lane

#### Scenario: Specialist prohibited from change promotion

- GIVEN a completed Change with all required phase artifacts accepted
- WHEN the Change is ready for integration into its declared Integration Target
- THEN automated promotion is blocked and explicit human confirmation is required
