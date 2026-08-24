# Spec Lens C — Orphan Reconciliation & Live-Spec Conflicts: Lane Status Observability

## Assumed requirements

This lens specifies `orphan-lane-reconciliation` (persisting runner PID at registration and sweeping dead-PID `running` lanes to `failed` with diagnostic notes). It audits live specs for `lane-execution`, `read-only-packet-schema`, and `batch-wave-view` to evaluate conflicts, confirm classifications, and author `batch-wave-view`'s requirement delta. Finally, it notes the schema v7 dependency between `orphan-lane-reconciliation` (`runs.pid`) and `lane-progress-telemetry` (`lane_progress` usage columns) across the shared SQLite STRICT migration seam.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| orphan-lane-reconciliation | New | openspec/specs/orphan-lane-reconciliation/spec.md | None |
| lane-execution | Existing | openspec/changes/lane-status-observability/specs/lane-execution/spec.md | openspec/specs/lane-execution/spec.md:1 |
| read-only-packet-schema | Existing | openspec/changes/lane-status-observability/specs/read-only-packet-schema/spec.md | openspec/specs/read-only-packet-schema/spec.md:1 |
| batch-wave-view | Existing | openspec/changes/lane-status-observability/specs/batch-wave-view/spec.md | openspec/specs/batch-wave-view/spec.md:1 |

## ADDED Requirements

### Requirement: Orphaned lane reconciliation

`RegisterRun` MUST record the runner process identifier (PID) upon run registration. The server MUST execute an orphan reconciliation sweep at startup and on a periodic ticker. When the sweep detects a run whose recorded process identifier is no longer alive, it MUST transition all associated lanes in `running` status to `failed` status and MUST append an `EventLaneNote` recording that the driving process terminated. When the recorded PID is alive, the sweep MUST leave `running` lanes unchanged.

**Terminal consumer**:
- Run PID recording: `cmd/lucind-ai/cli.go:314-321` invoking `internal/ledger/runs.go:29-40` (`RegisterRun`).
- Server sweeper: `internal/serve` startup and periodic ticker routine.
- Status transition: `internal/ledger/ledger.go:452-475` (`SetStatus`) and `internal/ledger/lanes_meta.go:39-83` / `internal/ledger/ledger.go:443` (`AppendEvent` with `EventLaneNote`).

#### Scenario: Dead-process lane swept to failed

- GIVEN a run with recorded PID whose process terminated while a lane remains `running`
- WHEN the server executes an orphan reconciliation sweep
- THEN the lane MUST become `failed` and record an explanatory `EventLaneNote`

#### Scenario: Active process lanes unchanged

- GIVEN a run with recorded PID whose process is alive
- WHEN the server executes an orphan reconciliation sweep
- THEN `running` lanes MUST remain untouched

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| lane-execution | openspec/specs/lane-execution/spec.md:1 | 3 | 6 | Yes (`Lane metadata dispatch persistence`) |
| read-only-packet-schema | openspec/specs/read-only-packet-schema/spec.md:1 | 5 | 11 | Yes (`Extended packet frontmatter parsing`) |
| batch-wave-view | openspec/specs/batch-wave-view/spec.md:1 | 1 | 2 | Yes (`Batch and DAG Wave Inspection`) |

## Conflicts

None. The change extends live specifications without contradicting lifecycle approval wait gates, read-only packet schema invariants, or barrier release semantics.

## Classification Corrections

- **lane-execution**: Confirmed ADDED. `Lane metadata dispatch persistence` adds a dispatch lifecycle step without changing `Gate Placement in the Lifecycle`, `Resolve Before Barrier Observation`, or `Additive Schema, Unchanged Enum`.
- **read-only-packet-schema**: Confirmed ADDED. `Extended packet frontmatter parsing` adds optional field parsing without altering `Frontmatter Read-Only Field Parsing`, `Default Value and Backward Compatibility`, `Explicit Flag Only — No Inference`, `The Envelope Cannot Declare or Override Mode`, or `Additive Rollback`.
- **Finding**: Confirmed ADDED for both capabilities.

## MODIFIED Full Blocks

### Requirement: Batch and DAG Wave Inspection

**Source**: `openspec/specs/batch-wave-view/spec.md:9` — 2 scenarios

