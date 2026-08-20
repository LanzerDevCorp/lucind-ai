# Tasks: Apply DAG Dispatch Hardening (cursor-agent)

Strict TDD. Runner: `go test ./...`. Every code item is RED then GREEN. Do not implement GREEN until the named RED test exists and fails for the stated reason.

Canonical sources (read in full before coding): `design.md`, `specs/apply-dag-dispatch/spec.md`, `specs/allowed-paths-enforcement/spec.md`. Spec citations use `specs/<capability>/spec.md#<Requirement name>`. Dispositions in `design.md` are final: fix-now items get tasks; deferred items do not.

## Review Workload Forecast

**This forecast fits `review_budget_lines: 10000`.** No new package, no new CLI subcommand: modifications to ~5 existing production files plus tests. Tests dominate because Strict TDD maps each missing regression in `design.md`'s table onto a failing test (temp-repo git for staged/rename/TOCTOU cases).

| Field | Value |
|-------|-------|
| Estimated changed lines | **750–1200** (impl ~250–380, tests ~480–780, comments ~20–40) |
| `review_budget_lines` | 10000 (`state.yaml`; this packet must not edit it) |
| Over-budget? | **No — under by ~8800–9250 lines.** |
| 400-line work-unit risk | Medium for unit 3 alone (four-way union + new `run_test.go` cases can land ~400–650 if kept as one commit); units 1–2 stay well under |
| Chained PRs recommended | **No** — total is far under budget; split commits, not PRs |
| Suggested split | one PR, three work-unit commits below |
| Delivery strategy | `single-pr` in `state.yaml` — **matches this forecast** |
| Chain strategy | n/a |

Decision needed before apply: **No.** `delivery_strategy: single-pr` is the right shape. This tasks file does not edit `state.yaml`.

**No `apply-dag.yaml` sidecar.** Units 1 and 2+3 are independent axes (DAG overlap vs. recorded-SHA scope union), so a two-node DAG is *possible*, but the whole change is ~750–1200 lines and unit 1 is too small to pay for sidecar orchestration. Sequential apply in one packet, three work-unit commits.

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `ValidateGlobalOverlap` inside `Waves`; delete per-wave `DisjointAllowedPaths` | PR 1 | `go test ./internal/dag ./cmd/lucind-ai -race -count=1` | `lucind-ai split --dag … --out …` on an unordered-overlap fixture (must write zero packet files) | `internal/dag/waves.go` plus whichever of `waves.go` / `overlap.go` / `reachability.go` apply chose; `waves_test.go`; CLI split table row |
| 2 | `Worktree.BaseSHA` + `HasUniqueCommits(ctx, path, baseSHA)` + test-double plumbing | PR 1 | `go test ./internal/worktree ./internal/run ./cmd/lucind-ai -race -count=1` | N/A: real git in `worktree_test.go`; stubs in `run_test.go` | `internal/worktree/worktree.go` field + `Create`/`HasUniqueCommits`; stub signatures in `run_test.go` / `cli_test.go` |
| 3 | Four-way `-z` `--name-status -M` union, `--cached` leg, parsers, `productionDeps` | PR 1 | `go test ./internal/run ./cmd/lucind-ai -race -count=1` | N/A: fakeExecutor + temp git repo (git *is* the spec here) | `internal/run/run.go` scope-check hunk; parsers (`run.go` or `gitpaths.go`); `cli.go` `HasUniqueLaneCommits` closure |

### Constraints (do not add tasks that violate these)

