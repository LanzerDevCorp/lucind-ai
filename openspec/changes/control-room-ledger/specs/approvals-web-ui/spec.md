# Delta for Approvals Web UI

## ADDED Requirements

### Requirement: Shell-Free Run and Progress Model DTOs

The `serve.Model` query surface MUST provide typed, shell-free read methods for run summaries and lane progress tails backed directly by SQLite queries without executing subprocess or git commands.

#### Scenario: Typed run and progress queries

- GIVEN persisted runs and progress in SQLite
- WHEN `serve.Model` queries run summaries or progress tails
- THEN typed DTO structs return directly from SQLite without subprocess or git calls.

#### Scenario: Query unknown run returns empty DTO

- GIVEN an unknown `run_id`
- WHEN `serve.Model` queries the run summary
- THEN a not-found error or empty DTO returns without invoking any shell command.

#### Scenario: Database error returns typed error

- GIVEN a broken SQLite connection backing `serve.Model`
- WHEN a run summary query executes
- THEN `serve.Model` returns a database error without shell fallback.
