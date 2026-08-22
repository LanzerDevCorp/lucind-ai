# Delta for Batch Wave View

## ADDED Requirements

### Requirement: Batch and DAG Wave Inspection

The dashboard UI MUST display active batch execution status, DAG wave grouping,
per-lane lifecycle status (`pending`, `running`, `done`, `blocked`, `deviated`,
`failed`), assigned executor, worktree directory path, per-lane execution
deadline, and barrier release state (Released, integration eligibility for
completed lanes, and preservation for non-done worktrees).

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
