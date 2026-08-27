# Design Lens C — Failure, Test & Rollback: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed architecture

This design assumes Candidate 1 from the proposal:
1. `internal/worktree/worktree.go`: Exports sentinel `ErrWorktreeDirty`. Updates `Cleanup(ctx, primaryRoot, laneID string, force bool) error` and `Remove(ctx, primaryRoot, path string, force bool) error` (`internal/worktree/worktree.go:247-261`). When `force` is `false`, `Remove` queries `PorcelainEmpty(ctx, path)` (`internal/worktree/worktree.go:319-325`); if dirty, it aborts without deleting files and returns `ErrWorktreeDirty`. When `force` is `true` or porcelain is empty, it executes `git worktree remove --force`. Nonexistent lane cleanup remains idempotent.
2. `cmd/lucind-ai/cli.go`: Updates `runWorktreeCleanup` (`cmd/lucind-ai/cli.go:1934-1978`) to accept `--force` / `-f`. Unforced cleanup on dirty worktrees prints porcelain status, inspection commands (`git diff`), and references `troubleshooting.md:7-18`, exiting with code 1 while preserving the worktree on disk.
3. Automated internal teardowns pass `force: true`: `DiscardCombined` and `RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:858-869`, `internal/run/integrate.go:159-165`), merge conflict abort (`internal/integrate/integrate.go:118-124`), and `ResolveCandidate` promotion (`internal/integrate/candidate.go:262-265`).
4. Static terminal guidance banners embedded at four milestones:
   - `printReport` (`cmd/lucind-ai/cli.go:698-726`) on non-done status -> `troubleshooting.md:7-18`.
   - `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) on non-empty `reverted_ids` -> `recovery-reconciliation.md:33-35` (`integrate retry`).
   - `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) -> `acceptance-promotion.md:18-30` (checklist steps 2–10).
   - `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) on multi-wave DAGs -> `recovery-reconciliation.md:27-30` (advance checkout, refresh `base_sha` and `expected_parent_sha`).

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Worktree Unit | Signature updates, clean removal, dirty fail-closed, and forced override | Update `TestRemove` and `TestCleanupRemovesExistingWorktree` with `force bool`. Assert clean worktree removes with `force: false`; dirty worktree with `force: false` returns `ErrWorktreeDirty` and preserves files; dirty worktree with `force: true` removes successfully. Assert `TestCleanupOnLaneWithNoWorktreeIsNoOp` returns `nil`. Verify `TestPorcelainEmpty` ignores `.gitignore` files. | `internal/worktree/worktree.go:247-261`, `internal/worktree/worktree_test.go:255-266`, `internal/worktree/worktree_test.go:536-595`, `internal/worktree/worktree_test.go:1034-1069` |
| CLI Cleanup | Exit codes, flags, output streams, and idempotency | Update `TestWorktreeCleanupCLI`. Assert clean worktree removes without `--force` (exit 0); dirty worktree without `--force` fails with exit 1, preserves worktree, and prints dirty status to stderr; dirty worktree with `--force` / `-f` removes worktree (exit 0); nonexistent lane cleanup exits 0; missing `--lane` exits nonzero. | `cmd/lucind-ai/cli.go:1934-1978`, `cmd/lucind-ai/cli_test.go:2974-3020` |
| CLI Banners | Banner rendering at failure and coordination gates | (1) `TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured` & `TestPrintReportOmitsDiagnosisBlockForDoneLane` verify non-done recovery banner referencing `troubleshooting.md:7-18`; (2) `TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs` verifies `integrate retry` instructions referencing `recovery-reconciliation.md:33-35` when `reverted_ids` is present; (3) `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` verifies receipt output and qualitative checklist reminder referencing `acceptance-promotion.md:18-30`; (4) `TestSplit_TwoWaveDAGSuccess` verifies stdout wave commands remain parseable while stderr receives multi-wave warning banner. | `cmd/lucind-ai/cli.go:485-516`, `cmd/lucind-ai/cli.go:685-690`, `cmd/lucind-ai/cli.go:698-726`, `cmd/lucind-ai/cli.go:730-750`, `cmd/lucind-ai/cli_test.go:685-724`, `cmd/lucind-ai/cli_test.go:729-777`, `cmd/lucind-ai/cli_test.go:4503-4545`, `internal/dag/split.go:34-43`, `internal/dag/split_test.go:13-111` |
| Internal Integration | Regression prevention across machine callers | Verify automated internal callers pass `force: true` across promotion, conflict recovery, and combined tree teardowns by running full repository check suite (`lucind-checks.sh:1-4`). | `cmd/lucind-ai/cli.go:858-869`, `internal/integrate/candidate.go:262-265`, `internal/integrate/integrate.go:118-124`, `internal/run/integrate.go:159-165`, `lucind-checks.sh:1-4` |

## Test Seams

- **Injectable / Fakeable Today**:
  - `worktree.DefaultGitRunner` (`internal/worktree/worktree.go:47-72`): `GitRunner` interface allows mocking git command execution, while `initRepo(t)` (`internal/worktree/worktree_test.go:244,1039`) provides isolated real git repositories in temp directories.
  - `worktree.PorcelainEmpty` (`internal/worktree/worktree.go:319-325`): Directly evaluated against real filesystem state and `.gitignore` configurations (`internal/worktree/worktree_test.go:536-595`).
  - CLI execution seam `run(ctx, args, stdout, stderr)`: Direct injection of `bytes.Buffer` captures stdout and stderr independently in in-memory tests (`cmd/lucind-ai/cli_test.go:2995-2998,4532-4535`).
  - `acceptVerifierFactory` (`cmd/lucind-ai/cli_test.go:4519-4521`): Package-level hook allows injecting mock acceptance verification results.
  - `dag.Split(dagPath, outDir, stdout)` (`internal/dag/split.go:34-43`): Accepts `io.Writer` directly for stdout verification (`internal/dag/split_test.go:59-63`).
- **New Seams Required**: None required. Existing seams provide full coverage for unit, CLI integration, and output banner verification.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: Change manages git worktree cleanup and terminal guidance banners; does not route, execute, or classify doc files as build scripts. | N/A: Not applicable to this change. | None (N/A). |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | `Cleanup` resolves path via `pathFor(primaryRoot, laneID)` (`internal/worktree/worktree.go:150-159`) and verifies linked worktree identity (`internal/worktree/worktree.go:271-292`). `runWorktreeCleanup` checks `resolvePrimaryRoot` and rejects invocation inside linked worktrees (`cmd/lucind-ai/cli.go:1959-1969`). | `TestWorktreeCleanupCLI` verifies rejection when run inside linked worktree; unit tests verify `Remove` fails cleanly on invalid paths without deleting outside directories. |
| Commit state | staged, `commit -a`, empty index | Applicable | `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) evaluates `git status --porcelain`. Any staged edit (`M  `), unstaged modification (` M`), or untracked file (`??`) returns `false`, causing unforced `Cleanup`/`Remove` to fail closed with `ErrWorktreeDirty`. Purely ignored files remain clean. | `TestRemove` and `TestCleanup` return `ErrWorktreeDirty` preserving files when index has staged changes, unstaged changes, or untracked files (`internal/worktree/worktree_test.go:536-595`); `TestWorktreeCleanupCLI` fails with exit 1 on dirty trees without `--force`. |
| Push state | tracking branch, first push, explicit refspec | N/A: Change does not execute git push or interact with remote refspecs. | N/A: Not applicable to this change. | None (N/A). |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: Change does not construct or invoke pull request commands or platform CLI tools. | N/A: Not applicable to this change. | None (N/A). |

