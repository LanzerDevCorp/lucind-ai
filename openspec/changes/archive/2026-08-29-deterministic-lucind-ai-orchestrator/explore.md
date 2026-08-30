## Exploration: deterministic-lucind-ai-orchestrator

### Current State
`lucind-ai` already has most of the runtime primitives required for deterministic delegated execution. The CLI resolves the primary repository root, validates explicit feature targets, exposes lifecycle/reconciliation/integration commands, and supports cleanup with an explicit force boundary (`cmd/lucind-ai/cli.go:791-815,946-979,1955-1984`). Runtime execution persists frozen candidate identity and result evidence before acceptance (`internal/run/run.go:608-665,1004-1019`; `internal/ledger/acceptance.go:20-32,55-105`). Acceptance revalidates hashes, commits, trees, result envelopes, declared changes, required skills, write/read-only obligations, and scope (`internal/accept/accept.go:213-341`).

The DAG layer already computes deterministic waves and rejects unordered path overlap (`internal/dag/waves.go:11-18,43-66`; `internal/dag/overlap.go:10-15,52-67`). Feature integration uses explicit target identity and CAS promotion, while retry reconstructs the same completed evidence without redispatch (`internal/run/integrate_feature.go:13-48`; `internal/run/integrate_retry.go:16-43`). These are strong foundations, so the proposed change should standardize orchestration and preflight rather than introduce another scheduler, lifecycle state, or promotion mechanism.

The skill layer is currently authoritative in `plugin/claude-code/skills/lucind-ai/SKILL.md`. It requires version checks, primary-root execution, explicit Change/target/scope/mode/strategy identification, modular decision-gate loading, and evidence-based acceptance (`SKILL.md:12-27,29-48,50-59`). OpenCode ships an exact copy of this skill and exposes only a native `lucind_ai` argv tool; orchestration remains in the skill and the Go binary remains the source of truth (`plugin/opencode/README.md:3-7,17-20`; `plugin/opencode/lucind-ai.ts:4-16`). The installer protects unrelated files through an ownership marker and refuses unsafe overwrites (`plugin/opencode/install.sh:27-60`).

OpenSpec already covers individual capabilities—apply waves, target-free packet authoring, parent integration, acceptance, mechanical checks, cleanup, lane lifecycle, orphan reconciliation, planning fan-out, and telemetry—but there is no single capability defining a deterministic cross-runtime orchestrator preflight and execution contract. Existing specs also contain useful contract details that must be preserved, such as late target binding and feature-parent CAS semantics (`openspec/specs/packet-authoring-contract/spec.md:5-30`; `openspec/specs/parent-feature-integration/spec.md:5-35`; `openspec/specs/sdd-apply/spec.md:1-30`).

### Affected Areas
- `plugin/claude-code/skills/lucind-ai/SKILL.md` — strengthen the canonical orchestrator contract: explicit preflight, phase routing, wave barriers, recovery/reconciliation, acceptance, and promotion evidence.
- `plugin/claude-code/skills/lucind-ai/references/core/safety.md` — make root, target, baseline, scope, stale-binary, and mutation checks deterministic and fail-closed.
- `plugin/claude-code/skills/lucind-ai/references/strategies/sdd.md` — define the complete SDD phase sequence and the exact transition from planning synthesis to apply waves, verify, and archive.
- `plugin/claude-code/skills/lucind-ai/references/coordination/recovery-reconciliation.md` — turn retry, stale leases, orphan worktrees, and partial-wave recovery into explicit decision paths.
- `plugin/claude-code/skills/lucind-ai/references/contracts/packets-results.md` and `acceptance-promotion.md` — align packet/result obligations with the preflight and terminal acceptance evidence.
- `plugin/opencode/skills/lucind-ai/SKILL.md` — preserve byte-for-byte parity with the canonical skill and add verification that parity is not silently lost.
- `plugin/opencode/lucind-ai.ts`, `plugin/opencode/process.mjs`, and `plugin/opencode/install.sh` — preserve the thin native wrapper, shell-free argv execution, cancellation, ownership protection, and safe installation behavior.
- `internal/packet/`, `internal/run/`, `internal/dag/`, `internal/ledger/`, and `internal/accept/` — implement only runtime gaps exposed by the contract; reuse existing packet admission, wave, frozen-evidence, and acceptance primitives.
- `cmd/lucind-ai/cli.go` — add or refine deterministic preflight/reporting only where the skill cannot reliably observe runtime truth; retain existing explicit target and integration boundaries.
- `openspec/specs/` — add a focused capability spec and modify existing specs only when the new contract changes their authoritative behavior.
- `plugin/opencode/test/`, `internal/*/*_test.go`, and likely new cross-surface contract tests — prove parity, argv/cancellation safety, schema/evidence determinism, concurrency, and recovery behavior.

