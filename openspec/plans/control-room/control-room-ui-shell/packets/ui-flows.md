---
id: ui-flows
executor: opencode
routed_by: SDD flows view lane selected after the shared store exists so phase and fanout data have one live source
model: openai/gpt-5.6-sol
agent: build
feature: control-room-ui-shell
parent_ref: refs/heads/control-room/control-room-ui-shell
base_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
expected_parent_sha: 35c00fa329a94d4e36f8ab46858a4a035219156d
allowed_paths: ["internal/serve/static/app.js","internal/serve/static_test.go"]
---

# Apply ui-flows

## Goal

Render each change's SDD rail from explore through archive, expand the three planning lenses and synthesis lane, and group apply/verify/archive work without displaying data absent from the server payload.

## Done criteria

- [ ] View tests cover complete, partial, and empty flows and visibly distinguish lens groups.
- [ ] Only the two allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.