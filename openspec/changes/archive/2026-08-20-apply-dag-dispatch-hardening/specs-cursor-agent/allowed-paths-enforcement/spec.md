# Allowed Paths Enforcement Delta

Three requirements change: **Base-SHA Three-Way Diff Union Defines "Actual Diff"** is renamed and modified; **Post-Execution Scope Check Demotes Done to Deviated** and **Git Inspection Failure Blocks, Never Guesses** are modified under their existing titles. The other five requirements are unchanged. The unmodified **.lucind/ Is Always Excluded From Scope Comparison** requirement still applies across all four legs of the modified union.

## RENAMED Requirements

### Requirement: Base-SHA Three-Way Diff Union Defines "Actual Diff"
RENAMED TO: Base-SHA Four-Way Diff Union Defines "Actual Diff"

## MODIFIED Requirements

### Requirement: Base-SHA Four-Way Diff Union Defines "Actual Diff"

The scope check MUST use the worktree's recorded birth SHA (`Worktree.BaseSHA`, captured at `worktree.Create`) as the base SHA, and MUST NOT re-resolve the primary repository's `HEAD` at check time. The check MUST compute changed paths as the union of four legs: (1) every path committed on the lane since that recorded birth SHA, regardless of commit count (`git diff --name-status -z --diff-filter=ACDMRT -M <baseSHA> HEAD`); (2) unstaged changes (worktree vs index); (3) staged changes (index vs `HEAD`, `git diff --cached`); (4) untracked files respecting `.gitignore`. Every git invocation in this union MUST use NUL-delimited (`-z`) output and MUST use `--name-status` (not `--name-only`) so both the source and destination path of a rename or copy are captured, not only the destination. The unmodified `.lucind/` exclusion still applies across all four legs. The check MUST NOT use `git diff --name-only HEAD~1` as the definition of "what the lane touched" — that breaks for a lane with zero commits (nothing to diff) or two-or-more commits (only the last one would be inspected). `enforceAllowedPaths` inside `Execute` is the terminal consumer of this union. (design.md § Scope Union and Base SHA; explore.md:21-26 citing `internal/run/run.go:458-520`, explore.md:34 citing `run.go:501-516`, explore.md:35, explore.md:36 citing `run.go:465-474` and `worktree.go:56-58`, explore.md:44 citing `run.go:508-510`.)

#### Scenario: Zero commits still evaluates correctly
- GIVEN a lane with zero commits, only untracked in-scope files, and a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN it MUST evaluate those untracked files against `AllowedPaths` and MUST NOT fail because `HEAD~1` does not resolve

#### Scenario: Two commits, the whole union is inspected
- GIVEN a lane with two commits where an earlier commit touched an out-of-scope path and the last commit did not
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST become `Deviated`, because the earlier commit is part of the union — a check that only inspected the last commit would incorrectly miss this

#### Scenario: Multiple in-scope commits stay Done
- GIVEN a lane with two or more commits that together touch only in-scope paths, plus a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST remain `Done`

#### Scenario: Staged-only path is included in the union
- GIVEN a lane with a staged-only path that matches the index, is not further modified, and is not committed, plus a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN that path MUST be included in the four-way diff union

#### Scenario: Rename source path is checked
- GIVEN a lane that renamed a file from an out-of-scope source path to an in-scope destination path, plus a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN the source path MUST be included in the union and MUST be `PathInScope`-checked, not only the destination

#### Scenario: Embedded whitespace is parsed from NUL-delimited output
- GIVEN a lane whose four-way union includes a path containing embedded whitespace or a special character, plus a `done` envelope
- WHEN `decideStatus` runs the scope check
- THEN that path MUST be parsed as a single path because the union is NUL-delimited, and MUST NOT be split on newline or trimmed as significant whitespace

### Requirement: Post-Execution Scope Check Demotes Done to Deviated

Inside `decideStatus`, after a schema-valid envelope maps to `lane.Done`, if `len(AllowedPaths) > 0` and any path in the computed diff union is outside `AllowedPaths` (neither an exact match nor a component-boundary prefix), the lane MUST be demoted to `lane.Deviated`, with a `lane_note`/`EventLaneNote` naming the offending paths. A staged-only out-of-scope path in that union MUST produce `Deviated` here, before completion-mode porcelain can mark the lane `Failed`. (Design Decision 2; design.md Gap 2 / four-way union; explore.md:21-26 citing `internal/run/run.go:458-520` and `run.go:333-338,536-559`.)

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

#### Scenario: Staged-only out-of-scope path becomes Deviated
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and a staged-only path outside those paths (present in the four-way union, matching the index, not further modified, not committed)
- WHEN `decideStatus` runs
- THEN the lane MUST become `Deviated` and MUST NOT become `Failed` via the later completion-mode porcelain check

### Requirement: Git Inspection Failure Blocks, Never Guesses

A git-command failure while computing the diff union MUST resolve to `lane.Blocked` with a diagnosis. It MUST NOT guess `Done` or `Deviated`. A worktree whose recorded `BaseSHA` is empty or missing MUST likewise resolve to `lane.Blocked` with a diagnosis naming the missing base SHA, and MUST NOT fall back to a live `rev-parse` of primary `HEAD`. (Design Decision 2; design.md § Scope Union and Base SHA / Signature changes; explore.md:36 citing `run.go:465-474` and `worktree.go:56-58`.)

#### Scenario: Git failure becomes Blocked
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and a git command in the three-way union that exits non-zero
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST become `Blocked` with a diagnosis, never `Done` or `Deviated`

#### Scenario: Missing recorded base SHA becomes Blocked
- GIVEN a `done` envelope, a non-empty `AllowedPaths`, and a worktree whose recorded `BaseSHA` is empty or missing
- WHEN `decideStatus` runs the scope check
- THEN the lane MUST become `Blocked` with a diagnosis naming the missing base SHA, and MUST NOT fall back to a live `rev-parse` of primary `HEAD`
