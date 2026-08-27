# Proposal Lens C — Risks, Rollback & Test Impact: Skill Anchoring & Worktree Cleanup Guardrails

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| Breaking internal callers on unconditional worktree removal | Automated internal teardowns during candidate promotion, merge conflicts, or combined tree discards fail or leave stale worktrees if dirty checks block without bypass. | Add an explicit `force bool` parameter to `worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) and `worktree.Remove` (`internal/worktree/worktree.go:256-261`). Internal lifecycle callers (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`) pass `force: true` since combined/promotion trees are machine-managed artifacts, while operator CLI `worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) defaults to `force: false`. | `cmd/lucind-ai/cli.go:858-869`, `cmd/lucind-ai/cli.go:1934-1978`, `internal/integrate/candidate.go:262-265`, `internal/integrate/integrate.go:118-124`, `internal/run/integrate.go:159-165`, `internal/worktree/worktree.go:247-261` |
| Guidance banners breaking downstream stdout stream parsers | Downstream CI scripts, test harnesses, or tools parsing stdout (e.g. DAG split wave commands or acceptance receipts) fail if banners pollute stdout streams. | Route operational guidance banners to `stderr` (e.g. multi-wave guidance in `cmd/lucind-ai/cli.go:485-516`, cleanup failure guidance in `cmd/lucind-ai/cli.go:1934-1978`) or strictly append banners after single-line machine records (`cmd/lucind-ai/cli.go:685-690`, `cmd/lucind-ai/cli.go:730-750`, `internal/dag/split.go:34-43`) without modifying record formats. | `cmd/lucind-ai/cli.go:485-516`, `cmd/lucind-ai/cli.go:685-690`, `cmd/lucind-ai/cli.go:730-740`, `cmd/lucind-ai/cli.go:742-750`, `cmd/lucind-ai/cli_test.go:729-777`, `cmd/lucind-ai/cli_test.go:4503-4545`, `internal/dag/split.go:34-43`, `internal/dag/split_test.go:13-111` |
| Dirty-detection false positives or false negatives (`PorcelainEmpty`) | Cleanup falsely blocks on ignored runtime files like `.lucind/result.json` (false positive) or destroys uncommitted tracked/untracked code (false negative). | Rely directly on `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`), which delegates to `git status --porcelain`, respecting repository `.gitignore` configurations while capturing untracked and modified working-tree files (`internal/worktree/worktree_test.go:536-595`). | `internal/worktree/worktree.go:319-325`, `internal/worktree/worktree_test.go:536-595` |
| Muscle-memory `--force` execution destroying salvageable WIP | Operators or automated scripts accustomed to running `--force` blindly delete uncommitted RED tests or partial GREEN implementations after a lane timeout or block. | CLI `worktree cleanup` fails closed on dirty worktrees with exit code 1, outputs `git status --porcelain` dirty file list, displays diagnostic inspection commands (`git diff`), and outputs prescriptive WIP-rescue steps referencing `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` before explaining `--force`. | `cmd/lucind-ai/cli.go:698-726`, `cmd/lucind-ai/cli.go:1934-1978`, `.agents/skills/lucind-apply/SKILL.md:10-21`, `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` |
| Multi-wave `base_sha` staleness across sequential waves | Multi-wave DAG splits emit static packets with frozen `base_sha`. Executing subsequent waves without advancing checkout and refreshing `base_sha` causes later waves to overwrite rather than accumulate prior wave commits. | Emit an explicit multi-wave invariant banner in `cmd/lucind-ai/cli.go:485-516` (to stderr) reminding operators to advance primary checkout, refresh `base_sha` and `expected_parent_sha`, and verify prior wave tree content before next wave dispatch (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`). | `cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` |
| Uncommitted TDD WIP lost on lane timeout | Timed-out apply lanes leaving uncommitted RED unit tests or partial GREEN logic lose progress if cleaned up without inspection. | Non-done lane reports in `cmd/lucind-ai/cli.go:698-726` output a prominent recovery banner referencing `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21`, instructing operators to inspect preserved worktrees (`internal/worktree/worktree.go:150-159`), commit WIP, and redispatch. | `cmd/lucind-ai/cli.go:698-726`, `internal/run/run.go:452-465`, `internal/worktree/worktree.go:150-159`, `.agents/skills/lucind-apply/SKILL.md:10-21`, `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` |

## Rollback & Additivity

**Rollback Plan**: Single git revert of the merge commit. The change introduces zero persistent database schema migrations, ledger table alterations (`internal/ledger/acceptance.go:41-47`), or result envelope schema changes (`internal/result/result.go:26-34`). Reverting the code restores prior unconditional worktree cleanup behavior and removes CLI guidance banners with zero state reconciliation or data migration required.

