# Proposal: Skill Anchoring & Worktree Cleanup Guardrails

**Chosen candidate: Candidate 1 — Fail-Closed Worktree Cleanup Guardrails with Direct CLI Banner Anchoring and Prescriptive TDD WIP-Rescue Protocol** (`openspec/changes/skill-anchoring-guardrails/explore.md:24-32`).

## Intent

Lucind AI executes parallel lane dispatches using linked git worktrees (`internal/worktree/worktree.go:150-159`). Three operational vulnerabilities risk unrecoverable code loss, premature redispatches, and process errors:

1. **Unconditional Worktree Deletion**: `worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) delegates to `worktree.Remove` (`internal/worktree/worktree.go:256-261`), running `git worktree remove --force`. On timeouts (`cmd/lucind-ai/cli.go:37-44`, `internal/run/run.go:452-465`) or blocks, uncommitted code is destroyed when running `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:58,1934-1978`). While `internal/worktree/worktree.go:319-325` provides `PorcelainEmpty`, cleanup omits dirty checks.
2. **Output Detached from Authority Skills**: Authoritative knowledge lives in `plugin/claude-code/skills/lucind-ai/references/`, but CLI output omits references at critical gates (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-49`). Non-done lanes (`cmd/lucind-ai/cli.go:698-726`), reverted batches (`cmd/lucind-ai/cli.go:730-740`), mechanical receipts (`cmd/lucind-ai/cli.go:685-690`), or multi-wave splits (`cmd/lucind-ai/cli.go:485-516`) lack guidance, prompting unnecessary redispatches (`cmd/lucind-ai/cli.go:1997-2023`).
3. **Absence of Prescriptive TDD WIP-Rescue Protocol**: When an apply lane times out, partial RED tests or GREEN code are salvageable. Neither `troubleshooting.md` (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`) nor `lucind-apply` (`.agents/skills/lucind-apply/SKILL.md:10-21`) documents a protocol to inspect preserved worktrees, commit partial WIP, and re-dispatch with extended deadlines.

## Scope

### In Scope
- **Fail-Closed Cleanup**: Add `force bool` to `worktree.Cleanup`/`Remove` (`internal/worktree/worktree.go:247-261`) checking `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) and returning `worktree.ErrWorktreeDirty` if dirty. Add `--force` (`-f`) flag to `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`).
- **Internal Teardowns**: Pass `force: true` for machine-managed worktrees (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`).
- **CLI Guidance Banners**: Embed static skill references in `printReport` (`troubleshooting.md:7-18`), `printIntegrateReport` (`recovery-reconciliation.md:33-35`), `renderAcceptanceReceipt` (`acceptance-promotion.md:18-30`), and `runSplit` (`recovery-reconciliation.md:27-30`).
- **TDD WIP-Rescue Protocol**: Document rescue procedure in `troubleshooting.md` and `.agents/skills/lucind-apply/SKILL.md:10-21`.

### Out of Scope
- Daemon/cron garbage collection of stale worktrees.
- Schema changes to `.lucind/result.json` (`internal/result/result.go:26-34`).
- Automatic commits by binary on timeout/failure (`internal/run/run.go:906-940`).
- Modifying feature lease acquisition/fencing (`internal/feature/feature.go:348-350`).
- Altering acceptance ledger storage/hashing (`internal/accept/accept.go:120-130`, `internal/ledger/acceptance.go:41-47`).
- Re-architecting multi-agent orchestrator adapters outside `plugin/claude-code/skills/lucind-ai/`.

## Capabilities

### New Capabilities
- `worktree-dirty-guardrail`: Fail-closed `PorcelainEmpty` check before removal; require `--force` (`-f`) for dirty trees.
- `failure-guidance-banners`: Terminal banners linking failures, reverts, receipts, and splits to authority skills.
- `tdd-wip-rescue-protocol`: Standardized procedure in skill documentation to inspect dirty worktrees, commit partial WIP, and re-dispatch.

### Modified Capabilities
- `lane-worktree-lifecycle`: `worktree.Cleanup`/`Remove` signatures accept explicit `force bool`.
- `worktree-cleanup-cli`: `lucind-ai worktree cleanup` adds `--force` (`-f`) flag and displays porcelain diff output on dirty abort.

## Approach

1. **Sentinel Error & Signatures**: Export `worktree.ErrWorktreeDirty` in `internal/worktree/worktree.go`. Update `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) with `force bool`. When `force` is false, `Remove` checks `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`), returning `ErrWorktreeDirty` without deleting files. Nonexistent cleanup remains idempotent (`internal/worktree/worktree_test.go:1059-1069`).
2. **CLI Surface & Operator Flow**: Update `cmd/lucind-ai/cli.go:58,1934-1978` to parse `--force` (`-f`). When `worktree.Cleanup` returns `ErrWorktreeDirty` without `--force`, `runWorktreeCleanup` prints porcelain diffs, `git diff` inspection steps, references `troubleshooting.md`, prints `--force`, and exits 1. Clean removal succeeds (`internal/worktree/worktree_test.go:1034-1057`, `cmd/lucind-ai/cli_test.go:2974-3010`).
3. **Internal Automated Callers**: Pass `force: true` for disposable scratch trees: `DiscardCombined` (`cmd/lucind-ai/cli.go:858-863`), `RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:864-869`, `internal/run/integrate.go:159-165`), `Combine` abort (`internal/integrate/integrate.go:118-124`), and `ResolveCandidate` teardown (`internal/integrate/candidate.go:262-265`).
4. **CLI Guidance Banners**: Embed static banners at four milestones: `printReport` (`cmd/lucind-ai/cli.go:698-726`) on non-done status -> `troubleshooting.md:7-18`; `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) on `reverted_ids` -> `recovery-reconciliation.md:33-35` and `integrate retry`; `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) -> `acceptance-promotion.md:18-30`; `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) -> `recovery-reconciliation.md:27-30`.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/worktree/worktree.go:247-261` | Modified | `Cleanup` and `Remove` accept `force bool`, check `PorcelainEmpty`, return `ErrWorktreeDirty`. |
| `cmd/lucind-ai/cli.go:58,1934-1978` | Modified | `worktree cleanup` adds `--force` (`-f`), fails closed on dirty worktrees. |
| `cmd/lucind-ai/cli.go:858-869` | Modified | `DiscardCombined` and `RemoveLaneWorktree` pass `force: true`. |
| `internal/integrate/integrate.go:118-124` | Modified | Merge conflict handler passes `force: true` to `worktree.Remove`. |
| `internal/integrate/candidate.go:262-265` | Modified | `ResolveCandidate` passes `force: true` to `worktree.Remove`. |
| `cmd/lucind-ai/cli.go:485-740` | Modified | Guidance banners in `printReport`, `printIntegrateReport`, `renderAcceptanceReceipt`, `runSplit`. |
| `plugin/claude-code/skills/lucind-ai/` | Modified | Document Hard Gates, recovery banners, and TDD WIP-rescue in skill references. |
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Modified | Reference TDD WIP-rescue protocol for apply lane timeouts. |

