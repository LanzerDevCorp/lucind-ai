# Delta for Read-Only Packet Schema

## ADDED Requirements

### Requirement: Extended packet frontmatter parsing

Packet parsing MUST accept optional SDD-phase, fanout-group, and skill frontmatter keys (exact key names remain an open design question) and MUST map present values onto the corresponding packet fields. Omitted keys MUST default to empty strings. Absence of these keys MUST NOT fail parsing. Live executor runtime skill telemetry MUST NOT be decoded from packet frontmatter.

#### Scenario: Parse frontmatter keys

- GIVEN a packet markdown document that declares SDD-phase, fanout-group, and skill values
- WHEN the packet is parsed
- THEN the returned packet MUST carry those declared values

#### Scenario: Optional keys omitted

- GIVEN a packet markdown document that omits SDD-phase, fanout-group, and skill keys
- WHEN the packet is parsed
- THEN parsing MUST succeed with empty values for those fields

#### Scenario: Empty frontmatter values handled

- GIVEN a packet document that includes those keys with empty values
- WHEN the packet is parsed
- THEN parsing MUST succeed and assign empty strings to the corresponding fields
