# Proposal Lens A — Candidate & Approach: Agentic Phase Specialist

## Selected Candidate & Approach

The SDD planning phases (explore, propose, design, spec, tasks) run a 3-lens fan-out and synthesis topology through `lucind-ai` (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47`), but currently the top-level Orchestrator reads raw synthesis notes and directly arbitrates contradictions and Lane Acceptance. This design inflates the Orchestrator's context window with full Lane evidence across multi-phase Changes (`docs/sdd-phase-specialist.md:7-9`).

We select the **Phase-Scoped Agentic Specialist** candidate (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8`, `docs/sdd-phase-specialist.md:21-30`). The existing `sdd-*` subagents (`sdd-explore` through `sdd-archive`) are reconfigured from direct single-author executors into phase Specialists (`CONTEXT.md:103-106`). The Specialist coordinates its phase's fan-out and synthesis dispatch, reviews synthesis notes, arbitrates contradictions, and independently accepts its phase's Lanes without human confirmation (`CONTEXT.md:103-106`).

The approach consists of four foundational mechanics:

1. **Phase Verdict Protocol**: The Specialist reports only a compressed **Phase Verdict** (`CONTEXT.md:107-109`) to the Orchestrator containing the outcome, the canonical artifact path, and unresolved divergence. Raw result envelopes, diffs, and synthesis notes remain encapsulated within the Specialist context (`docs/sdd-phase-specialist.md:25`).
2. **Tool-Constrained Dispatch Flow**: Existing `sdd-*` subagents possess read, edit, search, and memory tools, but lack Bash and Task/Agent dispatch capabilities. In the near-term bootstrapping workflow, the Specialist authors lens and synthesis packet contents and evaluates resulting synthesis notes, while the Orchestrator mechanically triggers `lucind-ai run` dispatches. Future unassisted autonomous dispatch will require providing the Specialist with an execution tool bridge.
3. **Scoped Check Gating (`lucind-checks.sh`)**: `internal/integrate.Check()` currently executes `lucind-checks.sh` unconditionally across all worktrees (`internal/integrate/integrate.go:159-176`, `internal/run/attempt.go:431-435`). In `internal/accept/accept.go:84-96`, `LaneMetadata` (`internal/ledger/lanes_meta.go:20-33`) is loaded only during authoring evidence validation. We widen this load so `accept.go` checks `metadata.SDDPhase == "apply"` before invoking `integrate.CheckPolicySnapshot()` and `v.check()` (`internal/accept/accept.go:120-126`). Planning phases writing exclusively to `openspec/changes/**` accept on qualitative criteria (schema, done criteria, allowed paths, diff review) without executing the full Go test suite. Scope validation via `allowed_paths` remains unconditional.
4. **Planning Path Dogfooding**: SDD planning phases formalize and dogfood the 3-lens fan-out and synthesis pattern administered by the phase Specialist, establishing a uniform execution model across planning and implementation.

## Conceptual Changes & Architecture Rationale

