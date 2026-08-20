# Allowed Paths Enforcement Specification

## Purpose

Give `packet.Packet` a real `AllowedPaths` field with two terminal consumers — an upfront batch-disjointness check, and a post-execution git-diff scope check that can only demote a `done` verdict — while packets that omit the field keep today's exact, unmodified behavior.

## Requirements

### Requirement: Packet AllowedPaths Field

`internal/packet.Packet` MUST gain `AllowedPaths []string`. `packet.Parse` MUST fill it from a single-line JSON array in frontmatter (`allowed_paths: ["internal/ledger/"]`). Omitted or empty MUST mean "not declared." Invalid JSON MUST be a parse error. A nested YAML list is NOT required, since `Parse` is line-oriented, not a YAML parser. (Design Decision 2.)

#### Scenario: JSON array fills the field
- GIVEN a packet whose frontmatter contains `allowed_paths: ["internal/ledger/", "cmd/lucind-ai/cli.go"]`
- WHEN `packet.Parse` runs
- THEN `AllowedPaths` MUST equal those two paths

#### Scenario: Omitted field stays undeclared
- GIVEN a packet whose frontmatter has no `allowed_paths` key
- WHEN `packet.Parse` runs
- THEN `AllowedPaths` MUST be empty and the packet MUST be treated as undeclared

#### Scenario: Invalid JSON is a parse error
- GIVEN a packet whose `allowed_paths` value is not valid JSON
- WHEN `packet.Parse` runs
- THEN parsing MUST fail and the packet MUST NOT be dispatched

### Requirement: Omitting AllowedPaths Preserves Today's Exact Path

When `AllowedPaths` is omitted or empty, dispatch MUST skip both the batch-disjointness check and the post-execution git-diff scope check. Existing propose, design, specs, and tasks packets that do not declare the field MUST still be able to reach `lane.Done` through the unmodified envelope path — this is regression-critical, since it must not disturb the working dual-executor dispatch flow that produced this very specification. (Design Decision 2, Decision 5; proposal: Success criteria.)

#### Scenario: Undeclared packet reaches Done unmodified
- GIVEN a schema-valid envelope with `status: done` and a packet that omits `allowed_paths`
- WHEN `decideStatus` runs
- THEN the lane MUST become `Done` without inspecting the worktree diff against any declared scope

#### Scenario: Empty list is also undeclared
- GIVEN a packet whose parsed `AllowedPaths` is empty
- WHEN dispatch and `decideStatus` run
- THEN both enforcement consumers MUST be skipped

### Requirement: Upfront Batch Disjointness Check

Before any worktree is created, the dispatch layer MUST reject a batch whose declared `AllowedPaths` overlap, using the same component-boundary prefix rule as split-time validation. This check MUST run at the CLI/dispatch layer (`packet.DisjointAllowedPaths`, called from `runDispatch` before `ExecuteBatch`), not inside `ExecuteBatch` itself. (Design Decision 2.)

#### Scenario: Overlapping --packet pair fails before Create
- GIVEN two packets whose declared `AllowedPaths` overlap under component-boundary prefix match
- WHEN `lucind-ai run --packet p1 --packet p2` starts
- THEN it MUST reject the batch before `worktree.Create` runs for either packet

#### Scenario: Disjoint declared paths proceed
- GIVEN two packets declaring `internal/foo/` and `internal/bar/`
- WHEN `lucind-ai run` dispatches them together
- THEN the disjointness check MUST pass and dispatch MUST proceed to `ExecuteBatch`

#### Scenario: ExecuteBatch's own contract is unchanged
- GIVEN a batch that passes the disjointness check
- WHEN `ExecuteBatch` starts
- THEN its first side effect MUST still be worktree creation, and lanes MUST still never cancel each other

### Requirement: Base-SHA Four-Way Diff Union Defines "Actual Diff"

The scope check MUST consume the worktree's recorded birth SHA (`Worktree.BaseSHA`, captured at `worktree.Create` time via `git rev-parse HEAD`), and MUST NOT re-resolve the primary repository's `HEAD` at check time. Changed paths MUST be computed as the four-way union of: (1) every path committed on the lane since that recorded base SHA, regardless of commit count (`git diff --name-status -z --diff-filter=ACDMRT -M <baseSHA> HEAD`); (2) unstaged changes, worktree vs index (`git diff --name-status -z --diff-filter=ACDMRT -M`); (3) staged changes, index vs `HEAD` (`git diff --cached --name-status -z --diff-filter=ACDMRT -M`); (4) untracked files respecting `.gitignore` (`git ls-files -z -o --exclude-standard`). Every git invocation in this union MUST use NUL-delimited (`-z`) output and MUST use `--name-status` (not `--name-only`), with `-M` explicit, so both the source and destination path of a rename or copy are captured and evaluated against `AllowedPaths`. The unmodified `.lucind/` exclusion (".lucind/ Is Always Excluded From Scope Comparison") MUST continue to apply across all four legs. The check MUST NOT use `git diff --name-only HEAD~1` as the definition of "what the lane touched" -- that breaks for a lane with zero commits (nothing to diff) or two-or-more commits (only the last one would be inspected). `enforceAllowedPaths` inside `Execute` is the terminal consumer. (Design: Scope Union and Base SHA; explore.md Gap 2, Items 4-7; `internal/run/run.go:458-520,501-516`; `internal/worktree/worktree.go:56-58,62-82`.)

#### Scenario: Zero commits still evaluates correctly
- GIVEN a lane with zero commits, only untracked in-scope files, and a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN it MUST evaluate those untracked files against `AllowedPaths` and MUST NOT fail because `HEAD~1` does not resolve

