# Tasks Lens B — Structured Telemetry & Schema v7: Lane Status Observability

## Assumed decomposition

This slice delivers structured progress telemetry and schema v7 migration across three phases: B1 executes schema v7 STRICT migration and ledger persistence; B2 maps decoder progress metrics across agy, claude, opencode, cursor-agent; B3 forwards progress in run execution and serves JSON DTOs with derived tool rates. Schema v7 (B1) must land before decoder telemetry (B2) or queries (B3) write/read columns.

## Phase B1: Schema v7 Migration & Ledger Progress Storage

- [ ] B1.1 RED test: In `internal/ledger/schema_test.go:14-97,99-144,146-201`, add `TestMigrateV6ToV7PreservesRowsAndAddsSchema`, `TestSchemaV7ConstraintsAndIndexes`, `TestSchemaV7ReopenIsIdempotent` asserting row preservation, defaults (`runs.pid=0`, usage 0), CHECK `>= 0`, reopen idempotency.
- [ ] B1.2 GREEN prod: In `internal/ledger/schema.go:10,313-409`, bump `schemaVersion = 7`, define verbatim `migrateV6ToV7DDL` (`design.md:113-161`), wire in `migrate()` loop:
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
- [ ] B1.3 RED test: In `internal/ledger/progress_test.go:14-49,51-108`, assert `TotalTokens`, `CostUSD`, `ToolCalls` round-trip in `AppendProgressBatch`/`GetProgressAfter` and negative metrics error.
- [ ] B1.4 GREEN prod: In `internal/ledger/progress.go:15-20,55-73,87-102,105-133`, add `TotalTokens`, `CostUSD`, `ToolCalls` to `ledger.LaneProgress`, validate `>= 0` in `validateProgress`, update insert/select SQL.

## Phase B2: Executor Progress Telemetry & Per-Decoder Wiring

- [ ] B2.1 RED test: In `internal/executor/agy_stream_test.go:49-102`, assert `ProgressEvent` populates `TotalTokens` from `agyUsage`, `CostUSD=0`, tool counts.
- [ ] B2.2 GREEN prod: In `internal/executor/executor.go:18-21` and `internal/executor/agy_stream.go:12-18,101-136`, add fields to `ProgressEvent` and update `agyStreamDecoder`.
- [ ] B2.3 RED test: In `internal/executor/claude_stream_test.go:42-67`, assert `ProgressEvent` sums tokens, extracts `CostUSD`, counts tools.
- [ ] B2.4 GREEN prod: In `internal/executor/claude_stream.go:17-22,35,71-80,148-168,190-210`, update `claudeStreamDecoder` telemetry emission.
- [ ] B2.5 RED test: In `internal/executor/opencode_stream_test.go:30-70`, assert `ProgressEvent` sums tokens, extracts `CostUSD`, counts tools.
- [ ] B2.6 GREEN prod: In `internal/executor/opencode_stream.go:104-113,120-127,141-175,197-240`, update `opencodeStreamDecoder` telemetry emission.
- [ ] B2.7 RED test: In `internal/executor/cursor_agent_stream_test.go:1-60`, assert `ProgressEvent` emits zeros for tokens/cost and counts tools.
- [ ] B2.8 GREEN prod: In `internal/executor/cursor_agent.go:239-270`, update `cursorStreamCapture` telemetry emission.

## Phase B3: Run Pipeline & Serve Model Telemetry

- [ ] B3.1 RED test: In `internal/run/run_test.go:140-190`, test `writeLaneProgress` forwards telemetry from `executor.ProgressEvent` to `ledger.LaneProgress`.
- [ ] B3.2 GREEN prod: In `internal/run/run.go:562-564`, update `writeLaneProgress` to forward telemetry fields to `ledger.LaneProgress`.
- [ ] B3.3 RED test: In `internal/serve/model_test.go:599-676`, assert `serve.LaneProgress` emits telemetry and derived `tool_rate` (tools/min, 1s floor).
- [ ] B3.4 GREEN prod: In `internal/serve/model.go:186-193,336-346`, add telemetry fields to `serve.LaneProgress` and compute `tool_rate` in `GetLaneProgress`.

## Dependency Order (this slice)

