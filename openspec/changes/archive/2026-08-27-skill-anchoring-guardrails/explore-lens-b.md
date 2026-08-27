# Explore Lens B — Capabilities & Scenarios: Skill Anchoring & Worktree Cleanup Guardrails

## User & Capability Impact

This change affects human operators driving CLI workflows and autonomous agents operating as orchestrators (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-28`) or lane executors (`.agents/skills/lucind-apply/SKILL.md:10-21`).

### Current Limitations and Root Causes

1. **Unconditional Worktree Destruction**: `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) unconditionally execute `git worktree remove --force`. If a lane times out or blocks with uncommitted RED-phase tests or partial GREEN implementations, running `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:58`, `cmd/lucind-ai/cli.go:1940-1978`) permanently wipes uncommitted progress with zero warning. Current tests only verify removal of clean worktrees (`internal/worktree/worktree_test.go:1034-1057`, `cmd/lucind-ai/cli_test.go:2989-3004`).
2. **Missing Skill Anchoring in CLI Banners**:
   - `printReport` (`cmd/lucind-ai/cli.go:698-726`) prints a non-completion warning but provides no prescriptive recovery steps, causing agents to reflexively discard worktrees.
   - `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) lists `reverted_ids` without directing operators to `lucind-ai integrate retry` (`cmd/lucind-ai/cli.go:1997-2018`) or the recovery reference (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`).
   - `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) outputs mechanical acceptance without referencing the mandatory qualitative review checklist (steps 2–10 in `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`).
   - `runSplit` (`cmd/lucind-ai/cli.go:485-516`) does not remind operators to refresh static `base_sha` values between sequential apply waves (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`).
3. **Absence of TDD Rescue Protocol**: `troubleshooting.md` (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:11-16`) mentions preserving work, but neither it nor `lucind-apply` (`.agents/skills/lucind-apply/SKILL.md:10-21`) specifies a concrete protocol for committing WIP state when a timeout occurs mid-cycle.

### New and Modified Capabilities

- **Destructive Operation Guardrail**: `worktree.Cleanup` and `worktree.Remove` leverage `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) to fail-closed with `ErrWorktreeDirty` unless `force=true`. The CLI `worktree cleanup` command requires `--force` (`-f`) to discard dirty worktrees.
- **Actionable CLI Banners**: Terminal output for blocked lanes, integration reverts, split DAG emission, and acceptance receipts embeds prescriptive recovery steps and exact skill reference paths (`plugin/claude-code/skills/lucind-ai/SKILL.md:33-49`).
- **Standardized TDD Progress Rescue**: Prescribes committing WIP tests/implementations on timeouts before re-dispatching with adjusted quotas or timeouts.

## Scenarios & Use Cases

### Scenario 1 — Blocked Cleanup on Dirty Worktree

- **Context**: A lane times out during test implementation, leaving uncommitted files in its linked worktree (`internal/worktree/worktree.go:247-253`).
- **Action**: An operator or agent runs `lucind-ai worktree cleanup --lane <id>` without `--force` (`cmd/lucind-ai/cli.go:1940-1978`).
- **Outcome**: The command fails with exit code 1, preserves the worktree intact on disk, reports uncommitted modifications via `git status --porcelain`, and outputs inspection commands (`git diff`) and the `--force` requirement.

### Scenario 2 — Intentional Force Cleanup

- **Context**: An operator audits a blocked lane, determines the uncommitted changes are discarded exploratory artifacts, and decides to reset the worktree.
- **Action**: The operator runs `lucind-ai worktree cleanup --lane <id> --force` (`cmd/lucind-ai/cli.go:58`, `cmd/lucind-ai/cli.go:1940-1978`).
- **Outcome**: The worktree is successfully removed (`internal/worktree/worktree.go:255-261`), exiting with code 0 and unblocking subsequent re-dispatches of the same packet ID.

### Scenario 3 — TDD RED/GREEN Progress Rescue on Lane Timeout

- **Context**: A lane executor completes RED-phase unit tests (`.agents/skills/lucind-apply/SKILL.md:10-21`) but hits the execution deadline (`cmd/lucind-ai/cli.go:698-726`) before finishing GREEN implementation.
- **Action**: `lucind-ai run` emits a terminal banner pointing to the TDD rescue protocol in `troubleshooting.md` (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:11-16`). The orchestrator commits partial progress as `wip: partial lane progress` and re-dispatches with an extended timeout.
- **Outcome**: Valuable test authoring and partial implementation work is preserved rather than discarded and re-computed from scratch.

### Scenario 4 — Feature Revert Recovery via Integrate Retry

- **Context**: A multi-lane batch executes to completion, but batch integration fails because the base commit was red (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`).
- **Action**: `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) lists `reverted_ids` and prints a warning instructing the operator not to re-dispatch with AI agents, directing them to fix the base and run `lucind-ai integrate retry --run <run-id>` (`cmd/lucind-ai/cli.go:1997-2018`).
- **Outcome**: The operator repairs the base commit and runs `integrate retry`, promoting all done lanes without incurring additional LLM token costs or execution latency.

### Scenario 5 — Qualitative Acceptance Checklist Enforcement

- **Context**: An operator runs `lucind-ai accept --run <run-id> --lane <lane-id>` after a lane completes (`cmd/lucind-ai/cli.go:685-690`).
- **Action**: `renderAcceptanceReceipt` emits the mechanical receipt along with a banner detailing qualitative review steps 2–10 (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`), highlighting line-by-line diff review (Step 4) and test semantics auditing (Step 6).
- **Outcome**: The operator performs qualitative inspection before Change Promotion, preventing mechanical-pass regressions.