The dashboard UI MUST display active batch execution status, DAG wave grouping, per-lane lifecycle status (`pending`, `running`, `done`, `blocked`, `deviated`, `failed`), assigned executor, worktree directory path, per-lane execution deadline, barrier release state (Released, integration eligibility for completed lanes, and preservation for non-done worktrees), lane dispatch metadata (`model`, `agent`, `sdd_phase`, `fanout_group`, `feature`/`skill` when present), a link to the dispatched packet body markdown endpoint, structured progress telemetry metrics (total tokens, USD cost, and tool rates when emitted), and diagnostic notes for swept-orphan failures.
(Previously: Dashboard displayed status, wave grouping, executor, worktree path, deadline, and barrier state without lane dispatch metadata, packet body links, structured telemetry metrics, or swept-orphan notes.)

#### Scenario: Wave grouping and lane lifecycle inspection

- GIVEN an active batch execution with multi-wave DAG packet dependencies
- WHEN the operator inspects the batch-wave view
- THEN each lane MUST display status, assigned executor, worktree path, DAG
  wave group, and deadline

#### Scenario: Barrier release with mixed terminal statuses

- GIVEN an evaluated batch with one `done` lane and one `failed` or `deviated` lane
- WHEN barrier evaluation completes
- THEN the UI MUST display Released status, mark the `done` lane as
  integration-eligible, and show non-done worktrees as preserved

#### Scenario: Lane card metadata, packet link, and telemetry inspection

- GIVEN a lane dispatched with metadata and emitting progress telemetry
- WHEN the operator inspects the batch-wave view
- THEN the UI MUST render populated metadata fields, a link to the dispatched packet markdown endpoint, and numeric token, cost, and tool metrics

#### Scenario: Swept-orphan lane inspection

- GIVEN a lane transitioned to `failed` by the orphan sweep
- WHEN the operator inspects the batch-wave view
- THEN the UI MUST display the lane in `failed` status and render the explanatory sweep note

## Removals and Renames

None. No requirements are removed or renamed by this change.

## Cross-Cutting Schema v7 Note

Schema migration v7 introduces one transactional migration (`internal/ledger/schema.go:182-219,221-308` pattern, upgrading from `schemaVersion = 6` at `internal/ledger/schema.go:10`) rebuilding two STRICT SQLite tables via create-copy-drop-rename:
1. `runs`: adds `pid INTEGER` for `orphan-lane-reconciliation` (`internal/ledger/runs.go:16-24,29-40`, `cmd/lucind-ai/cli.go:314-321`).
2. `lane_progress`: adds numeric usage/tool columns for `lane-progress-telemetry` (`internal/ledger/progress.go:15-20`, `internal/executor/executor.go:17-21`).

Both capabilities share the same v6→v7 table rebuild seam because STRICT tables cannot widen in place. The synthesizer must unify them into one coherent v7 design.

## Open Questions

