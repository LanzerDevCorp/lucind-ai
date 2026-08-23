---
id: schema-v6
executor: opencode
routed_by: schema migration lane selected because v5 is the current ledger version and the proposal requires v6 telemetry tables
model: openai/gpt-5.6-sol
agent: build
feature: control-room-telemetry
parent_ref: refs/heads/control-room/control-room-telemetry
base_sha: 25a520e98217ee911c5f77286cc4c2ccb984f344
expected_parent_sha: 25a520e98217ee911c5f77286cc4c2ccb984f344
allowed_paths: ["internal/ledger/schema.go","internal/ledger/schema_test.go"]
---

# Apply schema-v6

## Goal

Implement and test ledger schema migration v6 exactly from the approved Control Room proposal. Preserve all v5 data, add the `runs` and `lane_progress` tables, add the declared lane metadata columns and indexes, open event type validation, and retention support without changing prior migration behavior.

## Preconditions

- The feature parent ref exists and the SHA fields were refreshed by the runbook.
- `internal/ledger/schema.go` is still at schema version 5 before this packet.

## Done criteria

- [ ] Focused migration tests cover v5-to-v6, idempotent reopen, preservation, constraints, and indexes.
- [ ] `go test ./internal/ledger` passes and the worktree contains only the two allowed paths.
- [ ] The implementation is committed with a conventional commit and no AI attribution.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.