# Read-Only Packet Schema Specification

## Purpose

Specify the optional `read_only` packet frontmatter key: its parsing into `Packet.ReadOnly`, its default, and why it cannot be inferred or self-declared after the fact.

## Requirements

### Requirement: Frontmatter Read-Only Field Parsing

The packet parser MUST accept the optional `read_only` YAML frontmatter key as a boolean, MUST store it on `Packet.ReadOnly`, and MUST reject a non-boolean value with a parse error. The key MUST NOT become a completeness gate. (Design Decision 1.)

#### Scenario: Explicit read_only true
- GIVEN a packet document with frontmatter containing `read_only: true`
- WHEN `packet.Parse` parses the document
- THEN `Packet.ReadOnly` MUST be `true` and parsing MUST succeed

#### Scenario: Explicit read_only false
- GIVEN a packet document with frontmatter containing `read_only: false`
- WHEN `packet.Parse` parses the document
- THEN `Packet.ReadOnly` MUST be `false` and parsing MUST succeed

#### Scenario: Non-boolean value rejected
- GIVEN a packet document with a non-boolean `read_only` value (a string or number)
- WHEN `packet.Parse` parses the document
- THEN `packet.Parse` MUST return an error and reject the packet

### Requirement: Default Value and Backward Compatibility

When `read_only` is absent, `Packet.ReadOnly` MUST default to `false`. Every existing required-key validation (`id`, `executor`, `routed_by`, non-empty body) MUST be preserved unchanged, and no new required key is introduced. (Design Decision 1.)

#### Scenario: Omitted read_only defaults to write packet
- GIVEN a packet document valid under the schema that predates this change, with no `read_only` key
- WHEN `packet.Parse` parses the document
- THEN parsing MUST succeed with `Packet.ReadOnly` set to `false`

#### Scenario: Existing validation gates preserved
- GIVEN a packet document missing `id`, `executor`, or `routed_by`, or having an empty body
- WHEN `packet.Parse` parses the document
- THEN parsing MUST fail with the corresponding existing error (`ErrMissingID`, `ErrMissingExecutor`, `ErrMissingRoutedBy`, or `ErrEmptyBody`), regardless of `read_only`

#### Scenario: Unknown frontmatter keys still ignored
- GIVEN a packet document with unrecognized frontmatter keys alongside valid fields
- WHEN `packet.Parse` parses the document
- THEN parsing MUST succeed and ignore the unrecognized keys

### Requirement: Explicit Flag Only — No Inference

The system MUST NOT infer read-only vs. write mode from a packet's `id`, lane name, or any path-list content. `read_only` MUST stay orthogonal to any field the sibling `apply-dag-dispatch` change introduces. (Design Decision 1.)

#### Scenario: Explore-prefixed packet is still write by default
- GIVEN a packet whose `id` starts with `explore-` and whose frontmatter omits `read_only`
- WHEN the packet is parsed
- THEN `Packet.ReadOnly` MUST be `false`

#### Scenario: An empty or absent path list is not a read-only signal
- GIVEN a packet that omits `read_only` and declares no allowed paths in its body
- WHEN completion mode is determined
- THEN the packet MUST be treated as write

### Requirement: The Envelope Cannot Declare or Override Mode

The packet declares read-only mode up front, in frontmatter, before dispatch. The result envelope schema (`.lucind/result.json` / `result.schema.json`) MUST NOT gain a `read_only` property, and an agent MUST NOT be able to self-declare or override the mode after the fact. `commit` MUST remain optional (`omitempty`) and MUST NOT become required. (Design Decision 2.)

#### Scenario: Envelope without commit stays valid
- GIVEN a result envelope that omits `commit` and carries every required field
- WHEN the envelope is validated against `result.schema.json`
- THEN validation MUST succeed

#### Scenario: Envelope cannot inject a read_only property
- GIVEN a result envelope JSON document that includes a `read_only` property
- WHEN the envelope is validated with `additionalProperties: false`
- THEN validation MUST fail

### Requirement: Additive Rollback

The schema change MUST stay purely additive: no ledger or envelope schema version bump, no feature flag, no SQLite migration. After a revert of the apply commits, `read_only` in a packet document MUST again be silently ignored as an unknown frontmatter key, and that packet MUST be treated as write. (Design Decision 4.)

#### Scenario: Revert restores the unknown-key drop
- GIVEN the apply commits that introduced `Packet.ReadOnly` have been reverted
- WHEN a packet document still contains `read_only: true`
- THEN parsing MUST ignore the key and the packet MUST be treated as write
