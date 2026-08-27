# Tasks: Skill Anchoring & Worktree Cleanup Guardrails

Single packet, sequential apply. No `apply-dag.yaml`: fits single-PR review budget (~250–450 lines) and `cmd/lucind-ai/cli.go` spans flags, closures, and banners. Strict-TDD RED/GREEN remains in one lane; atomic compilation requires signature updates and internal callers to land together.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~250–450 lines (impl ~110, tests ~220, doc ~120) |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Implement fail-closed `worktree.Cleanup`/`Remove` guardrails, update internal automated teardowns, add CLI `--force` flag, embed 4 guidance banners, and document TDD WIP-rescue in skill references | PR 1 | `go test ./internal/worktree ./internal/integrate ./internal/run ./internal/dag ./cmd/lucind-ai -count=1` | `lucind-ai worktree cleanup --lane <id>` on dirty/clean/missing trees; `lucind-ai run`, `integrate retry`, `accept`, and `split` banner outputs | Single `git revert` of merge commit restoring unconditional worktree removal and removing CLI banners/docs |

Apply order: Unit 1 executed by `agy` in Isolated Mode. Intra-wave unit pair count is zero; path collision risk is eliminated.

## Phase 1: Core Worktree Guardrail & Automated Callers

- [ ] 1.1 RED: In `internal/worktree/worktree_test.go:240-266,536-595,1034-1069` add unit tests for `force bool` parameter on `Remove` and `Cleanup`: assert unforced dirty worktree returns `ErrWorktreeDirty` preserving files (staged, unstaged, untracked), forced dirty removal succeeds, clean removal succeeds, invalid path fails cleanly, and nonexistent lane cleanup returns nil idempotently.
- [ ] 1.2 GREEN: In `internal/worktree/worktree.go:26-45` export `ErrWorktreeDirty = errors.New("worktree: linked worktree has uncommitted changes")`. In `internal/worktree/worktree.go:247-261` update `worktree.Cleanup` and `worktree.Remove` signatures to accept trailing `force bool`, calling `PorcelainEmpty` (`:319-325`) when `force == false` and returning `ErrWorktreeDirty`. Prove: `go test ./internal/worktree -run 'TestRemove|TestCleanup|TestPorcelainEmpty'`.
- [ ] 1.3 GREEN: Update internal automated removal call sites passing `force: true`: `internal/integrate/integrate.go:118-124` (merge conflict abort in `Combine`), `internal/integrate/candidate.go:262-265` (`ResolveCandidate` promotion teardown), `internal/run/integrate.go:159-165` (`completeIntegration` lane teardown), and `cmd/lucind-ai/cli.go:858-869` (`DiscardCombined` and `RemoveLaneWorktree` in `productionDeps`). Prove: `go test ./internal/integrate ./internal/run ./cmd/lucind-ai -count=1`.

## Phase 2: CLI Worktree Cleanup Command

- [ ] 2.1 RED: In `cmd/lucind-ai/cli_test.go:2974-3024` add CLI tests in `TestWorktreeCleanupCLI`: assert unforced dirty cleanup exits 1 with stderr diagnostics referencing `troubleshooting.md:7-18` and preserves files, dirty cleanup with `--force`/`-f` exits 0 and removes files, clean cleanup exits 0, nonexistent lane cleanup exits 0 idempotently, missing `--lane` exits nonzero, and invocation inside linked worktree exits 1.
- [ ] 2.2 GREEN: In `cmd/lucind-ai/cli.go:58,1934-1978` update `runWorktreeCleanup` to parse `--force`/`-f` flag; invoke `worktree.Cleanup(ctx, primaryRoot, *laneID, force)`; on `errors.Is(err, worktree.ErrWorktreeDirty)` format porcelain status diff diagnostic referencing `troubleshooting.md:7-18` and exit 1; update `const usage` (`:58`). Prove: `go test ./cmd/lucind-ai -run TestWorktreeCleanupCLI`.

## Phase 3: CLI Failure Guidance Banners & Operator Documentation

