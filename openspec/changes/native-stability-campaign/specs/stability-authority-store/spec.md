# Delta for Stability Authority Store

## ADDED Requirements

### Requirement: Common-directory SQLite and WAL authority

Mutable campaign and trial lifecycle state MUST be stored in SQLite/WAL at `<git-common-dir>/lucind-ai/stability/v1/stability.db`, isolated from the primary run ledger at `<primaryRoot>/.lucind/lucind.db`.

#### Scenario: Isolated storage location

- GIVEN an active stability campaign
- WHEN lifecycle states are updated
- THEN updates MUST persist to `<git-common-dir>/lucind-ai/stability/v1/stability.db` without altering `<primaryRoot>/.lucind/lucind.db`

#### Scenario: Crash recovery from WAL

- GIVEN uncheckpointed WAL transactions after process crash
- WHEN the store reopens
- THEN SQLite WAL recovery MUST restore consistent campaign state

### Requirement: Single-active campaign constraint

The authority store MUST enforce a single-active campaign gate rejecting initialization of a new campaign when an unclosed campaign record exists.

#### Scenario: Concurrent campaign rejection

- GIVEN an active campaign in progress
- WHEN a second `lucind-ai stability run` executes
- THEN the transaction MUST reject the second run non-zero without mutating active state
