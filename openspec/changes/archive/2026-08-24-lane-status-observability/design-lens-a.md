# Design Lens A — Metadata, Frontmatter & Packet Body: Lane Status Observability

## Assumed architecture

Static dispatch metadata (`Model`, `Agent`, `Feature`, `SDDPhase`, `FanoutGroup`, `Skill`, `PacketPath`, `AllowedPaths`, `Dependencies`, `BodyDigest`) is wired through `internal/run` into `internal/ledger` via `UpdateLaneMetadata` immediately after `RegisterLane`. Frontmatter parsing in `internal/packet` extracts optional `sdd_phase`, `fanout_group`, and `skill` keys, while the CLI attaches the on-disk packet path. The serve layer (`internal/serve`) exposes `GET /api/packets/{runID}/{laneID}` returning raw packet Markdown and surfaces metadata and packet links in `/api/state`. Schema v7 migration (owned by Lens B) widens `runs.pid` and `lane_progress` usage metrics, while runner PID registration and periodic dead-process reconciliation (owned by Lens C) sweep orphaned `running` lanes to `failed`.

## Decision 1 — Frontmatter key names (resolves Open Question 1)

**Choice**: `sdd_phase`, `fanout_group`, and `skill`.
**Alternatives considered**: `phase`, `group`, and `generated_by` (`openspec/changes/lane-status-observability/proposal.md:185`).
**Rationale**: Matches repository snake_case frontmatter conventions matching Go struct fields (`read_only`->`ReadOnly`, `parent_ref`->`ParentRef`, `allowed_paths`->`AllowedPaths` in `internal/packet/packet.go:94-138`) and existing `LaneMetadata` fields/tags (`SDDPhase`->`sdd_phase`, `FanoutGroup`->`fanout_group` in `internal/ledger/lanes_meta.go:25-26`). `phase`/`group` collide with generic terms; `generated_by` mislabels the skill as a generator.
**Terminal consumer**: `internal/packet/packet.go:94-138` (frontmatter parser switch mapping keys to `Packet` fields) and `internal/serve/static/app.js:534-536` (lane card rendering `sdd_phase`, `fanout_group`, and `skill`).

## Decision 2 — Packet-path persistence mechanism (resolves Open Question 2)

**Choice**: JSON audit-event field on `LaneMetadata` (`LaneMetadata.PacketPath string json:"packet_path"`, serialized into `lane_metadata:v1:` snapshot in `events`).
**Alternatives considered**: A dedicated `packet_path` column on the `lanes` table (`openspec/changes/lane-status-observability/proposal.md:186`).
**Rationale**: Follows `LaneMetadata` precedent (`internal/ledger/lanes_meta.go:20-32` and `internal/ledger/lanes_meta.go:67-77`), where only index attributes (`Model`, `Agent`, `Feature`) occupy `lanes` columns (`internal/ledger/schema.go:249-251`), while extended metadata (`SDDPhase`, `FanoutGroup`, `Change`, `AllowedPaths`, `Dependencies`, `BodyDigest`) lives in the `lane_metadata:v1:` JSON payload in `events`. Packet path is accessed exclusively via `(run_id, lane_id)` point lookups in `GetLaneMetadata` (`internal/ledger/lanes_meta.go:89-127`), never queried across lanes. Storing `PacketPath` in `LaneMetadata` avoids SQLite schema DDL changes or STRICT table rebuilds in migration v7.
**Terminal consumer**: `internal/serve/handlers.go:190-357` (packet-body route reading `metadata.PacketPath` via `GetLaneMetadata` to read the file from disk) and `internal/serve/model.go:322-333` (`laneDTO` mapping `metadata.PacketPath` to `serve.Lane`).

## Decision 3 — Packet-body HTTP endpoint

**Choice**: Method `GET`, path `/api/packets/{runID}/{laneID}`. Returns HTTP 200 with raw packet Markdown (`Content-Type: text/markdown; charset=utf-8`). Returns HTTP 404 (`writeJSONError(w, http.StatusNotFound, "...")`) if the run/lane is unknown (`ErrLaneUnknown`), if `metadata.PacketPath` is empty, or if the file cannot be read (`os.IsNotExist` or read error), never terminating the server process.
**Alternatives considered**: `GET /api/runs/{runID}/lanes/{laneID}/packet` (nested path) or query params `GET /api/packet?run_id=...&lane_id=...`.
**Rationale**: Follows `/approvals/{runID}/{laneID}` two-segment path parsing (`internal/serve/handlers.go:316-350`), under `/api/` matching all read routes (`/api/features`, `/api/attempts/`, `/api/approvals`, `/api/batch/` in `internal/serve/handlers.go:195-255`).
**Terminal consumer**: `internal/serve/handlers.go:190-357` (route registration and HTTP handler execution) and `internal/serve/static/app.js:520-560` (browser client fetching packet content for inspection).