## Rollback and Additivity

**Choice**: Single `git revert` of the merge commit.
**Alternatives considered**: Feature flags / configuration toggle (rejected: adds dead configuration paths for purely additive safety guardrails); partial branch revert (rejected: worktree signature changes and CLI updates are co-dependent).
**Rationale**: The change introduces zero persistent database migrations, SQLite ledger table alterations (`internal/ledger/acceptance.go:41-47`), or result schema changes (`internal/result/result.go:26-34`). Reverting the commit restores prior unconditional cleanup behavior and removes banner messages with zero data migration or ledger state reconciliation.

No schema, ledger, or result envelope version moves.

## Out of Scope

- Background daemon or automatic cron-based garbage collection of stale worktrees.
- Schema modifications to `.lucind/result.json` or `result.schema.json` (`internal/result/result.go:26-34`).
- Automatic git commits created by the binary upon lane timeout or failure (`internal/run/run.go:452-465`).
- Modifying feature lease acquisition, fencing tokens, or lease renewal in `internal/feature/feature.go:348-350`.
- Altering core acceptance ledger storage or receipt hashing (`internal/accept/accept.go:121-130`, `internal/ledger/acceptance.go:41-47`).
- Re-architecting multi-agent orchestrator adapters outside `plugin/claude-code/skills/lucind-ai/`.

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Strict TDD Lifecycle defining RED, GREEN, and SWEEP phases |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` emits wave commands to stdout and requires multi-wave warning banner |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` emits mechanical acceptance receipt lines |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` renders lane summary and non-done banner |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` formats integration summary count and ID lists |
| `cmd/lucind-ai/cli.go:742-750` | `printIDList` formats single-line `integrated_ids` and `reverted_ids` records |
| `cmd/lucind-ai/cli.go:858-869` | `DiscardCombined` and `RemoveLaneWorktree` dependencies invoking `worktree.Remove` |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` CLI implementation invoking `worktree.Cleanup` |
| `cmd/lucind-ai/cli_test.go:685-724` | `TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured` and `TestPrintReportOmitsDiagnosisBlockForDoneLane` asserting report format |
| `cmd/lucind-ai/cli_test.go:729-777` | `TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs` and `TestPrintIntegrateReportAllIntegratedExplicitlyEmptyRevertedIDs` asserting integrate report output |
| `cmd/lucind-ai/cli_test.go:2974-3020` | `TestWorktreeCleanupCLI` testing stale worktree removal, idempotent non-existent lane cleanup, and missing `--lane` flag |
| `cmd/lucind-ai/cli_test.go:4503-4545` | `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` asserting mechanical receipt stdout records |
| `internal/accept/accept.go:121-130` | `AcceptanceVerifier.Verify` constructs immutable `AcceptanceReceipt` and persists it to ledger |
| `internal/dag/split.go:34-43` | `Split` formats and emits `lucind-ai run` wave commands to stdout |
| `internal/dag/split_test.go:13-111` | `TestSplit_TwoWaveDAGSuccess` asserting stdout wave command lines |
| `internal/feature/feature.go:348-350` | Feature lease acquisition and monotonic fencing token invariants remain unchanged |
| `internal/integrate/candidate.go:262-265` | `ResolveCandidate` removes linked worktree and branch on successful promotion via `worktree.Remove` |
| `internal/integrate/integrate.go:118-124` | `Combine` merge conflict handler aborting merge and invoking `worktree.Remove` and `worktree.DeleteBranch` |
| `internal/ledger/acceptance.go:41-47` | `AcceptanceReceipt` struct definition and ledger schema remain unchanged |
| `internal/result/result.go:26-34` | `HardStop` and `FileChange` envelope structures in `.lucind/result.json` remain unchanged |
| `internal/run/integrate.go:159-165` | `completeIntegration` invokes `deps.RemoveLaneWorktree` for integrated lanes |
| `internal/run/run.go:452-465` | Lane timeout handling where timed-out lanes record diagnosis with worktree preserved |
| `internal/worktree/worktree.go:47-72` | `GitRunner` interface and `DefaultGitRunner` implementation for git command execution |
| `internal/worktree/worktree.go:150-159` | `pathFor` computes linked worktree path at `../<repo>-worktrees/<lane-id>` |
| `internal/worktree/worktree.go:247-253` | `Cleanup` checks path existence and delegates to `Remove` |
| `internal/worktree/worktree.go:256-261` | `Remove` invokes `git worktree remove --force` |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` evaluates `git status --porcelain` |
| `internal/worktree/worktree_test.go:255-266` | `TestRemove` asserting linked worktree removal |
| `internal/worktree/worktree_test.go:536-595` | `TestPorcelainEmpty` asserting dirty detection with ignored and untracked files |
| `internal/worktree/worktree_test.go:1034-1057` | `TestCleanupRemovesExistingWorktree` asserting cleanup of existing clean worktree |
| `internal/worktree/worktree_test.go:1059-1069` | `TestCleanupOnLaneWithNoWorktreeIsNoOp` asserting idempotent cleanup of non-existent worktree |
| `lucind-checks.sh:1-4` | Project build and full test suite verification script |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Acceptance 10-step sequence with mechanical checks and qualitative review checklist |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing guidance for advancing checkout and refreshing `base_sha` |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Troubleshooting dispatch and integration table |
