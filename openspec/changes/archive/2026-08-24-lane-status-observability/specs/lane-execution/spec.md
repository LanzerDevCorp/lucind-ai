# Delta for Lane Execution

## ADDED Requirements

### Requirement: Lane metadata dispatch persistence

After a lane is registered, dispatch MUST persist packet and routing metadata to the ledger so dashboard consumers can return populated fields. Historical rows from before this requirement MUST NOT be backfilled.

#### Scenario: Dispatch persists metadata

- GIVEN a packet with model, agent, feature, and SDD attributes dispatched through lane execution
- WHEN lane registration succeeds
- THEN the ledger MUST retain the metadata snapshot and listing lanes MUST return populated metadata fields rather than an unavailable placeholder

#### Scenario: Historical rows preserved

- GIVEN pre-existing lane records without an audited metadata snapshot
- WHEN listing lanes queries the ledger
- THEN the query MUST return the recorded schema-v6 columns with empty values for unrecorded extended fields without error

#### Scenario: Pre-dispatch failure persists metadata

- GIVEN a batch lane that fails before executor execution
- WHEN the failed lane is registered
- THEN dispatch MUST persist packet and routing metadata on that failed lane record
