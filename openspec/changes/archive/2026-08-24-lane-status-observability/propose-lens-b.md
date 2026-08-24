# Proposal Lens B — Capability Impact & Specs: Lane Status Observability

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `lane-execution` | Modified | Dispatch callers invoke `UpdateLaneMetadata` after `RegisterLane` to persist dispatch attributes (`model`, `agent`, `feature`, `sdd_phase`, `fanout_group`, `change`, `skill`), eliminating "Unavailable" UI values without historical backfill. | `internal/run/run.go:334-355`, `internal/run/batch.go:184-200`, `internal/ledger/lanes_meta.go:20-83` |
| `read-only-packet-schema` | Modified | Extend parser to decode optional `sdd_phase`, `fanout_group`, and static `skill` frontmatter keys into `Packet`, defaulting omitted keys to empty strings. | `internal/packet/packet.go:33-75`, `internal/packet/packet.go:78-167`, `cmd/lucind-ai/cli.go:160-174` |
| `lane-envelope-inspector` | Modified | Serve raw dispatched packet markdown by run and lane ID via a new HTTP endpoint mapped from CLI arguments. | `cmd/lucind-ai/cli.go:141-174`, `internal/serve/model.go:163-184`, `internal/serve/handlers.go:33-60` |
| `lane-progress-telemetry` | Added | Stream numeric tokens, USD cost, and generic tool activity in `ProgressEvent`/`LaneProgress` for `agy`, `claude`, and `opencode` (zeroed for `cursor-agent`), backed by a v7 schema migration. | `internal/executor/executor.go:17-21`, `internal/executor/agy_stream.go:12-39`, `internal/executor/claude_stream.go:17-36`, `internal/executor/opencode_stream.go:100-125`, `internal/executor/cursor_agent.go:35-60`, `internal/ledger/progress.go:14-45`, `internal/ledger/schema.go:298-308` |
| `orphan-lane-reconciliation` | Added | Track runner PID in `runs` (v7 migration) and sweep dead-process `running` lanes to `failed` with diagnostic ledger notes on startup and periodic ticker. | `internal/ledger/runs.go:16-41`, `internal/ledger/schema.go:298-308`, `internal/serve/server.go:1-60`, `internal/serve/handlers.go:33-60` |
| `batch-wave-view` | Modified | Render live metadata, packet links, structured usage, and swept orphan failure states in dashboard lane cards. | `internal/serve/model.go:163-184`, `internal/serve/static/app.js:525-545` |

## Delta Specifications

### Requirement: Lane Metadata Dispatch Persistence

`Execute` and `ensureLaneFailed` MUST invoke `ledger.UpdateLaneMetadata` following `ledger.RegisterLane` to persist `Model`, `Agent`, `Feature`, `SDDPhase`, `FanoutGroup`, `Change`, `AllowedPaths`, `Dependencies`, and `BodyDigest` (`internal/run/run.go:334-355`, `internal/run/batch.go:184-200`, `internal/ledger/lanes_meta.go:20-83`). Historical ledger rows SHALL NOT be modified or backfilled.

#### Scenario: Dispatch persists metadata
- GIVEN a packet with model, agent, and feature fields dispatched via `run.Execute`
- WHEN `RegisterLane` succeeds
- THEN `UpdateLaneMetadata` MUST persist dispatch attributes and `serve.Lane` MUST return them rather than "Unavailable"

#### Scenario: Historical rows preserved
- GIVEN pre-existing lane rows in the ledger
- WHEN `serve.ListLanes` queries the ledger
- THEN legacy rows MUST be returned as recorded without backfill

### Requirement: Extended Packet Frontmatter Parsing

`packet.Parse` MUST parse optional `sdd_phase`, `fanout_group`, and `skill` YAML keys into `packet.Packet` (`internal/packet/packet.go:33-75`, `internal/packet/packet.go:78-167`). Missing keys MUST default to empty strings. Parsing MUST NOT fail when optional keys are absent. Live executor skill telemetry SHALL NOT be decoded.

#### Scenario: Parse frontmatter keys
- GIVEN a packet with `sdd_phase: propose`, `fanout_group: lens-b`, and `skill: sdd-propose`
- WHEN `packet.Parse` parses the document
- THEN `Packet.SDDPhase`, `Packet.FanoutGroup`, and `Packet.Skill` MUST match the declared values

