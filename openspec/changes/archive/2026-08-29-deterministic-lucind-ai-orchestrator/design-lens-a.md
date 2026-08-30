# Design Lens A — Decisions: Deterministic lucind-ai Orchestrator

## Assumed architecture

This change extends the existing two-layer architecture without adding new lifecycle states, schedulers, or routing engines. The prompt/reference layer is authored canonically under `plugin/claude-code/skills/lucind-ai/` and replicated byte-identically into `plugin/opencode/skills/lucind-ai/` with automated parity checks. The Go runtime layer extends `cmd/lucind-ai`, `internal/packet`, `internal/dag`, `internal/run`, `internal/ledger`, and `internal/result` for preflight verification, target-free packet admission, deterministic wave barriers, frozen evidence verification, and CAS attempt lifecycle.

## Technical Approach

We standardize SDD execution across Claude Code and OpenCode by combining a canonical prompt orchestrator contract with fail-closed Go runtime enforcement. The orchestrator executes deterministic preflight checks prior to worktree allocation per `deterministic-orchestrator-contract` (`Cross-Runtime Orchestrator Preflight and Sequencing`). Packets are authored as reusable templates omitting hardcoded feature targets and defaulting omitted `allowed_paths` to open scope, with target identity bound dynamically at wave dispatch per `packet-authoring-contract` (`Target-Free Packet Authoring and Late Binding`). Runtime execution determines lane and batch acceptance strictly from immutable candidate commits, tree hashes, schema-validated result envelopes (`.lucind/result.json`), and clean porcelain state per `acceptance-verifier` (`Frozen Evidence Acceptance Verification`). DAG wave execution enforces fail-closed barriers, advancing to wave N+1 only when wave N exits 0 with all lanes completed and integrated without path collisions per `sdd-apply` (`Orchestrator Advances Only on a Passing Wave`). Feature parent integration uses lease-backed attempts, CAS ref updates, and idempotent no-redispatch retry/recovery that fails closed on ref mismatches while preserving evidence and worktrees per `parent-feature-integration` (`Recoverable Idempotent Attempts`).

## Decision 1 — Two-Layer Split: Canonical Claude Skill with Parity-Verified OpenCode Copy and Enforcing Runtime

**Choice**: Author the canonical orchestrator contract solely in `plugin/claude-code/skills/lucind-ai/`, maintain `plugin/opencode/skills/lucind-ai/` as a verified byte-identical copy, and enforce runtime invariants in `cmd/lucind-ai` and `internal/*`.
**Alternatives considered**: Skill-only contract without Go runtime checks; dual-authored divergent skill trees; embedding all orchestration state logic into Go.
**Rationale**: Prompts cannot enforce filesystem, process, SQLite, or schema truth deterministically across agent runtimes. Dual skill trees cause prompt drift. The two-layer model preserves prompt flexibility while guaranteeing machine-enforced safety (`proposal.md:30-34`, `explore.md:36-39`).
**Terminal consumer**: `Cross-Runtime Orchestrator Preflight and Sequencing` in `specs/deterministic-orchestrator-contract/spec.md:5-26`, executed via `plugin/claude-code/skills/lucind-ai/SKILL.md:1-8` and `cmd/lucind-ai/cli.go:95-127`.

## Decision 2 — Preflight Verification and Fail-Closed Worktree Allocation Barrier

**Choice**: Enforce deterministic preflight checks (clean workspace root, non-linked worktree, binary/schema freshness, skill parity) before worktree creation or lane dispatch, halting on failure.
**Alternatives considered**: Lazy validation during lane worktree creation; post-dispatch verification in `Integrate`; manual operator checks.
**Rationale**: Creating worktrees before verifying preconditions creates orphan worktrees and wastes agent quota on invalid dispatches (`cmd/lucind-ai/cli.go:277-280`, `internal/run/run.go:294-296,307-310`). Preflight isolation prevents cross-talk in sibling worktrees (`internal/worktree/worktree.go:1-14`).
**Terminal consumer**: `resolvePrimaryRoot` and `worktree.IsLinkedWorktree` at `cmd/lucind-ai/cli.go:267-280,791-800` and `specs/deterministic-orchestrator-contract/spec.md:5-26`.

## Decision 3 — Target-Free Packet Authoring with Late Dynamic Target Binding

