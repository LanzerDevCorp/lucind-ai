# Proposal: Apply DAG Dispatch Hardening

## Intent

Land the apply-DAG correctness holes that `feature-parent-integration` named as a prerequisite, before that change begins. `dag.Waves` today accepts overlapping packets that share no transitive `depends_on` path across waves, and post-run scope enforcement misses staged-only paths so they land as `lane.Failed` instead of `lane.Deviated`. Close those two confirmed gaps and the five candidate hardening items the exploration already evaluated.

## Scope

Authority for what belongs here is the "Prerequisite hardening" paragraph in `openspec/changes/feature-parent-integration/tasks.md`, plus `openspec/changes/apply-dag-dispatch-hardening/explore.md`'s `file:line` citations. Design of the mechanism is the parallel `design.md`; this proposal names contracts, not implementation.

### In Scope

Every item below traces to a confirmed gap or an evaluated candidate item (3-7).

- **Gap 1 + Item 3 -- global disjointness via transitive reachability.** `internal/dag/waves.go:64-74` calls `packet.DisjointAllowedPaths` only inside the current Kahn wave (`waves.go:30-39,57-61`). Same-wave overlap is rejected (`waves_test.go:94-118`). Cross-wave overlap with a direct `depends_on` edge is allowed (`waves_test.go:196-232`). Cross-wave overlap with no path between the pair is allowed and untested -- the real gap. Item 3 is partial: `depends_on` is used only for Kahn in-degree, never to authorize overlap, and `internal/dag` has no reachability helper. Require every overlapping `allowed_paths` pair to be ordered by transitive DAG reachability; keep ordered overlap (direct or transitive) allowed.
- **Gap 1 emit/CLI constraints.** `dag.EmitPacketContent` drops `depends_on` (`emit.go:11-33`) and `packet.Packet` has no `DependsOn` (`packet.go:32-57`), so `cmd/lucind-ai/cli.go:187` only ever sees one wave after `split`. Validation must happen on the full `dag.DAG` before packets are ever emitted -- not by re-threading edges into packet files. The CLI check and the per-wave check stay two distinct, non-overlapping gates for two distinct entry points (ad-hoc multi-packet dispatch vs. DAG-driven dispatch); this change closes Gap 1 at the DAG-driven entry point.
- **Gap 2 + Item 4 -- staged-only paths in the scope union.** `enforceAllowedPaths` (`internal/run/run.go:458-520`) unions committed-since-base (`run.go:465-474`), unstaged (`run.go:482-488`), and untracked `ls-files -o` (`run.go:490`). There is no `git diff --cached` leg. A staged path that matches the index is invisible, then fails `enforceCompletionMode` porcelain (`run.go:333-338,536-559`) as `lane.Failed` instead of `lane.Deviated`. Existing scope tests stub `PorcelainEmpty=true` (`run_test.go:102-107`). Include index-vs-HEAD in the union. Preserve the `.lucind/` exclusion (`run.go:508-510`).
- **Item 5 -- NUL-delimited git output for the scope union.** All `enforceAllowedPaths` commands run without `-z`; paths are split on `\n` + `TrimSpace` (`run.go:501-516`). Parse NUL-delimited output for every scope-union leg, including the new staged leg, so embedded newlines and significant whitespace are not mis-read.
- **Item 6 -- both git-rename endpoints.** `--name-only --diff-filter=ACDMRT` with default rename detection emits only the destination; the source is never `PathInScope`-checked. Both endpoints must enter the union.
- **Item 7 -- recorded start SHA, not live primary HEAD.** `enforceAllowedPaths` re-resolves live primary `HEAD` at check time (`run.go:465-474`); `worktree.Worktree` stores no birth SHA (`worktree.go:56-58`). `HasUniqueCommits` already uses `git merge-base HEAD <live-HEAD>` (`worktree.go:134-165`) -- a different, more careful notion of "base" than a live re-resolve. Record a birth SHA at worktree creation and reconcile both post-run checks to consume it, preserving `HasUniqueCommits`'s merge-base semantics (do not downgrade it to a plain SHA equality check).
- **Regression coverage.** New tests must construct both confirmed gaps' actual failure cases: unordered cross-wave overlap, and a staged-only out-of-scope path. No existing test does either.

### Out of Scope

- Code-level design of reachability, git command flags, or `Worktree` field shape -- the parallel `design.md`.
- `feature-parent-integration` itself (parent refs, reconciliation, CAS, approval UI).
- Inferring DAG edges or `allowed_paths` from `tasks.md` prose.
- Combine / resolve / bisect behavior.
- Copy (`-C`) detection beyond opportunistically parsing a `C` status if git happens to emit one; not a scoped requirement of this change.
- `PorcelainEmpty`'s `-z` flag: it only tests dirty/clean emptiness, never parses individual paths, so NUL-delimiting it adds no correctness value here.
- **Exploration edge case 6**: a packet dispatched via `lucind-ai run` directly (bypassing `split`) with empty `AllowedPaths` skips both disjointness layers. Distinct from Gap 1 (declared overlapping paths across waves) and from Items 3-7 -- empty `AllowedPaths` is today's documented packet contract for exploratory/read-only dispatch (`packet.go:52-54`), and `split` already refuses empty `allowed_paths` (`validate.go:30-32`). Tightening direct `lucind-ai run` dispatch is a product-policy change, not a fix to either confirmed gap. **Follow-up, not in this change.**
- Writing unsuffixed `proposal.md`/`design.md` is this synthesis step itself, not a lane deliverable.

