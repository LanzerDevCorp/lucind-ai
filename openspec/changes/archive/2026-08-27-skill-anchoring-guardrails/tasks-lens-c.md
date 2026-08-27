# Tasks Lens C — Proof & Review Burden: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed decomposition

Independent analysis assumes a 4-phase sequential implementation:
- **Phase 1 (Foundation & Core Guardrails)**: Extend `worktree.Cleanup` and `worktree.Remove` signatures with `force bool`, export `ErrWorktreeDirty`, verify `PorcelainEmpty` before removal when unforced (`internal/worktree/worktree.go:26-45,247-261,319-325`), and update automated teardown callers to pass `force: true` (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`).
- **Phase 2 (CLI Worktree Cleanup)**: Add `--force`/`-f` flag parsing to `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:58,1934-1978`), fail closed with exit 1 on `ErrWorktreeDirty`, print porcelain diff diagnostics referencing `troubleshooting.md:7-18`, and preserve worktree files.
- **Phase 3 (Milestone Guidance Banners)**: Implement targeted guidance banners at four execution milestones: `printReport` non-done diagnostic banner (`cmd/lucind-ai/cli.go:698-726`), `printIntegrateReport` reverted IDs retry banner (`cmd/lucind-ai/cli.go:730-740`), `renderAcceptanceReceipt` qualitative checklist banner (`cmd/lucind-ai/cli.go:685-690`), and `runSplit` multi-wave stderr warning banner (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`).
- **Phase 4 (Skill Anchoring & Documentation)**: Update operational references in `troubleshooting.md:7-18`, `recovery-reconciliation.md:27-35`, `acceptance-promotion.md:18-30`, and `.agents/skills/lucind-apply/SKILL.md:10-21` with TDD WIP-rescue and banner reconciliation protocols.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~450 lines (basis: `internal/worktree/worktree.go` signature & error ~25 lines, `internal/worktree/worktree_test.go` dirty & force tests ~65 lines, caller sites `integrate.go`/`candidate.go`/`run/integrate.go` ~10 lines, `cmd/lucind-ai/cli.go` flag & 4 banners ~75 lines, `cmd/lucind-ai/cli_test.go` suite updates ~120 lines, `internal/dag/` split updates ~35 lines, documentation & skill prose ~120 lines) |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | single-pr (human-confirmed) |
| Chain strategy | pending (only if chaining needed) |

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths | N/A: Change manages git worktree cleanup and guidance banners; does not route or execute doc files. | None (N/A) | N/A: Threat boundary outside worktree safety and banner routing. | None (N/A) |
| Git repository selection | Applicable | `TestWorktreeCleanupCLI_RejectsInsideLinkedWorktree` and `TestRemove_FailsOnInvalidPath` (`cmd/lucind-ai/cli_test.go:2974-3024`, `internal/worktree/worktree_test.go:240-266`) | `runWorktreeCleanup` checks `resolvePrimaryRoot`, refusing execution inside linked worktree with exit 1; `worktree.Remove` fails cleanly on invalid primary/worktree paths without panic (`cmd/lucind-ai/cli.go:1959-1969`, `internal/worktree/worktree.go:256-261`). | Phase 1 `worktree.Remove` path validation and Phase 2 CLI primary root check (`internal/worktree/worktree.go:247-261`, `cmd/lucind-ai/cli.go:1934-1978`). |
| Commit state | Applicable | `TestRemove_DirtyWorktreeFailsClosedWithoutForce` and `TestWorktreeCleanupCLI_DirtyRefusesWithoutForce` (`internal/worktree/worktree_test.go:536-595`, `cmd/lucind-ai/cli_test.go:2974-3024`) | `worktree.Remove` and `worktree.Cleanup` return `ErrWorktreeDirty` and preserve files when staged (`M `), unstaged (` M`), or untracked (`??`) edits exist unless `force: true`; CLI exits 1 without `--force` (`internal/worktree/worktree.go:26-45,247-261,319-325`, `cmd/lucind-ai/cli.go:1934-1978`). | Phase 1 core dirty guardrail check and Phase 2 CLI `--force` flag enforcement (`internal/worktree/worktree.go:247-261`, `cmd/lucind-ai/cli.go:1934-1978`). |
| Push state | N/A: Change does not execute git push or interact with remote refspecs. | None (N/A) | N/A: No git push or remote refspec interactions in scope. | None (N/A) |
| PR commands | N/A: Change does not construct or invoke pull request commands or platform CLI tools. | None (N/A) | N/A: No pull request creation or platform CLI invocations in scope. | None (N/A) |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Worktree dirty guardrail unit safety | `go test -v ./internal/worktree -run 'TestRemove|TestCleanup|TestPorcelainEmpty'` (`internal/worktree/worktree_test.go:240-266,536-595,1034-1069`) | `worktree.Remove` and `Cleanup` return `ErrWorktreeDirty` preserving files when dirty without force; clean trees remove idempotently; `force: true` overrides dirty check. | Does not prove CLI flag parsing, terminal formatting, or banner output streams. |
| CLI worktree cleanup `--force` flag & error handling | `go test -v ./cmd/lucind-ai -run TestWorktreeCleanupCLI` (`cmd/lucind-ai/cli_test.go:2974-3024`) | `lucind-ai worktree cleanup` parses `--force`/`-f`, exits 1 with diagnostic diffs on unforced dirty tree, exits 0 when clean, nonexistent, or forced. | Does not prove failure banners in run dispatch, integrate reporting, or receipt rendering. |
| Non-done report diagnostic banner | `go test -v ./cmd/lucind-ai -run 'TestPrintReportOmitsDiagnosisBlock'` (`cmd/lucind-ai/cli_test.go:685-724`) | `printReport` outputs preserved worktree path, diff inspection steps, and `troubleshooting.md` reference on non-done status; omits banner on `lane.Done`. | Does not prove integrate retry banner or receipt review reminders. |
| Reverted IDs integrate recovery banner | `go test -v ./cmd/lucind-ai -run 'TestPrintIntegrateReport'` (`cmd/lucind-ai/cli_test.go:729-777`) | `printIntegrateReport` appends `integrate retry` instructions citing `recovery-reconciliation.md` when `reverted_ids` is non-empty; formats clean list when empty. | Does not prove DAG split multi-wave warnings or mechanical acceptance banners. |
| Acceptance receipt qualitative reminder | `go test -v ./cmd/lucind-ai -run TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (`cmd/lucind-ai/cli_test.go:4503-4545`) | `renderAcceptanceReceipt` appends qualitative checklist reminder citing `acceptance-promotion.md` steps 2–10 on mechanical verification pass. | Does not prove qualitative checklist completion by human reviewer. |
| DAG split multi-wave stderr warning | `go test -v ./internal/dag -run TestSplit_TwoWaveDAGSuccess` (`internal/dag/split_test.go:13-111`) | `dag.Split` emits multi-wave warning banner to stderr instructing checkout advance and SHA refresh while keeping stdout wave commands parseable. | Does not prove operator actually advances git checkout between waves. |
| Repository integration & regression prevention | `./lucind-checks.sh` (`lucind-checks.sh:1-4`) | Entire repository compiles and passes all unit, integration, and race-detector tests with zero regressions across automated callers. | Does not prove documentation prose clarity in Markdown skills. |

## Verification Gaps

None. All required safety behaviors, CLI flags, output banners, and automated caller paths are verified by executable unit and CLI tests without requiring new mock seams.

## Open Questions

- [ ] None. All architecture decisions, signature updates, and banner routing strategies are fully specified and frozen in `design.md` and spec deltas.

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Guides strict TDD lifecycle and documentation references for apply lanes. |
| `cmd/lucind-ai/cli.go:58-58` | Usage string documents CLI syntax for `worktree cleanup` and subcommands. |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` CLI handler parses DAG arguments and executes `dag.Split`. |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` formats mechanical acceptance output and receipt metadata. |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` renders lane execution summary and non-done failure banner. |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` formats integration outcome, integrated IDs, and reverted IDs. |
| `cmd/lucind-ai/cli.go:858-869` | `productionDeps` wires `DiscardCombined` and `RemoveLaneWorktree` closures. |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` implements `lucind-ai worktree cleanup` subcommand logic and primary root checks. |
| `cmd/lucind-ai/cli_test.go:685-724` | Unit tests asserting `printReport` output formatting and non-done diagnosis banners. |
| `cmd/lucind-ai/cli_test.go:729-777` | Unit tests asserting `printIntegrateReport` output formatting and reverted IDs lines. |
| `cmd/lucind-ai/cli_test.go:2974-3024` | CLI tests asserting `lucind-ai worktree cleanup` execution, idempotency, and argument validation. |
| `cmd/lucind-ai/cli_test.go:4503-4545` | CLI test asserting `lucind-ai accept` mechanical evidence rendering and receipt output. |
| `internal/dag/split.go:34-43` | `dag.Split` formats and writes wave dispatch commands to stdout. |
| `internal/dag/split_test.go:13-111` | Unit test verifying two-wave DAG splitting and output command generation. |
| `internal/integrate/candidate.go:262-265` | `ResolveCandidate` performs automated worktree cleanup upon candidate resolution. |
| `internal/integrate/integrate.go:118-124` | `Combine` executes automated worktree cleanup on merge conflict abort. |
| `internal/run/integrate.go:159-165` | `completeIntegration` cleans up lane worktrees following batch integration. |
| `internal/worktree/worktree.go:26-45` | Sentinel error definitions for the `worktree` package. |
| `internal/worktree/worktree.go:150-159` | `pathFor` computes linked worktree filesystem path from lane ID. |
| `internal/worktree/worktree.go:247-261` | `Cleanup` and `Remove` implement worktree directory removal logic. |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` evaluates git status porcelain output in worktree directory. |
| `internal/worktree/worktree_test.go:240-266` | Unit tests asserting `worktree.Remove` linked worktree deletion. |
| `internal/worktree/worktree_test.go:536-595` | Unit tests asserting `worktree.PorcelainEmpty` status inspection behavior. |
| `internal/worktree/worktree_test.go:1034-1069` | Unit tests asserting `worktree.Cleanup` removal and idempotent no-op handling. |
| `lucind-checks.sh:1-4` | Shell script defining the full repository compilation and race-detector test suite. |
| `openspec/changes/skill-anchoring-guardrails/design.md:110-137` | Design specification detailing testing strategy, test seams, and threat matrix. |
| `openspec/changes/skill-anchoring-guardrails/proposal.md:40-46` | Accepted proposal approach defining guardrail architecture and guidance banners. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Documents the canonical 10-step post-lane acceptance protocol and checklist. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-35` | Documents multi-wave base SHA discipline and bisection retry procedures. |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Documents troubleshooting table for lane failures, dirty roots, and retry procedures. |
