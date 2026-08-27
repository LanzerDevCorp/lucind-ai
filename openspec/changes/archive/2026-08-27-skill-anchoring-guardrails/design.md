# Design: Skill Anchoring & Worktree Cleanup Guardrails

## Technical Approach

Implement Candidate 1 from the accepted proposal (`openspec/changes/skill-anchoring-guardrails/proposal.md:40-46`):
1. **Worktree Cleanup Guardrail**: `worktree.Cleanup` and `worktree.Remove` (`internal/worktree/worktree.go:247-261`) accept `force bool`. When `force` is false, `Remove` checks `PorcelainEmpty` (`:319-325`) and returns `ErrWorktreeDirty` (`:26-45`) without deleting files. Nonexistent paths return `nil` idempotently. `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:58,1934-1978`) parses `--force`/`-f`; on dirty refusal, it outputs porcelain status, diff commands, references `troubleshooting.md:7-18`, and exits 1.
2. **Internal Teardowns**: Automated internal callers pass `force: true`: `DiscardCombined`/`RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:858-869`, `internal/run/integrate.go:159-165`), merge conflict abort (`internal/integrate/integrate.go:118-124`), and `ResolveCandidate` teardown (`internal/integrate/candidate.go:262-265`).
3. **Guidance Banners**: Static banners embedded at four call sites:
   - `printReport` (`cmd/lucind-ai/cli.go:698-726`) on non-done status (`internal/run/run.go:452-465`) appends diagnostics and links `troubleshooting.md:7-18` on stdout.
   - `printIntegrateReport` (`cmd/lucind-ai/cli.go:730-740`) on non-empty `reverted_ids` appends `integrate retry` instructions citing `recovery-reconciliation.md:33-35` on stdout.
   - `renderAcceptanceReceipt` (`cmd/lucind-ai/cli.go:685-690`) on mechanical check (`internal/accept/accept.go:120-130`) appends a reminder for qualitative checklist steps 2–10 in `acceptance-promotion.md:18-30` on stdout.
   - `runSplit` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) on multi-wave DAGs emits a stderr warning to advance checkout and refresh `base_sha`/`expected_parent_sha` per `recovery-reconciliation.md:27-30`.
4. **TDD WIP-Rescue**: Document rescue protocol in `troubleshooting.md:7-18` and `.agents/skills/lucind-apply/SKILL.md:10-21` to inspect preserved worktrees (`internal/worktree/worktree.go:150-159`), commit partial WIP, and re-dispatch.

## Architecture Decisions

### Decision 1 — Signature shape: `force bool` parameter vs. wrapper function

