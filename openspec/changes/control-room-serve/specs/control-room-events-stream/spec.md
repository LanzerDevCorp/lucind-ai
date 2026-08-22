# Delta for control-room-events-stream

## ADDED Requirements

### Requirement: Real-Time Event Streaming via SSE

The server MUST stream ledger `events` and `integration_events` as Server-Sent Events with `Content-Type: text/event-stream` at `GET /api/v1/events/stream` using `http.Flusher`, and MUST terminate the push loop immediately when the client disconnects or request context is cancelled.

#### Scenario: Live event stream flushes rows

- GIVEN `serve` running and events being appended to the ledger
- WHEN `GET /api/v1/events/stream` with `Accept: text/event-stream`
- THEN stream event rows using `text/event-stream` and flush frames as appended

#### Scenario: Client disconnect terminates loop

- GIVEN an active SSE client stream
- WHEN the client disconnects closing request context
- THEN terminate the push loop with no leaked goroutines
