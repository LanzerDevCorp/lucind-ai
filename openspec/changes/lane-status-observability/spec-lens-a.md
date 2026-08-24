# Spec Lens A — Metadata & Frontmatter: Lane Status Observability

## Assumed requirements

This lens specifies two dispatch-side requirements: **Lane metadata dispatch persistence** targeting capability `lane-execution` and **Extended packet frontmatter parsing** targeting capability `read-only-packet-schema`. Both are classified as ADDED rather than MODIFIED because they introduce new obligations and fields without changing or deprecating any existing requirement text or contract. `lane-execution` gains a post-registration metadata persistence obligation while keeping existing lifecycle states intact, and `read-only-packet-schema` gains optional frontmatter keys while preserving existing parsing rules and error cases.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `lane-execution` | existing | `openspec/changes/lane-status-observability/specs/lane-execution/spec.md` | `openspec/specs/lane-execution/spec.md:1` |
| `read-only-packet-schema` | existing | `openspec/changes/lane-status-observability/specs/read-only-packet-schema/spec.md` | `openspec/specs/read-only-packet-schema/spec.md:1` |

## ADDED Requirements

### Requirement: Lane metadata dispatch persistence

Dispatch callers `Execute` (`internal/run/run.go:334`) and `ensureLaneFailed` (`internal/run/batch.go:184`) MUST invoke `UpdateLaneMetadata` (`internal/ledger/lanes_meta.go:39`) immediately after `RegisterLane` succeeds, persisting packet and routing metadata to the ledger. Historical ledger rows predating this requirement SHALL NOT be backfilled.

**Terminal consumer**: `internal/run/run.go:334` (`Execute`), `internal/run/batch.go:184` (`ensureLaneFailed`), and `internal/ledger/lanes_meta.go:39` (`UpdateLaneMetadata`), consumed downstream by `internal/serve/model.go:288-301` (`ListLanes` via `GetLaneMetadata` at `internal/ledger/lanes_meta.go:89`).

#### Scenario: Dispatch persists metadata

- GIVEN a packet with model, agent, feature, and SDD attributes dispatched via `run.Execute`
- WHEN `RegisterLane` succeeds
- THEN `UpdateLaneMetadata` MUST persist the metadata snapshot to the ledger and `serve.ListLanes` MUST return populated metadata fields rather than "Unavailable"

#### Scenario: Historical rows preserved

- GIVEN pre-existing lane records without an audited `lane_metadata:v1:` event
- WHEN `serve.ListLanes` queries the ledger
- THEN `GetLaneMetadata` MUST return the recorded schema-v6 columns with empty values for unrecorded extended fields without error

#### Scenario: Pre-dispatch failure persists metadata

- GIVEN a lane in a batch that fails prior to executor execution handled by `ensureLaneFailed`
- WHEN `RegisterLane` registers the failed lane row
- THEN `ensureLaneFailed` MUST call `UpdateLaneMetadata` so the failed lane record retains its packet and routing metadata

### Requirement: Extended packet frontmatter parsing

`packet.Parse` (`internal/packet/packet.go:78-167`) MUST parse optional `sdd_phase`, `fanout_group`, and `skill` frontmatter keys into the corresponding `Packet` struct fields (`internal/packet/packet.go:33-75`). Omitted keys MUST default to empty strings, and their absence MUST NOT cause parsing to fail. Live executor runtime skill telemetry SHALL NOT be decoded.

**Terminal consumer**: `internal/packet/packet.go:78-167` (`packet.Parse` switch at `packet.go:94-138` populating `Packet` at `packet.go:33-75`), consumed by `internal/run/run.go:334` and `internal/run/batch.go:184` to construct `LaneMetadata` for `internal/ledger/lanes_meta.go:39`.

#### Scenario: Parse frontmatter keys

- GIVEN a packet markdown document containing `sdd_phase: propose`, `fanout_group: lens-a`, and `skill: sdd-propose`
- WHEN `packet.Parse` parses the document
- THEN the returned `Packet` struct MUST have `SDDPhase == "propose"`, `FanoutGroup == "lens-a"`, and `Skill == "sdd-propose"`

#### Scenario: Optional keys omitted

