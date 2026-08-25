# Delta for Stability Resume and Abort

## ADDED Requirements

### Requirement: Fail-closed resume reconciliation

The `stability resume` command MUST reconcile active processes, leases, worktrees, and refs before continuing; any ambiguous or non-deterministic state discrepancy MUST fail closed and prohibit resumption.

#### Scenario: Ambiguous state fails closed

- GIVEN an interrupted campaign with mismatched worktrees or orphaned processes
- WHEN `lucind-ai stability resume` executes
- THEN the command MUST detect discrepancy, fail closed, and prohibit resumption

#### Scenario: Clean resume execution

- GIVEN an interrupted campaign with unambiguous persisted state and clean worktrees
- WHEN `lucind-ai stability resume` executes
- THEN execution MUST resume active Trial at the exact interrupted stage

### Requirement: Idempotent abort and blocked cleanup

The `stability abort` command MUST idempotently terminate processes, release leases, and remove ephemeral worktrees; unremovable residue MUST transition the campaign to `blocked_cleanup` without redispatching tasks.

#### Scenario: Idempotent abort cleanup

- GIVEN an interrupted campaign in `blocked_cleanup` state
- WHEN operator executes `lucind-ai stability abort`
- THEN residual worktrees and branches MUST be purged without AI dispatches

#### Scenario: Abort handles unremovable residue

- GIVEN unremovable residue during abort
- WHEN `stability abort` fails to clear filesystem
- THEN campaign MUST transition to `blocked_cleanup` without redispatching
