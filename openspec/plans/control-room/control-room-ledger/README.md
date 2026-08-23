# control-room-ledger

## Scope

Add the v6 ledger API surface in new files: run lifecycle, lane metadata/progress writes, progress reads, and pruning/retention. Keep `internal/ledger/ledger.go` untouched so the three same-wave lanes are legal under component-boundary disjointness.

## Non-scope

Do not edit schema migration DDL, executors, run orchestration, HTTP handlers, or UI. The schema feature must already be integrated.

## Exact allowed paths

- `internal/ledger/runs.go`, `internal/ledger/runs_test.go`
- `internal/ledger/progress.go`, `internal/ledger/progress_test.go`
- `internal/ledger/lanes_meta.go`, `internal/ledger/lanes_meta_test.go`

## Acceptance criteria

- The three API groups compile against the v6 schema without modifying `ledger.go`.
- Run/lane/progress operations are transactional where required, ordered by durable IDs/sequence, and pruning is bounded and testable.
- The three packets can run in one batch because their file scopes are pairwise disjoint.

## Definition of done

The DAG validator accepts the sidecar, its one wave exits 0, focused ledger tests and `lucind-ai check` pass, and the feature parent is promoted.