## User and Capability Impact

| Who / Component | Impact |
|---|---|
| Operators driving CLI | Prevents accidental deletion of uncommitted work; provides diagnostic commands and exact skill references. |
| Apply Lane Executors | Partial progress preserved on timeout; documented WIP-rescue protocol avoids lost work. |
| Integration & Recovery | Operators recover reverted batches via `lucind-ai integrate retry --run <run-id>` with zero redundant AI tokens. |
| Downstream Parsers | Banners routed to `stderr` or appended after records preserve stdout compatibility. |

## Delta Specifications

### Requirement: Worktree cleanup dirty guardrail and force flag
`worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) and `worktree.Remove` (`internal/worktree/worktree.go:256-261`) MUST check `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) before deleting any linked worktree (`internal/worktree/worktree.go:150-159`), failing closed with `ErrWorktreeDirty` unless `force: true`. `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) MUST accept `--force` (`-f`); unforced cleanup on dirty worktrees MUST exit 1, display status, and preserve files. Clean or nonexistent worktrees MUST be removed idempotently.

#### Scenario: Refuse cleanup on dirty worktree without force
- GIVEN a dirty linked worktree at `pathFor` (`internal/worktree/worktree.go:150-159`)
- WHEN running `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1934-1978`) without `--force`
- THEN the command MUST exit 1, preserve the worktree on disk, and require `--force`

#### Scenario: Force cleanup removes dirty worktree
- GIVEN a dirty linked worktree
- WHEN running `lucind-ai worktree cleanup --lane <id> --force` (`cmd/lucind-ai/cli.go:1934-1978`)
- THEN the command MUST delete the worktree via `worktree.Remove` (`internal/worktree/worktree.go:256-261`) and exit 0

#### Scenario: Clean worktree cleanup succeeds idempotently
- GIVEN a clean linked worktree (`internal/worktree/worktree.go:319-325`) or nonexistent path
- WHEN running `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1934-1978`) without `--force`
- THEN `worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) MUST remove the worktree (if present) and exit 0

