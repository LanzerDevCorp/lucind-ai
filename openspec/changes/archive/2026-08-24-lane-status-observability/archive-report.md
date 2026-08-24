# Archive Report: Lane Status Observability

## Verdict

**PASSED** — Single-round verification. The orchestrator dispatched one qualitative judgment lane (`verify-lane-status-observability-agy` via `agy` `gemini-3.7-flash-high`) against candidate commit `677ff57` (carrying mechanical check log `verify-mechanical.log`, exit 0, all packages green under `lucind-checks.sh`). The lane reported `done` with no blocking findings; task-completion and mechanical gates both passed. Standard dual dispatch (`agy` + `cursor-agent`) was reduced to a single lane by explicit user decision after reviewing the clean result.

## What Shipped

Six capability specs were added or updated to reflect the implemented observability features:

- `dispatched-packet-body` (Added capability): 1 requirement (`Dispatched packet body inspection`), 3 scenarios (`Retrieve packet content`, `Unknown lane returns 404`, `Missing or unreadable packet file on disk`). Exposes `GET /api/packets/{runID}/{laneID}` to serve verbatim markdown of dispatched packets.
- `lane-progress-telemetry` (Added capability): 1 requirement (`Structured progress telemetry`), 3 scenarios (`Decoders populate usage`, `Cursor-agent emits zeroed metrics`, `Real-time telemetry broadcast`). Adds numeric token totals, USD cost, and tool-call counts across all four executor stream decoders and broadcasts via WebSocket.
- `orphan-lane-reconciliation` (Added capability): 1 requirement (`Orphaned lane reconciliation`), 2 scenarios (`Dead-process lane swept to failed`, `Active process lanes unchanged`). Implements runner PID capture on run registration and a background `Sweeper` that transitions dead-process running lanes to failed with explanatory notes.
- `read-only-packet-schema` (Modified capability): 1 added requirement (`Extended packet frontmatter parsing`), 3 scenarios (`Parse frontmatter keys`, `Optional keys omitted`, `Empty frontmatter values handled`). Parses optional `sdd_phase`, `fanout_group`, and `skill` keys from packet frontmatter.
- `batch-wave-view` (Modified capability): 1 modified requirement (`Batch and DAG Wave Inspection`), 4 scenarios (`Wave grouping and lane lifecycle inspection`, `Barrier release with mixed terminal statuses`, `Lane card metadata, packet link, and telemetry inspection`, `Swept-orphan lane inspection`). Updates the dashboard to display metadata, packet markdown links, telemetry numerics/rates, and orphan sweep notes.
- `lane-execution` (Modified capability): 1 added requirement (`Lane metadata dispatch persistence`), 3 scenarios (`Dispatch persists metadata`, `Historical rows preserved`, `Pre-dispatch failure persists metadata`). Persists packet and routing metadata upon lane registration and pre-dispatch failure.

Total: 3 new capability specifications created, 3 existing capability specifications updated. 6 requirements synced (5 added, 1 modified) across 18 scenarios (16 added, 2 updated/preserved).

## Dispatch Record

The SDD cycle for `lane-status-observability` executed across the following phases:

| Phase | Lanes / Artifacts | Executor / Model | Outcome |
|---|---|---|---|
| Propose | 3 lens explorations (`propose-lens-a`, `propose-lens-b`, `propose-lens-c`) + synthesis | Fan-out synthesis | Synthesized `proposal.md` |
| Spec | 3 lens explorations (`spec-lens-a`, `spec-lens-b`, `spec-lens-c`) + synthesis | Fan-out synthesis | Synthesized delta specs in `specs/` |
| Design | 3 lens explorations (`design-lens-a`, `design-lens-b`, `design-lens-c`) + synthesis | Fan-out synthesis | Synthesized `design.md` (Schema v7) |
| Tasks | 3 lens explorations (`tasks-lens-a`, `tasks-lens-b`, `tasks-lens-c`) + synthesis | Fan-out synthesis | Synthesized `tasks.md` (41 tasks) |
| Apply | 1 sequential apply lane (3 work units across commits `894badd`, `8a6062b`, `efdff0d`) | Sequential apply | 41/41 tasks complete |
| Verify (Mechanical) | 1 frozen check run (`verify-mechanical.log`, commit `677ff57`) | `lucind-checks.sh` | Passed (exit 0, 48.35s) |
| Verify (Qualitative) | 1 judgment lane (`verify-lane-status-observability-agy.md`) | `agy` (`gemini-3.7-flash-high`) | `done`, verdict PASSED |
| Archive | 1 mechanical archival lane (`archive-lane-status-observability.md`) | `agy` (`gemini-3.7-flash-high`) | `done` |

Preserved dispatch artifacts under `openspec/changes/lane-status-observability/`:
- `packets/verify-lane-status-observability-agy.md`
- `packets/archive-lane-status-observability.md`
- `envelopes/verify-lane-status-observability-agy.json`

## Follow-ups

- **GitHub issue #1**: `lanes.started_at` is never written by `RegisterLane` or `SetStatus` (`internal/ledger/ledger.go:463-484`), causing `deriveToolRate` (`internal/serve/model.go:372-381`) to fall back to the 1-second floor (`toolRateFloorMinutes`). Pre-existing ledger lifecycle gap safely handled by fallback logic, tracked separately.
- **GitHub issue #2**: `TestLeaseAcquisitionAndMonotonicFence` / `TestConcurrentLeaseAcquisition` flakiness under `-race`. Pre-existing, unrelated to this change.
- **Sweeper CLI test assertion**: `TestServeStartsSweeperBesideHub` (`cmd/lucind-ai/cli_test.go:2131-2150`) asserts sweeper wiring by matching source text in `cli.go` rather than runtime goroutine execution. Non-blocking test-quality observation.
- **Dual-lane verification scope reduction**: The standard dual-lane verify dispatch was reduced to a single `agy` lane by explicit user decision. A `cursor-agent` lane may be dispatched against `verify-mechanical.log` in the future if further corroboration is desired.

## Gaps and Contradictions

- **Artifacts**: All required artifacts (`proposal.md`, `specs/`, `design.md`, `tasks.md`, `verify.md`) and lens synthesis notes are present and complete.
- **Tasks**: All 41 implementation tasks in `tasks.md` were checked (`- [x]`) and verified prior to archival.
- **Verification**: No CRITICAL issues were raised in `verify.md`. Both findings are confirmed non-blocking observations.
- **Contradictions**: No uncorroborated claims or contradictions between intermediate snapshots and final candidate state were encountered.