### Scenario 6 — Multi-Wave DAG Packet Generation

- **Context**: An operator runs `lucind-ai split --dag <path> --out <dir>` (`cmd/lucind-ai/cli.go:485-516`) for a multi-wave change.
- **Action**: `runSplit` successfully writes packet files and prints a reminder on `stdout` regarding static `base_sha` invariants (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`).
- **Outcome**: The operator is reminded to update `base_sha` and `expected_parent_sha` between sequential waves, preventing candidate replacement bugs.

## Success Criteria

- [ ] `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) return `ErrWorktreeDirty` without removing files when the worktree contains uncommitted tracked or untracked changes (`internal/worktree/worktree.go:319-325`), unless `force=true`.
- [ ] Idempotent cleanup on nonexistent worktrees continues to return `nil` (`internal/worktree/worktree_test.go:1059-1069`).
- [ ] `lucind-ai worktree cleanup --lane <id>` exits 1 with actionable guidance when dirty and succeeds when `--force` (`-f`) is passed (`cmd/lucind-ai/cli.go:1940-1978`).
- [ ] `printReport` (`cmd/lucind-ai/cli.go:698-726`) prints a recovery banner referencing `references/operations/troubleshooting.md` when lane status is not `done`.
- [ ] `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) emits `integrate retry` instructions and references `references/coordination/recovery-reconciliation.md` when `reverted_ids` is non-empty.
- [ ] `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) outputs reminders for qualitative review steps 2–10 from `references/contracts/acceptance-promotion.md`.
- [ ] `runSplit` (`cmd/lucind-ai/cli.go:485-516`) emits a multi-wave `base_sha` maintenance banner.
- [ ] Skill documentation (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-28`, `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:11-16`, `.agents/skills/lucind-apply/SKILL.md:10-21`) documents the Pre-Action Hard Gates and TDD rescue protocol.

## Open Questions

- [ ] Should `worktree.Cleanup` return a distinct `ErrWorktreeDirty` sentinel error to allow callers to programmatically differentiate dirty worktrees from missing or invalid worktree paths?
- [ ] Should CLI warning banners be routed strictly to `stderr` to preserve stdout stream parseability for downstream tooling?

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Strict TDD Lifecycle steps (RED, GREEN, SWEEP) requiring timeout recovery protocol |
| `cmd/lucind-ai/cli.go:58` | Usage string lacking `--force` flag on `worktree cleanup` command |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` emits DAG packets without multi-wave `base_sha` warning banner |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` prints mechanical acceptance without qualitative checklist steps 2–10 |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` non-done lane banner lacks prescriptive recovery and skill pointers |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` lists `reverted_ids` without pointing to `integrate retry` |
| `cmd/lucind-ai/cli.go:1940-1978` | `runWorktreeCleanup` executes cleanup unconditionally without dirty worktree checks |
| `cmd/lucind-ai/cli.go:1997-2018` | `runIntegrateRetry` provides no-redispatch integration recovery for reverted lanes |
| `cmd/lucind-ai/cli_test.go:2989-3004` | CLI test verifies cleanup on clean worktree but does not test dirty worktree guardrails |
| `internal/worktree/worktree.go:247-253` | `Cleanup` removes worktrees unconditionally without checking dirty status |
| `internal/worktree/worktree.go:255-261` | `Remove` passes `--force` directly to git, deleting uncommitted modifications |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` checks whether worktree status is clean via `git status --porcelain` |
| `internal/worktree/worktree_test.go:1034-1057` | Unit test verifies removal of clean worktree without dirty-tree assertions |
| `internal/worktree/worktree_test.go:1059-1069` | Unit test verifies idempotent cleanup on nonexistent worktree |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:18-28` | Skill hard rules lacking pre-action hard gates table |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:33-49` | Decision gates table mapping runtime situations to reference modules |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | 10-step acceptance protocol defining mechanical and qualitative review gates |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing guidance for refreshing static `base_sha` and `expected_parent_sha` |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Recovery protocol for feature-targeted reverted lanes using `integrate retry` |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:11-16` | Troubleshooting guidance for missing results, reverts, and timeouts |
