# Spec Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

None. All three lens drafts converged on the scope, requirement statements, and behavioral semantics. Lens A authored the authoritative requirement set across the five capability domains. Lens B contributed the full set of testable Given/When/Then scenarios and coverage mappings without inventing out-of-scope requirements. Lens C conducted a comprehensive audit of all 24 live specifications in `openspec/specs/` and confirmed that no live spec covers worktree cleanup guardrails, failure guidance banners, or the TDD WIP-rescue protocol.

## Coverage Gaps

None. All five capabilities (`worktree-dirty-guardrail`, `lane-worktree-lifecycle`, `worktree-cleanup-cli`, `failure-guidance-banners`, and `tdd-wip-rescue-protocol`) and all six requirements are fully specified with RFC 2119 keywords and concrete Given/When/Then scenarios. Open questions regarding standard error vs standard output routing and helper wrappers are implementation details resolved cleanly: CLI warning banners for DAG split are routed to stderr to protect pipeline-parseable wave commands, while reporting banners are formatted into their standard CLI display outputs.

## Dropped Citations

None. Across the three lens drafts, 81 citation manifest entries (Lens A: 25, Lens B: 30, Lens C: 26) representing 57 unique file and line-range citations were opened and verified against the real code, tests, documentation, and live specifications in this worktree. All cited line ranges are valid and substantiate their claims.

## Requirement Divergence

The two capabilities designated as "Modified Capabilities" in the proposal (`lane-worktree-lifecycle` and `worktree-cleanup-cli`) were reclassified to ADDED in the canonical delta spec tree. An independent verification of all 24 live specifications under `openspec/specs/` confirmed Lens C and Lens A's finding that neither `lane-worktree-lifecycle` nor `worktree-cleanup-cli` (nor any worktree cleanup guardrails in `lane-execution` or elsewhere) exists in the live specs repository. Consequently, no existing live specification blocks exist to modify; all five capability domains are specified as new ADDED specifications.
