# Delta for defect-records

## ADDED Requirements

### Requirement: Ledger schema v8 persistence for defect records

The ledger MUST migrate to `schemaVersion = 8` (`internal/ledger/schema.go:10`) and create a `defect_records` table with columns `(id, feature_id, run_id, lane_id, error_signature, evidence, disposition, created_at, updated_at)` and a `CHECK` constraint on `disposition IN ('recorded','repaired','declined','deferred')`. An index `idx_defect_records_feature` on `(feature_id, id)` MUST be created. The migration step `migrateV7ToV8DDL` MUST be idempotent and backwards-compatible with existing schema tables.

#### Scenario: Idempotent migration from v7 to v8

- GIVEN a ledger database at schema version 7
- WHEN `migrate` executes during `ledger.Open`
- THEN the ledger MUST upgrade `schemaVersion` to 8 and create the `defect_records` table and index without modifying existing tables

#### Scenario: Invalid disposition rejected by database constraint

- GIVEN a candidate defect record with an unsupported disposition string
- WHEN insertion is attempted in the `defect_records` table
- THEN the database CHECK constraint MUST reject the insertion

### Requirement: Non-critical non-blocking defect persistence

When a pre-existing defect is determined to be neither critical nor blocking for any active feature branch, ultrafixer MUST persist a Defect Record with disposition `recorded` via `Ledger.RecordDefect` (`internal/ledger/ledger.go`). Ultrafixer MUST NOT generate a repair commit, fix Change, or modify workspace code, and MUST emit a `done` result envelope.

#### Scenario: Non-critical non-blocking defect recorded without code changes

- GIVEN a pre-existing defect that is neither critical nor blocking for any active feature branch
- WHEN ultrafixer completes evaluation
- THEN ultrafixer MUST insert a Defect Record with disposition `recorded` into the ledger, touch no workspace files, and emit a `done` envelope

#### Scenario: Defect record stores complete error signature and evidence

- GIVEN a non-critical defect with stack trace and command failure evidence
- WHEN `Ledger.RecordDefect` is invoked
- THEN the persisted record MUST contain the exact error signature, evidence, feature ID, and timestamp

### Requirement: Defect record query and retrieval API

The ledger package MUST provide `RecordDefect`, `ListDefects`, and `GetDefect` methods (`internal/ledger/ledger.go`). `ListDefects` MUST return all defect records for a specified `feature_id` ordered chronologically by `created_at`. `GetDefect` MUST return a single `DefectRecord` by primary key `id`.

#### Scenario: List defect records for a feature

- GIVEN multiple defect records recorded across different features in the ledger
- WHEN `ListDefects` is queried for a specific `feature_id`
- THEN it MUST return only the defect records associated with that feature ordered by `created_at`

#### Scenario: Retrieve single defect record by ID

- GIVEN an existing defect record ID
- WHEN `GetDefect` is called with that ID
- THEN it MUST return the matching `DefectRecord` or an error if not found

### Requirement: CLI defect inspection commands

The CLI MUST provide `lucind-ai defect record` and `lucind-ai defect list` subcommands (`cmd/lucind-ai/cli.go`). `lucind-ai defect list --feature <id>` MUST query `Ledger.ListDefects` and format the list of recorded defects on stdout. `lucind-ai defect record` MUST accept defect metadata and insert a record via `Ledger.RecordDefect`.

#### Scenario: CLI lists defects for feature

- GIVEN recorded defects for feature `feat-123` in the ledger
- WHEN the operator executes `lucind-ai defect list --feature feat-123`
- THEN the command MUST output the recorded defect signatures, IDs, and dispositions

#### Scenario: CLI records defect from arguments

- GIVEN a feature ID, error signature, and evidence
- WHEN the operator executes `lucind-ai defect record` with those flags
- THEN a new defect record MUST be inserted into the ledger
