# Proposal: Apply-DAG Dispatch Hardening

## Intent

Land the apply-DAG correctness holes that `feature-parent-integration` named as a prerequisite, before that change begins. Split today accepts overlapping packets that share no transitive `depends_on` path, and post-run scope enforcement misses staged-only paths so they land as `lane.Failed` instead of `lane.Deviated`. Close those two confirmed gaps and the five candidate items the exploration already evaluated.

## Scope

Authority for what belongs here is the Prerequisite hardening paragraph in `openspec/changes/feature-parent-integration/tasks.md` plus the exploration's `file:line` citations. Design of the fix is the parallel design packet; this proposal only names the contracts.

### In Scope

Every item below traces to a confirmed gap or an evaluated candidate (Items 3–7).

- **Gap 1 + Item 3 — global disjointness via transitive reachability.** `internal/dag/waves.go:64-74` calls `packet.DisjointAllowedPaths` only inside the current Kahn wave (`waves.go:30-39,57-61`). Same-wave overlap is rejected (`waves_test.go:94-118`). Cross-wave overlap with a direct `depends_on` edge is allowed (`waves_test.go:196-232`). Cross-wave overlap with no path between the pair is allowed and untested — the real gap. Item 3 is partial: `depends_on` is used only for Kahn in-degree, never to authorize overlap, and `internal/dag` has no reachability helper. Require every overlapping `allowed_paths` pair to be ordered by transitive DAG reachability; keep ordered overlap allowed.
- **Gap 1 emit/CLI constraints (edges 1–2).** `dag.EmitPacketContent` drops `depends_on` (`emit.go:11-33`) and `packet.Packet` has no `DependsOn` (`packet.go:32-57`), so `cmd/lucind-ai/cli.go:187` only ever sees one wave after `split`. Any Gap 1/Item 3 fix must re-thread edges through packets or validate globally before emit. The CLI check and the per-wave check are two independent, non-DAG-aware layers; they must not remain the only protection.
- **Gap 2 + Item 4 — staged-only paths in the scope union.** `enforceAllowedPaths` (`internal/run/run.go:458-520`) unions committed-since-base (`run.go:465-474`), unstaged (`run.go:482-488`), and untracked `ls-files -o` (`run.go:490`). There is no `git diff --cached` leg. A staged path that matches the index is invisible, then fails `enforceCompletionMode` porcelain (`run.go:333-338,536-559`) as `lane.Failed` instead of `lane.Deviated`. Existing scope tests stub `PorcelainEmpty=true` (`run_test.go:102-107`). Include index-vs-HEAD in the union. Preserve the `.lucind/` exclusion (`run.go:508-510`).
- **Item 5 — NUL-delimited git output.** All three `enforceAllowedPaths` commands and `PorcelainEmpty`'s `git status --porcelain` run without `-z`; paths are split on `\n` + `TrimSpace` (`run.go:501-516`). Parse NUL-delimited output so embedded newlines and significant whitespace are not mis-read.
- **Item 6 — both git-rename endpoints.** `--name-only --diff-filter=ACDMRT` with default rename detection emits only the destination; the source is never `PathInScope`-checked. Both endpoints must be in the union.
- **Item 7 — recorded start SHA, not live primary HEAD.** `enforceAllowedPaths` re-resolves live primary `HEAD` at check time (`run.go:465-474`); `worktree.Worktree` stores no birth SHA (`worktree.go:56-58`). `HasUniqueCommits` already uses `git merge-base HEAD <live-HEAD>` (`worktree.go:134-165`). Compare against a recorded start SHA and reconcile both post-run checks to that same base.
- **Regression coverage (edge 7).** New tests must construct unordered cross-wave overlap and a staged-only out-of-scope path. No existing test does.

### Out of Scope

- Code-level design of reachability, emit-time vs packet-field threading, git command flags, or Worktree field shape — parallel design packet.
- `feature-parent-integration` itself (parent refs, reconciliation, CAS, approval UI).
- Inferring DAG edges or `allowed_paths` from `tasks.md` prose.
- Combine / resolve / bisect behavior.
- Exploration edge case 6: a packet dispatched via `lucind-ai run` with empty `AllowedPaths` skipping both disjointness layers. Distinct from Gap 1 (declared overlapping paths across waves) and from Items 3–7. Follow-up.
- Writing unsuffixed `proposal.md`, `design*.md`, `specs/`, or `tasks.md`.

