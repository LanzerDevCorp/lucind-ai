# Proposal: Deterministic lucind-ai Orchestrator

## Intent

Make SDD execution reproducible across Claude Code and OpenCode. The canonical skill must tell one orchestrator exactly how to sequence planning, waves, evidence, recovery, and reporting; `lucind-ai` must enforce facts that prompts cannot make deterministic. Fork-local roots, ledgers, targets, and worktrees must remain isolated.

## Scope

### In Scope
- Define the two-layer contract: canonical Claude skill/reference state machine plus machine-observable CLI/runtime invariants.
- Make OpenCode ship an exact byte-for-byte copy of the Claude skill tree, with parity verification; keep its TypeScript wrapper thin, argv-safe, and cancellable.
- Add deterministic preflight, per-wave late target binding, consumer-test ownership, evidence precedence, retry/recovery/cleanup, and terminal reporting.

### Out of Scope
- New lifecycle states, scheduler/wave engine, flags, routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives.
- Choosing executor, model, provider, or profile; semantic approval and promotion remain human-owned.
- Cross-fork coordination, global state, automatic main promotion, or redesign of unrelated SDD flows.

## Capabilities

### New Capabilities
- `deterministic-orchestrator-contract`: Cross-runtime preflight, sequencing, evidence, recovery, and terminal-report contract.

### Modified Capabilities
- `sdd-apply`: Make phase/wave barriers, DAG target handling, and consumer-test ownership explicit.
- `parent-feature-integration`: Preserve immutable target identity, CAS, no-redispatch retry, and isolated recovery.
- `packet-authoring-contract`: Was classified New against this Change's original `main` base (`705cf49`), where no live spec existed. `feature/skill-provisioning-and-phase-specialist` (merged into this branch at `61aa0cc` on the human's explicit direction, after having been an intentionally isolated sibling Change for most of this Change's planning) delivered a comprehensive live spec first. Reclassified Modified: this Change now adds one narrow scenario — `allowed_paths` omitted defaults to open scope and skips diff-boundary/overlap checks — that the live spec did not already state.
- `acceptance-verifier`: Same history as above. Reclassified Modified: this Change now extends "Fail-Closed Mechanical Criteria" to make explicit that a fired hard stop demotes the lane to blocked regardless of the envelope's claimed status — a mechanism the live spec's prose already implies but the runtime does not yet enforce (`internal/run/run.go` `decideStatus` has no loop over `HardStop.Fired`).

## Approach

**Prompt/reference layer:** author only in `plugin/claude-code/skills/lucind-ai/`; model preflight → phase routing → synthesis → late-bind each wave → dispatch/barrier → verify/archive → terminal report. Frozen packet/result/schema/check/ledger evidence outranks narrative; qualitative approval stays separate. `plugin/opencode/skills/lucind-ai/` is generated/verified as the exact copy, never a second contract.

**Runtime layer:** add only narrow enforcement/reporting at existing `cmd/lucind-ai`, `internal/packet`, `internal/run`, `internal/dag`, `internal/ledger`, `internal/accept`, and `internal/worktree` boundaries: embedded schema/content freshness, SQLite-safe concurrent writes and truthful ledger projections, packet/DAG target validation, per-wave binding, and owned cleanup/recovery. Consumer tests belong to this Change and must assert the actual terminal consumers, not just producers.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| Claude skill/references and OpenCode copy | Modified | Canonical contract, parity checks, safety/recovery/reporting guidance. |
| CLI, packet/DAG, run/ledger/accept/worktree packages | Modified | Fail-closed invariants and observable evidence only. |
| `openspec/specs/` and tests | Modified | New contract plus deltas and cross-surface consumer tests. |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Stale skill/schema or SQLite projection races | Med | Content/version checks and real-SQLite `-race` tests. |
| Recovery deletes evidence or redispatches | Med | Preserve existing force boundary, immutable receipts, and idempotent retry. |

## Rollback Plan

Revert the skill/reference, parity-test, and additive runtime commits independently; retain existing packet, ledger, lifecycle, and CAS behavior. Do not migrate or rewrite prior evidence.

## Dependencies

- Existing lifecycle, packet admission, DAG, ledger, acceptance, feature-parent, and cleanup primitives; strict checks: `go test ./... -race -count=1`, `go build ./...`, `./lucind-checks.sh`, `gofmt`.

## Success Criteria

- [ ] Claude and OpenCode skill trees are byte-identical and parity-tested.
- [ ] Invalid preflight fails before allocation; every wave binds the correct target and advances only after a passing barrier.
- [ ] Retry is no-redispatch, cleanup is owner-safe, and terminal output names authoritative evidence, integrated/reverted lanes, blockers, and promotion status.
- [ ] All required checks pass without adding duplicate states, schedulers, flags, or routing.
