# Design Lens B — Surface & Flow: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed architecture

We assume Candidate 1 from the accepted proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:1-46`):

1. **Sentinel Error & Guardrail Check**: Export `ErrWorktreeDirty` in `internal/worktree` (`internal/worktree/worktree.go:26-45`). Add `force bool` to `worktree.Cleanup` (`internal/worktree/worktree.go:247-253`) and `worktree.Remove` (`internal/worktree/worktree.go:256-261`). When `force == false`, `worktree.Remove` evaluates `worktree.PorcelainEmpty` (`internal/worktree/worktree.go:319-325`); if dirty, removal aborts fail-closed returning `ErrWorktreeDirty` without file deletion. Nonexistent paths remain idempotent (`nil`).
2. **CLI Surface & Operator Triage**: Add `--force` / `-f` to `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:58,1934-1978`). When unforced cleanup encounters `ErrWorktreeDirty`, `runWorktreeCleanup` prints uncommitted status, diagnostic commands (`git diff`), references `troubleshooting.md:7-18`, notes `--force`, and exits 1.
3. **Internal Callers**: Internal automated teardown sites pass `force: true` to `worktree.Remove`: `DiscardCombined` (`cmd/lucind-ai/cli.go:858-869`), `RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:858-869`), merge conflict abort (`internal/integrate/integrate.go:118-124`), and candidate promotion cleanup (`internal/integrate/candidate.go:262-265`).
4. **Lifecycle Guidance Banners**: Static guidance banners are embedded at four milestones:
   - `printReport` (`cmd/lucind-ai/cli.go:698-726`) on non-done status (`blocked`, `failed`, timeout) routes to stdout citing `troubleshooting.md:7-18`.
   - `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) on non-empty `reverted_ids` routes to stdout citing `recovery-reconciliation.md:33-35` and `integrate retry`.
   - `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) on mechanical verification routes to stdout citing qualitative checklist steps 2–10 in `acceptance-promotion.md:18-30`.
   - `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) on multi-wave DAGs routes a warning banner to stderr citing `recovery-reconciliation.md:27-30`.
5. **Skill Protocol**: Document the TDD WIP-rescue protocol in `troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21`.

## Flow and Invariants

```
[ Operator CLI / Automated Internal Caller ]
                     │
                     ▼
  [ Parse --force / -f flag (CLI) or pass force bool (Internal) ]
                     │
                     ▼
       [ worktree.Cleanup(ctx, primaryRoot, laneID, force) ]
                     │
        ┌────────────┴────────────┐
   (path exists)            (path not found)
        │                         │
        ▼                         ▼
 [ worktree.Remove(..., force) ]  [ Return nil (Idempotent 0) ]
        │
   ┌────┴────────────────────────┐
 (force == true)          (force == false)
   │                             │
   │                             ▼
   │                 [ worktree.PorcelainEmpty(path) ]
   │                             │
   │                    ┌────────┴────────┐
   │                 (clean)           (dirty)
   │                    │                 │
   ▼                    ▼                 ▼
[ git worktree remove --force ]   [ Return ErrWorktreeDirty ]
   │                                      │
   ▼                                      ▼
[ Return nil (Exit 0) ]           [ CLI: Print diff/guidance & exit 1 ]
```

### Invariants

