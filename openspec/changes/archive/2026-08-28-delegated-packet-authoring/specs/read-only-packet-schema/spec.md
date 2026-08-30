# Delta for Read-Only Packet Schema

## ADDED Requirements

### Requirement: Read-Only Input Path Preservation and Visibility

Packet parsing MUST preserve every declared `read_only_paths` value as normalized worktree-relative input scope, distinct from `AllowedPaths` and the `read_only` execution-mode flag. The exact declared read-only paths MUST be available in the executor-visible assignment context without granting write authority or rewriting an admitted manual packet body.

#### Scenario: Declared inputs reach the executor
- GIVEN a packet declaring two valid read-only input paths
- WHEN it is parsed, admitted, and dispatched
- THEN both paths MUST remain distinguishable from write scope and MUST be visible to the executor

#### Scenario: Omitted inputs preserve compatibility
- GIVEN a legacy manual packet with no `read_only_paths`
- WHEN it is dispatched
- THEN parsing MUST succeed with no declared read-only inputs and its body MUST remain unchanged

#### Scenario: Read-only input does not grant writes
- GIVEN a path appears only in `read_only_paths`
- WHEN the lane changes that path
- THEN changed-path enforcement MUST treat it as outside write scope

### Requirement: Read-Only Path Validation

Declared read-only input paths MUST be non-empty, repository-relative, normalized, and free of traversal outside the repository. Invalid declarations MUST fail universal admission before executor activity.

#### Scenario: Traversal path rejected
- GIVEN a packet declares `../secret` as a read-only input
- WHEN admission validates paths
- THEN admission MUST fail with a diagnostic identifying that path

#### Scenario: Rename crosses read-only input scope
- GIVEN a lane renames a declared read-only input path into an allowed write path
- WHEN changed paths are evaluated
- THEN the source deletion MUST remain unauthorized and the lane MUST NOT be accepted as in scope
