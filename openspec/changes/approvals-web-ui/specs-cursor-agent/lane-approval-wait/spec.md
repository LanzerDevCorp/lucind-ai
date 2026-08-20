# Lane Approval Wait Specification

## Purpose

Timeout gate between computed and terminal persist.

## Requirements

### Requirement: Blocking Gate

If wait is enabled and status is `done`, Execute MUST wait before `SetStatus`. Timeout or reject MUST yield `blocked`, never auto-approve. Zero timeout skips wait.

#### Scenario: Wait then approve
- GIVEN `decideStatus` is `done` and wait is enabled
- WHEN the user approves
- THEN Execute MUST wait, then `SetStatus` `done`

#### Scenario: Timeout or reject blocks
- GIVEN a pending approval
- WHEN the deadline elapses or the user rejects
- THEN the lane MUST be `blocked` with `timed_out` or `rejected`

### Requirement: Approval Ledger

Decisions MUST be `pending|approved|rejected|timed_out`. Ledger MUST store who, when, and `defect_surfaced_later` only via a later mark. `lane.Status` MUST stay six values; approvals are a separate table.
<!-- Alt: omit naming `running`. -->

#### Scenario: Approve records who and when
- GIVEN a pending approval
- WHEN the user approves
- THEN it MUST store `approved` with approver and time; lane stays `running` until `SetStatus`

#### Scenario: Later failure not inferred
- GIVEN an approved row and a later packet is `failed`
- WHEN that later status is persisted
- THEN `defect_surfaced_later` MUST stay unset unless marked
