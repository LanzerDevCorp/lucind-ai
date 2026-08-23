---
id: ledger-progress
executor: opencode
routed_by: progress persistence API lane selected because decoded lane events need bounded durable writes and reads
model: openai/gpt-5.6-sol
agent: build
feature: control-room-ledger
parent_ref: refs/heads/control-room/control-room-ledger
base_sha: 8dd263066b1b1ba4fcfa5b66d571861531d0be21
expected_parent_sha: 8dd263066b1b1ba4fcfa5b66d571861531d0be21
allowed_paths: ["internal/ledger/progress.go","internal/ledger/progress_test.go"]
---

# Apply ledger-progress

## Goal

Implement batched append and ordered query APIs for `lane_progress`, including sequence handling, event validation, bounded retention, and best-effort error reporting hooks required by the capture writer.

## Done criteria

- [ ] `internal/ledger/progress.go` and its test file are the only authored paths.
- [ ] `go test ./internal/ledger -run Progress` passes and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.