**Additivity**: Purely additive across all components:
- **Go API & CLI Interfaces**: Adds an explicit `force bool` parameter to `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) and an optional `--force` (`-f`) flag to CLI `worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`). Default behavior is fail-closed on dirty worktrees.
- **Output Streams**: Informational guidance banners are added to `stderr` or appended non-destructively after existing single-line records (`cmd/lucind-ai/cli.go:485-516`, `cmd/lucind-ai/cli.go:685-690`, `cmd/lucind-ai/cli.go:698-726`, `cmd/lucind-ai/cli.go:730-750`). Structured stdout record formats (`integrated_ids:`, `reverted_ids:`, `acceptance receipt:`) remain unchanged.
- **Ledgers & Schemas**: Zero modifications to SQLite ledger tables (`internal/ledger/acceptance.go:41-47`), feature lease management (`internal/feature/feature.go:348-350`), or `.lucind/result.json` schema (`internal/result/result.go:26-34`).

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| Worktree Unit Tests (`internal/worktree`) | Update `TestCleanupRemovesExistingWorktree` (`internal/worktree/worktree_test.go:1034-1057`), `TestCleanupOnLaneWithNoWorktreeIsNoOp` (`internal/worktree/worktree_test.go:1059-1069`), and `TestRemove` (`internal/worktree/worktree_test.go:255-266`) for new signature. Add unit tests asserting: (a) `Cleanup` on clean worktree without force succeeds; (b) `Cleanup` on dirty worktree without force returns `ErrWorktreeDirty` and preserves files; (c) `Cleanup` on dirty worktree with `force: true` removes worktree; (d) `Remove` on dirty worktree without force returns `ErrWorktreeDirty`; (e) `Remove` with `force: true` removes worktree. | `internal/worktree/worktree.go:247-261`, `internal/worktree/worktree_test.go:255-266`, `internal/worktree/worktree_test.go:536-595`, `internal/worktree/worktree_test.go:1034-1057`, `internal/worktree/worktree_test.go:1059-1069` |
| CLI Worktree Cleanup Tests (`cmd/lucind-ai`) | Update `TestWorktreeCleanupCLI` (`cmd/lucind-ai/cli_test.go:2974-3020`) to verify: (a) clean worktree cleanup succeeds without `--force`; (b) dirty worktree cleanup without `--force` fails with exit code 1, preserves worktree, and prints dirty status and recovery guidance; (c) dirty worktree cleanup with `--force` / `-f` succeeds with exit code 0; (d) nonexistent lane cleanup remains idempotent (exit 0). | `cmd/lucind-ai/cli.go:1934-1978`, `cmd/lucind-ai/cli_test.go:2974-3020` |
| Output Parsing & Banner Tests (`cmd/lucind-ai`, `internal/dag`) | Verify guidance banners do not break machine parsers across existing test suites: (a) `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (`cmd/lucind-ai/cli_test.go:4503-4545`) asserting receipt stdout records and qualitative checklist reminder; (b) `TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured` (`cmd/lucind-ai/cli_test.go:685-724`) asserting non-done recovery banner referencing `troubleshooting.md:7-18`; (c) `TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs` (`cmd/lucind-ai/cli_test.go:729-777`) asserting `integrate retry` instructions on non-empty `reverted_ids`; (d) `TestSplit_TwoWaveDAGSuccess` (`internal/dag/split_test.go:13-111`) proving stdout wave commands remain parseable while multi-wave banner routes to stderr. | `cmd/lucind-ai/cli.go:485-516`, `cmd/lucind-ai/cli.go:685-690`, `cmd/lucind-ai/cli.go:698-726`, `cmd/lucind-ai/cli.go:730-750`, `cmd/lucind-ai/cli_test.go:685-724`, `cmd/lucind-ai/cli_test.go:729-777`, `cmd/lucind-ai/cli_test.go:4503-4545`, `internal/dag/split.go:34-43`, `internal/dag/split_test.go:13-111` |
| Internal Lifecycle Integration | Ensure automated internal callers pass `force: true` so that internal worktree lifecycles (conflict recovery, promotion teardowns, combined tree discards) continue to succeed cleanly without regression across full repository check (`lucind-checks.sh:1-4`). | `cmd/lucind-ai/cli.go:858-869`, `internal/integrate/candidate.go:262-265`, `internal/integrate/integrate.go:118-124`, `internal/run/integrate.go:159-165`, `lucind-checks.sh:1-4` |

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
| `internal/worktree/worktree.go:150-159` | `pathFor` computes linked worktree path at `../<repo>-worktrees/<lane-id>` |
| `internal/worktree/worktree.go:247-253` | `Cleanup` checks path existence and delegates to `Remove` |
| `internal/worktree/worktree.go:256-261` | `Remove` invokes `git worktree remove --force` |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` evaluates `git status --porcelain` |
| `internal/worktree/worktree_test.go:255-266` | `TestRemove` asserting linked worktree removal |
| `internal/worktree/worktree_test.go:536-595` | `TestPorcelainEmpty` asserting dirty detection with ignored and untracked files |
| `internal/worktree/worktree_test.go:1034-1057` | `TestCleanupRemovesExistingWorktree` asserting cleanup of existing clean worktree |
| `internal/worktree/worktree_test.go:1059-1069` | `TestCleanupOnLaneWithNoWorktreeIsNoOp` asserting idempotent cleanup of non-existent worktree |
| `lucind-checks.sh:1-4` | Project build and full test suite verification script |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing guidance for advancing checkout and refreshing `base_sha` |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Troubleshooting dispatch and integration table |
