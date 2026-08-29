# Delta for phase-specialist-dispatch

## ADDED Requirements

### Requirement: Specialist sequencing and canonical artifact generation

The phase specialist MUST ingest `gentle-ai sdd-status` JSON, dispatch child lanes through lucind-ai, MUST NOT start synthesis until all required planning lenses are accepted and merged, and MUST land canonical phase artifacts at `openspec/changes/<change>/<phase>.md`.

#### Scenario: Fan-out lenses merged before synthesis dispatch

- GIVEN an active propose phase with all required lenses (`lens-a`, `lens-b`, `lens-c`) accepted and merged
- WHEN the phase specialist checks `gentle-ai sdd-status`
- THEN the specialist MUST dispatch the propose synthesis lane for `openspec/changes/<change>/propose.md`.

#### Scenario: Unchanged phase state generates no dispatches

- GIVEN `gentle-ai sdd-status` reporting phase complete with canonical artifact present
- WHEN the phase specialist inspects lifecycle state
- THEN the specialist MUST complete without dispatching redundant lanes.

#### Scenario: Synthesis blocked while lenses unmerged

- GIVEN an active propose phase with an unmerged lens
- WHEN the phase specialist evaluates next action
- THEN the specialist MUST NOT dispatch synthesis and MUST wait.
