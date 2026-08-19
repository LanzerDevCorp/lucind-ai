# Lane Approval Wait Specification

## Purpose

Provide a blocking gate halting lane status persistence until an approval decision or timeout.

## Requirements

### Requirement: Blocking Approval Gate

When enabled, the system MUST pause `done` lanes until an approval decision is recorded.

#### Scenario: Approved decision unblocks lane

- GIVEN a lane with computed status `done`
- WHEN an approver records `approved`
- THEN the system MUST persist status `done` and resume.

#### Scenario: Rejected decision blocks lane

- GIVEN a lane awaiting approval
- WHEN an approver records `rejected`
- THEN the system MUST persist status `blocked` and resume.

### Requirement: Approval Timeout

The system MUST enforce a configurable timeout and MUST NOT auto-approve.

#### Scenario: Timeout elapses

- GIVEN a waiting lane with an active timeout
- WHEN timeout elapses without a decision
- THEN the system MUST record `timed_out` and persist status `blocked`.

#### Scenario: Zero timeout bypass

- GIVEN approval timeout is zero
- WHEN a lane computes status `done`
- THEN the system MUST bypass the gate and persist immediately.
