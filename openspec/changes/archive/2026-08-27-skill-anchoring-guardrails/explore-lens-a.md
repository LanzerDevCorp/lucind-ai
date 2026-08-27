# Explore Lens A — Problem & Candidates: Skill Anchoring & Worktree Cleanup Guardrails

## Problem Space

Lucind AI uses isolated git worktrees for concurrent lane dispatches. However, three critical operational gaps lead operators and autonomous agents to act on intuition or reflex during failures, risking data loss and wasted execution tokens:

1. **Unconditional Worktree Deletion Without Dirty Guardrails**:
   In `internal/worktree/worktree.go:247-254`, `Cleanup` delegates directly to `Remove` (`internal/worktree/worktree.go:256-261`), which unconditionally executes `git worktree remove --force`. If an agent times out (`cmd/lucind-ai/cli.go:37-44`, `internal/run/run.go:452-465`) or blocks mid-implementation, uncommitted files in the worktree are permanently destroyed when `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:1934-1978`) is invoked to reset state. While `internal/worktree/worktree.go:319-325` provides `PorcelainEmpty`, cleanup operations fail to verify dirty status before removal.

2. **Terminal Output Detached from Authority Skills**:
   The repository maintains thorough operational guidance under `plugin/claude-code/skills/lucind-ai/references/`. However, CLI commands fail to surface these references at critical failure points:
   - Non-done lane reports (`cmd/lucind-ai/cli.go:698-726`) warn that the worktree is preserved but provide no prescriptive TDD rescue steps or pointers to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.
   - Integration reports listing `reverted_ids` (`cmd/lucind-ai/cli.go:730-740`) omit instructions directing operators to `lucind-ai integrate retry --run <run-id>` (`plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`), causing operators to trigger expensive AI redispatches.
   - Acceptance receipts (`cmd/lucind-ai/cli.go:636-690`) note that mechanical checks passed but do not remind operators to perform qualitative review steps 2–10 (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`).
   - DAG splitting (`cmd/lucind-ai/cli.go:485-516`) does not warn about refreshing `base_sha` and `expected_parent_sha` between consecutive waves.

3. **Absence of a Prescriptive TDD Rescue Protocol**:
   When a lane times out, partial progress (e.g., newly written RED tests or partial GREEN implementations) is often salvageable. The codebase currently lacks a defined protocol and CLI guidance to inspect, commit WIP progress, and re-dispatch with an extended deadline before cleanups occur.

## Candidate Approaches

### Candidate 1 — Fail-Closed Worktree Guardrails with Direct CLI Banner Anchoring

**Approach**: Update `internal/worktree/worktree.go:247-261` to check `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) and return `ErrWorktreeDirty` unless an explicit `force bool` is provided. Add `--force` / `-f` to `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`). Enhance CLI report generators (`printReport`, `printIntegrateReport`, `runAccept`, `runSplit` in `cmd/lucind-ai/cli.go:485-740`) to print prescriptive recovery guidance and exact reference skill paths. Formalize the TDD WIP rescue protocol in `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.
**Pros**: Direct, fail-closed prevention of accidental data loss; zero breaking changes to DAG or ledger architecture; leverages existing worktree inspection functions; anchors skills in terminal outputs at the moment of failure.
**Cons**: Requires updating `Cleanup` and `Remove` call sites across unit tests and internal callers; slightly increases CLI banner output verbosity.
**Feasibility**: High. `PorcelainEmpty` is already implemented and proven in `internal/worktree/worktree.go:319-325`. Call sites in `cmd/lucind-ai/cli.go:858-869` and `cmd/lucind-ai/cli.go:1934-1978` are well-isolated.

### Candidate 2 — Automatic Stash/WIP Commit on Teardown with Prompt Context Injection

**Approach**: Modify `internal/worktree/worktree.go:256-261` to automatically record uncommitted changes to an ephemeral ref (`refs/lucind/wip/<lane-id>`) before removing the worktree. Extend executor runners to inject relevant skill reference files directly into subsequent agent prompts on failure rather than printing terminal banners.
**Pros**: Guarantees zero data loss without requiring manual operator intervention or explicit commit commands.
**Cons**: Pollutes the git ref namespace with dangling WIP references; automated prompt injection couples executor dispatch to skill filesystem paths and increases token consumption unpredictably.
**Feasibility**: Low. Creating synthetic commits during teardown complicates clean-tree guarantees enforced in `internal/run/run.go:906-940` and feature attempt recovery.

### Candidate 3 — Interactive TTY Prompting with Ledger Quarantine State

**Approach**: Add interactive terminal confirmation prompts to `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) when dirty worktrees are detected. Introduce a `quarantined` state in `internal/ledger` to track dirty worktrees, requiring an explicit resolution command before worktree paths can be reused.
**Pros**: Highly explicit interactive experience for human terminal operators.
**Cons**: Breaks headless, non-interactive batch dispatches (`cmd/lucind-ai/cli.go:37-44`); substantially increases ledger schema and state machine complexity for a local filesystem safety check.
**Feasibility**: Medium. Headless execution is a core architectural requirement, making interactive prompts a mismatch that requires complex non-interactive fallback handling.

