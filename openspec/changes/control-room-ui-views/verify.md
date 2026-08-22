# Verify: control-room-ui-views

**Overall verdict: PASSED**

One round, one lane. No blocker, no CRITICAL issue. Three findings, all confirmations rather than defects. One structural deviation from the plan was surfaced deliberately and cleared.

## Stage 1 — Mechanical check

Final run on `6a7dfa8c3da4f8087385f65ad86f006c13263647`: **passed**, 59.47s, exit 0, every package green under `CGO_ENABLED=0 go build ./... && go test ./... -race -count=1`. Frozen transcript: `verify-mechanical.log`. The judgment lane did not re-run it.

## Stage 2 — Qualitative dispatch

| Round | Candidate | Lane | Executor | Result |
|---|---|---|---|---|
| 1 | `6a7dfa8` | `verify-control-room-ui-views` | agy `gemini-3.7-flash-high` | done, no blocker raised |

The lane reported `done` with a complete envelope. Its integration attempt was reverted by the check gate on two known-flaky `internal/run` concurrency tests (`TestExecuteBatchConcurrentLedgerWritesDoNotErrorOrLoseData`, `TestExecuteBatchAppliesPerLaneDeadlineIndependently`). This is a read-only lane that produces no commit, so the revert discarded nothing, and the same suite had passed cleanly at the identical SHA minutes earlier — recorded in `verify-mechanical.log`.

## The deviation the lane was asked to check

Three of the four apply lanes were dispatched with `allowed_paths` naming files that do not exist in this repository's layout: `internal/serve/static/views/{features,timeline,approvals}.js`, `internal/serve/{features,timeline,approvals}_static_test.go`, and `cmd/lucind-ai/cli_control_room_test.go`. Every lane instead extended the existing `app.js`, `index.html`, `static_test.go`, and `cli_test.go`. Two of them (`ui-approvals`, `cli-wiring`) were therefore marked `deviated` and hand-integrated after their commits were inspected and found valid.

The packet asked the lane directly whether that consolidation dropped any spec requirement the separate-module decomposition had been carrying. It concluded it did not, citing the wiring site and the contract assertions that cover it.

## Findings

All three are confirmations with concrete citations; none is a defect.

1. **Consolidation preserved every capability.** `app.js:1854-1880` wires approvals, fleet grid, apply DAG, SDD flows, feature swimlanes, and timeline; covered by static asset contract assertions at `static_test.go:823-882`.

2. **Dispatch gating is secure by default.** `--enable-dispatch` defaults off and returns 403 when disabled; when enabled it enforces same-origin plus constant-time bearer token validation. `internal/serve/handlers.go:311-349`, `cmd/lucind-ai/cli.go:683-738`, covered end to end by `cmd/lucind-ai/cli_test.go:1200-1410`.

3. **Live store degrades correctly.** SSE streaming with 2-second polling fallback, DOM text-node updates rather than innerHTML (XSS containment), and keyed card patching to avoid flicker and scroll reset. `app.js:44-183, 386-421, 597-617, 1269-1300, 1699-1737`, covered by `static_test.go:168-223`.

## Follow-ups

- The `apply-dag.yaml` sidecars for this change name files that never existed. They are preserved as dispatched, for the record, but any re-dispatch from them would deviate again.
