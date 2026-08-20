# Tasks: Apply DAG Dispatch Hardening

Strict TDD. Runner: `go test ./...`. Every code item is RED then GREEN. Do not implement GREEN until the named RED test exists and fails for the stated reason.

Canonical sources (read in full before coding): `design.md`, `specs/apply-dag-dispatch/spec.md`, `specs/allowed-paths-enforcement/spec.md`. Spec citations use `specs/<capability>/spec.md#<Requirement name>`.

## Review Workload Forecast

This forecast fits well within `review_budget_lines: 10000` (from `state.yaml`). This hardening change addresses two confirmed dispatch holes and candidate hardening items across existing packages (`internal/dag`, `internal/worktree`, `internal/run`, and `cmd/lucind-ai`) with no new package or CLI subcommand. Tests dominate the line count to prove both confirmed-gap failure cases and all regression scenarios.

| Field | Value |
|---|---|
| Estimated changed lines | **500–750** (impl ~160–235, tests ~340–515) |
| `review_budget_lines` | 10000 (`state.yaml`; this packet must not edit it) |
| Over-budget? | **No** — comfortably fits within budget (~9250–9500 lines of headroom). |
| 400-line work-unit risk | Low (all modified files and work units stay well below 400 lines) |
| Chained PRs recommended | **No** — single PR matches `state.yaml`'s `delivery_strategy: single-pr`. |
| Suggested split | Single PR (or 3 logical work units below) |
| Delivery strategy | `single-pr` in `state.yaml` — **consistent with this forecast** |
| Chain strategy | single PR |

Decision needed before apply: **No** — single PR fits well within budget.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `internal/dag`: `reaches` + `ValidateGlobalOverlap` + `Waves` global check | Single PR (Unit 1) | `go test ./internal/dag -race -count=1` | N/A: pure in-memory DAG / table tests | `internal/dag/waves.go` (and optional `reachability.go`/`overlap.go`), `internal/dag/waves_test.go` |
| 2 | `internal/worktree`: `Worktree.BaseSHA` + `HasUniqueCommits(ctx, wt, baseSHA)` | Single PR (Unit 2) | `go test ./internal/worktree -race -count=1` | Real git worktree in `t.TempDir()` | `internal/worktree/worktree.go`, `internal/worktree/worktree_test.go` |
| 3 | `internal/run` & `cmd/lucind-ai`: 4-way NUL union, parsers, `BaseSHA` scope enforcement, CLI wiring | Single PR (Unit 3) | `go test ./internal/run ./cmd/lucind-ai -race -count=1` | Fake executor + git worktree in `t.TempDir()` | `internal/run/run.go` (and optional `gitpaths.go`), `internal/run/run_test.go`, `cmd/lucind-ai/cli.go` |

### Constraints (do not add tasks that violate these)

- **Do not** add `DependsOn` to `packet.Packet` and do not emit `depends_on` from `EmitPacketContent`. Validation happens on `dag.DAG` before emit. Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`, `design.md#Architecture Decisions`.
- **Do not** modify the upfront CLI `packet.DisjointAllowedPaths` check (`cmd/lucind-ai/cli.go:185-189`) — it remains the only overlap gate for ad-hoc `lucind-ai run --packet A --packet B` dispatch outside a DAG. Spec: `design.md#Architecture Decisions`.
- **Do not** modify `PorcelainEmpty` to add `-z` (it is an emptiness-only check, never parses paths). Spec: `design.md#Architecture Decisions`.
- **Do not** add `-C` copy detection flag to git diff commands (opportunistic `C*` status parsing in `parseDiffNameStatusZ` only). Spec: `design.md#Architecture Decisions`.
- **Do not** downgrade `HasUniqueCommits` to a plain equality check (`wtHead != baseSHA`); preserve `git merge-base HEAD <baseSHA>` comparison semantics. Spec: `design.md#Architecture Decisions`.
- **Do not** edit `internal/run/integrate.go`, `internal/resolve/resolve.go`, `internal/integrate/integrate.go`, or `internal/run/batch.go`.
- **Do not** add new ledger columns or event types. Deviation notes reuse existing `ledger.EventLaneNote`.
- **Do not** touch exploration edge case 6 (empty `allowed_paths` direct dispatch) — explicitly deferred follow-up per `proposal.md`.

