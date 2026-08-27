# Tasks: Acceptance Verifier

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 1,450–1,850 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | Single PR; size:exception required |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Frozen identity and receipts | PR 1 | `go test ./internal/ledger ./internal/run` | N/A—temp SQLite | `internal/ledger/{acceptance.go,schema.go}`, `internal/run/run.go` |
| 2 | Isolated verifier | PR 1 | `go test ./internal/accept ./internal/integrate` | N/A—temp Git repos | `internal/accept`, `internal/integrate` |
| 3 | Receipt-gated CLI | PR 1 | `go test ./cmd/lucind-ai -run TestAccept` | N/A—no E2E runner | `cmd/lucind-ai/cli.go` |

## Phase 1: Identity and Durable Evidence

- [x] 1.1 RED: add `internal/ledger/{acceptance,schema}_test.go` cases for v9 immutable rows, atomic insert, exact cache reuse, and mismatches with no receipt.
- [x] 1.2 GREEN: add v9 migration and `internal/ledger/acceptance.go` APIs with transactions, full binding/evidence, abort triggers, and exact comparison.
- [x] 1.3 REFACTOR: table-drive fixtures; keep `internal/ledger/schema.go` migration order deterministic.
- [x] 1.4 RED: add `internal/run/run_test.go` cases for atomic packet digest, base/candidate commit/tree, allowed paths, and absent identity.
- [x] 1.5 GREEN: persist authoritative terminal identity through `SetDoneCandidate`, never reconstructed refs.
- [x] 1.6 REFACTOR: isolate the done-candidate hook behind existing run/ledger seams.

## Phase 2: Fail-Closed Verification

- [x] 2.1 RED: in `internal/accept/accept_test.go`, prove invalid schema/identity, hard stop, unmet criterion, and bad scope fail with no receipt; `requirements.txt`, `CMakeLists.txt`, executable MD/MDX, and `README.sh` are scope-only, never executable.
- [x] 2.2 RED: reject relative/foreign/mismatched `-C`; use only clean detached candidate (not staged, `commit -a`, empty-index); fixed `sh <owned>/lucind-checks.sh` keeps metacharacters one argv; reject bad cwd, hostile/missing env, Start error, exit 7, signal, timeout, and TERM-ignoring child after reap/cleanup.
- [x] 2.3 GREEN: replace untrusted `internal/accept` scaffold with `Verifier.Verify`: `result.Read`, admission, versioned hashes, exact cache, and unique marker-fenced detached owned worktree.
- [x] 2.4 GREEN: harden `internal/integrate/{integrate,integrate_test}.go` with allowlisted env, owned deadline/process group, escalation, diagnostics, and foreign-worktree preservation.
- [x] 2.5 REFACTOR: centralize cleanup validation; cleanup failure rejects, reports its outcome, and never deletes foreign worktrees.

## Phase 3: CLI Admission

- [x] 3.1 RED: add `cmd/lucind-ai/cli_test.go` cases where `accept --run --lane` succeeds only with an exact receipt, absent receipt exits nonzero, refs remain unchanged, and output calls it mechanical evidence only.
- [x] 3.2 GREEN: add the thin `accept` adapter in `cmd/lucind-ai/cli.go`; render receipt identity without Promotion/CAS or qualitative-review authority.
- [x] 3.3 REFACTOR: keep CLI parsing and verifier construction separate from receipt rendering.

## Phase 4: Verification and Rollback

- [x] 4.1 Run focused ledger/run, accept/integrate, and CLI tests; then run `go test ./... -race -count=1` and `CGO_ENABLED=0 go build ./...`.
- [x] 4.2 Verify receipt immutability; rollback removes callers/adapter while retaining audit rows; exclude Promotion/CAS, qualitative review, lease recovery, metadata/docs, and unrelated dirty files.
