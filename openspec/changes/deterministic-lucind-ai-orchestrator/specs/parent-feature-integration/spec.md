# Delta for parent-feature-integration

## MODIFIED Requirements

### Requirement: Recoverable Idempotent Attempts

Each integration attempt SHALL maintain an immutable idempotency key and recorded inputs. A retry MUST return the recorded terminal result without re-dispatching lanes, or resume from those inputs without a second promotion. After interruption or lease expiry, recovery MUST verify recorded expected and current refs before resuming; unsafe recovery SHALL fail closed while preserving evidence and worktrees. CAS promotion failure due to stale parent SHAs MUST preserve all worktrees and ledger evidence.
(Previously: Each attempt SHALL have a durable identity and recorded inputs, returning terminal results on retry or resuming without second promotion.)

#### Scenario: Completed attempt is retried
- GIVEN an attempt already reached a terminal result
- WHEN its identity is replayed
- THEN the same result SHALL be returned without another ref update and without re-dispatching lanes

#### Scenario: Expired lease is recovered
- GIVEN an interrupted attempt whose lease expired
- WHEN recovery verifies unchanged expected and current refs
- THEN it MAY reacquire the lease and resume from recorded inputs

#### Scenario: Recovery finds changed state
- GIVEN an interrupted attempt whose recorded refs no longer match
- WHEN recovery runs
- THEN it MUST remain blocked and preserve diagnostic artifacts, worktrees, and ledger evidence
