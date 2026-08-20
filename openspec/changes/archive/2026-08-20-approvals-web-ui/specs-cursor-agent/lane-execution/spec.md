# Lane Execution Specification

## Purpose

Wait after `decideStatus`, before `SetStatus` and `b.Observe`.

## Requirements

### Requirement: Gate Placement

Execute MUST wait after `decideStatus` and before `deps.Ledger.SetStatus`.

#### Scenario: Approve then persist done
- GIVEN `decideStatus` is `done` and the user approves
- WHEN the wait resolves
- THEN `SetStatus` MUST record `done` only after that decision

#### Scenario: Timeout before SetStatus
- GIVEN `decideStatus` is `done` and the wait times out
- WHEN Execute persists
- THEN `SetStatus` MUST record `blocked`, never `done`

### Requirement: Before Observe

Wait MUST resolve before `b.Observe`.

#### Scenario: Observe after persist
- GIVEN a lane still waiting
- WHEN the batch would observe it
- THEN `Observe` MUST wait for terminal persist

#### Scenario: Barrier idle while waiting
- GIVEN one lane waiting and others terminal
- WHEN the batch barrier is checked
- THEN it MUST NOT treat the waiting lane as observed
