---
id: run-writer
executor: opencode
routed_by: progress writer lane selected after all executor adapters exist so one goroutine per lane can persist decoded events
model: openai/gpt-5.6-sol
agent: build
feature: control-room-capture
parent_ref: refs/heads/control-room/control-room-capture
base_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
expected_parent_sha: 7853eb07df3c8ebe36577e78664a8a8ae3e4a19f
allowed_paths: ["internal/run/batch.go","internal/run/progress.go","internal/run/progress_test.go"]
---

# Apply run-writer

## Goal

Wire one progress channel and writer goroutine per running lane, batch ledger inserts every 250 ms or 32 events, and make insert errors observable without changing lane status. Preserve the existing barrier and result-envelope contract.

## Done criteria

- [ ] Focused batch tests cover flush thresholds, shutdown flush, concurrent lanes, and dropped progress writes.
- [ ] Only the three allowed paths change and the commit is recorded.


## Result envelope

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.