## Decision 4 — DAG-wave packet metadata scope (resolves Open Question 5)

**Choice**: Explicit follow-up (out of scope for this change).
**Alternatives considered**: Adding `sdd_phase`, `fanout_group`, `skill` to `dag.Node` (`internal/dag/parse.go:21-37`) and `dag.EmitPacketContent` (`internal/dag/emit.go:26-76`) in this change (`openspec/changes/lane-status-observability/proposal.md:189`).
**Rationale**: `internal/dag/parse.go:21-37` and `internal/dag/emit.go:26-76` serve task implementation waves (`apply-dag.yaml`), not SDD planning fanouts. When DAG-emitted packets omit `sdd_phase`, `fanout_group`, and `skill`, `packet.Parse` (`internal/packet/packet.go:94-138`) safely defaults them to empty strings, satisfying `openspec/changes/lane-status-observability/specs/read-only-packet-schema/spec.md:15-20`'s "Optional keys omitted" scenario without breaking.
**Terminal consumer**: `internal/dag/emit.go:26-76` (emitting packets without SDD keys) and `internal/packet/packet.go:94-138` (defaulting omitted frontmatter keys to empty strings).

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `packet.Packet` | `internal/packet/packet.go:33-75` | Add `SDDPhase`, `FanoutGroup`, `Skill`, `Path` string fields | Yes (zero values on existing callers) |
| `packet.Parse` frontmatter | `internal/packet/packet.go:94-138` | Parse optional `sdd_phase`, `fanout_group`, `skill` keys | Yes (omitted keys default to empty strings) |
| `ledger.LaneMetadata` | `internal/ledger/lanes_meta.go:20-32` | Add `Skill` and `PacketPath` string fields with JSON tags | Yes (unmarshals omitted fields as empty strings) |
| `serve.Lane` & `laneDTO` | `internal/serve/model.go:163-184,322-333` | Add `Skill` and `PacketPath` fields to JSON state DTO | Yes (new optional JSON fields on existing DTO) |
| HTTP Mux Routes | `internal/serve/handlers.go:190-357` | Add `GET /api/packets/{runID}/{laneID}` | Yes (new endpoint, no existing route modified) |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `cmd/lucind-ai/cli.go` | Modify | Populate `p.Path = path` during `--packet` flag loop (`cmd/lucind-ai/cli.go:160-174`) | `internal/run/run.go:334-344` |
| `internal/packet/packet.go` | Modify | Add fields to `Packet`; parse optional keys in `Parse` (`internal/packet/packet.go:33-75,94-138`) | `cmd/lucind-ai/cli.go:160-174` |
| `internal/ledger/lanes_meta.go` | Modify | Add `Skill` and `PacketPath` to `LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`) | `internal/serve/model.go:322-333` |
| `internal/run/run.go` | Modify | Call `UpdateLaneMetadata` after `RegisterLane` (`internal/run/run.go:334-344`) | `internal/serve/model.go:288-302` |
| `internal/run/batch.go` | Modify | Call `UpdateLaneMetadata` after `RegisterLane` in `ensureLaneFailed` (`internal/run/batch.go:184-193`) | `internal/serve/model.go:288-302` |
| `internal/serve/model.go` | Modify | Add `Skill` and `PacketPath` to `serve.Lane` in `laneDTO` (`internal/serve/model.go:163-184,322-333`) | `internal/serve/static/app.js:520-560` |
| `internal/serve/handlers.go` | Modify | Register `GET /api/packets/{runID}/{laneID}` route (`internal/serve/handlers.go:190-357`) | `internal/serve/static/app.js:520-560` |

