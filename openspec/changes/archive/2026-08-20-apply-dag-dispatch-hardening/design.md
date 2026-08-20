# Design: Apply DAG Dispatch Hardening

## Technical Approach

Close two confirmed dispatch holes before `feature-parent-integration` starts: unordered cross-wave path overlap is allowed today, and a staged-only out-of-scope file is invisible to the three-way scope union (it fails porcelain as `lane.Failed` instead of `lane.Deviated`). Validate overlap on the full DAG at `dag.Waves` time, before any packet is emitted; record the worktree birth SHA at creation and union four NUL-parsed git path lists (committed, staged, unstaged, untracked) against that SHA, checking both rename endpoints.

Keep `depends_on` on `dag.Node` only -- do not add `DependsOn` to `packet.Packet` and do not emit `depends_on` from `EmitPacketContent`. Add `dag.ValidateGlobalOverlap(d DAG) error`, called from `dag.Waves` after Kahn grouping succeeds, replacing the existing per-wave `packet.DisjointAllowedPaths` loop (same-wave overlap is unordered overlap by construction -- a second, duplicate check adds nothing). `dag.Split` cannot emit packets until every overlapping pair is ordered by reachability. The per-invocation CLI check `packet.DisjointAllowedPaths` (`cmd/lucind-ai/cli.go:185-189`) is left unchanged: it is the only overlap gate for ad-hoc `lucind-ai run --packet A --packet B` dispatch with no DAG, a different entry point that this change does not touch.

On the run side, `worktree.Create` records `Worktree.BaseSHA` at the moment of worktree birth; `enforceAllowedPaths` and `HasUniqueCommits` both consume that recorded SHA instead of a live re-resolve of primary `HEAD`. The scope union gains a `--cached` leg and a shared `--name-status -z -M` parser that yields both endpoints of a rename.

## Architecture Decisions

