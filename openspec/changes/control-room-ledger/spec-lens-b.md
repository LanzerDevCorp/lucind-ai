# Spec Lens B — Scenarios & Coverage: Control Room Ledger

## Assumed requirements

This change adds durable run lifecycle tracking, sequenced progress streaming, isolated progress pruning, and typed serve DTOs to the Control Room ledger. We assume six requirements from the proposal: `First-class run persistence` (`run-lifecycle-ledger`), `Lane dispatch metadata` (`lane-execution`), `Progress ingest and cursor tail` (`lane-progress-stream`), `Isolated progress pruning` (`progress-stream-pruning`), `Shell-free run DTOs` (`approvals-web-ui`), and `Primary-root isolation (preserve)`.

## Scenarios

### Requirement: First-class run persistence

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

### Requirement: Lane dispatch metadata

#### Scenario: Persist metadata on lane registration

- GIVEN a packet declaring `model`, `agent`, and `feature`
- WHEN `RegisterLane` executes
- THEN the lane row persists those column values.

#### Scenario: Schema v6 migration preserves existing lanes

- GIVEN a schema v5 database with existing lanes
- WHEN transactional `migrate` runs to v6
- THEN existing lanes are preserved with null or empty metadata.

#### Scenario: Unadmitted event type rejected

- GIVEN a schema v6 database
- WHEN appending an event with an unadmitted type
- THEN the insert fails with a CHECK constraint error.

### Requirement: Progress ingest and cursor tail

#### Scenario: Ascending tail read after cursor

- GIVEN chunks 1–10 in `lane_progress` for a lane
- WHEN querying progress tail with `afterSeq = 5`
- THEN chunks 6–10 return in ascending order.

#### Scenario: Cursor at latest sequence returns empty

- GIVEN chunks 1–10 in `lane_progress` for a lane
- WHEN querying progress tail with `afterSeq = 10`
- THEN an empty slice returns with no error.

#### Scenario: Duplicate sequence append rejected

- GIVEN sequence 1 exists for a run and lane
- WHEN appending a chunk with duplicate sequence 1
- THEN the insert fails with a primary key error.

### Requirement: Isolated progress pruning

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

### Requirement: Shell-free run DTOs

#### Scenario: Typed run and progress queries

- GIVEN persisted runs and progress in SQLite
- WHEN `serve.Model` queries run summaries or progress tails
- THEN typed DTO structs return directly from SQLite without subprocess or git calls.

#### Scenario: Query unknown run returns empty DTO

- GIVEN an unknown `run_id`
- WHEN `serve.Model` queries the run summary
- THEN a not-found error or empty DTO returns without invoking any shell command.

#### Scenario: Database error returns typed error

- GIVEN a broken SQLite connection backing `serve.Model`
- WHEN a run summary query executes
- THEN `serve.Model` returns a database error without shell fallback.

### Requirement: Primary-root isolation (preserve)

#### Scenario: Primary root resolves database path

- GIVEN execution from a primary repository root
- WHEN `ledger.Open` or `lucind-ai run` executes
- THEN the database path resolves to `<primaryRoot>/.lucind/lucind.db`.

#### Scenario: Subdirectory execution resolves to primary root

- GIVEN execution from a subdirectory in the primary repository
- WHEN `ledgerpath.Resolve` executes
- THEN the path resolves to root `.lucind/lucind.db`.

#### Scenario: Linked worktree execution refused

- GIVEN execution of `lucind-ai run` or `lucind-ai serve` in a linked worktree
- WHEN the command starts
- THEN execution exits with code 1 and an error on stderr.

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| First-class run persistence | covered | covered | covered | `cmd/lucind-ai/cli.go:282-290` (new seam: `internal/ledger/runs.go`) |
| Lane dispatch metadata | covered | covered | covered | `internal/ledger/ledger.go:255-282`, `internal/ledger/schema.go:38-39` |
| Progress ingest and cursor tail | covered | covered | covered | `internal/run/run.go:422-434` (new seam: `internal/ledger/progress.go`) |
| Isolated progress pruning | covered | covered | covered | `internal/ledger/ledger.go:877-890` (new seam: `internal/ledger/progress.go`) |
| Shell-free run DTOs | covered | covered | covered | `internal/serve/model.go:14-25`, `internal/serve/handlers.go:79-85` |
| Primary-root isolation (preserve) | covered | covered | covered | `internal/ledgerpath/ledgerpath.go:36-38`, `cmd/lucind-ai/cli.go:277-280` |

## Untestable Assertions

None. All scenarios assert concrete, observable states via database queries, return values, or process exit codes.

## Open Questions

- [ ] Should `lane_progress` auto-pruning be triggered periodically by `lucind-ai serve` or on-demand via CLI commands?
- [ ] Should run status transitions emit `run_status_changed` via `AppendEvent` (`internal/ledger/ledger.go:366-381`) or via dedicated transaction helpers?
- [ ] Should progress `message` remain a raw text string or adopt structured JSON distinguishing stdout, stderr, and system events?
