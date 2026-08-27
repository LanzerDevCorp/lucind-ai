# Delta for Lane Worktree Lifecycle

## ADDED Requirements

### Requirement: Lane worktree lifecycle force parameter and automated teardown

`worktree.Cleanup` and `worktree.Remove` MUST accept an explicit `force bool` parameter. Internal automated teardown callers (`DiscardCombined`, `RemoveLaneWorktree`, merge conflict abort in `Combine`, `ResolveCandidate` teardown, `integrateAttempt`) MUST pass `force: true` to bypass dirty checks for scratch and temporary worktrees.

#### Scenario: Internal automated callers pass force true for teardown
- GIVEN a temporary or scratch worktree created during integration, conflict resolution, or promotion teardown
- WHEN internal callers (`DiscardCombined`, `RemoveLaneWorktree`, `Combine` abort, `ResolveCandidate`, `integrateAttempt`) invoke worktree removal
- THEN they MUST pass `force: true` so the worktree is removed unconditionally

#### Scenario: Nonexistent worktree path teardown is idempotent
- GIVEN a lane with no existing worktree directory on disk
- WHEN `worktree.Cleanup` is invoked with `force: true` or `force: false`
- THEN the operation MUST return nil error without attempting removal commands