## Initial Recommendations

**Recommendation**: Adopt **Candidate 1**.
**Rationale**: Candidate 1 implements deterministic, fail-closed safety without introducing unnecessary schema complexity or polluting git refs. It directly addresses dirty worktree destruction by requiring `--force` when `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) detects uncommitted files. Furthermore, anchoring skill references directly in CLI banners (`cmd/lucind-ai/cli.go:698-740`) ensures both human operators and autonomous orchestrators receive immediate, actionable guidance at critical decision gates without breaking headless execution.

## Open Questions

- [ ] Should internal teardown callers that operate on temporary integration branches (such as `DiscardCombined` in `cmd/lucind-ai/cli.go:858-869` and `completeIntegration` in `internal/run/integrate.go:159-165`) pass `force: true` directly or use an explicit un-guarded internal helper?
- [ ] Should CLI banner paths reference worktree-relative paths (`plugin/claude-code/skills/lucind-ai/references/...`) or standard user skill directories (`~/.claude/skills/...`)?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:37-44` | Default lane timeout definition for headless execution. |
| `cmd/lucind-ai/cli.go:485-516` | Implementation of `runSplit` subcommand. |
| `cmd/lucind-ai/cli.go:636-690` | Implementation of `runAccept` subcommand and acceptance receipt rendering. |
| `cmd/lucind-ai/cli.go:698-726` | Implementation of `printReport` for lane outcomes. |
| `cmd/lucind-ai/cli.go:730-740` | Implementation of `printIntegrateReport` listing integrated and reverted lanes. |
| `cmd/lucind-ai/cli.go:858-869` | Production dependencies wiring for worktree removal and combined branch discard. |
| `cmd/lucind-ai/cli.go:1934-1978` | Implementation of `runWorktreeCleanup` subcommand. |
| `internal/run/integrate.go:159-165` | Worktree removal for integrated lanes during integration completion. |
| `internal/run/run.go:452-465` | Context handling and outcome persistence during lane timeout. |
| `internal/run/run.go:906-940` | Completion mode enforcement verifying unique commits and clean working tree. |
| `internal/worktree/worktree.go:247-254` | Implementation of `Cleanup` checking existence and delegating to `Remove`. |
| `internal/worktree/worktree.go:256-261` | Implementation of `Remove` executing unconditional `git worktree remove --force`. |
| `internal/worktree/worktree.go:319-325` | Implementation of `PorcelainEmpty` checking `git status --porcelain`. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | 10-step acceptance protocol checklist. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Reverted lane recovery protocol specifying `integrate retry`. |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Troubleshooting matrix for dispatch and integration symptoms. |
