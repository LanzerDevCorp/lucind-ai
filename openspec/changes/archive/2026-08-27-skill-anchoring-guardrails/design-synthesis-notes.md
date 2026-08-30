# Synthesis Notes: Skill Anchoring & Worktree Cleanup Guardrails

## Unresolved Contradictions

None

## Coverage Gaps

None

## Dropped Citations

None — all 58 citations across the union of the three lens manifests were opened, verified against the codebase in this worktree, and confirmed to accurately support their respective claims. Two test ranges were tightened to match their full function boundaries (`cmd/lucind-ai/cli_test.go:2974-3024` for `TestWorktreeCleanupCLI` and `cmd/lucind-ai/cli_test.go:4503-4545` for `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly`).

## Architecture Divergence

None — all three converged independently on Candidate 1 from the accepted proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:40-46`):

- **Worktree Cleanup Dirty Guardrail**: `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) accept an explicit `force bool` parameter. When `force` is false, `Remove` evaluates `PorcelainEmpty` (`:319-325`) and returns `ErrWorktreeDirty` (`:26-45`) without deleting uncommitted files. Nonexistent worktrees return `nil` idempotently. `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:58,1934-1978`) parses `--force`/`-f` and prints porcelain status, diff inspection commands, and references `troubleshooting.md:7-18` on refusal.
- **Automated Internal Teardowns**: Internal automated deletion sites pass `force: true` to `worktree.Remove` (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`).
- **Lifecycle Guidance Banners**: Embedded static guidance banners at four milestone call sites (`printReport` at `cmd/lucind-ai/cli.go:698-726`, `printIntegrateReport` at `:730-740`, `renderAcceptanceReceipt` at `:685-690`, `runSplit` at `:485-516` / `internal/dag/split.go:34-43`). Multi-wave `split` warnings route to `stderr`, while report banners append to `stdout`.
- **TDD WIP-Rescue Protocol**: Standardized operator procedure in `troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21` to inspect preserved worktrees (`internal/worktree/worktree.go:150-159`), commit partial WIP, and re-dispatch.

Lens A supplied authoritative decisions 1–4; Lens B supplied flow invariants, surface deltas, and stream separation rules; Lens C supplied test strategy, test seams, 5-boundary threat matrix, and single-revert rollback.
