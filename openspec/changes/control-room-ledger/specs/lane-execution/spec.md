# Delta for Lane Execution

## ADDED Requirements

### Requirement: Lane Dispatch Metadata Persistence

Schema v6 `lanes` table MUST include nullable `model`, `agent`, and `feature` columns, and `RegisterLane` MUST persist these metadata attributes when present on `packet.Packet`. Transactional migration to schema v6 MUST preserve existing lane records with null or empty metadata values.

#### Scenario: Persist metadata on lane registration

- GIVEN a packet declaring `model`, `agent`, and `feature`
- WHEN `RegisterLane` executes
- THEN the lane row persists those column values.

#### Scenario: Schema v6 migration preserves existing lanes

- GIVEN a schema v5 database with existing lanes
- WHEN transactional `migrate` runs to v6
- THEN existing lanes are preserved with null or empty metadata.

### Requirement: Admitted Run Status Event Types

Schema v6 `events` table CHECK constraint MUST admit `run_status_changed` alongside existing event types (`run_started`, `lane_registered`, `lane_status_changed`, `lane_note`, `barrier_released`, `run_ended`).

#### Scenario: Unadmitted event type rejected

- GIVEN a schema v6 database
- WHEN appending an event with an unadmitted type
- THEN the insert fails with a CHECK constraint error.