### Sequencing notes (not blockers)

- Exact file placement for `reaches` and `ValidateGlobalOverlap` (`internal/dag/waves.go` or a new `internal/dag/reachability.go`/`overlap.go`) is an apply-time implementation choice, not a design constraint.
- Exact file placement for `parseDiffNameStatusZ` and `parseLSFilesZ` (`internal/run/run.go` or a new `internal/run/gitpaths.go`) is an apply-time implementation choice, not a design constraint.
- Adding `BaseSHA` to `worktree.Worktree` and changing `HasUniqueCommits`'s signature requires updating `internal/run/run.go`'s `Deps.HasUniqueLaneCommits` signature and `cmd/lucind-ai/cli.go`'s `productionDeps` closure, as well as test doubles in `run_test.go` (`newTestDeps`) and `worktree_test.go`.

---

## Phase 1: `internal/dag` — Transitive reachability and global overlap validation

Matches File Changes row: `internal/dag/waves.go` (and optional `reachability.go`/`overlap.go`), `internal/dag/waves_test.go`.

- [ ] 1.1 RED `internal/dag/waves_test.go`: test unexported reachability helper `reaches(dependents map[string][]string, from, to string) bool` — (a) direct edge `A -> B` returns `true`; (b) transitive chain `A -> B -> C` returns `true` for `from="A", to="C"`; (c) reverse direction `from="C", to="A"` returns `false`; (d) disjoint nodes `from="A", to="D"` returns `false`. Must fail because `reaches` does not exist. Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.2 GREEN `internal/dag/waves.go` (or `reachability.go`): implement `reaches(dependents map[string][]string, from, to string) bool` using BFS/DFS traversal over the `dependents` adjacency map. Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.3 RED `internal/dag/waves_test.go`: test `ValidateGlobalOverlap(d DAG) error` — (a) disjoint `allowed_paths` across all packets returns `nil`; (b) overlapping `allowed_paths` with a direct `depends_on` edge returns `nil`; (c) overlapping `allowed_paths` with a transitive `depends_on` path (e.g. C depends on B, B depends on A, A and C overlap) returns `nil`; (d) overlapping `allowed_paths` without any `depends_on` path in either direction returns `fmt.Errorf("%w: %q and %q", ErrUnorderedOverlap, a.ID, b.ID)`. Must fail because `ValidateGlobalOverlap` and `ErrUnorderedOverlap` do not exist. Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.4 GREEN `internal/dag/waves.go` (or `overlap.go`): define `var ErrUnorderedOverlap = errors.New("dag: overlapping allowed_paths without a depends_on path")` and implement `ValidateGlobalOverlap(d DAG) error` checking all unordered pairs of nodes for component-boundary prefix overlap via `packet.PathInScope`; require `reaches(dependents, a.ID, b.ID) || reaches(dependents, b.ID, a.ID)` for every overlapping pair. Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.5 RED `internal/dag/waves_test.go`: `TestWaves_CrossWaveOverlapWithoutEdgeRejected` — two overlapping packets placed in different Kahn waves with no `depends_on` path between them (e.g. packet C lands in wave 2 only due to depending on unrelated packet B, while overlapping with packet A in wave 1) returns `ErrUnorderedOverlap`. (Regression test from `design.md` table / Gap 1). Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.6 RED `internal/dag/waves_test.go`: `TestWaves_CrossWaveOverlapAllowedWithTransitiveEdge` — three packets where C depends on B, B depends on A, and A and C declare overlapping `allowed_paths` (with no direct edge between A and C) successfully groups into 3 waves with A before C, returning no error. (Regression test from `design.md` table / Item 3). Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.7 GREEN `internal/dag/waves.go`: in `Waves(d DAG)`, replace the per-wave `packet.DisjointAllowedPaths` loop (`waves.go:64-74`) with `ValidateGlobalOverlap(d)` called after Kahn grouping succeeds, before `Waves` returns. Tests 1.5 and 1.6 pass; existing `TestWaves_SameWaveOverlapRejected` and `TestWaves_CrossWaveOverlapAllowedWithEdge` continue to pass. Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.

---

## Phase 2: `internal/worktree` — `Worktree.BaseSHA` record and `HasUniqueCommits` merge-base against recorded SHA