- GIVEN a packet markdown document omitting `sdd_phase`, `fanout_group`, and `skill` keys
- WHEN `packet.Parse` parses the document
- THEN parsing MUST succeed with empty string values for `SDDPhase`, `FanoutGroup`, and `Skill`

#### Scenario: Empty frontmatter values handled

- GIVEN a packet document containing explicit but empty values such as `sdd_phase:`
- WHEN `packet.Parse` parses the document
- THEN parsing MUST succeed and assign an empty string to the corresponding struct field

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Lane metadata dispatch persistence | Scenario: Dispatch persists metadata | Scenario: Historical rows preserved | Scenario: Pre-dispatch failure persists metadata | `internal/ledger/lanes_meta_test.go:15-70` (ledger CRUD), `internal/serve/model_test.go:599-670` (serve read), new seam required in `internal/run/run_test.go` and `internal/run/batch_test.go` (dispatch wiring) |
| Extended packet frontmatter parsing | Scenario: Parse frontmatter keys | Scenario: Optional keys omitted | Scenario: Empty frontmatter values handled | `internal/packet/packet_test.go:15-67` (existing parser tests), new seam required in `internal/packet/packet_test.go` (extended key test cases) |

## Open Questions

- [ ] Exact frontmatter key names (`sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs `generated_by`) remain open per Proposal Open Question 1; names used in this spec are working placeholders pending design resolution.
- [ ] Packet path persistence mechanism (`LaneMetadata.PacketPath` field vs `lanes` column) is tracked in Proposal Open Question 2 and owned by sibling lenses.
- [ ] SDD phase process precedence: this draft follows packet instructions for three-lens capability-domain decomposition; full spec generation under `specs/` is deferred to the synthesizer.

## Citation Manifest

| citation | claim |
|---|---|
| `internal/ledger/lanes_meta.go:12` | `laneMetadataAuditPrefix` defines the event detail prefix `lane_metadata:v1:` |
| `internal/ledger/lanes_meta.go:20-32` | `LaneMetadata` struct definition carrying packet and SDD dispatch metadata |
| `internal/ledger/lanes_meta.go:39` | `UpdateLaneMetadata` updates schema-v6 columns and appends metadata snapshot event in one transaction |
| `internal/ledger/lanes_meta.go:67-77` | `UpdateLaneMetadata` inserts `EventLaneNote` audit event with JSON snapshot |
| `internal/ledger/lanes_meta.go:89` | `GetLaneMetadata` queries schema-v6 columns and latest audit snapshot |
| `internal/ledger/lanes_meta_test.go:15-70` | `TestUpdateAndGetLaneMetadata` verifies metadata persistence and audit log round-trip |
| `internal/packet/packet.go:33-75` | `Packet` struct fields representing parsed packet frontmatter and body |
| `internal/packet/packet.go:78-167` | `packet.Parse` scans frontmatter delimiters, decodes keys, and validates required fields |
| `internal/packet/packet.go:94-138` | `packet.Parse` key-switch mapping frontmatter keys to `Packet` fields |
| `internal/packet/packet_test.go:15-67` | Unit tests for `packet.Parse` verifying delimiter splitting, field extraction, and absent key handling |
| `internal/run/batch.go:184` | `ensureLaneFailed` registers unstarted failed lanes without currently calling `UpdateLaneMetadata` |
| `internal/run/run.go:334` | `Execute` registers new lanes via `RegisterLane` without currently calling `UpdateLaneMetadata` |
| `internal/serve/model.go:163-184` | `serve.Lane` DTO includes metadata fields (`Model`, `Agent`, `Feature`, `SDDPhase`, `FanoutGroup`, etc.) |
| `internal/serve/model.go:288-301` | `ListLanes` queries ledger rows and enriches each with `GetLaneMetadata` |
| `internal/serve/model_test.go:599-670` | `TestModelRunLaneAndProgressJSONContract` verifies serve-layer JSON contracts for lanes with metadata |
| `openspec/specs/lane-execution/spec.md:1` | Live capability specification for `lane-execution` exists |
| `openspec/specs/read-only-packet-schema/spec.md:1` | Live capability specification for `read-only-packet-schema` exists |
