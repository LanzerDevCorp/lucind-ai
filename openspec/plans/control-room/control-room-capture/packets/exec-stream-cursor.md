---
id: exec-stream-cursor
executor: opencode
routed_by: second stream adapter lane selected after the optional contract exists and empirical output shape is verified
model: openai/gpt-5.6-sol
agent: build
feature: control-room-capture
parent_ref: refs/heads/control-room/control-room-capture
base_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
expected_parent_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
allowed_paths: ["internal/executor/cursor_agent.go","internal/executor/cursor_stream.go","internal/executor/cursor_agent_test.go"]
---

# Apply exec-stream-cursor

## Goal

Empirically determine cursor-agent's live stream output, implement a tolerant normalized decoder, and degrade to the existing blocking JSON path when the stream is absent or unparseable. Do not invent a field schema from the proposal.

## Done criteria

- [ ] Empirical output capture is attached to the result evidence and decoder tests cover known and unknown records.
- [ ] Only the three allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.