Matches File Changes row: `internal/worktree/worktree.go`, `internal/worktree/worktree_test.go`.

- [ ] 2.1 RED `internal/worktree/worktree_test.go`: `TestCreateRecordsBaseSHA` — calling `worktree.Create(ctx, primaryRoot, "lane1")` returns a `Worktree` struct whose `BaseSHA` field matches the hex SHA returned by `git rev-parse HEAD` in the newly created worktree. Must fail because `Worktree` struct has no `BaseSHA` field. (Regression test from `design.md` table / Item 7). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 2.2 GREEN `internal/worktree/worktree.go`: add `BaseSHA string` to `type Worktree`. In `Create`, immediately after successful `git worktree add`, execute `git -C <path> rev-parse HEAD`. If rev-parse succeeds, populate `BaseSHA: strings.TrimSpace(string(out))`. If rev-parse fails, remove the created worktree via `git worktree remove --force <path>` and return the error (fail closed, never return a worktree without a birth SHA). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [ ] 2.3 RED `internal/worktree/worktree_test.go`: `TestHasUniqueCommitsUsesRecordedBaseSHANotLivePrimaryHEAD` — create worktree, record `BaseSHA`, advance primary `HEAD` with a new commit in the primary repository; (a) `HasUniqueCommits(ctx, wt.Path, wt.BaseSHA)` returns `false` when worktree has no commits of its own; (b) after committing a new change on the lane branch in `wt.Path`, `HasUniqueCommits(ctx, wt.Path, wt.BaseSHA)` returns `true`; (c) verify that `git merge-base HEAD <baseSHA>` comparison semantics are preserved (lane commits reachable from baseSHA report false; commits not reachable from baseSHA report true even after rebase/fast-forward). (Regression test from `design.md` table / Item 7). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 2.4 RED `internal/worktree/worktree_test.go`: `TestHasUniqueCommitsRejectsEmptyBaseSHA` — calling `HasUniqueCommits(ctx, wt.Path, "")` returns an error rather than attempting git execution. Spec: `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [ ] 2.5 GREEN `internal/worktree/worktree.go`: update signature to `func HasUniqueCommits(ctx context.Context, worktreePath, baseSHA string) (bool, error)`. If `baseSHA == ""`, return `errors.New("worktree: baseSHA must not be empty")`. Run `git -C worktreePath rev-parse HEAD` and `git -C worktreePath merge-base HEAD baseSHA`, returning `wtHead != mergeBase`. Update existing calls in `worktree_test.go` to pass `wt.BaseSHA` instead of `primaryRoot`. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.

---

## Phase 3: `internal/run` — NUL-delimited git output parsers (`parseDiffNameStatusZ` and `parseLSFilesZ`)

Matches File Changes row: `internal/run/run.go` (or new `gitpaths.go`), `internal/run/run_test.go` (or `gitpaths_test.go`).

- [ ] 3.1 RED `internal/run/run_test.go`: `TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair` — (a) ordinary status `M\0file1.go\0A\0file2.go\0` yields `["file1.go", "file2.go"]`; (b) rename status `R100\0old/path.go\0new/path.go\0` yields both `["old/path.go", "new/path.go"]`; (c) copy status `C100\0src/path.go\0dst/path.go\0` yields both `["src/path.go", "dst/path.go"]`; (d) path containing embedded newline `M\0path\nwith\nnewline.go\0` is preserved intact as a single path; (e) empty or whitespace-only input returns an empty slice. Must fail because `parseDiffNameStatusZ` does not exist. (Regression test from `design.md` table / Items 5 & 6). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 3.2 GREEN `internal/run/run.go` (or `gitpaths.go`): implement `func parseDiffNameStatusZ(output []byte) []string` tokenizing by NUL byte (`\0`): for ordinary statuses (A, D, M, T), extract 1 path token; for rename (`R*`) and copy (`C*`) statuses, extract both source and destination path tokens. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 3.3 RED `internal/run/run_test.go`: `TestParseLSFilesZ_EmbeddedNewline` — `ls-files -z` output `untracked\nfile.go\0clean.go\0` yields each exact path token without splitting on newline or trimming significant whitespace. Must fail because `parseLSFilesZ` does not exist. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 3.4 GREEN `internal/run/run.go` (or `gitpaths.go`): implement `func parseLSFilesZ(output []byte) []string` splitting bytes on NUL byte (`\0`) and filtering out empty tokens. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.

---

## Phase 4: `internal/run` — Four-way diff union, `BaseSHA` consumption, and scope check enforcement

Matches File Changes row: `internal/run/run.go`, `internal/run/run_test.go`.

- [ ] 4.1 RED `internal/run/run_test.go`: update `Deps.HasUniqueLaneCommits` signature to `func(ctx context.Context, worktreePath, baseSHA string) (bool, error)` in `newTestDeps` (`run_test.go:102`) and populate `Worktree.BaseSHA` in `CreateWorktree` test doubles (`run_test.go:98`). Must fail compilation until GREEN. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 4.2 GREEN `internal/run/run.go`: update `Deps.HasUniqueLaneCommits` in `run.go:162` to take `baseSHA string`. Update `enforceCompletionMode(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet)` signature and pass `baseSHA` into `deps.HasUniqueLaneCommits(ctx, worktreePath, baseSHA)`. In `Execute` (`run.go:338`), pass `wt.BaseSHA` to `enforceCompletionMode`. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 4.3 RED `internal/run/run_test.go`: `TestExecuteScopeCheckStagedOnlyPathDetected` — stage an out-of-scope file in the index (uncommitted, matching index, not further modified), schema-valid `done` envelope -> `Execute` returns `lane.Deviated` with a ledger note naming the out-of-scope path, and does NOT return `lane.Failed` via completion-mode porcelain check. Must not stub `PorcelainEmpty=true`. (Regression test from `design.md` table / Gap 2 / Item 4). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`.
- [ ] 4.4 RED `internal/run/run_test.go`: `TestExecuteScopeCheckStagedOnlyInScopeReachesDone` — stage an in-scope file in the index (uncommitted, matching index) -> four-way union includes it without triggering deviation; lane reaches `lane.Done` (or `lane.Failed` per completion mode). (Regression test from `design.md` table / Gap 2). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`.
- [ ] 4.5 RED `internal/run/run_test.go`: `TestExecuteScopeCheckRenameSourceAndDestChecked` — (a) rename an out-of-scope file to an in-scope path -> returns `lane.Deviated` because rename source is out of scope; (b) rename an in-scope file to an out-of-scope path -> returns `lane.Deviated` because rename destination is out of scope. (Regression test from `design.md` table / Item 6). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`.
- [ ] 4.6 RED `internal/run/run_test.go`: `TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD` — create worktree with recorded `BaseSHA`, advance primary `HEAD` with new commits; lane modifies and commits in-scope file against `BaseSHA` -> scope check diffs against `BaseSHA` (not live primary HEAD) and returns `lane.Done`. (Regression test from `design.md` table / Item 7 TOCTOU). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 4.7 RED `internal/run/run_test.go`: `TestExecuteScopeCheckMissingBaseSHAResolvesToBlocked` — worktree with empty `BaseSHA` reaching `enforceAllowedPaths` returns `lane.Blocked` with diagnosis naming the missing base SHA, never falling back to live primary `HEAD`. Spec: `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [ ] 4.8 RED `internal/run/run_test.go`: update `TestExecuteScopeCheckForceAddedLucindFileExcludedFromUnion` (`run_test.go:1898`) to verify that a staged `.lucind/` file in the new `--cached` leg is excluded from the union across all four legs and does not demote `lane.Done` to `lane.Deviated`. (Regression test from `design.md` table / Edge case 5). Spec: `specs/allowed-paths-enforcement/spec.md#.lucind/ Is Always Excluded From Scope Comparison`, `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 4.9 RED `internal/run/run_test.go`: retarget `TestExecuteScopeCheckGitFailureResolvesToBlocked` (`run_test.go:1944`) to assert that non-zero exit from any of the four git union commands or a missing `BaseSHA` returns `lane.Blocked` with diagnosis, removing the obsolete test case for primary-root `rev-parse` failure. Spec: `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [ ] 4.10 GREEN `internal/run/run.go`: update `enforceAllowedPaths(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string)`:
  - If `baseSHA == ""`, return `lane.Blocked, "worktree missing recorded base SHA"`.
  - Remove live `git -C deps.PrimaryRoot rev-parse HEAD`.
  - Execute four legs in `worktreePath`:
    1. Committed: `git -C wt diff --name-status -z --diff-filter=ACDMRT -M <baseSHA> HEAD`
    2. Unstaged: `git -C wt diff --name-status -z --diff-filter=ACDMRT -M`
    3. Staged (new): `git -C wt diff --cached --name-status -z --diff-filter=ACDMRT -M`
    4. Untracked: `git -C wt ls-files -z -o --exclude-standard`
  - If any git command fails, return `lane.Blocked` with diagnostic reason; never guess `Done` or `Deviated`.
  - Parse legs 1–3 with `parseDiffNameStatusZ` and leg 4 with `parseLSFilesZ`.
  - Filter out `.lucind/` and `.lucind` path prefixes, deduplicate, and evaluate remaining paths with `packet.PathInScope(path, p.AllowedPaths)`.
  - If out-of-scope paths exist, return `lane.Deviated` with reason listing offending paths.
  - In `Execute` (`run.go:334`), pass `wt.BaseSHA` to `enforceAllowedPaths(ctx, deps, wt.Path, wt.BaseSHA, p)`.
  Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`, `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`, `specs/allowed-paths-enforcement/spec.md#.lucind/ Is Always Excluded From Scope Comparison`.