## Approach and Authority

Internal correctness hardening of shipped apply-DAG dispatch; no user-facing workflow.

Exploration is closed and independently cited (`explore.md`). `feature-parent-integration/tasks.md` already named this change's contents: global transitive-reachability ordering for every overlapping pair; NUL-safe staged diff including index-only paths; both rename endpoints; immutable start SHA. This proposal adopts that list; it does not reopen which candidate item is in scope.

Observable contracts, left to `design.md` for mechanism:

1. Overlap with no transitive `depends_on` path is rejected before dispatch (at `lucind-ai split` time). Overlap with a transitive or direct path stays allowed.
2. A staged-only out-of-scope path is `lane.Deviated`, not `lane.Failed`.
3. The scope union is NUL-safe, rename-endpoint-complete, and compared against a recorded start SHA shared consistently between the scope check and the unique-commits check.
4. `.lucind/` stays excluded from the union.

Split-time validation is the cheapest place to reject unordered overlapping pairs, before any worktree exists. Post-run `enforceAllowedPaths` is where the diff union already lives, so staged/NUL/rename/SHA close there. The existing per-invocation CLI disjointness check (`cli.go:185-189`) is a separate, valid gate for ad-hoc multi-packet dispatch outside a DAG; this change does not fold it into the DAG-aware check or add a third independent layer.

## Capabilities

### New Capabilities

- None. This hardens existing apply-DAG and allowed-paths contracts.

### Modified Capabilities

- `apply-dag-dispatch`: disjointness becomes global over the full DAG. Transitive reachability authorizes overlap; unordered overlap is rejected at `split` time, before packets are emitted.
- `allowed-paths-enforcement`: the scope union includes staged-only paths, NUL-delimited parsing, both rename endpoints, and a recorded start SHA shared with unique-commit detection.
- `completion-mode-enforcement`: contract unchanged, but Gap 2's misroute into a porcelain failure is closed -- a dirty-index out-of-scope path is reported as a scope deviation, not a completion-mode failure.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/dag` (`waves.go`, plus a new reachability helper) | Modified | Global transitive-reachability overlap validation, called before wave/packet emission. |
| `internal/worktree` (`worktree.go`) | Modified | Record birth SHA at worktree creation; reconcile `HasUniqueCommits` to consume it, preserving merge-base semantics. |
| `internal/run` (`run.go`) | Modified | Four-way NUL-delimited diff union (adds a `--cached` leg), both rename endpoints, comparison against the recorded birth SHA instead of live primary `HEAD`. |
| `cmd/lucind-ai/cli.go` | Modified | Wiring for the changed `HasUniqueCommits` signature; per-invocation `DisjointAllowedPaths` unchanged in behavior. |
| Tests (`internal/dag`, `internal/run`, `internal/worktree`) | Modified | New regression coverage for both confirmed gaps and the addressed candidate items. |

## Risks and Rollback

| Risk | Mitigation |
|---|---|
| Over-rejecting legitimately ordered overlap | Keep today's direct-edge allowance (`waves_test.go:196-232`); extend it to transitive reachability, not "any overlap is fatal." |
| `-z` output without a matching parser | Treat Item 5 as one paired command+parser change per leg, not a flag-only edit. |
| Recorded start SHA disagrees with `HasUniqueCommits`' existing merge-base semantics | Item 7 reconciles both checks to the same recorded SHA while preserving `HasUniqueCommits`'s merge-base comparison, not a plain equality check. |
| `.lucind/` envelope writes flagged as a deviation | Preserve `run.go:508-510`'s exclusion when adding the staged leg. |
| Scope creep into edge case 6 (empty-`allowed_paths` direct dispatch) | Explicitly out of scope; tracked as a named follow-up, not silently folded in. |

Rollback is a revert of this change. Per-wave disjointness and the three-way union return. No data migration. Packets already emitted without `depends_on` are unaffected either way, since `depends_on` is never re-threaded into packet files.

## Success Criteria

- [ ] Unordered cross-wave overlapping `allowed_paths` is rejected at `dag.Waves`/`lucind-ai split` time; ordered (direct or transitive) overlap remains allowed.
- [ ] A staged-only out-of-scope path is `lane.Deviated`, not `lane.Failed`.
- [ ] Scope union uses NUL-delimited git output, checks both rename endpoints, and compares against a recorded start SHA shared with `HasUniqueCommits` (merge-base semantics preserved).
- [ ] `.lucind/` remains excluded from the union.
- [ ] New tests construct both confirmed-gap failure cases end to end.
- [ ] `feature-parent-integration` does not start until this change has landed.
