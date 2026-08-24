# Batch Wave View Specification

## Purpose

Serve a dashboard view for inspecting active batch execution status, DAG wave grouping, per-lane lifecycle status and deadlines, assigned executors, worktree directory paths, and barrier release state.

## Requirements

### Requirement: Batch and DAG Wave Inspection

The dashboard UI MUST display active batch execution status, DAG wave grouping,
per-lane lifecycle status (`pending`, `running`, `done`, `blocked`, `deviated`,
`failed`), assigned executor, worktree directory path, per-lane execution
deadline, barrier release state (Released, integration eligibility for
completed lanes, and preservation for non-done worktrees), lane dispatch
metadata (model, agent, SDD phase, fanout group, feature, and skill when
present), a link to the dispatched packet body, structured progress telemetry
(token totals, USD cost, and tool rates when emitted), and diagnostic notes
for swept-orphan failures.
(Previously: Dashboard displayed status, wave grouping, executor, worktree path, deadline, and barrier state without lane dispatch metadata, packet body links, structured telemetry, or swept-orphan notes.)

#### Scenario: Wave grouping and lane lifecycle inspection

- GIVEN an active batch execution with multi-wave DAG packet dependencies
- WHEN the operator inspects the batch-wave view
- THEN each lane MUST display status, assigned executor, worktree path, DAG
  wave group, and deadline

#### Scenario: Barrier release with mixed terminal statuses

- GIVEN an evaluated batch with one `done` lane and one `failed` or `deviated` lane
- WHEN barrier evaluation completes
- THEN the UI MUST display Released status, mark the `done` lane as
  integration-eligible, and show non-done worktrees as preserved

#### Scenario: Lane card metadata, packet link, and telemetry inspection

- GIVEN a lane dispatched with metadata and emitting progress telemetry
- WHEN the operator inspects the batch-wave view
- THEN the UI MUST render populated metadata fields, a link to the dispatched packet body, and numeric token, cost, and tool metrics

#### Scenario: Swept-orphan lane inspection

- GIVEN a lane transitioned to `failed` by the orphan sweep
- WHEN the operator inspects the batch-wave view
- THEN the UI MUST display the lane in `failed` status and render the explanatory sweep note

