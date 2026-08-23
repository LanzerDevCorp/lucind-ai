---
id: exec-contract
executor: opencode
routed_by: executor contract lane selected because optional progress transport must preserve nil-channel blocking behavior
model: openai/gpt-5.6-sol
agent: build
feature: control-room-capture
parent_ref: refs/heads/control-room/control-room-capture
base_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
expected_parent_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
allowed_paths: ["internal/executor/executor.go","internal/executor/executor_test.go"]
---

# Apply exec-contract

## Goal

Add the optional progress request channel and canonical progress event type without changing existing executor defaults, model validation, or outcome routing when the channel is nil.

## Done criteria

- [ ] Contract and regression tests are limited to the two allowed paths.
- [ ] Existing executor tests pass and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.