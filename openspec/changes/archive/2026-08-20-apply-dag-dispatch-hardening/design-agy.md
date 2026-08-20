# Design: Apply DAG Dispatch Hardening

## Technical Approach

Harden the `apply-dag` dispatch pipeline across graph validation, worktree lifecycle, and diff scope enforcement. In `internal/dag`, enforce global transitive reachability on overlapping path scopes during DAG validation before packet emission and wave command generation. In `internal/worktree`, reify the immutable base commit SHA at worktree creation time (`worktree.Worktree.BaseSHA`) and reconcile downstream commit detection to eliminate TOCTOU races against a mutable primary `HEAD`. In `internal/run`, upgrade `enforceAllowedPaths` to a four-way NUL-delimited diff union covering committed, staged (`--cached`), unstaged, and untracked changes, parsing `--name-status -z` to validate both source and target paths of git renames before porcelain evaluation.

```
+----------------------------------------------------------------------------------------------------+
|                                    1. apply-dag.yaml Parsing & Validation                          |
|  - Parse (parse.go:40): Structural and body_path checks                                            |
|  - ValidateGlobalOverlap (validate.go): Asserts all path-overlapping nodes have transitive edges   |
|  - Waves (waves.go:18): Kahn grouping; preserves YAML order within wave                            |
|  - Split (split.go:18): Emits isolated packets and prints per-wave 'lucind-ai run' commands        |
+----------------------------------------------------------------------------------------------------+
                                                 |
                                                 v
+----------------------------------------------------------------------------------------------------+
|                                    2. Lane Worktree Creation                                       |
|  - worktree.Create (worktree.go:62): Resolves primary HEAD once at lane birth                      |
|  - Returns Worktree{Path, Branch, BaseSHA} storing immutable birth commit SHA                      |
+----------------------------------------------------------------------------------------------------+
                                                 |
                                                 v
+----------------------------------------------------------------------------------------------------+
|                                    3. Post-Execution Scope Enforcement                             |
|  - enforceAllowedPaths (run.go:464): 4-way NUL-delimited diff union against Worktree.BaseSHA       |
|    1. git diff --diff-filter=ACDMRT --name-status -z <BaseSHA> HEAD (committed changes)           |
|    2. git diff --cached --diff-filter=ACDMRT --name-status -z       (staged-only index changes)    |
|    3. git diff --diff-filter=ACDMRT --name-status -z                (unstaged working tree changes)|
|    4. git ls-files -o -z --exclude-standard                         (untracked files)              |
|  - parseGitDiffNameStatusZ: Extracts both source and destination of renames (R/C records)          |
|  - Out-of-scope path -> lane.Deviated (evaluated before enforceCompletionMode porcelain check)     |
+----------------------------------------------------------------------------------------------------+
```

## Architecture Decisions

| Choice | Rejected alternative / rationale |
|---|---|
| Validate global DAG transitive reachability upfront in `internal/dag` before `split` emits packets | Re-threading `depends_on` through packet frontmatter into `lucind-ai run`. *Rationale:* `lucind-ai run` executes an isolated batch of concurrent packets where pairwise disjointness is the invariant; the DAG sidecar is the single source of truth for graph topology and sequencing. Validating global transitive reachability before emitting packets prevents invalid wave plans from ever being generated or printed, without adding graph-routing complexity to standalone packet execution. |
| Four-way NUL-delimited diff union with `--name-status -z` rename extraction in `internal/run` | OS-level filesystem sandboxing (e.g. bubblewrap/seatbelt) or basic `--name-only` extensions. *Rationale:* Git plumbing provides portable, deterministic post-execution verification across Linux and macOS without requiring elevated privileges. `--name-status -z` captures both source (deletion) and target (creation) endpoints of renames, preventing unauthorized path modifications disguised as in-scope renames. |
| Record immutable birth commit SHA in `worktree.Worktree` at creation time | Re-resolving primary `HEAD` or computing dynamic merge-base at check time. *Rationale:* Re-resolving primary `HEAD` introduces a Time-of-Check to Time-of-Use (TOCTOU) vulnerability where a concurrent integration or push shifts the diff comparison baseline. Storing `BaseSHA` in `Worktree` reifies the exact commit the lane branched from, making diff scope checks and unique commit detection consistent and immune to external repository movement. |
| Strict non-empty `allowed_paths` in DAG validation, optional `AllowedPaths` in standalone packets | Making `allowed_paths` optional in DAG nodes, or requiring non-empty `AllowedPaths` for all direct CLI dispatches. *Rationale:* DAG nodes represent planned implementation units where explicit scope boundaries are mandatory to guarantee parallel safety. Standalone packets dispatched directly via `lucind-ai run` support read-only exploration and ad-hoc single-packet execution where scope bounding is not required. |
| Run diff scope check before porcelain check so staged out-of-scope paths yield `lane.Deviated` | Letting staged changes fall through to porcelain check failure (`lane.Failed`). *Rationale:* A lane touching an unlisted path is a scope breach (`lane.Deviated`), not an unhandled runtime failure (`lane.Failed`). Evaluating `enforceAllowedPaths` (with the staged `--cached` leg) prior to `enforceCompletionMode` guarantees accurate status reporting in the ledger. |

