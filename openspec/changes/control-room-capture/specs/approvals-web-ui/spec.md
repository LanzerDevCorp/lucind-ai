# Delta for approvals-web-ui

## ADDED Requirements

### Requirement: Loopback HTTP stream access

`lucind-ai serve` MUST expose read-only loopback endpoints for live SSE log tailing and finished transcript download. Client disconnects or HTTP backpressure MUST NOT stall, leak, or backpressure running child executor processes.

#### Scenario: Live SSE tail of in-flight lane

- GIVEN an active running lane dispatch
- WHEN an HTTP client requests the live log stream on the loopback server
- THEN the server MUST stream file appends via SSE without blocking child execution

#### Scenario: Post-mortem download of finished lane transcript

- GIVEN a finished lane with a persisted log file on primary root
- WHEN an HTTP client requests the lane transcript
- THEN the server MUST return HTTP 200 with the full stored log content

#### Scenario: Client disconnect during live tail

- GIVEN an active SSE live tail connection
- WHEN the HTTP client disconnects
- THEN the handler MUST end the tail without leaking resources or stalling the child

#### Scenario: Log request for non-existent lane returns 404

- GIVEN a request for a lane ID with no corresponding log file
- WHEN the client requests the log endpoint
- THEN the server MUST return HTTP 404
