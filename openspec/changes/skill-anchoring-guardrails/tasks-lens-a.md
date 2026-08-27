# Tasks Lens A — Decomposition & Ordering: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed decomposition

The implementation decomposes into three sequential phases: (1) core worktree signatures, dirty sentinel error, and automated teardown callers; (2) CLI cleanup subcommand flag parsing, fail-closed handling, and production dependency wiring; (3) CLI failure guidance banners and operational documentation for TDD WIP-rescue. The critical path requires Phase 1 before Phase 2 because CLI cleanup and production dependencies depend on the updated `worktree.Cleanup` and `worktree.Remove` signatures. Phase 3 depends on Phases 1 and 2 only through shared call sites in `cmd/lucind-ai/cli.go` and reference documentation.

## Phase 1: Core Worktree Guardrail & Internal Callers

- [ ] 1.1 Export `ErrWorktreeDirty` sentinel error in `internal/worktree/worktree.go:26-45`.
- [ ] 1.2 Update `worktree.Cleanup` and `worktree.Remove` signatures to accept trailing `force bool` in `internal/worktree/worktree.go:247-261`, checking `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) when `force == false` and returning `ErrWorktreeDirty`.
- [ ] 1.3 Update internal automated removal call sites in `internal/integrate/integrate.go:118-124` (merge conflict abort) and `internal/integrate/candidate.go:262-265` (candidate promotion teardown) passing `force: true`.
- [ ] 1.4 Update unit tests in `internal/worktree/worktree_test.go:244-266,536-595,1034-1069` for `force bool`, asserting clean removal, dirty unforced rejection returning `ErrWorktreeDirty` with files preserved, forced removal with `force: true`, and idempotent clean/nonexistent removal.

## Phase 2: CLI Worktree Cleanup Command & Production Dependencies

- [ ] 2.1 Update `productionDeps` closures `DiscardCombined` and `RemoveLaneWorktree` in `cmd/lucind-ai/cli.go:858-869` passing `force: true` to `worktree.Remove`.
- [ ] 2.2 Update `runWorktreeCleanup` in `cmd/lucind-ai/cli.go:58,1934-1978` to parse `--force`/`-f`, calling `worktree.Cleanup(ctx, primaryRoot, *laneID, force)` and formatting dirty diagnostics citing `troubleshooting.md:7-18` on exit 1.
- [ ] 2.3 Update CLI tests in `cmd/lucind-ai/cli_test.go:2974-3024` asserting clean cleanup (exit 0), dirty cleanup without `--force` failing with exit 1 and stderr diagnostics, dirty cleanup with `--force`/`-f` (exit 0), and nonexistent lane idempotency (exit 0).

## Phase 3: CLI Guidance Banners & Operator Documentation

- [ ] 3.1 Embed non-done troubleshooting guidance banner in `printReport` (`cmd/lucind-ai/cli.go:698-726`) on `r.Status != lane.Done` citing `troubleshooting.md:7-18`.
- [ ] 3.2 Embed retry guidance banner in `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) when `len(rep.Reverted) > 0` citing `recovery-reconciliation.md:33-35`.
- [ ] 3.3 Embed qualitative checklist reminder banner in `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) citing `acceptance-promotion.md:18-30`.
- [ ] 3.4 Embed multi-wave checkout and SHA refresh stderr warning banner in `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) citing `recovery-reconciliation.md:27-30`.
- [ ] 3.5 Update CLI and DAG tests in `cmd/lucind-ai/cli_test.go:685-777,4503-4545` and `internal/dag/split_test.go:13-111` asserting all four guidance banners render on trigger and omit when not triggered.
- [ ] 3.6 Update operational references in `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-35`, and `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` documenting TDD WIP-rescue protocol, recovery steps, and qualitative review checklist.
- [ ] 3.7 Update `.agents/skills/lucind-apply/SKILL.md:10-21` documenting the TDD WIP-rescue protocol referencing `troubleshooting.md:7-18` for timed-out or blocked apply lanes.

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | None | `ErrWorktreeDirty` sentinel error is a root prerequisite for worktree dirty checks. |
| 1.2 | 1.1 | `worktree.Cleanup` and `worktree.Remove` return `ErrWorktreeDirty` when `PorcelainEmpty` reports dirty. |
| 1.3 | 1.2 | `Combine` conflict abort and `ResolveCandidate` teardown must satisfy the updated `worktree.Remove` signature. |
| 1.4 | 1.2, 1.3 | Unit tests compile and verify `worktree` package dirty guardrail and automated teardown invariants. |
| 2.1 | 1.2 | `productionDeps` closures invoke `worktree.Remove` with `force: true` for combined and lane trees. |
| 2.2 | 1.1, 1.2, 2.1 | `runWorktreeCleanup` consumes the new `worktree.Cleanup(..., force)` signature and `ErrWorktreeDirty` sentinel. |
| 2.3 | 2.2 | CLI cleanup tests exercise `runWorktreeCleanup` flag parsing and fail-closed exit behavior. |
| 3.1 | None | `printReport` non-done banner modifies CLI report output format independently of worktree signatures. |
| 3.2 | None | `printIntegrateReport` retry guidance banner is an independent formatting addition in `cli.go`. |
| 3.3 | None | `renderAcceptanceReceipt` qualitative checklist banner is an independent formatting addition in `cli.go`. |
| 3.4 | None | `runSplit` multi-wave stderr warning banner is an independent stream formatting addition. |
| 3.5 | 3.1, 3.2, 3.3, 3.4 | Banner tests in `cli_test.go` and `split_test.go` verify all banner rendering and suppression scenarios. |
| 3.6 | None | Operational reference documentation in `plugin/claude-code/skills/` is independent of code changes. |
| 3.7 | 3.6 | `lucind-apply/SKILL.md` references the TDD WIP-rescue protocol established in `troubleshooting.md`. |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `worktree-dirty-guardrail`: Worktree dirty guardrail check | 1.1, 1.2, 1.4 |
| `lane-worktree-lifecycle`: Lane worktree lifecycle force parameter and automated teardown | 1.2, 1.3, 1.4, 2.1 |
| `worktree-cleanup-cli`: Worktree cleanup CLI force flag and diagnostic status reporting | 2.2, 2.3 |
| `failure-guidance-banners`: Blocked and timeout lane report guidance banner | 3.1, 3.5 |
| `failure-guidance-banners`: Integration report reverted IDs recovery banner | 3.2, 3.5 |
| `failure-guidance-banners`: Acceptance receipt qualitative review banner | 3.3, 3.5 |
| `failure-guidance-banners`: DAG split multi-wave base SHA warning banner | 3.4, 3.5 |
| `tdd-wip-rescue-protocol`: Prescriptive TDD WIP-rescue protocol documentation | 3.6, 3.7 |

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Strict TDD lifecycle and sweep verification steps updated with WIP-rescue guidance. |
| `cmd/lucind-ai/cli.go:58` | Subcommand usage string declaring `worktree cleanup` syntax. |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` implementation parsing apply DAG and outputting wave commands. |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` printing mechanical acceptance receipt details. |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` formatting lane outcome and non-done diagnosis. |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` printing integrate counts and lane ID lists. |
| `cmd/lucind-ai/cli.go:858-869` | `productionDeps` closures `DiscardCombined` and `RemoveLaneWorktree` invoking `worktree.Remove`. |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` implementing worktree cleanup command logic. |
| `cmd/lucind-ai/cli_test.go:685-777` | Unit tests for `printReport` diagnosis and `printIntegrateReport` ID lists. |
| `cmd/lucind-ai/cli_test.go:2974-3024` | `TestWorktreeCleanupCLI` testing stale worktree removal, idempotency, and flag errors. |
| `cmd/lucind-ai/cli_test.go:4503-4545` | `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` testing acceptance receipt output. |
| `internal/dag/split.go:34-43` | `Split` iterating waves and printing `lucind-ai run` commands to stdout. |
| `internal/dag/split_test.go:13-111` | `TestSplit_TwoWaveDAGSuccess` asserting multi-wave packet emission and stdout command lines. |
| `internal/integrate/candidate.go:262-265` | `ResolveCandidate` teardown removing promotion worktree and branch. |
| `internal/integrate/integrate.go:118-124` | `Combine` merge conflict handler aborting merge and removing worktree. |
| `internal/run/integrate.go:159-165` | `completeIntegration` tearing down integrated lane worktrees via `deps.RemoveLaneWorktree`. |
| `internal/worktree/worktree.go:26-45` | Sentinel errors defined for worktree operations. |
| `internal/worktree/worktree.go:150-159` | `pathFor` computing linked worktree disk paths. |
| `internal/worktree/worktree.go:247-261` | `Cleanup` and `Remove` functions performing worktree deletion. |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` inspecting working directory status via `git status --porcelain`. |
| `internal/worktree/worktree_test.go:244-266` | `TestRemove` verifying worktree removal. |
| `internal/worktree/worktree_test.go:536-595` | `TestPorcelainEmpty` asserting clean state, gitignore respect, and dirty detection. |
| `internal/worktree/worktree_test.go:1034-1069` | `TestCleanupRemovesExistingWorktree` and `TestCleanupOnLaneWithNoWorktreeIsNoOp`. |
| `openspec/changes/skill-anchoring-guardrails/design.md:5-14` | Technical approach overview for guardrails, teardowns, banners, and WIP rescue. |
| `openspec/changes/skill-anchoring-guardrails/design.md:17-44` | Architecture decisions 1 through 4 defining signatures, sentinels, dirty checks, and banners. |
| `openspec/changes/skill-anchoring-guardrails/design.md:78-84` | System invariants for fail-closed deletion, automation, idempotency, and stream separation. |
| `openspec/changes/skill-anchoring-guardrails/design.md:98-108` | File changes table defining modified files, actions, and terminal consumers. |
| `openspec/changes/skill-anchoring-guardrails/design.md:109-127` | Testing strategy and test seams across unit, CLI, banner, and integration layers. |
| `openspec/changes/skill-anchoring-guardrails/specs/failure-guidance-banners/spec.md:5-18` | Requirement and scenarios for blocked and timeout lane report guidance banner. |
| `openspec/changes/skill-anchoring-guardrails/specs/failure-guidance-banners/spec.md:19-32` | Requirement and scenarios for integration report reverted IDs recovery banner. |
| `openspec/changes/skill-anchoring-guardrails/specs/failure-guidance-banners/spec.md:33-46` | Requirement and scenarios for acceptance receipt qualitative review banner. |
| `openspec/changes/skill-anchoring-guardrails/specs/failure-guidance-banners/spec.md:47-59` | Requirement and scenarios for DAG split multi-wave base SHA warning banner. |
| `openspec/changes/skill-anchoring-guardrails/specs/lane-worktree-lifecycle/spec.md:5-17` | Requirement and scenarios for lane worktree lifecycle force parameter and automated teardown. |
| `openspec/changes/skill-anchoring-guardrails/specs/tdd-wip-rescue-protocol/spec.md:5-12` | Requirement and scenario for prescriptive TDD WIP-rescue protocol documentation. |
| `openspec/changes/skill-anchoring-guardrails/specs/worktree-cleanup-cli/spec.md:5-22` | Requirement and scenarios for worktree cleanup CLI force flag and diagnostic status reporting. |
| `openspec/changes/skill-anchoring-guardrails/specs/worktree-dirty-guardrail/spec.md:5-22` | Requirement and scenarios for worktree dirty guardrail check. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step acceptance protocol and qualitative review checklist. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing guidelines for advancing checkout and refreshing base SHAs. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Bisection boundary and integrate retry protocol for reverted lanes. |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Dispatch and integration troubleshooting table covering dirty roots, reverts, and timeouts. |
