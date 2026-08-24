# Tasks Lens A — Metadata, Frontmatter & Packet Body: Lane Status Observability

## Assumed decomposition

Lens A delivers three sequential phases covering frontmatter extension, dispatch metadata persistence, and packet body inspection. Phase A1 extends packet frontmatter parsing with optional SDD keys and records on-disk packet paths at CLI invocation. Phase A2 updates ledger metadata snapshots and hooks dispatch execution to persist lane attributes. Phase A3 exposes the verbatim packet body over HTTP and surfaces skill metadata and packet links in the dashboard. The critical path runs A1.2 -> A1.4 -> A2.2 -> A2.4 -> A3.2 -> A3.4.

## Phase A1: Packet Frontmatter & CLI Path

- [ ] A1.1 [RED] Add unit tests in `internal/packet/packet_test.go` for parsing optional `sdd_phase`, `fanout_group`, and `skill` keys, empty defaults, explicit empty values, and static non-telemetry handling.
- [ ] A1.2 [GREEN] Add `SDDPhase`, `FanoutGroup`, `Skill`, and `Path` to `Packet` and parse `sdd_phase`, `fanout_group`, and `skill` frontmatter keys in `internal/packet/packet.go`.
- [ ] A1.3 [RED] Add unit tests in `cmd/lucind-ai/cli_test.go` verifying `--packet` flags populate `Packet.Path`.
- [ ] A1.4 [GREEN] Assign `Packet.Path` from `--packet` flags during packet loading loop in `cmd/lucind-ai/cli.go`.

## Phase A2: Ledger Metadata & Dispatch Persistence

- [ ] A2.1 [RED] Add unit tests in `internal/ledger/lanes_meta_test.go` for `Skill` and `PacketPath` round-trip in `UpdateLaneMetadata` and `GetLaneMetadata`.
- [ ] A2.2 [GREEN] Add `Skill` and `PacketPath` JSON fields to `LaneMetadata` in `internal/ledger/lanes_meta.go`.
- [ ] A2.3 [RED] Add unit tests in `internal/run/run_test.go` verifying `Execute` and `ensureLaneFailed` persist metadata via `UpdateLaneMetadata`.
- [ ] A2.4 [GREEN] Call `UpdateLaneMetadata` after `RegisterLane` in `internal/run/run.go` (`Execute`) and `internal/run/batch.go` (`ensureLaneFailed`).

## Phase A3: Packet Body Endpoint & Serve/UI Integration

- [ ] A3.1 [RED] Add unit tests in `internal/serve/model_test.go` verifying `Lane` struct and `laneDTO` map `Skill` and `PacketPath`.
- [ ] A3.2 [GREEN] Add `Skill` and `PacketPath` to `Lane` and wire them in `laneDTO` in `internal/serve/model.go`.
- [ ] A3.3 [RED] Add HTTP tests in `internal/serve/server_test.go` for `GET /api/packets/{runID}/{laneID}` (200 raw markdown, 404 unknown lane, 404 empty path, 404 unreadable file without crashing).
- [ ] A3.4 [GREEN] Register `GET /api/packets/{runID}/{laneID}` in `internal/serve/handlers.go` returning 200 `text/markdown; charset=utf-8` or 404 JSON via `writeJSONError`.
- [ ] A3.5 [RED] Add asset contract tests in `internal/serve/static_test.go` verifying `app.js` normalizes `skill` and renders packet link.
- [ ] A3.6 [GREEN] Update `internal/serve/static/app.js` to normalize `skill`, render it in fleet cards, and add packet markdown link.

## Dependency Order (this slice)

