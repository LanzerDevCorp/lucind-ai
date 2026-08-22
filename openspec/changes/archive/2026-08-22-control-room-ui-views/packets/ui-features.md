---
id: ui-features
executor: agy
routed_by: feature view lane selected because feature leases attempts and reconciliation payloads are exposed by the server
model: gemini-3.7-flash-high
read_only: false
allowed_paths: ["internal/serve/static/app.js","internal/serve/static_test.go"]
feature: control-room-ui-views
parent_ref: refs/heads/control-room/control-room-ui-views
base_sha: 2fd04c678635aafb38c942fb5868bee2482120ad
expected_parent_sha: 2fd04c678635aafb38c942fb5868bee2482120ad
legacy_main: false
---

# Apply ui-features

## Goal

Render feature swimlanes with parent/base refs, lease holder/fence/live TTL, integration attempt states, overlap evidence, and reconciliation badges from API data.

## Done criteria

- [ ] View tests cover active, expired, blocked, promoted, and reconciliation-required states.
- [ ] Only the declared allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.