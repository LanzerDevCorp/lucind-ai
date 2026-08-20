# Lane Approval Wait Specification

## Purpose

A blocking gate, with a configurable timeout, that halts a lane's terminal status persistence
until an approval decision is recorded — and the ledger record that decision lives in.

## Requirements

### Requirement: Blocking Approval Gate

When enabled, the system MUST pause a lane whose computed status is `done` until an approval
decision is recorded, and MUST NOT auto-approve.

#### Scenario: Approved decision persists done

- GIVEN a lane with computed status `done`
- WHEN an approver records `approved`
- THEN the system MUST persist status `done`.

#### Scenario: Rejected decision blocks the lane

- GIVEN a lane awaiting approval
- WHEN an approver records `rejected`
- THEN the system MUST persist status `blocked`.

### Requirement: Approval Timeout

The system MUST enforce a configurable timeout. When it elapses without a decision, the lane MUST
persist as `blocked` with decision `timed_out`. When the configured timeout is zero, the gate MUST
be bypassed and the lane MUST persist immediately.

#### Scenario: Timeout elapses

- GIVEN a waiting lane with an active timeout
- WHEN the timeout elapses without a decision
- THEN the system MUST record `timed_out` and persist status `blocked`.

#### Scenario: Zero timeout bypasses the gate

- GIVEN the approval timeout is configured to zero
- WHEN a lane computes status `done`
- THEN the system MUST bypass the wait and persist status `done` immediately.

### Requirement: Approval Ledger Record

Every decision MUST be one of `pending | approved | rejected | timed_out`. The ledger MUST record
who decided and when. A later-surfaced defect MUST only be linked via an explicit later mark —
never inferred automatically from a subsequent packet's own status.

#### Scenario: Approve records who and when

- GIVEN a pending approval
- WHEN the user approves
- THEN the ledger MUST store `approved` with the approver's identity and a timestamp.

#### Scenario: Later failure is not auto-inferred

- GIVEN an approved row, and a later packet with the same `packet_id` later reports `failed`
- WHEN that later status is persisted
- THEN `defect_surfaced_later` MUST stay unset unless an operator explicitly marks it.
