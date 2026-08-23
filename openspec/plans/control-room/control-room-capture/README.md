# control-room-capture

## Scope

Add the optional streaming executor path and per-lane progress writer. The contract packet is deliberately separate from the three executor adapters; the writer follows all adapters. New decoder files isolate the unverified external stream formats.

## Non-scope

Do not change ledger schema/API, HTTP/SSE, UI, or result-envelope routing. A decoder must never make telemetry parsing part of lane success/failure control flow.

## Exact allowed paths

- `internal/executor/executor.go`, `executor_stream_test.go`
- `internal/executor/agy.go`, `agy_stream.go`, `agy_stream_test.go`
- `internal/executor/cursor_agent.go`, `cursor_stream.go`, `cursor_stream_test.go`
- `internal/executor/opencode.go`, `opencode_stream.go`, `opencode_stream_test.go`
- `internal/run/batch.go`, `internal/run/progress.go`, `internal/run/progress_test.go`

## Acceptance criteria

- Nil progress channel preserves today's blocking JSON behavior.
- Each executor's stream shape is empirically verified before its tolerant decoder is finalized; unknown or malformed telemetry degrades to the blocking path and records a note.
- Progress writes are batched and best-effort; a failed progress insert cannot fail a lane.
- Same-wave decoder paths are disjoint and the writer waits for all three adapters.

## Definition of done

All five DAG waves exit 0 in order, focused executor/run tests pass, `lucind-ai check` passes, and stream evidence is stored in the feature's result envelopes.
