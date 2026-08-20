# Packet Declaration Specification

## Purpose

Specify packet frontmatter schema extensions for declaring read-only execution mode, boolean parsing rules, default value assignment, and backward compatibility with existing packet documents.

## Requirements

### Requirement: Frontmatter Read-Only Field Parsing (Decision 1)

The packet parser MUST parse the optional `read_only` YAML frontmatter key into `Packet.ReadOnly` as a boolean (`true` or `false`) and MUST reject non-boolean values with a parse error.

#### Scenario: Explicit read_only true
- GIVEN a packet document with frontmatter containing `read_only: true`
- WHEN `packet.Parse` parses the document
- THEN `Packet.ReadOnly` MUST be `true`.

#### Scenario: Explicit read_only false
- GIVEN a packet document with frontmatter containing `read_only: false`
- WHEN `packet.Parse` parses the document
- THEN `Packet.ReadOnly` MUST be `false`.

#### Scenario: Non-boolean value rejected
- GIVEN a packet document with frontmatter containing a non-boolean `read_only` value (such as a string or number)
- WHEN `packet.Parse` parses the document
- THEN `packet.Parse` MUST return an error and reject the packet.

### Requirement: Default Value and Backward Compatibility (Decision 1)

When the `read_only` frontmatter key is omitted or absent, the parser MUST default `Packet.ReadOnly` to `false`, and MUST preserve all existing required-key validations (`id`, `executor`, `routed_by`, and non-empty body) without introducing new required keys.

#### Scenario: Omitted read_only defaults to write packet
- GIVEN a packet document valid under the existing schema without a `read_only` frontmatter key
- WHEN `packet.Parse` parses the document
- THEN parsing MUST succeed with `Packet.ReadOnly` set to `false`.

#### Scenario: Existing validation gates preserved
- GIVEN a packet document missing `id`, `executor`, or `routed_by`, or having an empty body
- WHEN `packet.Parse` parses the document
- THEN parsing MUST fail with the corresponding missing-field error (`ErrMissingID`, `ErrMissingExecutor`, `ErrMissingRoutedBy`, or `ErrEmptyBody`), regardless of `read_only` presence.

#### Scenario: Unknown frontmatter keys ignored
- GIVEN a packet document containing unknown frontmatter keys alongside valid fields
- WHEN `packet.Parse` parses the document
- THEN parsing MUST succeed and ignore the unrecognized keys.