- **Superseding Deterministic Specialist**: Supersedes the non-agentic `internal/phasespec.Adapter` (`internal/phasespec/phasespec.go:338-350`, `cmd/lucind-ai/cli.go:2517-2649`, `openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/proposal.md:188`). `CLIStatusQuerier.QueryStatus` (`internal/phasespec/phasespec.go:308-333`) and packet templating mechanics are retained as internal tools, but decision authority transfers to the agentic Specialist.
- **Acceptance vs. Promotion Authority**: Acceptance authority is delegated to the phase Specialist for its scoped Lanes without human confirmation (`CONTEXT.md:103-106`, `docs/sdd-phase-specialist.md:22-24`). Promotion remains change-scoped and strictly human-confirmed at the end of the entire SDD lifecycle (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`). No agent or Specialist holds Promotion authority.
- **Hard Rule Carve-Out**: Carves out an explicit exception in `plugin/claude-code/skills/lucind-ai/SKILL.md:19` and `plugin/opencode/skills/lucind-ai/SKILL.md:19` ("Agents own Lanes, not... Acceptance, or Promotion") allowing phase-scoped Specialist Acceptance while keeping Promotion forbidden to all agents.
- **Contract and Strategy Updates**:
  - `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47` and `plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47`: Moves synthesis note review and contradiction arbitration from Orchestrator to Specialist.
  - `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36`: Upgrades Acceptance Subagent delegation from evidence collection to decision-bearing Specialist Acceptance for planning phases.
- **Bounded Correction Protocol**: If an Orchestrator rejects a Phase Verdict, it executes a single bounded correction transaction rather than restarting a full phase fan-out (`docs/sdd-phase-specialist.md:26`).

## Alternatives Considered & Rejected

- **Evidence-Only Acceptance Delegate**: Extending the subagent delegation pattern in `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` without granting Acceptance authority. Rejected because returning full evidence to the Orchestrator perpetuates context exhaustion (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:11`).
- **Unconditional `lucind-checks.sh` Across All Phases**: Retaining unconditional check execution (`internal/integrate/integrate.go:159-176`, `internal/accept/accept.go:120-126`). Rejected because planning phases only modify documentation in `openspec/changes/**`, making full race-enabled Go test suites irrelevant and computationally wasteful (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:12`).
- **Deterministic Go Specialist as Sole Coordinator**: Keeping `internal/phasespec.Adapter` (`cmd/lucind-ai/cli.go:2517-2649`, `internal/phasespec/phasespec.go:338-350`) as the only specialist. Rejected because deterministic code cannot evaluate qualitative done criteria or arbitrate semantic contradictions across lens drafts.
- **Full Phase Relaunch on Verdict Disagreement**: Relaunching all three lens lanes upon Orchestrator disagreement. Rejected because single bounded correction transactions are faster, lower cost, and prevent circular fan-out churn (`docs/sdd-phase-specialist.md:26`).

## Open Questions

- [ ] What specific tool interface or CLI bridge will be provided in future iterations to allow `sdd-*` subagents to invoke `lucind-ai run` without Orchestrator mediation?
- [ ] Should Tier A Change planning phases require Dual-Judge evaluation for Specialist Acceptance, or should Dual-Judge remain restricted to apply and verify lanes?

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:103-106` | Defines the Specialist as a phase-scoped Agent with independent Acceptance authority |
| `CONTEXT.md:107-109` | Defines the Phase Verdict as the compressed summary returned to the Orchestrator |
| `cmd/lucind-ai/cli.go:2517-2649` | Implements phaseDispatch with deterministic phasespec Adapter wiring and packet templating |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | Records accepted architecture for Specialist authority, Phase Verdict, and scoped checks |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:11` | Documents rejection of evidence-only delegate due to Orchestrator context costs |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:12` | Documents rejection of running unconditional checks on planning lanes |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-17` | Notes required Hard Rule carve-out and fan-out reference update |
| `docs/sdd-phase-specialist.md:7-9` | Identifies Orchestrator context inflation from reading full synthesis notes |
| `docs/sdd-phase-specialist.md:13-17` | Summarizes existing deterministic Adapter, fan-out strategy, and unconditional checks |
| `docs/sdd-phase-specialist.md:21-30` | Details resolved decisions on Specialist role, Acceptance authority, Phase Verdict, and check gating |
| `internal/accept/accept.go:84-96` | Shows GetLaneMetadata called conditionally for authoring evidence validation |
| `internal/accept/accept.go:120-126` | Shows CheckPolicySnapshot and v.check executed unconditionally without phase gating |
| `internal/integrate/integrate.go:159-176` | Shows Check executing lucind-checks.sh unconditionally across all worktrees |
| `internal/ledger/lanes_meta.go:20-33` | Defines LaneMetadata struct containing SDDPhase field |
| `internal/phasespec/phasespec.go:308-333` | Implements CLIStatusQuerier querying external gentle-ai sdd-status CLI |
| `internal/phasespec/phasespec.go:338-350` | Implements deterministic Adapter struct without agentic reasoning capabilities |
| `internal/run/attempt.go:431-435` | Shows default assignment of integrate.Check in attempt execution workflow |
| `openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/proposal.md:188` | Documents archived decision rejecting tool access for packet-author specialist |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19` | Specifies Hard Rule forbidding agents from owning Acceptance or Promotion |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Defines evidence-only Acceptance Subagent delegation pattern |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | Defines Promotion gate as strictly human-confirmed |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47` | Assigns synthesis note reading and contradiction arbitration to Orchestrator |
| `plugin/opencode/skills/lucind-ai/SKILL.md:19` | Mirrors Hard Rule forbidding agent Acceptance ownership in OpenCode skill tree |
| `plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47` | Mirrors Orchestrator assignment of synthesis note arbitration in OpenCode skill tree |
