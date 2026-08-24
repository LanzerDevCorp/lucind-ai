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

### Requirement: Lane metadata dispatch persistence

After a lane is registered, dispatch MUST persist packet and routing metadata to the ledger so dashboard consumers can return populated fields. Historical rows from before this requirement MUST NOT be backfilled.

#### Scenario: Dispatch persists metadata

- GIVEN a packet with model, agent, feature, and SDD attributes dispatched through lane execution
- WHEN lane registration succeeds
- THEN the ledger MUST retain the metadata snapshot and listing lanes MUST return populated metadata fields rather than an unavailable placeholder

#### Scenario: Historical rows preserved

- GIVEN pre-existing lane records without an audited metadata snapshot
- WHEN listing lanes queries the ledger
- THEN the query MUST return the recorded schema-v6 columns with empty values for unrecorded extended fields without error

#### Scenario: Pre-dispatch failure persists metadata

- GIVEN a batch lane that fails before executor execution
- WHEN the failed lane is registered
- THEN dispatch MUST persist packet and routing metadata on that failed lane record

