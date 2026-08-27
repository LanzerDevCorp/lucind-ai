# Spec Lens A — Capabilities & Requirements: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed requirements

This change introduces six requirements across five capabilities proposed in `openspec/changes/skill-anchoring-guardrails/proposal.md:29-39`: three candidate New capabilities (`worktree-dirty-guardrail`, `failure-guidance-banners`, and `tdd-wip-rescue-protocol`) and two candidate Modified capabilities (`lane-worktree-lifecycle` and `worktree-cleanup-cli`). An audit of the live specifications in `openspec/specs/` confirms that neither `lane-worktree-lifecycle` nor `worktree-cleanup-cli` exists in the repository, and their behaviors are not covered under `lane-execution` (`openspec/specs/lane-execution/spec.md:1-84`), `acceptance-verifier` (`openspec/specs/acceptance-verifier/spec.md:1-121`), or any other existing live spec. Consequently, all five capabilities are classified as New, and all six requirements are specified as ADDED with `## MODIFIED Requirements` omitted.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `worktree-dirty-guardrail` | New | `openspec/specs/worktree-dirty-guardrail/spec.md` | — |
| `failure-guidance-banners` | New | `openspec/specs/failure-guidance-banners/spec.md` | — |
| `tdd-wip-rescue-protocol` | New | `openspec/specs/tdd-wip-rescue-protocol/spec.md` | — |
| `lane-worktree-lifecycle` | New | `openspec/specs/lane-worktree-lifecycle/spec.md` | — |
| `worktree-cleanup-cli` | New | `openspec/specs/worktree-cleanup-cli/spec.md` | — |

## ADDED Requirements

### Requirement: Worktree cleanup dirty guardrail and force flag

`worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) and `worktree.Remove` (`internal/worktree/worktree.go:256-261`) MUST check `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) before deleting any linked worktree directory (`internal/worktree/worktree.go:150-159`), failing closed and returning `worktree.ErrWorktreeDirty` unless `force: true` is explicitly passed. `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:1934-1978`) MUST accept a `--force` (`-f`) flag; unforced cleanup invocations against a dirty worktree MUST exit 1, output the dirty status and diff diagnostic commands, and preserve the worktree files on disk. Clean worktrees and nonexistent worktree paths MUST be cleaned up idempotently with exit code 0. Internal automated teardown callers (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`) MUST pass `force: true` to proceed without dirty checks.

**Terminal consumer**: `cmd/lucind-ai/cli.go:1934-1978` (`runWorktreeCleanup`), `internal/worktree/worktree.go:247-261` (`Cleanup` and `Remove`), `cmd/lucind-ai/cli.go:858-869` (`DiscardCombined` and `RemoveLaneWorktree`), `internal/integrate/integrate.go:118-124` (`Combine` conflict teardown), `internal/integrate/candidate.go:262-265` (`ResolveCandidate` teardown), and `internal/run/integrate.go:159-165` (`integrateAttempt`).

### Requirement: Blocked and timeout lane report guidance banner

Upon reporting a lane terminating with any status other than `lane.Done` (including `blocked`, `failed`, or dispatch timeout) (`internal/run/run.go:452-465`), `printReport` (`cmd/lucind-ai/cli.go:698-726`) MUST print a visual warning banner displaying the preserved worktree path (`internal/worktree/worktree.go:150-159`), porcelain status diagnostic inspection steps, and an explicit reference to `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`. The guidance banner SHALL NOT be printed for lanes completing with `lane.Done`.

**Terminal consumer**: `cmd/lucind-ai/cli.go:698-726` (`printReport`), rendered on stdout during CLI lane execution reporting when `Report.Status != lane.Done`.

### Requirement: Integration report reverted IDs recovery banner

When an integration batch finishes with one or more reverted lanes (`reverted_ids` non-empty), `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) MUST print recovery guidance instructing operators to run `lucind-ai integrate retry --run <run-id>` (`cmd/lucind-ai/cli.go:1997-2023`) and referencing the recovery protocol in `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35`.

**Terminal consumer**: `cmd/lucind-ai/cli.go:730-740` (`printIntegrateReport`), rendered on stdout during integration outcome reporting when `IntegrateReport.Reverted` is non-empty.

### Requirement: Acceptance receipt qualitative review banner