## Global Overlap Validation and Transitive Reachability

### Problem and Context
`internal/dag/waves.go:64-74` currently evaluates `packet.DisjointAllowedPaths` only within each individual Kahn wave. If packet $A$ is in wave 1 and packet $B$ is in wave 2, but there is no dependency path between $A$ and $B$ (i.e. $B$ was placed in wave 2 because it depends on an unrelated packet $C$), $A$ and $B$ are allowed to declare overlapping `allowed_paths`. This creates an unvalidated ordering hazard: $A$ and $B$ are not causally ordered by explicit dependency, meaning any re-topologizing or independent scheduling can execute them concurrently.

Furthermore, `cmd/lucind-ai/cli.go:187` runs `packet.DisjointAllowedPaths` only across the packets passed to a single `lucind-ai run` invocation. Because `dag.Split` emits per-wave run commands and `dag.EmitPacketContent` drops `depends_on` (`emit.go:11-33`), the CLI check never sees cross-wave pairs.

### Design and Data Structures
We introduce global transitive overlap validation to `internal/dag`:

```go
// ValidateGlobalOverlap verifies that for every pair of distinct packets (u, v)
// in d, if their declared AllowedPaths overlap under PathInScope component-boundary
// matching, there exists a directed dependency path from u to v OR from v to u.
func ValidateGlobalOverlap(d DAG) error
```

#### Transitive Reachability Algorithm
1. Construct a directed adjacency graph `dependents map[string][]string` where an edge $u \to v$ exists if $v$ lists $u$ in `DependsOn`.
2. Compute the transitive closure / reachability query `isReachable(from, to string, dependents map[string][]string) bool` using Breadth-First Search (BFS) or Depth-First Search (DFS) over the DAG. (Because `Validate` / `Waves` verifies acyclicity, reachability terminates unconditionally).
3. For every unordered pair of nodes $(u, v)$ with $u \neq v$:
   - Check if any path in $u.\text{AllowedPaths}$ overlaps with any path in $v.\text{AllowedPaths}$ via `packet.PathInScope(pathA, []string{pathB}) || packet.PathInScope(pathB, []string{pathA})`.
   - If an overlap exists, assert:
     $$\text{isReachable}(u, v) \lor \text{isReachable}(v, u)$$
   - If neither $u$ can reach $v$ nor $v$ can reach $u$, return a diagnostic error:
     ```
     dag: overlapping allowed_paths between "lane-a" (internal/foo/) and "lane-b" (internal/foo/bar.go) without dependency ordering
     ```

#### Terminal Consumer and Call Sites
- **Terminal Consumer**: `dag.Waves` (`internal/dag/waves.go:18`) and `dag.Split` (`internal/dag/split.go:18`).
- **Enforcement Point**: Inside `dag.Waves(d DAG)` immediately following `Validate(d)`. `ValidateGlobalOverlap` ensures that Kahn wave partitioning and per-wave command generation only succeed for validly ordered DAGs.

## Four-Way Diff Union and Scope Enforcement

