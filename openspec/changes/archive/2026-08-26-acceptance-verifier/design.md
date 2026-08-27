# Design: Acceptance Verifier

## Technical Approach

Build a deep, fail-closed `internal/accept` module. `run.Execute` freezes identity; `Verifier.Verify` checks that commit in owned isolation and atomically persists mechanical evidence, never promotion or qualitative review.

## Architecture Decisions

| Option | Tradeoff | Decision / rationale |
|---|---|---|
| Caller refs vs persisted identity | Refs drift | Request only run/lane; `run.go:545` atomically stores packet/base/candidate identity before barrier observation. |
| Broad options vs one interface | Options permit ambiguity | `Verifier.Verify(ctx, request)` hides Git, scope, `integrate.Check`, process lifecycle, cleanup, and storage. |
| Mutable report vs immutable row | Updates weaken evidence | Schema v9 receipts use complete binding hashes. |

## Data Flow

`run.Execute → SetDoneCandidate(tx) → Verify → exact-cache lookup → detached owned worktree → result.Read/scope/checks → fenced cleanup → InsertReceipt(tx)`

Admission requires `done`, valid matching result identity, clear hard stops, met criteria, exact NUL-safe Git/result files, in-scope changes, and passing checks. Any mismatch or operational failure returns no receipt.

## Interfaces / Contracts

```go
type AcceptanceRequest struct { RunID, LaneID string }
type Binding struct {
  RunID, LaneID, PacketID, PacketDigest string
  BaseCommit, BaseTree, CandidateCommit, CandidateTree string
  AllowedPathsHash, CheckPolicyHash, EnvironmentHash string
}
type AcceptanceReceipt struct {
  ReceiptID, BindingHash, ResultHash, ChecksHash string
  Binding Binding; CreatedAt time.Time; Cleanup string
}
func (v *Verifier) Verify(context.Context, AcceptanceRequest) (AcceptanceReceipt, error)
```

`PacketDigest` hashes packet semantics, normalized allowed paths, and raw body; `run.go:366` persists it. Git supplies full commit/tree IDs. Paths reject empty/absolute/traversing values before normalization, deduplication, and sorting. Policy hash binds version, timeout, and script blob; environment hash binds platform/toolchain and the complete ordered allowlist passed. Hashes are versioned, ordered, length-prefixed SHA-256; binding hash covers every field.

Isolation is sibling `lucind-ai-worktrees/accept-<lane>-<UUID>` at detached candidate. Its marker binds root/path/candidate. Cleanup validates marker and Git path/HEAD; failure rejects and preserves foreign worktrees.

Schema v9 adds immutable `lane_candidates` terminal identity and `acceptance_receipts` binding/evidence columns with abort triggers. `INSERT OR IGNORE` plus exact comparison gives idempotency; cache lookup by `binding_hash` revalidates columns.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/accept/accept.go`, `accept_test.go` | Modify | Verifier and RED coverage. |
| `internal/integrate/integrate.go`, `integrate_test.go` | Modify | Harden existing subprocess lifecycle; preserve interface. |
| `internal/ledger/acceptance.go`, `acceptance_test.go` | Create | Candidate/receipt operations. |
| `internal/ledger/schema.go`, `schema_test.go` | Modify | Schema v9. |
| `internal/run/run.go`, `run_test.go` | Modify | Atomic `Done` candidate hook. |
| `cmd/lucind-ai/cli.go`, `cli_test.go` | Modify | `accept --run --lane`; success only with receipt. |

## Testing Strategy

Strict TDD covers admission, hashes/cache, migration, isolation, CLI, and every applicable row. Use tables, `t.TempDir()`, and command seams; real Git/scripts skip in short mode. No E2E runner exists.

## Threat Matrix

| Boundary | Applicability | Safe/failure behavior and planned RED tests |
|---|---|---|
| Documentation-like paths | Applicable | Treat all paths uniformly; execute only root `lucind-checks.sh`. RED: `requirements.txt`, `CMakeLists.txt`, executable MD/MDX, `README.sh` are scope-checked and never selected for execution. |
| Git repository selection | Applicable | Canonical persisted root/owned cwd only; reject relative, foreign, or mismatched `-C`. RED each selector. |
| Commit state | Applicable | Verify clean detached commit, never index/worktree state. RED staged, `commit -a`, empty-index candidates. |
| Push state | N/A | No push operation. |
| PR commands | N/A | No PR command composition. |
| Check invocation | Applicable | Fixed argv `sh`, `<owned-root>/lucind-checks.sh`; no interpolation. Failure rejects. RED: metacharacter path remains one argv value. |
| Check cwd | Applicable | Set `cmd.Dir` to canonical frozen candidate; invalid/mismatched cwd rejects. RED: script prints cwd and must equal owned root. |
| Check environment | Applicable | Pass/hash an ordered policy allowlist, never inherited extras. Missing values reject. RED: hostile parent variable is absent; allowlisted value arrives. |
| Check start | Applicable | Any `Start` error is mechanical failure with no receipt and cleanup still runs. RED: command seam returns start error. |
| Check exit/signal | Applicable | Non-zero or signal rejects with bounded diagnostics and cleanup. RED: `exit 7` and self-TERM produce no receipt. |
| Check cancellation/timeout | Applicable | Require a verifier-owned deadline; cancellation/expiry rejects and begins bounded termination. RED: blocking script exceeds timeout and returns no receipt promptly. |
| Descendant lifecycle | Applicable | Own a process group; cancel signals all, escalates after grace, waits/reaps, then cleans. Any failure rejects. RED: TERM-ignoring child is gone and worktree cleaned before return. |

## Migration / Rollout

Additive v9 migration; rollback removes callers but retains audit rows. No backfill or ref change; Promotion/CAS and qualitative review remain separate. Scope stays within the 2,000-line single PR.

## Open Questions

None.