**Choice**: Author reusable packet templates without hardcoded feature targets (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`), injecting targets dynamically at wave dispatch. Default omitted `allowed_paths` to open scope.
**Alternatives considered**: Requiring hardcoded feature targets in all templates; defaulting omitted targets to `main` (`legacy_main`); static feature-pinned packet generation during planning.
**Rationale**: Hardcoded targets couple reusable templates to specific branches, causing routing mistakes or premature resolution (`internal/packet/packet.go:63-72,114-138`, `internal/run/integrate_feature.go:31-77`). Omitted `allowed_paths` defaulting to open scope enables full-repo sweeps safely (`internal/packet/disjoint.go:24-48`, `internal/run/run.go:408-410`).
**Terminal consumer**: `validatePacketAdmission` in `internal/run/run.go:270-285` and `Target-Free Packet Authoring and Late Binding` in `specs/packet-authoring-contract/spec.md:5-26`.

## Decision 4 — Frozen Evidence Acceptance Verification Over Narrative Claims

**Choice**: Determine lane and batch acceptance strictly from immutable candidate commits, tree hashes, schema-compliant result envelopes (`.lucind/result.json`), and clean worktrees. Violated hard stops or undeclared path modifications automatically demote status regardless of claimed criteria.
**Alternatives considered**: Relying on executor exit codes or narrative summary text; trusting `done_criteria` booleans without verification; allowing qualitative approvals to override fired hard stops.
**Rationale**: Dispatched agents have reported success despite fired hard stops or undeclared path modifications. Machine verification of result envelopes (`result.Read` against `result.schema.json`), four-way diff unions (`enforceAllowedPaths`), and completion modes (`enforceCompletionMode`) guarantees truthfulness (`internal/run/run.go:402-415,549-691`, `internal/result/result.go:117-162`).
**Terminal consumer**: `decideStatus`, `enforceAllowedPaths`, and `enforceCompletionMode` in `internal/run/run.go:402-415,549-691`, satisfying `Frozen Evidence Acceptance Verification` in `specs/acceptance-verifier/spec.md:5-26`.

## Decision 5 — Deterministic Wave Barriers and Strict Non-Advancement on Reverted or Blocked Lanes

**Choice**: Advance to wave N+1 if and only if wave N's `lucind-ai run` exits 0 with all lanes completed (`status=done`) and integrated without unhandled path overlap. Any non-zero exit, blocked/deviated/failed lane, or reverted lane halts remaining waves for human review.
**Alternatives considered**: Speculative dispatch of downstream waves on partial failure; automated skipping of failed nodes; swallowing reverted lanes.
**Rationale**: Downstream DAG nodes depend on the committed parent tree produced by prior waves. Advancing on partial integration or unhandled path overlap causes cascade merge conflicts and corrupted branches (`internal/dag/waves.go:16-72`, `internal/dag/overlap.go:52-79`, `cmd/lucind-ai/cli.go:361-370`).
**Terminal consumer**: `runDispatch` exit code check in `cmd/lucind-ai/cli.go:365-370` and `Orchestrator Advances Only on a Passing Wave` in `specs/sdd-apply/spec.md:5-19`.

## Decision 6 — Idempotent Integration Attempts, CAS Promotion, and Fail-Closed State Recovery

**Choice**: Maintain immutable idempotency keys and recorded inputs in SQLite for integration attempts. Replays return recorded terminal outcomes without redispatch. Interrupted attempts verify recorded expected vs. current parent ref SHAs before re-executing CAS; ref mismatches fail closed (remain `blocked`) while preserving worktrees, diagnostic artifacts, and ledger rows.
**Alternatives considered**: Blind redispatch of all batch lanes on retry; unconditional ref overwriting without CAS; destructive cleanup of worktrees on recovery failure.
**Rationale**: Redispatching completed lanes wastes API quota and risks non-deterministic code changes. Unchecked ref promotion causes git history clobbering. Preserving worktrees and ledger evidence on CAS/recovery failure enables safe inspection and reconciliation (`internal/run/attempt.go:217-256,508-570,576-682`, `internal/run/integrate_feature.go:80-140`).
**Terminal consumer**: `ExecuteAttempt` and `RecoverAttempt` in `internal/run/attempt.go:217-256,576-682`, satisfying `Recoverable Idempotent Attempts` in `specs/parent-feature-integration/spec.md:5-24`.

## Open Questions

- [ ] None.
