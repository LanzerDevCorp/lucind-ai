# Design: Lane Status Observability

Wire the existing `LaneMetadata` write path, persist the on-disk packet path, expose the dispatched Markdown, store structured usage, and reconcile dead-PID `running` lanes. One PR (`size:exception`). No historical backfill. Linux-only PID liveness. Static `skill:` frontmatter only.

**Open Question 1 resolved:** frontmatter keys are `sdd_phase`, `fanout_group`, `skill`.
**Open Question 2 resolved:** packet path is `LaneMetadata.PacketPath` in the `lane_metadata:v1:` JSON snapshot, not a `lanes` column.
**Open Question 3 resolved:** orphan ticker is `10 * time.Second`.
**Open Question 4 resolved:** liveness is POSIX `kill(pid, 0)` via `os.FindProcess(pid).Signal(syscall.Signal(0))`; Linux-only.
**Open Question 5 resolved:** `dag.Node` / `dag.EmitPacketContent` stay unchanged; omitted SDD keys default empty.

## Technical Approach

After `RegisterLane`, `Execute` and `ensureLaneFailed` call `UpdateLaneMetadata` with packet fields plus `Skill` and `PacketPath`. `packet.Parse` accepts the three optional keys. The CLI sets `Packet.Path` from the `--packet` argument. Serve adds `GET /api/packets/{runID}/{laneID}` and maps `Skill` / `PacketPath` through `laneDTO`. Executors populate `ProgressEvent` numeric fields; `writeLaneProgress` persists them; `GetLaneProgress` derives `tool_rate`. Schema v7 rebuilds `runs` (`pid`) and `lane_progress` (usage columns) in one STRICT create-copy-drop-rename. `RegisterRun` stores `os.Getpid()`. A serve `Sweeper` runs an immediate sweep then a 10s ticker and marks orphaned `running` lanes `failed`.

## Architecture Decisions

### Decision: Frontmatter keys `sdd_phase`, `fanout_group`, `skill` (OQ1)

**Choice**: Those three snake_case keys.
**Alternatives considered**: `phase` / `group` / `generated_by` (`proposal.md:185`).
**Rationale**: Matches existing parser mapping (`read_only`→`ReadOnly`, `parent_ref`→`ParentRef`, `allowed_paths`→`AllowedPaths` in `internal/packet/packet.go:94-138`) and existing `LaneMetadata` tags (`sdd_phase`, `fanout_group` at `internal/ledger/lanes_meta.go:25-26`). `phase`/`group` collide with generic terms; `generated_by` mislabels a skill as a generator.
**Terminal consumer**: `packet.Parse` switch; `internal/serve/static/app.js:534-536` (extend with `skill` beside `sdd_phase` / `fanout_group` / `feature`).

### Decision: Packet path in `LaneMetadata` JSON (OQ2)

**Choice**: `LaneMetadata.PacketPath string \`json:"packet_path"\`` inside the existing `lane_metadata:v1:` event snapshot.
**Alternatives considered**: A `packet_path` column on `lanes` (`proposal.md:186`).
**Rationale**: Only `model` / `agent` / `feature` occupy `lanes` columns (`internal/ledger/schema.go:249-251`). Extended fields already live in the JSON snapshot (`internal/ledger/lanes_meta.go:20-32,67-77`). Lookups are `(run_id, lane_id)` point reads (`GetLaneMetadata` at `internal/ledger/lanes_meta.go:89-127`), never cross-lane queries. Avoids a third STRICT rebuild in v7.
**Terminal consumer**: packet-body handler via `GetLaneMetadata`; `laneDTO` (`internal/serve/model.go:322-333`).

### Decision: `GET /api/packets/{runID}/{laneID}`

**Choice**: Two-segment path under `/api/`. 200 + raw Markdown (`text/markdown; charset=utf-8`). 404 via `writeJSONError` when the lane is unknown (`ErrLaneUnknown`), `PacketPath` is empty, or the file is missing/unreadable. Never abort the serve process.
**Alternatives considered**: Nested `/api/runs/{runID}/lanes/{laneID}/packet`; query-param `GET /api/packet?...`.
**Rationale**: Same two-segment parse as `/approvals/{runID}/{laneID}` (`internal/serve/handlers.go:316-350`); `/api/` prefix matches existing reads (`internal/serve/handlers.go:195-255`).
**Terminal consumer**: `NewHandlerWithConfig` mux (`internal/serve/handlers.go:190-357`); `app.js` packet link.

