# Delta for packet-authoring-contract

## MODIFIED Requirements

### Requirement: Versioned Contract and Late Target Binding

An authored contract MUST declare its contract version, route intent, execution mode, write paths, read-only input paths, goal, ordered done criteria, ordered hard stops, result obligations, and optional `lane_role` and `required_skills` declarations. It MUST NOT contain live feature, parent, base, expected-parent, or commit values. The compiled packet body MUST include a Required skills section listing resolved filesystem paths for every required skill. Compilation MUST accept exactly one validated typed binding: feature target or legacy-main target. Authoring evidence version MUST remain `lane-authoring-evidence/v1`.
(Previously: Authored contracts declared version, paths, criteria, stops, and result obligations without lane_role or rendered required skills.)

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

#### Scenario: Required skills rendered in packet body

- GIVEN a compiled contract with a resolved required skill
- WHEN the packet body is rendered
- THEN the body MUST contain a Required skills section listing resolved filesystem paths.

#### Scenario: Legacy authoring evidence hash stability

- GIVEN frozen authoring evidence under `lane-authoring-evidence/v1` without `required_skills`
- WHEN that evidence is decoded
- THEN hash verification MUST succeed without schema migration.
