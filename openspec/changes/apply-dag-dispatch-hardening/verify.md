# Verify: apply-dag-dispatch-hardening

**Date:** 2026-08-20
**Overall verdict: PASSED**

## Stage 1 -- Mechanical Check

`lucind-ai check` at commit `f84e085`, exit 0, 12.51s. Full transcript: `openspec/changes/apply-dag-dispatch-hardening/verify-mechanical.log`. Every package `ok`, including `internal/dag`, `internal/run`, `internal/worktree`, `cmd/lucind-ai`.

## Stage 2 -- Dual Qualitative Judgment

Dispatched via real `lucind-ai run` (two `read_only: true` packets, `agy` and `cursor-agent`, dispatched in parallel). Both lanes reached `status: done`, both integrated cleanly with 0 reverted, no AI-attribution trailer. Envelopes: `.lucind/results/verify-apply-dag-dispatch-hardening-{agy,cursor-agent}.json`.

**Unanimous Pass (done/done).** Both lanes independently confirmed: every MODIFIED/RENAMED requirement in `specs/apply-dag-dispatch/spec.md` and `specs/allowed-paths-enforcement/spec.md` is implemented in production, not merely checked off in `tasks.md`; all 10 regression tests named in `design.md`'s "missing tests" table exist and exercise real git repos, real `dag.Waves` output, or real `lane.Status` transitions; every introduced symbol (`ValidateGlobalOverlap`, `reaches`, `Worktree.BaseSHA`, `parseDiffNameStatusZ`, `parseLSFilesZ`) is consumed by a named terminal consumer; and the apply-phase manual fast-forward merge recovery (commits `4aaf2b9`, `b4623d3`, `a4556eb`) was exactly those three work-unit commits with no silent production extras.

## Stage 3 -- Evidence Cross-Checking

Every `file:line` citation below was independently re-verified against the real codebase on `main` at `85e2123`, not accepted on the lanes' word alone.

### Confirmed spec compliance (both lanes agree, independently re-checked)

