# Proposal: Lane Status Observability

**Chosen candidate: Candidate 1 — wire existing metadata path + PID orphan sweep + structured telemetry, one PR** (`openspec/changes/lane-status-observability/explore.md:58-66`). `delivery_strategy` is `exception-ok`; six items ship together under `size:exception`.

## Intent

The live dashboard renders most lane-card fields as "Unavailable", and lanes can stay `running` for 25+ hours after the driving process is gone. Missing writers, not display bugs.

`ledger.LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`) already carries `Model`, `Agent`, `SDDPhase`, `FanoutGroup`, `Change`, `Feature`, `AllowedPaths`, `Dependencies`, `BodyDigest`. `UpdateLaneMetadata` / `GetLaneMetadata` (`lanes_meta.go:39,89`) already persist it: `model`/`agent`/`feature` on schema-v6 `lanes` columns (`internal/ledger/schema.go:249-251`), the rest as a `lane_metadata:v1:` JSON snapshot on `EventLaneNote` (`lanes_meta.go:12,67-77`). `serve.Lane` (`internal/serve/model.go:163-184`) and `ListLanes` (`model.go:288-299`) already consume it; `app.js:532-538` already falls back to "Unavailable". **Zero production callers of `UpdateLaneMetadata`** — only tests. `internal/run/run.go:334` (`Execute`) and `internal/run/batch.go:184` (`ensureLaneFailed`) call `RegisterLane` and never follow up.

`packet.Parse` (`internal/packet/packet.go:78-167`) recognizes `id, executor, routed_by, model, agent, read_only, feature, parent_ref, base_sha, expected_parent_sha, legacy_main, allowed_paths` (`packet.go:94-138`). No `sdd_phase` / `fanout_group` / `skill` key; `Packet` (`packet.go:33-75`) has no `Path` or `Skill`. Only `cmd/lucind-ai/cli.go:160-174` knows the on-disk path via index-aligned `packetFlags`/`ps`.

Usage numbers are parsed and discarded as prose: `agyUsage` (`internal/executor/agy_stream.go:12-18`) via `formatAgyUsage` (`:160-162`); `claudeUsage`+`CostUSD` (`claude_stream.go:17-36`) via `formatClaudeUsage` (`:212-218`); `opencodeTokens`+`Cost` (`opencode_stream.go:100-123`) into `ProgressEvent.Message` (`opencode_stream.go:226-228`). `ProgressEvent` (`executor.go:17-21`) and `ledger.LaneProgress` (`internal/ledger/progress.go:15-20`) have no numeric fields. The SSE hub reads `lane_progress` (`schema.go:298-307`), which has none either. `cursor-agent` has no usage struct (`cursor_agent.go:1-60`; `cursor_agent_stream.go` parses tools only). `app.js:542-544` already looks for `total_tokens`, `cost_usd`, and `tool_rate`.

No PID or heartbeat is stored (`ledger.Run`, `internal/ledger/runs.go:16-24`; `RegisterRun` at `:29-40` and `cli.go:314-321` insert no `pid`). No orphan-sweep code exists. `runs` and `lane_progress` are STRICT; SQLite cannot widen them in place (`schema.go:182-189,221-224`).

## Scope

### In Scope
- Call `UpdateLaneMetadata` after `RegisterLane` in `Execute` and `ensureLaneFailed`.
- Parse optional static `sdd_phase`, `fanout_group`, `skill` frontmatter (working names; see Open Questions).
- Persist dispatched packet path; serve the `.md` body from a new `internal/serve` route; link it from lane cards.
- Structured token/cost/tool-call fields on `ProgressEvent`/`LaneProgress` for `agy`, `claude`, `opencode`.
- Schema v7: `runs.pid` + `lane_progress` usage/tool columns, one STRICT create-copy-drop-rename.
- Store runner PID on `RegisterRun`; `serve` sweeps dead-PID `running` lanes to `failed` on startup and a ticker.