### Problem and Context
`internal/run/run.go:458-520` (`enforceAllowedPaths`) currently unions three git queries:
1. `git diff --name-only --diff-filter=ACDMRT <base> HEAD` (committed changes on lane branch)
2. `git diff --name-only --diff-filter=ACDMRT` (unstaged changes: worktree vs index)
3. `git ls-files -o --exclude-standard` (untracked files)

This three-way union has three critical gaps:
1. **Missing Staged Leg (`--cached`):** If an agent stages an out-of-scope file (`git add out-of-scope.go`) without committing or modifying it further in the worktree, the file is invisible to all three legs. It passes `enforceAllowedPaths` as `lane.Done` and subsequently fails `deps.PorcelainEmpty` in `enforceCompletionMode`, misclassifying a scope violation as `lane.Failed`.
2. **Newline and Whitespace Splitting:** Outputs are parsed via `strings.Split(string(out), "\n")` and `strings.TrimSpace`, mis-parsing paths with embedded whitespace or newlines.
3. **Rename Endpoint Blindness:** `--name-only` emits only the target path of a rename. If `internal/forbidden/old.go` is renamed to `internal/allowed/new.go`, `old.go` (a deletion outside scope) is never checked.

### Design and Implementation

`enforceAllowedPaths` is upgraded to execute four NUL-delimited git queries and parse `--name-status -z`:

```go
func enforceAllowedPaths(ctx context.Context, deps Deps, worktreePath, baseSHA string, p packet.Packet) (lane.Status, string) {
    // Leg 1: Committed changes since BaseSHA
    // git -C <wt> diff --diff-filter=ACDMRT --name-status -z <baseSHA> HEAD
    
    // Leg 2: Staged changes in index (index vs HEAD)
    // git -C <wt> diff --cached --diff-filter=ACDMRT --name-status -z
    
    // Leg 3: Unstaged changes in worktree (worktree vs index)
    // git -C <wt> diff --diff-filter=ACDMRT --name-status -z
    
    // Leg 4: Untracked files
    // git -C <wt> ls-files -o -z --exclude-standard
}
```

#### NUL-Delimited Parser for `--name-status -z`
Git `--name-status -z` emits null-byte separated tokens. For standard modifications/additions/deletions, records follow `<status>\0<path>\0`. For renames (`R...`) and copies (`C...`), records follow `<status>\0<src-path>\0<dst-path>\0`.

```go
// parseGitDiffNameStatusZ extracts all modified paths from NUL-delimited git diff --name-status -z output.
// For renames and copies (status starting with 'R' or 'C'), both the source and target paths are extracted.
func parseGitDiffNameStatusZ(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	tokens := bytes.Split(data, []byte{0})
	var paths []string
	for i := 0; i < len(tokens); i++ {
		token := string(tokens[i])
		if token == "" {
			continue
		}
		status := token
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			// Rename/Copy: next token is source path, token after is target path
			if i+2 < len(tokens) {
				paths = append(paths, string(tokens[i+1]), string(tokens[i+2]))
				i += 2
			}
		} else {
			// Standard change (A, M, D, T): next token is path
			if i+1 < len(tokens) {
				paths = append(paths, string(tokens[i+1]))
				i += 1
			}
		}
	}
	return paths
}
```

#### `.lucind/` Exclusion and Scope Evaluation
1. All extracted paths across all 4 legs are deduplicated into `changedPaths`.
2. Paths matching `.lucind/` or `.lucind` are ignored.
3. Every path in `changedPaths` is verified via `packet.PathInScope(path, p.AllowedPaths)`.
4. If unauthorized paths exist, return `lane.Deviated, "actual diff touched paths outside declared allowed_paths: <list>"`.

#### Terminal Consumer and Call Sites
- **Terminal Consumer**: `run.Execute` in `internal/run/run.go:334`.
- **Enforcement Point**: Called after `decideStatus` and before `enforceCompletionMode`. Detecting staged out-of-scope files here demotes the lane to `lane.Deviated` before `enforceCompletionMode` can fail it as dirty porcelain.

## Worktree Base Reification and TOCTOU Elimination