- **No edits** to `internal/packet/packet.go` or `internal/dag/emit.go`. `depends_on` stays dropped at emit; validation happens on `dag.DAG` before `Emit`. Design: Architecture Decisions (rejected re-thread).
- **No edits** to `internal/run/integrate.go`, `internal/resolve/resolve.go`, `internal/integrate/integrate.go`, `internal/run/batch.go`.
- **Keep** CLI `DisjointAllowedPaths` (`cli.go:185-189`) unchanged. It is the ad-hoc `lucind-ai run` gate, not Gap 1's enforcement point. Design: edge case 2.
- **Replace** the per-wave `DisjointAllowedPaths` loop in `Waves` (`waves.go:64-74`); do not keep both checks. Same-wave overlap is unordered overlap by construction.
- **No** `-C` copy detection. Parser may still emit both paths if a `C` status appears. Design: Item 6 defer.
- **No** `-z` on `PorcelainEmpty`. Emptiness-only; not path parsing. Design: Item 5 defer.
- **No** requiring non-empty `allowed_paths` on direct `lucind-ai run`. Exploration edge case 6 is an explicit follow-up (`proposal.md` Out of Scope).
- **Preserve** the `.lucind/` / `.lucind` exclusion (`run.go:508-510`) across all four union legs.
- **Preserve** `HasUniqueCommits` merge-base semantics (`wtHead != mergeBase`). Do not downgrade to `wtHead != baseSHA` equality.
- **Preserve** `TestWaves_CrossWaveOverlapAllowedWithEdge` (`waves_test.go:196-232`): direct-edge overlap stays allowed.
- Exact file placement of `reaches` / `ValidateGlobalOverlap` / `ErrUnorderedOverlap` (`waves.go` vs new `overlap.go` / `reachability.go`) is an **apply-time choice**, not a design mandate. Same for `parseDiffNameStatusZ` / `parseLSFilesZ` (`run.go` vs new `gitpaths.go`). Do not treat either layout as required.

### Sequencing notes (not blockers)

- Unit 1 (`internal/dag`) is independent of units 2–3. Units 2 then 3 are ordered: `Worktree.BaseSHA` must exist before `Execute` threads it.
- `run_test.go` is `package run_test`. Export the unexported parsers through the existing `internal/run/export_test.go` (`package run`) so the parser RED can compile.
- Populate `Worktree.BaseSHA` on every `CreateWorktree` stub that reaches `enforceAllowedPaths` **before** the GREEN that fail-closes on empty SHA. Otherwise every existing scope-check test becomes `Blocked`. Git-free `Execute` tests omit `AllowedPaths` and never hit that gate.
- `setupGitWorktree` (`run_test.go:1490`) adds the worktree, then tests commit into it, then call `newTestDeps`. Birth SHA is HEAD **immediately after add**, before those commits — not HEAD at stub-`Create` time.
- Retarget `TestExecuteScopeCheckGitFailureResolvesToBlocked` (`run_test.go:1944`) away from live primary-root `rev-parse`. That command is deleted.
- Existing `HasUniqueLaneCommits func(context.Context, string)` stubs in `run_test.go` and `cli_test.go` will not compile once the `baseSHA` argument lands; updating them is a named GREEN, not a side effect.

---

## Phase 1: `internal/dag` — global overlap via transitive reachability

Matches File Changes rows: `internal/dag/waves.go` (call `ValidateGlobalOverlap`; delete per-wave loop) and `reaches` / `ValidateGlobalOverlap` / `ErrUnorderedOverlap` (placement is apply-time). Design: Gap 1, Item 3, Architecture Decisions, Global Overlap Validation.

Invariant: every pair whose `AllowedPaths` overlap under `packet.PathInScope` must have a directed `depends_on` path in either direction. Direct edges stay allowed. No path at all is Gap 1.

