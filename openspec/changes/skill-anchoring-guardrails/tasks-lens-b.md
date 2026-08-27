# Tasks Lens B — Partition & Dispatch Shape: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed decomposition

The change decomposes into five functional units derived from the design file-changes table (`openspec/changes/skill-anchoring-guardrails/design.md:100-107`):

1. **Worktree Dirty Guardrail**: Add trailing `force bool` to `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`), export `ErrWorktreeDirty` (`:26-45`), and check `PorcelainEmpty` (`:319-325`) on unforced deletion; update unit tests (`internal/worktree/worktree_test.go:255-266,536-595,1034-1069`).
2. **Internal Automated Callers**: Pass `force: true` to `worktree.Remove` at automated teardown call sites in `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`, and `cmd/lucind-ai/cli.go:858-869`.
3. **CLI Worktree Cleanup Surface**: Parse `--force`/`-f` in `runWorktreeCleanup` (`cmd/lucind-ai/cli.go:58,1934-1978`), fail closed on `ErrWorktreeDirty` with diagnostic output referencing `troubleshooting.md:7-18`, and verify via `cmd/lucind-ai/cli_test.go:2974-3024`.
4. **Failure Guidance Banners**: Embed static banners in `runSplit` stderr (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`), `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`), `printReport` (`:698-726`), and `printIntegrateReport` (`:730-740`); verify via `cmd/lucind-ai/cli_test.go:685-777,4503-4545` and `internal/dag/split_test.go:13-111`.
5. **Skill Anchoring & WIP-Rescue Protocol**: Document TDD WIP-rescue in `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21`; update references in `recovery-reconciliation.md:27-35` and `acceptance-promotion.md:18-30`.

Because `worktree.Cleanup`/`Remove` signature changes create an immediate compilation barrier across `internal/integrate/`, `internal/run/`, and `cmd/lucind-ai/`, and `cmd/lucind-ai/cli.go` is touched across three distinct concerns (flags, internal closures, banners), units 1–4 are co-dependent and form a single dispatchable unit.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| 1 | Implement fail-closed `worktree.Cleanup`/`Remove` guardrails, update internal automated teardowns, add CLI `--force` flag, embed 4 guidance banners, and document TDD WIP-rescue in skill references | `cmd/lucind-ai/cli.go`<br>`cmd/lucind-ai/cli_test.go`<br>`internal/dag/split_test.go`<br>`internal/integrate/candidate.go`<br>`internal/integrate/integrate.go`<br>`internal/run/integrate.go`<br>`internal/worktree/worktree.go`<br>`internal/worktree/worktree_test.go`<br>`plugin/claude-code/skills/lucind-ai/references/`<br>`.agents/skills/lucind-apply/SKILL.md` | `agy` | Single `git revert` of merge commit restoring unconditional worktree removal and removing CLI banners/docs |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| 1 | Unit 1 | No (single lane) | Yes: Contains the complete `worktree` signature update, all internal caller adjustments, CLI flags, guidance banners, and test coverage; compiles cleanly with `CGO_ENABLED=0 go build ./...` and passes full check suite (`go test ./... -race -count=1`) via `lucind-checks.sh:1-4`. |

## Disjointness Check

Evaluation of candidate sub-unit partitioning under component-boundary prefix rules (`internal/packet/disjoint.go:8-22,24-47`):

1. **CLI Surface vs. Internal Callers vs. Banners**:
   - `cmd/lucind-ai/cli.go` contains `runWorktreeCleanup` (`:1934-1978`), `productionDeps` closures (`:858-869`), and banner renderers (`:485-516,685-740`).
   - Attempting to partition CLI flags, internal callers, or banners into separate parallel packets results in overlapping `allowed_paths` on `cmd/lucind-ai/cli.go`. Under `internal/packet/disjoint.go:13-22`, `PathInScope` matches identically, and `DisjointAllowedPaths` (`:29-47`) fails closed. Verdict: **COLLISION (NOT DISJOINT)**.
2. **Worktree Signature vs. Callers**:
   - Splitting `internal/worktree/worktree.go` from `cmd/lucind-ai/cli.go` or `internal/integrate/integrate.go` across sequential waves causes Wave 1 to fail compilation at `lucind-checks.sh:1-4`, triggering bisection revert (`internal/run/integrate.go:50-59,83-98`).
3. **Consolidated Wave 1**:
   - Wave 1 contains a single consolidated unit (Unit 1). Intra-wave unit pairs count is zero. Path collision risk is eliminated. Verdict: **DISJOINT (PASS)**.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**:
- **Review Budget**: Total change is ~250–450 lines, well within the single-PR 2000-line budget (`openspec/changes/skill-anchoring-guardrails/proposal.md:40-46`).
- **Same-File Concentration**: `cmd/lucind-ai/cli.go` is modified for flag parsing (`:58,1934-1978`), internal closures (`:858-869`), and banners (`:485-516,685-740`). Splitting touches into multiple packets violates `internal/packet/disjoint.go:24-47`.
- **Atomic Compilation**: Breaking signature changes on `worktree.Cleanup`/`Remove` (`internal/worktree/worktree.go:247-261`) require atomic caller updates in `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, and `internal/run/integrate.go:159-165` to satisfy `lucind-checks.sh:1-4`.
- **Precedent**: Archived precedent `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` declined an `apply-dag.yaml` sidecar for a 650–1200 line change on identical grounds. A single packet executed sequentially by `agy` in Isolated Mode is optimal.

## Open Questions

- [ ] Task skill contract supersession: `~/.claude/skills/sdd-tasks/SKILL.md` prescribes a monolithic `tasks.md` with checklist, review workload forecast, and Engram persistence, which is superseded by this 3-lens parallel task decomposition workflow returning `.lucind/result.json`.
- [ ] Dual-judge qualitative verification: Isolated Mode apply and primary verification use `agy`, while the second qualitative verification judge runs on `cursor-agent` per `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43`.

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Strict TDD lifecycle phases and pre-commit verification checklist |
| `cmd/lucind-ai/cli.go:58` | CLI usage string specifying `worktree cleanup --lane <id>` |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` command parsing flags and invoking `dag.Split` |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` rendering mechanical acceptance receipt output |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` formatting lane outcome and non-done completion warning |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` printing integrated and reverted lane outcome summary |
| `cmd/lucind-ai/cli.go:858-869` | `productionDeps` defining `DiscardCombined` and `RemoveLaneWorktree` closures |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` implementing `worktree cleanup` subcommand execution |
| `cmd/lucind-ai/cli_test.go:685-777` | Tests verifying `printReport` diagnosis suppression and `printIntegrateReport` ID formatting |
| `cmd/lucind-ai/cli_test.go:2974-3024` | `TestWorktreeCleanupCLI` testing stale worktree removal, idempotency, and missing flag handling |
| `cmd/lucind-ai/cli_test.go:4503-4545` | `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` testing mechanical acceptance output |
| `internal/dag/split.go:34-43` | `Split` iterating over waves and emitting `lucind-ai run` commands to stdout |
| `internal/dag/split_test.go:13-111` | `TestSplit_TwoWaveDAGSuccess` testing multi-wave DAG splitting and stdout output formatting |
| `internal/integrate/candidate.go:262-265` | `ResolveCandidate` teardown removing candidate worktree and branch after promotion |
| `internal/integrate/integrate.go:118-124` | `Combine` merge conflict abort handler removing temporary worktree and branch |
| `internal/packet/disjoint.go:8-22` | `PathInScope` implementing component-boundary prefix matching rule for POSIX paths |
| `internal/packet/disjoint.go:24-47` | `DisjointAllowedPaths` verifying pairwise path disjointness across packet definitions |
| `internal/run/integrate.go:50-59` | `Integrate` executing check suite against combined worktree and triggering bisection on check failure |
| `internal/run/integrate.go:83-98` | `handleRedBatch` isolating clean subset via bisection and reverting failing lanes |
| `internal/run/integrate.go:159-165` | `completeIntegration` calling `RemoveLaneWorktree` to clean up promoted worktrees |
| `internal/worktree/worktree.go:26-45` | Sentinel error declarations for worktree operations |
| `internal/worktree/worktree.go:247-261` | `Cleanup` and `Remove` functions removing linked git worktrees |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` inspecting worktree directory for uncommitted changes |
| `internal/worktree/worktree_test.go:255-266` | `TestRemove` asserting clean linked worktree removal |
| `internal/worktree/worktree_test.go:536-595` | `TestPorcelainEmpty` verifying clean, ignored, and untracked file status detection |
| `internal/worktree/worktree_test.go:1034-1069` | `TestCleanupRemovesExistingWorktree` and `TestCleanupOnLaneWithNoWorktreeIsNoOp` testing cleanup behavior |
| `lucind-checks.sh:1-4` | Repository integration check script running build and test suites |
| `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26-27` | Archived precedent declining `apply-dag.yaml` sidecar for change fitting review budget |
| `openspec/changes/skill-anchoring-guardrails/design.md:100-107` | Design file-changes table specifying modified files, actions, and terminal consumers |
| `openspec/changes/skill-anchoring-guardrails/proposal.md:40-46` | Proposal technical approach specifying fail-closed cleanup, internal teardowns, and guidance banners |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step acceptance protocol and checklist |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43` | Dual-judge qualitative acceptance review protocol for Tier A changes |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-35` | Multi-wave sequencing discipline and bisection recovery protocol |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Dispatch and integration troubleshooting diagnosis and recovery table |
