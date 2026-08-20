# Tasks: Approvals Web UI

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1400–1800 |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | single PR (budget 2000) |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | v3 approvals ledger | PR 1 | `go test ./internal/ledger -race -count=1` | N/A: TempDir SQLite | `internal/ledger/schema.go`, `internal/ledger/ledger.go` |
| 2 | Execute wait gate | PR 1 | `go test ./internal/run -race -count=1` | N/A: fakeExecutor + real ledger | `internal/run/run.go` |
| 3 | Loopback HTTP + UI | PR 1 | `go test ./internal/serve -race -count=1` | `lucind-ai serve --addr 127.0.0.1:7433` | `internal/serve/*.go`, `internal/serve/static/*` |
| 4 | CLI serve flags | PR 1 | `go test ./cmd/lucind-ai -race -count=1` | `lucind-ai serve --help` | `cmd/lucind-ai/cli.go` |

## Phase 1: Foundation

- [ ] 1.1 RED `internal/ledger/schema.go`: TempDir `Open` fails until v3 `approvals` (PK `run_id,lane_id`; `defect_surfaced_later DEFAULT 0`). Spec: Additive Schema / Persist approval record.
- [ ] 1.2 GREEN `internal/ledger/schema.go`: `currentVersion<3` migration, same tx as v2.

## Phase 2: Core Implementation

- [ ] 2.1 RED `internal/ledger/ledger.go`: RequestApproval; one-row Decide; WaitDecision; ApproverRate (Zero defect history, Own rate only); MarkDefectSurfaced; Later failure is not auto-inferred. Stdlib `testing`, TempDir+Open.
- [ ] 2.2 GREEN `internal/ledger/ledger.go`: CRUD, 250ms poll wait, rate=flagged/approved per approver. No new event type.
- [ ] 2.3 RED `internal/run/run.go`: timeout 0 skips wait; goroutine Decide → done; timeout/reject → blocked, never auto-approve; barrier idle until terminal persist. fakeExecutor, real ledger.
- [ ] 2.4 GREEN `internal/run/run.go`: `ApprovalTimeout`; wait on persistCtx between decideStatus `:315` and SetStatus `:338`.

## Phase 3: Integration / Wiring

- [ ] 3.1 RED (threat: unauthenticated loopback HTTP releasing a waiting lane) `internal/serve/*.go`: non-loopback listen fails; bulk POST body 400. httptest + real ledger. Specs: Non-loopback rejected; Bulk request rejected.
- [ ] 3.2 GREEN `internal/serve/*.go`: bind `127.0.0.1` only; GET `/`; POST one `(run_id,lane_id)`; POST `.../defect`.
- [ ] 3.3 RED `internal/serve/static/*`: embed.FS has no approve-all; Fresh load starts unselected; Unselected item cannot be decided; Evidence and command visible; Bare claim withheld.
- [ ] 3.4 GREEN `internal/serve/static/*`: per-item UI, nothing preselected, evidence inline, opencode argv.
- [ ] 3.5 RED `cmd/lucind-ai/cli.go`: `--addr 127.0.0.1:7433`; `--approver` from Username; `--approval-timeout 30m`.
- [ ] 3.6 GREEN `cmd/lucind-ai/cli.go`: `case "serve":`; reject non-loopback `--addr`.

## Phase 4: Testing

- [ ] 4.1 `go test ./... -race -count=1` covers remaining specs: Approved decision persists done; Timeout elapses; Approve records who and when; Approve then persist done; Barrier waits for terminal persist.

## Phase 5: Cleanup

- [ ] 5.1 `internal/serve/*.go`, `internal/run/run.go`: comments at wait hook and loopback reject; drop unused helpers.