### Problem and Context
`internal/worktree/worktree.go:56-59` defines `Worktree` as:
```go
type Worktree struct {
    Path   string
    Branch string
}
```
When `enforceAllowedPaths` runs (`run.go:465-474`), it calls `git -C deps.PrimaryRoot rev-parse HEAD` live at check time. If another lane finishes and merges into `primaryRoot` while lane $L$ is executing, `primaryRoot`'s `HEAD` advances. The diff `git diff <new-HEAD> HEAD` in $L$'s worktree compares $L$'s branch against an unrelated commit, corrupting the diff baseline.

Additionally, `worktree.HasUniqueCommits` (`worktree.go:134-165`) re-resolves `primaryRoot`'s `HEAD` and computes `git merge-base HEAD <live-HEAD>`. `enforceAllowedPaths` and `HasUniqueCommits` thus use two different, mutating concepts of "base".

### Design and Data Structures

1. Extend `worktree.Worktree` to capture the immutable birth commit:
```go
type Worktree struct {
	Path    string // absolute path to the worktree directory
	Branch  string // the branch checked out in it
	BaseSHA string // commit SHA of primaryRoot at creation time
}
```

2. Update `worktree.Create` (`internal/worktree/worktree.go:62`):
```go
func Create(ctx context.Context, primaryRoot, laneID string) (Worktree, error) {
	if laneID == "" {
		return Worktree{}, ErrEmptyLaneID
	}
	path := pathFor(primaryRoot, laneID)
	if _, err := os.Stat(path); err == nil {
		return Worktree{}, ErrWorktreeExists
	}
	branch := BranchFor(laneID)

	// Resolve primary HEAD before worktree creation
	cmdHead := exec.CommandContext(ctx, "git", "-C", primaryRoot, "rev-parse", "HEAD")
	var stderrHead strings.Builder
	cmdHead.Stderr = &stderrHead
	outHead, err := cmdHead.Output()
	if err != nil {
		return Worktree{}, fmt.Errorf("worktree: git rev-parse HEAD failed: %w: %s", err, strings.TrimSpace(stderrHead.String()))
	}
	baseSHA := strings.TrimSpace(string(outHead))

	// Explicitly branch from the resolved baseSHA
	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branch, path, baseSHA)
	cmd.Dir = primaryRoot
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Worktree{}, fmt.Errorf("worktree: git worktree add failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return Worktree{Path: path, Branch: branch, BaseSHA: baseSHA}, nil
}
```

3. Reconcile `HasUniqueCommits` to take `baseSHA`:
```go
// HasUniqueCommits reports whether the worktree at worktreePath has commits
// not present in baseSHA, verified by checking if worktree HEAD differs from baseSHA.
func HasUniqueCommits(ctx context.Context, worktreePath, baseSHA string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "rev-parse", "HEAD")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("worktree: git rev-parse HEAD failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	wtHead := strings.TrimSpace(string(out))
	return wtHead != baseSHA, nil
}
```

4. Update `run.Deps` and `run.Execute`:
- `Deps.HasUniqueLaneCommits` signature updated to `func(ctx context.Context, worktreePath, baseSHA string) (bool, error)`.
- `run.Execute` passes `wt.BaseSHA` to `enforceAllowedPaths` and `deps.HasUniqueLaneCommits`.

#### Terminal Consumer and Call Sites
- **Terminal Consumer**: `worktree.Create` (`internal/worktree/worktree.go:62`) populates `BaseSHA`; `run.Execute` (`internal/run/run.go:234, 334, 338`) consumes `wt.BaseSHA` to drive scope verification and unique commit checks.

## Item and Edge-Case Disposition Summary

