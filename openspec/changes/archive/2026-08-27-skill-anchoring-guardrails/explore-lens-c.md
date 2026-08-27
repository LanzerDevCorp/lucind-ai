# Explore Lens C — Risks, Trade-offs & Spikes: Skill Anchoring & Worktree Cleanup Guardrails

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|
| Backward-compat breakage across internal callers and unit tests expecting unconditional deletion | High | Preserve forced deletion for automated internal teardowns (`integrate.Combine` abort and promotion cleanup) via explicit flags or helper; gate operator-facing cleanup behind dirty checks requiring `--force`. Update unit tests to assert both dirty rejection and forced cleanup. | `internal/worktree/worktree.go:247-253`, `internal/worktree/worktree.go:255-261`, `internal/integrate/integrate.go:121-124`, `internal/integrate/candidate.go:262-265`, `cmd/lucind-ai/cli.go:858-869`, `cmd/lucind-ai/cli.go:1934-1978`, `internal/worktree/worktree_test.go:1034-1057`, `cmd/lucind-ai/cli_test.go:2974-3010` |
| Output stream contamination: guidance banners breaking downstream CLI stdout parsers | Medium | Route interactive guidance banners, checklists, and multi-wave warnings to `stderr` or place after machine-parseable delimiters. Maintain strict whitespace and token formatting for single-line stdout records (`acceptance receipt:`, `integrated_ids:`, `reverted_ids:`). | `cmd/lucind-ai/cli.go:58-59`, `cmd/lucind-ai/cli.go:485-516`, `cmd/lucind-ai/cli.go:685-690`, `cmd/lucind-ai/cli.go:728-740`, `cmd/lucind-ai/cli.go:742-750`, `internal/dag/split.go:34-43`, `cmd/lucind-ai/cli_test.go:4515-4530`, `lucind-checks.sh:1-4` |
| False-positive or false-negative dirty detection across ignored or transient files | Medium | Reuse established `worktree.PorcelainEmpty` relying on `git status --porcelain` rather than custom parsing. Verify ignored state (`.lucind/result.json`) remains clean while untracked code modifications trigger dirty detection. | `internal/worktree/worktree.go:317-325`, `internal/worktree/worktree_test.go:536-595` |
| Muscle-memory `--force` execution bypassing WIP recovery and destroying partial TDD work | High | Pair cleanup failure with immediate diagnostic commands (`git status`, `git diff`), prescriptive WIP commit steps (`git commit -am "wip:..."`), and explicit skill anchoring pointing directly to troubleshooting docs before explaining `--force`. | `cmd/lucind-ai/cli.go:698-726`, `cmd/lucind-ai/cli.go:1934-1978`, `plugin/claude-code/skills/lucind-ai/SKILL.md:29-49`, `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` |
| Multi-wave `base_sha` staleness causing downstream waves to branch from stale ancestors | High | Emit a multi-wave invariant warning banner on `lucind-ai split` instructing operators to advance primary checkout and refresh `base_sha` and `expected_parent_sha` in next-wave packets before dispatch. | `cmd/lucind-ai/cli.go:485-516`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:25-30` |

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|
| **Go API Signature**: `worktree.Cleanup(ctx, root, id, force bool)` vs separate `CleanupForced` | Explicit `force bool` parameter forces compile-time auditing of every call site across packages; zero silent un-guarded calls. | Requires touching ~15 call sites across unit and integration tests. | Low (one-time refactoring of test call sites). |
| **Output Channel**: Diagnostic banners to `stderr` vs structured `stdout` | `stderr` guarantees zero breakage for scripts/harnesses capturing stdout (e.g. DAG split wave commands, receipt parsing). | Banners might be omitted if calling harness redirects or suppresses `stderr`. | Low (clean separation of data and diagnostics). |
| **Skill Anchoring**: Static string literals in CLI vs dynamic markdown parser | Static banners have zero runtime overhead, zero disk lookup failure risk, and work deterministically across isolated environments. | Updating skill path references requires binary recompile. | Very Low. |
| **Partial TDD Rescue**: Prescriptive operator-driven WIP commit vs automatic runner commit | Keeps human/orchestrator in control of commit history, avoids committing broken syntax or untracked debris automatically. | Requires human or orchestrator intervention to commit WIP and adjust timeout before redispatch. | Low. |

## Potential Spikes / Proof of Concepts

- **Spike 1: Worktree dirty guardrail and `--force` CLI override (`cmd/lucind-ai/cli.go:1934-1978`, `internal/worktree/worktree.go:247-253`, `internal/worktree/worktree.go:255-261`)**
  Prototype dirty validation in `worktree.Cleanup` using `PorcelainEmpty` (`internal/worktree/worktree.go:317-325`). Verify that untracked or modified files return `ErrWorktreeDirty` and preserve files on disk, while passing `--force` removes the worktree cleanly. Verify against `internal/worktree/worktree_test.go:1034-1057`, `internal/worktree/worktree_test.go:1059-1069`, and `cmd/lucind-ai/cli_test.go:2974-3010`.

- **Spike 2: Output stream isolation for acceptance and split banners (`cmd/lucind-ai/cli.go:485-516`, `cmd/lucind-ai/cli.go:636-683`, `cmd/lucind-ai/cli.go:685-690`)**
  Implement qualitative checklist reminder in `runAccept` / `renderAcceptanceReceipt` and multi-wave warning banner in `runSplit`. Execute automated parsing tests (`cmd/lucind-ai/cli_test.go:4515-4530`, `internal/dag/split.go:34-43`) to prove machine-readable stdout remains uncorrupted.

- **Spike 3: TDD WIP rescue protocol end-to-end simulation (`cmd/lucind-ai/cli.go:698-726`, `internal/worktree/worktree.go:150-159`)**
  Simulate a lane timeout during RED test authoring. Verify that the worktree at `pathFor` (`internal/worktree/worktree.go:150-159`) is preserved, `printReport` outputs the recovery banner with exact diff inspection commands, manual WIP commit succeeds, and subsequent lane redispatch builds on the saved work (`plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18`, `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:31-35`).

## Out of Scope

- Background daemon or timer-based automatic garbage collection of stale worktrees.
- Schema modifications to `.lucind/result.json` or `result.schema.json` (`internal/result/result.go:10-45`).
- Automatic git commits created by the binary upon lane timeout or failure (preserves clean history and human authority).
- Modifying lease acquisition, fence tokens, or recovery mechanics in `internal/feature/feature.go:300-350`.
- Altering core acceptance ledger storage or receipt hashing (`internal/accept/accept.go:120-130`, `internal/ledger/acceptance.go:40-60`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:14-30`).
- Re-architecting multi-agent orchestrator adapters outside `plugin/claude-code/skills/lucind-ai/`.

