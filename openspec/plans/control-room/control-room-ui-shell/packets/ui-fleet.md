---
id: ui-fleet
executor: opencode
routed_by: Fleet view lane selected after the shared store exists so live lane cards have one canonical state source
model: openai/gpt-5.6-sol
agent: build
feature: control-room-ui-shell
parent_ref: refs/heads/control-room/control-room-ui-shell
base_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
expected_parent_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
allowed_paths: ["internal/serve/static/app.js","internal/serve/static_test.go"]
---

# Apply ui-fleet

## Goal

Render one live Fleet card per lane with executor, model, SDD phase, fanout group, feature, worktree, attempt, elapsed time, status shape, latest activity, token/cost, and tool-rate indicators.

## Done criteria

- [ ] View tests cover empty, running, blocked, and progress-rich states without color-only status.
- [ ] Only the two allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.