Upon completing mechanical verification and rendering an acceptance receipt (`internal/accept/accept.go:120-130`), `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) MUST print an explicit reminder banner prompting operators to perform qualitative review checklist steps 2–10 defined in `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`. The output MUST reaffirm that the receipt constitutes mechanical evidence only and does not imply qualitative approval.

**Terminal consumer**: `cmd/lucind-ai/cli.go:685-690` (`renderAcceptanceReceipt`), rendered on stdout during `lucind-ai accept` execution.

### Requirement: DAG split multi-wave base SHA warning banner

When `lucind-ai split` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) processes an apply DAG with two or more execution waves, it MUST output a warning banner instructing operators to advance the primary checkout and refresh `base_sha` and `expected_parent_sha` in next-wave packets between wave dispatches per `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30`. The warning banner MUST be routed to stderr to preserve pipeline-parseable wave commands on stdout.

**Terminal consumer**: `cmd/lucind-ai/cli.go:485-516` (`runSplit`) and `internal/dag/split.go:34-43` (`Split`), emitting banner to stderr.

### Requirement: Prescriptive TDD WIP-rescue protocol documentation

`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21` MUST document a prescriptive TDD WIP-rescue procedure for timed-out or blocked apply lanes. The documented procedure MUST instruct operators to inspect uncommitted diffs within preserved worktrees (`internal/worktree/worktree.go:150-159`), create a partial WIP commit, update packet timeout parameters, and re-dispatch without losing uncommitted RED test or GREEN implementation progress.

**Terminal consumer**: `.agents/skills/lucind-apply/SKILL.md:10-21` and `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` (consumed by human operators and autonomous lane agents executing TDD lifecycles).

## Open Questions

- [ ] Stderr vs stdout routing for `lucind-ai accept` qualitative reminder: Should the qualitative review reminder banner route to stderr or append to stdout following receipt fields? (The proposal tracks this under Open Question 1; DAG split banner is already pinned to stderr to preserve stdout wave commands).
- [ ] Internal caller signature pattern: Should internal callers pass `force: true` to `worktree.Cleanup`/`Remove` directly or utilize dedicated helper wrappers like `RemoveForced`? (Tracked under Proposal Open Question 2).
- [ ] SDD spec phase structure: The `sdd-spec` skill describes a monolithic delta spec under `specs/`, while this execution follows the parallel three-lens decomposition where Lens A authors the capability map and requirement definitions and synthesis completes the final delta tree. (Process note acknowledging packet authority).

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | TDD lifecycle phases and pre-commit verification steps |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` parses DAG flags and executes `dag.Split` for wave command generation |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` outputs receipt ID, binding hash, candidate commit, and mechanical meaning |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` outputs lane status, worktree path, summary, and non-done banner |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` formats integration outcome, integrated IDs, and reverted IDs |
| `cmd/lucind-ai/cli.go:858-869` | `DiscardCombined` and `RemoveLaneWorktree` dependencies delegate to `worktree.Remove` |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` implements CLI worktree cleanup subcommand for stale lane worktrees |
| `cmd/lucind-ai/cli.go:1997-2023` | `runIntegrateRetry` implements CLI integration retry subcommand for reverted lane batches |
| `internal/accept/accept.go:120-130` | `Verifier.Verify` constructs and persists `AcceptanceReceipt` record to ledger |
| `internal/dag/split.go:34-43` | `Split` emits wave command invocations to stdout writer |
| `internal/integrate/candidate.go:262-265` | `ResolveCandidate` removes scratch candidate worktree and branch on successful promotion |
| `internal/integrate/integrate.go:118-124` | `Combine` aborts merge and removes scratch worktree and branch on conflict |
| `internal/run/integrate.go:159-165` | `integrateAttempt` removes lane worktrees and branches for integrated lanes |
| `internal/run/run.go:452-465` | `Execute` handles non-done lane outcomes and records progress diagnostics |
| `internal/worktree/worktree.go:150-159` | `pathFor` computes linked worktree destination directory for a lane |
| `internal/worktree/worktree.go:247-253` | `Cleanup` checks worktree path existence and delegates to `Remove` |
| `internal/worktree/worktree.go:256-261` | `Remove` invokes git worktree remove command |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` inspects worktree git status for uncommitted changes |
| `openspec/changes/skill-anchoring-guardrails/proposal.md:29-39` | Capabilities section proposing 3 New and 2 Modified capabilities |
| `openspec/specs/acceptance-verifier/spec.md:1-121` | Live specification for acceptance-verifier capability |
| `openspec/specs/lane-execution/spec.md:1-84` | Live specification for lane-execution capability |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Acceptance 10-step sequence including qualitative review checklist |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing guidance for advancing checkout and refreshing packet SHAs |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Bisection recovery and integration retry guidance for reverted lane batches |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Troubleshooting table covering lane failures, timeouts, and stale worktrees |
