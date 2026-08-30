# Delta for packet-authoring-contract

## MODIFIED Requirements

### Requirement: Versioned Contract and Late Target Binding

An authored contract MUST declare its contract version, route intent, execution mode, write paths, read-only input paths, goal, ordered done criteria, ordered hard stops, and result obligations. It MUST NOT contain live feature, parent, base, expected-parent, or commit values. Compilation MUST accept exactly one validated typed binding: feature target or legacy-main target. A packet whose `allowed_paths` is omitted MUST bind with an empty declared scope and MUST NOT trigger diff-boundary or overlap-scope validation failures at admission or post-run for that packet.
(Previously: An authored contract MUST declare its contract version, route intent, execution mode, write paths, read-only input paths, goal, ordered done criteria, ordered hard stops, and result obligations. It MUST NOT contain live feature, parent, base, expected-parent, or commit values. Compilation MUST accept exactly one validated typed binding: feature target or legacy-main target.)

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

#### Scenario: Packet omitting allowed paths defaults to open scope safely

- GIVEN a packet template that omits `allowed_paths`
- WHEN the packet is parsed and admitted
- THEN `AllowedPaths` MUST remain empty and diff-boundary and overlap-scope checks MUST be skipped for that packet
