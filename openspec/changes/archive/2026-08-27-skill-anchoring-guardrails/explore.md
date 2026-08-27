# Explore: Skill Anchoring & Worktree Cleanup Guardrails

This exploration addresses three critical operational gaps in `lucind-ai`: unconditional worktree destruction during cleanup, missing prescriptive CLI guidance referencing authority skills during failures/reverts, and the absence of a standardized TDD WIP-rescue protocol.

## Problem Statement & Background

Lucind AI manages parallel, isolated lane dispatches using linked git worktrees located at `../<basename>-worktrees/<laneID>` (`internal/worktree/worktree.go:150-159`). However, three operational vulnerabilities lead operators and autonomous agents to act on intuition or reflex during failures, risking unrecoverable data loss and wasted tokens:

1. **Unconditional Worktree Deletion Without Dirty Guardrails**:
   `worktree.Cleanup` (`internal/worktree/worktree.go:247-254`) delegates directly to `worktree.Remove` (`internal/worktree/worktree.go:256-261`), which unconditionally invokes `git worktree remove --force`. If an agent times out (`cmd/lucind-ai/cli.go:37-44`, `internal/run/run.go:452-465`) or blocks mid-implementation, uncommitted files in the worktree are permanently destroyed when `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:58`, `cmd/lucind-ai/cli.go:1934-1978`) is executed to reset state. While `internal/worktree/worktree.go:319-325` provides `PorcelainEmpty` (`git status --porcelain`), cleanup fails to verify dirty status before removal. Existing tests only assert deletion of clean worktrees (`internal/worktree/worktree_test.go:1034-1057`, `cmd/lucind-ai/cli_test.go:2989-3004`).

2. **Terminal Output Detached from Authority Skills**:
   Operational knowledge is documented under `plugin/claude-code/skills/lucind-ai/references/`, but CLI commands fail to surface these references at critical failure points:
   - Non-done lane reports (`cmd/lucind-ai/cli.go:698-726`) warn that the worktree is preserved but provide no prescriptive TDD rescue steps or pointers to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.
   - Integration reports listing `reverted_ids` (`cmd/lucind-ai/cli.go:730-740`) omit instructions directing operators to `lucind-ai integrate retry --run <run-id>` (`cmd/lucind-ai/cli.go:1997-2018`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`), prompting expensive and unnecessary AI redispatches.
   - Acceptance receipts (`cmd/lucind-ai/cli.go:636-690`, `cmd/lucind-ai/cli_test.go:4515-4530`) output mechanical verification details (`internal/accept/accept.go:120-130`) without reminding operators to complete mandatory qualitative review steps 2–10 (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`).
   - DAG splitting (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) emits wave commands without warning operators to refresh `base_sha` and `expected_parent_sha` between sequential waves (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`).

3. **Absence of a Prescriptive TDD WIP-Rescue Protocol**:
   When an apply lane times out, newly written RED tests or partial GREEN implementations are often salvageable. Neither `troubleshooting.md` (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`) nor `lucind-apply` (`.agents/skills/lucind-apply/SKILL.md:10-21`) specifies a concrete protocol to inspect, commit WIP progress, and re-dispatch with an extended deadline before cleanups occur.

## Candidate Approaches

| Approach | Pros | Cons | Feasibility |
|---|---|---|---|
| **1. Fail-Closed Guardrails with Direct CLI Banner Anchoring** (Recommended) | Deterministic prevention of data loss; leverages existing `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`); zero breaking changes to DAG or ledger; anchors skills in terminal outputs at failure gates. | Requires updating `Cleanup`/`Remove` call sites across internal callers and unit tests; slightly increases banner verbosity. | **High** |
| **2. Automatic Stash/WIP Commit with Prompt Context Injection** | Guarantees zero data loss without manual operator commit commands. | Pollutes git ref namespace; synthetic commits break clean-tree guarantees (`internal/run/run.go:906-940`); dynamic prompt injection inflates token usage and couples runners to skill paths. | **Low / Unviable** |
| **3. Interactive TTY Prompting with Ledger Quarantine State** | Explicit interactive experience for human terminal operators. | Breaks headless, non-interactive batch dispatches (`cmd/lucind-ai/cli.go:37-44`); increases ledger schema complexity for a local filesystem check. | **Medium / Premature** |

### Recommendation
Adopt **Candidate 1**. It enforces fail-closed safety by requiring `--force` (`-f`) on `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) when `PorcelainEmpty` detects uncommitted files. Static CLI banners in `printReport`, `printIntegrateReport`, `renderAcceptanceReceipt`, and `runSplit` (`cmd/lucind-ai/cli.go:485-740`) anchor operators to exact authority skills (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-49`) without breaking headless automation.

## User & Capability Impact