1. **Fail-Closed Deletion Invariant**: A dirty worktree containing uncommitted tracked or untracked changes (`internal/worktree/worktree.go:319-325`) is NEVER deleted unless `force == true` is explicitly provided.
2. **Internal Automation Invariant**: Automated internal teardown sites (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`) always pass `force: true` to prevent blocking ephemeral scratch tree disposal.
3. **Idempotent Clean Removal Invariant**: If a worktree does not exist on disk or is clean, `Cleanup` succeeds returning `nil` regardless of the `force` value (`internal/worktree/worktree.go:247-253`).
4. **Stream Separation Invariant**: Guidance banners on `lucind-ai split` route to `stderr` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) preserving scriptable `stdout` pipelines; banners in `printReport`, `printIntegrateReport`, and `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-740`) append after structured key-value lines on `stdout` without breaking parser prefixes.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `worktree.Cleanup` signature | `internal/worktree/worktree.go:247-253` | Extend with `force bool`: `Cleanup(ctx context.Context, primaryRoot, laneID string, force bool) error` | No (internal package signature updated across callers) |
| `worktree.Remove` signature | `internal/worktree/worktree.go:256-261` | Extend with `force bool`: `Remove(ctx context.Context, primaryRoot, path string, force bool) error` | No (internal package signature updated across callers) |
| `worktree.ErrWorktreeDirty` sentinel | `internal/worktree/worktree.go:26-45` | Add exported error: `var ErrWorktreeDirty = errors.New("worktree: linked worktree has uncommitted changes")` | Yes (additive exported symbol) |
| `lucind-ai worktree cleanup` flags | `cmd/lucind-ai/cli.go:58,1934-1978` | Parse `--force` / `-f`; fail closed on `ErrWorktreeDirty` when false | Yes (additive flag; unforced dirty removal becomes safe) |
| `printReport` non-done banner | `cmd/lucind-ai/cli.go:698-726` | Append diagnostic inspection steps and reference to `troubleshooting.md:7-18` under non-done banner on stdout | Yes (additive human-readable stdout text) |
| `printIntegrateReport` reverted IDs banner | `cmd/lucind-ai/cli.go:730-740` | When `len(rep.Reverted) > 0`, append retry instructions referencing `recovery-reconciliation.md:33-35` on stdout | Yes (additive human-readable stdout text after `reverted_ids:`) |
| `renderAcceptanceReceipt` review banner | `cmd/lucind-ai/cli.go:685-690` | Append qualitative checklist reminder citing `acceptance-promotion.md:18-30` on stdout | Yes (additive stdout line preserving receipt header keys) |
| `runSplit` multi-wave warning banner | `cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43` | When `len(waves) > 1`, emit warning to stderr to advance checkout and refresh `base_sha` per `recovery-reconciliation.md:27-30` | Yes (stderr routing keeps stdout wave commands pipeable) |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/worktree/worktree.go:247-261` | Modify | Add `force bool` to `Cleanup`/`Remove`, export `ErrWorktreeDirty` (`internal/worktree/worktree.go:26-45`), check `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) when `force == false` | `runWorktreeCleanup` (`cmd/lucind-ai/cli.go:1934-1978`), `DiscardCombined`/`RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:858-869`), `Combine` (`internal/integrate/integrate.go:118-124`), `ResolveCandidate` (`internal/integrate/candidate.go:262-265`) |
| `cmd/lucind-ai/cli.go:58,1934-1978` | Modify | Parse `--force` / `-f` in `runWorktreeCleanup`, pass `force` to `worktree.Cleanup`, output diff guidance and exit 1 on `ErrWorktreeDirty` | Operator executing `lucind-ai worktree cleanup` from terminal or scripts (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`) |
| `cmd/lucind-ai/cli.go:858-869` | Modify | Update `productionDeps` closures `DiscardCombined` and `RemoveLaneWorktree` to pass `force: true` to `worktree.Remove` | `completeIntegration` in `internal/run/integrate.go:159-165` during batch integration |
| `internal/integrate/integrate.go:118-124` | Modify | Pass `force: true` to `worktree.Remove` during merge conflict abort cleanup | `Combine` merge conflict abort handler in `internal/integrate/integrate.go:118-124` |
| `internal/integrate/candidate.go:262-265` | Modify | Pass `force: true` to `worktree.Remove` during candidate promotion teardown | `ResolveCandidate` in `internal/integrate/candidate.go:262-265` invoked by `reconcile resolve` |
| `cmd/lucind-ai/cli.go:485-740` | Modify | Embed guidance banners in `runSplit` (`cmd/lucind-ai/cli.go:485-516`), `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`), `printReport` (`cmd/lucind-ai/cli.go:698-726`), and `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) | Operators and orchestrators inspecting terminal output from `lucind-ai run`, `lucind-ai integrate`, `lucind-ai accept`, and `lucind-ai split` |
| `plugin/claude-code/skills/lucind-ai/` | Modify | Update `troubleshooting.md:7-18`, `recovery-reconciliation.md:27-35`, and `acceptance-promotion.md:18-30` documenting fail-closed worktrees, TDD WIP-rescue, retry without redispatch, and qualitative review | Operators and orchestrators consulting skill references during lane dispatch and failure triage (`plugin/claude-code/skills/lucind-ai/SKILL.md:33-49`) |
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Modify | Add reference to TDD WIP-rescue protocol in `troubleshooting.md:7-18` for inspecting preserved worktrees and salvaging partial progress after timeouts | Apply lane executors dispatched under `lucind-apply` (`.agents/skills/lucind-apply/SKILL.md:10-21`) |

## Open Questions

- [ ] None

## Citation Manifest

| Citation | Claim |
|---|---|
| `.agents/skills/lucind-apply/SKILL.md:10-21` | Strict TDD lifecycle documentation in apply lane skill to be updated with TDD WIP-rescue protocol reference on timeout. |
| `cmd/lucind-ai/cli.go:58` | CLI global usage string defining command signatures, to be updated with `lucind-ai worktree cleanup --lane <id> [--force|-f]`. |
| `cmd/lucind-ai/cli.go:485-516` | `runSplit` subcommand implementation executing `dag.Split`, updated to emit multi-wave stderr warning banner. |
| `cmd/lucind-ai/cli.go:685-690` | `renderAcceptanceReceipt` writing mechanical acceptance output, updated to append qualitative review checklist reminder to stdout. |
| `cmd/lucind-ai/cli.go:698-726` | `printReport` formatting lane execution summary, updated to append diagnostic steps and `troubleshooting.md` reference to stdout on non-done status. |
| `cmd/lucind-ai/cli.go:730-740` | `printIntegrateReport` formatting integration summary, updated to append `integrate retry` instructions to stdout when `reverted_ids` is non-empty. |
| `cmd/lucind-ai/cli.go:858-869` | `productionDeps` defining `DiscardCombined` and `RemoveLaneWorktree` closures, updated to pass `force: true` to `worktree.Remove`. |
| `cmd/lucind-ai/cli.go:1934-1978` | `runWorktreeCleanup` CLI subcommand implementation, updated to parse `--force`/`-f` and fail closed on `ErrWorktreeDirty`. |
| `internal/dag/split.go:34-43` | `dag.Split` loop emitting wave commands to stdout, updated with multi-wave stderr warning banner when `len(waves) > 1`. |
| `internal/integrate/candidate.go:262-265` | `ResolveCandidate` promotion teardown calling `worktree.Remove`, updated to pass `force: true`. |
| `internal/integrate/integrate.go:118-124` | `Combine` merge conflict abort handler calling `worktree.Remove`, updated to pass `force: true`. |
| `internal/run/integrate.go:159-165` | `completeIntegration` calling `deps.RemoveLaneWorktree` for integrated lanes in batch. |
| `internal/worktree/worktree.go:26-45` | Package sentinel error definitions in `worktree` package where `ErrWorktreeDirty` will be declared. |
| `internal/worktree/worktree.go:247-253` | `worktree.Cleanup` implementation checking path existence and delegating to `Remove`. |
| `internal/worktree/worktree.go:247-261` | Current signatures and implementations of `worktree.Cleanup` and `worktree.Remove` to be updated with `force bool`. |
| `internal/worktree/worktree.go:256-261` | `worktree.Remove` implementation executing `git worktree remove --force`. |
| `internal/worktree/worktree.go:319-325` | `PorcelainEmpty` function used to detect uncommitted tracked and untracked changes. |
| `openspec/changes/skill-anchoring-guardrails/proposal.md:1-46` | Accepted proposal Intent, Scope, and Approach defining Candidate 1 guardrails architecture. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:33-49` | Decision gates table mapping runtime situations to skill reference modules. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step acceptance sequence and qualitative checklist referenced by acceptance banner. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:27-30` | Multi-wave sequencing guidance for advancing checkout and refreshing `base_sha` referenced by `split` warning banner. |
| `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md:33-35` | Integration bisection boundary and no-redispatch `integrate retry` procedure referenced by integrate report banner. |
| `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md:7-18` | Troubleshooting symptom-diagnosis table and TDD WIP-rescue procedure referenced by non-done report banner. |
