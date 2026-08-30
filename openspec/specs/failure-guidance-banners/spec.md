# Failure Guidance Banners Specification

## Purpose

Define user-facing diagnostic and recovery guidance banners across CLI execution, integration, acceptance, and DAG split workflows.

## Requirements

### Requirement: Blocked and timeout lane report guidance banner

Upon reporting a lane terminating with any status other than `lane.Done` (including `blocked`, `failed`, or dispatch timeout), `printReport` MUST print a visual warning banner displaying the preserved worktree path, porcelain status diagnostic inspection steps, and an explicit reference to `troubleshooting.md`. The guidance banner SHALL NOT be printed for lanes completing with `lane.Done`.

#### Scenario: Non-done lane displays troubleshooting banner
- GIVEN a lane ending with status `blocked`, `failed`, or timeout
- WHEN `printReport` formats output
- THEN output MUST include a banner with worktree path, diff inspection steps (`git diff`), and reference to `troubleshooting.md`

#### Scenario: Done lane omits troubleshooting banner
- GIVEN a lane completing with `lane.Done`
- WHEN `printReport` renders output
- THEN output MUST NOT display the non-done warning banner or `troubleshooting.md` reference

### Requirement: Integration report reverted IDs recovery banner

When an integration batch finishes with one or more reverted lanes (`reverted_ids` non-empty), `printIntegrateReport` MUST print recovery guidance instructing operators to run `lucind-ai integrate retry --run <run-id>` and referencing the recovery protocol in `recovery-reconciliation.md`. When `reverted_ids` is empty, the retry guidance banner SHALL NOT be printed.

#### Scenario: Reverted integration outcome surfaces retry instructions
- GIVEN an integration batch with non-empty `reverted_ids`
- WHEN `printIntegrateReport` renders summary
- THEN output MUST list `reverted_ids` and append retry instructions with `lucind-ai integrate retry --run <run-id>` referencing `recovery-reconciliation.md`

#### Scenario: Fully integrated batch omits recovery banner
- GIVEN an integration batch where all lanes integrated and `reverted_ids` is empty
- WHEN `printIntegrateReport` formats output
- THEN `reverted_ids:` MUST be printed explicitly empty (`reverted_ids:\n`) without the retry guidance banner

### Requirement: Acceptance receipt qualitative review banner

Upon completing mechanical verification and rendering an acceptance receipt, `renderAcceptanceReceipt` MUST print an explicit reminder banner prompting operators to perform qualitative review checklist steps 2–10 defined in `acceptance-promotion.md`. The output MUST reaffirm that the receipt constitutes mechanical evidence only and does not imply qualitative approval. When mechanical verification fails, the command MUST exit non-zero without rendering an acceptance receipt or qualitative review reminder.

#### Scenario: Mechanical acceptance output prints qualitative checklist reminder
- GIVEN a candidate commit passing mechanical checks
- WHEN `renderAcceptanceReceipt` renders output
- THEN output MUST state mechanical evidence passed and append a reminder to complete qualitative checklist steps 2–10 from `acceptance-promotion.md`

#### Scenario: Failing mechanical checks exit non-zero without receipt
- GIVEN a candidate commit failing mechanical checks
- WHEN `lucind-ai accept` executes
- THEN the command MUST exit non-zero and MUST NOT output an acceptance receipt or qualitative checklist reminder

### Requirement: DAG split multi-wave base SHA warning banner

When `lucind-ai split` processes an apply DAG with two or more execution waves, it MUST output a warning banner instructing operators to advance the primary checkout and refresh `base_sha` and `expected_parent_sha` in next-wave packets between wave dispatches per `recovery-reconciliation.md`. The warning banner MUST be routed to stderr to preserve pipeline-parseable wave commands on stdout. When splitting a single-wave DAG, the multi-wave warning banner SHALL NOT be printed.

#### Scenario: Multi-wave DAG split emits base SHA warning
- GIVEN an `apply-dag.yaml` defining two or more sequential waves
- WHEN `runSplit` executes `dag.Split`
- THEN output MUST append a warning banner to advance checkout and refresh `base_sha`/`expected_parent_sha` per `recovery-reconciliation.md`

#### Scenario: Single-wave DAG split omits base SHA warning
- GIVEN an `apply-dag.yaml` defining a single wave
- WHEN `runSplit` executes
- THEN output MUST emit the single `lucind-ai run` command without the multi-wave warning banner
