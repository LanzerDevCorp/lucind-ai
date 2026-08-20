# Tasks: Approvals Web UI

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700–900 lines |
| 400-line budget risk | Low |
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
| 1 | Schema v3, approval wait gate, and ledger operations | PR 1 | `go test ./internal/ledger ./internal/run -race -count=1` | N/A (unit tests) | `internal/ledger`, `internal/run` |
| 2 | Loopback HTTP server, embed UI, and CLI serve command | PR 1 | `go test ./internal/serve ./cmd/lucind-ai -race -count=1` | `lucind-ai serve --addr 127.0.0.1:7433` | `internal/serve`, `cmd/lucind-ai` |

## Phase 1: Foundation (Ledger & Schema)

- [ ] 1.1 RED: Write tests for schema v3 migration and `approvals` table in `internal/ledger/ledger_test.go`
- [ ] 1.2 GREEN: Add schema v3 migration for `approvals` table in `internal/ledger/schema.go`
- [ ] 1.3 RED: Write tests for `WaitDecision`, `MarkDefectSurfaced`, and `ApproverRate` in `internal/ledger/ledger_test.go`
- [ ] 1.4 GREEN: Implement `WaitDecision`, `MarkDefectSurfaced`, and `ApproverRate` in `internal/ledger/ledger.go`

## Phase 2: Core Implementation (Run Gate & Serve)

- [ ] 2.1 RED: Write tests for `Execute` approval wait, timeout to `blocked`, and zero timeout bypass in `internal/run/run_test.go`
- [ ] 2.2 GREEN: Implement `ApprovalTimeout` wait between `decideStatus` and `SetStatus` in `internal/run/run.go`
- [ ] 2.3 RED: Write threat matrix tests for non-loopback listen rejection and bulk request 400 in `internal/serve/server_test.go`
- [ ] 2.4 GREEN: Implement loopback server, mux, and single-item decision handlers in `internal/serve/server.go` and `internal/serve/handlers.go`
- [ ] 2.5 RED: Write tests verifying embedded UI assets and absence of bulk approval controls in `internal/serve/static_test.go`
- [ ] 2.6 GREEN: Implement embedded UI with individual decisions and inline evidence in `internal/serve/static/index.html` and `internal/serve/static/app.js`

## Phase 3: Integration / Wiring

- [ ] 3.1 RED: Write tests for `serve` subcommand flag parsing (`--addr`, `--approver`, `--approval-timeout`) in `cmd/lucind-ai/cli_test.go`
- [ ] 3.2 GREEN: Register `case "serve":` command dispatch and flag handling in `cmd/lucind-ai/cli.go`
- [ ] 3.3 RED: Write tests ensuring batch barrier observation waits for terminal approval persist in `internal/run/batch_test.go`
- [ ] 3.4 GREEN: Verify `runOneLane` batch barrier observation ordering after terminal persist in `internal/run/run.go`

## Phase 4: Testing & Verification

- [ ] 4.1 Test full approval lifecycle (approve, timeout, rate) against spec scenarios in `internal/ledger/ledger_test.go` and `internal/run/run_test.go`
- [ ] 4.2 Test loopback binding, error responses, and embed UI serving in `internal/serve/server_test.go` and `cmd/lucind-ai/cli_test.go`

## Phase 5: Cleanup & Polish

- [ ] 5.1 Format Go code and verify test suite passes via `internal/ledger/ledger.go`, `internal/run/run.go`, `internal/serve/server.go`, and `cmd/lucind-ai/cli.go`
- [ ] 5.2 Validate static assets and doc comments in `internal/serve/static/index.html` and `internal/serve/server.go`
