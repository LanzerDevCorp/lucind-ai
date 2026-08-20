# Lane Execution Specification

## Purpose

Hook the approval-wait gate into the existing lane lifecycle without disturbing the shared batch
barrier or the six-value `lane.Status` enum.

## Requirements

### Requirement: Gate Placement in the Lifecycle

Approval wait MUST run after status computation and MUST resolve before that status is persisted
to the ledger.

#### Scenario: Approve then persist done

- GIVEN a lane's computed status is `done` and the user approves
- WHEN the wait resolves
- THEN the terminal status MUST be persisted as `done` only after that decision.

#### Scenario: Timeout persists blocked, never done

- GIVEN a lane's computed status is `done` and the wait times out
- WHEN the lane persists
- THEN the terminal status MUST be `blocked`, never `done`.

### Requirement: Resolve Before Barrier Observation

Approval wait MUST resolve, and the lane's status MUST be persisted, before the batch barrier
observes that lane.

#### Scenario: Barrier waits for terminal persist

- GIVEN a lane still waiting on an approval decision
- WHEN the batch would otherwise observe that lane
- THEN the barrier MUST NOT treat it as observed until persistence completes.

#### Scenario: Barrier stays idle while one lane waits

- GIVEN one lane waiting on approval and every other lane already terminal
- WHEN the batch barrier is checked
- THEN it MUST NOT release until the waiting lane's status is persisted.

### Requirement: Additive Schema, Unchanged Enum

Approval records MUST be stored in a separate, additive table. The six-value `lane.Status` enum
MUST NOT gain a seventh value for this feature.

#### Scenario: Persist approval record

- GIVEN a lane awaiting approval
- WHEN a decision is recorded
- THEN the ledger MUST write it to the `approvals` table without changing `lane.Status`'s valid
  values.

#### Scenario: Mark a defect surfaced later

- GIVEN an approved lane whose packet later surfaces a defect
- WHEN an operator flags it
- THEN the ledger MUST update that approval's `defect_surfaced_later` column without altering the
  lane's historic status.
