---
id: cli-wiring
executor: agy
routed_by: application wiring lane selected after all terminal views exist so the embedded shell and explicit dispatch gate have consumers
model: gemini-3.7-flash-high
read_only: false
allowed_paths: ["internal/serve/static/app.js","internal/serve/static/index.html","internal/serve/static_test.go","cmd/lucind-ai/cli.go","cmd/lucind-ai/cli_control_room_test.go"]
feature: control-room-ui-views
parent_ref: refs/heads/control-room/control-room-ui-views
base_sha: 036ff23f4ffb6405110a39c3a923779abeaf10b6
expected_parent_sha: 036ff23f4ffb6405110a39c3a923779abeaf10b6
legacy_main: false
---

# Apply cli-wiring

## Goal

Wire all six views into the embedded application, retain `/api/state` polling fallback, and expose an explicit `--enable-dispatch` serve flag whose default is disabled and whose control path requires the proposal's origin/session-token guard.

## Done criteria

- [ ] CLI tests prove default-disabled and explicit-enabled behavior, and static wiring tests prove all view modules load.
- [ ] Only the declared allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.