### Decision: DAG wave metadata is a follow-up (OQ5)

**Choice**: Do not add SDD keys to `dag.Node` (`internal/dag/parse.go:21-37`) or `dag.EmitPacketContent` (`internal/dag/emit.go:26-76`).
**Alternatives considered**: Emit the keys from apply-DAG waves in this change (`proposal.md:189`).
**Rationale**: Those types serve `apply-dag.yaml` task waves, not SDD planning fan-out. Omitted keys already default empty (`packet.go:94-138`), satisfying the optional-keys scenario.
**Terminal consumer**: `EmitPacketContent` (unchanged); `packet.Parse` (empty defaults).

### Decision: Aggregate telemetry fields and per-decoder mapping

**Choice**: Add `TotalTokens int64`, `CostUSD float64`, `ToolCalls int64` to `ProgressEvent` (`internal/executor/executor.go:18-21`), `ledger.LaneProgress` (`internal/ledger/progress.go:15-20`), and `serve.LaneProgress` (`internal/serve/model.go:186-193`; JSON `total_tokens`, `cost_usd`, `tool_calls`). Mapping: agy `TotalTokens` as-is, `CostUSD=0` (`internal/executor/agy_stream.go:12-18`); claude sum of input+output+cache-read+cache-creation, `CostUSD` from the record (`internal/executor/claude_stream.go:18-21,35`); opencode sum of input+output+reasoning+cache read/write, `CostUSD` from the part (`internal/executor/opencode_stream.go:106-112,123`); cursor-agent emits `0` / `0.0` (`internal/executor/cursor_agent.go:239-270`). Tool-call counts increment on tool events in all four decoders, including cursor-agent.
**Alternatives considered**: Separate input/output token columns.
**Rationale**: Dashboard already consumes aggregate `total_tokens` (`internal/serve/static/app.js:542-544`). cursor-agent zeros tokens/cost rather than omitting fields or failing decode.
**Terminal consumer**: `app.js:542-544`.

### Decision: Persist `tool_calls`, derive `tool_rate` (count vs rate)

**Choice**: Store cumulative `tool_calls INTEGER` on `lane_progress`. JSON also emits derived `tool_rate` (tools/min) in `GetLaneProgress` (`internal/serve/model.go:336-346`): `float64(tool_calls) / max(elapsed_minutes, 1.0/60.0)` from lane `StartedAt` to the progress `At` (1s floor).
**Alternatives considered**: Persist the rate in SQLite.
**Rationale**: Counts are facts; rates are time-window derivatives. Resolves telemetry spec (count) vs batch-wave-view spec (rate) while matching the `/min` formatter (`app.js:542-544`).
**Terminal consumer**: `app.js:542-544`.

### Decision: v7 STRICT rebuild of `runs` and `lane_progress`

**Choice**: One `migrateV6ToV7DDL` constant: create-copy-drop-rename of both tables. `runs.pid INTEGER NOT NULL DEFAULT 0 CHECK (pid >= 0)`. `lane_progress` adds `total_tokens`, `cost_usd`, `tool_calls` (all `NOT NULL DEFAULT 0` with `CHECK >= 0`). Recreate `idx_lane_progress_run_lane_seq` (`internal/ledger/schema.go:306-307`). Pre-migration rows get `pid=0` and zeroed usage. No backfill. Bump `schemaVersion` 6→7 (`internal/ledger/schema.go:10`).
**Alternatives considered**: `ALTER TABLE ADD COLUMN`.
**Rationale**: SQLite cannot add CHECK columns to STRICT tables in place (`internal/ledger/schema.go:183-189,223-224`). Same pattern as v5 and v6 (`internal/ledger/schema.go:190-219,225-308`).
**Terminal consumer**: `migrate` version loop (`internal/ledger/schema.go:313-409`).

### Decision: PID on `RegisterRun`; serve-side sweeper (OQ3, OQ4)

