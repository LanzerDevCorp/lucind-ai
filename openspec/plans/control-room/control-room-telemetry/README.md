# control-room-telemetry

## Scope

Add ledger schema v6 for the Control Room: `runs`, `lane_progress`, lane metadata columns, indexes, open event types, and retention support. Follow the existing create-copy-drop-rename migration pattern in `internal/ledger/schema.go`; the current schema is v5 (`schema.go:9-10`, `182-219`).

## Non-scope

Do not add executor streaming, HTTP/SSE routes, UI assets, CLI flags, or new methods to `internal/ledger/ledger.go`. Those belong to later features.

## Exact allowed paths

- `internal/ledger/schema.go` (existing; schema/migration implementation)
- `internal/ledger/schema_v6_test.go` (new; migration and constraint tests)

## Acceptance criteria

- Opening a v5 ledger migrates it exactly once to version 6 and preserves existing rows.
- New tables/columns/indexes and event type behavior match the proposal's observable contract.
- Reopening the same ledger is idempotent and all focused ledger tests pass.
- No existing migration versions are rewritten.

## Definition of done

The apply packet commits only the two allowed paths, `go test ./internal/ledger` and `lucind-ai check` pass, and the feature-targeted integration attempt is promoted on its named parent branch.
