# Spec Lens A — Capabilities & Requirements: Agentic Phase Specialist

## Assumed requirements

Per the accepted proposal (`openspec/changes/agentic-phase-specialist/proposal.md:26-35`), this specification establishes four capabilities across the Agentic Phase Specialist architecture: one New capability (`phase-verdict-reporting`) and three Modified capabilities (`phase-specialist-dispatch`, `acceptance-verifier`, and `sdd-planning-fan-out`). Each capability introduces exactly one requirement statement (totaling four ADDED requirements). Together, they specify independent phase-scoped Acceptance authority and Hard Rule carve-outs, compressed structured Phase Verdict reporting, phase-gated mechanical verification check execution, and Specialist-owned synthesis note arbitration. No requirements are modified in-place, removed, or renamed.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `phase-verdict-reporting` | New | `openspec/specs/phase-verdict-reporting/spec.md` | — |
| `phase-specialist-dispatch` | Existing | `openspec/changes/agentic-phase-specialist/specs/phase-specialist-dispatch/spec.md` | `openspec/specs/phase-specialist-dispatch/spec.md:1-29` |
| `acceptance-verifier` | Existing | `openspec/changes/agentic-phase-specialist/specs/acceptance-verifier/spec.md` | `openspec/specs/acceptance-verifier/spec.md:1-167` |
| `sdd-planning-fan-out` | Existing | `openspec/changes/agentic-phase-specialist/specs/sdd-planning-fan-out/spec.md` | `openspec/specs/sdd-planning-fan-out/spec.md:1-107` |

## ADDED Requirements

### Requirement: Specialist Phase Acceptance and Authority Carve-Out

The phase Specialist MUST independently decide Acceptance for Lanes in its assigned phase without human confirmation, and MUST direct the Orchestrator to execute the corresponding `lucind-ai accept` invocation upon its decision (`CONTEXT.md:51-53,103-106`). The Hard Rule (`plugin/claude-code/skills/lucind-ai/SKILL.md:19`) MUST carve out named `sdd-*` Specialists for Acceptance of their own phase's Lanes only. Ordinary delegated workers and non-specialist agents MUST NOT decide Acceptance or direct acceptance execution. Promotion MUST remain human-confirmed and MUST NOT be decided or requested by any Specialist or delegated worker (`CONTEXT.md:91-93`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`, `openspec/specs/acceptance-verifier/spec.md:124-127`). For Tier A Changes, Specialist Acceptance decisions MUST enforce Dual-Judge evaluation before acceptance (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43`).

**Terminal consumer**: `plugin/claude-code/skills/lucind-ai/SKILL.md:19`

### Requirement: Structured Phase Verdict Reporting

Upon phase completion, the Specialist MUST return a structured Phase Verdict reporting `outcome` (`accepted` or `needs-revision`), `canonical_artifact_path`, and `unresolved_divergence` to the Orchestrator (`CONTEXT.md:107-109`). The Specialist MUST NOT include raw result envelopes, candidate diffs, or full synthesis notes in the verdict report unless explicitly requested by the Orchestrator (`docs/sdd-phase-specialist.md:21-30`). On receipt of a `needs-revision` verdict, the Orchestrator MUST dispatch at most one bounded correction transaction rather than initiating a full phase re-fan-out.

**Terminal consumer**: `new, introduced by this change`

### Requirement: SDD Phase-Gated Verification Check Execution

Lane acceptance verification (`internal/accept`) and attempt execution (`internal/run`) MUST gate the execution of `lucind-checks.sh` on `LaneMetadata.SDDPhase` (`internal/ledger/lanes_meta.go:20-47`, `internal/accept/accept.go:84-96,120-137`, `internal/run/attempt.go:431-435`). The verifier MUST execute `lucind-checks.sh` when `SDDPhase` equals `"apply"`, when `sdd_phase` is empty or missing, or when an explicit check exception is configured; for all other declared SDD phases, the verifier MUST skip `lucind-checks.sh` while continuing to enforce result schema validation, hard stops, done criteria, and `allowed_paths` scope constraints (`openspec/specs/acceptance-verifier/spec.md:30-33`). `integrate.Check` MUST remain an ungated verification primitive at its own definition (`internal/integrate/integrate.go:159-200`).

**Terminal consumer**: `internal/accept/accept.go:84-96,120-137`

### Requirement: Specialist-Owned Synthesis Arbitration

The phase Specialist MUST inspect synthesis notes, arbitrate unresolved contradictions across lens drafts, and verify canonical citations prior to deciding phase acceptance (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `openspec/specs/sdd-planning-fan-out/spec.md:9-12`). Full synthesis notes and raw contradiction details MUST remain with the Specialist, and the top-level Orchestrator SHALL NOT inspect raw synthesis notes or perform contradiction arbitration during normal phase execution.

**Terminal consumer**: `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:51-53` | Acceptance definition confirms lane inclusion can occur without human confirmation |
| `CONTEXT.md:91-93` | Promotion definition confirms integration into target remains human-confirmed |
| `CONTEXT.md:103-106` | Specialist definition establishes phase-scoped agent with lane acceptance authority |
| `CONTEXT.md:107-109` | Phase Verdict definition specifies compressed outcome, path, and divergence reporting |
| `docs/sdd-phase-specialist.md:21-30` | Resolved decisions define Specialist runtime, verdict contents, bounded relaunch, and check gating |
| `internal/accept/accept.go:84-96` | LaneMetadata loading and target binding validation in acceptance verifier |
| `internal/accept/accept.go:120-137` | Check execution and policy snapshot in verifier owned isolation |
| `internal/integrate/integrate.go:159-200` | Check function executes lucind-checks.sh as an ungated primitive |
| `internal/ledger/lanes_meta.go:20-47` | LaneMetadata struct definition containing SDDPhase field at line 25 |
| `internal/run/attempt.go:431-435` | Attempt execution resolves checkFunc defaulting to integrate.Check |
| `openspec/changes/agentic-phase-specialist/proposal.md:26-35` | Proposal Capabilities section defines phase-verdict-reporting, phase-specialist-dispatch, acceptance-verifier, and sdd-planning-fan-out |
| `openspec/changes/agentic-phase-specialist/proposal.md:65-132` | Proposal Delta Specifications section defines draft requirements across the four capabilities |
| `openspec/specs/acceptance-verifier/spec.md:1-167` | Live specification root for existing acceptance-verifier capability |
| `openspec/specs/acceptance-verifier/spec.md:30-33` | Live requirement block for Fail-Closed Mechanical Criteria |
| `openspec/specs/acceptance-verifier/spec.md:124-127` | Live requirement block for No Promotion Authority |
| `openspec/specs/phase-specialist-dispatch/spec.md:1-29` | Live specification root for existing phase-specialist-dispatch capability |
| `openspec/specs/phase-specialist-dispatch/spec.md:9-12` | Live requirement block for Specialist sequencing and canonical artifact generation |
| `openspec/specs/sdd-planning-fan-out/spec.md:1-107` | Live specification root for existing sdd-planning-fan-out capability |
| `openspec/specs/sdd-planning-fan-out/spec.md:9-12` | Live requirement block for Two-Wave Planning Fan-Out Protocol |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19` | Hard Rule mandating Orchestrator authority and forbidding Agent ownership of Acceptance and Promotion |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43` | Dual-Judge acceptance protocol required for Tier A Changes |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | Promotion gate requiring explicit human confirmation |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Planning fan-out synthesis protocol specifying synthesis notes review and contradiction arbitration |
