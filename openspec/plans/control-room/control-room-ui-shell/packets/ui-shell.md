---
id: ui-shell
executor: opencode
routed_by: UI shell lane selected because the server contract is complete and the project requires embedded zero-build assets
model: openai/gpt-5.6-sol
agent: build
feature: control-room-ui-shell
parent_ref: refs/heads/control-room/control-room-ui-shell
base_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
expected_parent_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
allowed_paths: ["internal/serve/static/index.html","internal/serve/static/app.js","internal/serve/static_test.go"]
---

# Apply ui-shell

## Goal

Implement the embedded no-build Control Room shell, shared live store, SSE/polling fallback, top-bar counters, view mount points, and accessible status treatment without introducing npm or another build artifact.

## Done criteria

- [ ] Static tests prove embedding, no forbidden bulk approval controls, and deterministic store fallback behavior.
- [ ] Only the three allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.