- [ ] What is the exact ticker interval for the periodic orphan reconciliation sweep in `internal/serve`? (Proposal Open Question 3; design must decide interval).
- [ ] What syscall or mechanism should be used for PID-liveness detection (`/proc/<pid>` vs `syscall.Kill(pid, 0)`), and is cross-platform portability beyond Linux required? (Proposal Open Question 4; design must specify implementation).
- [ ] What are the exact frontmatter key names for static SDD metadata (`sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs `generated_by`)? (Proposal Open Question 1; owned by Lens A).
- [ ] How should dispatched packet path be persisted: `LaneMetadata` JSON in `EventLaneNote` vs a `lanes` table column? (Proposal Open Question 2; owned by Lens B).
- [ ] Should DAG packet generator types (`internal/dag/parse.go:21-37` `Node` and `internal/dag/emit.go:11-60` `EmitPacketContent`) receive the new fields in this change or a follow-up? (Proposal Open Question 5).
- [ ] Workflow precedence: the general `sdd-spec` skill describes single-agent multi-file generation under `specs/`, whereas this change executes parallel lens decomposition writing `spec-lens-c.md` only; the synthesizer unifies the full spec tree.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:160-174` | CLI parses packet flags and holds on-disk packet paths in memory |
| `cmd/lucind-ai/cli.go:314-321` | Production `RegisterRun` call site currently registers runs without PID |
| `internal/dag/emit.go:11-60` | `EmitPacketContent` packet emission function |
| `internal/dag/parse.go:21-37` | DAG `Node` parser definition |
| `internal/executor/executor.go:17-21` | `ProgressEvent` struct definition lacks numeric usage and cost metrics |
| `internal/ledger/lanes_meta.go:12` | `lane_metadata:v1:` audit note prefix constant |
| `internal/ledger/lanes_meta.go:20-32` | `LaneMetadata` struct carries model, agent, SDD phase, and group fields |
| `internal/ledger/lanes_meta.go:39-83` | `UpdateLaneMetadata` updates columns and appends `EventLaneNote` audit JSON |
| `internal/ledger/lanes_meta.go:89-100` | `GetLaneMetadata` retrieves audited metadata snapshot |
| `internal/ledger/ledger.go:443` | `EventLaneNote` constant definition |
| `internal/ledger/ledger.go:452-475` | `SetStatus` transactionally updates lane status and writes `lane_status_changed` |
| `internal/ledger/progress.go:15-20` | `LaneProgress` struct definition lacks structured usage fields |
| `internal/ledger/runs.go:16-24` | `Run` struct definition currently lacks `PID` field |
| `internal/ledger/runs.go:29-40` | `RegisterRun` persists run row to SQLite `runs` table |
| `internal/ledger/schema.go:10` | Current `schemaVersion` constant set to 6 |
| `internal/ledger/schema.go:182-219` | `migrateV4ToV5DDL` STRICT create-copy-drop-rename migration pattern |
| `internal/ledger/schema.go:221-308` | `migrateV5ToV6DDL` STRICT create-copy-drop-rename migration pattern |
| `internal/ledger/schema.go:226-234` | Current `runs` table schema definition without `pid` column |
| `internal/ledger/schema.go:298-305` | Current `lane_progress` table schema definition without numeric telemetry columns |
| `internal/packet/packet.go:33-75` | `Packet` struct definition |
| `internal/packet/packet.go:78-167` | `Parse` decodes YAML frontmatter keys into `Packet` |
| `internal/run/batch.go:184-193` | `ensureLaneFailed` invokes `RegisterLane` without subsequent `UpdateLaneMetadata` |
| `internal/run/run.go:334-344` | `Execute` invokes `RegisterLane` without subsequent `UpdateLaneMetadata` |
| `internal/run/run.go:355` | `Execute` transitions lane to `running` status via `SetStatus` |
| `internal/serve/handlers.go:190` | `NewHandlerWithConfig` registers HTTP mux endpoints |
| `internal/serve/model.go:163-184` | `Lane` DTO definition in serve layer |
| `internal/serve/model.go:187-193` | `LaneProgress` DTO definition in serve layer |
| `internal/serve/model.go:288-301` | `ListLanes` queries lanes and retrieves lane metadata |
| `internal/serve/static/app.js:532-538` | UI falls back to "Unavailable" when lane metadata is empty |
| `internal/serve/static/app.js:542-544` | UI extracts `total_tokens`, `cost_usd`, and `tool_rate` from telemetry |
| `openspec/changes/lane-status-observability/proposal.md:1` | Proposal candidate selection and scope definition |
| `openspec/changes/lane-status-observability/proposal.md:37-46` | Capabilities breakdown for `lane-status-observability` |
| `openspec/changes/lane-status-observability/proposal.md:138-151` | Proposal draft requirement and scenarios for orphaned lane reconciliation |
| `openspec/specs/batch-wave-view/spec.md:1` | Batch Wave View live specification header |
| `openspec/specs/batch-wave-view/spec.md:9` | "Batch and DAG Wave Inspection" is the sole requirement in `batch-wave-view` |
| `openspec/specs/lane-execution/spec.md:1` | Lane Execution live specification header |
| `openspec/specs/lane-execution/spec.md:10` | "Gate Placement in the Lifecycle" live requirement in `lane-execution` |
| `openspec/specs/lane-execution/spec.md:27` | "Resolve Before Barrier Observation" live requirement in `lane-execution` |
| `openspec/specs/lane-execution/spec.md:44` | "Additive Schema, Unchanged Enum" live requirement in `lane-execution` |
| `openspec/specs/read-only-packet-schema/spec.md:1` | Read-Only Packet Schema live specification header |
| `openspec/specs/read-only-packet-schema/spec.md:9` | "Frontmatter Read-Only Field Parsing" live requirement |
| `openspec/specs/read-only-packet-schema/spec.md:28` | "Default Value and Backward Compatibility" live requirement |
| `openspec/specs/read-only-packet-schema/spec.md:47` | "Explicit Flag Only — No Inference" live requirement |
| `openspec/specs/read-only-packet-schema/spec.md:61` | "The Envelope Cannot Declare or Override Mode" live requirement |
| `openspec/specs/read-only-packet-schema/spec.md:75` | "Additive Rollback" live requirement |