- [ ] 1.1 RED `internal/dag/waves_test.go`: `TestWaves_CrossWaveOverlapWithoutEdgeRejected` — table with at least (a) spec scenario "Unordered overlap across unrelated waves rejected": packets A, B, C; A and C overlap (`internal/foo/` vs `internal/foo/bar.go`); C `DependsOn: [B]`; B does not depend on A; `dag.Waves` returns an error wrapping `ErrUnorderedOverlap` that names `"A"` and `"C"`; (b) optional 4-packet diamond: A and B disjoint, C depends on A, D depends on B, C and D overlap, no A–D or C–D path → same error naming C and D. Must fail because `Waves` only checks inside a Kahn wave (`waves.go:64-74`) and these pairs are never co-located. Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.2 RED `internal/dag/waves_test.go`: `TestWaves_CrossWaveOverlapAllowedWithTransitiveEdge` — A → B → C (`C.DependsOn: [B]`, `B.DependsOn: [A]`), A and C overlap, no direct A–C edge; `Waves` returns nil error and places A before C on different waves. Must fail because today's allowance is a direct edge only (`TestWaves_CrossWaveOverlapAllowedWithEdge`). Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.3 RED `cmd/lucind-ai/cli_test.go`: add a table row to `TestRunSplitValidationFailuresExit1AndWriteNoFiles` (`cli_test.go:837`) named `unordered cross-wave overlap` whose YAML is the 3-packet case from 1.1; exit code 1, stderr non-empty, `outDir` has zero files. Terminal consumer of `ErrUnorderedOverlap` (`Waves` → `Split` → `runSplit`). Spec: `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 1.4 GREEN `internal/dag`: add `var ErrUnorderedOverlap = errors.New("dag: overlapping allowed_paths without a depends_on path")`; unexported `reaches(dependents map[string][]string, from, to string) bool` (BFS/DFS over the same adjacency `Waves` already builds at `waves.go:28-37`; no transitive-closure cache); `ValidateGlobalOverlap(d DAG) error` walks unordered pairs, uses `packet.PathInScope`, skips nothing on empty `AllowedPaths` (those already fail `dag.Validate` via `ErrEmptyAllowedPaths`), and on overlap without reachability in either direction returns `fmt.Errorf("%w: %q and %q", ErrUnorderedOverlap, a.ID, b.ID)`. Place these in `waves.go` **or** a new `overlap.go` / `reachability.go` — apply-time choice. Spec: same as 1.1.
- [ ] 1.5 GREEN `internal/dag/waves.go`: after Kahn grouping succeeds, **delete** the per-wave `DisjointAllowedPaths` loop (`waves.go:64-74`) and call `ValidateGlobalOverlap(d)` before `Waves` returns. `TestWaves_SameWaveOverlapRejected` still fails (no path). `TestWaves_CrossWaveOverlapAllowedWithEdge` still passes (direct path). `Split` (`split.go:24-30`) needs no new call site. Spec: same as 1.1.

---

## Phase 2: `internal/worktree` — recorded birth SHA (Item 7)

Matches File Changes row: `internal/worktree/worktree.go`. Design: Scope Union and Base SHA — Data structure + Signature changes. Do not rename `Path` / `Branch`. `Create`'s signature stays `(ctx, primaryRoot, laneID) (Worktree, error)`.

- [ ] 2.1 RED `internal/worktree/worktree_test.go`: `TestCreateRecordsBaseSHA` — `Create` on a real temp repo; `wt.BaseSHA` is non-empty and equals `git -C wt.Path rev-parse HEAD`. Must fail because `Worktree` has no `BaseSHA` (`worktree.go:56-59`). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 2.2 GREEN `internal/worktree/worktree.go`: add `BaseSHA string` to `Worktree`. After a successful `git worktree add`, `git -C <path> rev-parse HEAD` records it. If that rev-parse fails, `Create` removes the new worktree and returns the error — fail closed; never return a `Worktree` without a birth SHA. Spec: same as 2.1.
- [ ] 2.3 RED `internal/worktree/worktree_test.go`: `TestHasUniqueCommitsUsesRecordedBaseSHANotLivePrimaryHEAD` — `Create`, capture `wt.BaseSHA`, commit on primary (advance live `HEAD`), then (a) fresh lane still reports no unique commits against the recorded SHA; (b) after a lane commit, unique commits are true against the recorded SHA even though primary moved. `HasUniqueCommits(ctx, wt.Path, wt.BaseSHA)` — third argument is the SHA, not `primaryRoot`. Must fail because today's third argument is `primaryRoot` and the function re-resolves live primary `HEAD` (`worktree.go:137-145`). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 2.4 RED `internal/worktree/worktree_test.go`: `HasUniqueCommits(ctx, path, "")` returns an error naming the missing/empty base SHA; does not `rev-parse` primary. Spec: `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [ ] 2.5 GREEN `internal/worktree/worktree.go`: `func HasUniqueCommits(ctx context.Context, worktreePath, baseSHA string) (bool, error)` — reject empty `baseSHA`; `rev-parse` worktree `HEAD`; `git -C worktreePath merge-base HEAD <baseSHA>`; return `wtHead != mergeBase`. Update existing callers in this package's tests (`TestHasUniqueCommits`, `TestHasUniqueCommitsHonoursCancelledContext`, `TestHasUniqueCommitsWrapsGitFailureWithStderr`) to pass a SHA, not `primaryRoot`. Spec: same as 2.3.

