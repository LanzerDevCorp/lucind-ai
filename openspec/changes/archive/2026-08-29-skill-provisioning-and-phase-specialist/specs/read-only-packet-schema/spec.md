# Delta for read-only-packet-schema

## MODIFIED Requirements

### Requirement: Extended packet frontmatter parsing

Packet parsing MUST accept optional `sdd_phase`, `fanout_group`, `skill`, and `lane_role` frontmatter keys, mapping present values onto the corresponding packet fields, and MUST default omitted keys to empty values. When `lane_role` is present, packet parsing MUST validate it against the closed set `{lens, synthesis, apply, verify, archive, ultrafixer, human}` and closed-validate `sdd_phase`; packets omitting `lane_role` MUST retain open `sdd_phase` parsing without failure. Live executor runtime skill telemetry MUST NOT be decoded from packet frontmatter.
(Previously: Extended packet frontmatter keys had open schema validation with exact key names left unresolved.)

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

#### Scenario: Valid lane_role and phase parsed

- GIVEN frontmatter declaring `lane_role: lens` and `sdd_phase: propose`
- WHEN the packet is parsed
- THEN the packet MUST carry `lane_role` `lens` and `sdd_phase` `propose`.

#### Scenario: Omitted lane_role preserves backward compatibility

- GIVEN frontmatter omitting `lane_role`
- WHEN the packet is parsed
- THEN parsing MUST succeed with empty `lane_role` and unvalidated `sdd_phase`.

#### Scenario: Unrecognized lane_role rejected

- GIVEN frontmatter declaring an unrecognized `lane_role`
- WHEN the packet is parsed
- THEN parsing MUST return a validation error.
