# Delta for sdd-apply

## MODIFIED Requirements

### Requirement: Orchestrator Advances Only on a Passing Wave

The orchestrator MUST advance to wave N+1 only when wave N's `lucind-ai run` exits 0 with all lanes completed and integrated without path overlap conflicts, binding target parent state deterministically per wave. On a non-zero exit, lane failure, or reversion the orchestrator MUST halt remaining waves for human review or replanning, not skip ahead.
(Previously: The orchestrator MUST advance to wave N+1 only when wave N's `lucind-ai run` exits 0 — meaning every lane is `done` and none were reverted.)

#### Scenario: Passed wave advances
- GIVEN wave N's stdout reports `passed=true` and the process exits 0
- WHEN the orchestrator considers wave N+1
- THEN it MUST dispatch the next printed `lucind-ai run` command using the updated parent state

#### Scenario: Reverted or blocked wave stops the DAG
- GIVEN wave N exits non-zero because a lane is `blocked`, `deviated`, `failed`, listed in `reverted_ids`, or unordered overlapping paths were rejected
- WHEN the orchestrator considers further waves
- THEN it MUST NOT dispatch any of them
