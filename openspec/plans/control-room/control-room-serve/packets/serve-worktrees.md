---
id: serve-worktrees
executor: opencode
routed_by: worktree visibility lane selected because the fleet view needs branch path disk size and stale state
model: openai/gpt-5.6-sol
agent: build
feature: control-room-serve
parent_ref: refs/heads/control-room/control-room-serve
base_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
expected_parent_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
allowed_paths: ["internal/serve/worktrees.go","internal/serve/worktrees_test.go"]
---

# Apply serve-worktrees

## Goal

Add a read-only worktree status payload that reports path, branch/lane association, disk bytes, and stale state using existing worktree conventions without mutating the repository.

## Done criteria

- [ ] Tests use temporary repositories/directories and assert stale and live cases.
- [ ] Only the two allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.