**Choice**: Extend `worktree.Cleanup` and `worktree.Remove` with trailing `force bool` (`internal/worktree/worktree.go:247-261`).
**Alternatives considered**: Helper wrappers (`RemoveForced`/`CleanupForced`), functional options (`WithForce()`), or config struct (`RemoveOptions{Force: bool}`).
**Rationale**: Only four automated deletion sites (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`) and one CLI site (`cmd/lucind-ai/cli.go:1934-1978`). Direct `force bool` enforces compile-time verified deletion semantics without wrapper indirection or struct allocations.
**Terminal consumer**: `runWorktreeCleanup` (`cmd/lucind-ai/cli.go:1934-1978`), `Combine` abort (`internal/integrate/integrate.go:118-124`), `ResolveCandidate` teardown (`internal/integrate/candidate.go:262-265`), `DiscardCombined`/`RemoveLaneWorktree` (`cmd/lucind-ai/cli.go:858-869`, `internal/run/integrate.go:159-165`).

### Decision 2 — Sentinel error placement and shape (`ErrWorktreeDirty`)

**Choice**: Export `var ErrWorktreeDirty = errors.New("worktree: linked worktree has uncommitted changes")` in `internal/worktree/worktree.go:26-45`.
**Alternatives considered**: Custom structured error struct, or unexported error strings matched via substring search.
**Rationale**: Matches sentinel conventions across `internal/worktree` (`ErrEmptyLaneID`, `ErrWorktreeExists`, `ErrEmptyBaseSHA`, `ErrInvalidParentRef`, `ErrInvalidBaseSHA` in `:26-45`). Allows clean `errors.Is(err, worktree.ErrWorktreeDirty)` branching in CLI to format diagnostics and exit 1.
**Terminal consumer**: `runWorktreeCleanup` (`cmd/lucind-ai/cli.go:1934-1978`).

### Decision 3 — Where dirty-check logic lives (inline vs. reusing `PorcelainEmpty`)

**Choice**: Call existing `worktree.PorcelainEmpty(ctx, path)` (`internal/worktree/worktree.go:319-325`) directly within `worktree.Remove` when `force == false`.
**Alternatives considered**: Inlining raw git commands in `worktree.Remove`, or checking in `cmd/lucind-ai` before calling `Cleanup`.
**Rationale**: `PorcelainEmpty` encapsulates `.gitignore`-respecting status inspection with unit tests (`internal/worktree/worktree_test.go:536-595`). Placing the guardrail in `worktree.Remove` enforces fail-closed safety at the core package boundary. Nonexistent trees remain idempotent in `Cleanup` (`internal/worktree/worktree.go:247-261`, `internal/worktree/worktree_test.go:1059-1069`).
**Terminal consumer**: `worktree.Remove` (`internal/worktree/worktree.go:247-261`).

### Decision 4 — Banner call-site strategy and stream routing

**Choice**: Targeted static prints at designated call sites in `cmd/lucind-ai/cli.go` and `internal/dag/split.go`. Route `runSplit` multi-wave warnings to `stderr` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`); append `printReport` (`:698-726`), `printIntegrateReport` (`:730-740`), and `renderAcceptanceReceipt` (`:685-690`) to `stdout` after structured key-value lines.
**Alternatives considered**: Dynamic template registry, reading Markdown from `references/` at CLI runtime, or routing all banners to `stderr`.
**Rationale**: Banner contexts are compile-time static with exact triggers. Direct formatting avoids runtime filesystem reads and latency. Routing `split` warnings to `stderr` keeps stdout wave commands pipeable, while appending report banners to stdout preserves line-prefix parsers (`integrated_ids:`, `acceptance receipt:`) without polluting stderr.
**Terminal consumer**: `printReport` (`cmd/lucind-ai/cli.go:698-726`), `printIntegrateReport` (`:730-740`), `renderAcceptanceReceipt` (`:685-690`), `runSplit` (`:485-516`, `internal/dag/split.go:34-43`).

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

