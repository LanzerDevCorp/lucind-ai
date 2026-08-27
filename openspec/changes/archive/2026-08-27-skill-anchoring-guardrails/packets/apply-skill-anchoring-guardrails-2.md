---
id: apply-skill-anchoring-guardrails-2
executor: agy
routed_by: corrected re-dispatch of apply-skill-anchoring-guardrails, which reached status=deviated because its allowed_paths omitted two pre-existing test files that had to change for the new worktree.Remove/Cleanup signature to compile — allowed_paths is now corrected and the already-completed, already-verified commit is reused via cherry-pick rather than re-implemented
allowed_paths: ["internal/worktree/worktree.go", "internal/worktree/worktree_test.go", "internal/integrate/integrate.go", "internal/integrate/integrate_test.go", "internal/integrate/candidate.go", "internal/run/integrate.go", "internal/run/isolation_test.go", "cmd/lucind-ai/cli.go", "cmd/lucind-ai/cli_test.go", "internal/dag/split.go", "internal/dag/split_test.go", "plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md", "plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md", "plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md", ".agents/skills/lucind-apply/SKILL.md", "openspec/changes/skill-anchoring-guardrails/tasks.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: eb02a8aab103194aa5d2aa9f3d572d5d9b2127bc
expected_parent_sha: eb02a8aab103194aa5d2aa9f3d572d5d9b2127bc
---

# Packet apply-skill-anchoring-guardrails-2

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/apply-skill-anchoring-guardrails-2 · **Branch:** lucind/apply-skill-anchoring-guardrails-2

## Goal

Land the exact, already-completed implementation of the `skill-anchoring-guardrails` change onto this fresh worktree by cherry-picking a known-good commit, then prove it — do not re-implement anything from scratch.

## Why this is safe to dispatch now

The prior packet `apply-skill-anchoring-guardrails` dispatched from this same `base_sha` and produced commit `d71d2f9` on branch `lucind/apply-skill-anchoring-guardrails` (still present in this repository's object store and branch list — do not delete it). That Lane's implementation is complete and correct: it implements every task in `tasks.md` (1.1–3.7). It was demoted from `done` to `deviated` purely because its packet's `allowed_paths` frontmatter omitted two pre-existing test files (`internal/integrate/integrate_test.go`, `internal/run/isolation_test.go`) that necessarily changed — they call the old 3-argument `worktree.Remove(ctx, root, path)` and had to be updated to the new 4-argument signature or the packages would not compile. This packet's `allowed_paths` (above) is corrected to include those two files plus `openspec/changes/skill-anchoring-guardrails/tasks.md` (which the prior Lane checked off as it completed each task — legitimate bookkeeping, not scope creep).

## Preconditions

- Branch `lucind/apply-skill-anchoring-guardrails` exists in this repository with commit `d71d2f9` (or its full SHA, resolvable via `git log --all --oneline | grep d71d2f9` if the short form is ambiguous) as its tip, one commit ahead of this packet's `base_sha` (`eb02a8aab103194aa5d2aa9f3d572d5d9b2127bc`).
- This worktree starts at `base_sha` with none of the target files yet modified.

## What to do

1. In this worktree, run `git log --oneline -1 lucind/apply-skill-anchoring-guardrails` to confirm the commit you are about to cherry-pick and its exact hash.
2. Run `git cherry-pick <that-exact-commit-hash>`. Because this worktree's `base_sha` is the exact same commit that Lane branched from, the cherry-pick should apply cleanly with zero conflicts. If it does NOT apply cleanly, STOP — do not resolve conflicts by guessing; treat conflict markers as a hard stop (see below) and report exactly what conflicted.
3. Confirm the resulting diff touches only files inside this packet's `allowed_paths` (`git diff --stat base_sha..HEAD`) — it should, since the correction above added exactly the two files that caused the previous deviation, plus `tasks.md`.
4. Run every proving command below and confirm they pass. Do not trust that the cherry-picked commit already passes — verify it fresh in this worktree.
5. Write the result envelope per the packet contract below. You are not implementing new logic; you are landing and proving existing, reviewed work, so most of your effort here is verification, not authorship.

## Proving commands (run all, in this worktree)

```
go test ./internal/worktree ./internal/integrate ./internal/run ./internal/dag ./cmd/lucind-ai -count=1
./lucind-checks.sh
```

Also manually verify (describe exact commands and observed output in your envelope):
1. A worktree with an uncommitted modified file: `worktree cleanup` without `--force` fails (exit 1), file preserved.
2. Same, with `--force`: succeeds (exit 0), worktree removed.
3. A non-done `lucind-ai run` lane report shows the troubleshooting banner.
4. A `split` on a 2+ wave DAG prints the multi-wave warning to **stderr**, not stdout.

## Out of scope

Everything the proposal's Out of Scope section names (daemon/cron GC, `.lucind/result.json` schema changes, automatic commits on timeout, feature lease changes, acceptance ledger changes, orchestrator-adapter changes outside `plugin/claude-code/skills/lucind-ai/`). Do not touch any file not listed in `allowed_paths`. Do not "improve" or re-author anything the cherry-picked commit already does — this packet's job is landing and proving that exact commit, not revising it.

## Allowed paths

Exactly the frontmatter `allowed_paths` list above.

## Allowed paths outside the repository

None. Write nothing outside this repository.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The cherry-pick does not apply cleanly (any conflict). Report the exact conflicting file(s) and hunk(s); do not resolve by guessing.
- `lucind/apply-skill-anchoring-guardrails` or its commit `d71d2f9` cannot be found in this worktree's git history.
- Any of the proving commands fail after a clean cherry-pick — this would mean the prior verification was wrong; report exactly what failed.
- The resulting diff touches a path outside `allowed_paths`.
- Any credential value would need to be chosen, generated, or written.

## Done criteria

- [ ] **The cherry-pick applied cleanly**, confirmed by `git status --porcelain` empty and `git log --oneline -2` showing the new commit atop `base_sha`.
- [ ] **`go test ./internal/worktree ./internal/integrate ./internal/run ./internal/dag ./cmd/lucind-ai -count=1` passes** — paste the output.
- [ ] **`./lucind-checks.sh` passes on this worktree's tree.**
- [ ] **The 4 manual verification scenarios above are demonstrated** with exact commands and observed output.
- [ ] **Every introduced indirection names and proves a terminal consumer**: `ErrWorktreeDirty` consumed by `errors.Is` checks in `worktree.Remove`/`runWorktreeCleanup`; `force bool` consumed by every listed call site; `--force`/`-f` consumed by `runWorktreeCleanup`'s flag parsing; each of the four banners consumed by its own printing function and reachable.
- [ ] **The work is committed** (the cherry-picked commit itself, or a commit carrying it), `git status --porcelain` empty, `git log --oneline -1` evidence. Conventional commit, no AI attribution — verify the cherry-picked commit message is still clean; strip any injected `Co-authored-by:` trailer if your tooling added one.

## Context

Change: **skill-anchoring-guardrails**. This is a corrected re-dispatch, not new design work — `proposal.md`, `design.md`, `tasks.md`, and `specs/` in this worktree remain the technical source of truth if you need to double check any behavior the cherry-picked diff implements, but your primary job is landing and proving the known-good commit `d71d2f9`, not re-deriving it. Execution: Isolated Mode, `agy` executor (already decided).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