---

## Phase 3: test-double plumbing for `BaseSHA` and `HasUniqueLaneCommits`

Matches File Changes row: signature / `BaseSHA` plumbing in `internal/run/run_test.go`, `internal/run/export_test.go`, `cmd/lucind-ai/cli_test.go`. Named explicitly: not an incidental side effect of Phase 5.

- [ ] 3.1 RED `internal/run/run_test.go`: a scope-check test that reaches `enforceAllowedPaths` (any existing `TestExecuteScopeCheck*` that already uses `setupGitWorktree`) asserts `CreateWorktree`'s returned `Worktree.BaseSHA` is the SHA captured immediately after `setupGitWorktree`, before test commits. Must fail because `newTestDeps` returns `{Path, Branch}` only (`run_test.go:97-98`). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 3.2 GREEN `internal/run/run_test.go`: `setupGitWorktree` (or its callers) captures `git rev-parse HEAD` immediately after `worktree add`. `newTestDeps`'s `CreateWorktree` stub (`run_test.go:97-98`) and every other `CreateWorktree` stub that reaches `enforceAllowedPaths` populate `Worktree.BaseSHA` with that birth SHA. Git-free tests that omit `AllowedPaths` may leave it empty. Spec: same as 3.1.
- [ ] 3.3 GREEN `internal/run/run_test.go` and `cmd/lucind-ai/cli_test.go`: every `HasUniqueLaneCommits` stub and call site grows the `baseSHA string` argument to match `Deps` once Phase 5 changes the signature (`run_test.go` closures at `:102` and the per-test overrides; `cli_test.go` stubs around `:970`, `:1064`, `:1279` and `HasUniqueLaneCommits(ctx, wt.Path)` calls in `TestProductionDepsWiresGitBackedInspectionFuncs` / `TestProductionDepsGitInspectionErrorPropagation`). Pass `wt.BaseSHA` (or the captured birth SHA) through. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.

---

## Phase 4: `internal/run` — NUL `--name-status` parsers (Items 5–6)

Matches File Changes row: `parseDiffNameStatusZ`, `parseLSFilesZ` in `run.go` or a new `gitpaths.go` (apply-time choice). Consumed only by `enforceAllowedPaths`. `run_test.go` is `package run_test` — export via `export_test.go`.

- [ ] 4.1 RED `internal/run/run_test.go`: `TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair` — table: (a) `R100\0old path\0new path\0` yields both `old path` and `new path`; (b) an ordinary `M\0dir/file.go\0` yields one path; (c) a path containing an embedded newline is returned intact, not split; (d) `C100\0src\0dst\0` (copy status appearing without `-C`) yields both paths. Must fail because the helpers do not exist. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 4.2 RED `internal/run/run_test.go`: `parseLSFilesZ` on `a.go\0file with spaces.txt\0dir/\nweird.go\0` yields three paths including the embedded-newline name; empty tokens skipped. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 4.3 GREEN `internal/run`: implement unexported `parseDiffNameStatusZ(output []byte) []string` and `parseLSFilesZ(output []byte) []string` as specified in design.md (ordinary A/C/D/M/T and R without a captured second path → one path; R* and C* → old and new). Place in `run.go` **or** a new `gitpaths.go` — apply-time choice. Export both from `internal/run/export_test.go` so 4.1/4.2 compile. Spec: same as 4.1.

---

## Phase 5: `internal/run/run.go` — four-way union against recorded SHA (Gap 2, Items 4–7)

Matches File Changes row: `internal/run/run.go`. Drop live `git -C deps.PrimaryRoot rev-parse HEAD` (`run.go:465-474`). Thread `baseSHA` into `enforceAllowedPaths` / `enforceCompletionMode` / `Deps.HasUniqueLaneCommits`. Four legs, all `-z`, `--name-status -M` on diffs, `--cached` is the new staged leg. `.lucind/` exclusion unchanged.

