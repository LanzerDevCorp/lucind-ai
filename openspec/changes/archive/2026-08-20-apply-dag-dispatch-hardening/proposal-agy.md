# Proposal: Apply DAG Dispatch Hardening

## Intent

Harden `apply-dag-dispatch` and `allowed-paths-enforcement` by fixing two confirmed correctness gaps and implementing five candidate hardening improvements identified during exploration. This ensures that cross-wave packet file overlaps are globally validated through transitive DAG dependencies, staged-but-uncommitted mutations are correctly captured and classified as scope deviations rather than porcelain failures, git command output parsing is newline- and whitespace-safe, renames are checked on both endpoints, and scope diffs evaluate against immutable birth base SHAs to eliminate TOCTOU races.

## Scope

### In Scope

- **Global Transitive DAG Scope Disjointness (Gap 1 & Item 3)**:
  - Address the per-wave disjointness gap in `internal/dag/waves.go:64-74` (which currently uses `waves.go:30-39,57-61` to compute Kahn waves from in-degrees and validates disjointness only within each wave, leaving transitively unordered cross-wave overlaps untested and unvalidated per `waves_test.go:94-118,196-232`).
  - Introduce global DAG reachability validation ensuring that any two packets in the DAG with overlapping `allowed_paths` possess a transitive dependency path (`depends_on`).
  - Resolve the packet emission dropping of dependency metadata in `internal/dag/emit.go:11-33` and reconcile the CLI check in `cmd/lucind-ai/cli.go:185-189` with `internal/packet/packet.go:32-57` so validation is fully DAG-aware.
- **Staged-Only Diff Detection (Gap 2 & Item 4)**:
  - Extend the three-way diff union in `internal/run/run.go:458-520` (which currently unions committed diffs `run.go:465-474`, unstaged worktree diffs `run.go:482-488`, and untracked files `run.go:490`) to include a fourth staged-only leg (`git diff --cached`).
  - Ensure staged out-of-scope mutations correctly demote lane status to `lane.Deviated` via `enforceAllowedPaths` instead of falling through to `enforceCompletionMode` (`run.go:333-338,536-559`) as misleading `lane.Failed` errors.
  - Preserve the `.lucind/` internal path exclusions (`run.go:508-510`) across all diff legs.
- **NUL-Delimited Git Output Parsing (Item 5)**:
  - Convert git status and diff invocations in `internal/run/run.go:458-520` and porcelain checks to use `-z` (NUL-delimited output), replacing newline/`TrimSpace` line splitting (`run.go:501-516`) to safely handle file paths with whitespace and special characters.
- **Dual-Endpoint Rename & Copy Scope Enforcement (Item 6)**:
  - Switch git diff queries from `--name-only` to `--name-status` or explicit source/destination inspection, validating that both source and destination paths of renames and copies satisfy `PathInScope`.
- **Immutable Base SHA vs Mutable HEAD Reconciliation (Item 7)**:
  - Record the birth base SHA in `internal/worktree/worktree.go:56-58` at worktree creation time, preventing TOCTOU races in `internal/run/run.go:465-474` where live primary `HEAD` is re-resolved after branch creation.
  - Reconcile the base SHA reference models between `HasUniqueCommits` (`internal/worktree/worktree.go:134-165`) and `enforceAllowedPaths` (`internal/run/run.go:465-474`).
- **Comprehensive Regression Test Coverage**:
  - Add test fixtures constructing unordered cross-wave overlaps and staged-only out-of-scope files (`internal/run/run_test.go:102-107`, `internal/dag/waves_test.go`).

### Out of Scope

- Implementing the subsequent `feature-parent-integration` change (this change is strictly the prerequisite hardening).
- Modifying the 6-value `lane.Status` enum or altering completion mode enforcement rules outside of correcting scope demotion classification.
- Re-architecting the SQLite ledger or execution barrier subsystems.

## Approach and Authority

