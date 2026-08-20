# Tasks: Approvals Web UI

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 900–1800 (agy: 700-900, cursor-agent: 1400-1800 — both well under budget) |
| 400-line budget risk | Low relative to this session's 2000-line budget |
| Chained PRs recommended | No |
| Suggested split | Single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Schema v3 + approval-wait gate + ledger ops | PR 1 | `go test ./internal/ledger ./internal/run -race -count=1` | N/A: TempDir SQLite + fakeExecutor | `internal/ledger/{schema,ledger}.go`, `internal/run/run.go` |
| 2 | Loopback HTTP server, embed UI, CLI `serve` | PR 1 | `go test ./internal/serve ./cmd/lucind-ai -race -count=1` | `lucind-ai serve --addr 127.0.0.1:7433` | `internal/serve/*`, `cmd/lucind-ai/cli.go` |

## Phase 1: Foundation (Ledger & Schema)

- [ ] 1.1 RED `internal/ledger/ledger_test.go`: schema v3 migration (`currentVersion<3`), `approvals` table (PK `run_id,lane_id`; `defect_surfaced_later DEFAULT 0`). Spec: Additive Schema / Persist approval record.
- [ ] 1.2 GREEN `internal/ledger/schema.go`: v3 migration, same tx pattern as v2.
- [ ] 1.3 RED `internal/ledger/ledger_test.go`: `RequestApproval`, one-row `Decide`, `WaitDecision`, `MarkDefectSurfaced`, `ApproverRate`. Specs: Zero defect history; Own rate only; Later failure is not auto-inferred.
- [ ] 1.4 GREEN `internal/ledger/ledger.go`: CRUD, 250ms poll wait, rate = flagged/approved per approver, no new event type.

## Phase 2: Core Implementation (Run Gate)

- [ ] 2.1 RED `internal/run/run_test.go`: approval wait blocks until `Decide`; zero timeout bypasses; timeout/reject → `blocked`, never auto-approve. fakeExecutor + real ledger. Specs: Approve then persist done; Timeout persists blocked, never done; Zero timeout bypasses the gate.
- [ ] 2.2 GREEN `internal/run/run.go`: `Deps.ApprovalTimeout`; wait on `persistCtx` between `decideStatus` (`:315`) and `SetStatus` (`:338`).
- [ ] 2.3 RED `internal/run/batch_test.go`: barrier `Observe` does not treat a waiting lane as observed; stays idle until terminal persist. Specs: Barrier waits for terminal persist; Barrier stays idle while one lane waits.
- [ ] 2.4 GREEN `internal/run/batch.go` / `run.go`: verify `runOneLane`'s `Observe` ordering after terminal persist (no code change expected if 2.2 is correctly placed — this task's job is proving it with a real barrier).

## Phase 3: Integration / Wiring (HTTP + CLI)

- [ ] 3.1 RED (threat: unauthenticated loopback HTTP releasing a waiting lane) `internal/serve/server_test.go`: non-loopback listen fails; bulk POST body → 400. httptest + real ledger. Specs: Non-loopback rejected; Bulk request rejected.
- [ ] 3.2 GREEN `internal/serve/server.go`, `internal/serve/handlers.go`: bind `127.0.0.1` only; `GET /`; `POST /approvals/{run}/{lane}`; `POST .../defect`.
- [ ] 3.3 RED `internal/serve/static_test.go`: `embed.FS` has no approve-all control; items start unselected; evidence + `opencode` command render inline. Specs: Fresh load starts unselected; Unselected item cannot be decided; Evidence and command visible; Bare claim withheld.
- [ ] 3.4 GREEN `internal/serve/static/index.html`, `internal/serve/static/app.js`: per-item UI, nothing preselected, evidence inline, `opencode` argv shown.
- [ ] 3.5 RED `cmd/lucind-ai/cli_test.go`: `serve` flags (`--addr 127.0.0.1:7433`, `--approver` from `user.Current().Username`, `--approval-timeout 30m`); non-loopback `--addr` rejected at CLI level.
- [ ] 3.6 GREEN `cmd/lucind-ai/cli.go`: `case "serve":` dispatch and flag registration.

## Phase 4: Testing

- [ ] 4.1 `go test ./... -race -count=1` covers remaining spec scenarios: Approved decision persists done; Rejected decision blocks the lane; Timeout elapses; Approve records who and when; Loopback listen.
- [ ] 4.2 Manual runtime check: `lucind-ai serve --addr 127.0.0.1:7433` starts, UI reachable, `Ctrl-C` clean shutdown.

## Phase 5: Cleanup

- [ ] 5.1 `gofmt`, `go vet`, doc comments at the wait hook (`run.go`) and loopback-reject (`serve/server.go`); drop unused helpers.