- [ ] 3.1 RED: In `cmd/lucind-ai/cli_test.go:685-777,4503-4545` and `internal/dag/split_test.go:13-111` add test cases asserting: (1) `printReport` emits non-done warning banner with worktree path, diff inspection steps, and `troubleshooting.md:7-18` reference when `Status != lane.Done`, and omits on `lane.Done`; (2) `printIntegrateReport` emits `integrate retry` banner citing `recovery-reconciliation.md:33-35` when `len(reverted_ids) > 0`, and omits when empty; (3) `renderAcceptanceReceipt` emits qualitative checklist reminder citing `acceptance-promotion.md:18-30` steps 2–10 on mechanical pass; (4) `runSplit` / `dag.Split` emits multi-wave warning to stderr citing `recovery-reconciliation.md:27-30` when `len(waves) > 1`, and omits on single wave.
- [ ] 3.2 GREEN: In `cmd/lucind-ai/cli.go:698-726` update `printReport` to append non-done diagnostic banner citing `troubleshooting.md:7-18` on `r.Status != lane.Done`.
- [ ] 3.3 GREEN: In `cmd/lucind-ai/cli.go:730-740` update `printIntegrateReport` to append retry guidance banner citing `recovery-reconciliation.md:33-35` when `len(rep.Reverted) > 0`.
- [ ] 3.4 GREEN: In `cmd/lucind-ai/cli.go:685-690` update `renderAcceptanceReceipt` to append qualitative checklist reminder banner citing `acceptance-promotion.md:18-30`.
- [ ] 3.5 GREEN: In `cmd/lucind-ai/cli.go:485-516` and `internal/dag/split.go:34-43` update `runSplit` / `dag.Split` to emit multi-wave checkout and SHA refresh warning to stderr when `len(waves) > 1`. Prove 3.1–3.5: `go test ./cmd/lucind-ai ./internal/dag -run 'TestPrint|TestAccept|TestSplit'`.
- [ ] 3.6 DOC: Update operational references in `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` (TDD WIP-rescue table entry), `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-35` (reverted retry & multi-wave base SHA discipline), and `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` (10-step qualitative checklist reminder).
- [ ] 3.7 DOC: Update `.agents/skills/lucind-apply/SKILL.md:10-21` documenting the TDD WIP-rescue protocol referencing `troubleshooting.md:7-18` for timed-out or blocked apply lanes.

## Dependency order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | None | Root prerequisite: unit test seams for `worktree` guardrails |
| 1.2 | 1.1 | Implement `ErrWorktreeDirty` sentinel and `force bool` parameter on `Cleanup`/`Remove` |
| 1.3 | 1.2 | Update automated internal teardowns across `integrate.go`, `candidate.go`, `run/integrate.go`, and `cli.go` to satisfy updated signature |
| 2.1 | 1.2 | RED tests for `worktree cleanup` CLI flag parsing, exit codes, and diagnostics |
| 2.2 | 2.1, 1.2, 1.3 | Implement `--force`/`-f` in `runWorktreeCleanup` consuming updated `worktree.Cleanup` signature |
| 3.1 | None | RED tests for CLI report banners, receipt qualitative reminder, and DAG split stderr warning |
| 3.2 | 3.1 | Implement non-done diagnostic banner in `printReport` |
| 3.3 | 3.1 | Implement reverted IDs retry guidance banner in `printIntegrateReport` |
| 3.4 | 3.1 | Implement qualitative checklist reminder in `renderAcceptanceReceipt` |
| 3.5 | 3.1 | Implement multi-wave stderr warning in `runSplit` / `dag.Split` |
| 3.6 | None | Update operational documentation in `plugin/claude-code/skills/lucind-ai/references/` |
| 3.7 | 3.6 | Update `lucind-apply/SKILL.md` referencing `troubleshooting.md` |

## Threat-matrix RED tests

| Adversarial case | RED task | Asserts | Precedes |
|---|---|---|---|
| Git repository selection | 1.1, 2.1 | `runWorktreeCleanup` checks `resolvePrimaryRoot`, refusing execution inside linked worktree with exit 1; `worktree.Remove` fails cleanly on invalid paths without panic | 1.2, 2.2 |
| Commit state | 1.1, 2.1 | `worktree.Remove` and `worktree.Cleanup` return `ErrWorktreeDirty` and preserve files when staged (`M `), unstaged (` M`), or untracked (`??`) edits exist unless `force: true`; CLI exits 1 without `--force` | 1.2, 2.2 |

## Requirement traceability

| Requirement | Tasks |
|---|---|
| `worktree-dirty-guardrail`: Worktree dirty guardrail check | 1.1, 1.2 |
| `lane-worktree-lifecycle`: Lane worktree lifecycle force parameter and automated teardown | 1.1, 1.2, 1.3 |
| `worktree-cleanup-cli`: Worktree cleanup CLI force flag and diagnostic status reporting | 2.1, 2.2 |
| `failure-guidance-banners`: Blocked and timeout lane report guidance banner | 3.1, 3.2 |
| `failure-guidance-banners`: Integration report reverted IDs recovery banner | 3.1, 3.3 |
| `failure-guidance-banners`: Acceptance receipt qualitative review banner | 3.1, 3.4 |
| `failure-guidance-banners`: DAG split multi-wave base SHA warning banner | 3.1, 3.5 |
| `tdd-wip-rescue-protocol`: Prescriptive TDD WIP-rescue protocol documentation | 3.6, 3.7 |
