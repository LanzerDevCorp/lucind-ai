# Lane Progress Stream Specification

## Purpose

Store sequenced mid-flight progress chunks separately from the audit event log.

## Requirements

### Requirement: Progress Ingest and Cursor Tail

The ledger MUST append sequenced mid-flight progress chunks to `lane_progress` with `(run_id, lane_id, seq)` and MUST return chunks with `seq > afterSeq` in strictly ascending sequence order without querying or appending to `events`.

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
