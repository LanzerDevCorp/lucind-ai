# Proposal Lens A — Candidate & Approach: Skill Anchoring & Worktree Cleanup Guardrails

## Selected Candidate & Approach

Following the unified recommendation in `openspec/changes/skill-anchoring-guardrails/explore.md:24-32`, this change adopts **Candidate 1: Fail-Closed Worktree Cleanup Guardrails with Direct CLI Banner Anchoring and Prescriptive TDD WIP-Rescue Protocol**.

Currently, `worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) delegates directly to `worktree.Remove` (`internal/worktree/worktree.go:256-261`), which unconditionally invokes `git worktree remove --force`. When an agent times out (`cmd/lucind-ai/cli.go:37-44`, `internal/run/run.go:452-465`) or encounters a blocking condition during implementation, salvageable uncommitted test or implementation files in `pathFor` (`internal/worktree/worktree.go:150-159`) are permanently destroyed whenever an operator executes `lucind-ai worktree cleanup --lane <id>` (`cmd/lucind-ai/cli.go:58`, `cmd/lucind-ai/cli.go:1934-1978`). While `internal/worktree/worktree.go:319-325` provides `PorcelainEmpty`, cleanup fails to inspect dirty status prior to deletion. Furthermore, terminal reports across `cmd/lucind-ai/cli.go` omit prescriptive pointers to authoritative skill modules under `plugin/claude-code/skills/lucind-ai/SKILL.md:18-49`, leading operators to guess recovery commands or prematurely re-dispatch costly AI lanes.

Candidate 1 solves these issues deterministically through three coordinated mechanisms:
1. **Fail-Closed Worktree Cleanup**: Updates `worktree.Cleanup` and `worktree.Remove` to evaluate `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`). If uncommitted modifications or untracked non-ignored files exist, cleanup aborts and returns `ErrWorktreeDirty` without deleting files unless an explicit `force` boolean is true. The CLI command `lucind-ai worktree cleanup` adds `--force` (`-f`) support (`cmd/lucind-ai/cli.go:1934-1978`).
2. **Deterministic CLI Banner Anchoring**: Embeds static guidance banners at four critical execution milestones in `cmd/lucind-ai/cli.go`:
   - Non-done lane reports in `printReport` (`cmd/lucind-ai/cli.go:698-726`) link to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`.
   - Integration reports with `reverted_ids` in `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) print instructions to run `lucind-ai integrate retry --run <run-id>` (`cmd/lucind-ai/cli.go:1997-2023`) per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`.
   - Acceptance output in `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) reminds operators of qualitative review steps 2–10 from `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`.
   - Apply DAG splitting in `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) reminds operators to refresh `base_sha` and `expected_parent_sha` between sequential waves per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`.
3. **Prescriptive TDD WIP-Rescue Protocol**: Documents the operator protocol in `troubleshooting.md` (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`) and `.agents/skills/lucind-apply/SKILL.md:10-21` to inspect dirty worktrees after timeouts, commit partial progress, and re-dispatch with extended deadlines without losing work.

## Conceptual Changes & Architecture Rationale

### Sentinel Error & API Signatures
- **Exported Error**: Introduce `worktree.ErrWorktreeDirty = errors.New("worktree: working tree contains uncommitted changes")` in `internal/worktree/worktree.go`.
- **Function Signatures**: Update signatures to:
  - `worktree.Cleanup(ctx context.Context, primaryRoot, laneID string, force bool) error`
  - `worktree.Remove(ctx context.Context, primaryRoot, path string, force bool) error`
- **Architecture Rationale**: Updating the parameter list to include `force bool` rather than adding a separate unexported bypass forces compile-time auditing of every call site across the codebase. Accidental deletion paths cannot compile without explicit handling. Idempotency on nonexistent worktrees remains preserved (`internal/worktree/worktree_test.go:1059-1069`).

### CLI Surface & Operator Experience
- **Flag Definition**: Update `cmd/lucind-ai/cli.go:1934-1978` and usage string (`cmd/lucind-ai/cli.go:58`) to add `--force` and `-f` (`fs.BoolVar(&force, "force", false, ...)` and `fs.BoolVar(&force, "f", false, ...)`).
- **Fail-Closed Behavior**: When `worktree.Cleanup` returns `ErrWorktreeDirty` (and `force == false`), `runWorktreeCleanup` prints the porcelain diff status, points the operator to `troubleshooting.md`, prints the exact `--force` command to override, and exits 1 without deleting files. Existing clean worktree removal continues to succeed cleanly (`cmd/lucind-ai/cli_test.go:2974-3010`, `internal/worktree/worktree_test.go:1034-1057`).

### Internal Automated Teardown Callers
Internal system callers managing disposable scratch worktrees or executing automated teardowns after validated promotion must pass `force: true` to preserve existing unconditional teardown invariants:
1. **Combined Scratch Teardown**: `DiscardCombined` in `cmd/lucind-ai/cli.go:858-863` passes `force: true` to discard disposable combined test trees on check failure or abort.
2. **Lane Worktree Teardown**: `RemoveLaneWorktree` in `cmd/lucind-ai/cli.go:864-869` passes `force: true` during post-promotion teardown in `internal/run/integrate.go:159-165`.
3. **Merge Conflict Abort**: `Combine` in `internal/integrate/integrate.go:118-124` passes `force: true` when removing the temporary worktree after a merge conflict abort.
4. **Candidate Promotion Teardown**: `ResolveCandidate` in `internal/integrate/candidate.go:262-265` passes `force: true` to clean up candidate worktrees upon successful promotion.

