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

### Requirement: Universal Pre-Dispatch Packet Admission

Dispatch MUST apply universal packet admission to manual and compiled packets before `ExecuteBatch`, worktree creation, or quota allocation. Admission MUST validate result delivery and schema obligations, route and mode consistency, path declarations, target completeness, and live target freshness. A rejected packet MUST produce field-specific diagnostics and MUST NOT partially dispatch its batch.

#### Scenario: Safe mixed batch
- GIVEN a batch of safe manual and compiled packets
- WHEN dispatch admission completes
- THEN the batch MAY proceed to execution without rewriting admitted manual bodies

#### Scenario: Unsafe packet blocks allocation
- GIVEN one packet lacks a result obligation or contradicts its declared mode
- WHEN the batch is admitted
- THEN the batch MUST fail before any worktree or quota allocation and identify the violation

#### Scenario: Target becomes stale
- GIVEN a compiled packet binding was valid when authored but its expected parent is stale at admission
- WHEN dispatch starts
- THEN the batch MUST fail before `ExecuteBatch` and report the stale target

### Requirement: Frozen Authored Candidate Evidence

Before executor work can become a lane candidate, lane execution MUST freeze the exact admitted packet identity and digest, contract version or explicit legacy mode, normalized versioned contract evidence when present, typed target binding, execution mode, write paths, read-only paths, and result obligations. Later packet, target, or checkout changes MUST NOT alter this evidence.

#### Scenario: Versioned candidate freezes correspondence evidence
- GIVEN a compiled packet passes admission
- WHEN its candidate evidence is recorded
- THEN Acceptance MUST be able to recover all declarations needed for independent correspondence checks

#### Scenario: Source packet changes later
- GIVEN frozen candidate evidence and a packet file edited after dispatch
- WHEN result or Acceptance verification runs
- THEN verification MUST use the frozen evidence rather than the edited file

#### Scenario: Legacy packet is explicit
- GIVEN an admitted unversioned manual packet
- WHEN candidate evidence is frozen
- THEN it MUST be marked legacy and MUST NOT be mistaken for a versioned contract

### Requirement: Frozen Authored Candidate Evidence and Required Skills Delivery

The system MUST record declared required skills inside frozen authoring evidence, MUST deliver required skills to the execution environment via `LUCIND_REQUIRED_SKILLS` and rendered body sections, and MUST demote any result envelope whose declared `skills_loaded` has a shortfall against authoring evidence to `lane.Deviated`.

#### Scenario: Envelope shortfall demoted to deviated

- GIVEN authoring evidence requiring `["lucind-executor", "lucind-fan-out-lens", "sdd-propose"]`
- WHEN an envelope declaring `skills_loaded: ["lucind-executor"]` is returned with status `done`
- THEN the system MUST demote lane status to `lane.Deviated` before candidate integration.

#### Scenario: Complete skills loaded preserved as done

- GIVEN authoring evidence requiring skills and an envelope declaring matching `skills_loaded` with status `done`
- WHEN completion status is evaluated
- THEN lane status MUST remain `lane.Done`.

