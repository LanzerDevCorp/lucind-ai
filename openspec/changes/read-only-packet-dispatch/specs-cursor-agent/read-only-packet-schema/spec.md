# Read-Only Packet Schema Specification

## Purpose

Optional `read_only` packet frontmatter maps to `Packet.ReadOnly`, defaulting write, without a new required key.

## Requirements

### Requirement: Optional Boolean Frontmatter Key

The packet parser MUST accept YAML frontmatter `read_only` as a boolean and MUST store it on `Packet.ReadOnly`. The key MUST NOT become a completeness gate. (Design Decision 1.)

#### Scenario: True round-trip
- GIVEN frontmatter `read_only: true` and the existing required keys
- WHEN the packet is parsed
- THEN `Packet.ReadOnly` MUST be `true` and parse MUST succeed

#### Scenario: False round-trip
- GIVEN frontmatter `read_only: false` and the existing required keys
- WHEN the packet is parsed
- THEN `Packet.ReadOnly` MUST be `false` and parse MUST succeed

#### Scenario: Non-boolean rejected
- GIVEN frontmatter `read_only` set to a non-boolean value
- WHEN the packet is parsed
- THEN parse MUST fail

### Requirement: Default Write When Omitted

When `read_only` is absent, `Packet.ReadOnly` MUST be `false`. Existing packets that omit the key MUST remain write packets. Completeness errors MUST stay `ErrMissingID`, `ErrMissingExecutor`, `ErrMissingRoutedBy`, and `ErrEmptyBody`. (Design Decision 1.)

#### Scenario: Omitted key is write
- GIVEN a packet with `id`, `executor`, `routed_by`, and body, and no `read_only` key
- WHEN the packet is parsed
- THEN `Packet.ReadOnly` MUST be `false`

#### Scenario: No new required key
- GIVEN a packet that parses today without `read_only`
- WHEN the packet is parsed after this change
- THEN parse MUST still succeed

### Requirement: Explicit Flag Only

The system MUST NOT infer read-only vs write from packet ID, lane name, or any path list. `read_only` MUST stay orthogonal to `apply-dag-dispatch`. (Design Decision 1.)

#### Scenario: Explore-prefixed write packet
- GIVEN a packet whose `id` starts with `explore-` and whose frontmatter omits `read_only`
- WHEN the packet is parsed
- THEN `Packet.ReadOnly` MUST be `false`

#### Scenario: Path list is not a mode
- GIVEN a packet that omits `read_only` and whose body lists no allowed paths
- WHEN completion mode is applied
- THEN the packet MUST be treated as write

### Requirement: Envelope Does Not Declare Mode

The result envelope schema MUST NOT add a `read_only` property. The `commit` field MUST remain optional and MUST NOT be required. The packet declares mode up front; the agent MUST NOT self-declare it after the fact. (Design Decision 2.)

#### Scenario: Envelope without commit is valid
- GIVEN a result envelope that omits `commit` and has the required fields
- WHEN the envelope is validated
- THEN validation MUST succeed

#### Scenario: Envelope cannot set mode
- GIVEN a result envelope that includes a `read_only` property
- WHEN the envelope is validated with `additionalProperties: false`
- THEN validation MUST fail

### Requirement: Additive Rollback

The schema change MUST remain additive: no ledger or envelope schema version bump, no feature flag, and no SQLite migration. After revert of the apply commits, `read_only` in a packet document MUST be ignored again as an unknown frontmatter key, and that packet MUST be treated as write. (Design Decision 4.)

#### Scenario: Revert restores unknown-key drop
- GIVEN the apply commits that added `Packet.ReadOnly` have been reverted
- WHEN a packet document still contains `read_only: true`
- THEN parse MUST ignore the key and treat the packet as write
