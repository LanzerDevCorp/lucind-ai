# Delta for Allowed Paths Enforcement

## MODIFIED Requirements

### Requirement: Base-SHA Four-Way Diff Union Defines "Actual Diff"

Scope enforcement MUST use the recorded worktree birth SHA and MUST NOT re-resolve primary `HEAD`. It MUST inspect the NUL-delimited name-status union of committed changes since base, unstaged changes, staged changes, and non-ignored untracked files. Rename and copy detection MUST preserve and check both source and destination paths. `.lucind/` MUST remain excluded. The union MUST cover zero or multiple commits and MUST NOT use only `HEAD~1`. The terminal scope consumer MUST evaluate every union entry against `AllowedPaths`.
(Previously: The four-way union enforced write scope but did not establish shared result classifications for frozen candidate correspondence.)

#### Scenario: Zero commits still evaluates correctly
- GIVEN zero commits and only untracked in-scope files
- WHEN scope enforcement runs
- THEN it MUST evaluate the untracked files without requiring `HEAD~1`

#### Scenario: Two commits, the whole union is inspected
- GIVEN an earlier commit touched an out-of-scope path and the latest did not
- WHEN scope enforcement runs
- THEN the lane MUST become `Deviated`

#### Scenario: Multiple in-scope commits stay Done
- GIVEN multiple commits together touch only in-scope paths and the result is `done`
- WHEN scope enforcement runs
- THEN the lane MUST remain `Done`

#### Scenario: Staged-only path included in diff union
- GIVEN a staged but uncommitted path
- WHEN scope enforcement runs
- THEN that path MUST be evaluated against `AllowedPaths`

#### Scenario: Both rename endpoints checked against allowed paths
- GIVEN a rename has one endpoint outside write scope
- WHEN scope enforcement runs
- THEN both endpoints MUST be checked and the lane MUST become `Deviated`

#### Scenario: Special-character path remains intact
- GIVEN a changed path contains whitespace, a newline, or special characters
- WHEN scope enforcement parses the NUL-delimited union
- THEN it MUST evaluate that exact path without splitting or trimming it

## ADDED Requirements

### Requirement: Canonical Candidate Change and Commit Semantics

For a frozen `done` candidate, runtime, result validation, and Acceptance MUST derive the same canonical base-to-candidate changed-path set and classifications from Git facts, excluding `.lucind/**`. Entries MUST be repository-relative, unique, and deterministically ordered. Added, modified, and deleted paths MUST map to `created`, `modified`, and `deleted`; a rename MUST be represented by a deleted source and created destination so both endpoints remain enforceable. A write result's commit MUST equal the frozen candidate commit. A read-only result MUST omit commit and its canonical change set MUST be empty.

#### Scenario: Deletion correspondence
- GIVEN the frozen candidate deletes an allowed file
- WHEN runtime, result validation, and Acceptance classify the candidate
- THEN each MUST require the same `deleted` path entry

#### Scenario: Rename correspondence
- GIVEN the frozen candidate renames an allowed source to an allowed destination
- WHEN canonical changes are compared with `files_changed`
- THEN the result MUST contain the source as `deleted` and destination as `created`

#### Scenario: Commit mismatch
- GIVEN a versioned write result names a commit other than the frozen candidate commit
- WHEN result correspondence is checked
- THEN the lane MUST not be accepted as `done`

#### Scenario: Read-only candidate reports changes
- GIVEN a read-only result names a commit or a changed path
- WHEN canonical correspondence is checked
- THEN correspondence MUST fail