- **Human Operators & Orchestrators**: Accidental deletion of uncommitted work is prevented. When a lane fails, reverts, or passes mechanical acceptance, the CLI prints immediate diagnostic commands (`git diff`) and exact skill references (`references/operations/troubleshooting.md`, `references/coordination/recovery-reconciliation.md`, `references/contracts/acceptance-promotion.md`).
- **Lane Executors (Apply Lanes)**: Authors working under strict TDD (`.agents/skills/lucind-apply/SKILL.md:10-21`) have a defined WIP-rescue protocol if a lane times out (`internal/run/run.go:452-465`), preserving valuable test logic.
- **Integration & Recovery**: Reverted features are recovered via `lucind-ai integrate retry --run <run-id>` (`cmd/lucind-ai/cli.go:1997-2018`) with zero additional LLM token expense.

## Scenarios & Use Cases

1. **Blocked Cleanup on Dirty Worktree**: A lane times out during test implementation, leaving uncommitted files. Running `lucind-ai worktree cleanup --lane <id>` without `--force` fails with exit code 1, preserves the worktree intact, displays uncommitted files via `git status --porcelain`, and outputs the `--force` requirement (`cmd/lucind-ai/cli.go:1934-1978`).
2. **Intentional Force Cleanup**: An operator reviews dirty uncommitted artifacts, determines they are disposable, and executes `lucind-ai worktree cleanup --lane <id> --force`. The worktree is cleanly removed (`internal/worktree/worktree.go:256-261`), exiting 0 (`internal/worktree/worktree_test.go:1034-1057`).
3. **TDD RED/GREEN Progress Rescue on Timeout**: A lane executor completes RED unit tests but times out before finishing GREEN implementation (`.agents/skills/lucind-apply/SKILL.md:10-21`, `cmd/lucind-ai/cli.go:698-726`). `lucind-ai run` emits a recovery banner referencing `troubleshooting.md`. The orchestrator commits the progress as `wip: partial lane progress` and re-dispatches with an extended timeout.
4. **Feature Revert Recovery via Integrate Retry**: An integration batch reverts because the base commit was red. `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) lists `reverted_ids` and prints instructions directing the operator to fix the base and run `lucind-ai integrate retry --run <run-id>` (`cmd/lucind-ai/cli.go:1997-2018`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`).
5. **Qualitative Acceptance Checklist Enforcement**: An operator runs `lucind-ai accept` (`cmd/lucind-ai/cli.go:636-690`). The CLI outputs the mechanical receipt (`internal/accept/accept.go:120-130`) along with a banner detailing qualitative review steps 2–10 from `acceptance-promotion.md` (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`).
6. **Multi-Wave DAG Packet Generation**: An operator runs `lucind-ai split` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`). The command emits packet files and outputs a reminder to advance the primary checkout and refresh `base_sha` and `expected_parent_sha` between waves (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`).

## Technical Risks & Trade-Offs Matrix

### Risks & Mitigations

| Risk / Unknown | Severity | Mitigation Strategy | Seam |
|---|---|---|---|
| Breaking internal callers expecting unconditional removal | High | Maintain forced deletion for automated internal teardowns (`DiscardCombined`, `RemoveLaneWorktree`, merge conflicts, and promotion cleanup) via explicit `force: true` or internal helper. Gate user-facing cleanup behind dirty checks. | `internal/worktree/worktree.go:247-261`, `internal/integrate/integrate.go:121-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`, `cmd/lucind-ai/cli.go:858-869`, `cmd/lucind-ai/cli.go:1934-1978` |
| Guidance banners breaking downstream stdout parsers | Medium | Route guidance banners to `stderr` or append after machine-parseable single-line output records (`acceptance receipt:`, `integrated_ids:`, `reverted_ids:`). | `cmd/lucind-ai/cli.go:58-59,485-516,685-690,730-750`, `internal/dag/split.go:34-43`, `cmd/lucind-ai/cli_test.go:4515-4530` |
| False positives on ignored files during dirty check | Medium | Rely directly on `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`), which respects `.gitignore` rules (e.g. `.lucind/result.json` remains clean). | `internal/worktree/worktree.go:319-325`, `internal/worktree/worktree_test.go:536-595` |
| Muscle-memory `--force` execution destroying salvageable WIP | High | Accompany cleanup failure with immediate diagnostic commands (`git diff`), prescriptive WIP commit steps, and troubleshooting skill pointers before mentioning `--force`. | `cmd/lucind-ai/cli.go:698-726,1934-1978`, `plugin/claude-code/skills/lucind-ai/SKILL.md:29-49`, `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` |
| Multi-wave `base_sha` staleness across sequential waves | High | Emit a multi-wave invariant banner on `lucind-ai split` reminding operators to advance primary checkout and refresh `base_sha` before next wave dispatch. | `cmd/lucind-ai/cli.go:485-516`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:25-30` |

### Trade-Offs

| Choice | Advantages | Disadvantages | Cost |
|---|---|---|---|
| **`force bool` in `worktree.Cleanup`** vs separate `CleanupForced` | Explicit boolean forces compile-time auditing of every call site across packages; zero accidental un-guarded removals. | Requires touching call sites across unit/integration tests. | Low |
| **Banners to `stderr`** vs structured `stdout` | Guarantees zero breakage for scripts capturing stdout (e.g. DAG wave commands, receipt parsing). | Banners omitted if calling harness redirects or suppresses `stderr`. | Low |
| **Static CLI Banners** vs dynamic file rendering | Zero runtime overhead, zero disk lookup failure risk, works deterministically in headless/isolated runs. | Updating skill path references requires binary rebuild. | Very Low |
| **Prescriptive WIP Protocol** vs automatic git commit | Keeps operator/orchestrator in control of commit history, avoiding automated commits of broken code. | Requires operator intervention to commit WIP and adjust timeout before redispatch. | Low |

## Potential Spikes & Proof of Concepts

1. **Worktree Dirty Guardrail & `--force` Override**: Implement dirty check in `worktree.Cleanup` using `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`). Verify dirty worktrees return `ErrWorktreeDirty` and remain on disk, while `--force` removes them cleanly (`cmd/lucind-ai/cli.go:1934-1978`, `internal/worktree/worktree_test.go:1034-1069`, `cmd/lucind-ai/cli_test.go:2974-3010`).
2. **Output Stream Isolation for Banners**: Implement qualitative checklist reminder in `runAccept` / `renderAcceptanceReceipt` and multi-wave warning banner in `runSplit`. Execute automated parsing tests (`cmd/lucind-ai/cli_test.go:4515-4530`, `internal/dag/split.go:34-43`) to prove machine-readable stdout remains intact.
3. **TDD WIP-Rescue Protocol Simulation**: Simulate a lane timeout during RED test authoring. Verify that the worktree at `pathFor` (`internal/worktree/worktree.go:150-159`) is preserved, `printReport` outputs the recovery banner, manual WIP commit succeeds, and subsequent redispatch builds on the saved work (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`).

## Success Criteria

- `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) return `ErrWorktreeDirty` without removing files when the worktree contains uncommitted tracked or untracked changes (`internal/worktree/worktree.go:319-325`), unless `force=true`.
- Idempotent cleanup on nonexistent worktrees continues to return `nil` (`internal/worktree/worktree_test.go:1059-1069`).
- `lucind-ai worktree cleanup --lane <id>` exits 1 with actionable guidance when dirty and succeeds when `--force` (`-f`) is passed (`cmd/lucind-ai/cli.go:1934-1978`).
- `printReport` (`cmd/lucind-ai/cli.go:698-726`) prints a recovery banner referencing `references/operations/troubleshooting.md` when lane status is not `done`.
- `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) emits `integrate retry` instructions referencing `references/coordination/recovery-reconciliation.md` when `reverted_ids` is non-empty.
- `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) outputs qualitative review checklist reminders from `references/contracts/acceptance-promotion.md`.
- `runSplit` (`cmd/lucind-ai/cli.go:485-516`) prints multi-wave `base_sha` maintenance guidance.
- Skill documentation (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-49`, `references/operations/troubleshooting.md:7-18`, `.agents/skills/lucind-apply/SKILL.md:10-21`) documents Pre-Action Hard Gates and the TDD WIP-rescue protocol.

## Out of Scope & Open Questions

### Out of Scope
- Background daemon or automatic cron-based garbage collection of stale worktrees.
- Schema modifications to `.lucind/result.json` or `result.schema.json` (`internal/result/result.go:10-45`).
- Automatic git commits created by the binary upon lane timeout or failure (`internal/run/run.go:906-940`).
- Modifying feature lease acquisition, fencing tokens, or lease renewal in `internal/feature/feature.go:300-350`.
- Altering core acceptance ledger storage or receipt hashing (`internal/accept/accept.go:120-130`, `internal/ledger/acceptance.go:40-60`).
- Re-architecting multi-agent orchestrator adapters outside `plugin/claude-code/skills/lucind-ai/`.

### Open Questions
- Should internal teardown callers (`internal/integrate/integrate.go:121-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`, `cmd/lucind-ai/cli.go:858-869`) pass `force: true` to `Cleanup`/`Remove` or utilize an explicit `RemoveForced` helper?
- Should CLI warning banners for `lucind-ai accept` and `lucind-ai split` be rendered to `stderr` exclusively or appended to `stdout` following the primary output record?