| Task | Depends on | Why |
|---|---|---|
| A1.1 | — | Extends existing parser unit tests |
| A1.2 | A1.1 | Production parser satisfies RED tests |
| A1.3 | A1.2 | CLI test requires `Packet.Path` field |
| A1.4 | A1.3 | CLI argument assignment satisfies RED tests |
| A2.1 | — | Ledger metadata test is self-contained |
| A2.2 | A2.1 | Metadata struct fields satisfy RED tests |
| A2.3 | A1.2, A1.4, A2.2 | Run tests need `Packet` fields and `LaneMetadata` |
| A2.4 | A2.3 | Dispatch metadata wiring satisfies RED tests |
| A3.1 | A2.2 | Serve model test requires `LaneMetadata` fields |
| A3.2 | A3.1 | Serve model DTO mapping satisfies RED tests |
| A3.3 | A3.2 | HTTP tests need `Lane` model metadata mapping |
| A3.4 | A3.3 | HTTP endpoint implementation satisfies RED tests |
| A3.5 | — | Static asset tests assert UI contracts |
| A3.6 | A3.4, A3.5 | Dashboard updates satisfy asset tests |

## Suggested Work Unit

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| Lens A Capability Slice | Frontmatter extension, dispatch metadata persistence, and packet body HTTP endpoint with UI link | `cmd/lucind-ai/cli.go`, `cmd/lucind-ai/cli_test.go`, `internal/packet/packet.go`, `internal/packet/packet_test.go`, `internal/ledger/lanes_meta.go`, `internal/ledger/lanes_meta_test.go`, `internal/run/run.go`, `internal/run/batch.go`, `internal/run/run_test.go`, `internal/serve/handlers.go`, `internal/serve/model.go`, `internal/serve/model_test.go`, `internal/serve/server_test.go`, `internal/serve/static/app.js`, `internal/serve/static_test.go` | agy | Revert Go code and JS changes; additive metadata snapshot and new route leave existing callers unaffected. |

## RED Tests from the Threat Matrix (this slice)

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| None — see lens C | No | — | — | — |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| A1.1, A1.2 | `go test -v ./internal/packet -run 'TestParse.*(SDDPhase\|FanoutGroup\|Skill)'` | Frontmatter keys parse into `Packet` and default to empty strings | CLI flag parsing or ledger persistence |
| A1.3, A1.4 | `go test -v ./cmd/lucind-ai -run 'Test.*PacketPath'` | CLI assigns `--packet` flag to `Packet.Path` | Ledger persistence during execution |
| A2.1, A2.2 | `go test -v ./internal/ledger -run 'Test.*LaneMetadata.*(Skill\|PacketPath)'` | `LaneMetadata` round-trips `Skill` and `PacketPath` in event snapshots | `run.Execute` calls `UpdateLaneMetadata` |
| A2.3, A2.4 | `go test -v ./internal/run -run 'Test.*UpdateLaneMetadata'` | `Execute` and `ensureLaneFailed` persist metadata after `RegisterLane` | HTTP endpoint retrieval |
| A3.1, A3.2 | `go test -v ./internal/serve -run 'TestModelRunLaneAndProgressJSONContract'` | `Lane` and `laneDTO` expose `Skill` and `PacketPath` in JSON | HTTP route handler execution |
| A3.3, A3.4 | `go test -v ./internal/serve -run 'TestPacketBodyEndpoint'` | `GET /api/packets/{runID}/{laneID}` returns 200 markdown or 404 without crashing | Frontend UI rendering |
| A3.5, A3.6 | `go test -v ./internal/serve -run 'TestFleetView.*Skill'` | `app.js` normalizes `skill` and renders packet markdown link | Browser visual rendering |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `Extended packet frontmatter parsing` | A1.1, A1.2 |
| `Dispatched packet body inspection` | A1.3, A1.4, A2.1, A2.2, A2.3, A2.4, A3.1, A3.2, A3.3, A3.4, A3.5, A3.6 |
| `Lane metadata dispatch persistence` | A2.1, A2.2, A2.3, A2.4, A3.1, A3.2 |

## Open Questions