- [ ] 5.1 RED `internal/run/run_test.go`: `TestExecuteScopeCheckStagedOnlyPathDetected` — real git worktree; stage an **out-of-scope** file; leave it uncommitted and matching the index (not further modified). Envelope `done`. **Do not** stub `PorcelainEmpty=true` (`run_test.go:102-107` is why this is missed today). Status is `lane.Deviated`, not `lane.Failed`; ledger note names the staged path. Spec: `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`, `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 5.2 RED `internal/run/run_test.go`: `TestExecuteScopeCheckStagedOnlyInScopeReachesDone` — stage an **in-scope** file the same way. Scope check must not `Deviate`. Stub `PorcelainEmpty=true` here so completion mode does not `Fail` a dirty tree; status is `lane.Done`. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 5.3 RED `internal/run/run_test.go`: `TestExecuteScopeCheckRenameSourceAndDestChecked` — rename an out-of-scope path to in-scope (and/or the reverse) with `git mv` so `-M` emits `R*`; envelope `done`; status `lane.Deviated` because the unscoped endpoint is in the union. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 5.4 RED `internal/run/run_test.go`: `TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD` — capture birth SHA after `setupGitWorktree`; commit an out-of-scope path on the lane; then advance primary `HEAD` with an unrelated commit; `CreateWorktree` stub returns the **birth** SHA; Execute still `Deviate`s (committed-since-base leg diffs against `BaseSHA`, not live primary). Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 5.5 RED `internal/run/run_test.go`: extend `TestExecuteScopeCheckForceAddedLucindFileExcludedFromUnion` (`run_test.go:1898`) with a staged-only `.lucind/` file (index-matching, uncommitted) through the new `--cached` leg; status stays `lane.Done`. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"` (unmodified `.lucind/` exclusion across all four legs).
- [ ] 5.6 RED `internal/run/run_test.go`: retarget `TestExecuteScopeCheckGitFailureResolvesToBlocked` (`run_test.go:1944`) — replace the `primaryRoot is not a git repo` subtest (live `rev-parse` is gone) with: (a) `CreateWorktree` returns `BaseSHA: ""` → `lane.Blocked`, diagnosis names the missing base SHA, no live primary `rev-parse` fallback; (b) keep `worktreePath is not a git repo` as a four-way-union git-command failure → `lane.Blocked`. Spec: `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [ ] 5.7 GREEN `internal/run/run.go`: `HasUniqueLaneCommits func(ctx context.Context, worktreePath, baseSHA string) (bool, error)`; `enforceAllowedPaths(ctx, deps, worktreePath, baseSHA, p)` and `enforceCompletionMode(ctx, deps, worktreePath, baseSHA, p)`; `Execute` (`run.go:333-338`) passes `wt.Path, wt.BaseSHA`. Empty `baseSHA` reaching `enforceAllowedPaths` returns `lane.Blocked` ("worktree missing recorded base SHA") — never a live-`rev-parse` fallback. Spec: `specs/allowed-paths-enforcement/spec.md#Git Inspection Failure Blocks, Never Guesses`.
- [ ] 5.8 GREEN `internal/run/run.go`: four-way union inside `enforceAllowedPaths`, all in the lane worktree against `baseSHA`:
  1. `git diff --name-status -z --diff-filter=ACDMRT -M <baseSHA> HEAD`
  2. `git diff --name-status -z --diff-filter=ACDMRT -M` (unstaged)
  3. `git diff --cached --name-status -z --diff-filter=ACDMRT -M` (staged — Gap 2 / Item 4)
  4. `git ls-files -z -o --exclude-standard`
  `-M` explicit; do not add `-C`. `addPaths` consumes the parsers from Phase 4 (skip empty tokens; skip `.lucind` / `.lucind/` prefix; then `PathInScope`). Offending paths still `lane.Deviated`. Existing zero-commit / two-commit / in-scope / untracked tests keep their meaning against the recorded SHA. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.

---

## Phase 6: `cmd/lucind-ai/cli.go` — `productionDeps` signature

Matches File Changes row: `cmd/lucind-ai/cli.go`. Terminal consumer of `HasUniqueCommits(..., baseSHA)` for real `lucind-ai run`.

