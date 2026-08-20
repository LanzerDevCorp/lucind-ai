# Lane Execution Specification

## Purpose

Hook approval waiting into lane execution before persistence and barrier observation, storing decisions additively.

## Requirements

### Requirement: Lifecycle Hook Ordering

The system MUST execute approval wait after status computation, before ledger persistence, and before barrier observation.

#### Scenario: Approval before barrier observation

- GIVEN a batch lane computes `done`
- WHEN approval wait resolves
- THEN the system MUST persist ledger status before invoking barrier observation.

#### Scenario: Non-approval unblocks barrier

- GIVEN a batch lane waiting for approval
- WHEN the wait resolves to `rejected` or `timed_out`
- THEN the system MUST persist status `blocked` and notify the barrier.

### Requirement: Additive Schema Recording

The system MUST store approval records in an additive table while preserving standard `lane.Status` values.

#### Scenario: Persist approval record

- GIVEN a lane awaiting approval
- WHEN a decision is recorded
- THEN the ledger MUST record in `approvals` without changing standard `lane.Status` values.

#### Scenario: Mark defect surfaced later

- GIVEN an approved lane that later surfaces a defect
- WHEN an operator flags the defect
- THEN the ledger MUST update `defect_surfaced_later` without changing historic lane status.
