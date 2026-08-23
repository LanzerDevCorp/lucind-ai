---
id: serve-model
executor: opencode
routed_by: server model lane selected because v6 ledger APIs exist but rich query data is not exposed over HTTP
model: openai/gpt-5.6-sol
agent: build
feature: control-room-serve
parent_ref: refs/heads/control-room/control-room-serve
base_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
expected_parent_sha: 28bbcc3eb08b183928efc4d931798873823d2a51
allowed_paths: ["internal/serve/model.go","internal/serve/model_test.go"]
---

# Apply serve-model

## Goal

Extend the shell-free serve query model with run, lane, progress, flow, overview, and derived payloads needed by the Control Room REST surface. Keep existing feature, lease, reconciliation, candidate, and audit queries compatible.

## Done criteria

- [ ] Model tests assert stable JSON-facing values and empty-list behavior.
- [ ] Only the two allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.