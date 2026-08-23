---
id: ledger-lanes
executor: opencode
routed_by: lane metadata API lane selected because fleet rendering needs model phase fanout feature and path fields persisted
model: openai/gpt-5.6-sol
agent: build
feature: control-room-ledger
parent_ref: refs/heads/control-room/control-room-ledger
base_sha: 8dd263066b1b1ba4fcfa5b66d571861531d0be21
expected_parent_sha: 8dd263066b1b1ba4fcfa5b66d571861531d0be21
allowed_paths: ["internal/ledger/lanes_meta.go","internal/ledger/lanes_meta_test.go"]
---

# Apply ledger-lanes

## Goal

Implement new-file ledger methods that write and query v6 lane metadata: model, agent, SDD phase, fanout group, change, feature, allowed paths, dependencies, and body digest. Preserve existing lane identity and make updates auditable.

## Done criteria

- [ ] `internal/ledger/lanes_meta.go` and its test file are the only authored paths.
- [ ] `go test ./internal/ledger -run Lane` passes and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.