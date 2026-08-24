# Spec Lens B — Packet Body & Telemetry: Lane Status Observability

## Assumed requirements

This lens specifies the behavioral contracts for two new observability capabilities: **Dispatched packet body inspection** (`dispatched-packet-body`) and **Structured progress telemetry** (`lane-progress-telemetry`). Both capabilities represent net-new system behaviors that currently lack existing specifications in `openspec/specs/`. Consequently, both requirements are authored as complete, full specifications rather than delta modifications against pre-existing capability specs.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `dispatched-packet-body` | new | `openspec/specs/dispatched-packet-body/spec.md` | |
| `lane-progress-telemetry` | new | `openspec/specs/lane-progress-telemetry/spec.md` | |

## Requirements

### Requirement: Dispatched packet body inspection

The CLI MUST preserve the association between each dispatched lane and its on-disk packet source path (`cmd/lucind-ai/cli.go:160-174`). The HTTP server MUST expose a dedicated GET route registered on its request multiplexer (`internal/serve/handlers.go:190`) that returns the verbatim, unparsed markdown body of the dispatched packet corresponding to a requested run ID and lane ID. When a requested run, lane, or packet association does not exist, the server MUST respond with HTTP 404 Not Found. If the mapped packet file on disk cannot be read, the server MUST respond with an error status rather than failing silently or terminating the serve process.

**Terminal consumer**: `cmd/lucind-ai/cli.go:160-174` (packet dispatch path mapping), `internal/serve/handlers.go:190` (HTTP GET route registration), and `internal/serve/static/app.js:200-249` (dashboard lane card packet link).

#### Scenario: Retrieve packet content

- GIVEN a valid run and lane dispatched from an on-disk packet file
- WHEN a client sends an HTTP GET request to `/api/runs/{run_id}/lanes/{lane_id}/packet`
- THEN the server MUST return HTTP 200 with `Content-Type: text/markdown` and the exact, unparsed file content

#### Scenario: Unknown lane returns 404

- GIVEN a request specifying a non-existent run ID or lane ID
- WHEN a client sends an HTTP GET request to the packet endpoint
- THEN the server MUST return HTTP 404 Not Found

#### Scenario: Missing or unreadable packet file on disk

- GIVEN a valid lane record whose recorded packet path points to a deleted or unreadable file
- WHEN a client sends an HTTP GET request to the packet endpoint
- THEN the server MUST return HTTP 404 Not Found and MUST NOT terminate or crash the server process

### Requirement: Structured progress telemetry

The progress event envelope (`internal/executor/executor.go:17-21`) and ledger progress model (`internal/ledger/progress.go:15-20`, `internal/serve/model.go:187-193`) MUST support optional structured numeric telemetry fields for `total_tokens`, `cost_usd`, and generic tool-call counts. Stream decoders for `agy` (`internal/executor/agy_stream.go:12-39` and `internal/executor/agy_stream.go:160-162`), `claude` (`internal/executor/claude_stream.go:17-36` and `internal/executor/claude_stream.go:212-218`), and `opencode` (`internal/executor/opencode_stream.go:100-125` and `internal/executor/opencode_stream.go:226-228`) MUST populate these numeric fields when decoding execution streams alongside human-readable message events. The `cursor-agent` decoder (`internal/executor/cursor_agent.go:1-60`, `internal/executor/cursor_agent_stream.go:1-50`) MUST populate numeric telemetry fields with zero values rather than omitting them or failing stream decoding. The ledger schema MUST persist these numeric values in strict `lane_progress` columns (`internal/ledger/schema.go:298-308`), and the server JSON API and SSE hub MUST emit field names matching `total_tokens`, `cost_usd`, and tool metrics consumed by the dashboard (`internal/serve/static/app.js:542-544`).

**Terminal consumer**: `internal/executor/executor.go:17-21` (`ProgressEvent`), `internal/ledger/progress.go:15-20` (`LaneProgress`), `internal/serve/model.go:187-193` (serve DTO), decoders `internal/executor/agy_stream.go:12-39`, `internal/executor/claude_stream.go:17-36`, `internal/executor/opencode_stream.go:100-125`, `internal/executor/cursor_agent_stream.go:1-50`, `internal/ledger/schema.go:298-308` (`lane_progress` table), and `internal/serve/static/app.js:542-544` (dashboard UI).

#### Scenario: Decoders populate usage

