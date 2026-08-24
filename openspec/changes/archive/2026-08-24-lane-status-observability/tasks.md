# Tasks: Lane Status Observability

Single packet, sequential apply, three work-unit commits. No `apply-dag.yaml`: accepted one PR (`size:exception`); Strict-TDD RED/GREEN stays in one lane; `Integrate` bisects a failing combined tree (`internal/run/integrate.go:50-59`). Cross-slice deps (v7 `runs.pid` before `Run.PID`; shared `cli.go` / `model.go` / `app.js`) are sequenced here, not as parallel waves.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 900–1600 (impl ~400–750, tests ~450–750, new sweeper pair ~80–120) |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | single PR, three work-unit commits |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Schema v7 + progress numerics + decoder telemetry + serve `tool_rate` | PR 1 | `go test ./internal/ledger ./internal/executor ./internal/run ./internal/serve -count=1` | N/A — SQLite + stream fixtures; dashboard already reads `total_tokens`/`cost_usd`/`tool_rate` (`app.js:542-544`) | revert v7 to v6; drop telemetry fields |
| 2 | Frontmatter keys, `Packet.Path`, metadata snapshot, packet GET, UI skill/link | PR 1 | `go test ./internal/packet ./internal/ledger ./internal/run ./internal/serve ./cmd/lucind-ai -count=1` | `httptest` GET `/api/packets/{runID}/{laneID}` (200 markdown / 404 JSON, process stays up) | revert Go/JS; new JSON keys and route are additive |
| 3 | `Run.PID` on `RegisterRun`, serve sweeper, Linux `kill(pid,0)` | PR 1 | `go test ./internal/ledger ./internal/serve ./cmd/lucind-ai -count=1` | N/A — probe self/dead/zero in `sweeper_test.go`; no live serve required | revert `runs.go`, `cli.go`, delete `sweeper.go`; v7 `pid` default 0 stays |

Apply order: Unit 1 (schema) → Unit 2 → Unit 3. Unit 3 must not start until Unit 1’s v7 DDL has landed. Shared-file edits stay sequential in this packet: `cli.go` Path at `:160-174` then PID at `:314-324` then Sweeper at `:770-774`; `model.go` Lane/`laneDTO` (`:163-184,:322-333`) then LaneProgress/`tool_rate` (`:186-193,:336-346`); `app.js` is Unit 2 only (skill + packet link). Unit 1 does not modify `app.js`.

## Phase 1: Schema, frontmatter, metadata, PID column

- [x] 1.1 RED: In `internal/ledger/schema_test.go` (v6 seams at `:14-97,:99-144,:146-201`) add `TestMigrateV6ToV7PreservesRowsAndAddsSchema`, `TestSchemaV7ConstraintsAndIndexes`, `TestSchemaV7ReopenIsIdempotent` — row preservation, `runs.pid=0` / usage 0, CHECK `>= 0`, reopen idempotent.
- [x] 1.2 GREEN: Bump `schemaVersion` to 7 (`schema.go:10`), add verbatim `migrateV6ToV7DDL` below, wire the `currentVersion < 7` step in `migrate` (`schema.go:313-409`). Prove: `go test ./internal/ledger -run 'Migrate|Schema'`.
- [x] 1.3 RED: In `progress_test.go:14-49,:51-108` assert `TotalTokens`, `CostUSD`, `ToolCalls` round-trip and negative metrics error.
- [x] 1.4 GREEN: Add those fields to `ledger.LaneProgress` (`progress.go:15-20`), validate `>= 0` in `validateProgress` (`:87-102`), update insert/select (`:55-73,:105-133`). Prove: `go test ./internal/ledger -run Progress`.
- [x] 1.5 RED: In `internal/packet/packet_test.go` (seams `:15-48,:50-90`) parse optional `sdd_phase`, `fanout_group`, `skill`; omitted and explicit-empty keys → `""`; no live Skill telemetry.
- [x] 1.6 GREEN: Add `SDDPhase`, `FanoutGroup`, `Skill`, `Path` to `Packet` (`packet.go:33-75`); parse the three keys in `Parse` (`:94-138`). Prove: `go test ./internal/packet -run Parse`.
- [x] 1.7 RED: In `lanes_meta_test.go:15-81,:177-203` round-trip `Skill` and `PacketPath` on `UpdateLaneMetadata`/`GetLaneMetadata`; v6 fallback leaves them empty.
- [x] 1.8 GREEN: Add `Skill` and `PacketPath` to `LaneMetadata` (`lanes_meta.go:20-32`); persist via existing snapshot (`:39-83,:89-128`). Prove: `go test ./internal/ledger -run LaneMetadata`.
- [x] 1.9 RED: In `runs_test.go:12-43` assert `Run.PID` via `RegisterRun`/`GetRun`/`ListRuns`/`scanRun`.
- [x] 1.10 GREEN: Add `PID int` to `Run` (`runs.go:16-24`); insert/select/scan `pid` (`:29-41,:63-76,:80-101,:165-188`). Depends on 1.2. Prove: `go test ./internal/ledger -run 'RegisterAndGetRun'`.