### Out of Scope
- Live runtime "Skill" telemetry from any executor (`agy`/`cursor-agent`/`opencode` are not Claude Code). Static `skill:` frontmatter only. Live activity is generic tool-call counts from the same decoders that already parse usage.
- `cursor-agent` usage telemetry.
- Backfilling historical ledger rows.
- Process supervision or auto-restart (sweep marks `failed` only).
- Changing `internal/dag` packet emission unless Open Question 5 brings it in.
- Cross-platform PID-liveness beyond what Open Question 4 settles.

## Capabilities

### New Capabilities
- `lane-progress-telemetry`: numeric tokens, USD cost, and generic tool-call counts on progress for `agy`/`claude`/`opencode`.
- `orphan-lane-reconciliation`: runner PID on `runs`; serve-side sweep of orphaned `running` lanes to `failed`.
- `dispatched-packet-body`: HTTP GET of the exact dispatched packet markdown by run/lane.

### Modified Capabilities
- `lane-execution`: dispatch callers invoke `UpdateLaneMetadata` after `RegisterLane`.
- `read-only-packet-schema`: `packet.Parse` accepts optional `sdd_phase`, `fanout_group`, `skill` (read_only unchanged).
- `batch-wave-view`: lane cards render metadata, packet link, usage, and swept-orphan `failed`.

## Approach

Hooks: `RegisterLane` at `internal/run/run.go:334-344` and `internal/run/batch.go:184-193`, then `UpdateLaneMetadata`. PID: production `RegisterRun` at `cmd/lucind-ai/cli.go:314-321`.

1. **Metadata.** After `RegisterLane`, populate `LaneMetadata` from `packet.Packet` (`Model`, `Agent`, `Feature`, plus `SDDPhase`/`FanoutGroup`/`Change`/`Skill`/`AllowedPaths`/`Dependencies`/`BodyDigest`). Keep the v6 split: queryable `lanes` columns vs `lane_metadata:v1:` JSON in `events`. No backfill.
2. **Frontmatter.** Extend `packet.Parse` (`packet.go:94-138`) with optional keys defaulting to empty; absent keys must not fail parse. Working names `sdd_phase`/`fanout_group`/`skill` pending Open Question 1.
3. **Packet body.** Capture the on-disk path from `cli.go:160-174`. Persistence mechanism is Open Question 2. New GET route on the existing mux (`internal/serve/handlers.go:190`); 200 with verbatim markdown, 404 for unknown run/lane.
4. **Telemetry.** Add optional `total_tokens`, `cost_usd`, and tool-call metrics to `ProgressEvent` (`executor.go:17-21`) and `LaneProgress` (`progress.go:15-20`; serve DTO `model.go:187-193`). Decoders populate them beside existing `emit()` prose: `agy_stream.go:12-18,27-39`, `claude_stream.go:17-36,46-60`, `opencode_stream.go:100-125`. `cursor-agent` leaves them zero.
5. **Schema v7.** Follow `migrateV4ToV5DDL` (`schema.go:182-219`) and `migrateV5ToV6DDL` (`schema.go:221-308`): `runs_new` (add `pid`) and `lane_progress_new` (add usage/tool columns), `INSERT ... SELECT` verbatim, drop, rename. One transaction; `migrate` is already idempotent (`schema.go:310-409`). `schemaVersion` is 6 (`schema.go:10`).
6. **Orphan sweep.** `serve` and `lucind-ai run` are separate processes on the same ledger file. Sweep at serve startup and on a ticker (interval is Open Question 3). Dead stored PID → still-`running` lanes → `failed` via `SetStatus` (`run.go:355` is the running transition; `ledger.SetStatus` writes `lane_status_changed`) plus an `EventLaneNote` ("orphaned: driving process no longer running"). Liveness syscall is Open Question 4. Not a heartbeat protocol.