The hardening builds directly on the established foundations of `apply-dag-dispatch` and `allowed-paths-enforcement`:

1. **DAG Reachability Authority**:
   - `internal/dag` becomes the single authority for validating packet dependency topologies.
   - Global pairwise disjointness is enforced during DAG validation: for every pair of packets $(A, B)$ sharing overlapping `allowed_paths`, DAG reachability ($A \rightsquigarrow B$ or $B \rightsquigarrow A$) must exist.
   - Validation occurs prior to packet emission (`internal/dag/emit.go`), ensuring split packets represent verified, deterministic execution units.

2. **Accurate Scope Enforcement and Classification**:
   - `internal/run/run.go` unions four distinct git queries using `-z` delimiters:
     1. Committed commits since recorded worktree base SHA.
     2. Staged changes vs HEAD (`git diff --cached -z`).
     3. Unstaged worktree changes vs index (`git diff -z`).
     4. Untracked files (`git ls-files -o -z`).
   - Rename/copy entries yield both source and destination paths for validation.
   - Any path outside declared `allowed_paths` (excluding `.lucind/`) triggers scope deviation before porcelain checks run.

3. **Immutable Base Reference**:
   - Worktree birth commits are captured during checkout/creation and stored in `worktree.Worktree`, eliminating sensitivity to concurrent primary branch updates.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `apply-dag-dispatch`: Enforces global transitive DAG reachability for all overlapping scope pairs across waves, and guarantees dependency/DAG awareness before packet emission.
- `allowed-paths-enforcement`: Enforces comprehensive 4-way diff unioning (including staged-only `git diff --cached`), NUL-delimited parsing (`-z`), dual-endpoint rename validation, and immutable birth-base SHA comparison to eliminate TOCTOU races and incorrect lane failure classification.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/dag/` (`waves.go`, `emit.go`, `split.go`, `waves_test.go`) | Modified | Transitive reachability ordering, global cross-wave scope disjointness check, packet emission edge preservation, and regression tests. |
| `internal/run/` (`run.go`, `run_test.go`) | Modified | 4-way diff union with staged-only (`--cached`), NUL-delimited `-z` parsing, dual-endpoint rename checking, and regression tests. |
| `internal/worktree/` (`worktree.go`, `worktree_test.go`) | Modified | Recording birth base SHA at creation time; reconciling base SHA resolution between `HasUniqueCommits` and scope checks. |
| `cmd/lucind-ai/` (`cli.go`) | Modified | DAG-aware disjointness and execution validation. |

## Risks and Rollback

| Risk | Mitigation |
|---|---|
| Transitive reachability check rejects previously permissible unordered DAGs | Clear validation errors specifying the conflicting packet pair, their overlapping paths, and the required dependency edge. |
| Staged diff queries misclassify internal engine artifacts as deviations | Explicit exclusion of `.lucind/` paths preserved across all diff query legs (`run.go:508-510`). |
| Incompatible base SHA if worktree creation base is rewritten | Base SHA recorded from initial checkout commit before any mutations occur; fallback to merge-base when upstream is rebased. |

Rollback is achieved by reverting to the per-wave check in `internal/dag/waves.go` and the 3-way diff union in `internal/run/run.go`.

## Success Criteria

- [ ] All cross-wave packet pairs with overlapping `allowed_paths` without a transitive `depends_on` path are rejected at DAG validation time.
- [ ] Staged-but-uncommitted out-of-scope modifications are detected and accurately demoted to `lane.Deviated` instead of failing porcelain checks as `lane.Failed`.
- [ ] File paths containing whitespace or special characters are safely handled via NUL-delimited (`-z`) git command execution.
- [ ] Both source and target paths of renamed/copied files are validated against declared `allowed_paths`.
- [ ] Worktree scope diffing compares against the recorded birth base SHA rather than mutable live `HEAD`.
- [ ] Existing valid DAG dispatch and scope enforcement test suites pass without regression.
