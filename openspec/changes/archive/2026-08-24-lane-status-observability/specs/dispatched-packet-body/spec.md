# Dispatched Packet Body Specification

## Purpose

Let operators inspect the exact markdown that was dispatched for a run and lane.

## Requirements

### Requirement: Dispatched packet body inspection

The CLI MUST preserve the association between each dispatched lane and its on-disk packet source (persistence mechanism remains an open design question). The HTTP server MUST expose a GET endpoint that returns the verbatim, unparsed markdown of that dispatched packet for a requested run and lane. When the run, lane, or packet association does not exist, or the mapped file cannot be read, the server MUST respond with HTTP 404 and MUST NOT terminate the serve process.

#### Scenario: Retrieve packet content

- GIVEN a valid run and lane dispatched from an on-disk packet file
- WHEN a client requests that lane's packet body
- THEN the server MUST return HTTP 200 with the exact, unparsed file content

#### Scenario: Unknown lane returns 404

- GIVEN a request specifying a non-existent run or lane
- WHEN a client requests the packet body
- THEN the server MUST return HTTP 404

#### Scenario: Missing or unreadable packet file on disk

- GIVEN a valid lane whose recorded packet path points to a deleted or unreadable file
- WHEN a client requests the packet body
- THEN the server MUST return HTTP 404 and MUST NOT crash the server process
