# Proposal Lens C — Risks, Rollback & Test Impact: Lane Status Observability

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| **Review budget overrun (`size:exception`)**: Six items in one PR exceeds 1200 lines, increasing cognitive review load. | High | **Accepted risk** per user decision (`explore.md:60-66`); mitigation is the explicit `size:exception` declaration, supported by modular files and unit tests. | `openspec/changes/lane-status-observability/explore.md:60-66` |
| **SQLite STRICT table migration failure (v7)**: `runs` and `lane_progress` are STRICT tables that cannot widen in place; migration abort risks database lock or data loss. | High | Use proven transactional create-copy-drop-rename pattern (`migrateV4ToV5DDL`, `migrateV5ToV6DDL`). Migration runs in a single transaction and is idempotent. | `internal/ledger/schema.go:182-219`, `internal/ledger/schema.go:221-308`, `internal/ledger/schema.go:313-409` |
| **PID reuse / false negative in orphan sweep**: Reassigned PID on crashed runner could prevent `serve` from sweeping orphaned `running` lanes. | Med | Compare recorded PID with run start timestamp/bounds; lane will still sweep when the reassigned process terminates. | `internal/ledger/runs.go:16-41`, `cmd/lucind-ai/cli.go:141-170`, `internal/serve/server.go:19-53` |
| **Premature orphan sweep false positive**: Loaded runner process prematurely marked `failed` by `serve`'s ticker. | High | Require verified `ESRCH` before marking failed; conservative ticker interval (15–30s); write diagnostic `lane_note` event. | `internal/ledger/lanes.go:35-50`, `internal/serve/server.go:19-53`, `internal/run/run.go:355-358` |
| **Packet parser backward incompatibility**: Legacy packets without new frontmatter keys fail parsing. | Med | Parse new keys as optional with safe zero values; test backward compatibility against legacy packets. | `internal/packet/packet.go:78-167`, `internal/packet/packet_test.go:15-67` |
| **Executor stream decode drift**: Upstream CLI JSON changes (`agy`, `claude`, `opencode`) cause unmarshal errors. | Low | Permissive union structs; raw stream fallback on error; `cursor-agent` leaves metrics empty. | `internal/executor/agy_stream.go:12-44`, `internal/executor/claude_stream.go:17-36`, `internal/executor/opencode_stream.go:100-120`, `internal/executor/cursor_agent.go:35-50` |
| **Unwired metadata on dispatch failure**: Lane failing before executor lookup leaves metadata unpopulated. | Low | Wire `UpdateLaneMetadata` in both `Execute` (`run.go:334`) and `ensureLaneFailed` (`batch.go:184`). | `internal/run/run.go:334-358`, `internal/run/batch.go:184-210`, `internal/ledger/lanes_meta.go:39-83` |

## Rollback & Additivity

**Rollback Plan**:
- **Code**: `git revert` the merged commit.
- **Schema**: Forward-compatible SQLite v7 migration. Reverting Go code continues reading existing columns safely because queries select explicit column lists (`internal/ledger/runs.go:65-66,82-83`, `internal/ledger/progress.go:56-68,107-110`). If full schema downgrade to v6 is needed, run create-copy-drop-rename dropping new columns and decrement `schema_migrations.version` to 6 (`internal/ledger/schema.go:12-15`).
- **Events**: `UpdateLaneMetadata` appends JSON audit events (`internal/ledger/lanes_meta.go:12-32,67-77`); reverted code safely ignores them.

