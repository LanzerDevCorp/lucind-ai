# Proposal Synthesis Notes: Native Stability Campaign

## Unresolved Contradictions

None.

All three lens drafts (`propose-lens-a.md`, `propose-lens-b.md`, `propose-lens-c.md`) converged cleanly on Candidate 2 (Modular Subpackage Decomposition under `internal/stability/`), common-dir SQLite/WAL persistence under `<git-common-dir>/lucind-ai/stability/v1/stability.db`, 3 sequential Trials (15 total dispatches pinned to `gemini-3.7-flash-high`), 10s lease TTL with monotonic fencing, abrupt `SIGKILL` crash recovery with `/proc` survivor verification, and terminal content-addressed JSON Stability Receipts.

## Coverage Gaps

None.

The canonical proposal covers all 9 required spine sections:
1. Executive Summary & Problem Statement
2. Selected Candidate & Technical Approach
3. Changes to System Concepts & Architecture Rationale
4. User & Capability Impact (8 capabilities)
5. Delta Specifications (5 requirements with 10 Given/When/Then scenarios)
6. Technical Risks & Failure Modes (8 prioritized risk mitigations)
7. Rollback Plan & Additivity (code-level revert, immutable common-dir data, untouched run ledger)
8. Test & Validation Impact (11 validation layers with verified seams)
9. Out of Scope & Open Questions (8 out-of-scope boundaries and 4 open design questions)

## Dropped Citations

1. **Defect 1 — Fabricated Claim in Lens A (`internal/run/attempt.go:737-755` & `internal/overlap/overlap.go:20-23`)**:
   - *Claim in Lens A*: Cited `internal/run/attempt.go:737-755` (alongside `internal/overlap/overlap.go:20-23`) as evidence that replacement Orchestrator B "verifies zero surviving descendant processes (including inherited MCP server grandchildren)."
   - *Verification against real code*: Inspection of `internal/run/attempt.go:737-755` reveals it is exclusively `evaluateOverlapGate` logic resolving `otherSHA` from feature refs. `internal/overlap/overlap.go:20-23` contains error definitions (`ErrMergeBaseEmpty`, `ErrMergeBaseNotFound`, `ErrMergeBaseConflict`). Neither file or line range performs descendant process checking, `/proc` scanning, or grandchild process auditing.
   - *Action*: Dropped the citation completely from the canonical proposal.

2. **Defect 2 — Line Range Overrun in Lens C (`internal/lane/status_test.go:1-32`)**:
   - *Range in Lens C*: Lens C citation manifest listed `internal/lane/status_test.go:1-32`.
   - *Verification against real code*: `internal/lane/status_test.go` has exactly 31 lines (`wc -l` is 31).
   - *Action*: Corrected the line range to `internal/lane/status_test.go:1-31` in the canonical proposal.

## Scope Divergence

1. **Modular Subpackage Decomposition (Candidate 2)**:
   All lenses agreed on Candidate 2 (`store`, `fixture`, `process`, `evidence`, `reconcile` under `internal/stability/`) over monolithic single package (Candidate 1) or separate top-level packages (Candidate 3).

2. **Storage Authority & Common-Dir SQLite**:
   Consensus reached that mutable Stability Campaign authority resides at `<git-common-dir>/lucind-ai/stability/v1/stability.db`, strictly isolated from `<primaryRoot>/.lucind/lucind.db`. Path resolution bypasses `.lucind/` validation constraints (`internal/ledgerpath/ledgerpath.go:40-58`).

3. **Three-Trial Orchestration & Dispatch Quota**:
   Consensus on 3 sequential deterministic Trials, 5 non-retryable `agy` dispatches per Trial pinned to `gemini-3.7-flash-high` (15 total dispatches), with timeout cascades (10m dispatch / 45m Trial / 135m Campaign).

4. **Non-Additive Executor Seam Modification (R5 Requirement)**:
   Real codebase inspection verified that `internal/executor/agy.go:193-205` creates child processes without `SysProcAttr`/`Setpgid`. Because `agy` spawns grandchild MCP server processes (`internal/executor/agy.go:19-40`, `internal/executor/agy_test.go:158-191`), killing the parent PID leaves orphaned grandchildren. Master plan R5 requires modifying `internal/executor/agy.go` to set `Setpgid: true` on Linux, introducing a reviewed blast radius on general lane execution (`internal/run/batch.go:66-70`).

5. **Crash Recovery & Lease Reclaim**:
   Change B is killed via `SIGKILL` after result persistence before Acceptance. Reclaims during the 10s lease TTL are rejected with `ErrLeaseHeld` (`internal/feature/feature.go:334-398`). After expiry, replacement B reclaims ownership, verifies zero surviving processes via `/proc`, adopts the persisted envelope without AI re-dispatch, and promotes B.
