# Proposal Lens B — Capability Impact & Specs: Skill Anchoring & Worktree Cleanup Guardrails

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| Worktree cleanup dirty guardrail | Modified | Enforce fail-closed check via `PorcelainEmpty` before removing worktrees; require explicit `--force` (`-f`) to discard uncommitted changes. | `cmd/lucind-ai/cli.go:1934-1978`, `internal/worktree/worktree.go:247-253`, `internal/worktree/worktree.go:256-261`, `internal/worktree/worktree.go:319-325` |
| Blocked and timeout lane guidance banner | Added | Emit terminal banner directing operators to `troubleshooting.md` when a lane terminates in a non-done status. | `cmd/lucind-ai/cli.go:698-726`, `internal/run/run.go:452-465` |
| Integration revert recovery banner | Added | Emit recovery instructions directing operators to `recovery-reconciliation.md` and `integrate retry` when integration reports `reverted_ids`. | `cmd/lucind-ai/cli.go:730-740` |
| Acceptance receipt qualitative review banner | Added | Append qualitative review checklist reminders from `acceptance-promotion.md` upon mechanical receipt generation. | `cmd/lucind-ai/cli.go:685-690`, `internal/accept/accept.go:120-130` |
| Multi-wave DAG split warning banner | Added | Print warning banner on `lucind-ai split` reminding operators to advance checkout and refresh `base_sha` between waves. | `cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43` |
| Prescriptive TDD WIP-rescue protocol | Added | Document structured rescue protocol in `troubleshooting.md` for inspecting, committing partial TDD WIP, and re-dispatching timed-out lanes. | `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`, `.agents/skills/lucind-apply/SKILL.md:10-21` |

## Delta Specifications

### Requirement: Worktree Cleanup Dirty Guardrail and Force Flag

`worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) and `worktree.Remove` (`internal/worktree/worktree.go:256-261`) MUST verify `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) before deleting any linked worktree (`internal/worktree/worktree.go:150-159`). If uncommitted modifications exist, the operation MUST fail closed with `ErrWorktreeDirty` without deleting files, unless `force` is true. `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) MUST accept a `--force` (`-f`) flag; cleanup on dirty worktrees without `--force` MUST exit 1, display dirty status, and preserve the worktree. Clean or nonexistent worktrees MUST be cleaned up idempotently without requiring `--force`.

#### Scenario: Refuse cleanup on dirty worktree without force

- GIVEN a linked worktree at `pathFor` (`internal/worktree/worktree.go:150-159`) containing uncommitted changes
- WHEN the operator executes `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1934-1978`) without `--force`
- THEN the command MUST fail with exit code 1, preserve the worktree on disk, and print an error requiring `--force`.

#### Scenario: Force cleanup removes dirty worktree

- GIVEN a linked worktree containing uncommitted changes
- WHEN the operator executes `lucind-ai worktree cleanup --lane <id> --force` (`cmd/lucind-ai/cli.go:1934-1978`)
- THEN the command MUST delete the worktree via `worktree.Remove` (`internal/worktree/worktree.go:256-261`) and exit with code 0.

#### Scenario: Clean worktree cleanup succeeds idempotently

- GIVEN a clean linked worktree (`internal/worktree/worktree.go:319-325`) or nonexistent path
- WHEN the operator executes `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1934-1978`) without `--force`
- THEN `worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) MUST remove the worktree (if present) and exit with code 0.

### Requirement: Blocked and Timeout Lane Report Guidance Banner

When a lane finishes with status other than `lane.Done` (`internal/run/run.go:452-465`), `printReport` (`cmd/lucind-ai/cli.go:698-726`) MUST print a diagnostic banner. The banner MUST display the worktree path, instruct inspecting diffs before cleanup, and cite `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.

#### Scenario: Non-done lane displays troubleshooting banner

- GIVEN a lane dispatch terminating with status `blocked`, `failed`, or timeout (`internal/run/run.go:452-465`)
- WHEN `printReport` (`cmd/lucind-ai/cli.go:698-726`) formats the terminal summary
- THEN the output MUST include a guidance banner containing worktree path, diagnostic instructions (`git diff`), and a reference to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.

### Requirement: Integration Report Reverted IDs Recovery Banner

When an integration batch completes with lane identifiers in `reverted_ids`, `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) MUST append recovery instructions. The instructions MUST inform the operator that lane branches remain intact and direct them to run `lucind-ai integrate retry --run <run-id>` per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`.

#### Scenario: Reverted integration outcome surfaces retry instructions

- GIVEN an integration batch outcome where `reverted_ids` is non-empty
- WHEN `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) outputs the summary
- THEN the output MUST print recovery steps instructing the operator to run `lucind-ai integrate retry --run <run-id>` referencing `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`.