| Item / Finding | Classification | Disposition | Concrete Design & Enforcement Point |
|---|---|---|---|
| **Gap 1: Per-wave disjointness only** | Confirmed Gap | **Fix now** | Implement `ValidateGlobalOverlap(d DAG)` in `internal/dag/validate.go`; invoked by `dag.Waves` (`waves.go:18`) and `dag.Split` (`split.go:18`). |
| **Gap 2: Diff union misses staged paths** | Confirmed Gap | **Fix now** | Add `git diff --cached --diff-filter=ACDMRT --name-status -z` leg to `enforceAllowedPaths` (`run.go:464`), running before `enforceCompletionMode`. |
| **Item 3: Transitive-dependency ordering** | Candidate Item | **Fix now** | Graph reachability check `isReachable(u, v)` inside `ValidateGlobalOverlap`, requiring transitive causal ordering for all overlapping scope pairs. |
| **Item 4: Explicit staged-only detection** | Candidate Item | **Fix now** | Subsumed into Gap 2 fix via the `--cached` diff leg in `internal/run/run.go`. |
| **Item 5: NUL-delimited (`-z`) git output** | Candidate Item | **Fix now** | Use `-z` across all 4 legs in `enforceAllowedPaths` and `git status --porcelain -z` in `PorcelainEmpty`; split on `\x00` bytes. |
| **Item 6: Rename source and destination endpoints** | Candidate Item | **Fix now** | Use `--name-status -z` across all diff legs; parse `R...`/`C...` status records to extract and validate both source and destination paths. |
| **Item 7: Recorded base SHA vs mutable HEAD** | Candidate Item | **Fix now** | Record `BaseSHA` in `worktree.Worktree` at creation; pass to `enforceAllowedPaths` and `HasUniqueCommits` to eliminate TOCTOU. |
| **Edge Case 1: `depends_on` dropped at emit** | Edge Case | **Fix now** | Enforce all transitive graph constraints on `dag.DAG` before emission in `dag.Split`; emitted standalone packets remain decoupled from DAG metadata. |
| **Edge Case 2: Independent CLI disjointness check** | Edge Case | **Fix now** | Retain `packet.DisjointAllowedPaths(ps)` in `cmd/lucind-ai/cli.go:187` as a defense-in-depth barrier for multi-packet batch dispatches. |
| **Edge Case 3: Staged out-of-scope misclassification** | Edge Case | **Fix now** | Resolved by running 4-way `enforceAllowedPaths` before `enforceCompletionMode` in `run.Execute`. |
| **Edge Case 4: Base SHA mismatch in checks** | Edge Case | **Fix now** | Reconcile both `enforceAllowedPaths` and `HasUniqueCommits` to use `wt.BaseSHA`. |
| **Edge Case 5: `.lucind/` path exclusion** | Edge Case | **Fix now** | Explicitly preserved in NUL-delimited parser across all 4 diff legs. |
| **Edge Case 6: Empty `allowed_paths` contract** | Edge Case | **Fix now** | Clarify contract: DAG validation requires non-empty `allowed_paths` (`validate.go:30`); standalone packet parser allows empty for exploratory dispatches. |
| **Edge Case 7: Missing regression test coverage** | Edge Case | **Fix now** | Implement explicit regression test functions across `internal/dag`, `internal/run`, and `internal/worktree`. |

## File Changes and Test Seams

### Modified and Created Files

| File | Action | Changes |
|---|---|---|
| `internal/dag/validate.go` | Modify | Add `ValidateGlobalOverlap(d DAG) error` enforcing transitive reachability on path-overlapping pairs. |
| `internal/dag/reachability.go` | Create | Helper function `isReachable(from, to string, dependents map[string][]string) bool`. |
| `internal/dag/waves.go` | Modify | Call `ValidateGlobalOverlap(d)` in `Waves(d)` (`:18`) to gate wave generation. |
| `internal/dag/waves_test.go` | Modify | Add tests for cross-wave overlap with and without transitive dependency edges. |
| `internal/worktree/worktree.go` | Modify | Add `BaseSHA string` to `Worktree` struct; resolve primary `HEAD` in `Create` (`:62`); update `HasUniqueCommits` (`:137`) to take `baseSHA`. |
| `internal/worktree/worktree_test.go` | Modify | Add tests for `BaseSHA` recording and `HasUniqueCommits` verification against recorded base. |
| `internal/run/run.go` | Modify | Update `enforceAllowedPaths` (`:464`) with 4-way diff union, `-z` output, and `--name-status` rename parser; update `Deps.HasUniqueLaneCommits`. |
| `internal/run/run_test.go` | Modify | Add regression tests for staged-only scope breach, rename source scope breach, and NUL-delimited paths. |
| `cmd/lucind-ai/cli.go` | Modify | Update `productionDeps` wiring to pass `wt.BaseSHA` to `HasUniqueLaneCommits`. |