#### Scenario: Optional keys omitted
- GIVEN a packet omitting `sdd_phase`, `fanout_group`, and `skill`
- WHEN `packet.Parse` parses the document
- THEN parsing MUST succeed and the new fields MUST be empty strings

### Requirement: Dispatched Packet Body Inspection

The CLI dispatch path MUST preserve the mapping of packet file paths to lanes (`cmd/lucind-ai/cli.go:160-174`). `internal/serve` MUST expose an HTTP route returning the verbatim Markdown packet content for a given run and lane ID (`internal/serve/handlers.go:33-60`, `internal/serve/model.go:163-184`).

#### Scenario: Retrieve packet content
- GIVEN a lane dispatched from a packet file
- WHEN a client requests the lane's packet content endpoint
- THEN the server MUST return HTTP 200 with the exact Markdown body

#### Scenario: Unknown lane returns 404
- GIVEN a request for a non-existent run or lane ID
- WHEN the endpoint executes
- THEN the server MUST return HTTP 404 Not Found

### Requirement: Structured Progress Telemetry Streaming

`executor.ProgressEvent` (`internal/executor/executor.go:17-21`) and `ledger.LaneProgress` (`internal/ledger/progress.go:14-45`) MUST support structured fields for token metrics (`input_tokens`, `output_tokens`, `total_tokens`), `cost_usd`, and generic tool call counts. Decoders for `agy` (`internal/executor/agy_stream.go:12-39`), `claude` (`internal/executor/claude_stream.go:17-36`), and `opencode` (`internal/executor/opencode_stream.go:100-125`) MUST populate these fields alongside messages. `cursor-agent` (`internal/executor/cursor_agent.go:35-60`) MUST leave metrics at zero value. SQLite migration v7 MUST add STRICT numeric columns to `lane_progress` (`internal/ledger/schema.go:298-308`).

#### Scenario: Decoders populate usage
- GIVEN an `agy`, `claude`, or `opencode` lane emitting usage records
- WHEN stream chunks decode
- THEN `LaneProgress` MUST persist populated token, cost, and tool metrics

#### Scenario: Cursor-agent emits zeroed metrics
- GIVEN a lane executed by `cursor-agent`
- WHEN progress events are recorded
- THEN numeric metrics in `LaneProgress` MUST remain zero

### Requirement: Process Liveness and Orphaned Lane Reconciliation

`ledger.RegisterRun` MUST record the runner PID in `runs`, backed by a `pid INTEGER` column in SQLite migration v7 (`internal/ledger/runs.go:16-41`, `internal/ledger/schema.go:298-308`). `internal/serve` MUST run an orphan reconciliation sweep at startup and on a periodic ticker (`internal/serve/server.go:1-60`, `internal/serve/handlers.go:33-60`). If a run's PID is dead, any associated `running` lane MUST transition to `failed` and record an `EventLaneNote`.

#### Scenario: Dead-process lane swept to failed
- GIVEN a run whose process has terminated while a lane remains `running`
- WHEN `internal/serve` executes the orphan sweep
- THEN the lane status MUST become `failed` with an explanatory `EventLaneNote`

#### Scenario: Active process lanes unchanged
- GIVEN a run whose process is actively running
- WHEN the orphan sweep runs
- THEN `running` lanes MUST remain untouched

## Open Questions

- [ ] Exact frontmatter key names: `sdd_phase` vs `phase`, `fanout_group` vs `group`, `skill` vs `generated_by`.
- [ ] Packet path persistence: a new `LaneMetadata.PacketPath` field (audit-event JSON, no migration) vs. a real `lanes` column (migration, but queryable/indexable).
- [ ] Ticker interval duration for the periodic orphan sweep.
- [ ] PID-liveness syscall choice (`/proc/<pid>` vs `syscall.Kill(pid, 0)`) and cross-platform portability scope beyond Linux.
- [ ] Whether `internal/dag/parse.go`'s `Node` and `internal/dag/emit.go`'s `EmitPacketContent` (the DAG-wave path) get the same new fields in this change or a follow-up.
- [ ] Execution-topology precedence: Three-lane parallel fan-out and skeleton take precedence over `~/.claude/skills/sdd-propose/SKILL.md:92-158` monolithic single-agent layout.
