---
id: ui-approvals
executor: agy
routed_by: approvals view lane selected because the existing individual decision contract must remain visible in the full console
model: gemini-3.7-flash-high
read_only: false
allowed_paths: ["internal/serve/static/app.js","internal/serve/static/index.html","internal/serve/static_test.go"]
feature: control-room-ui-views
parent_ref: refs/heads/control-room/control-room-ui-views
base_sha: f15b206f2668c5ee9d5d1d4ecbc357813b532691
expected_parent_sha: f15b206f2668c5ee9d5d1d4ecbc357813b532691
legacy_main: false
---

# Apply ui-approvals

## Goal

Render the existing approvals inbox inside the full console, including evidence and defect actions, while making bulk approval impossible in the UI as well as the server.

## Done criteria

- [ ] View tests cover pending, decided, evidence, defect, and bulk-action refusal states.
- [ ] Only the declared allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.