| Choice | Rejected alternative / rationale |
|---|---|
| Validate the whole DAG globally in `dag.Waves` via `ValidateGlobalOverlap`, before `Split` calls `Emit` | Re-thread `depends_on` through packet frontmatter so `lucind-ai run` can authorize overlap at dispatch time. Rejected: `packet.Packet` has no `DependsOn` (`packet.go:32-57`), `EmitPacketContent` drops `depends_on` at emit (`emit.go:11-33`), and `split`'s per-wave commands mean `cli.go:187` would still only ever see one wave at a time. The full graph exists only as `dag.DAG`/`dag.Node` (`parse.go:21-36`) -- that is where the invariant must be checked. |
| Replace the per-wave `DisjointAllowedPaths` loop (`waves.go:64-74`) with the global check, in the same function | Keep both checks. Rejected: same-wave overlap *is* unordered overlap by construction (Kahn never places a reachable pair in one wave), so a kept second check would be a redundant duplicate, not a distinct enforcement point. |
| Record birth SHA on `worktree.Worktree` at `Create` time and thread it through `Execute` | Re-resolve live primary `HEAD` at check time (today's TOCTOU), or store the SHA only in the ledger. Rejected: `enforceAllowedPaths` (`run.go:465-474`) and `HasUniqueCommits` (`worktree.go:134-165`) already disagree about "base" when primary has moved; the SHA must live on the value `Create` returns, because that call is the actual moment of birth (`run.go:234`). |
| Preserve `HasUniqueCommits`'s existing `git merge-base HEAD <base>` comparison, just against the recorded SHA instead of a live one | Downgrade to a plain `wtHead != baseSHA` equality check. Rejected: merge-base is strictly more correct -- it still answers "does this lane have commits not reachable from base" even if the lane's `HEAD` was rebased or fast-forwarded onto base after birth. A plain equality check is a silent regression, not a simplification. |
| Add `-M` explicitly to every diff leg's git invocation | Rely on git's default `diff.renames` config for rename detection. Rejected: default rename detection is a repository/environment config value, not a code guarantee; making the fix depend on it would make Item 6's coverage non-deterministic across environments. |
| Defer `-C` (copy) detection; opportunistically parse both paths only if a `C` status happens to appear | Add explicit copy detection as part of this change. Rejected: not named in the exploration's confirmed gaps or evaluated items; scope discipline keeps this change to what was actually investigated. |
| Defer NUL-delimiting `PorcelainEmpty`'s `git status --porcelain` | Add `-z` there too, "for consistency." Rejected: `PorcelainEmpty` only tests whether output is empty; it never parses individual path tokens, so `-z` changes nothing observable and is not a fix to anything in scope. |
| Leave CLI `DisjointAllowedPaths` (`cli.go:185-189`) unchanged | Make `run` DAG-aware, or delete the CLI check. Rejected: ad-hoc `lucind-ai run --packet A --packet B` has no DAG; this check is the only overlap gate before `CreateWorktree` for that entry point. It is the correct gate for a different flow, not a duplicate of Gap 1's fix. |
| Defer exploration edge case 6 (empty `allowed_paths` bypasses both disjointness layers on direct `lucind-ai run`) | Require non-empty `allowed_paths` on every `lucind-ai run` dispatch. Rejected: empty `AllowedPaths` is today's documented contract for exploratory/read-only packets (`packet.go:52-54`); `split` already refuses empty `allowed_paths` for DAG-driven dispatch (`validate.go:30-32`). Tightening the direct-dispatch path is a product-policy change outside either confirmed gap. |

## Global Overlap Validation

**Invariant:** for every pair of nodes whose `AllowedPaths` overlap under `packet.PathInScope` (`disjoint.go:13-22`), there must be a directed `depends_on` path in either direction. Direct edges are today's allowed case (`waves_test.go:196-232`); transitive paths are the rest of Item 3; no path at all is Gap 1.

### New API (`internal/dag`)

```go
var ErrUnorderedOverlap = errors.New("dag: overlapping allowed_paths without a depends_on path")

// reaches reports whether to is reachable from from by following DependsOn
// edges (from is an ancestor of to). Unexported; consumed only by
// ValidateGlobalOverlap.
func reaches(dependents map[string][]string, from, to string) bool

// ValidateGlobalOverlap rejects any pair of packets whose allowed_paths
// overlap under packet.PathInScope unless one reaches the other.
func ValidateGlobalOverlap(d DAG) error
```

`reaches` runs BFS/DFS over the same `dependents` adjacency `Waves` already builds (`waves.go:28-37`) -- no separate transitive-closure cache; these DAGs are small. `ValidateGlobalOverlap` walks unordered pairs, skips nothing on empty `AllowedPaths` (those already fail `dag.Validate` via `ErrEmptyAllowedPaths`, `validate.go:30-32`), and on overlap without reachability returns `fmt.Errorf("%w: %q and %q", ErrUnorderedOverlap, a.ID, b.ID)` naming the colliding packet IDs. Exact file placement (extend `waves.go`, or a new `internal/dag/overlap.go`/`reachability.go`) is an apply-time implementation choice, not a design constraint.

### Call sites (terminal consumers)

| Symbol | Call site | Why this site, not a duplicate |
|---|---|---|
| `reaches` | `ValidateGlobalOverlap` only | Needed to *authorize* overlap; `depends_on` is used today only for Kahn in-degree (`waves.go:30-39`), never consulted to authorize overlap. |
| `ValidateGlobalOverlap` | `Waves`, after Kahn grouping succeeds and the per-wave `DisjointAllowedPaths` loop is removed, before `Waves` returns | `Waves` is today's overlap gate (`waves.go:64-74`) and the only function `Split` calls before `Emit` (`split.go:24-30`). CLI `DisjointAllowedPaths` cannot see the full DAG. |
| `ErrUnorderedOverlap` | Propagates `Waves` -> `Split` -> `runSplit` (`cli.go`) | `lucind-ai split` is the terminal consumer: it must fail before writing any packet file. |

Existing tests keep their meaning: `TestWaves_SameWaveOverlapRejected` still fails (no path); `TestWaves_CrossWaveOverlapAllowedWithEdge` still passes (direct path). New tests below cover the missing unordered and transitive cases.

## Scope Union and Base SHA

**Invariant:** every path the lane actually changed -- committed since birth, staged, unstaged, or untracked, including both ends of a rename -- is `PathInScope`-checked against `p.AllowedPaths`, compared to the SHA recorded at `worktree.Create`, never a live primary `HEAD`. Out-of-scope -> `lane.Deviated` from `enforceAllowedPaths` (`run.go:333-334`) *before* `enforceCompletionMode` (`run.go:337-338`). The `.lucind/` exclusion (`run.go:508-510`) is unchanged.

### Data structure

```go
// internal/worktree/worktree.go -- add a field, do not rename Path/Branch
type Worktree struct {
    Path    string // absolute path to the worktree directory
    Branch  string // the branch checked out in it
    BaseSHA string // hex SHA of this worktree's HEAD immediately after Create
}
```

`Create` today (`worktree.go:62-82`) runs `git worktree add -b <branch> <path>` with `cmd.Dir = primaryRoot` and returns `{Path, Branch}`. After a successful add, `git -C <path> rev-parse HEAD` records `BaseSHA`. If that rev-parse fails, `Create` removes the new worktree and returns the error -- fail closed, no worktree is ever returned without a birth SHA. `func Create(ctx context.Context, primaryRoot, laneID string) (Worktree, error)`'s signature is unchanged; `Deps.CreateWorktree` (`run.go:150`) is unchanged.

### Signature changes

```go
// worktree.go -- stop taking primaryRoot; consume the recorded SHA, preserve merge-base semantics
func HasUniqueCommits(ctx context.Context, worktreePath, baseSHA string) (bool, error)

// run.go Deps -- thread the SHA into the injected hook
HasUniqueLaneCommits func(ctx context.Context, worktreePath, baseSHA string) (bool, error)

// run.go -- both post-dispatch checks take the recorded SHA
func enforceAllowedPaths(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string)
func enforceCompletionMode(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string)
```

`HasUniqueCommits` rejects an empty `baseSHA`, `rev-parse`s worktree `HEAD`, runs `git -C worktreePath merge-base HEAD <baseSHA>`, and returns `wtHead != mergeBase` -- preserving today's merge-base semantics, just against the recorded SHA instead of a live re-resolve of primary `HEAD` (`worktree.go:138-145` today). `productionDeps`'s `HasUniqueLaneCommits` closure (`cli.go`) drops `primaryRoot` and passes `baseSHA` through.

`Execute` (`run.go:333-338`) becomes:

```go
status, reason = enforceAllowedPaths(ctx, deps, wt.Path, wt.BaseSHA, p)
status, reason = enforceCompletionMode(ctx, deps, wt.Path, wt.BaseSHA, p)
```

An empty `baseSHA` reaching `enforceAllowedPaths` returns `lane.Blocked` ("worktree missing recorded base SHA") -- never a live-`rev-parse` fallback. This replaces today's `TestExecuteScopeCheckGitFailureResolvesToBlocked` primary-`rev-parse`-failure scenario with a `Create`-time (or missing-`BaseSHA`) failure scenario instead.

### Four-way union inside `enforceAllowedPaths`

Drop the live `git -C deps.PrimaryRoot rev-parse HEAD` (`run.go:465-474`). All four legs run in the lane worktree against `baseSHA`:

| Leg | Command |
|---|---|
| Committed since birth | `git -C wt diff --name-status -z --diff-filter=ACDMRT -M <baseSHA> HEAD` |
| Unstaged (worktree vs index) | `git -C wt diff --name-status -z --diff-filter=ACDMRT -M` |
| Staged (index vs HEAD) -- **new, Gap 2 / Item 4** | `git -C wt diff --cached --name-status -z --diff-filter=ACDMRT -M` |
| Untracked | `git -C wt ls-files -z -o --exclude-standard` |

`-M` is explicit so rename detection does not depend on the `diff.renames` config value. `-C` (copy detection) is not added in this change; if a `C` status appears anyway, the parser still extracts both of its paths.

```go
// parseDiffNameStatusZ parses `git diff --name-status -z` output.
// Ordinary status (A/C/D/M/T, and R without a captured second path) yields
// one path; R* and C* yield both the old and the new path. Consumed only by
// enforceAllowedPaths.
func parseDiffNameStatusZ(output []byte) []string

// parseLSFilesZ parses `git ls-files -z` output. Consumed only by
// enforceAllowedPaths.
func parseLSFilesZ(output []byte) []string
```

`addPaths` (`run.go:501-516`) changes from `Split("\n")` + `TrimSpace` to: take each leg's parsed path list, skip empty tokens, skip the `.lucind`/`.lucind/` prefix (preserving `run.go:508-510`), then `PathInScope`. Offending paths still produce `lane.Deviated` (`run.go:529-530`).

`PorcelainEmpty` (`worktree.go:170-171`) is **not** switched to `-z` in this change: it only tests emptiness, never parses a path, so `-z` there fixes nothing in scope.

### Call sites (terminal consumers)

| Symbol | Call site | Why this site |
|---|---|---|
| `Worktree.BaseSHA` | `Execute` (`run.go:234` -> `333-338`) | Only `Create` knows the birth SHA; only `Execute` runs the two post-dispatch checks that currently disagree about "base." |
| `HasUniqueCommits(..., baseSHA)` | `enforceCompletionMode` via `Deps.HasUniqueLaneCommits` | Completion mode asks "did this lane commit since *birth*," not "vs. live primary." |
| `--cached` leg + `parseDiffNameStatusZ` | `enforceAllowedPaths` (`run.go:458-520`) | That is the existing scope union; porcelain (`run.go:548-558`) is the wrong signal for a scope violation (Gap 2's consequence). |
| `parseLSFilesZ` | untracked leg of the same union | Same consumer; newline splitting there is Item 5's hole. |

## Item and Edge-Case Dispositions

### Confirmed gaps (fix now)

| Gap | Fix |
|---|---|
| 1 -- per-wave-only disjointness (`waves.go:64-74`) | `ValidateGlobalOverlap` inside `Waves`; remove the per-wave `DisjointAllowedPaths` loop. |
| 2 -- staged-only paths missing from the union (`run.go:458-520`) | Fourth `--cached` leg; out-of-scope staged files become `lane.Deviated` before porcelain can mark `lane.Failed`. |

### Candidate items

| # | Item | Disposition |
|---|---|---|
| 3 | Transitive ordering for overlapping pairs | **Fix now.** `ValidateGlobalOverlap` uses reachability, not just a direct `depends_on` edge; the direct-edge allowance stays (`TestWaves_CrossWaveOverlapAllowedWithEdge`). |
| 4 | Staged-only / index-vs-HEAD detection | **Fix now**, via the `--cached` leg in `enforceAllowedPaths`. |
| 5 | NUL-delimited git output | **Fix now** for every scope-union leg, including the new staged leg. **Defer** `PorcelainEmpty -z`: it is emptiness-only, not path parsing. |
| 6 | Both rename endpoints | **Fix now**, via `--name-status -z -M` and a parser that emits both old and new paths. **Defer** `-C` copy detection. |
| 7 | Recorded base SHA vs. mutable HEAD | **Fix now**, via `Worktree.BaseSHA`, consumed by both post-run checks, preserving `HasUniqueCommits`'s merge-base semantics. |

### Additional edge cases (from exploration)

| # | Edge | Disposition |
|---|---|---|
| 1 | `depends_on` dropped at emit (`emit.go:11-33`) | **Keep dropping it.** Validation happens on `dag.DAG` before emit; re-threading is the rejected architecture (see Architecture Decisions). |
| 2 | CLI `DisjointAllowedPaths` and the DAG per-wave check are independent | **Keep the CLI check** as the correct gate for the ad-hoc same-invocation entry point (`cli.go:185-189`, before `CreateWorktree`). It is not Gap 1's enforcement point and is not touched. |
| 3 | Staged-only out-of-scope currently misclassifies as `lane.Failed` | **Fix now**, as a direct consequence of the Gap 2 fix: the scope check runs first and returns `Deviated`. |
| 4 | `HasUniqueCommits` (merge-base) vs. `enforceAllowedPaths` (live HEAD) already disagree | **Fix now**, as a direct consequence of Item 7: both consume `BaseSHA`. |
| 5 | `.lucind/` excluded from the union (`run.go:508-510`) | **Preserve** across all four legs, unchanged. |
| 6 | Empty `allowed_paths` bypasses both disjointness layers on direct `lucind-ai run` | **Defer.** Documented packet contract for exploratory/read-only dispatch (`packet.go:52-54`); `split` already refuses empty `allowed_paths` for DAG-driven dispatch (`validate.go:30-32`). A product-policy change, not either confirmed gap; matches `proposal.md`'s explicit out-of-scope follow-up. |
| 7 | No test constructs either gap's actual failure case | **Fix now** -- named tests below. |

## File Changes and Test Seams

| File | Change |
|---|---|
| `internal/dag/waves.go` | Call `ValidateGlobalOverlap`; delete the per-wave `DisjointAllowedPaths` loop. |
| `internal/dag/waves.go` (or a new `overlap.go`/`reachability.go`) | `reaches`, `ValidateGlobalOverlap`, `ErrUnorderedOverlap`. Exact placement is an apply-time choice. |
| `internal/worktree/worktree.go` | `Worktree.BaseSHA`; `Create` records it; `HasUniqueCommits(ctx, worktreePath, baseSHA)` preserving merge-base semantics. |
| `internal/run/run.go` | `Deps.HasUniqueLaneCommits` gains a `baseSHA` arg; `enforceAllowedPaths`/`enforceCompletionMode` take `baseSHA`; four-way `-z` union; `addPaths` NUL parsing. |
| `internal/run/run.go` (or a new `gitpaths.go`) | `parseDiffNameStatusZ`, `parseLSFilesZ`. |
| `cmd/lucind-ai/cli.go` | `productionDeps`'s `HasUniqueLaneCommits` closure matches the new signature. |
| `internal/dag/waves_test.go`, `internal/run/run_test.go`, `internal/run/export_test.go`, `internal/worktree/worktree_test.go` | New regression coverage; signature/`BaseSHA` plumbing in existing test doubles. |

Do not modify `internal/packet/packet.go` or `internal/dag/emit.go` in this change.

### Regression tests this exploration showed are missing

| Test | Proves |
|---|---|
| `TestWaves_CrossWaveOverlapWithoutEdgeRejected` | Gap 1: two overlapping packets in different waves with no path between them -> `ErrUnorderedOverlap`. |
| `TestWaves_CrossWaveOverlapAllowedWithTransitiveEdge` | Item 3: a transitive (not direct) `depends_on` chain still authorizes overlap -> no error. |
| `TestExecuteScopeCheckStagedOnlyPathDetected` | Gap 2: stage an out-of-scope file, leave it uncommitted and matching the index -> `lane.Deviated`, not `lane.Failed`. Must not stub `PorcelainEmpty=true` (why `run_test.go:102-107` misses this today). |
| `TestExecuteScopeCheckStagedOnlyInScopeReachesDone` | A staged-only *in-scope* path is in the union and does not false-`Deviate`; porcelain still decides `Failed` vs. `Done`. |
| `TestExecuteScopeCheckRenameSourceAndDestChecked` | Item 6: renaming an out-of-scope path to in-scope (or the reverse) -> `Deviated`, because the unscoped endpoint is checked. |
| `TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair` | Item 5/6 parser unit: NUL split; `R100\0old\0new\0` yields both paths; a path containing a newline is not mis-split. |
| `TestCreateRecordsBaseSHA` | Item 7: `Create` populates `BaseSHA` equal to the new worktree's `HEAD`. |
| `TestHasUniqueCommitsUsesRecordedBaseSHANotLivePrimaryHEAD` | Item 7: advance primary `HEAD` after `Create`; the unique-commit answer still compares against the recorded birth SHA via merge-base. |
| `TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD` | Item 7 TOCTOU: move primary `HEAD` after lane birth; the committed-since-base leg still diffs against `BaseSHA`. |
| `TestExecuteScopeCheckForceAddedLucindFileExcludedFromUnion` | Keep existing (`run_test.go:1898`); extend with a staged-only `.lucind/` file through the new leg (edge case 5). |

`newTestDeps` (`run_test.go:97-98`) and every `CreateWorktree` stub reaching `enforceAllowedPaths` must populate `Worktree.BaseSHA`. `TestExecuteScopeCheckGitFailureResolvesToBlocked` (`run_test.go:1944`) must be retargeted at a `Create`-time rev-parse failure or a missing `BaseSHA`, not a live primary-root `rev-parse` failure.

## Threat Matrix

| Boundary | Applicability; safe/failure behavior; planned RED test |
|---|---|
| Cross-lane file clobbering | Applicable: unordered overlapping `allowed_paths` can run in different Kahn waves with no path between them. Fail closed at `split` (`ValidateGlobalOverlap` inside `Waves` means `Split` never reaches `Emit`). RED: `TestWaves_CrossWaveOverlapWithoutEdgeRejected`. |
| Silent scope-check bypass (staged-only) | Applicable: an index-matching staged path skips all three current legs and fails porcelain as `Failed`. Fail as `Deviated` from `enforceAllowedPaths`, before `enforceCompletionMode` runs. RED: `TestExecuteScopeCheckStagedOnlyPathDetected`. |
| Silent scope-check bypass (rename source) | Applicable: `--name-only` emits only the destination. Both endpoints enter the union via `-M --name-status -z`. RED: `TestExecuteScopeCheckRenameSourceAndDestChecked`. |
| Silent scope-check bypass (newline in path) | Applicable: `\n`-split + `TrimSpace` mis-parses. NUL-delimited parsers close it. RED: `TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair`. |
| TOCTOU on a moved primary HEAD | Applicable: `enforceAllowedPaths` re-resolves live primary `HEAD`; `HasUniqueCommits` used live `merge-base`. Both now consume `Worktree.BaseSHA`; an empty SHA fails closed (`Blocked`/`Create` error), never a live fallback. RED: `TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD`, `TestHasUniqueCommitsUsesRecordedBaseSHANotLivePrimaryHEAD`. |
| `.lucind/` envelope paths | Applicable: the exclusion must survive the new staged leg. Skip `.lucind`/`.lucind/` after parsing, on all four legs. RED: existing exclusion test, extended with a staged-only `.lucind/` case. |
| Documentation-like / executable classification | N/A: no file-type classification changes. |
| Ad-hoc `lucind-ai run` without `split` | Residual, deferred (edge case 6): empty `allowed_paths` still skips both disjointness layers there. Not a silent bypass of a *declared* scope -- undeclared stays explicit. No RED in this change. |

## Open Questions

None. The Gap 1 architecture fork (re-thread `depends_on` into packets vs. validate the full DAG before `Emit`) is resolved above: validate on `dag.DAG` inside `Waves`. `PorcelainEmpty -z`, `-C` copy detection, and edge case 6 (requiring `allowed_paths` on every `lucind-ai run`) are deferred with stated reasons, not left open.