- [ ] 6.1 RED `cmd/lucind-ai/cli_test.go`: `TestProductionDepsWiresGitBackedInspectionFuncs` (`cli_test.go:667`) calls `HasUniqueLaneCommits(ctx, wt.Path, wt.BaseSHA)` after `worktree.Create`; fresh worktree is still false; after a lane commit, true — against the recorded SHA, not by closing over `primaryRoot`. Must fail until the closure's signature matches. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`.
- [ ] 6.2 GREEN `cmd/lucind-ai/cli.go`: `productionDeps`'s `HasUniqueLaneCommits` closure (`cli.go:508-510`) is `func(ctx, worktreePath, baseSHA string) (bool, error)` and calls `worktree.HasUniqueCommits(ctx, worktreePath, baseSHA)` — drop `primaryRoot`. Spec: same as 6.1.

---

## Phase 7: Testing sweep

- [ ] 7.1 `go test ./... -race -count=1` covers remaining spec scenarios not given new names above: zero-commit untracked in-scope (`TestExecuteScopeCheckZeroCommitsUntrackedInScopeReachesDone`); two-commit earlier out-of-scope (`TestExecuteScopeCheckTwoCommitsEarlierOutOfScopeDemotesToDeviated`); multiple in-scope commits (`TestExecuteScopeCheckMultipleInScopeCommitsReachesDone`); in-scope-only stays Done; out-of-scope tracked/untracked still `Deviated`; git-command failure still `Blocked`; `TestWaves_SameWaveOverlapRejected` still errors; `TestWaves_CrossWaveOverlapAllowedWithEdge` still two waves. Spec: `specs/allowed-paths-enforcement/spec.md#Base-SHA Four-Way Diff Union Defines "Actual Diff"`, `specs/allowed-paths-enforcement/spec.md#Post-Execution Scope Check Demotes Done to Deviated`, `specs/apply-dag-dispatch/spec.md#Cross-Wave Overlap Requires Transitive Dependency Ordering`.
- [ ] 7.2 Confirm `git grep` on the apply diff: **zero** hunks in `internal/packet/packet.go`, `internal/dag/emit.go`, `internal/run/integrate.go`, `internal/resolve/resolve.go`, `internal/integrate/integrate.go`, `internal/run/batch.go`. CLI `DisjointAllowedPaths` call site (`cli.go:185-189`) unchanged. No `-C` added. `PorcelainEmpty` still without `-z`.

---

## Phase 8: Cleanup

- [ ] 8.1 `internal/run/run.go`: replace the "three-way" comment at `run.go:329-332` with four-way against `Worktree.BaseSHA`; state that live primary `HEAD` and `HEAD~1` are both forbidden.
- [ ] 8.2 `internal/dag/split.go`: update the `Split` doc comment (`split.go:13`) from "verifies same-wave path disjointness" to global overlap via `ValidateGlobalOverlap` (transitive reachability) before `Emit`.
- [ ] 8.3 `internal/dag/waves.go`: short comment at the `ValidateGlobalOverlap` call that the per-wave `DisjointAllowedPaths` loop was removed because same-wave overlap is unordered overlap by construction.
- [ ] 8.4 Drop unused helpers introduced during RED.

---

## Out of scope (do not add tasks)

- Exploration edge case 6: empty `allowed_paths` on direct `lucind-ai run` bypasses both disjointness layers (`proposal.md` Out of Scope; design.md additional edge case 6). Follow-up, not this change.
- `PorcelainEmpty -z` (Item 5 defer).
- Explicit `-C` copy detection (Item 6 defer). Opportunistic parse of a `C` status that appears anyway is in scope (Phase 4).
- Re-threading `depends_on` into packet frontmatter / `packet.Packet` / `EmitPacketContent`.
- Deleting or DAG-ifying CLI `DisjointAllowedPaths`.
- Keeping the per-wave `DisjointAllowedPaths` loop alongside `ValidateGlobalOverlap`.
- Inferring DAG edges or `allowed_paths` from `tasks.md` prose.
- `feature-parent-integration` itself.
- Combine / resolve / bisect behavior.
- Writing unsuffixed `tasks.md` or `apply-dag.yaml` (this draft lane; no sidecar — see forecast).
- Any edit under `internal/` or `cmd/` from **this** packet (apply phase only).
