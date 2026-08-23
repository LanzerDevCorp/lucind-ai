---
id: exec-stream-agy
executor: opencode
routed_by: first stream adapter lane selected after the optional contract exists and empirical stream-json output is captured
model: openai/gpt-5.6-sol
agent: build
feature: control-room-capture
parent_ref: refs/heads/control-room/control-room-capture
base_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
expected_parent_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
allowed_paths: ["internal/executor/agy.go","internal/executor/agy_stream.go","internal/executor/agy_stream_test.go"]
---

# Apply exec-stream-agy

## Goal

Empirically capture agy `stream-json` records and implement a tolerant decoder behind the common stream interface. Emit normalized progress events, preserve final Outcome semantics, and fall back to blocking JSON on unsupported or malformed records.

## Done criteria

- [ ] Real stream probe evidence and focused stub tests cover tool calls, text, edits, usage, malformed input, and fallback.
- [ ] Only the three allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.