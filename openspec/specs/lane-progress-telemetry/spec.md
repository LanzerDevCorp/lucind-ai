# Lane Progress Telemetry Specification

## Purpose

Carry optional numeric token, cost, and tool-call metrics on progress events so the dashboard can render usage instead of discarding it as prose.

## Requirements

### Requirement: Structured progress telemetry

Progress events and persisted lane progress MUST support optional numeric fields for total tokens, USD cost, and generic tool-call counts. Stream decoders for agy, claude, and opencode MUST populate those fields when usage or tool events are present, alongside human-readable messages. The cursor-agent decoder MUST report zeros for numeric token and cost fields rather than omitting them or failing decode. As part of the same schema v7 migration described under `orphan-lane-reconciliation`, the ledger MUST persist these numeric values. The server JSON API and live progress broadcast MUST emit field names matching total tokens, USD cost, and tool metrics consumed by the dashboard.

#### Scenario: Decoders populate usage

- GIVEN an agy, claude, or opencode dispatch emitting token usage, cost, or tool events
- WHEN the stream decoder processes the event stream
- THEN it MUST emit progress containing populated numeric token, cost, and tool-call fields alongside message prose, and persisted lane progress MUST retain those numeric fields

#### Scenario: Cursor-agent emits zeroed metrics

- GIVEN a cursor-agent dispatch emitting tool-call progress events
- WHEN the cursor-agent stream decoder processes the event stream
- THEN emitted progress and persisted lane progress MUST report zero values for numeric token and cost fields without decode errors

#### Scenario: Real-time telemetry broadcast

- GIVEN an active lane appending progress events with structured metrics
- WHEN the server processes that progress for connected web clients
- THEN it MUST broadcast the progress including total tokens, USD cost, and tool counts