---

## Phase 5: `cmd/lucind-ai` — CLI wiring for `HasUniqueLaneCommits`

Matches File Changes row: `cmd/lucind-ai/cli.go`, `cmd/lucind-ai/cli_test.go`.

- [ ] 5.1 RED `cmd/lucind-ai/cli_test.go`: verify CLI tests compile and run with updated `productionDeps` wiring. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 5.2 GREEN `cmd/lucind-ai/cli.go`: in `productionDeps` (`cli.go:508-510`), update `HasUniqueLaneCommits` closure to `func(ctx context.Context, worktreePath, baseSHA string) (bool, error)` calling `worktree.HasUniqueCommits(ctx, worktreePath, baseSHA)`. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.

---

## Phase 6: Testing sweep

- [ ] 6.1 `go test ./... -race -count=1` covers every unit and package across `internal/dag`, `internal/worktree`, `internal/run`, `cmd/lucind-ai`, confirming zero regressions.
- [ ] 6.2 Confirm `git grep` on the apply diff:
  - Zero modifications to `internal/run/integrate.go`, `internal/resolve/resolve.go`, `internal/integrate/integrate.go`, `internal/run/batch.go`.
  - `packet.Packet` has no `DependsOn` field.
  - `dag.EmitPacketContent` does not emit `depends_on`.
  - Upfront CLI `packet.DisjointAllowedPaths` check (`cli.go:185-189`) remains untouched.
  - `PorcelainEmpty` remains unmodified without `-z`.

