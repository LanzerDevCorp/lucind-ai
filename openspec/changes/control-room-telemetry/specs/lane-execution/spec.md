# Delta for Lane Execution

## ADDED Requirements

### Requirement: High-Frequency SQLite Ledger Isolation

High-frequency stdout and stderr stream chunks MUST NOT be inserted into the SQLite `events` table. The ledger MUST record only the six admitted lifecycle types (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`), and failure diagnostics in `lane_note` rows MUST remain bounded to at most 4096 bytes per stream with truncation markers.

#### Scenario: High-volume output stays off SQLite ledger

- GIVEN a lane generating multi-megabyte stdout during dispatch
- WHEN the child process executes to completion
- THEN raw output MUST exist only in the worktree log and in-memory hub while `events` contains only coarse lifecycle rows

#### Scenario: Terminal failure truncates diagnostic note

- GIVEN a failed lane execution where stderr exceeds 4096 bytes
- WHEN execute records the failure note in the ledger
- THEN the `lane_note` event detail MUST truncate stderr to 4096 bytes with a truncation marker while full output is preserved in the worktree log

#### Scenario: Invalid event type rejected by ledger schema

- GIVEN an attempt to write a streaming chunk directly as a ledger event
- WHEN an event is appended with an unlisted event type
- THEN SQLite MUST reject the insertion with a CHECK constraint error
