# Progress Stream Pruning Specification

## Purpose

Expire progress chunks by time cutoff without touching governance or lifecycle rows.

## Requirements

### Requirement: Isolated Progress Cutoff Pruning

The ledger MUST delete `lane_progress` rows older than a specified cutoff timestamp without deleting, modifying, or cascading onto rows in `runs`, `lanes`, `events`, or `approvals`.

#### Scenario: Prune expired progress only

- GIVEN `lane_progress` rows older than `T` alongside active runs, lanes, and approvals
- WHEN progress pruning runs with cutoff `T`
- THEN only `lane_progress` rows older than `T` are deleted
- AND all `runs`, `lanes`, `events`, and `approvals` remain intact.

#### Scenario: Prune with cutoff before all rows

- GIVEN all `lane_progress` rows newer than cutoff `T`
- WHEN progress pruning runs with cutoff `T`
- THEN 0 rows are deleted with no error.

#### Scenario: Prune on closed database fails

- GIVEN a closed ledger database handle
- WHEN progress pruning runs
- THEN it returns a non-nil error.
