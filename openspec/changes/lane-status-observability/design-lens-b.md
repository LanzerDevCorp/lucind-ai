# Design Lens B — Telemetry & Schema v7: Lane Status Observability

## Assumed architecture

The system captures structured telemetry across executors (`agy`, `claude`, `opencode`, `cursor-agent`) into SQLite ledger tables (`runs`, `lanes`, `lane_progress`), surfaced via `/api/state` and SSE to the dashboard. Schema v7 executes a single transactional STRICT rebuild of `runs` (adding `pid`) and `lane_progress` (adding numeric metrics) without historical backfill. The dashboard renders live fleet cards while orphan reconciliation marks dead-process lanes failed.

## Decision 1 — Telemetry field names and per-decoder mapping

**Choice**:
Add `TotalTokens int64`, `CostUSD float64`, `ToolCalls int64` to `executor.ProgressEvent` (`internal/executor/executor.go:18-21`), `ledger.LaneProgress` (`internal/ledger/progress.go:15-20`), and `serve.LaneProgress` (`internal/serve/model.go:186-193`, JSON tags `total_tokens`, `cost_usd`, `tool_calls`).
Per-decoder `TotalTokens` arithmetic:
- **agy** (`internal/executor/agy_stream.go:12-18`): `TotalTokens = int64(usage.TotalTokens)` directly from `agyUsage.TotalTokens` (`internal/executor/agy_stream.go:17`); `CostUSD = 0.0`.
- **claude** (`internal/executor/claude_stream.go:17-22`): `TotalTokens = int64(usage.InputTokens + usage.OutputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens)` from `claudeUsage` (`internal/executor/claude_stream.go:18-21`); `CostUSD = record.CostUSD` (`internal/executor/claude_stream.go:35`).
- **opencode** (`internal/executor/opencode_stream.go:104-113`): `TotalTokens = int64(tokens.Input + tokens.Output + tokens.Reasoning + tokens.Cache.Read + tokens.Cache.Write)` from `opencodeTokens` (`internal/executor/opencode_stream.go:106-112`); `CostUSD = part.Cost` (`internal/executor/opencode_stream.go:123`).
- **cursor-agent** (`internal/executor/cursor_agent.go:239-270`): Emits `0` for `TotalTokens` and `0.0` for `CostUSD` (`internal/executor/cursor_agent.go:1-60`, `internal/executor/cursor_agent_stream.go:1-218`).

**Alternatives considered**:
Multi-column token breakdown (`input_tokens`, `output_tokens`): rejected because the UI (`internal/serve/static/app.js:542-544`) consumes aggregate `total_tokens`.

**Rationale**:
Matches the dashboard fallback chain (`internal/serve/static/app.js:542-544`) while normalizing heterogeneous CLI usage structs.

**Terminal consumer**:
`internal/serve/static/app.js:542-544`

## Decision 2 — Tool-activity metric: count or rate (resolves the count/rate discrepancy)

**Choice**:
Persist cumulative `tool_calls INTEGER NOT NULL DEFAULT 0` in SQLite (`lane_progress` table) and carry `ToolCalls int64` on `executor.ProgressEvent` (`internal/executor/executor.go:18-21`) and `ledger.LaneProgress` (`internal/ledger/progress.go:15-20`). Emit both `tool_calls` (`int64`) and derived `tool_rate` (`float64`, tools/min) over the wire on `serve.LaneProgress` (`internal/serve/model.go:186-193`), where `tool_rate` is computed at the JSON layer in `GetLaneProgress` (`internal/serve/model.go:336-346`) as `float64(tool_calls) / max(elapsed_minutes, 1.0/60.0)`.

**Alternatives considered**:
Persisting computed rate in `lane_progress`: rejected because rates are time-window derivatives vulnerable to startup spikes; raw counts are immutable facts.

**Rationale**:
Resolves the spec conflict (`openspec/changes/lane-status-observability/specs/lane-progress-telemetry/spec.md:1-30` count vs `openspec/changes/lane-status-observability/specs/batch-wave-view/spec.md:1-43` rate) by storing ground-truth counts while satisfying the dashboard's `/min` formatter (`internal/serve/static/app.js:542-544`).

