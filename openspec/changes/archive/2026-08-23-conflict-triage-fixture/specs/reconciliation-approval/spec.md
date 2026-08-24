# Delta for Reconciliation Approval

## ADDED Requirements

### Requirement: Two-step close and retry CAS

Clearing a ClassRequired promotion block MUST require direction approval via `reconcile approve`, then candidate SHA registration via `reconcile resolve` from the primary repository root. Retry MUST adopt the registered candidate SHA only when the other feature's tip is unchanged, and MUST promote via compare-and-swap. If the other tip has moved, retry MUST NOT adopt the stale SHA and MUST block.

#### Scenario: Approved candidate promotes on retry

- GIVEN an approved request with a registered candidate SHA and an unchanged target feature tip
- WHEN the blocked feature is retried
- THEN the gate adopts the candidate SHA and the target parent advances by compare-and-swap

#### Scenario: Target tip drift rejects a stale SHA

- GIVEN an approved request whose target feature tip has moved since the candidate SHA was registered
- WHEN retry overlap evaluation runs
- THEN the gate MUST NOT adopt the stale SHA and MUST block the attempt
