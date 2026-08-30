# Design Lens A — Decisions: Agentic Phase Specialist

## Assumed architecture

The Go engine changes in `internal/accept/accept.go` and `internal/run/attempt.go` to unconditionally load `LaneMetadata` and skip `lucind-checks.sh` for non-apply planning phases (`sdd_phase != "apply"`), preserving unconditional result schema, hard stops, and `allowed_paths` validation. Outside Go code, `SKILL.md` (both trees) gains a Hard Rule carve-out for named `sdd-*` Specialist Acceptance, `fan-out.md` and `acceptance-promotion.md` transfer synthesis-note review and Acceptance judgment to the Specialist, and `sdd-*` subagents orchestrate their phase. Deterministic infrastructure (`internal/phasespec.Adapter`, `CLIStatusQuerier`, `phaseDispatch`), the ungated `integrate.Check` primitive, and human-only Promotion remain completely unchanged.

## Technical Approach

We implement the Phase-Scoped Agentic Specialist by mapping the proposal and four spec capabilities to decoupled seams:

1. **`phase-verdict-reporting`**: The Specialist encapsulates raw Lane artifacts and diffs, returning a concise structured markdown Phase Verdict (`outcome`, `canonical_artifact_path`, `unresolved_divergence`) to the Orchestrator (`CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30`).
2. **`phase-specialist-dispatch`**: Synthesis dispatch remains gated on all required planning lenses being accepted and merged (`openspec/specs/phase-specialist-dispatch/spec.md:9-12`). The agentic `sdd-*` subagent authors packets, while `internal/phasespec.Adapter` (`phasespec.go:338-350`) and `cmd/lucind-ai/cli.go:2517-2649` remain callable as deterministic tools.
3. **`acceptance-verifier`**: `internal/accept/accept.go:84-137` loads `LaneMetadata` (`lanes_meta.go:20-47`) unconditionally and bypasses `integrate.CheckPolicySnapshot` / `v.check` for non-apply phases, while enforcing schema, hard stops, and `allowed_paths` (`accept.go:214-261`). `internal/run/attempt.go:431-435` mirrors this gate.
4. **`sdd-planning-fan-out`**: Synthesis-note review and contradiction arbitration move to the Specialist (`fan-out.md:47-48`, `openspec/specs/sdd-planning-fan-out/spec.md:9-12`), with `needs-revision` triggering one bounded correction (`docs/sdd-phase-specialist.md:21-30`).

## Decision 1 — Phase Verdict Format: Structured Markdown Section

**Choice**: Format the Phase Verdict as a structured markdown section in the Specialist subagent's chat response (`Outcome: accepted | needs-revision`, `Canonical Artifact: <path>`, `Unresolved Divergence: <text>`), rather than a new JSON schema under `internal/result/`.
**Alternatives considered**:
- JSON schema and struct in `internal/result/result.go`: Rejected because `internal/result/` models isolated worktree `.lucind/result.json` envelopes (`result.go:1-12`), whereas the Specialist is a conversation-level subagent without Go binary invocations.
- Free-form chat response: Rejected because the Orchestrator needs deterministic parsing of outcomes and divergence.
**Rationale**: `sdd-*` Specialists are runtime subagents requiring no Go code changes in this Change (`openspec/changes/agentic-phase-specialist/proposal.md:18-19`). Structured markdown fulfills the contract (`CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30`) without polluting `internal/result/` or bumping schema versions.
**Terminal consumer**: Top-level Orchestrator conversation evaluating subagent completion (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `docs/sdd-phase-specialist.md:21-30`).

## Decision 2 — SDD Phase Check-Gating Insertion Points

