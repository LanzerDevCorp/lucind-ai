# Packet Authoring Contract Specification

## Purpose

Define a versioned, target-free authoring contract that produces safe, deterministic packets while retaining an explicit manual compatibility path.

## Requirements

### Requirement: Versioned Contract and Late Target Binding

An authored contract MUST declare its contract version, route intent, execution mode, write paths, read-only input paths, goal, ordered done criteria, ordered hard stops, and result obligations. It MUST NOT contain live feature, parent, base, expected-parent, or commit values. Compilation MUST accept exactly one validated typed binding: feature target or legacy-main target.

#### Scenario: Compile with a feature binding
- GIVEN a valid target-free contract and a valid feature-target binding
- WHEN compilation runs
- THEN the artifact MUST contain the bound target values and identify the contract version

#### Scenario: Reject authored target authority
- GIVEN specialist or versioned manual contract data containing a live target SHA
- WHEN contract validation runs
- THEN validation MUST fail with a diagnostic identifying the forbidden field

#### Scenario: Reject a stale binding
- GIVEN a binding whose expected parent no longer matches the live parent
- WHEN dispatch admission validates the binding
- THEN admission MUST fail before worktree or quota allocation

### Requirement: Deterministic Rendering and Digest

Compilation of identical normalized contract and binding values MUST produce byte-identical packet Markdown, normalized evidence, and digest across repeated runs. Ordering, path normalization, encoding, and newline differences MUST NOT vary by runtime iteration or host environment. Any semantically relevant contract or binding change MUST change the digest.

#### Scenario: Deterministic replay
- GIVEN identical contract and binding values
- WHEN compilation is repeated in separate runs
- THEN rendered bytes and digests MUST be identical

#### Scenario: Relevant input changes
- GIVEN two inputs differing in one criterion, stop, mode, path, or bound target
- WHEN both are compiled
- THEN their normalized evidence and digests MUST differ

### Requirement: Universal Admission and Manual Compatibility

Every compiled or manual packet MUST pass admission before dispatch. Admission MUST reject missing or contradictory result-path, result-schema, route, mode, target, or path obligations with actionable diagnostics. An admitted unversioned manual packet MUST remain in legacy compatibility mode, MUST retain its dispatch body bytes unchanged, and MUST NOT acquire strict versioned correspondence retroactively.

#### Scenario: Safe legacy manual packet
- GIVEN an unversioned manual packet satisfying universal safety checks
- WHEN it is admitted
- THEN its body MUST remain byte-identical and it MUST dispatch in compatibility mode

#### Scenario: Unsafe legacy manual packet
- GIVEN a manual packet omitting `.lucind/result.json` delivery or schema validation
- WHEN admission runs
- THEN it MUST fail before dispatch with the missing obligation identified

#### Scenario: Contradictory mode
- GIVEN packet metadata declares read-only while its body requires a commit
- WHEN admission runs
- THEN it MUST reject the packet with both contradictory declarations identified

### Requirement: Versioned Result Correspondence

For a versioned artifact, the system MUST freeze the normalized contract and require the result to correspond exactly to its packet identity, ordered criteria, ordered hard stops, mode, commit obligation, and canonical changed paths. Missing, extra, duplicate, or altered criteria or stops MUST fail correspondence. Write results MUST name the frozen candidate commit; read-only results MUST omit commit and report no canonical changes.

#### Scenario: Exact versioned result
- GIVEN a versioned write result matching all frozen declarations and candidate facts
- WHEN correspondence is checked
- THEN the result MUST be eligible for mechanical acceptance

#### Scenario: Omitted or extra declaration
- GIVEN a versioned result omits, duplicates, alters, or adds a criterion or hard stop
- WHEN correspondence is checked
- THEN correspondence MUST fail and no acceptance receipt may be created

### Requirement: Packet Contract Extension and Rendered Delivery

Compiled packet contracts MUST include derived and configured required skills in the contract representation, MUST render resolved skill filesystem paths under `## Required skills` in the markdown body between `## Hard stops` and `## Return`, and MUST preserve legacy authoring evidence hash stability under `lane-authoring-evidence/v1`.

#### Scenario: Stale skill binding rejected at admission

- GIVEN a compiled contract where a required skill cannot be resolved at dispatch time
- WHEN dispatch admission validates the binding
- THEN admission MUST fail before worktree or quota allocation

#### Scenario: Required skills rendered in packet body

- GIVEN a compiled contract with a resolved required skill
- WHEN the packet body is rendered
- THEN the body MUST contain a Required skills section listing resolved filesystem paths.

#### Scenario: Legacy authoring evidence hash stability

- GIVEN frozen authoring evidence under `lane-authoring-evidence/v1` without `required_skills`
- WHEN that evidence is decoded
- THEN hash verification MUST succeed without schema migration.

