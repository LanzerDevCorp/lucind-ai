# Delta for skill-derivation

## ADDED Requirements

### Requirement: Deterministic multi-tier derivation

The system MUST deterministically derive required skills from `(sdd_phase, lane_role)`, unioning derived, stack (`lucind.yaml`), and ad-hoc tiers without dropping derived skills, and MUST reject any candidate set exceeding budget (default 3) at admission before worktree or quota allocation.

#### Scenario: Planning lens derivation

- GIVEN `sdd_phase: propose` and `lane_role: lens` without stack or ad-hoc additions
- WHEN derivation runs at batch admission
- THEN required skills MUST equal `["lucind-executor", "lucind-fan-out-lens", "sdd-propose"]`.

#### Scenario: Stack deduplication within budget

- GIVEN `lucind.yaml` stack listing `lucind-executor`, already in the derived set
- WHEN derivation unions derived, stack, and ad-hoc tiers
- THEN `lucind-executor` MUST deduplicate, counting as 1 toward budget default 3.

#### Scenario: Over-budget skill set rejected

- GIVEN combined derived, stack, and ad-hoc skills exceeding budget 3
- WHEN batch admission runs
- THEN admission MUST fail closed before allocating any worktree.