- **Gap 1 / Item 3 closed at the DAG, not re-threaded into packets.** `internal/dag/overlap.go:54-79` implements `ValidateGlobalOverlap`/`reaches`; `internal/dag/waves.go:65-70` calls it after Kahn grouping, before `Waves` returns, replacing the old per-wave loop. `internal/packet` has no `DependsOn` field (confirmed via grep). `TestWaves_CrossWaveOverlapAllowedWithEdge` (direct), `TestWaves_CrossWaveOverlapAllowedWithTransitiveEdge` (transitive, `waves_test.go:344`), and `TestWaves_CrossWaveOverlapWithoutEdgeRejected` (both the 3-packet and 4-packet diamond case, `waves_test.go:244`) all pass. CLI terminal consumer (`cli_test.go:888-944`) writes zero packet files on rejection.
- **Gap 2 / Item 4 closed with the correct terminal classification.** `TestExecuteScopeCheckStagedOnlyPathDetected` (`run_test.go:1979`) stages an out-of-scope file in a real git worktree, does not stub `PorcelainEmpty=true`, and asserts `lane.Deviated` (not `Failed`) with a ledger note naming the path. `enforceAllowedPaths` runs before `enforceCompletionMode` (`run.go:333-338`), confirmed by direct read.
- **Four-way union, recorded BaseSHA, both rename endpoints -- confirmed in production code, not just tests.** `internal/run/run.go:458-537` runs four legs (committed-since-base, unstaged, staged `--cached`, untracked) all `-z --name-status -M` against `baseSHA`, never a live primary-root `rev-parse`. `internal/worktree/worktree.go:62-97,151-175` records `BaseSHA` at `Create` and `HasUniqueCommits` consumes it via `git merge-base HEAD <baseSHA>` -- **the merge-base semantics are preserved**, confirmed by direct read of `worktree.go:165`; this was the real regression risk identified during design reconciliation (agy's design draft had silently downgraded this to a plain equality check) and it did not make it into the shipped code.
- **`.lucind/` exclusion preserved across all four legs**, confirmed at `run.go:510-511`, and by `TestExecuteScopeCheckForceAddedLucindFileExcludedFromUnion` extended with a staged-only `.lucind/` case (`run_test.go:1927,1946-1950`).
- **Empty/missing `BaseSHA` fails closed to `lane.Blocked`**, never a live-`rev-parse` fallback -- confirmed at `run.go:465-466` and `TestExecuteScopeCheckGitFailureResolvesToBlocked`'s retargeted subtest (`run_test.go:2180-2255`).

### Non-blocking findings (confirmed real, not spec violations, not production defects)

1. **`TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD` (`run_test.go:2135-2175`) does not uniquely prove its own claim.** Independently re-read: the test commits an *out-of-scope* path on the lane, then advances primary `HEAD`, then asserts `Deviated`. That assertion would also pass under the old, buggy live-primary-HEAD implementation, since the lane's own out-of-scope commit is caught either way -- the test never exercises the case that actually distinguishes the two behaviors (an *in-scope-only* lane, plus an unrelated primary-side advance, staying `Done`). Production code is correct regardless (`run.go:334,469` do pass `wt.BaseSHA`); this is a test-design gap, not a shipped defect. The sibling `HasUniqueCommits` TOCTOU test (`worktree_test.go:434`) has the same fixture-shape limitation. **Follow-up, not a blocker**: strengthen both fixtures to actually distinguish recorded-SHA from live-HEAD behavior.
2. **Leftover `strings.TrimSpace` call in `addPaths` (`run.go:506`).** Independently re-read: confirmed present. A holdover from the pre-NUL newline-parsing era; with NUL-delimited parsing the tokens are already exact, so this call is now redundant and could (in the exotic case of a path with meaningful leading/trailing whitespace) strip characters that should be preserved. Does not break the embedded-whitespace scenario the spec actually names (mid-path whitespace survives). **Follow-up, not a blocker**: drop the `TrimSpace` call now that parsing is NUL-delimited.
3. **Whitespace/special-character scenario proven at the parser-unit level only**, not through a full `Execute()` call against a real on-disk path containing a space. Parser tests (`TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair`, `TestParseLSFilesZ`) cover the NUL-split logic directly. **Low-severity follow-up.**
4. **Reverse-rename direction (in-scope -> out-of-scope) is untested**; only out-of-scope -> in-scope is. The spec scenario states the case as "or," so this is additive coverage, not a missing requirement.
5. **Stale doc comment** in `TestProductionDepsWiresGitBackedInspectionFuncs` (`cli_test.go:664-667`) still references closing over `primaryRoot`; the test body itself correctly passes `wt.BaseSHA`. Cosmetic.
6. **`-C` copy detection matches the design's explicit deferral**, not an oversight -- `-M` is passed, `-C` is not, and `parseDiffNameStatusZ` opportunistically parses a `C*` pair if git emits one anyway (`gitpaths.go:30-39`). Matches `tasks.md`'s "Out of scope" list.

None of the above findings dispute a spec requirement, and none were rated as blocking by either judgment lane or by this independent re-check.

## Manual Merge Recovery -- Independently Re-Verified Clean

`state.yaml`'s apply-phase note records that the apply lane's `allowed_paths` omitted `openspec/changes/apply-dag-dispatch-hardening/tasks.md` even though the packet instructed checking off its boxes -- the binary's own allowed-paths-enforcement scope check (the exact capability this change hardens) correctly caught the out-of-scope touch and demoted `done` -> `deviated`. Re-verified directly: `git log 4aaf2b9^..a4556eb` shows exactly the three work-unit commits the apply note describes, each with a plain conventional subject and no `Co-Authored-By`/AI-attribution trailer; `git log a4556eb..HEAD -- internal/ cmd/` is empty (no silent production changes after the merge); `f84e085` only touches `state.yaml`; `85e2123` only adds `verify-mechanical.log`. The demotion was process (a packet-authoring omission), not a code defect.

## Verdict

**PASSED.** All requirements in both delta specs are implemented and tested. Both confirmed gaps (unordered cross-wave overlap; staged-only path misclassification) are closed with regression coverage that exercises the actual failure case. `HasUniqueCommits`'s merge-base semantics -- the specific regression risk flagged during design reconciliation -- shipped correctly, not downgraded. Six non-blocking findings are recorded above as follow-ups; none block this change.

## Follow-ups (not blockers, not tracked as separate SDD changes unless requested)

- Strengthen `TestExecuteScopeCheckUsesRecordedBaseSHANotLivePrimaryHEAD` and its `HasUniqueCommits` sibling to actually distinguish recorded-SHA from live-HEAD behavior.
- Drop the redundant `strings.TrimSpace` call in `internal/run/run.go:506`.
- Optional: an `Execute()`-level test for a real on-disk path containing whitespace; reverse-rename coverage; refresh the stale doc comment in `cli_test.go:664-667`.
- Exploration edge case 6 (empty `allowed_paths` bypassing disjointness on direct `lucind-ai run`) remains an explicit deferred follow-up, unchanged from `proposal.md`.