### Approaches
1. **Skill-only orchestration contract** — encode all preflight, sequencing, and recovery rules in `SKILL.md` and references, leaving the Go runtime unchanged.
   - Pros: small implementation surface; works in both runtimes through the copied skill; easy to iterate on wording.
   - Cons: cannot make filesystem, binary, SQLite, schema, or ledger facts deterministic; risks repeating the observed stale-binary, consumer-test, and recovery failures in prompt prose.
   - Effort: Low

2. **Two-layer deterministic contract** — make the shared skill an explicit orchestration state machine and add narrowly scoped Go/plugin enforcement and reporting for facts that must be machine-checked; keep Claude and OpenCode wrappers thin and parity-tested.
   - Pros: separates human-approved strategy from machine-enforced invariants; reuses existing runtime evidence and CAS boundaries; supports fail-closed preflight, reproducible wave barriers, and actionable recovery across both runtimes.
   - Cons: requires coordinated skill, CLI/runtime, plugin, and consumer-test changes; needs careful schema/version compatibility and migration planning.
   - Effort: Medium/High

### Recommendation
Choose the two-layer deterministic contract. Keep `plugin/claude-code/skills/lucind-ai/` as the canonical authored skill because the current OpenCode distribution explicitly depends on an exact copy, then verify parity as a release/install invariant. The skill should decide the approved Change, mode, strategy, phase, scope, and recovery branch; the Go binary should verify observable facts such as binary/schema version, canonical root, packet admission, target/base identity, deterministic wave ordering, frozen evidence, and acceptance bindings. Add no duplicate lifecycle or scheduler model. The proposal should define one explicit preflight report and one terminal evidence path consumed identically by Claude Code and OpenCode.

The implementation should be split into independently testable slices: first contract/preflight and parity tests, then runtime enforcement and evidence changes, then recovery/reporting improvements. Each slice must use the repository's strict commands from `openspec/config.yaml:19-29` and preserve rollback compatibility with existing packets and ledger rows.

### Risks
- A stale installed `lucind-ai` binary or copied skill can make the orchestrator appear compliant while executing an older contract; version and content-hash checks must fail closed.
- SQLite WAL and multi-connection behavior can still expose lock or stale-projection races; real-SQLite race tests are required rather than in-memory-only tests (`internal/ledger/ledger.go:165-179`).
- Late target binding is necessary for reusable packets, but premature target resolution or hardcoded `legacy_main` can silently route work to the wrong parent.
- Adding checks only to the Claude skill or only to the OpenCode wrapper would create cross-runtime drift; parity and consumer tests must be owned by the change.
- Existing acceptance evidence is immutable and hash-bound; changing schemas or contract fields without a migration/version strategy will invalidate previously persisted candidates and receipts.
- Retry and cleanup changes can accidentally redispatch work, delete preserved evidence, or orphan worktrees; the existing no-redispatch and explicit-force boundaries must remain intact.
- The active branch contains unrelated feature work and must not be treated as the target Change; this exploration remains read-only except for the new artifact.

### Ready for Proposal
Yes. The orchestrator should propose the two-layer approach with a narrow capability boundary: deterministic preflight and cross-runtime contract parity, machine enforcement for runtime-observable invariants, and explicit recovery/evidence semantics. It should state exact call sites and schema/version migrations before design, preserve the existing lifecycle primitives, and include a rollback plan for skill, plugin, and ledger compatibility.
