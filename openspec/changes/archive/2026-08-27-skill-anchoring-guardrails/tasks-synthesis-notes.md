# Tasks Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

None.

All three lens drafts align on the architecture decisions from `design.md`, sentinel error placement (`ErrWorktreeDirty`), signature changes (`force bool`), fail-closed dirty checking with `PorcelainEmpty`, automated caller updates (`force: true`), `--force`/`-f` flag in `worktree cleanup`, the four milestone guidance banners, and the TDD WIP-rescue documentation in `troubleshooting.md` and `lucind-apply/SKILL.md`. There are no unresolved contradictions across the drafts.

## Coverage Gaps

None.

All requirements across the five spec deltas (`failure-guidance-banners`, `lane-worktree-lifecycle`, `tdd-wip-rescue-protocol`, `worktree-cleanup-cli`, `worktree-dirty-guardrail`) are traced directly to tasks. All applicable threat-matrix cases from `design.md` (`Git repository selection`, `Commit state`) have explicit RED-test tasks preceding their production tasks.

## Dropped Citations

None.

All `file:line` citations across `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` (and their respective citation manifests) were opened and confirmed directly against the code, tests, designs, specs, and skill references in this worktree:
- `.agents/skills/lucind-apply/SKILL.md:10-21` (TDD lifecycle and pre-commit checklist)
- `cmd/lucind-ai/cli.go:58, 485-516, 685-690, 698-726, 730-740, 858-869, 1934-1978, 1959-1969` (usage, split, accept receipt, report, integrate report, productionDeps closures, worktree cleanup)
- `cmd/lucind-ai/cli_test.go:685-777, 685-724, 729-777, 2974-3024, 4503-4545` (report diagnosis tests, integrate tests, worktree cleanup CLI tests, accept tests)
- `internal/dag/split.go:34-43` and `internal/dag/split_test.go:13-111` (Split stdout emission and multi-wave tests)
- `internal/integrate/integrate.go:118-124` and `internal/integrate/candidate.go:262-265` (Combine abort and ResolveCandidate teardown)
- `internal/run/integrate.go:50-59, 83-98, 159-165` (Integrate check/bisection/revert and completeIntegration lane teardown)
- `internal/worktree/worktree.go:26-45, 150-159, 247-261, 256-261, 271-292, 319-325` (sentinel errors, pathFor, Cleanup/Remove, IsLinkedWorktree, PorcelainEmpty)
- `internal/worktree/worktree_test.go:240-266, 244-266, 255-266, 536-595, 1034-1069` (TestRemove, TestPorcelainEmpty, TestCleanup)
- `internal/packet/disjoint.go:8-22, 24-47, 13-22, 29-47` (PathInScope and DisjointAllowedPaths)
- `lucind-checks.sh:1-4` (integration checks script)
- `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` (archived single-packet precedent)
- `openspec/changes/skill-anchoring-guardrails/design.md:5-14, 17-44, 78-84, 98-108, 100-107, 109-127, 110-137`
- `openspec/changes/skill-anchoring-guardrails/proposal.md:40-46`
- `openspec/changes/skill-anchoring-guardrails/specs/` (all 5 delta specs)
- `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30, 38-43`
- `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30, 27-35, 33-35`
- `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`
- `plugin/claude-code/skills/lucind-ai/SKILL.md:33-49`

All citations resolve cleanly and accurately support the claims in the text. Zero citations dropped.

## Decomposition Divergence

None — all three converged.

All three lens drafts converged on the same underlying functional decomposition:
- Lens A decomposed into 3 sequential phases: Phase 1 (Core Worktree Guardrail & Automated Callers), Phase 2 (CLI Worktree Cleanup Command), Phase 3 (CLI Failure Guidance Banners & Operator Documentation).
- Lens B identified 5 units that map directly to Lens A's 3 phases (Units 1 & 2 to Phase 1, Unit 3 to Phase 2, Unit 4 to Phase 3 banners, Unit 5 to Phase 3 documentation) and proved that consolidating them into a single packet with no `apply-dag.yaml` sidecar is required to prevent path collisions on `cmd/lucind-ai/cli.go` and bisection reverts during `Integrate`.
- Lens C identified 4 phases that map directly to Lens A (splitting Lens A Phase 3 into Phase 3 Banners and Phase 4 Documentation).

Canonical `tasks.md` adopts Lens A's authoritative 3-phase decomposition, incorporates Lens C's explicit RED-test threat-matrix mapping, and validates Lens B's single-packet dispatch recommendation.
