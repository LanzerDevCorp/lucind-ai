# Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

None

## Coverage Gaps

None. All nine proposal spine items (Executive summary and problem statement, selected candidate and approach, changes to system concepts and architecture rationale, user and capability impact table, delta specifications, technical risks and failure modes, rollback plan and additivity, test and validation impact, out of scope and open questions) are substantively covered in `proposal.md`.

## Dropped Citations

None. All 42 citations across Lens A, Lens B, and Lens C were opened and verified against the worktree. Every citation resolves to existing code and accurately supports its stated claim.

## Scope Divergence

None — all three converged.

Lens A's selected Candidate 1 (Fail-Closed Worktree Cleanup Guardrails with Direct CLI Banner Anchoring and Prescriptive TDD WIP-Rescue Protocol) is authoritative. Lens B and Lens C independently converged on Candidate 1 and assumed its exact architecture, error types, flag surfaces, and milestone guidance banners.

Independent convergence across all three drafts:
1. **Fail-Closed Worktree Cleanup**: `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) accept an explicit `force bool` parameter and check `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) before deletion, returning `worktree.ErrWorktreeDirty` without removing files if uncommitted tracked or untracked changes exist.
2. **CLI Flag Surface**: `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) adds `--force` (`-f`) flag support; cleanup on dirty worktrees without `--force` exits with code 1, prints porcelain diff status, and preserves the worktree.
3. **Internal Automated Teardown Callers**: Automated internal callers pass `force: true` (`DiscardCombined` at `cmd/lucind-ai/cli.go:858-863`, `RemoveLaneWorktree` at `cmd/lucind-ai/cli.go:864-869` / `internal/run/integrate.go:159-165`, `Combine` merge conflict abort at `internal/integrate/integrate.go:118-124`, and `ResolveCandidate` promotion cleanup at `internal/integrate/candidate.go:262-265`).
4. **Milestone Guidance Banners**: Static CLI guidance banners embedded across four critical milestones:
   - `printReport` (`cmd/lucind-ai/cli.go:698-726`) on non-done lane status links to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.
   - `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) on non-empty `reverted_ids` directs operators to `lucind-ai integrate retry --run <run-id>` referencing `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`.
   - `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) reminds operators of qualitative checklist review steps 2–10 from `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`.
   - `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) warns operators to advance checkout and refresh `base_sha` and `expected_parent_sha` between waves per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`.
5. **Prescriptive TDD WIP-Rescue Protocol**: Standardized operator protocol documented in `troubleshooting.md` and `.agents/skills/lucind-apply/SKILL.md:10-21` to inspect preserved dirty worktrees, commit partial WIP, and re-dispatch without losing work.
6. **Pure Additivity & Rollback**: Purely additive with zero SQLite database migrations, zero feature lease modifications, and a single `git revert` rollback plan.
