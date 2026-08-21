# Delta for Parent Feature Integration

## ADDED Requirements

### Requirement: Explicit Feature Target

Every dispatchable unit SHALL identify its feature, parent ref, immutable base SHA, and expected parent SHA. Lucind MUST NOT infer any target from the primary checkout. Legacy packets and current single-`main` flows MUST fail closed unless an explicit legacy mode declares `main` and its expected SHA.

#### Scenario: Explicit target accepted
- GIVEN a unit with all four target fields
- WHEN it is admitted
- THEN its recorded parent ref and SHAs SHALL be used unchanged

#### Scenario: Missing or implicit target rejected
- GIVEN a unit omits target data or relies on checkout state
- WHEN it is admitted without explicit legacy mode
- THEN admission MUST fail before worktree creation or ref mutation

### Requirement: Managed Parent Lifecycle

Lucind SHALL create a feature parent from the declared base and MAY advance it while active. It MUST NOT rewrite active parent history. Closure and integration to `main` SHALL remain external; Lucind MUST NOT automatically promote to `main` or own review or delivery.

#### Scenario: Active parent advances
- GIVEN Lucind created an active parent at the declared base
- WHEN a valid integration is promoted
- THEN only that parent SHALL advance without history rewriting

#### Scenario: Feature closure remains external
- GIVEN an active parent is ready for delivery
- WHEN Lucind completes parent integration
- THEN `main`, review state, and delivery state MUST remain unchanged

### Requirement: Immutable Starts and Serialized Promotion

Lane and combine worktrees MUST start at the explicitly recorded immutable parent revision. Promotion SHALL hold a durable per-feature lease and MUST update the named parent only by expected-SHA compare-and-swap (CAS). Different parents MAY progress concurrently.

#### Scenario: Independent parents progress
- GIVEN attempts target different parent refs with valid leases and SHAs
- WHEN both promote concurrently
- THEN each CAS SHALL affect only its named parent

#### Scenario: Same-parent attempt becomes stale
- GIVEN another attempt advances the same parent first
- WHEN the stale attempt validates or performs CAS
- THEN it MUST fail without changing any ref and SHALL preserve evidence and worktrees

### Requirement: Recoverable Idempotent Attempts

Each attempt SHALL have a durable identity and recorded inputs. A retry MUST return its terminal result or resume from those inputs without a second promotion. After interruption or lease expiry, recovery MUST verify recorded expected and current refs before resuming; unsafe recovery SHALL fail closed while preserving evidence and worktrees.

#### Scenario: Completed attempt is retried
- GIVEN an attempt already reached a terminal result
- WHEN its identity is replayed
- THEN the same result SHALL be returned without another ref update

#### Scenario: Expired lease is recovered
- GIVEN an interrupted attempt whose lease expired
- WHEN recovery verifies unchanged expected and current refs
- THEN it MAY reacquire the lease and resume from recorded inputs

#### Scenario: Recovery finds changed state
- GIVEN an interrupted attempt whose recorded refs no longer match
- WHEN recovery runs
- THEN it MUST remain blocked and preserve diagnostic artifacts