---

## Phase 7: Cleanup

- [ ] 7.1 `internal/dag/waves.go`: verify clear comment documenting global reachability overlap validation and why Kahn grouping guarantees same-wave pairs are unordered.
- [ ] 7.2 `internal/worktree/worktree.go`: verify doc comments on `Worktree.BaseSHA` and `HasUniqueCommits` explaining birth-SHA capture and merge-base semantics.
- [ ] 7.3 `internal/run/run.go`: verify doc comments on `enforceAllowedPaths` explaining the four-way NUL union and why live primary `HEAD` re-resolve is forbidden.
- [ ] 7.4 Drop any temporary test helpers or unused intermediate functions introduced during RED phases.

---

## Out of scope (do not add tasks)

- Exploration edge case 6 (empty `allowed_paths` direct dispatch without `split`) — explicit follow-up per `proposal.md`.
- Re-threading `depends_on` through `packet.Packet` or emitted packet frontmatter.
- `-C` (copy detection) flag on git diff commands (opportunistic `C*` status parsing in `parseDiffNameStatusZ` only).
- NUL-delimited `-z` flag on `PorcelainEmpty`.
- Edits to `internal/run/integrate.go`, `internal/resolve/resolve.go`, `internal/integrate/integrate.go`, `internal/run/batch.go`.
- Modifying `apply-dag.yaml` format or writing `apply-dag.yaml`.
- Modifying `tasks.md` (unsuffixed).
- Any change to `openspec/changes/feature-parent-integration/`.