**Choice**: Pass `PID: os.Getpid()` on `ledger.Run` from `runDispatch` (`cmd/lucind-ai/cli.go:314-324`) into `RegisterRun`. New `internal/serve/sweeper.go`: `Sweeper.Run(ctx)` does one immediate sweep then ticks every 10s, mirroring `Hub.Run` (`internal/serve/hub.go:213-235`) without sharing `defaultPollInterval` (`internal/serve/hub.go:24`). Launch beside Hub in `serveDispatch` (`cmd/lucind-ai/cli.go:770-774`). Liveness: `os.FindProcess(pid).Signal(syscall.Signal(0))`. Alive: `err == nil` or `errors.Is(err, syscall.EPERM)`. Dead: `errors.Is(err, syscall.ESRCH)` or `errors.Is(err, os.ErrProcessDone)`. `pid <= 0` skips the probe and leaves `running` unchanged. Dead PID: `SetStatus(..., lane.Failed)` plus `EventLaneNote` ("orphaned: driving process no longer running"). Unknown probe errors log; do not crash. No supervision or restart. Linux-only.
**Alternatives considered**: Per-lane child PIDs; serve PID; `/proc/<pid>`; 100ms or 60s ticker; sweep inside `lucind-ai run`.
**Rationale**: `runDispatch` is the process whose death orphans every in-flight lane. Serve is the long-lived daemon on the shared ledger. 10s is prompt for the dashboard without Hub-style SQLite chatter. `kill(pid, 0)` is the standard existence probe; cross-platform PID checks are out of scope. Zero PID means "untracked" (v7 default), not "dead".
**Terminal consumer**: `RegisterRun` / `scanRun` (`internal/ledger/runs.go:29-41,165-188`); `SetStatus` (`internal/ledger/ledger.go:452-484`); `AppendEvent` (`internal/ledger/ledger.go:366-378`).

## Flow and Invariants

```
CLI --packet path ──Parse──► Packet (+Path, SDD keys)
        │
        ├─ RegisterRun(PID=getpid)
        └─ Execute / ensureLaneFailed
               RegisterLane ──► UpdateLaneMetadata(Skill, PacketPath, …)
               writeLaneProgress(TotalTokens, CostUSD, ToolCalls)
                      │
serve: GET /api/state (laneDTO + tool_rate)
       GET /api/packets/{runID}/{laneID} ──GetLaneMetadata.PacketPath──► os.ReadFile
       Sweeper: startup + 10s ── kill(pid,0) ── running→failed + note
```

Invariants: omitted frontmatter keys are empty strings, never parse errors. Historical metadata/usage/pid rows stay zero/empty. cursor-agent numeric token/cost fields are zero, never omitted. Sweeper never restarts processes. `pid <= 0` is never treated as dead. Packet GET 404s instead of crashing. v7 is one transaction; reopen is idempotent.

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `cmd/lucind-ai/cli.go` | Modify | Set `Packet.Path`; pass `os.Getpid()` to `RegisterRun`; launch `Sweeper` | `run.go:334-344`; `runs.go:29-41`; `cli.go:770-774` |
| `internal/packet/packet.go` | Modify | Add `SDDPhase`, `FanoutGroup`, `Skill`, `Path`; parse optional keys | `cli.go:160-174` |
| `internal/ledger/lanes_meta.go` | Modify | Add `Skill`, `PacketPath` | `model.go:322-333` |
| `internal/run/run.go` | Modify | `UpdateLaneMetadata` after `RegisterLane`; forward telemetry in `writeLaneProgress` | `model.go:288-302`; `progress.go:36-79` |
| `internal/run/batch.go` | Modify | `UpdateLaneMetadata` after `RegisterLane` in `ensureLaneFailed` | `model.go:288-302` |
| `internal/executor/executor.go` | Modify | Numeric fields on `ProgressEvent` | `run.go:562-564` |
| `internal/executor/{agy,claude,opencode}_stream.go` | Modify | Map usage/cost; count tools | `executor.go` Progress channel |
| `internal/executor/cursor_agent.go` | Modify | Zero tokens/cost; count tools | `executor.go` Progress channel |
| `internal/ledger/schema.go` | Modify | `schemaVersion=7`; `migrateV6ToV7DDL`; wire loop | `schema.go:313-409` |
| `internal/ledger/progress.go` | Modify | Numeric fields + SQL | `model.go:336-346` |
| `internal/ledger/runs.go` | Modify | `Run.PID`; insert/select/scan | `runs.go:29-41,165-188` |
| `internal/serve/model.go` | Modify | `Skill`, `PacketPath` on `Lane`; telemetry + `tool_rate` on `LaneProgress` | `app.js:520-560` |
| `internal/serve/handlers.go` | Modify | Register packet GET | `app.js:520-560` |
| `internal/serve/static/app.js` | Modify | Render `skill`; packet link | Browser |
| `internal/serve/sweeper.go` | Create | Sweep, probe, fail+note | `ledger.go:452-484,366-378` |

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

