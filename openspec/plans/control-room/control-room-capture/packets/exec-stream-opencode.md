---
id: exec-stream-opencode
executor: opencode
routed_by: third stream adapter lane selected after the optional contract exists and empirical output shape is verified
model: openai/gpt-5.6-sol
agent: build
feature: control-room-capture
parent_ref: refs/heads/control-room/control-room-capture
base_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
expected_parent_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
allowed_paths: ["internal/executor/opencode.go","internal/executor/opencode_stream.go","internal/executor/opencode_stream_test.go"]
---

# Apply exec-stream-opencode

## Goal

Empirically determine opencode's live stream output, implement a tolerant normalized decoder, preserve the existing `--format json` result path when progress is disabled, and degrade safely on decode failure.

## Done criteria

- [ ] Empirical output capture is attached to result evidence and focused tests cover known, unknown, and malformed records.
- [ ] Only the three allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.