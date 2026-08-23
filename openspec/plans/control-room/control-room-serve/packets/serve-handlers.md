---
id: serve-handlers
executor: opencode
routed_by: HTTP composition lane selected after model transport and worktree payloads exist so routes have terminal consumers
model: openai/gpt-5.6-sol
agent: build
feature: control-room-serve
parent_ref: refs/heads/control-room/control-room-serve
base_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
expected_parent_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
allowed_paths: ["internal/serve/handlers_api.go","internal/serve/handlers_api_test.go","internal/serve/handlers.go"]
---

# Apply serve-handlers

## Goal

Register the REST snapshot routes, `/api/stream` SSE route, guarded control endpoints, and compatibility fallback in the existing handler. Preserve loopback-only deployment and the individual-only approval/defect contract while making dispatch control disabled unless explicitly enabled, origin-checked, and token-checked.

## Done criteria

- [ ] Handler tests cover every declared read route, SSE resume/resync, control refusal by default, origin/token checks, and unchanged bulk approval rejection.
- [ ] Only the three allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.