# Verify: lane-status-observability

**Overall verdict: PASSED**

Single-round verification. Orchestrator dispatched one qualitative judgment lane (`agy`) rather than the standard dual (`agy` + `cursor-agent`) dispatch, per explicit user instruction. The lane reported `done` with no blocking findings; task-completion and mechanical gates both green.

## Stage 1 — Mechanical check

Candidate `lucind/apply-lane-status-observability` at `efdff0d8a0ce7cdd0ef8c03f095a129cfe4b4dff`: **passed**, 48.35s, exit 0, every package green under `lucind-checks.sh` (`go test ./...`). Frozen transcript committed to the candidate branch as `verify-mechanical.log` (commit `677ff57`). The judgment lane did not re-run it.

## Stage 2 — Qualitative dispatch

| Round | Candidate | Lane | Executor | Result |
|---|---|---|---|---|
| 1 | `677ff57` | `verify-lane-status-observability-agy` | agy `gemini-3.7-flash-high` | done, no blocker raised |

Only one lane was dispatched (`agy`); the standard dual `agy` + `cursor-agent` protocol was skipped by explicit user decision after reviewing the single lane's clean result. This is a deliberate scope reduction, not an omission — recorded here so it isn't mistaken for a lapsed process.

## Findings

Both are confirmations/observations from the `agy` lane; neither is a spec violation blocking this change.

1. **`lanes.started_at` gap confirmed, correctly out of scope.** `lanes.started_at` is never written by `RegisterLane` or `SetStatus` (`internal/ledger/ledger.go:463-484`), so `deriveToolRate` (`internal/serve/model.go:372-381`) always receives a nil `startedAt` and falls back to the `toolRateFloorMinutes` (1s) floor, computing `toolCalls * 60` tools/min unconditionally. This is a pre-existing ledger lifecycle gap, safely handled by `deriveToolRate`'s fallback, not a requirement this change introduced or violated. Already tracked as GitHub issue #1, filed and independently verified before this verify round.

2. **Sweeper CLI-wiring test asserts on source text, not runtime behavior.** `TestServeStartsSweeperBesideHub` (`cmd/lucind-ai/cli_test.go:2131-2150`) verifies sweeper initialization in `serveDispatch` by reading and pattern-matching the source text of `cli.go`, rather than asserting the goroutine actually dispatches or the HTTP server's lifecycle. Sweeper mechanics themselves are covered properly by `internal/serve/sweeper_test.go`. A test-quality gap, not a coverage gap — carried forward as a follow-up.

## Done criteria — traced by the lane

Every indirection this change introduced was traced to a terminal consumer with `file:line` evidence: packet frontmatter (`SDDPhase`/`FanoutGroup`/`Skill`) parsed → persisted → rendered in `app.js`; `PacketPath` set in `cli.go` → served by `handlePacketBody` → linked in the UI; Run `PID` recorded → consumed by the Sweeper's `processAlive` liveness check; telemetry (`TotalTokens`/`CostUSD`/`ToolCalls`) decoded across all four executors → persisted → rendered on fleet cards; `ToolRate` derived and displayed. Full citations in `.lucind/results/verify-lane-status-observability-agy.json` (preserved under `envelopes/` at archive).

Worktree carried no unique commits and no working-tree drift relative to its birth point (`git status --porcelain` empty; `HEAD` equals `git merge-base HEAD <primary HEAD>`).

## Follow-ups

- GitHub issue #1 (`lanes.started_at` never written) — pre-existing, tracked separately, not a blocker for this change.
- GitHub issue #2 (`TestLeaseAcquisitionAndMonotonicFence`/`TestConcurrentLeaseAcquisition` flakiness under `-race`) — pre-existing, unrelated to this change, surfaced only as a retry-needed flake during its own mechanical checks.
- `TestServeStartsSweeperBesideHub` asserts on source text rather than runtime behavior (finding 2 above) — worth strengthening to a real runtime assertion in a future change; not blocking.
- The standard dual-lane (`agy` + `cursor-agent`) verify dispatch was reduced to a single `agy` lane by explicit user decision. If stronger corroboration is ever wanted for this change, a `cursor-agent` lane can still be dispatched against the same frozen mechanical log.