### Requirement: Blocked and timeout lane report guidance banner
On non-done lane status (`internal/run/run.go:452-465`), `printReport` (`cmd/lucind-ai/cli.go:698-726`) MUST print a banner with worktree path, diff inspection steps, and a link to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.

#### Scenario: Non-done lane displays troubleshooting banner
- GIVEN a lane terminating with status `blocked`, `failed`, or timeout (`internal/run/run.go:452-465`)
- WHEN `printReport` (`cmd/lucind-ai/cli.go:698-726`) formats summary
- THEN output MUST include worktree path, diagnostic steps, and reference to `troubleshooting.md:7-18`

### Requirement: Integration report reverted IDs recovery banner
On non-empty `reverted_ids`, `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) MUST print recovery instructions for `lucind-ai integrate retry --run <run-id>` referencing `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`.

#### Scenario: Reverted integration outcome surfaces retry instructions
- GIVEN an integration batch outcome with non-empty `reverted_ids`
- WHEN `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) outputs summary
- THEN output MUST print `integrate retry` instructions referencing `recovery-reconciliation.md:33-35`

### Requirement: Acceptance receipt qualitative review banner
Upon mechanical verification (`internal/accept/accept.go:120-130`), `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) MUST print a reminder to complete qualitative checklist review steps 2–10 from `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`.

#### Scenario: Mechanical acceptance output prints qualitative checklist reminder
- GIVEN a candidate commit passing mechanical checks
- WHEN `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) renders output
- THEN output MUST state mechanical evidence passed and remind operators of qualitative steps 2–10 in `acceptance-promotion.md:18-30`

### Requirement: DAG split multi-wave base SHA warning banner
When `lucind-ai split` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) processes multi-wave DAGs, it MUST output a warning banner to advance checkout and refresh `base_sha`/`expected_parent_sha` per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`.

#### Scenario: Multi-wave DAG split emits base SHA warning
- GIVEN an apply DAG defining sequential waves
- WHEN `runSplit` (`cmd/lucind-ai/cli.go:485-516`) executes `dag.Split` (`internal/dag/split.go:34-43`)
- THEN output MUST include a warning banner to refresh SHAs between waves per `recovery-reconciliation.md:27-30`

### Requirement: Prescriptive TDD WIP-rescue protocol documentation
`troubleshooting.md` (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`) MUST document a TDD WIP-rescue protocol for apply lanes (`.agents/skills/lucind-apply/SKILL.md:10-21`) to inspect preserved worktrees, commit partial WIP, adjust deadlines, and re-dispatch without data loss.

#### Scenario: Operator executes TDD WIP-rescue after lane timeout
- GIVEN an apply lane timing out with uncommitted files (`.agents/skills/lucind-apply/SKILL.md:10-21`)
- WHEN following the rescue protocol in `troubleshooting.md:7-18`
- THEN the operator inspects uncommitted diffs in the preserved worktree (`internal/worktree/worktree.go:150-159`), creates a WIP commit, updates timeout, and re-dispatches

## Technical Risks and Failure Modes

