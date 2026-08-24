# Orphan Lane Reconciliation Specification

## Purpose

Record the runner process on each run and sweep `running` lanes whose driver is gone to `failed`, so the dashboard does not show a live lane for a dead process.

## Requirements

### Requirement: Orphaned lane reconciliation

Run registration MUST record the runner process identifier. Schema v7 MUST add that identifier on recorded runs and numeric usage columns on lane progress as one STRICT rebuild (telemetry columns are consumed under `lane-progress-telemetry`). The server MUST run an orphan sweep at startup and on a periodic ticker (interval and liveness mechanism remain open design questions). When the recorded process is no longer alive, the sweep MUST move associated `running` lanes to `failed` and MUST append a lane note that the driving process terminated. When the recorded process is alive, the sweep MUST leave `running` lanes unchanged.

#### Scenario: Dead-process lane swept to failed

- GIVEN a run whose recorded process has terminated while a lane remains `running`
- WHEN the server executes an orphan reconciliation sweep
- THEN the lane MUST become `failed` and MUST record an explanatory lane note

#### Scenario: Active process lanes unchanged

- GIVEN a run whose recorded process is alive
- WHEN the server executes an orphan reconciliation sweep
- THEN `running` lanes MUST remain untouched