**Choice**: In `internal/accept/accept.go:84-96`, lift `GetLaneMetadata` out of the conditional `AuthoringEvidenceVersion` block to load `LaneMetadata` unconditionally. In `internal/accept/accept.go:120-137`, gate `integrate.CheckPolicySnapshot()` and `v.check()` to execute only when `metadata.SDDPhase == "apply"`, when `sdd_phase` is empty/missing, or on explicit check exceptions. In `internal/run/attempt.go:431-435`, mirror this gate before `checkFunc(ctx, wtPath)`. Leave `integrate.Check` in `internal/integrate/integrate.go:159-200` as an ungated primitive.
**Alternatives considered**:
- Gating inside `integrate.Check()`: Rejected because `integrate.Check` is a general execution primitive for `lucind-checks.sh` and should not know ledger metadata (`integrate.go:159-200`).
- Gating on `.go` file modifications: Rejected because diff heuristics are fragile; `SDDPhase` is explicitly declared at dispatch (`lanes_meta.go:20-47`).
- Unconditional `lucind-checks.sh`: Rejected because planning lanes modify only `openspec/changes/**`, making full Go test suites wasteful (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:11-12`).
**Rationale**: Unconditional metadata loading provides phase context for all candidates. Skipping Go suites on non-apply planning phases preserves velocity while fail-closed schema, hard stop, and `allowed_paths` verification (`accept.go:97-98,214-261`) ensure scope integrity.
**Terminal consumer**: `internal/accept/accept.go:120-137` and `internal/run/attempt.go:431-435`.

## Decision 3 — Hard Rule Carve-Out Scope and Promotion Boundary

**Choice**: In `plugin/claude-code/skills/lucind-ai/SKILL.md:18-21` and `plugin/opencode/skills/lucind-ai/SKILL.md:18-21`, update the Hard Rule to: *"Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion; named `sdd-*` Specialists may independently execute Acceptance for their own phase's Lanes only. Promotion remains strictly human-confirmed and forbidden to all Agents."*
**Alternatives considered**:
- Authorizing all agent subagents to accept lanes: Rejected because worker subagents lack cross-lane context and must not judge their own work (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:16-30`).
- Delegating Promotion to Specialists: Rejected because Promotion alters repository integration refs and must remain strictly human-confirmed (`CONTEXT.md:91-93`, `acceptance-promotion.md:44-50`).
- Updating only Claude Code skills: Rejected because contract tests enforce byte-identical parity across trees (`internal/packet/packet_test.go:943-967`).
**Rationale**: Explicitly restricts independent Acceptance authority to named `sdd-*` Specialists for their assigned phase's Lanes (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-18`), preserving the immutable rule that Promotion requires human confirmation (`acceptance-promotion.md:44-50`).
**Terminal consumer**: `plugin/claude-code/skills/lucind-ai/SKILL.md:18-21` and `internal/packet/packet_test.go:943-967`.

## Decision 4 — Tool-Constrained Dispatch Architecture and Adapter Coexistence