**Additivity**:
- **Ledger Schema**: **Additive**. Schema v7 widens `runs` (`pid`) and `lane_progress` (`total_tokens`, `cost_usd`, tool metrics) with nullable/defaulted columns (`internal/ledger/schema.go:226-308`). Primary keys are preserved.
- **Ledger APIs**: **Additive**. `UpdateLaneMetadata` (`internal/ledger/lanes_meta.go:39-83`) gains first production callers in `internal/run/run.go:334-358` and `internal/run/batch.go:184-210`.
- **Packet Frontmatter**: **Additive**. `packet.Parse` (`internal/packet/packet.go:93-138`) adds optional keys (`sdd_phase`, `fanout_group`, `skill`). Existing packets parse unchanged.
- **Executor / Telemetry**: **Additive**. `ProgressEvent` (`internal/executor/executor.go:17-21`) and `LaneProgress` (`internal/ledger/progress.go:14-20`) receive optional numeric fields. `cursor-agent` (`internal/executor/cursor_agent.go:1-60`) leaves fields zeroed.
- **HTTP Routes**: **Additive**. New packet body endpoint in `internal/serve/handlers.go:30-120` and model fields in `internal/serve/model.go:163-193` augment payloads without breaking `/api/state` (`internal/serve/static/app.js:525-548`).

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| **Ledger Schema Migration (v6 → v7)** | Test `TestMigrateV6ToV7PreservesRowsAndAddsSchema` creates `runs_new` and `lane_progress_new`, copies v6 rows verbatim, verifies STRICT enforcement, and tests reopen idempotency. | `internal/ledger/schema_test.go:14-97,150-201,247-269` |
| **Ledger CRUD & Metadata** | Test `RegisterRun` with `pid`, `GetRun` scanning `pid`, `AppendProgress` with telemetry, and `UpdateLaneMetadata`/`GetLaneMetadata` round-trip. | `internal/ledger/runs_test.go:1-50`, `internal/ledger/progress_test.go:1-60`, `internal/ledger/lanes_meta_test.go:15-60` |
| **Packet Frontmatter Parsing** | Test `packet.Parse` with new optional keys (`sdd_phase`, `fanout_group`, `skill`) and verify backward compatibility for legacy packets. | `internal/packet/packet_test.go:15-67` |
| **Executor Stream Decoders** | Test `agy`, `claude`, and `opencode` decoders extracting numeric tokens, costs, and tool calls into `ProgressEvent`; verify `cursor-agent` stream emits zero-valued metrics. | `internal/executor/agy_stream_test.go:1-50`, `internal/executor/claude_stream_test.go:1-50`, `internal/executor/opencode_stream_test.go:1-50`, `internal/executor/cursor_agent_stream_test.go:1-50` |
| **Run & Dispatch Lifecycle** | Verify `Execute` and `ensureLaneFailed` invoke `UpdateLaneMetadata` and record runner PID in `RegisterRun` on success and failure paths. | `internal/run/run_test.go:25-60`, `internal/run/batch_test.go:170-210` |
| **Serve Model & Orphan Sweep** | Test `Model.ListLanes` surfacing metadata/telemetry; test packet body HTTP endpoint; unit test orphan sweep transitioning dead-PID lanes `running` → `failed` with audit note. | `internal/serve/model_test.go:599-650`, `internal/serve/server_test.go:42-93` |

## Out of Scope

- **Live runtime "Skill" telemetry**: Executors lack a Claude Code "Skill" tool abstraction (`explore.md:42-47`). Only static `skill:` frontmatter from orchestrators is in scope.
- **Historical ledger backfills**: No backfill for historical runs/lanes executed prior to v7 (`internal/ledger/runs.go:103-137`).
- **Process supervision / auto-restart**: Orphan sweep transitions dead runner lanes to `failed` without restarting processes (`internal/lane/status.go:10-28`).
- **Cross-platform PID interrogation beyond Linux/POSIX**: `/proc` and `syscall.Kill(pid, 0)` scoping beyond supported POSIX environments is excluded.
- **Sidecar DAG engine extensions**: Changes to `internal/dag/parse.go:21-37` and `internal/dag/emit.go:11-60` are deferred to a follow-up.

## Open Questions

- [ ] Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs `generated_by` (`internal/packet/packet.go:93-138`).
- [ ] Packet path persistence: `LaneMetadata.PacketPath` in audit JSON vs `lanes.packet_path` column requiring `lanes` table migration (`internal/ledger/lanes_meta.go:20-32`, `internal/ledger/schema.go:236-267`).
- [ ] Ticker interval for periodic orphan sweep (`internal/serve/server.go:19-53`).
- [ ] PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and portability (`internal/serve/server.go:19-53`).
- [ ] Whether `internal/dag/parse.go`'s `Node` / `internal/dag/emit.go`'s `EmitPacketContent` get new fields now or in a follow-up (`internal/dag/parse.go:21-37`, `internal/dag/emit.go:11-60`).
- [ ] Skill contract drift: `~/.claude/skills/sdd-propose/SKILL.md:92-158` monolithic single-agent template superseded by 3-lens parallel fan-out packet.