Rejected: two-PR split (`explore.md:60-66`, explore's own default); metadata-only slice; live executor Skill telemetry.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/run/run.go:334`, `batch.go:184` | Modified | `UpdateLaneMetadata` after `RegisterLane`. |
| `internal/packet/packet.go:33-75,94-138` | Modified | Optional SDD/skill keys. |
| `cmd/lucind-ai/cli.go:160-174,314-321` | Modified | Packet path mapping; PID on `RegisterRun`. |
| `internal/ledger/schema.go`, `runs.go:16-40`, `progress.go:15-20` | Modified | v7 STRICT rebuild; `pid`; numeric progress. |
| `internal/executor/{agy,claude,opencode}_stream.go` | Modified | Structured usage/tool fields. |
| `internal/serve/` | Modified | Packet GET; orphan sweeper; DTO/UI already shaped (`model.go:163-193`, `app.js:532-544`). |

## User and Capability Impact

| Who | Change |
|-----|--------|
| Operators on the dashboard | Real `MODEL` / `SDD PHASE` / `FANOUT GROUP` / `skill` for future dispatches; historical rows stay empty. |
| Operators | Link to the dispatched packet body. |
| Operators | Token/cost and generic tool-call activity for `agy`/`claude`/`opencode` (not `cursor-agent`). |
| Operators | Orphaned lanes become `failed` with a clear note instead of `running` for days. |

## Delta Specifications

### Requirement: Lane metadata dispatch persistence

`Execute` and `ensureLaneFailed` MUST call `UpdateLaneMetadata` after `RegisterLane` (`run.go:334-344`, `batch.go:184-193`, `lanes_meta.go:20-83`). Historical rows SHALL NOT be backfilled.

#### Scenario: Dispatch persists metadata
- GIVEN a packet with model, agent, and feature dispatched via `run.Execute`
- WHEN `RegisterLane` succeeds
- THEN `UpdateLaneMetadata` MUST persist attributes and `serve.Lane` MUST return them rather than "Unavailable"

#### Scenario: Historical rows preserved
- GIVEN pre-existing lane rows
- WHEN `serve.ListLanes` queries the ledger
- THEN legacy rows MUST be returned as recorded

### Requirement: Extended packet frontmatter parsing

`packet.Parse` MUST parse optional `sdd_phase`, `fanout_group`, and `skill` into `Packet` (`packet.go:33-75,78-167`). Missing keys MUST default to empty strings. Parse MUST NOT fail when they are absent. Live executor Skill telemetry SHALL NOT be decoded.

#### Scenario: Parse frontmatter keys
- GIVEN a packet with `sdd_phase: propose`, `fanout_group: lens-b`, `skill: sdd-propose`
- WHEN `packet.Parse` parses it
- THEN the new fields MUST match the declared values

#### Scenario: Optional keys omitted
- GIVEN a packet omitting those keys
- WHEN `packet.Parse` parses it
- THEN parse MUST succeed and the new fields MUST be empty

### Requirement: Dispatched packet body inspection

The CLI MUST preserve packet-path-to-lane mapping (`cli.go:160-174`). `internal/serve` MUST return the verbatim markdown for a run/lane.

#### Scenario: Retrieve packet content
- GIVEN a lane dispatched from a packet file
- WHEN a client requests that lane's packet endpoint
- THEN the server MUST return HTTP 200 with the exact markdown

#### Scenario: Unknown lane returns 404
- GIVEN a non-existent run or lane
- WHEN the endpoint executes
- THEN the server MUST return HTTP 404

### Requirement: Structured progress telemetry

`ProgressEvent` (`executor.go:17-21`) and `LaneProgress` (`progress.go:15-20`) MUST carry optional `total_tokens`, `cost_usd`, and generic tool-call counts. `agy` (`agy_stream.go:12-39`), `claude` (`claude_stream.go:17-36`), and `opencode` (`opencode_stream.go:100-125`) MUST populate them alongside messages. `cursor-agent` MUST leave them zero. v7 MUST add STRICT numeric columns to `lane_progress` (`schema.go:298-307`).

#### Scenario: Decoders populate usage
- GIVEN an `agy`, `claude`, or `opencode` lane emitting usage records
- WHEN stream chunks decode
- THEN `LaneProgress` MUST persist populated token, cost, and tool metrics

#### Scenario: Cursor-agent emits zeroed metrics
- GIVEN a `cursor-agent` lane
- WHEN progress is recorded
- THEN numeric metrics MUST remain zero

### Requirement: Orphaned lane reconciliation

`RegisterRun` MUST record the runner PID (`runs.go:16-40`); v7 adds `pid` to `runs` (`schema.go:226-234`). `internal/serve` MUST sweep at startup and on a ticker. A dead PID MUST flip associated `running` lanes to `failed` with an `EventLaneNote`. An alive PID MUST leave `running` lanes untouched.

#### Scenario: Dead-process lane swept to failed
- GIVEN a run whose process has died while a lane remains `running`
- WHEN serve executes the sweep
- THEN the lane MUST become `failed` with an explanatory `EventLaneNote`

#### Scenario: Active process lanes unchanged
- GIVEN a run whose process is alive
- WHEN the sweep runs
- THEN `running` lanes MUST remain untouched

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Review budget overrun (`size:exception`) | High | **Accepted.** Named exception; modular files and tests. Not a defect to "fix". |
| STRICT v7 abort / lock | High | Same transactional create-copy-drop-rename as v5/v6 (`schema.go:182-219,221-308,310-409`). |
| PID reuse hides an orphan | Med | Sweep again when the reassigned PID later dies; do not invent a second identity check until design. |
| Sweep false-positive (live runner marked failed) | High | Conservative liveness check and ticker (Open Questions 3–4); diagnostic `lane_note`. |
| Legacy packets fail parse | Med | New keys optional; prove against existing `packet_test.go` cases (`:15-67`). |
| Upstream stream JSON drift | Low | Existing union structs already skip unknown shapes; `cursor-agent` stays zeroed. |
| Failure before executor lookup skips metadata | Low | Wire both `Execute` (`run.go:334`) and `ensureLaneFailed` (`batch.go:184`). |

## Rollback Plan and Additivity

**Rollback:** `git revert` the merge. v7 is additive. Reverted Go still reads old columns because queries list them explicitly (`runs.go:65-66,82-83`; `progress.go:56-68,107-110`). Full downgrade to v6 is a reverse create-copy-drop-rename dropping new columns and leaving `schema_migrations` at 6 (`schema.go:10-15`). `lane_metadata:v1:` events (`lanes_meta.go:12,67-77`) are ignored by reverted readers.

**Additivity:** v7 widens `runs` and `lane_progress` with defaulted columns; PKs preserved. `UpdateLaneMetadata` gains its first production callers. Frontmatter keys are optional. Progress numerics are optional. Packet GET and sweeper are new serve behavior; `/api/state` (`app.js:525-544`) is augmented, not replaced.

## Test and Validation Impact

| Layer | Coverage |
|-------|----------|
| Schema v6→v7 | Preserve-rows + STRICT + reopen idempotency, mirroring `schema_test.go:14-97,146-201,247-269`. |
| Ledger CRUD | `RegisterRun`/`GetRun` with `pid`; `AppendProgress` numerics; `UpdateLaneMetadata` round-trip (`lanes_meta_test.go:15-60`, `runs_test.go:12-43`, `progress_test.go:14-49`). |
| Packet | Optional new keys; legacy packets still parse (`packet_test.go:15-67`). |
| Decoders | `agy`/`claude`/`opencode` populate numerics; `cursor-agent` stays zero (`agy_stream_test.go`, `claude_stream_test.go`, `opencode_stream_test.go`, `cursor_agent_stream_test.go`). |
| Dispatch | `Execute` and `ensureLaneFailed` call `UpdateLaneMetadata`; `RegisterRun` stores PID. |
| Serve | `ListLanes` metadata (`model_test.go:599-650`); packet GET 200/404; sweep `running`→`failed` with note. |

## Open Questions

These stay open. Design must not guess them closed.

1. Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs `generated_by`.
2. Packet path persistence: a new `LaneMetadata.PacketPath` field (audit-event JSON, no migration) vs. a real `lanes` column (migration, but queryable/indexable).
3. Ticker interval for the periodic orphan sweep.
4. PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and whether cross-platform portability beyond Linux is in scope.
5. Whether `internal/dag/parse.go`'s `Node` (`:21-37`) / `internal/dag/emit.go`'s `EmitPacketContent` (`:11-60`) get the same new fields in this change or a follow-up.