|Task|Depends on|Why|
|---|---|---|
|B1.1|None|Test fixture|
|B1.2|B1.1|TDD: v7 DDL|
|B1.3|B1.2|Tests require v7 schema|
|B1.4|B1.3, B1.2|Progress CRUD requires v7 DDL|
|B2.1|None|Test fixture|
|B2.2|B2.1, B1.4|Types require ledger persistence|
|B2.3–B2.8|B2.2|Decoders require `ProgressEvent`|
|B3.1|B2.2, B1.4|Run requires progress types|
|B3.2|B3.1|TDD: Run forwarding|
|B3.3|B1.4|Serve requires `LaneProgress`|
|B3.4|B3.3|TDD: Serve DTO & `tool_rate`|

## Suggested Work Unit

|Unit|Goal|allowed_paths|Executor|Rollback boundary|
|---|---|---|---|---|
|Unit B: Structured Telemetry & Schema v7|Schema v7 migration, decoder telemetry, run forwarding, serve DTO `tool_rate`|`internal/executor/agy_stream*.go`<br>`internal/executor/claude_stream*.go`<br>`internal/executor/cursor_agent*.go`<br>`internal/executor/executor.go`<br>`internal/executor/opencode_stream*.go`<br>`internal/ledger/progress*.go`<br>`internal/ledger/schema*.go`<br>`internal/run/run*.go`<br>`internal/serve/model*.go`|`agy`|Revert commit restoring schema v6 and removing telemetry fields|

## RED Tests from the Threat Matrix (this slice)

|Threat row|Applicable|RED test|Asserts|Production task it precedes|
|---|---|---|---|---|
|Process integration|None — see lens C|`TestMigrateV6ToV7PreservesRowsAndAddsSchema` (`internal/ledger/schema_test.go:14-97,99-144,146-201`)|STRICT rebuild preserves rows, enforces CHECK `>= 0`, idempotent reopen|B1.2|

## Acceptance Evidence

|Task|Proving command|What a pass proves|What it does not prove|
|---|---|---|---|
|B1.1–B1.2|`go test ./internal/ledger -run 'Migrate\|Schema'`|STRICT migration preserves rows, adds columns, idempotent|Decoders/queries|
|B1.3–B1.4|`go test ./internal/ledger -run 'Progress'`|`LaneProgress` metrics validate, persist, query|Decoders|
|B2.1–B2.8|`go test ./internal/executor -run 'Stream\|Decode'`|Decoders parse/sum tokens, set cost, zero cursor-agent, count tools|Persistence|
|B3.1–B3.2|`go test ./internal/run -run 'Progress'`|`run.Execute` maps `ProgressEvent` to `ledger.LaneProgress`|Serve DTO|
|B3.3–B3.4|`go test ./internal/serve -run 'Progress'`|`serve.LaneProgress` emits telemetry and `tool_rate`|UI|

## Requirement Traceability

|Requirement|Tasks|
|---|---|
|`lane-progress-telemetry: Structured progress telemetry`|B1.1–B1.4, B2.1–B2.2, B3.1–B3.4|
|`lane-progress-telemetry: Decoders populate usage`|B2.1–B2.6|
|`lane-progress-telemetry: Cursor-agent emits zeroed metrics`|B2.7–B2.8|
|`lane-progress-telemetry: Real-time telemetry broadcast`|B3.1–B3.4|
|`batch-wave-view: Lane card metadata, packet link, and telemetry inspection`|B3.3–B3.4|

## Open Questions

- [ ] Lens C dependency: Lens C's `internal/ledger/runs.go` updates (`Run.PID`) depend on `runs.pid` from v7 DDL (B1.2); synthesizer sequences v7 before `runs.go`.
- [ ] Lens A overlap: `internal/serve/model.go` and `app.js` are touched by lens A (`Skill`, `PacketPath`, link) and lens B (`LaneProgress` telemetry, `tool_rate`).
- [ ] Skill & packet constraints: `~/.claude/skills/sdd-tasks/SKILL.md` budget is superseded by this packet's Lens B scope, 1000-word budget, and `.lucind/result.json` return.

## Citation Manifest

