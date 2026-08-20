# Allowed Paths Enforcement Specification

## RENAMED Requirements

### Requirement: Base-SHA Three-Way Diff Union Defines "Actual Diff"
RENAMED TO: Base-SHA Four-Way Diff Union Defines "Actual Diff"

## MODIFIED Requirements

### Requirement: Base-SHA Four-Way Diff Union Defines "Actual Diff"

The scope check MUST consume the worktree's recorded birth SHA (`Worktree.BaseSHA`, captured at `worktree.Create` time via `git rev-parse HEAD`), never a live re-resolve of primary repository `HEAD` at check time. Changed paths MUST be computed as the four-way union of: (1) every path committed on the lane since that recorded base SHA (`git diff --name-status -z --diff-filter=ACDMRT -M <baseSHA> HEAD`); (2) unstaged changes (worktree vs index, `git diff --name-status -z --diff-filter=ACDMRT -M`); (3) staged changes (index vs HEAD, `git diff --cached --name-status -z --diff-filter=ACDMRT -M`); (4) untracked files respecting `.gitignore` (`git ls-files -z -o --exclude-standard`). All git diff and file inspection commands in the union MUST use NUL-delimited (`-z`) output and MUST use `--name-status -M` (not `--name-only`) so that both the source and destination paths of a rename or copy are captured and evaluated against `AllowedPaths`. The `.lucind/` directory exclusion (unmodified requirement: ".lucind/ Is Always Excluded From Scope Comparison") MUST continue to apply across all four legs of the union. (Design: Scope Union and Base SHA; design.md Architecture Decisions; explore.md Gap 2, Items 4-7; internal/run/run.go:458-520; internal/worktree/worktree.go:62-82.)

#### Scenario: Zero commits still evaluates correctly
- GIVEN a lane with zero commits, only untracked in-scope files, and a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN it MUST evaluate those untracked files against `AllowedPaths` and MUST NOT fail because `HEAD~1` does not resolve

#### Scenario: Two commits, the whole union is inspected
- GIVEN a lane with two commits where an earlier commit touched an out-of-scope path and the last commit did not
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST become `Deviated`, because the earlier commit is part of the four-way union against the recorded birth SHA — a check that only inspected the last commit would incorrectly miss this

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
- THEN both the source and destination paths of the rename MUST be evaluated against `AllowedPaths`, and the out-of-scope endpoint MUST cause the lane to become `Deviated`

#### Scenario: Path with special characters or whitespace parsed correctly
- GIVEN a lane that touches a path containing embedded whitespace, newlines, or special characters
- WHEN `decideStatus` runs the scope check
- THEN the four-way diff union MUST parse the path correctly via NUL-delimited parsing without truncation or improper splitting

### Requirement: Post-Execution Scope Check Demotes Done to Deviated

Inside `decideStatus`, after a schema-valid envelope maps to `lane.Done`, if `len(AllowedPaths) > 0` and any path in the computed diff union is outside `AllowedPaths` (neither an exact match nor a component-boundary prefix), the lane MUST be demoted to `lane.Deviated`, with a `lane_note`/`EventLaneNote` naming the offending paths. (Design: Scope Union and Base SHA; design.md Decision 2; explore.md Gap 2, Item 4; internal/run/run.go:333-338, 458-520.)

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

#### Scenario: Staged-only out-of-scope path demotes to Deviated
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and an uncommitted staged file outside those paths
- WHEN `decideStatus` runs
- THEN the scope check MUST demote the lane to `Deviated` with a ledger note naming the out-of-scope path before completion-mode porcelain checks can misclassify it as `Failed`

### Requirement: Git Inspection Failure Blocks, Never Guesses

A git-command failure while computing the diff union or retrieving worktree state MUST resolve to `lane.Blocked` with a diagnosis. It MUST NOT guess `Done` or `Deviated`. (Design: Scope Union and Base SHA; explore.md Item 7; internal/run/run.go:465-474; internal/worktree/worktree.go:62-82.)

#### Scenario: Git failure becomes Blocked
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and a git command in the four-way union that exits non-zero
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST become `Blocked` with a diagnosis, never `Done` or `Deviated`

#### Scenario: Missing or empty recorded BaseSHA resolves to Blocked
- GIVEN a lane worktree whose recorded `BaseSHA` is empty or missing, and a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST resolve to `lane.Blocked` with a diagnosis naming the missing base SHA, and MUST NOT fall back to a live `rev-parse` of primary `HEAD`