## Open Questions

- [ ] Should `worktree.Remove` signature be modified to `Remove(ctx, root, path, force bool)` directly, or should a dedicated `worktree.RemoveForced` helper be provided for internal lifecycle callers (`internal/integrate/integrate.go:121-124`, `internal/integrate/candidate.go:262-265`)?
- [ ] Should CLI warning banners for `lucind-ai accept` and `lucind-ai split` be rendered to `stderr` exclusively or appended to `stdout` following the primary output record?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:58-59` | CLI usage string defining command signatures |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` implementation lacking multi-wave `base_sha` warning banner |
| `cmd/lucind-ai/cli.go:636-683` | `runAccept` implementation invoking receipt verification |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` rendering mechanical receipt without qualitative steps 2-10 checklist reminder |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` displaying uncompleted lane banner without actionable TDD WIP rescue protocol |
| `cmd/lucind-ai/cli.go:728-740` | `printIntegrateReport` printing integrate summary without no-redispatch instruction |
| `cmd/lucind-ai/cli.go:742-750` | `printIDList` single-line formatting of integrated and reverted IDs |
| `cmd/lucind-ai/cli.go:858-869` | `DiscardCombined` and `RemoveLaneWorktree` dependencies wiring unconditional `worktree.Remove` |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` implementing worktree cleanup without `--force` flag |
| `cmd/lucind-ai/cli_test.go:2974-3010` | `TestWorktreeCleanupCLI` testing worktree cleanup behavior |
| `cmd/lucind-ai/cli_test.go:4515-4530` | `TestCLIAccept` testing acceptance receipt output format |
| `internal/accept/accept.go:120-130` | Acceptance receipt construction in verifier |
| `internal/dag/split.go:34-43` | `Split` writing wave dispatch commands to stdout |
| `internal/feature/feature.go:300-350` | Lease acquisition and status handling in feature service |
| `internal/integrate/candidate.go:262-265` | Cleanup of worktree upon successful candidate promotion via `worktree.Remove` |
| `internal/integrate/integrate.go:121-124` | Cleanup of worktree upon merge conflict in `Combine` via `worktree.Remove` |
| `internal/ledger/acceptance.go:40-60` | `AcceptanceReceipt` ledger struct definition |
| `internal/result/result.go:10-45` | `Envelope` and `HardStop` struct definitions |
| `internal/worktree/worktree.go:150-159` | `pathFor` resolving worktree destination path |
| `internal/worktree/worktree.go:247-253` | `Cleanup` removing worktree via unconditional `Remove` |
| `internal/worktree/worktree.go:255-261` | `Remove` executing `git worktree remove --force` |
| `internal/worktree/worktree.go:317-325` | `PorcelainEmpty` checking worktree status with `git status --porcelain` |
| `internal/worktree/worktree_test.go:536-595` | `TestPorcelainEmpty` testing clean worktree, ignored files, and untracked files |
| `internal/worktree/worktree_test.go:1034-1057` | `TestCleanupRemovesExistingWorktree` testing unconditional worktree cleanup |
| `internal/worktree/worktree_test.go:1059-1069` | `TestCleanupOnLaneWithNoWorktreeIsNoOp` testing cleanup idempotency |
| `lucind-checks.sh:1-4` | Full-tree test execution script |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:29-49` | Decision Gates table for orchestrator module routing |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:14-30` | Canonical 10-step Acceptance protocol (mechanical vs qualitative) |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:25-30` | Multi-wave sequencing hazards and `base_sha` refresh protocol |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:31-35` | Bisection boundary, reverted IDs, and `integrate retry` no-redispatch protocol |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Dispatch and integration troubleshooting table |
