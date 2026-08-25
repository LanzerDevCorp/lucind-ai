# Delta for Stability Campaign State Machine

## ADDED Requirements

### Requirement: Sequential three-Trial progression and reset-on-failure

A stability campaign MUST execute three sequential Trials without automatic retries; any dispatch failure, crash, or budget exhaustion MUST immediately fail the campaign and reset consecutive pass count to zero.

#### Scenario: Three passing Trials

- GIVEN an active campaign at Trial 1
- WHEN Trials 1, 2, and 3 pass sequentially
- THEN the consecutive count MUST reach 3 and trigger terminal verification

#### Scenario: Slot failure reset

- GIVEN an active campaign in Trial 2
- WHEN any lane or slot fails
- THEN consecutive counter MUST reset to 0 and the campaign MUST fail

### Requirement: Execution timeout budgets

Stability execution MUST enforce budgets of 10 minutes per dispatch, 45 minutes per Trial, and 135 minutes per Campaign; exceeding any budget MUST terminate the active dispatch and fail the campaign.

#### Scenario: Dispatch timeout terminates campaign

- GIVEN an active dispatch exceeding 10 minutes
- WHEN the deadline expires
- THEN execution MUST terminate the child process group and fail the campaign

#### Scenario: Trial timeout enforcement

- GIVEN a Trial exceeding 45 cumulative minutes
- WHEN the budget timer fires
- THEN active dispatches MUST halt and the campaign MUST fail
