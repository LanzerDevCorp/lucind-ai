# Explore: apply-dag-dispatch-hardening

**Date:** 2026-08-20
**Method:** Real dispatch via `lucind-ai run --packet .lucind/packets/explore-apply-dag-dispatch-hardening.md`, executor `cursor-agent`, `read_only: true`. Result envelope persisted at `.lucind/results/explore-apply-dag-dispatch-hardening.json`. Lane `done`, clean worktree, integrated with 0 reverted.

## Why this change exists

Named as a prerequisite step in `openspec/changes/feature-parent-integration/tasks.md`, before the larger `feature-parent-integration` change begins. Two gaps were first surfaced live during the `apply-dag-dispatch` / `verify-dual-dispatch` verify passes (now archived on `main`). This exploration re-verified both against current code and evaluated five candidate hardening items.

## Confirmed gaps (both real, both unambiguous in code)

### Gap 1 — Disjointness check is per-wave only, not global

`internal/dag/waves.go:64-74` calls `packet.DisjointAllowedPaths` only on the packets within the *current* Kahn wave (built from `depends_on` in-degree, `waves.go:30-39,57-61`). There is no second pass across waves.

- Same-wave overlap: rejected (`waves_test.go:94-118`).
- Cross-wave overlap **with** a direct `depends_on` edge: allowed (`waves_test.go:196-232`) — correct, they're ordered.
- Cross-wave overlap **without** any path between the pair (transitively unordered): **allowed, untested, unvalidated.** This is the real gap.
- `cmd/lucind-ai/cli.go:187` runs a second `DisjointAllowedPaths` per `lucind-ai run` invocation, but `packet.Packet` carries no `DependsOn` field (`packet.go:32-57`) and `dag.EmitPacketContent` drops `depends_on` at emit time (`emit.go:11-33`). Following `split`'s printed per-wave commands, this CLI check only ever sees one wave at a time — it cannot close the gap either.

### Gap 2 — Three-way diff union misses staged-but-uncommitted paths

`internal/run/run.go:458-520` (`enforceAllowedPaths`) unions three git queries: committed-since-base diff, worktree-vs-index diff (unstaged), and untracked (`ls-files -o`). There is **no `git diff --cached`** (index-vs-HEAD / staged-only) leg.

- A path that is staged and matches the index (not further modified) is invisible to all three legs.
- Consequence, confirmed via `run.go:333-338,536-559`: such a lane doesn't get demoted to `deviated` by scope (never seen) — it fails the *later* `enforceCompletionMode` porcelain check instead, landing as `lane.Failed`, not `lane.Deviated`. Existing scope tests stub `PorcelainEmpty=true`, so this path is untested end-to-end.

## Candidate hardening items — evaluation

| # | Item | Status | Real behavior found |
|---|------|--------|----------------------|
| 3 | Transitive-dependency ordering for every overlapping scope pair | **Partial today** | Same-wave pairwise check only; `depends_on` is used solely to compute Kahn in-degree, never consulted to *authorize* an overlap. No reachability helper exists in `internal/dag`. |
| 4 | Explicit staged-only (index vs HEAD) detection | **Missing** | No `git diff --cached`/`--staged` anywhere in `internal/run` or `internal/worktree` production code. |
| 5 | NUL-delimited (`-z`) git output | **Missing** | All three `enforceAllowedPaths` git commands and `PorcelainEmpty`'s `git status --porcelain` run without `-z`. Paths split on `\n` + `TrimSpace` (`run.go:501-516`) — a path containing a newline or significant leading/trailing whitespace is mis-parsed; no test covers it. |
| 6 | Both endpoints of a git rename covered | **Missing** | `--name-only --diff-filter=ACDMRT` includes rename/copy but is not `--name-status`; with git's default rename detection, only the destination path is emitted, so the source path is never `PathInScope`-checked. No rename/copy test exists. |
| 7 | Compare against recorded initial base SHA, not mutable HEAD | **Missing (real TOCTOU)** | `enforceAllowedPaths` re-resolves `git rev-parse HEAD` in the *primary root* live, at check time (`run.go:465-474`) — `worktree.Worktree` (`worktree.go:56-58`) stores no birth SHA. If primary `HEAD` moved between lane creation and lane completion (another integrate, a push), the diff's left side silently shifts. Note: `HasUniqueCommits` (a *different* function, `worktree.go:134-165`) already uses `git merge-base HEAD <live-HEAD>` — so the two post-run checks disagree on what "base" means when primary has moved. |

## Additional edge cases discovered (not in the original candidate list)

1. `depends_on` is dropped at packet emit (`emit.go:11-33`) — the DAG edge information needed to fix Gap 1/Item 3 doesn't survive past `split` today; any fix must either re-thread edges through packets or validate globally before emit.
2. CLI's per-invocation `DisjointAllowedPaths` (`cli.go:185-189`) and DAG's per-wave check are two independent, non-overlapping disjointness checks — neither is DAG-aware.
3. A staged-only out-of-scope file currently produces `lane.Failed` (dirty porcelain), not `lane.Deviated` (scope violation) — the wrong signal for the wrong reason, worth fixing alongside Gap 2.
4. `HasUniqueCommits` (merge-base) and `enforceAllowedPaths` (live HEAD) already use two different notions of "base" — Item 7's fix should reconcile both, not just one.
5. `.lucind/` paths are explicitly excluded from the scope union (`run.go:508-510`) — any staged-only fix must preserve this exclusion.
6. `dag.Validate` requires non-empty `allowed_paths` at `split` time, but `packet.DisjointAllowedPaths` silently skips packets with empty `AllowedPaths` as "undeclared" — a packet dispatched via `lucind-ai run` directly (bypassing `split`) can omit `allowed_paths` and skip both disjointness layers entirely.
7. No existing test constructs the actual failure case for Gap 1 (unordered cross-wave overlap) or Gap 2 (staged-only path) — both need new regression coverage, not just a production fix.

## Full evidence

Complete `file:line`-cited findings (16 entries) are in `.lucind/results/explore-apply-dag-dispatch-hardening.json` and mirrored in Engram (`sdd/apply-dag-dispatch-hardening/explore`).

## Scope boundary for next phases

This exploration only investigated `internal/dag/waves.go`, `internal/run/run.go`, and their direct call graph (`internal/packet`, `internal/dag/emit.go`, `internal/dag/split.go`, `cmd/lucind-ai/cli.go:185-189`, `internal/worktree/worktree.go`). `feature-parent-integration` itself was explicitly out of scope and untouched.
