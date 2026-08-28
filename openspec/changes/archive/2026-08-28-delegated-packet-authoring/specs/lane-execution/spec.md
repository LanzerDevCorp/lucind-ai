# Delta for Lane Execution

## ADDED Requirements

### Requirement: Universal Pre-Dispatch Packet Admission

Dispatch MUST apply universal packet admission to manual and compiled packets before `ExecuteBatch`, worktree creation, or quota allocation. Admission MUST validate result delivery and schema obligations, route and mode consistency, path declarations, target completeness, and live target freshness. A rejected packet MUST produce field-specific diagnostics and MUST NOT partially dispatch its batch.

#### Scenario: Safe mixed batch
- GIVEN a batch of safe manual and compiled packets
- WHEN dispatch admission completes
- THEN the batch MAY proceed to execution without rewriting admitted manual bodies

#### Scenario: Unsafe packet blocks allocation
- GIVEN one packet lacks a result obligation or contradicts its declared mode
- WHEN the batch is admitted
- THEN the batch MUST fail before any worktree or quota allocation and identify the violation

#### Scenario: Target becomes stale
- GIVEN a compiled packet binding was valid when authored but its expected parent is stale at admission
- WHEN dispatch starts
- THEN the batch MUST fail before `ExecuteBatch` and report the stale target

### Requirement: Frozen Authored Candidate Evidence

Before executor work can become a lane candidate, lane execution MUST freeze the exact admitted packet identity and digest, contract version or explicit legacy mode, normalized versioned contract evidence when present, typed target binding, execution mode, write paths, read-only paths, and result obligations. Later packet, target, or checkout changes MUST NOT alter this evidence.

#### Scenario: Versioned candidate freezes correspondence evidence
- GIVEN a compiled packet passes admission
- WHEN its candidate evidence is recorded
- THEN Acceptance MUST be able to recover all declarations needed for independent correspondence checks

#### Scenario: Source packet changes later
- GIVEN frozen candidate evidence and a packet file edited after dispatch
- WHEN result or Acceptance verification runs
- THEN verification MUST use the frozen evidence rather than the edited file

#### Scenario: Legacy packet is explicit
- GIVEN an admitted unversioned manual packet
- WHEN candidate evidence is frozen
- THEN it MUST be marked legacy and MUST NOT be mistaken for a versioned contract
