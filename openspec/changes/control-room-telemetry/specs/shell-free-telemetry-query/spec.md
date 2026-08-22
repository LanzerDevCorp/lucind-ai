# Delta for Shell-Free Telemetry Query

## ADDED Requirements

### Requirement: Shell-Free Run Lifecycle Query

The serve model MUST query run lifecycle events, lane execution outcomes, execution durations, and terminal statuses from the SQLite ledger without invoking `os/exec`, git subprocesses, or shell commands. Telemetry streaming MUST NOT add a seventh `lane.Status` value and MUST NOT delay batch barrier release beyond a bounded in-memory stream flush.

#### Scenario: Telemetry event history queried without shell out

- GIVEN completed lane lifecycle events recorded in SQLite
- WHEN the serve model executes a telemetry query
- THEN it MUST return populated records from SQLite without invoking `os/exec` or git commands

#### Scenario: Batch barrier observes only persisted terminal status

- GIVEN parallel lanes executing with active telemetry streams
- WHEN child processes exit and bounded stream flush completes
- THEN the batch barrier MUST NOT release until every lane's terminal status is persisted in the ledger

#### Scenario: Unpersisted or non-terminal lane blocks barrier release

- GIVEN a batch with multiple lanes where one lane has finished execution but has not persisted its terminal status
- WHEN the batch barrier is evaluated
- THEN evaluation MUST report not released and MUST prevent batch integration