1. **Fail-Closed Deletion**: A dirty worktree containing uncommitted changes (`internal/worktree/worktree.go:319-325`) is never deleted unless `force == true` is explicitly passed.
2. **Internal Automation**: Automated teardowns (`cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165`) always pass `force: true` for disposable trees.
3. **Idempotent Clean Removal**: If a worktree does not exist or is clean, `Cleanup` succeeds returning `nil` regardless of `force` (`internal/worktree/worktree.go:247-253`, `internal/worktree/worktree_test.go:1059-1069`).
4. **Stream Separation**: Guidance banners on `lucind-ai split` route to `stderr` (`cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43`) preserving scriptable stdout; report banners (`cmd/lucind-ai/cli.go:685-740`) append after structured records on `stdout`.

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `worktree.Cleanup` | `internal/worktree/worktree.go:247-253` | Add `force bool`: `Cleanup(ctx, primaryRoot, laneID, force)` | No |
| `worktree.Remove` | `internal/worktree/worktree.go:256-261` | Add `force bool`: `Remove(ctx, primaryRoot, path, force)` | No |
| `worktree.ErrWorktreeDirty` | `internal/worktree/worktree.go:26-45` | Add exported error `var ErrWorktreeDirty = errors.New(...)` | Yes |
| `lucind-ai worktree cleanup` | `cmd/lucind-ai/cli.go:58,1934-1978` | Parse `--force`/`-f`; fail closed on `ErrWorktreeDirty` when false | Yes |
| `printReport` banner | `cmd/lucind-ai/cli.go:698-726` | Append diagnostics and link `troubleshooting.md:7-18` on stdout | Yes |
| `printIntegrateReport` banner | `cmd/lucind-ai/cli.go:730-740` | Append retry instructions citing `recovery-reconciliation.md:33-35` on stdout | Yes |
| `renderAcceptanceReceipt` banner | `cmd/lucind-ai/cli.go:685-690` | Append qualitative reminder citing `acceptance-promotion.md:18-30` on stdout | Yes |
| `runSplit` banner | `cmd/lucind-ai/cli.go:485-516`, `internal/dag/split.go:34-43` | When `len(waves) > 1`, emit warning to stderr citing `recovery-reconciliation.md:27-30` | Yes |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/worktree/worktree.go` | Modify | Add `force bool` to `Cleanup`/`Remove` (`:247-261`), export `ErrWorktreeDirty` (`:26-45`), check `PorcelainEmpty` (`:319-325`) | `runWorktreeCleanup` (`cmd/lucind-ai/cli.go:1934-1978`), `DiscardCombined`/`RemoveLaneWorktree` (`:858-869`), `Combine` (`internal/integrate/integrate.go:118-124`), `ResolveCandidate` (`internal/integrate/candidate.go:262-265`) |
| `cmd/lucind-ai/cli.go` | Modify | Parse `--force`/`-f` in `runWorktreeCleanup` (`:58,1934-1978`); update `productionDeps` closures (`:858-869`) passing `force: true`; embed banners in `runSplit` (`:485-516`), `renderAcceptanceReceipt` (`:685-690`), `printReport` (`:698-726`), `printIntegrateReport` (`:730-740`) | CLI operators; `completeIntegration` in `internal/run/integrate.go:159-165`; operators inspecting output |
| `internal/integrate/integrate.go` | Modify | Pass `force: true` to `worktree.Remove` in merge conflict abort (`:118-124`) | `Combine` abort handler (`:118-124`) |
| `internal/integrate/candidate.go` | Modify | Pass `force: true` to `worktree.Remove` in candidate promotion teardown (`:262-265`) | `ResolveCandidate` in `candidate.go:262-265` |
| `plugin/claude-code/skills/lucind-ai/` | Modify | Update `troubleshooting.md:7-18`, `recovery-reconciliation.md:27-35`, and `acceptance-promotion.md:18-30` | Operators consulting skill references (`plugin/claude-code/skills/lucind-ai/SKILL.md:33-49`) |
| `.agents/skills/lucind-apply/SKILL.md` | Modify | Reference TDD WIP-rescue protocol in `troubleshooting.md:7-18` for apply timeouts (`:10-21`) | Apply lane executors (`:10-21`) |

## Testing Strategy and Test Seams

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Worktree Unit | Signature updates, clean removal, dirty fail-closed, forced override | Update `TestRemove` and `TestCleanupRemovesExistingWorktree` with `force bool`. Assert clean removal succeeds; dirty without force returns `ErrWorktreeDirty` preserving files; dirty with `force: true` succeeds. Assert `TestCleanupOnLaneWithNoWorktreeIsNoOp` returns `nil`. Verify `TestPorcelainEmpty` ignores `.gitignore` files. | `internal/worktree/worktree.go:247-261`, `internal/worktree/worktree_test.go:255-266,536-595,1034-1069` |
| CLI Cleanup | Exit codes, flags, output streams, idempotency | Update `TestWorktreeCleanupCLI`. Assert clean cleanup succeeds without `--force` (exit 0); dirty cleanup without `--force` fails with exit 1, preserves worktree, and prints stderr status; dirty cleanup with `--force`/`-f` removes worktree (exit 0); nonexistent lane cleanup exits 0; missing `--lane` exits nonzero. | `cmd/lucind-ai/cli.go:1934-1978`, `cmd/lucind-ai/cli_test.go:2974-3024` |
| CLI Banners | Banner rendering at milestones | Verify: (1) non-done recovery banner referencing `troubleshooting.md:7-18`; (2) `integrate retry` instructions referencing `recovery-reconciliation.md:33-35` when `reverted_ids` is present; (3) receipt output and qualitative checklist reminder referencing `acceptance-promotion.md:18-30`; (4) parseable stdout wave commands with stderr multi-wave banner. | `cmd/lucind-ai/cli.go:485-516,685-690,698-750`, `cmd/lucind-ai/cli_test.go:685-777,4503-4545`, `internal/dag/split.go:34-43`, `internal/dag/split_test.go:13-111` |
| Internal Integration | Machine caller regression prevention | Verify internal callers pass `force: true` across promotion, conflict recovery, and combined tree teardowns via full check suite (`lucind-checks.sh:1-4`). | `cmd/lucind-ai/cli.go:858-869`, `internal/integrate/candidate.go:262-265`, `internal/integrate/integrate.go:118-124`, `internal/run/integrate.go:159-165`, `lucind-checks.sh:1-4` |

### Test Seams

- **Injectable / Fakeable Today**:
  - `worktree.DefaultGitRunner` (`internal/worktree/worktree.go:47-72`): `GitRunner` allows mocking git commands; `initRepo(t)` (`internal/worktree/worktree_test.go:244,1039`) provides isolated real git repositories.
  - `worktree.PorcelainEmpty` (`internal/worktree/worktree.go:319-325`): Evaluated against filesystem and `.gitignore` (`internal/worktree/worktree_test.go:536-595`).
  - CLI execution seam `run(ctx, args, stdout, stderr)`: In-memory `bytes.Buffer` captures stdout and stderr independently (`cmd/lucind-ai/cli_test.go:2995-2998,4532-4535`).
  - `acceptVerifierFactory` (`cmd/lucind-ai/cli_test.go:4519-4521`): Package hook allows injecting mock acceptance verifier.
  - `dag.Split(dagPath, outDir, stdout)` (`internal/dag/split.go:34-43`): Accepts `io.Writer` directly for stdout verification (`internal/dag/split_test.go:59-63`).
- **New Seams Required**: None. Existing seams provide complete coverage.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: Change manages git worktree cleanup and guidance banners; does not route or execute doc files. | N/A: Not applicable to this change. | None (N/A). |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | `Cleanup` resolves path via `pathFor(primaryRoot, laneID)` (`internal/worktree/worktree.go:150-159`) and verifies linked worktree identity (`:271-292`). `runWorktreeCleanup` checks `resolvePrimaryRoot` and rejects invocation inside linked worktrees (`cmd/lucind-ai/cli.go:1959-1969`). | `TestWorktreeCleanupCLI` verifies rejection inside linked worktree; unit tests verify `Remove` fails cleanly on invalid paths. |
| Commit state | staged, `commit -a`, empty index | Applicable | `PorcelainEmpty` (`internal/worktree/worktree.go:319-325`) evaluates `git status --porcelain`. Any staged edit (`M  `), unstaged modification (` M`), or untracked file (`??`) returns `false`, causing unforced `Cleanup`/`Remove` to fail closed with `ErrWorktreeDirty`. Ignored files remain clean. | `TestRemove` and `TestCleanup` return `ErrWorktreeDirty` preserving files when index has staged, unstaged, or untracked changes (`internal/worktree/worktree_test.go:536-595`); `TestWorktreeCleanupCLI` fails with exit 1 on dirty trees without `--force`. |
| Push state | tracking branch, first push, explicit refspec | N/A: Change does not execute git push or interact with remote refspecs. | N/A: Not applicable to this change. | None (N/A). |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: Change does not construct or invoke pull request commands or platform CLI tools. | N/A: Not applicable to this change. | None (N/A). |

## Rollback and Additivity

**Choice**: Single `git revert` of the merge commit.
**Alternatives considered**: Feature flags / configuration toggles (rejected: adds dead code paths for purely additive safety guardrails); partial branch revert (rejected: worktree signature changes and CLI updates are co-dependent).
**Rationale**: Introduces zero database migrations, SQLite ledger table alterations (`internal/ledger/acceptance.go:41-47`), or result schema changes (`internal/result/result.go:26-34`). Reverting restores prior unconditional cleanup behavior and removes banner messages with zero data migration.

No schema, ledger, or result envelope version moves.

## Open Questions and Out of Scope

### Open Questions

- None (all proposal-era questions resolved by architecture decisions 1–4 and flow invariants).

### Out of Scope

- Background daemon or automatic cron-based garbage collection of stale worktrees.
- Schema modifications to `.lucind/result.json` or `result.schema.json` (`internal/result/result.go:26-34`).
- Automatic git commits created by the binary upon lane timeout or failure (`internal/run/run.go:452-465,906-940`).
- Modifying feature lease acquisition, fencing tokens, or lease renewal in `internal/feature/feature.go:348-350`.
- Altering core acceptance ledger storage or receipt hashing (`internal/accept/accept.go:120-130`, `internal/ledger/acceptance.go:41-47`).
- Re-architecting multi-agent orchestrator adapters outside `plugin/claude-code/skills/lucind-ai/`.
