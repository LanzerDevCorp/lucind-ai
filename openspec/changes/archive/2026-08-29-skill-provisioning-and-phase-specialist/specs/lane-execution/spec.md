# Delta for lane-execution

## MODIFIED Requirements

### Requirement: Frozen Authored Candidate Evidence

Before executor work can become a lane candidate, lane execution MUST freeze the exact admitted packet identity and digest, contract version or explicit legacy mode, normalized versioned contract evidence including `required_skills` and `lane_role` when present, typed target binding, execution mode, write paths, read-only paths, and result obligations, and MUST enforce that envelope skill shortfalls demote terminal status to `lane.Deviated`. Later packet, target, or checkout changes MUST NOT alter this evidence.
(Previously: Frozen candidate evidence recorded versioned contract fields and target bindings without required skills or skill shortfall demotion.)

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

#### Scenario: Skill shortfall demoted

- GIVEN required skills including a name omitted from the envelope's `skills_loaded`
- WHEN lane status is decided after execution
- THEN the lane MUST be demoted to `lane.Deviated`.
