---
id: ui-dag
executor: opencode
routed_by: DAG view lane selected after the shared store exists so wave topology and live status use the same state model
model: openai/gpt-5.6-sol
agent: build
feature: control-room-ui-shell
parent_ref: refs/heads/control-room/control-room-ui-shell
base_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
expected_parent_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
allowed_paths: ["internal/serve/static/app.js","internal/serve/static_test.go"]
---

# Apply ui-dag

## Goal

Render parsed apply DAG waves, packet nodes, dependency edges, live status, and overlap violations from the API without recomputing or inventing graph state in the browser.

## Done criteria

- [ ] View tests cover multiple waves, dependencies, terminal statuses, and overlap-error styling.
- [ ] Only the two allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.