### New Regression Test Coverage

1. `internal/dag/waves_test.go`:
   - `TestWaves_CrossWaveOverlapWithoutEdgeRejected`: Packets $A$ and $B$ overlap in `allowed_paths`, $B$ is in Wave 2 (via dependency on $C$), but $B$ does not depend on $A$. Verifies `dag.Waves` returns an error.
   - `TestWaves_TransitiveDependencyOverlapAccepted`: Packet $C$ depends on $B$, $B$ depends on $A$. $A$ and $C$ share `allowed_paths`. Verifies `dag.Waves` accepts the DAG and orders $A$ in Wave 1 and $C$ in Wave 3.
2. `internal/run/run_test.go`:
   - `TestExecuteScopeCheckStagedOnlyPathDetected`: An out-of-scope file is staged (`git add`) in the worktree but uncommitted. Verifies `run.Execute` returns `lane.Deviated` with the offending path, not `lane.Failed`.
   - `TestExecuteScopeCheckRenameSourceDetected`: An out-of-scope file `internal/secret/foo.go` is renamed to in-scope `internal/ledger/foo.go`. Verifies `run.Execute` returns `lane.Deviated` identifying the source path.
   - `TestExecuteScopeCheckNulDelimitedPaths`: Files with spaces and special characters are modified in scope. Verifies NUL-delimited parser handles them cleanly without deviation.
   - `TestExecuteScopeCheckRecordedBaseSHA`: Primary repository `HEAD` moves after worktree creation. Verifies scope diff evaluates against `Worktree.BaseSHA` rather than moved primary `HEAD`.
3. `internal/worktree/worktree_test.go`:
   - `TestCreateRecordsBaseSHA`: Verifies `worktree.Create` records the exact primary `HEAD` commit in `Worktree.BaseSHA`.
   - `TestHasUniqueCommitsReconciledWithBaseSHA`: Verifies commit detection compares worktree `HEAD` against `baseSHA` consistently.

## Threat Matrix

| Boundary | Applicability; safe/failure behavior; planned RED test |
|---|---|
| **Cross-Lane File Overlap (Unordered Cross-Wave)** | Applicable: Packets in different waves touching overlapping paths without explicit dependency edges risk write contention. Safe behavior: `ValidateGlobalOverlap` rejects DAG during `dag.Waves`/`dag.Split`. Planned RED test: `TestWaves_CrossWaveOverlapWithoutEdgeRejected`. |
| **Staged-Only Scope Check Bypass** | Applicable: Agent stages out-of-scope files without committing, evading 3-way diff union and misclassifying as dirty porcelain. Safe behavior: 4-way union detects staged index changes and returns `lane.Deviated`. Planned RED test: `TestExecuteScopeCheckStagedOnlyPathDetected`. |
| **Rename Source Escape** | Applicable: Agent renames an out-of-scope source file to an in-scope destination, mutating/deleting out-of-scope files unnoticed by `--name-only`. Safe behavior: `--name-status -z` extracts both endpoints and returns `lane.Deviated`. Planned RED test: `TestExecuteScopeCheckRenameSourceDetected`. |
| **TOCTOU on Mutable Primary HEAD** | Applicable: Concurrent merge or push advances primary `HEAD` during lane execution, corrupting diff baseline. Safe behavior: Scope diff and unique commit check use immutable `Worktree.BaseSHA`. Planned RED test: `TestExecuteScopeCheckRecordedBaseSHA`. |
| **Whitespace / Special Character Path Corruption** | Applicable: Paths with whitespace or special characters mis-parsed by newline splitting. Safe behavior: NUL-delimited `-z` parsing preserves exact byte sequences. Planned RED test: `TestExecuteScopeCheckNulDelimitedPaths`. |
| **Direct CLI Dispatch Scope Safety** | Applicable: Packets dispatched directly via `lucind-ai run` without DAG sidecar. Safe behavior: CLI enforces upfront pairwise disjointness on batch; runtime enforces diff scope on each lane. Planned RED test: Existing CLI disjointness and scope tests. |

## Open Questions

None.