Packet GET: `GET /api/packets/{runID}/{laneID}` → 200 `text/markdown; charset=utf-8` or 404 JSON. `serve.Lane` gains optional `skill`, `packet_path`. `serve.LaneProgress` gains `total_tokens`, `cost_usd`, `tool_calls`, `tool_rate`.

## Testing Strategy

| Layer | What | Approach | Seam |
|---|---|---|---|
| Unit (`packet`) | Optional keys; empty defaults | Table-driven buffers | `internal/packet/packet_test.go:15-67` |
| Unit (`ledger`) | Metadata round-trip with `Skill`/`PacketPath`; PID insert/scan; progress numerics | In-memory SQLite | `lanes_meta_test.go:15-81`; `runs.go:29-41`; `progress_test.go:14-49,56-73` |
| Schema | v6→v7 row preservation, STRICT, reopen idempotency | v6 fixture then migrate | `schema_test.go:14-97,146-201` |
| Decoders | agy/claude/opencode populate usage; cursor-agent zeros tokens/cost and counts tools | Stream fixtures | `agy_stream_test.go:49-60`; claude usage fixtures after `:50-60` argv test; `opencode_stream_test.go:30-60`; `cursor_agent_stream_test.go:14-58` |
| Unit (`run`) | `Execute` / `ensureLaneFailed` call `UpdateLaneMetadata` | Fake ledger | `run_test.go:1-50` |
| HTTP | Packet GET 200/404; no process abort | `httptest` + temp file | `server_test.go:47-100` (status/error pattern, not packet coverage today) |
| Serve DTO | JSON contract for new lane/progress fields and `tool_rate` | Existing JSON asserts | `model_test.go:599-676` |
| Sweeper | Live PID retained; dead PID → `failed`+note; `pid<=0` skipped | Probe self / dead / zero | New tests; `Hub.Run` (`hub.go:213-235`) is the loop pattern only |

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | N/A: packet GET reads a stored path; no executable-file classification | — | — |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: no git CLI repository selection | — | — |
| Commit state | staged, `commit -a`, empty index | N/A: no git commit creation | — | — |
| Push state | tracking branch, first push, explicit refspec | N/A: no git push | — | — |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: no PR command synthesis | — | — |
| Process integration | Live PID, dead PID, PID 0, PID recycling, `EPERM` | Applicable | Safe: `kill(pid,0)` alive on nil/`EPERM`, dead on `ESRCH`/`ErrProcessDone`, skip `pid<=0`. Failure: dead PID → `failed` + note; unknown errors log without crash. Recycled PID is accepted until that PID later dies (no second identity check). | `TestSweeper_LivePIDRetained`, `TestSweeper_DeadPIDReconciled`, `TestSweeper_ZeroPIDIgnored` |

## Rollback and Additivity

Revert the Go commit. v7 columns are additive named columns with defaults; reverted readers that list columns explicitly keep working. Full downgrade is a reverse create-copy-drop-rename dropping `pid` and usage columns and leaving `schema_migrations` at 6. `lane_metadata:v1:` events with new JSON keys are ignored by old readers. Packet GET and sweeper are new serve behavior; `/api/state` is augmented, not replaced. Optional frontmatter keys cannot break legacy packets. No feature flag. No historical backfill to undo.

## Open Questions and Out of Scope

Open Questions 1–5 are closed above. No remaining blocking design questions.

Out of scope: live executor Skill telemetry; `internal/dag` Node/emit changes; process supervision or auto-restart; cross-platform PID liveness; historical-row backfill; `internal/resolve` / `internal/conflicttriage`.
