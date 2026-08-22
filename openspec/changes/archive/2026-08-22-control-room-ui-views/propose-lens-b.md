# Proposal Lens B — Capability Impact & Specs: Control Room UI Views

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `approvals-web-ui` | Modified | Preserves individual decisions, bulk-approval rejection (HTTP 400), inline verification evidence, and approver wrong-approval rates while integrating into a multi-panel view structure. | `internal/serve/handlers.go:36-118, 161-176`, `internal/serve/static/app.js:12-20, 22-70`, `openspec/specs/approvals-web-ui/spec.md:26-83` |
| `batch-wave-view` | Added | Visualizes concurrent batch runs, DAG wave progression, lane lifecycle states, independent per-lane deadlines, worktree paths, and barrier synchronization outcomes. | `internal/run/batch.go:19-27, 40-43`, `internal/dag/waves.go:40-70`, `internal/lane/status.go:10-28`, `internal/barrier/barrier.go:21-60`, `internal/worktree/worktree.go:168-238` |
| `feature-lease-monitor` | Added | Displays feature status, active lease ownership, monotonic fences, integration attempt CAS promotion states, and 4-way overlap evidence without shell commands. | `internal/ledger/schema.go:96-139`, `internal/serve/model.go:27-70, 128-266`, `internal/feature/feature.go:48-93, 293-360` |
| `reconciliation-workspace` | Added | Surfaces cross-feature reconciliation requests, candidate diffs bounded to 400 lines, check outcomes, and immutable audit event timelines. | `internal/ledger/schema.go:141-180`, `internal/serve/model.go:74-125, 278-343`, `cmd/lucind-ai/cli.go:1043-1065` |
| `lane-envelope-inspector` | Added | Inspects lane result envelopes (`result.json`), verifying mandatory hard stops, terminal consumer evidence, and `allowed_paths` boundary violations demoting Done to Deviated. | `internal/result/result.schema.json:1-98`, `internal/run/run.go:576-654`, `internal/run/batch.go:50-52` |

## Delta Specifications

### Requirement: Anti-Rubber-Stamping Invariants in Multi-View Shell

The UI MUST maintain strict individual item selection and decision submission on the approvals surface. The server MUST reject bulk or multi-item decision payloads with HTTP 400 Bad Request. The UI MUST render evidence inline only when it matches command output or `file:line` syntax (`internal/serve/static/app.js:12-20`), display the operator's personal wrong-approval rate (`internal/serve/handlers.go:130-140`), and display the post-merge `opencode` review command (`internal/serve/handlers.go:139`).

#### Scenario: Bulk approval payload rejected

- GIVEN multiple pending approval items in the ledger (`internal/ledger/schema.go:45-56`)
- WHEN an HTTP client sends an array or multi-item decision body to `/approvals/` (`internal/serve/handlers.go:161-176`)
- THEN the server MUST return HTTP 400 Bad Request and record no decision in the ledger.

#### Scenario: Unsupported claim withheld as evidence

- GIVEN a pending approval whose evidence string lacks command output prefix and `file:line` citation (`internal/serve/static/app.js:12-20`)
- WHEN the UI renders the approval card (`internal/serve/static/app.js:51-55`)
- THEN the UI MUST NOT display the string as valid evidence and MUST render an explicit fallback note.

### Requirement: Batch and DAG Wave Progress Inspection

The UI MUST render the active batch run status, wave progression derived from `internal/dag/waves.go:40-70`, per-lane execution status (`pending`, `running`, `done`, `blocked`, `deviated`, `failed` in `internal/lane/status.go:10-17`), assigned executor (`internal/ledger/schema.go:22`), worktree path (`internal/worktree/worktree.go:212-238`), and barrier release state (`internal/barrier/barrier.go:21-60`).

#### Scenario: Wave and lane status presentation

- GIVEN an active batch execution with concurrent lanes across DAG waves (`internal/run/batch.go:19-27`)
- WHEN the operator opens the batch inspector view
- THEN the UI MUST display each lane's current status, executor identifier, worktree path, and whether the barrier has released (`internal/barrier/barrier.go:21-29`).

#### Scenario: Barrier release with preserved non-terminal lanes

- GIVEN a batch where one lane is `done` and another lane is `failed` or `deviated` (`internal/lane/status.go:21-28`)
- WHEN the barrier evaluates terminal states (`internal/barrier/barrier.go:36-60`)
- THEN the UI MUST display the barrier as released, indicating integration eligibility for the done lane and preservation for the deviated or failed lane worktree (`internal/run/batch.go:50-52`).

### Requirement: Shell-Free Feature and Lease Monitoring

The UI MUST query feature lifecycle records and lease metadata exclusively via `serve.Model` (`internal/serve/model.go:14-24`) without executing shell or git commands. The view MUST display feature state (`internal/serve/model.go:27-35`), lease owner and monotonic fence (`internal/serve/model.go:52-59`), latest integration attempt CAS status (`internal/serve/model.go:37-50`), and overlap evidence diffs (`internal/serve/model.go:62-70`).

#### Scenario: Active lease and attempt inspection

- GIVEN a feature with an active lease and recorded integration attempts (`internal/ledger/schema.go:96-129`)
- WHEN the operator selects the feature in the UI
- THEN the UI MUST display the lease owner, fence counter, attempt status, and candidate SHA retrieved from `Model` (`internal/serve/model.go:128-227`).

### Requirement: Reconciliation Candidate Inspection

The UI MUST surface reconciliation requests, candidate resolution metadata, check outcomes, and CAS evaluation outcomes (`internal/serve/model.go:74-115, 278-323`). Candidate output diffs MUST be constrained to 400 lines per resolution rules (`internal/serve/model.go:109-110`).

#### Scenario: Candidate resolution diff display

- GIVEN a reconciliation request with an AI-generated candidate (`internal/ledger/schema.go:141-169`)
- WHEN the operator inspects the candidate in the reconciliation workspace
- THEN the UI MUST render candidate status, allowed paths, check outcomes, and the candidate output diff (`internal/serve/model.go:101-115`).

### Requirement: Lane Result Envelope and Path Violation Diagnosis

The UI MUST inspect `.lucind/result.json` envelopes against `internal/result/result.schema.json:1-98`. If a lane status is demoted from Done to Deviated due to modifying paths outside `allowed_paths` (`internal/run/run.go:576-654`), the UI MUST display the demotion reason, list the offending paths, and cite the preserved worktree location (`internal/run/batch.go:50-52`).

#### Scenario: Deviated lane diagnosis display

- GIVEN a lane demoted to `deviated` by `enforceAllowedPaths` for diffs outside declared `allowed_paths` (`internal/run/run.go:643-653`)
- WHEN the operator inspects the lane in the envelope viewer
- THEN the UI MUST display status `deviated`, the list of out-of-scope paths, and the preserved worktree path (`internal/run/batch.go:50-52`).

## Open Questions

- [ ] Reconciliation action mutation transport: whether the UI will execute reconcile actions (`approve`, `decline`, `cancel`, `renew`, `resolve` in `cmd/lucind-ai/cli.go:1043-1065`) directly via POST endpoints or only render copy-pasteable CLI commands (`internal/serve/handlers.go:36-118`).
- [ ] Overlap evidence formatting: whether `evidence_json` (`internal/serve/model.go:68`) is formatted as syntax-highlighted JSON, `<pre>` text, or structured diff blocks.
- [ ] Expiry countdown synchronization: whether lease and reconciliation expirations (`internal/serve/model.go:56, 84`) should compute remaining seconds on the server or provide server time for browser clock reconciliation.
