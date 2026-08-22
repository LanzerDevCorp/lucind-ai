# Spec Lens B — Scenarios & Coverage: Control Room UI Views

## Assumed requirements

This change introduces four new capabilities (`batch-wave-view`, `feature-lease-monitor`, `reconciliation-workspace`, `lane-envelope-inspector`) and modifies `approvals-web-ui` in a modular dashboard. We assume five requirements from `proposal.md`: `Anti-rubber-stamping in the multi-view shell` (`approvals-web-ui`), `Batch and DAG wave inspection` (`batch-wave-view`), `Shell-free feature and lease monitoring` (`feature-lease-monitor`), `Reconciliation candidate inspection` (`reconciliation-workspace`), and `Lane demotion diagnosis` (`lane-envelope-inspector`). Each requirement asserts observable behavior across `serve.Model` reads, HTTP routes, and UI rendering without shell commands or bulk-approval regressions.

## Scenarios

### Requirement: Anti-rubber-stamping in the multi-view shell

#### Scenario: Single approval submission with valid inline evidence

- GIVEN a pending lane approval with valid command output or `file:line` evidence
- WHEN the operator submits an approval decision for that single lane
- THEN the server returns HTTP 200 and persists the decision in the ledger
- AND the UI displays the operator's wrong-approval rate and the `opencode` command

#### Scenario: Unsupported claim withheld from evidence display

- GIVEN an approval item whose evidence is neither command output nor a `file:line` reference
- WHEN the approvals panel renders the card
- THEN the card displays fallback text `(no command output or file:line evidence provided)` without bare prose

#### Scenario: Bulk approval payload rejected

- GIVEN multiple pending lane approvals in the ledger
- WHEN a client posts an array or multi-item payload to `/approvals/`
- THEN the server returns HTTP 400 Bad Request and records no decisions in the ledger

### Requirement: Batch and DAG wave inspection

#### Scenario: Wave grouping and lane lifecycle inspection

- GIVEN an active batch execution with multi-wave DAG packet dependencies
- WHEN the operator inspects the batch-wave view
- THEN each lane displays status, assigned executor, worktree path, DAG wave group, and deadline

#### Scenario: Barrier release with mixed terminal statuses

- GIVEN an evaluated batch with one `done` lane and one `failed` or `deviated` lane
- WHEN barrier evaluation completes
- THEN the UI displays Released status, marks the `done` lane for integration, and preserves non-done worktrees

#### Scenario: Cyclic DAG dependency halts wave scheduling

- GIVEN a batch definition containing cyclical packet dependencies
- WHEN DAG wave decomposition executes
- THEN wave decomposition returns `ErrCycleDetected` and the UI presents the cycle error without releasing lanes

### Requirement: Shell-free feature and lease monitoring

#### Scenario: Active lease and attempt inspection via model

- GIVEN a feature with an active lease, fence counter, and integration attempts in the ledger
- WHEN the operator opens the feature-lease monitor
- THEN the UI displays feature status, lease owner, fence counter, attempt status, and candidate SHA via `serve.Model`

#### Scenario: On-demand overlap evidence display

- GIVEN a feature with classified overlap evidence rows in the ledger
- WHEN the operator expands the overlap evidence panel
- THEN the UI fetches and renders the evidence class, hash, and escaped JSON payload without polling during hot refresh

#### Scenario: Expired lease queried without shell subprocess

- GIVEN a feature whose lease `expires_at` timestamp is in the past
- WHEN `serve.Model` executes `ListLeases`
- THEN the response returns the recorded expiration timestamp and owner without spawning git or shell subprocesses

### Requirement: Reconciliation candidate inspection

#### Scenario: Read-only reconciliation request and check inspection

- GIVEN a reconciliation request with candidate diffs, check outcomes, and CAS status in the ledger
- WHEN the operator opens the reconciliation workspace
- THEN the UI renders request direction, allowed paths, model, candidate SHA, checks output, CAS result, and audit log

#### Scenario: Failed CAS promotion displays failure reason

- GIVEN a reconciliation request where compare-and-swap failed due to SHA mismatch
- WHEN the operator inspects the candidate
- THEN the UI displays CAS outcome `failed` alongside the recorded failure reason and candidate output

#### Scenario: Non-existent reconciliation request returns 404

- GIVEN a request querying an unknown reconciliation request ID
- WHEN the HTTP client requests `/api/reconcile/requests/{id}`
- THEN the server returns HTTP 404 Not Found without modifying ledger reconciliation state

### Requirement: Lane demotion diagnosis

#### Scenario: Deviated lane displays offending paths and preserved worktree

- GIVEN a lane execution demoted from Done to Deviated due to modifying paths outside `allowed_paths`
- WHEN the operator inspects the lane envelope view
- THEN the UI displays status `deviated`, offending paths from the `lane_note` event, and the preserved worktree path

#### Scenario: Multiple out-of-scope paths formatted in diagnosis note

- GIVEN a lane modifying multiple files outside declared `allowed_paths`
- WHEN `serve.Model` reads the demotion diagnosis
- THEN the inspector displays the full comma-separated list of offending paths captured in the `lane_note`

#### Scenario: Preserved worktree inspected without direct disk read

- GIVEN a demoted lane with a preserved worktree on disk
- WHEN `serve.Model` processes the lane envelope inspection query
- THEN `serve.Model` retrieves diagnosis data exclusively from the SQLite ledger without reading worktree files via `os`

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Anti-rubber-stamping in the multi-view shell | covered | covered | covered | `internal/serve/server_test.go:42-93`, `internal/serve/static_test.go:11-39` |
| Batch and DAG wave inspection | covered | covered | covered | `internal/dag/waves_test.go:41-70`, `internal/barrier/barrier_test.go:21-60` |
| Shell-free feature and lease monitoring | covered | covered | covered | `internal/serve/model_test.go:74-347`, `internal/serve/model_test.go:595-627` |
| Reconciliation candidate inspection | covered | covered | covered | `internal/serve/model_test.go:278-343`, `internal/serve/server_test.go:196-236` |
| Lane demotion diagnosis | covered | covered | covered | `internal/run/run_test.go:643-653`, `internal/serve/model_test.go:610-618` |

## Untestable Assertions

None

## Open Questions

- [ ] Whether reconciliation actions (`approve`, `decline`, `cancel`, `renew`, `resolve` matching `cmd/lucind-ai/cli.go:1044-1065`) will be supported via HTTP POST endpoints or restricted to copy-paste CLI command rendering (`internal/serve/handlers.go:36-118`).
- [ ] Whether overlap `evidence_json` (`internal/serve/model.go:68`) should render as escaped `<pre>` blocks or via a zero-dependency client-side diff tokenizer (`internal/serve/static/app.js:51-55, 91-94`).
- [ ] Whether countdowns should be computed as server-side `remaining_seconds` on `serve.Model` or derived client-side from `expires_at` plus server timestamp (`internal/serve/model.go:56, 84, 354-357`).