|citation|claim|
|---|---|
|`internal/executor/agy_stream.go:12-18`|`agyUsage` struct definition with `TotalTokens`|
|`internal/executor/agy_stream.go:101-136`|`agyStreamDecoder.consumeStep` agent response usage and tool event emission|
|`internal/executor/agy_stream_test.go:49-102`|`TestAgyRunStreamsNormalizedProgressAndPreservesFinalResult` testing stream JSON usage and tool events|
|`internal/executor/claude_stream.go:17-22`|`claudeUsage` struct with input, output, cache-read, and cache-creation token fields|
|`internal/executor/claude_stream.go:35`|`CostUSD` field on `claudeStreamRecord`|
|`internal/executor/claude_stream.go:71-80`|`claudeStreamDecoder` struct definition with tool tracker|
|`internal/executor/claude_stream.go:148-168`|`consumeAssistantBlock` handling text and tool_use starting events|
|`internal/executor/claude_stream.go:190-210`|`consumeResult` handling terminal subtype and usage formatting|
|`internal/executor/claude_stream_test.go:42-67`|`TestClaudeRunSendsVerboseWithStreamJSON` testing stream-json flag setup|
|`internal/executor/cursor_agent.go:239-270`|`cursorStreamCapture.decode` tool decoding and progress event emission|
|`internal/executor/cursor_agent_stream_test.go:1-60`|Subprocess stream test harness and tool timing test seam|
|`internal/executor/executor.go:18-21`|`ProgressEvent` struct definition|
|`internal/executor/opencode_stream.go:104-113`|`opencodeTokens` struct with input, output, reasoning, and cache tokens|
|`internal/executor/opencode_stream.go:120-127`|`opencodeStreamPart` union struct with `Cost` and `Tokens`|
|`internal/executor/opencode_stream.go:141-175`|`decodeOpencodeToolUse` tool start/finish event decoding|
|`internal/executor/opencode_stream.go:197-240`|`decodeOpencodeRecord` handling step_finish and tool_use events|
|`internal/executor/opencode_stream_test.go:30-70`|`TestDecodeOpencodeRecord` test cases for step and tool events|
|`internal/ledger/progress.go:15-20`|`LaneProgress` struct definition|
|`internal/ledger/progress.go:55-73`|`AppendProgressBatch` SQL insert statements|
|`internal/ledger/progress.go:87-102`|`validateProgress` field validation rules|
|`internal/ledger/progress.go:105-133`|`GetProgressAfter` SQL select query and row scan|
|`internal/ledger/progress_test.go:14-49`|`TestAppendProgressBatchAndCursorTail` testing batch append and cursor tailing|
|`internal/ledger/progress_test.go:51-108`|`TestAppendProgressSequenceValidationAndAtomicity` testing progress validation and rollback|
|`internal/ledger/schema.go:10`|`schemaVersion` constant currently set to 6|
|`internal/ledger/schema.go:313-409`|`migrate` function version loop|
|`internal/ledger/schema_test.go:14-97`|`TestMigrateV5ToV6PreservesRowsAndAddsSchema` test seam for migration|
|`internal/ledger/schema_test.go:99-144`|`TestSchemaV6ConstraintsAndIndexes` testing STRICT constraints and indices|
|`internal/ledger/schema_test.go:146-201`|`TestSchemaV6ReopenIsIdempotent` testing migration reopen idempotency|
|`internal/run/run.go:562-564`|`writeLaneProgress` mapping `ProgressEvent` to `ledger.LaneProgress`|
|`internal/run/run_test.go:140-190`|`TestExecuteProgressWriterFlushesAtBatchSize` testing progress batch flushing|
|`internal/serve/model.go:186-193`|`serve.LaneProgress` DTO struct definition|
|`internal/serve/model.go:336-346`|`GetLaneProgress` mapping ledger rows to DTOs|
|`internal/serve/model_test.go:599-676`|`TestModelRunLaneAndProgressJSONContract` testing JSON contract serialization|
|`internal/serve/static/app.js:542-544`|Dashboard metric display fallback chain for total_tokens, cost_usd, and tool_rate|
|`openspec/changes/lane-status-observability/design.md:113-161`|`migrateV6ToV7DDL` contract in design interfaces|
|`openspec/changes/lane-status-observability/specs/batch-wave-view/spec.md:32-37`|Scenario for lane card metadata and telemetry inspection|
|`openspec/changes/lane-status-observability/specs/lane-progress-telemetry/spec.md:9-30`|Structured progress telemetry requirement and scenarios|