## Phase 2: CLI capture, decoders, dispatch wiring

- [x] 2.1 RED: In `cmd/lucind-ai/cli_test.go` (packet-flag seams `TestRunMissingPacketFlagIsUsageError`, `TestRunRepeatablePacketFlagPreservesOrderAndProcessesEachOne`) assert `--packet` paths populate `Packet.Path`.
- [x] 2.2 GREEN: In the load loop (`cli.go:160-174`) set `p.Path` from the flag after `Parse`. Prove: `go test ./cmd/lucind-ai -run Packet`.
- [x] 2.3 RED: Extend `TestRunDispatchRegistersRunRowInLedger` so `RegisterRun` records `os.Getpid()`.
- [x] 2.4 GREEN: In `runDispatch` (`cli.go:314-324`) pass `PID: os.Getpid()`. Depends on 1.10. Prove: `go test ./cmd/lucind-ai -run TestRunDispatch`.
- [x] 2.5 RED: `agy_stream_test.go:49-102` — `ProgressEvent.TotalTokens` from `agyUsage.TotalTokens` (`agy_stream.go:12-18`), `CostUSD=0`, tool counts from `consumeStep` (`:101-136`).
- [x] 2.6 GREEN: Add fields on `ProgressEvent` (`executor.go:18-21`); map in `agyStreamDecoder`.
- [x] 2.7 RED: Claude fixtures after `TestClaudeRunSendsVerboseWithStreamJSON` (`claude_stream_test.go:42-67`, argv only today) — sum input+output+cache-read+cache-creation (`claude_stream.go:17-22`), `CostUSD` (`:35`), tool counts (`:148-168,:190-210`).
- [x] 2.8 GREEN: Update `claudeStreamDecoder` (`:71-80` and consume paths).
- [x] 2.9 RED: `opencode_stream_test.go:30-70` — sum input+output+reasoning+cache (`opencode_stream.go:104-113,:120-127`), `CostUSD` from the part, tools (`:141-175,:197-240`).
- [x] 2.10 GREEN: Update `opencodeStreamDecoder`.
- [x] 2.11 RED: `cursor_agent_stream_test.go:1-60` — tokens/cost `0`/`0.0`, tools counted (`cursor_agent.go:239-270` emits `{Message, At}` only today).
- [x] 2.12 GREEN: Emit zeros + tool counts from `cursorStreamCapture`. Prove 2.5–2.12: `go test ./internal/executor -run 'Stream|Decode|Cursor'`.
- [x] 2.13 RED: `run_test.go:140-190` — `writeLaneProgress` forwards telemetry onto `ledger.LaneProgress`.
- [x] 2.14 GREEN: Map fields in `writeLaneProgress` (`run.go:562-564`). Prove: `go test ./internal/run -run Progress`.
- [x] 2.15 RED: `run_test.go` (`testPacket` at `:79-93`) — `Execute` and `ensureLaneFailed` call `UpdateLaneMetadata`.
- [x] 2.16 GREEN: After `RegisterLane` in `Execute` (`run.go:334-358`) and `ensureLaneFailed` (`batch.go:167-217`) call `UpdateLaneMetadata` with packet fields plus `Skill`/`PacketPath`. Prove: `go test ./internal/run -run UpdateLaneMetadata`.

## Phase 3: Serve, UI, sweeper

- [x] 3.1 RED: `model_test.go:599-669` — `Lane`/`laneDTO` JSON includes `skill`, `packet_path`.
- [x] 3.2 GREEN: Add those fields on `Lane` and `laneDTO` (`model.go:163-184,:322-333`).
- [x] 3.3 RED: Same contract test — `serve.LaneProgress` emits `total_tokens`, `cost_usd`, `tool_calls`, derived `tool_rate` (tools/min, 1s floor, elapsed from lane `StartedAt` to progress `At`).
- [x] 3.4 GREEN: Extend `LaneProgress` and `GetLaneProgress` (`model.go:186-193,:336-346`). Prove 3.1–3.4: `go test ./internal/serve -run TestModelRunLaneAndProgressJSONContract`.
- [x] 3.5 RED: In `server_test.go` (httptest pattern `TestBulkRequestBodyReturns400` at `:47-100`, not packet coverage today) 200 raw markdown; 404 unknown lane / empty path / unreadable file; process does not abort.
- [x] 3.6 GREEN: Register `GET /api/packets/{runID}/{laneID}` in `handlers.go` using the two-segment parse (`:316-350`) and `/api/` 404 fallback (`:352-354`); 200 `text/markdown; charset=utf-8` or 404 via `writeJSONError`. Prove: `go test ./internal/serve -run TestPacketBodyEndpoint`.
- [x] 3.7 RED: `static_test.go:286-300` — `app.js` normalizes `skill` and renders a packet markdown link.
- [x] 3.8 GREEN: Extend `normalizeFleetState` (`app.js:534-536`) and fleet cards (`:575-593`). Prove: `go test ./internal/serve -run TestFleetView`.
- [x] 3.9 RED: `internal/serve/sweeper_test.go` — `TestSweeper_LivePIDRetained`: live PID (`err == nil`) leaves `running` (`run.go:355-358`).
- [x] 3.10 RED: `TestSweeper_DeadPIDReconciled`: `ESRCH`/`os.ErrProcessDone` → `lane.Failed` (`status.go:11-17`) via `SetStatus` (`ledger.go:452-484`) plus `EventLaneNote` “orphaned: driving process no longer running” (`:366-378,:440-446`).
- [x] 3.11 RED: `TestSweeper_ZeroPIDIgnored`: `pid <= 0` skips probe, leaves `running`.
- [x] 3.12 RED: `TestSweeper_RecycledPIDAndEPERM`: `EPERM` is alive; recycled live PID is not failed (design: no second identity check).
- [x] 3.13 GREEN: Create `internal/serve/sweeper.go` — `Sweeper.Run(ctx)` immediate sweep then 10s ticker (pattern `Hub.Run` at `hub.go:213-235`, not `defaultPollInterval` at `:24`); `os.FindProcess(pid).Signal(syscall.Signal(0))`; Linux-only. Prove: `go test ./internal/serve -run TestSweeper_`.
- [x] 3.14 RED: Assert `serveDispatch` starts Sweeper beside Hub (seam `startTestServe` / `cli.go:770-774`).
- [x] 3.15 GREEN: Launch `go func() { _ = sweeper.Run(ctx) }()` in `serveDispatch`. Prove: `go test ./cmd/lucind-ai -run TestServe`.

