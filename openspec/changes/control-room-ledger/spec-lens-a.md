# Spec Lens A — Capabilities & Requirements: Control Room Ledger

## Assumed requirements

This change specifies seven requirements across five capabilities: three new capabilities (`run-lifecycle-ledger`, `lane-progress-stream`, and `progress-stream-pruning`) targeting new full specs, and two existing capabilities (`lane-execution` and `approvals-web-ui`) targeting delta specs. The new full specs define `First-Class Run Persistence` and `Primary-Root Isolation Preservation` for `run-lifecycle-ledger`, `Progress Ingest and Cursor Tail` for `lane-progress-stream`, and `Isolated Progress Cutoff Pruning` for `progress-stream-pruning`. The delta specs introduce new added requirements while preserving all existing live behaviors: `lane-execution` receives `Lane Dispatch Metadata Persistence` and `Admitted Run Status Event Types`, and `approvals-web-ui` receives `Shell-Free Run and Progress Model DTOs`. No existing requirements are modified, removed, or renamed.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `run-lifecycle-ledger` | New | `openspec/specs/run-lifecycle-ledger/spec.md` | |
| `lane-progress-stream` | New | `openspec/specs/lane-progress-stream/spec.md` | |
| `progress-stream-pruning` | New | `openspec/specs/progress-stream-pruning/spec.md` | |
| `lane-execution` | Existing | `openspec/changes/control-room-ledger/specs/lane-execution/spec.md` | `openspec/specs/lane-execution/spec.md:1-62` |
| `approvals-web-ui` | Existing | `openspec/changes/control-room-ledger/specs/approvals-web-ui/spec.md` | `openspec/specs/approvals-web-ui/spec.md:1-83` |

## ADDED Requirements

### Requirement: First-Class Run Persistence

The ledger MUST store a durable row in `runs` at CLI dispatch with status `running` and UTC `started_at`, and MUST update the row to terminal status with non-null UTC `ended_at` when all lanes complete. Run lifecycle status MUST NOT be derived solely by scanning lanes.

**Terminal consumer**: `cmd/lucind-ai/cli.go:282-290` (CLI dispatch registration) and `internal/ledger/runs.go` (new, introduced by this change).

### Requirement: Lane Dispatch Metadata Persistence

Schema v6 `lanes` table MUST include nullable `model`, `agent`, and `feature` columns, and `RegisterLane` MUST persist these metadata attributes when present on `packet.Packet`. Transactional migration to schema v6 MUST preserve existing lane records with null or empty metadata values.

**Terminal consumer**: `internal/ledger/ledger.go:255-282` (lane registration) and `internal/packet/packet.go:43-54` (packet dispatch attributes).

### Requirement: Admitted Run Status Event Types

Schema v6 `events` table CHECK constraint MUST admit `run_status_changed` alongside existing event types (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`).

**Terminal consumer**: `internal/ledger/schema.go:38-39` (schema CHECK constraint) and `internal/run/run.go:425-435` (event appending).

### Requirement: Progress Ingest and Cursor Tail

The ledger MUST append sequenced mid-flight progress chunks to `lane_progress` with `(run_id, lane_id, seq)` and MUST return chunks with `seq > afterSeq` in strictly ascending sequence order without querying or appending to `events`.

**Terminal consumer**: `internal/run/run.go:422-434` (progress ingest) and `internal/ledger/progress.go` (new, introduced by this change).

### Requirement: Isolated Progress Cutoff Pruning

The ledger MUST delete `lane_progress` rows older than a specified cutoff timestamp without deleting, modifying, or cascading onto rows in `runs`, `lanes`, `events`, or `approvals`.

**Terminal consumer**: `internal/ledger/ledger.go:877-890` (analog: `PruneIntegrationEvents`) and `internal/ledger/progress.go` (new, introduced by this change).

### Requirement: Shell-Free Run and Progress Model DTOs

The `serve.Model` query surface MUST provide typed, shell-free read methods for run summaries and lane progress tails backed directly by SQLite queries without executing subprocess or git commands.

**Terminal consumer**: `internal/serve/model.go:14-25` (DTO query model) and `internal/serve/handlers.go:79-85` (HTTP state endpoint).

### Requirement: Primary-Root Isolation Preservation

`ledger.Open` MUST resolve database paths via `ledgerpath.Resolve` to `<primaryRoot>/.lucind/lucind.db`, and `lucind-ai run` and `lucind-ai serve` MUST exit with code 1 when invoked inside a linked worktree.

**Terminal consumer**: `cmd/lucind-ai/cli.go:277-280,702-705` (worktree guard) and `internal/ledgerpath/ledgerpath.go:36-38` (path resolver).

## Open Questions

- [ ] Should `lane_progress` auto-pruning be triggered periodically by `lucind-ai serve` or on-demand via CLI commands?
- [ ] Should run status transitions emit `run_status_changed` via `AppendEvent` (`internal/ledger/ledger.go:366-381`) or via dedicated transaction helpers?
- [ ] Should progress `message` remain a raw text string or adopt structured JSON distinguishing stdout, stderr, and system events?
- [ ] Execution-topology note: Spec authoring is partitioned across three parallel lenses (Lens A, Lens B, Lens C) feeding a synthesis lane per packet authorization, superseding single-subagent delta spec generation in `~/.claude/skills/sdd-spec/SKILL.md`.