- [ ] Overlap on `internal/serve/model.go` and `internal/serve/static/app.js` with Lens B (telemetry fields and `tool_rate` derivation): synthesizer must sequence or merge edits in single PR.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:160-174` | CLI packet loading loop parses packets without assigning on-disk `Path` |
| `cmd/lucind-ai/cli_test.go:40-60` | CLI test suite structure for command flag and execution testing |
| `internal/ledger/lanes_meta.go:20-32` | `LaneMetadata` struct definition lacking `Skill` and `PacketPath` fields |
| `internal/ledger/lanes_meta.go:39-83` | `UpdateLaneMetadata` transactional update and event note snapshot persistence |
| `internal/ledger/lanes_meta.go:89-128` | `GetLaneMetadata` snapshot retrieval falling back to v6 lane columns |
| `internal/ledger/lanes_meta_test.go:15-81` | `TestUpdateAndGetLaneMetadata` verifies metadata snapshot round-trip |
| `internal/ledger/lanes_meta_test.go:177-203` | `TestGetLaneMetadataWithoutAuditReturnsV6Columns` verifies legacy fallback |
| `internal/packet/packet.go:33-75` | `Packet` struct fields lacking `SDDPhase`, `FanoutGroup`, `Skill`, and `Path` |
| `internal/packet/packet.go:94-138` | `Parse` frontmatter switch lacks cases for `sdd_phase`, `fanout_group`, and `skill` |
| `internal/packet/packet_test.go:15-48` | `TestParseSeparatesFrontmatterFromBody` verifies basic frontmatter separation |
| `internal/packet/packet_test.go:50-90` | `TestParseModelPresentIsParsed` and empty default tests |
| `internal/run/batch.go:167-217` | `ensureLaneFailed` registers lane and sets failed without `UpdateLaneMetadata` |
| `internal/run/run.go:334-358` | `Execute` registers lane and sets running without calling `UpdateLaneMetadata` |
| `internal/run/run_test.go:79-93` | `testPacket` helper definition for execution testing |
| `internal/serve/handlers.go:316-350` | Two-segment URL path parsing pattern for `/approvals/{runID}/{laneID}` |
| `internal/serve/handlers.go:352-354` | Mux fallback returning 404 JSON for unmatched `/api/` paths |
| `internal/serve/model.go:163-184` | `Lane` JSON model struct definition lacking `Skill` and `PacketPath` |
| `internal/serve/model.go:322-333` | `laneDTO` constructor mapping `ledger.LaneMetadata` to `serve.Lane` |
| `internal/serve/model_test.go:599-669` | `TestModelRunLaneAndProgressJSONContract` asserts `Lane` JSON serialization |
| `internal/serve/server_test.go:47-100` | `httptest` recorder and server route testing pattern |
| `internal/serve/static/app.js:534-536` | `normalizeFleetState` maps `sdd_phase`, `fanout_group`, and `feature` but lacks `skill` |
| `internal/serve/static/app.js:575-593` | Fleet field definitions rendered in fleet cards |
| `internal/serve/static_test.go:286-300` | Asset contract assertions for fleet field normalization in `app.js` |
| `openspec/changes/lane-status-observability/design.md:17-23` | Decision 1 resolves frontmatter keys `sdd_phase`, `fanout_group`, `skill` |
| `openspec/changes/lane-status-observability/design.md:24-30` | Decision 2 places packet path in `LaneMetadata` JSON snapshot |
| `openspec/changes/lane-status-observability/design.md:31-37` | Decision 3 defines `GET /api/packets/{runID}/{laneID}` endpoint behavior |
| `openspec/changes/lane-status-observability/design.md:38-44` | Decision 4 defers DAG wave metadata and defaults omitted keys |
| `openspec/changes/lane-status-observability/design.md:179-189` | Threat matrix shows Process Integration owned by lens C and N/A for lens A |
| `openspec/changes/lane-status-observability/specs/dispatched-packet-body/spec.md:9-30` | Requirement and scenarios for dispatched packet body inspection |
| `openspec/changes/lane-status-observability/specs/lane-execution/spec.md:5-26` | Requirement and scenarios for lane metadata dispatch persistence |
| `openspec/changes/lane-status-observability/specs/read-only-packet-schema/spec.md:5-26` | Requirement and scenarios for extended packet frontmatter parsing |
