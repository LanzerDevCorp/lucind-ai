# Tasks Lens B — Partition & Dispatch Shape: Control Room Serve

## Assumed decomposition

The implementation spans 4 primary files across `internal/ledger/`, `internal/serve/`, and `cmd/lucind-ai/`, decomposing into two sequential work units. Unit 1 delivers additive SQLite cursor queries (`Runs`, `EventsSince`, `IntegrationEventsSince`) on `*ledger.Ledger`. Unit 2 delivers the serve HTTP surface (`NewHandler` with `*Model`, `/api/v1/*` REST routes, SSE event stream tailing, JSON 404 under `/api/`, CLI wiring in `serveDispatch`, and test suite updates). The critical path is sequential: Unit 2 depends on Unit 1's ledger queries to compile, while `internal/serve/handlers.go` and `cmd/lucind-ai/cli.go` must be co-delivered in Unit 2 due to the breaking `NewHandler` signature change.

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| Unit 1 | Additive ledger cursor query methods (`Runs`, `EventsSince`, `IntegrationEventsSince`) on `*ledger.Ledger` | `internal/ledger/ledger.go`<br>`internal/ledger/ledger_test.go` | `agy` | `internal/ledger/ledger.go`, `internal/ledger/ledger_test.go` — removes the three cursor query methods without breaking existing callers. |
| Unit 2 | Serve HTTP routing (`/api/v1/*`, JSON 404, SSE event stream), `*Model` injection in `NewHandler`, CLI wiring, and test coverage | `internal/serve/handlers.go`<br>`cmd/lucind-ai/cli.go`<br>`internal/serve/server_test.go`<br>`cmd/lucind-ai/cli_test.go` | `cursor-agent` | `internal/serve/handlers.go`, `cmd/lucind-ai/cli.go`, `internal/serve/server_test.go`, `cmd/lucind-ai/cli_test.go` — restores 3-argument `NewHandler`, reverts `/api/v1/*` routes and SSE stream, and returns to `/api/state`-only polling. |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| Wave 1 | Unit 1 | No | Yes — additive query methods on `*ledger.Ledger` introduce no breaking API changes and pass `lucind-checks.sh` (`go build ./...` and `go test ./...`) independently. |
| Wave 2 | Unit 2 | No | Yes — depends on Wave 1; wires `*Model` into `NewHandler` in lockstep across `handlers.go`, `cli.go`, and `server_test.go`, passing `lucind-checks.sh` cleanly. |

## Disjointness Check

- **Wave 1 (Unit 1)**: Single-unit wave; pairwise disjointness check is not applicable.
- **Wave 2 (Unit 2)**: Single-unit wave; pairwise disjointness check is not applicable.

### Cross-Unit & Intra-Wave Partition Analysis
- **Unit 1 vs Unit 2 (Parallel Wave 1 rejected)**: Path sets `{internal/ledger/ledger.go, internal/ledger/ledger_test.go}` and `{internal/serve/handlers.go, cmd/lucind-ai/cli.go, internal/serve/server_test.go, cmd/lucind-ai/cli_test.go}` are disjoint under component-boundary prefix rules (`disjoint.go:8-22`), but fail the `Integrate` gate ("Green on its own") because Unit 2 cannot compile without Unit 1's ledger methods.
- **Handlers vs CLI (Parallel Unit 2 split rejected)**: Path sets `{internal/serve/handlers.go, internal/serve/server_test.go}` and `{cmd/lucind-ai/cli.go, cmd/lucind-ai/cli_test.go}` are disjoint under component-boundary prefix rules, but fail the `Integrate` gate because updating `NewHandler`'s signature on either branch alone breaks `go build ./...` on the other.

## Sidecar Recommendation

**Recommendation**: single packet, no sidecar
**Rationale**: Following the precedent in `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md`, a sequential two-unit decomposition where the initial unit is trivial (~60 lines of additive queries in `internal/ledger/ledger.go`) does not pay for `apply-dag.yaml` sidecar orchestration, `lucind-ai split`, and per-wave bisection overhead. The entire change is ~400–550 lines across 4 files. A single packet executed sequentially with internal work-unit commits satisfies all verification gates cleanly with zero dispatch overhead.

## Open Questions

- [ ] None
