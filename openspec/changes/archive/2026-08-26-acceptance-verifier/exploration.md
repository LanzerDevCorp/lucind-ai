## Exploration: acceptance-verifier

### Current State
`run.Execute` already fail-closes dispatched lanes: it admits a packet with feature/base identity, creates a lane worktree, validates `.lucind/result.json` through `result.Read`, checks the recorded-base four-way diff against `allowed_paths`, then persists the terminal lane status. `integrate.PromoteCAS` separately protects ref mutation with an expected SHA.

The untracked `internal/accept` scaffold is a convenience flow: resolve a lane branch, infer or accept a base, inspect the diff, parse an envelope, create `accept-<lane>` detached worktree, run `integrate.Check` (`lucind-checks.sh`), remove it, and print/write a report. The CLI returns failure only for failed checks or `DEVIATED` scope.

That does not meet the protocol. It silently ignores packet open/parse failures; JSON-unmarshals instead of schema-validating; accepts missing/invalid envelopes, fired hard stops, unmet criteria, undeclared scope, and candidate/envelope identity mismatches; reads the envelope from the primary checkout rather than the frozen candidate; and silently ignores report-write and cleanup errors. It also derives a merge-base when the recorded dispatch base is unavailable, losing the packet identity required by the protocol.

Hashes and a receipt can prove binding and integrity (packet/base/candidate/tree/check-policy/environment/receipt linkage), not that the diff or tests are semantically correct. Full-diff and changed-test review remain human or independent read-only-agent judgment. Automatic Lane acceptance is distinct from human Change promotion; this change must not call promotion/CAS as an acceptance side effect.

### Affected Areas
- `internal/accept/accept.go` — untracked scaffold; reusable diff/check/report mechanics, but currently fail-open and unbound.
- `internal/accept/accept_test.go` — one happy-path integration-style test; lacks negative identity, schema, hard-stop, cleanup-race, and persistence cases.
- `cmd/lucind-ai/cli.go` / `cmd/lucind-ai/cli_test.go` — exposes `lucind-ai accept`; CLI should remain a thin adapter over the verifier and report a receipt/result.
- `internal/run/run.go` — authoritative dispatch-time packet/base metadata and fail-closed envelope/scope behavior to reuse, not duplicate loosely.
- `internal/result/result.go` — authoritative schema validator; acceptance must use it and additionally bind its fields to the request/candidate.
- `internal/worktree/worktree.go` — stable sibling-worktree convention, but current forced cleanup has no verifier ownership/fencing.
- `internal/feature/feature.go` — lease fencing exists; scaffolded `ForceReleaseLease` expires any lease without owner/fence/liveness validation and is unsafe for concurrent recovery.
- `internal/integrate/integrate.go` — `Check` is the repository-script seam; `PromoteCAS` stays outside acceptance and retains promotion authority.
- `docs/orchestrator-acceptance-protocol.md` — accurately identifies the irreducible review steps and isolated verification requirement, but is evidence/requirements input, not approved implementation design.
- `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md` — checklist wording is reusable guidance only; it must not claim the scaffold is trustworthy before verifier receipts exist.

### Approaches
1. **Deep verifier module** — Implement `Verify(AcceptanceRequest) -> AcceptanceReceipt` behind one internal seam; the verifier resolves immutable request identity, freezes a detached candidate worktree, validates scope/envelope/check results, emits a durable atomic receipt, and fails closed.
   - Pros: concentrates Git/worktree/check/CAS-adjacent choreography; testable through one interface; receipt gives callers an auditable decision; preserves acceptance/promotion authority split.
   - Cons: requires explicit receipt storage/schema and carefully injected Git, clock, worktree, check, and persistence dependencies.
   - Effort: Medium.

2. **Harden the existing `accept.Execute` command flow** — Add validations and error handling directly to the scaffold while preserving its broad options/report shape.
   - Pros: lower initial code movement; retains existing CLI output.
   - Cons: shallow interface leaks path/ref/base/worktree details; encourages callers to construct identities independently; makes receipt atomicity, concurrency ownership, and test seams harder; risks preserving fail-open defaults.
   - Effort: Medium.

### Recommendation
Choose the deep verifier. `AcceptanceRequest` must require immutable lane/packet identity, recorded base SHA, candidate commit/tree, allowed-path/check-policy hashes, and execution environment identity. The verifier MUST create a unique owned detached worktree, validate via `result.Read`, reject absent/mismatched envelope packet/commit/hard-stops/criteria, run the repository script in that worktree, and atomically persist an immutable `AcceptanceReceipt` or return an error/no receipt. Cache reuse is valid only for the same binding tuple.

CLI: parse stable identifiers, invoke `Verify`, render receipt/result, and never promote. Repository script: define project-specific mechanical checks only. Read-only subagents: inspect frozen diff and tests, produce qualitative findings bound to the receipt candidate; they cannot write receipts, mutate refs, clean foreign worktrees, or release leases. Promotion remains a human-confirmed CAS operation after acceptance.

Scaffold classification: reusable — `inspectDiff`'s NUL-safe name-status parsing, isolated detached verification concept, `integrate.Check`, reporting presentation, and happy-path fixture shape. Incomplete — CLI wiring, packet/base resolution, scope classification, and tests. Unsafe — envelope `json.Unmarshal`, missing hard-stop/criterion/identity checks, fallback merge-base, fixed forced worktree deletion/prune, ignored cleanup/output errors, and un-fenced `ForceReleaseLease`. Out-of-scope — unrelated CONTEXT/plugin/marketplace metadata and reconciliation/defect changes; the acceptance CLI must not absorb them.

### Risks
- **Fail-open:** confirmed. The current scaffold can report success for missing or invalid acceptance evidence; receipt emission and CLI success must be fail-closed.
- **Unbound identities:** confirmed. Packet parse errors, loose lane refs, inferred bases, and primary-root envelope reads permit packet/base/candidate/envelope drift.
- **Concurrent cleanup:** confirmed. Fixed `accept-<lane>` path plus unconditional force-remove/prune can delete another verifier's worktree; ownership token plus lease/fence-aware cleanup is required. Lease force-release is additionally unsafe without owner/fence and positive liveness proof.
- **Silent persistence:** confirmed. `--out` suppresses directory/write errors; cleanup errors are discarded. Receipt persistence must be transactional and all attempted persistence/cleanup outcomes explicit.
- **Strict TDD / review budget:** constrain the single PR to verifier request/receipt and storage, CLI adapter, scoped repository-script adapter, and table-driven tests. Start RED with identity/schema/hard-stop/scope/check/receipt/cleanup-race cases using `t.TempDir()` and fake command seams; external Git/script cases remain skippable under `testing.Short()`. Do not include documentation rewrites, lease-recovery redesign, or promotion changes. Keep changed lines at or below 2000.

### Ready for Proposal
Yes — propose a fail-closed deep verifier and immutable receipt, explicitly excluding promotion and unrelated scaffolding.
