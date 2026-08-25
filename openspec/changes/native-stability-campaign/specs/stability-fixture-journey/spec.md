# Delta for Stability Fixture Journey

## ADDED Requirements

### Requirement: Concurrent multi-change fixture execution

Each Trial MUST create distinct ephemeral integration targets and dispatch Changes A and B concurrently, ensuring both orchestrators hold active ownership leases and dispatch lanes before promotion begins.

#### Scenario: Concurrent lane execution

- GIVEN ephemeral targets for Changes A and B
- WHEN lane packets are dispatched concurrently
- THEN both Orchestrators MUST hold active leases before initiating promotion

#### Scenario: Pinned model enforcement

- GIVEN dispatch requests for Changes A and B
- WHEN requests pass to executor
- THEN every dispatch MUST enforce pinned model `gemini-3.7-flash-high`

### Requirement: Accelerated lease expiry and crash recovery

Abrupt termination of Change B after result persistence MUST release ownership only after a 10-second lease TTL; reclaims before expiry MUST return `ErrLeaseHeld`, and post-expiry reclaim MUST verify zero `/proc` survivors, adopt the persisted envelope, and promote without redispatch.

#### Scenario: Expired lease reclaim and envelope adoption

- GIVEN Orchestrator B terminated abruptly after persisting result envelope
- WHEN replacement Orchestrator B reclaims after 10-second lease expiry
- THEN replacement MUST verify zero surviving processes, adopt the persisted envelope, and promote without redispatch

#### Scenario: Early reclaim rejected

- GIVEN Orchestrator B killed with active unexpired 10-second lease
- WHEN replacement attempts acquisition before expiry
- THEN store MUST return `ErrLeaseHeld` and increment fence counter

### Requirement: Deterministic fixture tree hash and ancestry isolation

Target promotions MUST verify Git commit ancestry and fixture digests: Change A target MUST contain only Fix and Change A commits, Change B target MUST contain only Change B commits, and final tree hashes MUST match deterministic fixtures.

#### Scenario: Commit ancestry isolation

- GIVEN completed integration targets A and B
- WHEN git ancestry is verified against base commits
- THEN Target A MUST contain only Fix and Change A commits, while Target B MUST contain only Change B commits

#### Scenario: Contaminated target rejection

- GIVEN Target B containing commits originating from Change A or Fix Change
- WHEN cross-target isolation check runs
- THEN verification MUST fail immediately and invalidate the trial