## Alternatives Considered & Rejected

### Candidate 2: Automatic Stash/Commit on Teardown with Dynamic Prompt Context Injection
- **Description**: Automatically creating a synthetic `wip: <lane-id>` commit or stash upon lane timeout or failure, combined with injecting skill documentation dynamically into executor prompts.
- **Reasons for Rejection**:
  1. Pollutes git history and ref namespaces with broken, unverified intermediate states.
  2. Violates strict clean-tree guarantees required by completion mode verification (`internal/run/run.go:906-940`).
  3. Dynamic prompt injection significantly inflates token consumption on every dispatch and tightly couples execution runners to mutable local file paths.

### Candidate 3: Interactive TTY Prompting with Ledger Quarantine State
- **Description**: Halting cleanup to prompt the operator interactively in the terminal (`[y/N]`), while recording a quarantined worktree state inside the ledger database.
- **Reasons for Rejection**:
  1. Incompatible with headless, non-interactive batch invocations and CI automation (`cmd/lucind-ai/cli.go:37-44`).
  2. Introduces unnecessary schema migrations and database state transitions for what is fundamentally a local filesystem guardrail.

## Open Questions

- [ ] None. (Execution-topology precedence: As authorized by this packet, the three-lens proposal fan-out and skeleton take precedence over `~/.claude/skills/sdd-propose/SKILL.md:92-158` monolithic single-agent proposal layout.)

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Strict TDD lifecycle phases and pre-commit verification rules in apply lanes. |
| `cmd/lucind-ai/cli.go:37-44` | Default lane timeout and headless dispatch configuration. |
| `cmd/lucind-ai/cli.go:58` | Top-level CLI usage string documenting subcommands and flags. |
| `cmd/lucind-ai/cli.go:485-516` | Implementation of `runSplit` generating packet files and wave commands. |
| `cmd/lucind-ai/cli.go:685-690` | Output formatting in `renderAcceptanceReceipt` for acceptance receipts. |
| `cmd/lucind-ai/cli.go:698-726` | Terminal output formatting in `printReport` for completed and non-done lanes. |
| `cmd/lucind-ai/cli.go:730-740` | Terminal output formatting in `printIntegrateReport` for integrated and reverted lanes. |
| `cmd/lucind-ai/cli.go:858-863` | `DiscardCombined` closure definition executing unconditional `worktree.Remove`. |
| `cmd/lucind-ai/cli.go:864-869` | `RemoveLaneWorktree` closure definition executing unconditional `worktree.Remove`. |
| `cmd/lucind-ai/cli.go:1934-1978` | Implementation of `runWorktreeCleanup` command and flag parsing. |
| `cmd/lucind-ai/cli.go:1997-2023` | Implementation and documentation of `runIntegrateRetry` subcommand. |
| `cmd/lucind-ai/cli_test.go:2974-3010` | Unit tests for `worktree cleanup` CLI behavior and idempotency. |
| `internal/dag/split.go:34-43` | Output formatting for wave commands emitted to stdout during split. |
| `internal/integrate/candidate.go:262-265` | Teardown of scratch worktree and branch upon successful candidate promotion. |
| `internal/integrate/integrate.go:118-124` | Teardown of scratch worktree and branch upon merge conflict in `Combine`. |
| `internal/run/integrate.go:159-165` | Automated cleanup of lane worktrees following successful batch integration. |
| `internal/run/run.go:452-465` | Timeout handling and diagnosis reporting during batch lane execution. |
| `internal/run/run.go:906-940` | Verification of unique commits and clean porcelain status in `enforceCompletionMode`. |
| `internal/worktree/worktree.go:150-159` | Calculation of linked worktree paths in `pathFor`. |
| `internal/worktree/worktree.go:247-253` | Implementation of `Cleanup` delegating to `Remove`. |
| `internal/worktree/worktree.go:256-261` | Implementation of `Remove` executing `git worktree remove --force`. |
| `internal/worktree/worktree.go:319-325` | Implementation of `PorcelainEmpty` checking `git status --porcelain`. |
| `internal/worktree/worktree_test.go:1034-1057` | Unit test verifying `Cleanup` removes an existing clean worktree. |
| `internal/worktree/worktree_test.go:1059-1069` | Unit test verifying `Cleanup` is an idempotent no-op on nonexistent worktrees. |
| `openspec/changes/skill-anchoring-guardrails/explore.md:24-32` | Exploration candidate comparison and recommendation of Candidate 1. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:18-49` | Core operational hard rules and decision gate routing table. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step acceptance checklist protocol and qualitative gates. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing rules for advancing parent ref and refreshing SHAs. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Integration retry protocol for recovering reverted feature attempts. |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Troubleshooting matrix and diagnostic steps for dispatch and integration failures. |
