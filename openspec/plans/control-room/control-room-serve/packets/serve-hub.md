---
id: serve-hub
executor: opencode
routed_by: event transport lane selected because SQLite cursors need a non-blocking subscriber fanout for SSE
model: openai/gpt-5.6-sol
agent: build
feature: control-room-serve
parent_ref: refs/heads/control-room/control-room-serve
base_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
expected_parent_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
allowed_paths: ["internal/serve/hub.go","internal/serve/hub_test.go"]
---

# Apply serve-hub

## Goal

Implement the SQLite event/progress tailer and SSE hub with indexed cursors, buffered subscribers, `id:` frames, reconnect support, and slow-consumer resync without blocking the tailer.

## Done criteria

- [ ] Hub tests cover cursor resume, ordering, disconnect, and full-buffer resync.
- [ ] Only the two allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.