### Requirement: Acceptance Receipt Qualitative Review Banner

Upon mechanical verification and receipt persistence (`internal/accept/accept.go:120-130`), `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) MUST print a qualitative review banner. The banner MUST state that mechanical checks do not imply qualitative approval and remind operators to complete steps 2–10 from `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`.

#### Scenario: Mechanical acceptance output prints qualitative checklist reminder

- GIVEN a frozen candidate commit that passes mechanical acceptance checks
- WHEN `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) renders the receipt output
- THEN the output MUST state mechanical evidence passed and display a reminder to execute qualitative review steps 2–10 in `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` before promotion.

### Requirement: DAG Split Multi-Wave Base SHA Warning Banner

When `lucind-ai split` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) processes an `apply-dag.yaml` defining multiple waves, the command MUST output a multi-wave warning banner. The banner MUST instruct operators to advance primary checkout and refresh `base_sha` and `expected_parent_sha` in next-wave packets before dispatching, referencing `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`.

#### Scenario: Multi-wave DAG split emits base SHA warning

- GIVEN an apply DAG containing multiple sequential execution waves
- WHEN `runSplit` (`cmd/lucind-ai/cli.go:485-516`) executes `dag.Split` (`internal/dag/split.go:34-43`) to emit wave commands
- THEN the command output MUST include a warning banner instructing operators to advance checkout and refresh `base_sha`/`expected_parent_sha` between waves per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`.

### Requirement: Prescriptive TDD WIP-Rescue Protocol Documentation

`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` MUST document a standardized TDD WIP-rescue protocol for apply lanes executing under strict TDD (`.agents/skills/lucind-apply/SKILL.md:10-21`). The protocol MUST specify steps for operators to inspect the preserved worktree, commit partial RED/GREEN progress as `wip: partial lane progress`, adjust packet deadlines, and re-dispatch without destroying worktrees.

#### Scenario: Operator executes TDD WIP-rescue after lane timeout

- GIVEN an apply lane that times out during RED test authoring or partial GREEN implementation (`.agents/skills/lucind-apply/SKILL.md:10-21`) leaving uncommitted files
- WHEN the operator follows the TDD rescue protocol in `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`
- THEN the operator inspects uncommitted diffs in the preserved worktree (`internal/worktree/worktree.go:150-159`), creates a WIP commit, updates the packet timeout, and re-dispatches without data loss.

## Open Questions

- [ ] Should internal teardown call sites (`internal/integrate/integrate.go:121-124`, `internal/integrate/candidate.go:262-265`, `cmd/lucind-ai/cli.go:858-869`) pass `force: true` to `worktree.Cleanup`/`worktree.Remove` or utilize an explicit `RemoveForced` internal helper?
- [ ] Should CLI warning banners for `lucind-ai accept` and `lucind-ai split` be rendered to `stderr` exclusively or appended to `stdout` following the primary machine-readable records?

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Strict TDD lifecycle phases RED, GREEN, and SWEEP defining lane execution expectations |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` parses apply DAG, validates structure, and outputs wave commands to stdout |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` formats mechanical acceptance receipt output lines |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` renders lane execution summary and failure banners for non-done statuses |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` formats integration results including `integrated_ids` and `reverted_ids` |
| `cmd/lucind-ai/cli.go:858-869` | `DiscardCombined` and `RemoveLaneWorktree` dependencies wiring unconditional `worktree.Remove` |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` executes worktree cleanup command without dirty checks or force flag |
| `internal/accept/accept.go:120-130` | Mechanical acceptance verification stores immutable receipt in ledger |
| `internal/dag/split.go:34-43` | `dag.Split` iterates waves and prints copy-pasteable `lucind-ai run` commands |
| `internal/integrate/candidate.go:262-265` | Cleanup of worktree upon successful candidate promotion via `worktree.Remove` |
| `internal/integrate/integrate.go:121-124` | Cleanup of worktree upon merge conflict in `Combine` via `worktree.Remove` |
| `internal/run/run.go:452-465` | Lane timeout handling bounds dispatch execution and preserves worktree with blocked status |
| `internal/worktree/worktree.go:150-159` | `pathFor` computes linked worktree path alongside primary repository |
| `internal/worktree/worktree.go:247-253` | `Cleanup` checks worktree existence and delegates directly to `Remove` |
| `internal/worktree/worktree.go:256-261` | `Remove` unconditionally executes `git worktree remove --force` |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` checks whether worktree working directory is clean via git status |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step acceptance sequence defining mandatory qualitative review checklist |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave discipline requiring primary checkout advance and packet SHA refresh |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Feature revert recovery using `integrate retry` to rebuild batch from preserved worktrees |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Operational troubleshooting guidance for lane failures, reverted IDs, and timeouts |
