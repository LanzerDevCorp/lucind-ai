---
id: ledger-runs
executor: opencode
routed_by: run lifecycle API lane selected because schema v6 provides durable run identity and status fields
model: openai/gpt-5.6-sol
agent: build
feature: control-room-ledger
parent_ref: refs/heads/control-room/control-room-ledger
base_sha: 8dd263066b1b1ba4fcfa5b66d571861531d0be21
expected_parent_sha: 8dd263066b1b1ba4fcfa5b66d571861531d0be21
allowed_paths: ["internal/ledger/runs.go","internal/ledger/runs_test.go"]
---

# Apply ledger-runs

## Goal

Implement the new-file ledger methods that create, update, list, and finish `runs` rows with the proposal's change, phase, wave, feature, parent, base, lane count, status, and timestamps. Make idempotency and terminal transitions observable through focused tests.

## Done criteria

- [ ] `internal/ledger/runs.go` and its test file are the only authored paths.
- [ ] `go test ./internal/ledger -run Run` passes and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.