## Dependency order

| Task | Depends on | Why |
|------|------------|-----|
| 1.2 | 1.1 | TDD: v7 DDL |
| 1.4 | 1.2, 1.3 | Progress SQL needs v7 columns |
| 1.6 | 1.5 | Parser GREEN |
| 1.8 | 1.7 | Metadata GREEN |
| 1.9–1.10 | **1.2** | **Cross-lens: `Run.PID` after `runs.pid`** |
| 2.2 | 1.6, 2.1 | `Packet.Path` field |
| 2.4 | 1.10, 2.3 | `Run.PID` exists |
| 2.6–2.12 | 2.5+ matching RED; 1.4 | `ProgressEvent` + persist types |
| 2.14 | 2.13, 2.6, 1.4 | Forwarding |
| 2.16 | 2.15, 1.6, 1.8, 2.2 | Packet + metadata fields |
| 3.2 | 3.1, 1.8 | Lane DTO |
| 3.4 | 3.3, 1.4 | Progress DTO / `tool_rate` |
| 3.6 | 3.5, 3.2, 2.16 | Handler reads `PacketPath` |
| 3.8 | 3.7, 3.6 | UI after route |
| 3.13 | 3.9–3.12, 1.10 | Sweeper needs persisted PID |
| 3.15 | 3.13, 3.14 | Launch after type exists |

Shared `cli.go` / `model.go` / `app.js`: no extra table edges. One packet, sequential commits; `allowed_paths` does not apply.

## Threat-matrix RED tests

Only Process integration is Applicable (`design.md:179-189`). N/A rows get no RED tasks. Schema migration tests (1.1) are TDD for v7, not this row.

| Adversarial case | RED task | Asserts | Precedes |
|------------------|----------|---------|----------|
| Live PID | 3.9 | `err == nil` keeps `running` | 3.13 |
| Dead PID | 3.10 | `ESRCH`/`ErrProcessDone` → `failed` + note | 3.13 |
| PID 0 | 3.11 | `pid <= 0` skipped | 3.13 |
| Recycling / `EPERM` | 3.12 | `EPERM` alive; recycled PID kept until that PID dies | 3.13 |

## Requirement traceability

| Requirement | Tasks |
|-------------|-------|
| Extended packet frontmatter parsing | 1.5, 1.6 |
| Lane metadata dispatch persistence | 1.7, 1.8, 2.15, 2.16, 3.1, 3.2 |
| Dispatched packet body inspection | 2.1, 2.2, 1.7, 1.8, 2.15, 2.16, 3.1–3.8 |
| Structured progress telemetry | 1.1–1.4, 2.5–2.14, 3.3, 3.4 |
| Decoders populate usage | 2.5–2.10 |
| Cursor-agent emits zeroed metrics | 2.11, 2.12 |
| Real-time telemetry broadcast | 2.13, 2.14, 3.3, 3.4 |
| Orphaned lane reconciliation | 1.9, 1.10, 2.3, 2.4, 3.9–3.15 |
| Dead-process lane swept to failed | 3.10, 3.13 |
| Active process lanes unchanged | 3.9, 3.11, 3.12, 3.13 |
| Batch and DAG Wave Inspection (metadata, packet link, telemetry) | 3.1–3.8 |
| Swept-orphan lane inspection | 3.10, 3.13 |

## Schema v7 DDL

Verbatim from `design.md:113-161`. No backfill: INSERT omits `pid` and usage columns so DEFAULT 0 applies.

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