**Terminal consumer**:
`internal/serve/static/app.js:542-544`

## Decision 3 — v7 migration shape

**Choice**:
One `migrateV6ToV7DDL` constant in `internal/ledger/schema.go` performing a transactional create-copy-drop-rename rebuild of `runs` (`runs_new`) and `lane_progress` (`lane_progress_new`):
- `runs_new`: v6 `runs` (`internal/ledger/schema.go:226-234`) plus `pid INTEGER NOT NULL DEFAULT 0 CHECK (pid >= 0)`.
- `lane_progress_new`: v6 `lane_progress` (`internal/ledger/schema.go:298-305`) plus `total_tokens INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0)`, `cost_usd REAL NOT NULL DEFAULT 0.0 CHECK (cost_usd >= 0.0)`, and `tool_calls INTEGER NOT NULL DEFAULT 0 CHECK (tool_calls >= 0)`.
- Recreates index `idx_lane_progress_run_lane_seq` (`internal/ledger/schema.go:306-307`).
- Pre-migration rows: `runs.pid` defaults to `0` (legacy untracked run; lens C's sweep ignores `pid <= 0`); `lane_progress` defaults to `0`, `0.0`, `0`. No backfill.

**Alternatives considered**:
`ALTER TABLE ADD COLUMN`: rejected because SQLite cannot alter STRICT tables with CHECK constraints in place (`internal/ledger/schema.go:183-189`, `internal/ledger/schema.go:223-224`).

**Rationale**:
Maintains STRICT table integrity and idempotent migration semantics (`internal/ledger/schema.go:190-219`, `internal/ledger/schema.go:225-308`).

**Terminal consumer**:
`internal/ledger/schema.go:313-409`

## Interfaces / Contracts

```sql
const migrateV6ToV7DDL = `
CREATE TABLE runs_new (
  run_id     TEXT    PRIMARY KEY,
  feature_id TEXT    NOT NULL,
  status     TEXT    NOT NULL,
  target_ref TEXT    NOT NULL,
  lane_count INTEGER NOT NULL CHECK (lane_count >= 0),
  started_at TEXT    NOT NULL,
  ended_at   TEXT,
  pid        INTEGER NOT NULL DEFAULT 0 CHECK (pid >= 0)
) STRICT;

INSERT INTO runs_new (
  run_id, feature_id, status, target_ref, lane_count, started_at, ended_at
)
SELECT
  run_id, feature_id, status, target_ref, lane_count, started_at, ended_at
FROM runs ORDER BY started_at, run_id;

DROP TABLE runs;

ALTER TABLE runs_new RENAME TO runs;

CREATE TABLE lane_progress_new (
  run_id       TEXT    NOT NULL,
  lane_id      TEXT    NOT NULL,
  seq          INTEGER NOT NULL,
  message      TEXT    NOT NULL,
  at           TEXT    NOT NULL,
  total_tokens INTEGER NOT NULL DEFAULT 0 CHECK (total_tokens >= 0),
  cost_usd     REAL    NOT NULL DEFAULT 0.0 CHECK (cost_usd >= 0.0),
  tool_calls   INTEGER NOT NULL DEFAULT 0 CHECK (tool_calls >= 0),
  PRIMARY KEY (run_id, lane_id, seq)
) STRICT;

INSERT INTO lane_progress_new (
  run_id, lane_id, seq, message, at
)
SELECT
  run_id, lane_id, seq, message, at
FROM lane_progress ORDER BY run_id, lane_id, seq;

DROP TABLE lane_progress;

ALTER TABLE lane_progress_new RENAME TO lane_progress;

CREATE INDEX IF NOT EXISTS idx_lane_progress_run_lane_seq
  ON lane_progress(run_id, lane_id, seq);
`
```

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `executor.ProgressEvent` | `internal/executor/executor.go:18-21` | Add `TotalTokens int64`, `CostUSD float64`, `ToolCalls int64` | Yes |
| `ledger.LaneProgress` | `internal/ledger/progress.go:15-20` | Add `TotalTokens int64`, `CostUSD float64`, `ToolCalls int64` | Yes |
| `serve.LaneProgress` | `internal/serve/model.go:186-193` | Add `total_tokens`, `cost_usd`, `tool_calls`, `tool_rate` fields | Yes |
| `runs` table schema | `internal/ledger/schema.go:226-234` | Add `pid INTEGER NOT NULL DEFAULT 0 CHECK (pid >= 0)` | Yes |
| `lane_progress` table schema | `internal/ledger/schema.go:298-305` | Add `total_tokens`, `cost_usd`, `tool_calls` STRICT columns | Yes |
| `schemaVersion` | `internal/ledger/schema.go:10` | Bump version `6` to `7` | Yes |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/executor/executor.go` | Modify | Add `TotalTokens`, `CostUSD`, `ToolCalls` to `ProgressEvent` | `internal/run/run.go:562-564` |
| `internal/executor/agy_stream.go` | Modify | Populate `TotalTokens` from `agyUsage` and count tool events | `internal/executor/executor.go:48` |
| `internal/executor/claude_stream.go` | Modify | Sum token counts, assign `CostUSD`, and count tool events | `internal/executor/executor.go:48` |
| `internal/executor/opencode_stream.go` | Modify | Sum token counts, assign `CostUSD`, and count tool events | `internal/executor/executor.go:48` |
| `internal/executor/cursor_agent.go` | Modify | Emit zero tokens/cost and count tool events | `internal/executor/executor.go:48` |
| `internal/ledger/schema.go` | Modify | Add `migrateV6ToV7DDL`, bump `schemaVersion = 7`, wire migration | `internal/ledger/schema.go:313-409` |
| `internal/ledger/progress.go` | Modify | Add numeric fields to `LaneProgress` and update CRUD SQL | `internal/serve/model.go:336-346` |
| `internal/run/run.go` | Modify | Forward telemetry in `writeLaneProgress` | `internal/ledger/progress.go:36-79` |
| `internal/serve/model.go` | Modify | Add telemetry/rate to `serve.LaneProgress` and compute `tool_rate` | `internal/serve/handlers.go:55-59` |

## Testing Strategy (this slice)

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Schema Migration | v6→v7 migration row preservation, STRICT enforcement, and reopen idempotency | Apply v6 fixture, migrate, assert columns/strict flags/idempotency | `internal/ledger/schema_test.go:14-97`, `internal/ledger/schema_test.go:146-201` |
| Ledger Progress CRUD | Append and tail progress with token, cost, and tool metrics | Test `AppendProgressBatch` and `GetProgressAfter` preserving telemetry | `internal/ledger/progress_test.go:14-49`, `internal/ledger/progress_test.go:56-73` |
| Agy Stream Decoder | Token usage and tool call extraction into `ProgressEvent` | Feed stream JSON fixture, assert emitted telemetry | `internal/executor/agy_stream_test.go:49-60` |
| Claude Stream Decoder | Summed token arithmetic, USD cost, and tool call extraction | Feed stream-json fixture, assert emitted telemetry | `internal/executor/claude_stream_test.go:50-60` |
| Opencode Stream Decoder | Summed token arithmetic, USD cost, and tool call extraction | Unit test `decodeOpencodeRecord` with tokens/cost/tools | `internal/executor/opencode_stream_test.go:30-60` |
| Cursor-Agent Decoder | Zero usage/cost emission and tool call lifecycle counting | Feed stream fixture, assert zero tokens/cost and tool count | `internal/executor/cursor_agent_stream_test.go:14-58` |
| Serve Model & JSON DTO | Serialization of telemetry fields and `tool_rate` calculation | Test `GetLaneProgress` JSON serialization contract | `internal/serve/model_test.go:599-676` |

## Open Questions

- [ ] Reconciled skill and packet constraints: `~/.claude/skills/sdd-design/SKILL.md` specifies an 800-word budget, Engram persistence, and return summary blocks, superseded by this packet's 1000-word budget (excluding SQL and manifest), direct `.lucind/result.json` return envelope, and capability-sliced design lens skeleton.
- [ ] Lens C consumption notice: Pre-migration `runs.pid` defaults to `0`; lens C's sweep ignores `pid <= 0` so legacy runs are not treated as dead processes.

## Citation Manifest

| citation | claim |
|---|---|
| `internal/executor/agy_stream.go:12-18` | agyUsage struct carrying TotalTokens |
| `internal/executor/agy_stream.go:17` | TotalTokens field on agyUsage |
| `internal/executor/agy_stream_test.go:49-60` | Test seam for agy stream JSON telemetry decoding |
| `internal/executor/claude_stream.go:17-22` | claudeUsage struct with separate input, output, and cache token fields |
| `internal/executor/claude_stream.go:18-21` | Input, output, and cache token fields in claudeUsage |
| `internal/executor/claude_stream.go:35` | CostUSD field on claudeStreamRecord |
| `internal/executor/claude_stream_test.go:50-60` | Test seam for claude stream-json telemetry decoding |
| `internal/executor/cursor_agent.go:1-60` | CursorAgent executor definition lacking usage struct |
| `internal/executor/cursor_agent.go:239-270` | cursorStreamCapture tool decoding and event emission |
| `internal/executor/cursor_agent_stream.go:1-218` | Cursor-agent stream decoder parsing tools only |
| `internal/executor/cursor_agent_stream_test.go:14-58` | Subprocess stream test runner for cursor-agent |
| `internal/executor/executor.go:18-21` | ProgressEvent struct definition |
| `internal/executor/executor.go:48` | Progress channel on Request struct |
| `internal/executor/opencode_stream.go:104-113` | opencodeTokens struct definition with component token fields |
| `internal/executor/opencode_stream.go:106-112` | Token component fields in opencodeTokens |
| `internal/executor/opencode_stream.go:123` | Cost field on opencodeStreamPart |
| `internal/executor/opencode_stream_test.go:30-60` | TestDecodeOpencodeRecord test cases for telemetry |
| `internal/ledger/progress.go:15-20` | LaneProgress struct definition |
| `internal/ledger/progress.go:36-79` | AppendProgressBatch implementation |
| `internal/ledger/progress_test.go:14-49` | TestAppendProgressBatchAndCursorTail test seam |
| `internal/ledger/progress_test.go:56-73` | TestAppendProgressSequenceValidationAndAtomicity test seam |
| `internal/ledger/schema.go:10` | schemaVersion constant definition |
| `internal/ledger/schema.go:183-189` | Rationale for create-copy-drop-rename on STRICT tables |
| `internal/ledger/schema.go:190-219` | migrateV4ToV5DDL table rebuild pattern |
| `internal/ledger/schema.go:223-224` | Rationale for STRICT table recreation in v6 |
| `internal/ledger/schema.go:225-308` | migrateV5ToV6DDL table rebuild pattern |
| `internal/ledger/schema.go:226-234` | runs table definition in v6 DDL |
| `internal/ledger/schema.go:298-305` | lane_progress table definition in v6 DDL |
| `internal/ledger/schema.go:306-307` | idx_lane_progress_run_lane_seq index definition |
| `internal/ledger/schema.go:313-409` | migrate version-gated execution loop |
| `internal/ledger/schema_test.go:14-97` | TestMigrateV5ToV6PreservesRowsAndAddsSchema test seam |
| `internal/ledger/schema_test.go:146-201` | TestSchemaV6ReopenIsIdempotent test seam |
| `internal/run/run.go:562-564` | ProgressEvent to LaneProgress mapping in writeLaneProgress |
| `internal/serve/handlers.go:55-59` | ServerState.LaneProgress field definition |
| `internal/serve/model.go:186-193` | LaneProgress DTO definition |
| `internal/serve/model.go:336-346` | GetLaneProgress DTO mapping |
| `internal/serve/model_test.go:599-676` | TestModelRunLaneAndProgressJSONContract test seam |
| `internal/serve/static/app.js:542-544` | Metric display fallback chain for total_tokens, cost_usd, and tool_rate |
| `openspec/changes/lane-status-observability/specs/batch-wave-view/spec.md:1-43` | Batch wave view delta specification |
| `openspec/changes/lane-status-observability/specs/lane-progress-telemetry/spec.md:1-30` | Lane progress telemetry specification |