## Testing Strategy (this slice)

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit (`packet`) | Parse optional `sdd_phase`, `fanout_group`, `skill` frontmatter; empty defaults | Table-driven subtests reading string buffers | `internal/packet/packet_test.go:15-67` |
| Unit (`ledger`) | `UpdateLaneMetadata` and `GetLaneMetadata` round-trip with `Skill` and `PacketPath` | Test ledger with in-memory SQLite DB | `internal/ledger/lanes_meta_test.go:15-81` |
| Unit (`run`) | `Execute` and `ensureLaneFailed` invoke `UpdateLaneMetadata` | Mock/fake ledger verifying metadata audit write after registration | `internal/run/run_test.go:1-50` |
| Unit / Integration (`serve`) | `GET /api/packets/{runID}/{laneID}` returns 200 with raw Markdown, 404 for missing/unreadable | `httptest.NewRecorder` against handler with temp file | `internal/serve/server_test.go:47-100` |
| Unit (`serve`) | `ListLanes` and `GetLane` populate `Skill` and `PacketPath` in `Lane` DTO | Model test verifying JSON serialization | `internal/serve/model_test.go:599-663` |

## Open Questions

- [ ] Skill sdd-design conflict: `~/.claude/skills/sdd-design/SKILL.md` defines a single-agent complete `design.md` document, while this packet instructs authoring `design-lens-a.md` (under 1000 words) as feedstock for synthesis; superseding packet instructions were followed per packet authority.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:160-174` | CLI iterates over packet flags and parses them into Packet slice |
| `internal/dag/emit.go:26-76` | EmitPacketContent produces packet frontmatter without SDD/skill keys |
| `internal/dag/parse.go:21-37` | Node struct declares DAG wave packet fields |
| `internal/ledger/lanes_meta.go:20-32` | LaneMetadata struct definition lacking Skill and PacketPath fields |
| `internal/ledger/lanes_meta.go:25-26` | LaneMetadata defines SDDPhase and FanoutGroup with json tags |
| `internal/ledger/lanes_meta.go:67-77` | UpdateLaneMetadata inserts JSON snapshot into events with EventLaneNote |
| `internal/ledger/lanes_meta.go:89-127` | GetLaneMetadata retrieves columns and decodes latest metadata snapshot |
| `internal/ledger/lanes_meta_test.go:15-81` | Tests verifying UpdateLaneMetadata and GetLaneMetadata round-trip |
| `internal/ledger/schema.go:249-251` | Lanes table columns model, agent, and feature in schema v6 |
| `internal/packet/packet.go:33-75` | Packet struct definition lacking SDDPhase, FanoutGroup, Skill, and Path |
| `internal/packet/packet.go:94-138` | Parse frontmatter switch mapping snake_case keys to struct fields |
| `internal/packet/packet_test.go:15-67` | Tests verifying packet Parse separates frontmatter from body and handles fields |
| `internal/run/batch.go:184-193` | ensureLaneFailed calls RegisterLane without following up with metadata |
| `internal/run/run.go:334-344` | Execute calls RegisterLane without following up with metadata |
| `internal/run/run_test.go:1-50` | Execute test suite fixtures and fake executor doubles |
| `internal/serve/handlers.go:190-357` | NewHandlerWithConfig sets up HTTP mux routes |
| `internal/serve/handlers.go:195-255` | Read endpoint handlers registered under /api/ prefix |
| `internal/serve/handlers.go:316-350` | Two-segment path parsing for /approvals/{runID}/{laneID} |
| `internal/serve/model.go:163-184` | serve.Lane struct definition for JSON serialization |
| `internal/serve/model.go:288-302` | ListLanes queries ledger lanes and retrieves LaneMetadata |
| `internal/serve/model.go:322-333` | laneDTO maps ledger.Lane and LaneMetadata to serve.Lane |
| `internal/serve/model_test.go:599-663` | Tests verifying serve.Model Lane JSON serialization and metadata |
| `internal/serve/server_test.go:47-100` | HTTP handler tests verifying status codes and error responses |
| `internal/serve/static/app.js:520-560` | Dashboard renderState extracts lane metadata and usage |
| `internal/serve/static/app.js:534-536` | Dashboard reads sdd_phase, fanout_group, and feature for lane cards |
| `openspec/changes/lane-status-observability/proposal.md:185` | Open Question 1 regarding frontmatter key naming options |
| `openspec/changes/lane-status-observability/proposal.md:186` | Open Question 2 regarding packet path persistence mechanism |
| `openspec/changes/lane-status-observability/proposal.md:189` | Open Question 5 regarding DAG-wave packet metadata scope |
| `openspec/changes/lane-status-observability/specs/read-only-packet-schema/spec.md:15-20` | Scenario for handling omitted optional frontmatter keys |
