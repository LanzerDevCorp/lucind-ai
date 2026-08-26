# Proposal: Acceptance Verifier

## Intent

Replace the untrusted convenience scaffold with a fail-closed verifier that produces durable, immutable proof that a completed lane satisfies mechanical acceptance rules. Acceptance is not human-confirmed Promotion/CAS and MUST never mutate refs.

## Scope

### In Scope
- A deep `Verify(AcceptanceRequest) -> AcceptanceReceipt` module with immutable lane, packet, base, candidate/tree, policy, and environment binding.
- Unique verifier-owned detached worktree; schema, hard-stop, done-criteria, and recorded-base scope validation; repository checks through `lucind-checks.sh`.
- Atomic immutable receipt persistence and exact-binding-only cache reuse; CLI adapter renders the result without authority escalation.

### Out of Scope
- Lease-recovery redesign, feature promotion/CAS changes, and ref mutation.
- Plugin/marketplace metadata, broad documentation rewrites, and work exceeding the 2,000-line single-PR budget.

## Capabilities

### New Capabilities
- `acceptance-verifier`: Fail-closed, receipt-bound mechanical acceptance for a frozen lane candidate.

### Modified Capabilities
None. Existing lane execution, allowed-path enforcement, promotion, and qualitative-verification requirements remain unchanged.

## Approach

Hook acceptance after `internal/run.Execute` has persisted the lane's terminal result, using its recorded dispatch identity—not inferred refs or a live merge-base. The verifier validates the frozen candidate's result via `result.Read`, rejects missing/mismatched packet or commit identity, fired hard stops, unmet criteria, undeclared/out-of-scope changes, or check failures, then writes one receipt atomically or returns no receipt. `integrate.Check` remains the `lucind-checks.sh` seam. Read-only subagents may attach qualitative findings to a receipt but cannot emit receipts, alter refs, or gain promotion authority.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/accept/` | Modified | Replace untrusted scaffold with verifier and tests. |
| `internal/ledger/` | Modified | Add atomic immutable receipt storage/migration. |
| `cmd/lucind-ai/cli.go` | Modified | Thin receipt-rendering adapter only. |
| `internal/result/`, `internal/integrate/`, `internal/worktree/` | Modified | Reuse validation, check, and owned-worktree seams. |
| `internal/run/run.go` | Modified | Expose/use persisted terminal-lane identity hook only. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Identity or cleanup race | Med | Immutable binding, unique ownership token, explicit cleanup errors. |
| Fail-open scaffold behavior | High | RED tests for every rejection; no receipt on failure. |
| Scope expansion | Med | Single PR, strict TDD, ≤2,000 changed lines. |

## Rollback Plan

Revert verifier, CLI adapter, and receipt migration together; retain receipts as audit evidence. No promoted refs require rollback.

## Dependencies

- `result.Read`, recorded lane metadata, `integrate.Check`, and ledger transactions.

## Success Criteria

- [ ] Invalid identity/schema/hard-stop/criteria/scope/check input returns an error and persists no receipt.
- [ ] A valid frozen candidate creates exactly one atomically persisted receipt bound to its full identity tuple.
- [ ] Cache reuse occurs only for an identical binding tuple; `accept` never calls promotion/CAS.
