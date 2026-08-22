---
id: ui-timeline
executor: agy
routed_by: timeline view lane selected because event and progress cursors are available from the server stream
model: gemini-3.7-flash-high
read_only: false
allowed_paths: ["internal/serve/static/app.js","internal/serve/static_test.go"]
feature: control-room-ui-views
parent_ref: refs/heads/control-room/control-room-ui-views
base_sha: 1ff2bb2ae3c5493fe5d045d41e0f119c60b4616d
expected_parent_sha: 1ff2bb2ae3c5493fe5d045d41e0f119c60b4616d
legacy_main: false
---

# Apply ui-timeline

## Goal

Render a bounded, filterable merged timeline of events, integration events, and lane progress with stable cursor ordering and virtualized large-result behavior.

## Done criteria

- [ ] View tests cover ordering, filters, cursor continuation, empty data, and large feeds.
- [ ] Only the declared allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.