| Risk | Impact | Mitigation | Seam |
|---|---|---|---|
| Breaking internal callers on worktree removal | Promotion, conflict, or combined tree teardowns fail if dirty checks block. | Add `force bool` to `worktree.Cleanup`/`Remove` (`internal/worktree/worktree.go:247-261`). Internal callers pass `force: true`; CLI defaults to `force: false`. | `cmd/lucind-ai/cli.go:858-869,1934-1978`, `internal/integrate/candidate.go:262-265`, `internal/integrate/integrate.go:118-124`, `internal/run/integrate.go:159-165` |
| Banners breaking stdout parsers | Downstream scripts parsing stdout fail if banners pollute stdout. | Route banners to `stderr` (`cmd/lucind-ai/cli.go:485-516,1934-1978`) or append after single-line records (`cmd/lucind-ai/cli.go:685-690,730-750`, `internal/dag/split.go:34-43`). | `cmd/lucind-ai/cli.go:485-516,685-690,730-750`, `cmd/lucind-ai/cli_test.go:729-777,4503-4545`, `internal/dag/split.go:34-43`, `internal/dag/split_test.go:13-111` |
| Dirty false positives on ignored files | Cleanup falsely blocks on ignored runtime files like `.lucind/result.json`. | Rely directly on `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`), respecting `.gitignore` rules (`internal/worktree/worktree_test.go:536-595`). | `internal/worktree/worktree.go:319-325`, `internal/worktree/worktree_test.go:536-595` |
| Accidental `--force` destroys WIP | Operators delete uncommitted code after timeouts. | CLI cleanup fails closed with exit 1, outputs porcelain dirty list, diagnostic commands (`git diff`), and rescue steps referencing `troubleshooting.md:7-18` before explaining `--force`. | `cmd/lucind-ai/cli.go:698-726,1934-1978`, `.agents/skills/lucind-apply/SKILL.md:10-21`, `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` |
| Multi-wave `base_sha` staleness | Subsequent waves overwrite rather than accumulate prior commits. | Emit multi-wave banner in `cmd/lucind-ai/cli.go:485-516` (to stderr) to advance checkout and refresh `base_sha`/`expected_parent_sha` (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`). | `cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` |

## Rollback Plan and Additivity

**Rollback Plan**: Single `git revert` of merge commit. Introduces zero database migrations, ledger alterations (`internal/ledger/acceptance.go:41-47`), or result schema changes (`internal/result/result.go:26-34`). Reverting restores prior cleanup and removes banners with zero data migration.

**Additivity**: Purely additive:
- **Go API & CLI**: Adds `force bool` to `worktree.Cleanup`/`Remove` (`internal/worktree/worktree.go:247-261`) and optional `--force` (`-f`) flag to `worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`). Default behavior is fail-closed on dirty worktrees.
- **Output Streams**: Banners are added to `stderr` or appended after existing single-line records (`cmd/lucind-ai/cli.go:485-516,685-690,698-726,730-750`). Structured stdout records (`integrated_ids:`, `reverted_ids:`, `acceptance receipt:`) remain unchanged.
- **Ledgers & Schemas**: Zero modifications to SQLite ledger tables (`internal/ledger/acceptance.go:41-47`), feature leases (`internal/feature/feature.go:348-350`), or result schema (`internal/result/result.go:26-34`).

## Test and Validation Impact

| Layer | Coverage / Required Assertions | Existing seam (file:line) |
|---|---|---|
| Worktree Unit Tests (`internal/worktree`) | Update `TestCleanupRemovesExistingWorktree` (`internal/worktree/worktree_test.go:1034-1057`), `TestCleanupOnLaneWithNoWorktreeIsNoOp` (`internal/worktree/worktree_test.go:1059-1069`), and `TestRemove` (`internal/worktree/worktree_test.go:255-266`) for new signature. Assert clean cleanup succeeds, dirty without force returns `ErrWorktreeDirty` preserving files, dirty with `force: true` removes worktree, and `Remove` enforces identical rules. | `internal/worktree/worktree.go:247-261`, `internal/worktree/worktree_test.go:255-266,536-595,1034-1069` |
| CLI Cleanup Tests (`cmd/lucind-ai`) | Update `TestWorktreeCleanupCLI` (`cmd/lucind-ai/cli_test.go:2974-3010`) to verify: clean cleanup succeeds without `--force`; dirty cleanup without `--force` fails with exit 1, preserves worktree, and prints dirty status/guidance; dirty cleanup with `--force`/`-f` succeeds with exit 0; nonexistent lane cleanup is idempotent. | `cmd/lucind-ai/cli.go:1934-1978`, `cmd/lucind-ai/cli_test.go:2974-3010` |
| Banner Tests (`cmd/lucind-ai`, `internal/dag`) | `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (`cmd/lucind-ai/cli_test.go:4503-4545`) for receipt stdout and checklist reminder; `TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured` (`cmd/lucind-ai/cli_test.go:685-724`) for non-done banner referencing `troubleshooting.md:7-18`; `TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs` (`cmd/lucind-ai/cli_test.go:729-777`) for retry instructions on reverted batches; `TestSplit_TwoWaveDAGSuccess` (`internal/dag/split_test.go:13-111`) for parseable stdout wave commands. | `cmd/lucind-ai/cli.go:485-516,685-690,698-750`, `cmd/lucind-ai/cli_test.go:685-777,4503-4545`, `internal/dag/split.go:34-43`, `internal/dag/split_test.go:13-111` |
| Internal Integration | Ensure internal callers pass `force: true` so internal lifecycles (conflict recovery, promotion teardowns, combined tree discards) continue to pass cleanly across full repository check (`lucind-checks.sh:1-4`). | `cmd/lucind-ai/cli.go:858-869`, `internal/integrate/candidate.go:262-265`, `internal/integrate/integrate.go:118-124`, `internal/run/integrate.go:159-165`, `lucind-checks.sh:1-4` |

## Open Questions

1. Should CLI warning banners for `lucind-ai accept` and `lucind-ai split` route to `stderr` exclusively or append to `stdout` after records?
2. Should internal teardowns (`internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `cmd/lucind-ai/cli.go:858-869`) pass `force: true` or use an explicit `RemoveForced` helper?
