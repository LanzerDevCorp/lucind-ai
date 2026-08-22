# Verify: control-room-ui-views

**Overall verdict: PASSED**

Two rounds. Round 1's judgment lane raised no blocker, but the orchestrator's own task-completion gate caught a defect the lane had missed: a method built by this change with no consumer anywhere. Round 2 closed it. No CRITICAL issue remains.

## Stage 1 — Mechanical check

Round 1 on `6a7dfa8c3da4f8087385f65ad86f006c13263647`: passed, 59.47s, exit 0.

Final run on `fea7459f7ab222243aa0ba51b7d39a430056fdef`: **passed**, 60.79s, exit 0, every package green under `CGO_ENABLED=0 go build ./... && go test ./... -race -count=1`. Frozen transcript: `verify-mechanical.log`. The judgment lane did not re-run either.

## Stage 2 — Qualitative dispatch

| Round | Candidate | Lane | Executor | Result |
|---|---|---|---|---|
| 1 | `6a7dfa8` | `verify-control-room-ui-views` | agy `gemini-3.7-flash-high` | done, no blocker raised |
| 2 | `de5f5cb` | `api-gaps` | agy `gemini-3.7-flash-high` | done, promoted, remediation |

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
- A single-lane qualitative verify passed a change containing dead code. The task-completion gate caught it; the lane did not. A dual dispatch, or a lane explicitly pointed at `tasks.md`'s unchecked boxes, would have caught it earlier.
- `tasks.md`'s four Open Questions (reconcile UI POST surface, lease countdown source, overlap evidence rendering, `NewHandler` model injection) remain open. They are design questions, not implementation tasks, and none blocks this change.

## Round 2 — the defect the judgment lane missed

Round 1's packet carried a done-criterion requiring that *every indirection introduced is demonstrably consumed by a terminal consumer*. The lane reported it satisfied. It was not.

`(*serve.Model).ListBatchLanes` and its `BatchLane` DTO were built by this change and had **zero callers** anywhere in the repository outside their own definition — `rg 'ListBatchLanes' --glob '!*_test.go'` returned only `model.go`. `tasks.md` had called for mounting it at `GET /api/batch/lanes`; that route was never mounted. The method was dead on arrival, and a spec-compliance reading alone did not surface it because no spec requirement names the route.

It was caught by checking `tasks.md`'s unchecked boxes against the code rather than trusting either the checkboxes or the lane's verdict. Three of `tasks.md`'s seven implementation tasks turned out to be genuinely undelivered, not stale.

Remediated in `fea7459` by the `api-gaps` lane:

- `GET /api/batch/{id}/lanes` mounted, calling `ListBatchLanes` — `handlers.go`. The literal path `tasks.md` guessed at (`/api/batch/lanes`) was rejected in favour of the path-segment style the surrounding handlers already use.
- `GET /api/approvals` mounted, sourcing from the same place the composed `/api/state` handler does.
- `TestBatchLanesRoundTrip` (`model_test.go`), `TestGetRoutesReturnJSON` (`server_test.go`), and a static asset contract test over the real rendered container ids (`static_test.go`).

413 insertions, no deletions, no existing test weakened.

## Task 4.1 is superseded, not incomplete

`tasks.md` 4.1 called for replacing the client's single `/api/state` poller with five per-panel fetches on a 2-second hot loop. The implementation instead converged on one composed `/api/state` endpoint plus an SSE stream with a 2-second polling fallback.

That is a better architecture than the one the task described, not a shortfall against it: it delivers push updates instead of a fixed poll, and it keeps one source of truth for composed state. Rewriting the client to poll five endpoints would be a regression. The task is marked `[~] SUPERSEDED` in `tasks.md` rather than checked, and is recorded here so the divergence is never mistaken for an omission.