**Choice**: Retain `internal/phasespec.Adapter` (`internal/phasespec/phasespec.go:338-350`), `CLIStatusQuerier` (`phasespec.go:308-333`), and `phaseDispatch` (`cmd/lucind-ai/cli.go:2517-2649`) as deterministic tool helpers callable by operators and CLI scripts, while delegating phase coordination and packet authoring to the agentic `sdd-*` subagent.
**Alternatives considered**:
- Deleting `internal/phasespec.Adapter`: Rejected because `lucind-ai phase <name>` provides useful deterministic synthesis templating and status querying (`cli.go:2517-2649`).
- Direct tool grants (Bash/Agent) for `sdd-*` subagents: Rejected as out of scope for this Change (`proposal.md:18-19`, `openspec/changes/agentic-phase-specialist/explore.md:33-35`).
**Rationale**: Decoupling the deterministic adapter from agentic decision authority maintains CLI backwards compatibility while enabling `sdd-*` subagents to drive the lifecycle through packet authoring and Acceptance (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8`, `openspec/specs/phase-specialist-dispatch/spec.md:9-12`).
**Terminal consumer**: `cmd/lucind-ai/cli.go:2517-2649` and `openspec/specs/phase-specialist-dispatch/spec.md:9-12`.

## Decision 5 — Specialist-Owned Contradiction Arbitration and Bounded Correction

**Choice**: The Specialist directly reviews synthesis notes, arbitrates lens contradictions, and validates citations (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `openspec/specs/sdd-planning-fan-out/spec.md:9-12`). Persistent contradictions trigger a `needs-revision` Phase Verdict, resulting in exactly one bounded correction transaction rather than a full phase re-fan-out (`docs/sdd-phase-specialist.md:21-30`).
**Alternatives considered**:
- Escalating contradictions to Orchestrator: Rejected because loading raw lens trade-offs into the top-level conversation causes context exhaustion (`docs/sdd-phase-specialist.md:7-9`).
- Relaunching all three lenses on divergence: Rejected because a targeted single correction transaction resolves divergence without circular fan-out churn (`docs/sdd-phase-specialist.md:21-30`).
**Rationale**: Moving synthesis review to the Specialist encapsulates phase deliberations while preserving Tier A Dual-Judge qualitative requirements (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-18`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43`).
**Terminal consumer**: `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` and `docs/sdd-phase-specialist.md:21-30`.

## Open Questions

- [ ] What specific tool interface or CLI bridge in a subsequent Change will allow `sdd-*` Specialists to trigger `lucind-ai run` without Orchestrator mediation?
- [ ] Should `lucind-ai accept` provide an explicit `--force-checks` CLI flag for manual override of non-apply phase skipping, or is packet-level exception metadata sufficient?

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:91-93` | Defines Promotion as human-confirmed integration into an Integration Target |
| `CONTEXT.md:107-109` | Defines the Phase Verdict as the compressed summary returned to the Orchestrator |
| `cmd/lucind-ai/cli.go:2517-2649` | Implements phaseDispatch subcommand and synthesis packet templating |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | Records accepted architecture for Specialist authority and scoped checks |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:11-12` | Documents rejection of evidence-only delegate and unconditional checks |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-18` | Records Hard Rule carve-out, fan-out update, and Tier A Dual-Judge rules |
| `docs/sdd-phase-specialist.md:7-9` | Identifies Orchestrator context inflation from reading full Lane evidence |
| `docs/sdd-phase-specialist.md:21-30` | Details decisions on Specialist role, Acceptance authority, Phase Verdict, and bounded correction |
| `internal/accept/accept.go:84-96` | Shows LaneMetadata currently loaded conditionally inside AuthoringEvidenceVersion check |
| `internal/accept/accept.go:97-98` | Shows unconditional result and scope validation call before verification |
| `internal/accept/accept.go:120-137` | Shows CheckPolicySnapshot and v.check executed unconditionally |
| `internal/accept/accept.go:214-261` | Implements validateResultAndScope with fail-closed hard stop, done criteria, and allowed_paths checks |
| `internal/integrate/integrate.go:159-200` | Implements Check executing lucind-checks.sh unconditionally as an ungated primitive |
| `internal/ledger/lanes_meta.go:20-47` | Defines LaneMetadata struct containing SDDPhase field |
| `internal/packet/packet_test.go:943-967` | Tests that Claude Code and OpenCode skill trees are byte-identical |
| `internal/phasespec/phasespec.go:308-333` | Implements CLIStatusQuerier executing gentle-ai sdd-status --json |
| `internal/phasespec/phasespec.go:338-350` | Implements deterministic Adapter struct coordinating lens status and dispatch |
| `internal/result/result.go:1-12` | Documents result envelope package role reading .lucind/result.json in worktrees |
| `internal/run/attempt.go:431-435` | Shows default checkFunc assignment in attempt execution |
| `openspec/changes/agentic-phase-specialist/explore.md:33-35` | Documents archived rejection of tool access for packet-author specialist |
| `openspec/changes/agentic-phase-specialist/proposal.md:18-19` | Out of Scope: Bash/Agent tools for sdd-* in near term |
| `openspec/changes/agentic-phase-specialist/proposal.md:36-47` | Selected Candidate & Approach for Agentic Phase Specialist |
| `openspec/specs/phase-specialist-dispatch/spec.md:9-12` | Specifies specialist sequencing and lens merge preconditions for synthesis |
| `openspec/specs/sdd-planning-fan-out/spec.md:9-12` | Specifies two-wave planning fan-out protocol and synthesis arbitration |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:18-21` | Hard Rules including Orchestrator authority and byte-identical skill trees |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:16-30` | Details canonical 10-step Acceptance checklist |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43` | Specifies Dual-Judge requirements for Tier A Changes |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | Specifies human-confirmed Promotion gate |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Specifies synthesis note reading and contradiction arbitration |
| `plugin/opencode/skills/lucind-ai/SKILL.md:18-21` | OpenCode mirror of Hard Rules and Orchestrator authority |
