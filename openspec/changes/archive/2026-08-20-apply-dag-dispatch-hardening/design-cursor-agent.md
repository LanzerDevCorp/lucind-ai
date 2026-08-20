# Design: Apply DAG Dispatch Hardening

Close two confirmed dispatch holes before `feature-parent-integration` starts: unordered cross-wave path overlap is allowed today, and a staged-only out-of-scope file is invisible to the three-way scope union (it fails porcelain as `lane.Failed` instead of `lane.Deviated`). Validate overlap on the full DAG at `split` time; record the worktree birth SHA and union four NUL-parsed git path lists (committed, staged, unstaged, untracked), checking both rename endpoints, against that SHA.

## Technical Approach

Keep `depends_on` on `dag.Node` only. Add `dag.ValidateGlobalOverlap(d DAG) error`, called from `dag.Waves` after Kahn grouping and before return, so `dag.Split` cannot emit packets until every overlapping pair is ordered by reachability. Do not put `DependsOn` on `packet.Packet`. On the run side, `worktree.Create` records `Worktree.BaseSHA`; `enforceAllowedPaths` and `HasUniqueCommits` both consume that SHA; the scope union gains a `--cached` leg and a shared `--name-status -z` parser. CLI `packet.DisjointAllowedPaths` stays as the ad-hoc same-invocation check — it is not the Gap 1 enforcement point.

## Architecture Decisions