#### Scenario: Two commits, the whole union is inspected
- GIVEN a lane with two commits where an earlier commit touched an out-of-scope path and the last commit did not
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST become `Deviated`, because the earlier commit is part of the four-way union against the recorded birth SHA -- a check that only inspected the last commit would incorrectly miss this

#### Scenario: Multiple in-scope commits stay Done
- GIVEN a lane with two or more commits that together touch only in-scope paths, plus a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST remain `Done`

#### Scenario: Staged-only path included in diff union
- GIVEN a lane with a file staged in the index (matching the index, uncommitted, not further modified) and a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN the staged file MUST be included in the four-way diff union via the staged (`--cached`) leg and evaluated against `AllowedPaths`

#### Scenario: Both rename endpoints checked against allowed paths
- GIVEN a lane where an out-of-scope file is renamed to an in-scope path (or an in-scope file is renamed to an out-of-scope path) and a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN both the source and destination paths of the rename MUST be evaluated against `AllowedPaths`, and an out-of-scope endpoint MUST cause the lane to become `Deviated`

#### Scenario: Path with embedded whitespace or special characters parsed correctly
- GIVEN a lane whose four-way union includes a path containing embedded whitespace, a newline, or a special character, plus a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN that path MUST be parsed as a single path via NUL-delimited output, and MUST NOT be split on newline or trimmed as significant whitespace

### Requirement: Post-Execution Scope Check Demotes Done to Deviated

Inside `decideStatus`, after a schema-valid envelope maps to `lane.Done`, if `len(AllowedPaths) > 0` and any path in the computed diff union is outside `AllowedPaths` (neither an exact match nor a component-boundary prefix), the lane MUST be demoted to `lane.Deviated`, with a `lane_note`/`EventLaneNote` naming the offending paths. A staged-only out-of-scope path present in the four-way union MUST produce `Deviated` here, before the later completion-mode porcelain check can misclassify it as `Failed`. (Design: Scope Union and Base SHA; explore.md Gap 2, Item 4; `internal/run/run.go:333-338,458-520,536-559`.)

#### Scenario: In-scope-only diff stays Done
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and every changed path in scope
- WHEN `decideStatus` runs
- THEN the lane MUST remain `Done`

#### Scenario: Out-of-scope tracked file becomes Deviated
- GIVEN a `done` envelope, `AllowedPaths: ["internal/ledger/"]`, and a modified file `internal/serve/server.go`
- WHEN `decideStatus` runs
- THEN the lane MUST become `Deviated`, and the ledger note MUST name `internal/serve/server.go`

#### Scenario: Out-of-scope untracked file becomes Deviated and is excluded from integration
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and an untracked file outside those paths
- WHEN `decideStatus` runs
- THEN the lane MUST become `Deviated` and MUST NOT be placed on `barrier.Outcome.Integrate`

#### Scenario: Staged-only out-of-scope path becomes Deviated, not Failed
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and a staged-only path outside those paths (present in the four-way union, matching the index, not further modified, not committed)
- WHEN `decideStatus` runs
- THEN the lane MUST become `Deviated` with a ledger note naming the out-of-scope path, and MUST NOT become `Failed` via the later completion-mode porcelain check

### Requirement: Blocked and Failed Are Never Rewritten to Deviated

A `blocked` or `failed` envelope MUST NEVER be rewritten into `deviated` by the scope check. Only a `done` verdict is subject to this override. (Design Decision 2.)

#### Scenario: Blocked envelope stays blocked despite an out-of-scope touch
- GIVEN a `blocked` envelope and a worktree that also touches a path outside `AllowedPaths`
- WHEN `decideStatus` runs
- THEN the lane MUST remain `Blocked`, not `Deviated`

#### Scenario: Failed envelope stays failed despite an out-of-scope touch
- GIVEN a `failed` envelope and a worktree that also touches a path outside `AllowedPaths`
- WHEN `decideStatus` runs
- THEN the lane MUST remain `Failed`, not `Deviated`

### Requirement: .lucind/ Is Always Excluded From Scope Comparison

`.lucind/` MUST always be excluded from the scope comparison, even if a forced git add would otherwise surface a file there. A `.lucind/` path MUST NOT count as a scope violation and MUST NOT count as evidence that the lane stayed in scope. (Design Decision 2.)

#### Scenario: Forced .lucind file is ignored either way
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and a force-added `.lucind/result.json`
- WHEN `decideStatus` runs the scope check
- THEN that path MUST NOT demote the lane to `Deviated` and MUST NOT count toward satisfying scope on its own

### Requirement: Git Inspection Failure Blocks, Never Guesses

A git-command failure while computing the diff union or retrieving worktree state MUST resolve to `lane.Blocked` with a diagnosis. It MUST NOT guess `Done` or `Deviated`. A worktree whose recorded `BaseSHA` is empty or missing MUST likewise resolve to `lane.Blocked` with a diagnosis naming the missing base SHA, and MUST NOT fall back to a live `rev-parse` of primary `HEAD`. (Design: Scope Union and Base SHA; explore.md Item 7; `internal/run/run.go:465-474`; `internal/worktree/worktree.go:56-58,62-82`.)

#### Scenario: Git failure becomes Blocked
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and a git command in the four-way union that exits non-zero
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST become `Blocked` with a diagnosis, never `Done` or `Deviated`

#### Scenario: Missing or empty recorded BaseSHA resolves to Blocked
- GIVEN a lane worktree whose recorded `BaseSHA` is empty or missing, and a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST resolve to `lane.Blocked` with a diagnosis naming the missing base SHA, and MUST NOT fall back to a live `rev-parse` of primary `HEAD`
