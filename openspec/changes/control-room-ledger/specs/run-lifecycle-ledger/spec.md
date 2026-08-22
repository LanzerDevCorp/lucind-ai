# Run Lifecycle Ledger Specification

## Purpose

Persist each Control Room run as a durable row whose status is not inferred from lanes.

## Requirements

### Requirement: First-Class Run Persistence

The ledger MUST store a durable row in `runs` at CLI dispatch with status `running` and UTC `started_at`, and MUST update the row to terminal status with non-null UTC `ended_at` when all lanes complete. Run lifecycle status MUST NOT be derived solely by scanning lanes.

#### Scenario: Register run at dispatch

- GIVEN CLI dispatch minting a run ID
- WHEN the run is registered
- THEN a `runs` row is created with status `running` and UTC `started_at`.

#### Scenario: Run transitions to terminal status

- GIVEN a `running` run with active lanes
- WHEN all lanes reach terminal status
- THEN the `runs` row updates to terminal status with non-null UTC `ended_at`.

#### Scenario: Duplicate run registration rejected

- GIVEN an existing `run_id` in `runs`
- WHEN registering the duplicate `run_id`
- THEN the insert fails with a unique constraint error.

### Requirement: Primary-Root Isolation Preservation

`ledger.Open` MUST resolve database paths via `ledgerpath.Resolve` to `<primaryRoot>/.lucind/lucind.db`, and `lucind-ai run` and `lucind-ai serve` MUST exit with code 1 when invoked inside a linked worktree.

#### Scenario: Primary root resolves database path

- GIVEN execution from a primary repository root
- WHEN `ledger.Open` or `lucind-ai run` executes
- THEN the database path resolves to `<primaryRoot>/.lucind/lucind.db`.

#### Scenario: Subdirectory execution resolves to primary root

- GIVEN execution from a subdirectory in the primary repository
- WHEN `ledgerpath.Resolve` executes
- THEN the path resolves to the primary root `.lucind/lucind.db`.

#### Scenario: Linked worktree execution refused

- GIVEN execution of `lucind-ai run` or `lucind-ai serve` in a linked worktree
- WHEN the command starts
- THEN execution exits with code 1 and an error on stderr.