| Choice | Rejected alternative / rationale |
|---|---|
| Validate the whole DAG globally in `dag.Waves` via `ValidateGlobalOverlap` before `Emit` | Re-thread `depends_on` through packet frontmatter so `lucind-ai run` can authorize overlap. Rejected: `packet.Packet` has no `DependsOn` (`packet.go:32-57`), `EmitPacketContent` drops `depends_on` (`emit.go:11-33`), and `split`'s per-wave commands mean `cli.go:187` still only ever sees one wave. The full graph exists only as `dag.DAG` / `dag.Node` (`parse.go:21-36`). |
| `Waves` is the sole caller of `ValidateGlobalOverlap` | Also call it from `Split` (duplicate: `Split` already calls `Waves` at `split.go:24` before `Emit` at `split.go:30`) or fold reachability into the per-wave Kahn loop (mixes grouping with a graph-global invariant that does not need wave identity). |
| Replace the per-wave `DisjointAllowedPaths` loop (`waves.go:64-74`) with the global check | Keep both. Rejected: same-wave overlap *is* unordered overlap (Kahn never places a reachable pair in one wave). A second check would be a duplicate, not a new enforcement point. |
| Record birth SHA on `worktree.Worktree` and thread it through `Execute` | Re-resolve live primary `HEAD` at check time (today's TOCTOU) or store the SHA only in the ledger. Rejected: `enforceAllowedPaths` (`run.go:465-474`) and `HasUniqueCommits` (`worktree.go:134-165`) already disagree about "base" when primary has moved; the SHA must be on the value `Create` returns because that is the moment of birth (`run.go:234`). |
| One `--name-status -z` parser for every scope-union git leg | Fix staged-only with `--name-only` and leave newline splitting. Rejected: the new `--cached` leg would inherit Item 5/6 holes; one parser rewrite covers Gap 2 and Items 4–6 together. |
| Leave CLI `DisjointAllowedPaths` (`cli.go:185-189`) unchanged | Make `run` DAG-aware, or delete the CLI check. Rejected: ad-hoc `lucind-ai run --packet A --packet B` has no DAG, and this check is the only overlap gate before `CreateWorktree`. It cannot close Gap 1; it is the correct gate for a different entry point. |

## Global Overlap Validation

**Invariant:** for every pair of nodes whose `AllowedPaths` overlap under `packet.PathInScope` (`disjoint.go:13-22`), there must be a directed `depends_on` path in either direction. Direct edges are the existing allowed case (`waves_test.go:196-232`); transitive paths are the rest of Item 3; no path is Gap 1.

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

`reaches` BFS/DFS over the same `dependents` adjacency `Waves` already builds (`waves.go:28-37`). DAGs here are tiny; no transitive-closure cache.

`ValidateGlobalOverlap` walks unordered pairs, skips nothing on empty `AllowedPaths` (those already fail `dag.Validate` via `ErrEmptyAllowedPaths`, `validate.go:30-32`), and on overlap without reachability returns `fmt.Errorf("%w: %q and %q", ErrUnorderedOverlap, a.ID, b.ID)` naming the colliding prefixes.

### Call site (terminal consumer)

| New symbol | Call site | Why this site, not a duplicate |
|---|---|---|
| `reaches` | `ValidateGlobalOverlap` only | Needed to *authorize* overlap; `depends_on` is used today only for Kahn in-degree (`waves.go:30-39`), never consulted for overlap. |
| `ValidateGlobalOverlap` | `Waves`, after Kahn succeeds and the per-wave `DisjointAllowedPaths` block is removed, before `return waves, nil` | `Waves` is the current overlap gate (`waves.go:64-74`) and the only function `Split` uses to validate before `Emit` (`split.go:24-30`). CLI `DisjointAllowedPaths` cannot see the DAG. |
| `ErrUnorderedOverlap` | returned through `Waves` → `Split` → `runSplit` (`cli.go:305-307`) | `lucind-ai split` is the terminal consumer: it must fail before writing packet files. |

Do not add `DependsOn` to `packet.Packet`. Do not emit `depends_on` from `EmitPacketContent`.

Existing tests keep their meaning: `TestWaves_SameWaveOverlapRejected` still fails (no path); `TestWaves_CrossWaveOverlapAllowedWithEdge` still passes (direct path). New tests below cover the missing unordered and transitive cases.

## Scope Union and Base SHA

**Invariant:** every path the lane actually changed — committed since birth, staged, unstaged, or untracked, including both ends of a rename — is `PathInScope`-checked against `p.AllowedPaths`, compared to the SHA recorded at `worktree.Create`, not a live primary `HEAD`. Out-of-scope → `lane.Deviated` from `enforceAllowedPaths` (`run.go:333-334`) *before* `enforceCompletionMode` (`run.go:337-338`). `.lucind/` exclusion (`run.go:508-510`) is unchanged.

### Data structure

```go
// internal/worktree/worktree.go — add field, do not rename Path/Branch
type Worktree struct {
    Path    string // absolute path to the worktree directory
    Branch  string // the branch checked out in it
    BaseSHA string // hex SHA of this worktree's HEAD immediately after Create
}
```

`Create` today (`worktree.go:62-82`) runs `git worktree add -b <branch> <path>` with `cmd.Dir = primaryRoot` and returns `{Path, Branch}`. After a successful add, `git -C <path> rev-parse HEAD` records `BaseSHA`. If that rev-parse fails, `Create` removes the new worktree and returns the error — fail closed, no worktree without a birth SHA.

`func Create(ctx context.Context, primaryRoot, laneID string) (Worktree, error)` signature unchanged. `Deps.CreateWorktree` (`run.go:150`) unchanged. Terminal consumer of `BaseSHA`: `Execute` at `run.go:234`, which already holds `wt` and is the only caller that then runs `enforceAllowedPaths` / `enforceCompletionMode`.

### Signature changes

```go
// worktree.go — stop taking primaryRoot; consume the recorded SHA
func HasUniqueCommits(ctx context.Context, worktreePath, baseSHA string) (bool, error)

// run.go Deps — thread the SHA into the injected hook
HasUniqueLaneCommits func(ctx context.Context, worktreePath, baseSHA string) (bool, error)

// run.go — both post-dispatch checks take the recorded SHA
func enforceAllowedPaths(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string)
func enforceCompletionMode(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string)
```

`HasUniqueCommits` rejects empty `baseSHA`, `rev-parse`s worktree `HEAD`, runs `git -C worktreePath merge-base HEAD <baseSHA>`, returns `wtHead != mergeBase`. It no longer `rev-parse`s live primary `HEAD` (`worktree.go:138-145`).

`productionDeps` (`cli.go:508-510`) becomes `return worktree.HasUniqueCommits(ctx, worktreePath, baseSHA)` — drops `primaryRoot` from this hook.

`Execute` (`run.go:333-338`):

```go
status, reason = enforceAllowedPaths(ctx, deps, wt.Path, wt.BaseSHA, p)
status, reason = enforceCompletionMode(ctx, deps, wt.Path, wt.BaseSHA, p)
```

Empty `baseSHA` in `enforceAllowedPaths` → `lane.Blocked` ("worktree missing recorded base SHA"), never a live `rev-parse` fallback. That replaces today's `TestExecuteScopeCheckGitFailureResolvesToBlocked` primary-`rev-parse` failure mode.

### Four-way union inside `enforceAllowedPaths`

Drop the live `git -C deps.PrimaryRoot rev-parse HEAD` (`run.go:465-474`). All four legs run in the lane worktree against `baseSHA`:

| Leg | Command |
|---|---|
| Committed since birth | `git -C wt diff --name-status -z --diff-filter=ACDMRT -M <baseSHA> HEAD` |
| Unstaged (worktree vs index) | `git -C wt diff --name-status -z --diff-filter=ACDMRT -M` |
| Staged (index vs HEAD) — **new, Gap 2 / Item 4** | `git -C wt diff --cached --name-status -z --diff-filter=ACDMRT -M` |
| Untracked | `git -C wt ls-files -z -o --exclude-standard` |

`-M` is explicit so rename detection does not depend on `diff.renames`. Do not add `-C` (copy detection) in this change; if a `C` status appears, parse both paths.

Unexported parsers in `internal/run` (testable via existing `export_test.go`):

```go
// parseDiffNameStatusZ parses `git diff --name-status -z`.
// Ordinary status (A/C/D/M/T and R without a second path) yields one path;
// R* and C* yield old and new. Consumed only by enforceAllowedPaths.
func parseDiffNameStatusZ(output []byte) []string

// parseLSFilesZ parses `git ls-files -z`. Consumed only by enforceAllowedPaths.
func parseLSFilesZ(output []byte) []string
```

`addPaths` (`run.go:501-516`) changes from `Split("\n")` + `TrimSpace` to: take the parser's path list, skip `""`, skip `.lucind` / `.lucind/` prefix (preserve `run.go:508-510`), then `PathInScope`. Offending paths still produce `lane.Deviated` (`run.go:529-530`).

Do not switch `PorcelainEmpty` (`worktree.go:170-171`) to `-z` in this change: it tests emptiness, not path identity.

### Call sites (terminal consumers)

| New / changed symbol | Call site | Why this site |
|---|---|---|
| `Worktree.BaseSHA` | `Execute` (`run.go:234` → `333-338`) | Only `Create` knows birth; only `Execute` runs the two post-dispatch checks that currently disagree about "base". |
| `HasUniqueCommits(..., baseSHA)` | `enforceCompletionMode` via `Deps.HasUniqueLaneCommits` (`run.go:543`) | Completion mode asks "did this lane commit since *birth*", not "vs live primary". |
| `--cached` leg + `parseDiffNameStatusZ` | `enforceAllowedPaths` (`run.go:458-520`) | That is the existing scope union; porcelain (`run.go:548-558`) is the wrong signal (Gap 2 consequence). |
| `parseLSFilesZ` | untracked leg of the same union | Same consumer; newline split is the Item 5 hole. |

## Dispositions

### Confirmed gaps (fix now)

| Gap | Fix |
|---|---|
| 1 — per-wave-only disjointness (`waves.go:64-74`) | `ValidateGlobalOverlap` in `Waves`; remove per-wave `DisjointAllowedPaths`. |
| 2 — staged-only paths missing from the union (`run.go:458-520`) | Fourth `--cached` leg; out-of-scope staged files become `lane.Deviated` before porcelain can mark `lane.Failed`. |

### Candidate items

| # | Item | Disposition |
|---|---|---|
| 3 | Transitive ordering for overlapping pairs | **Fix now.** `ValidateGlobalOverlap` uses reachability, not just a direct `depends_on` edge. Direct-edge allowance stays (`TestWaves_CrossWaveOverlapAllowedWithEdge`). |
| 4 | Staged-only / index-vs-HEAD | **Fix now.** `--cached` leg in `enforceAllowedPaths`. |
| 5 | NUL-delimited git output | **Fix now** for every scope-union leg (including the new staged leg). **Defer** `PorcelainEmpty -z`: emptiness-only, not path parsing. |
| 6 | Both rename endpoints | **Fix now.** `--name-status -z` + `-M`; parser emits old and new. **Defer** `-C` copy detection. |
| 7 | Recorded base SHA vs mutable HEAD | **Fix now.** `Worktree.BaseSHA`; both post-run checks consume it. |

### Additional edge cases

| # | Edge | Disposition |
|---|---|---|
| 1 | `depends_on` dropped at emit (`emit.go:11-33`) | **Keep dropping.** Validation happens on `dag.DAG` before emit. Re-threading is the rejected architecture. |
| 2 | CLI `DisjointAllowedPaths` and DAG per-wave check are independent | **Keep the CLI check** as the ad-hoc same-invocation gate (`cli.go:185-189`, before `CreateWorktree`). It is not Gap 1's enforcement point. |
| 3 | Staged-only out-of-scope currently `lane.Failed` | **Fix now** with Gap 2: scope check runs first and returns `Deviated`. |
| 4 | `HasUniqueCommits` (merge-base vs live HEAD) vs `enforceAllowedPaths` (live HEAD) | **Fix now** with Item 7: both use `BaseSHA`. |
| 5 | `.lucind/` excluded from the union (`run.go:508-510`) | **Preserve** for all four legs. |
| 6 | `dag.Validate` requires `allowed_paths`; `DisjointAllowedPaths` skips empty as undeclared; direct `lucind-ai run` can omit them | **Defer.** Empty `AllowedPaths` is the documented packet contract (`packet.go:52-54`) and the explore/read-only path (`TestExecuteHappyPathEnvelopeDoneReachesLaneDone`, `run_test.go:122-123`). Split already refuses empty (`validate.go:30-32`). Residual risk is only bypassing `split`; that is a product policy change, not either confirmed gap. |
| 7 | No test constructs either gap's actual failure case | **Fix now** — named tests in the next section. |

## File Changes and Test Seams

Implementation (not this packet) touches:

| File | Change |
|---|---|
| `internal/dag/waves.go` | Call `ValidateGlobalOverlap`; delete per-wave `DisjointAllowedPaths` loop. |
| `internal/dag/overlap.go` (new) or `waves.go` | `reaches`, `ValidateGlobalOverlap`, `ErrUnorderedOverlap`. |
| `internal/worktree/worktree.go` | `Worktree.BaseSHA`; `Create` records SHA; `HasUniqueCommits(ctx, worktreePath, baseSHA)`. |
| `internal/run/run.go` | `Deps.HasUniqueLaneCommits` SHA arg; `enforceAllowedPaths` / `enforceCompletionMode` take `baseSHA`; four-way `-z` union. |
| `internal/run/gitpaths.go` (new) or `run.go` | `parseDiffNameStatusZ`, `parseLSFilesZ`. |
| `cmd/lucind-ai/cli.go` | `productionDeps` `HasUniqueLaneCommits` closure matches new signature. |
| `internal/run/run_test.go`, `internal/run/export_test.go`, `internal/worktree/worktree_test.go`, `internal/dag/waves_test.go`, `cmd/lucind-ai/cli.go` test stubs | Signature / `BaseSHA` plumbing. |

Do not modify `internal/packet/packet.go` or `internal/dag/emit.go` in this change.

### Regression tests this exploration showed are missing

| Test | Proves |
|---|---|
| `TestWaves_CrossWaveOverlapWithoutEdgeRejected` | Gap 1: A and B disjoint; C depends on A; D depends on B; C and D overlap (no A–D or C–D path) → `ErrUnorderedOverlap`. |
| `TestWaves_CrossWaveOverlapAllowedWithTransitiveEdge` | Item 3: A → B → C, A and C overlap, no direct A–C edge → two-or-more waves, no error. |
| `TestValidateGlobalOverlap_SameWaveStillRejected` | Replacement of `waves.go:64-74` still rejects the `TestWaves_SameWaveOverlapRejected` fixture (keep that test; this one may be the existing test left in place). |
| `TestExecuteScopeCheckStagedOnlyPathDetected` | Gap 2: stage an out-of-scope file, leave it uncommitted and matching the index → `lane.Deviated`, not `lane.Failed`. Must not stub `PorcelainEmpty=true` (`run_test.go:102-107` is why this is untested today). |
| `TestExecuteScopeCheckStagedOnlyInScopeReachesDone` | Staged-only *in-scope* path is in the union and does not false-Deviate; porcelain still decides Failed vs Done. |
| `TestExecuteScopeCheckRenameSourceAndDestChecked` | Item 6: `git mv` out-of-scope → in-scope (or the reverse) → `Deviated` because the unscoped endpoint is checked. |
| `TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair` | Item 5/6 parser unit: NUL split; `R100\0old\0new\0` yields both paths; a path containing a newline is not split on `\n`. |
| `TestCreateRecordsBaseSHA` | Item 7: `Create` populates `BaseSHA` equal to the new worktree's `HEAD`. |
| `TestHasUniqueCommitsUsesRecordedBaseSHANotLivePrimaryHEAD` | Item 7: advance primary `HEAD` after `Create`; unique-commit answer still compares against birth SHA. |
| `TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD` | Item 7 TOCTOU: move primary `HEAD` after lane birth; committed-since-base still diffs against `BaseSHA`. |
| `TestExecuteScopeCheckForceAddedLucindFileExcludedFromUnion` | Keep existing (`run_test.go:1898`); extend a staged-only `.lucind/` file through the new leg (edge case 5). |

`newTestDeps` (`run_test.go:97-98`) and every `CreateWorktree` stub that can reach `enforceAllowedPaths` must populate `Worktree.BaseSHA`. `TestExecuteScopeCheckGitFailureResolvesToBlocked` (`run_test.go:1944`) must target empty/`rev-parse` failure at `Create` or a failed `git diff <baseSHA> HEAD`, not live primary `rev-parse`.

## Threat Matrix

| Boundary | Applicability; safe/failure behavior; planned RED test |
|---|---|
| Cross-lane file clobbering | Applicable: unordered overlapping `allowed_paths` can run in different Kahn waves with no path between them. Fail closed at `split` (`ValidateGlobalOverlap` → `Waves` → `Split` does not `Emit`). RED: `TestWaves_CrossWaveOverlapWithoutEdgeRejected`. |
| Silent scope-check bypass (staged-only) | Applicable: index-matching staged paths skip all three current legs and fail porcelain as `Failed`. Fail as `Deviated` from `enforceAllowedPaths` before `enforceCompletionMode`. RED: `TestExecuteScopeCheckStagedOnlyPathDetected`. |
| Silent scope-check bypass (rename source) | Applicable: `--name-only` emits only the destination. Both endpoints enter the union. RED: `TestExecuteScopeCheckRenameSourceAndDestChecked`. |
| Silent scope-check bypass (newline in path) | Applicable: `\n` split + `TrimSpace` mis-parses. `-z` parsers. RED: `TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair`. |
| TOCTOU on a moved primary HEAD | Applicable: `enforceAllowedPaths` re-resolves live primary `HEAD`; `HasUniqueCommits` uses live `merge-base`. Both consume `Worktree.BaseSHA`. Empty SHA → `Blocked` / `Create` error, never a live fallback. RED: `TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD`, `TestHasUniqueCommitsUsesRecordedBaseSHANotLivePrimaryHEAD`. |
| `.lucind/` envelope paths | Applicable: exclusion must survive the new staged leg. Skip `.lucind` / `.lucind/` after parse, all four legs. RED: existing lucind exclusion test plus a staged-only `.lucind/` file. |
| Documentation-like / executable classification | N/A: no file-type classification changes. |
| Ad-hoc `lucind-ai run` without `split` | Residual, deferred (edge 6): empty `allowed_paths` still skips both disjointness layers. Not a silent bypass of a declared scope; undeclared remains explicit. No RED in this change. |

## Open Questions

None. The Gap 1 architecture fork (re-thread `depends_on` vs validate the DAG before `Emit`) is resolved in Architecture Decisions: validate on `dag.DAG` inside `Waves`. Item 5 for `PorcelainEmpty` and edge 6 (require `allowed_paths` on every `lucind-ai run`) are deferred with reasons, not left open.