- GIVEN an `agy`, `claude`, or `opencode` dispatch emitting token usage, cost, or tool events
- WHEN the executor stream decoder processes the event stream
- THEN the decoder MUST emit a `ProgressEvent` containing populated numeric fields for `total_tokens`, `cost_usd`, and tool-call counts alongside the message prose, and `LaneProgress` MUST persist those numeric fields

#### Scenario: Cursor-agent emits zeroed metrics

- GIVEN a `cursor-agent` dispatch emitting tool-call progress events
- WHEN the cursor-agent stream decoder processes the event stream
- THEN emitted `ProgressEvent` records and persisted `LaneProgress` entries MUST report zero values for numeric token and cost fields without decode errors

#### Scenario: Real-time telemetry broadcast via SSE

- GIVEN an active lane appending progress events with structured metrics to the ledger
- WHEN the progress event is processed by the server SSE hub
- THEN the server MUST broadcast the progress event including `total_tokens`, `cost_usd`, and tool counts to connected web clients

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Dispatched packet body inspection | Retrieve packet content | Missing or unreadable packet file on disk | Unknown lane returns 404 | `internal/serve/handlers_test.go` (new seam required) |
| Structured progress telemetry | Decoders populate usage | Cursor-agent emits zeroed metrics | Real-time telemetry broadcast via SSE | `internal/executor/*_stream_test.go`, `internal/ledger/progress_test.go:14-49`, `internal/serve/hub_test.go` |

## Untestable Assertions

None. All THEN clauses specify observable HTTP responses, decoded in-memory structs, persisted ledger columns, or SSE stream payloads verifiable via standard unit and integration test harnesses.

## Open Questions

- [ ] Open Question 2 (Packet path persistence mechanism): Specification requires preservation of the packet-path-to-lane mapping without dictating whether it is stored via `LaneMetadata.PacketPath` (JSON event snapshot) or a dedicated `lanes` database column.
- [ ] Open Question 5 (DAG-wave packet inspection scope): Design must confirm whether DAG-wave generated packets (`internal/dag/emit.go:11-60`) receive packet-body GET routing and metadata persistence in this change or a follow-up.
- [ ] SDD Process Precedence: Three-lens parallel fan-out packet instructions take precedence over the single-agent delta spec template in `~/.claude/skills/sdd-spec/SKILL.md`.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:160-174` | CLI parses packet flags and maps packet indexes to on-disk file paths |
| `internal/dag/emit.go:11-60` | `EmitPacketContent` formats frontmatter and body for DAG-wave packet nodes |
| `internal/executor/agy_stream.go:12-39` | `agyUsage` token counts and `agyStepUpdate` stream event definitions |
| `internal/executor/agy_stream.go:160-162` | `formatAgyUsage` formats token counts into progress message string |
| `internal/executor/claude_stream.go:17-36` | `claudeUsage` struct and `claudeStreamRecord` with `total_cost_usd` |
| `internal/executor/claude_stream.go:212-218` | `formatClaudeUsage` formats token counts and USD cost into string |
| `internal/executor/cursor_agent.go:1-60` | `CursorAgent` executor definition without structured usage metrics |
| `internal/executor/cursor_agent_stream.go:1-50` | `cursor-agent` stream decoder parses tool calls without usage fields |
| `internal/executor/executor.go:17-21` | `ProgressEvent` struct with `Message` and `At` timestamp |
| `internal/executor/opencode_stream.go:100-125` | `opencodeTokens` and `opencodeStreamPart` structs carrying tokens and cost |
| `internal/executor/opencode_stream.go:226-228` | `opencodeStreamDecoder` formats usage into `ProgressEvent.Message` |
| `internal/ledger/progress.go:15-20` | `LaneProgress` ledger struct definition |
| `internal/ledger/progress_test.go:14-49` | Existing ledger progress append and query tests |
| `internal/ledger/schema.go:298-308` | Pre-v7 `CREATE TABLE lane_progress` DDL without numeric usage columns |
| `internal/serve/handlers.go:190` | `NewHandlerWithConfig` HTTP serve mux registration entrypoint |
| `internal/serve/model.go:187-193` | `serve.LaneProgress` JSON DTO structure |
| `internal/serve/static/app.js:200-249` | Dashboard UI lane card header rendering packet identifier |
| `internal/serve/static/app.js:542-544` | Dashboard UI telemetry normalization expecting `total_tokens`, `cost_usd`, and `tool_rate` |
