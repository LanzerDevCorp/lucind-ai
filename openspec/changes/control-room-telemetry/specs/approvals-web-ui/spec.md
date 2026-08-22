# Delta for Approvals Web UI

## ADDED Requirements

### Requirement: Loopback Server-Sent Events Telemetry Stream

`lucind-ai serve` MUST expose a loopback-only Server-Sent Events endpoint `/api/telemetry/events` on the existing serve mux to stream live lane stdout/stderr chunks to subscribed clients. The endpoint MUST enforce loopback binding, MUST unregister subscribers on client disconnect, and MUST NOT bypass or alter individual per-item approval decisions.

#### Scenario: Loopback client receives live event stream

- GIVEN `lucind-ai serve` running on `127.0.0.1:7433`
- WHEN a loopback HTTP client sends a `GET` request to `/api/telemetry/events`
- THEN the server MUST return HTTP 200 with `Content-Type: text/event-stream` and flush lane events as they occur

#### Scenario: Client disconnect cleans up subscription

- GIVEN an active loopback SSE subscriber connection receiving live stream events
- WHEN the client closes the connection or the request context cancels
- THEN the hub MUST unregister the subscriber and stop dispatching events without error or leaked work

#### Scenario: Non-loopback address rejected

- GIVEN a non-loopback bind address `0.0.0.0:7433`
- WHEN serve listen is attempted with this address
- THEN the server MUST reject binding, return a non-loopback error, and exit with an error