## Approach and Authority

Internal correctness hardening of shipped apply-DAG dispatch; no user-facing workflow.

Exploration is closed and independently cited. `feature-parent-integration/tasks.md` already named this change's contents: global transitive-reachability ordering for every overlapping pair; NUL staged diff including index-only paths; both rename endpoints; immutable start SHA. This proposal adopts that list; it does not reopen which candidate is in.

Observable contracts, left to design for mechanism:

1. Overlap with no transitive `depends_on` path is rejected before dispatch. Overlap with a transitive path stays allowed.
2. A staged-only out-of-scope path is `lane.Deviated`, not `lane.Failed`.
3. The scope union is NUL-safe, rename-endpoint-complete, and compared against a recorded start SHA shared with `HasUniqueCommits`.
4. `.lucind/` stays excluded.

Split-time validation is the cheapest place to reject unordered overlapping pairs (before worktrees exist). Post-run `enforceAllowedPaths` is where the three-way union already lives, so staged/NUL/rename/SHA close there. Design may retire or DAG-enable the CLI check; do not add a third independent layer.

## Capabilities

### New Capabilities

- None. This hardens existing apply-DAG and allowed-paths contracts.

### Modified Capabilities

- `apply-dag-dispatch`: disjointness is global over the DAG. Transitive reachability authorizes overlap; unordered overlap is rejected. Per-wave pairwise check alone is not sufficient.
- `allowed-paths-enforcement`: scope union includes staged-only paths, NUL-delimited parsing, both rename endpoints, and a recorded start SHA.
- `completion-mode-enforcement`: contract unchanged. Gap 2's misroute into porcelain-failure is closed so a dirty-index out-of-scope path is no longer reported as a completion-mode failure.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/dag` (waves, emit, validate) | Modified | Global overlap/reachability. `depends_on` must survive emit or be validated before it (`waves.go:64-74`, `emit.go:11-33`). |
| `internal/packet` | Maybe | No `DependsOn` today (`packet.go:32-57`). `DisjointAllowedPaths` is not DAG-aware. |
| `cmd/lucind-ai/cli.go` | Maybe | Per-invocation `DisjointAllowedPaths` (`cli.go:185-189`) cannot close Gap 1 after `split`. |
| `internal/run/run.go` | Modified | Staged leg, `-z` parsing, rename endpoints, recorded base (`run.go:458-520`, `run.go:501-516`). |
| `internal/worktree` | Modified | Record start SHA (`worktree.go:56-58`); reconcile `HasUniqueCommits` (`worktree.go:134-165`). |
| Tests (`waves_test.go`, `run_test.go`) | Modified | Regression cases for both confirmed gaps. |

## Risks and Rollback

| Risk | Mitigation |
|------|------------|
| Over-rejecting legitimately ordered overlap | Keep today's direct-edge allowance (`waves_test.go:196-232`); extend it to transitive reachability, not "any overlap is fatal". |
| `-z` flag without a NUL parser | Treat Item 5 as a paired command+parser change (`run.go:501-516`). |
| Start SHA disagrees with `HasUniqueCommits` | Item 7 reconciles both checks to one base (exploration edge 4). |
| `.lucind/` envelope writes flagged as deviation | Preserve `run.go:508-510` when adding the staged leg. |
| CLI vs DAG check drift | Design retires or DAG-enables `cli.go:185-189`; do not add a third layer. |

Rollback is a revert of this change. Per-wave disjointness and the three-way union return. No data migration. Packets already emitted without `depends_on` keep today's CLI check until they are re-split.

## Success Criteria

- [ ] Unordered cross-wave overlapping `allowed_paths` is rejected; ordered (direct or transitive) overlap remains allowed (`waves.go:64-74`, `waves_test.go:94-118,196-232`).
- [ ] A staged-only out-of-scope path is `lane.Deviated`, not `lane.Failed` (`run.go:458-520,333-338,536-559`).
- [ ] Scope union uses NUL-delimited git output (Item 5), checks both rename endpoints (Item 6), and compares against a recorded start SHA shared with `HasUniqueCommits` (Item 7).
- [ ] `.lucind/` remains excluded from the union (`run.go:508-510`).
- [ ] New tests construct both confirmed-gap failure cases (exploration edge 7).
- [ ] `feature-parent-integration` does not start until this change has landed.
