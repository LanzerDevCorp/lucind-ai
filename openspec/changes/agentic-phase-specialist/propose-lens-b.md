# Proposal Lens B — Capability Impact & Specs: Agentic Phase Specialist

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `phase-specialist-dispatch` | Modified | Replace deterministic Go adapter with agentic `sdd-*` subagent Specialist owning phase execution strategy, packet authoring, synthesis review, and independent Lane Acceptance. | `openspec/specs/phase-specialist-dispatch/spec.md:9-11`, `CONTEXT.md:103-106`, `docs/sdd-phase-specialist.md:13-18`, `docs/sdd-phase-specialist.md:21-30` |
| `acceptance-verifier` | Modified | Gate `lucind-checks.sh` in `internal/accept` and `internal/integrate` on `LaneMetadata.SDDPhase == "apply"`, skipping redundant Go check runs for non-apply planning and audit lanes. | `internal/accept/accept.go:84-96`, `internal/accept/accept.go:125-137`, `internal/integrate/integrate.go:159-176`, `internal/ledger/lanes_meta.go:20-47`, `openspec/specs/acceptance-verifier/spec.md:30-33` |
| `sdd-planning-fan-out` | Modified | Transfer synthesis note review, contradiction arbitration, and lens Acceptance from Orchestrator to phase Specialist, restricting Orchestrator intake to compressed Phase Verdicts. | `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `openspec/specs/sdd-planning-fan-out/spec.md:9-12` |
| `acceptance-promotion` | Modified | Promote Acceptance Subagent from evidence gatherer to decision-maker scoped to phase Specialists, carving out Hard Rule line 19 while keeping Promotion strictly human-confirmed. | `plugin/claude-code/skills/lucind-ai/SKILL.md:18-19`, `plugin/opencode/skills/lucind-ai/SKILL.md:18-19`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:16-30`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`, `CONTEXT.md:51-53`, `CONTEXT.md:91-93` |
| `phase-verdict-reporting` | Added | Establish structured Phase Verdict returned from Specialist to Orchestrator encapsulating outcome status, canonical artifact path, and unresolved divergence. | `CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30`, `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-7` |

## Delta Specifications

### Requirement: Specialist Phase Acceptance and Authority Carve-Out

The phase Specialist (`sdd-*` subagent) MUST have decision authority to evaluate evidence and execute Acceptance (`lucind-ai accept`) for Lanes in its assigned SDD phase without human confirmation (`CONTEXT.md:51-53`, `CONTEXT.md:103-106`, `docs/sdd-phase-specialist.md:21-30`). The Hard Rule prohibiting Agent acceptance (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-19`, `plugin/opencode/skills/lucind-ai/SKILL.md:18-19`) MUST carve out phase-scoped Specialists. Promotion of a Change to its Integration Target MUST remain human-confirmed (`CONTEXT.md:91-93`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`, `openspec/specs/acceptance-verifier/spec.md:124-127`). For Tier A Changes, Specialist Acceptance MUST enforce the Dual-Judge requirement (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43`).

#### Scenario: Specialist independently accepts phase planning lanes

- GIVEN a completed planning lane candidate with valid schema, passing qualitative checks, and clean scope
- WHEN the phase Specialist executes the acceptance protocol (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:16-30`)
- THEN the Specialist MUST issue the acceptance receipt without requesting human confirmation

#### Scenario: Specialist prohibited from executing change promotion

- GIVEN a completed SDD Change with all phase artifacts accepted by Specialists
- WHEN the Change is ready for integration into its Integration Target
- THEN Promotion MUST require explicit human confirmation (`CONTEXT.md:91-93`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`) and MUST NOT be executed by any Specialist

#### Scenario: Non-specialist agent denied acceptance authority

- GIVEN an ordinary delegated worker executing a lane
- WHEN the lane reaches completion
- THEN the worker MUST NOT judge acceptance or execute `lucind-ai accept` (`plugin/claude-code/skills/lucind-ai/SKILL.md:18-19`)

### Requirement: Structured Phase Verdict Reporting

Upon completing phase synthesis review or encountering blockers, the Specialist MUST return a structured Phase Verdict to the Orchestrator (`CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30`). The Phase Verdict MUST specify `outcome` (`accepted` or `needs-revision`), `canonical_artifact_path`, and `unresolved_divergence` (`CONTEXT.md:107-109`). The Specialist MUST NOT transmit raw result envelopes, line diffs, or verbose synthesis notes to the Orchestrator unless requested (`docs/sdd-phase-specialist.md:21-30`). When the Orchestrator receives outcome `needs-revision`, it MUST dispatch a single bounded correction lane rather than re-running the full phase fan-out (`docs/sdd-phase-specialist.md:21-30`).

#### Scenario: Successful phase completion returns compressed verdict

- GIVEN a synthesized phase artifact accepted by the Specialist
- WHEN the Specialist reports to the Orchestrator
- THEN the Specialist MUST deliver a Phase Verdict with outcome `accepted`, the artifact path, and empty divergence without raw lane transcripts

#### Scenario: Synthesis divergence reported in verdict

- GIVEN unresolvable contradictions between lens drafts during phase synthesis
- WHEN the Specialist prepares the Phase Verdict
- THEN the verdict MUST designate outcome `needs-revision` and populate `unresolved_divergence`

#### Scenario: Orchestrator triggers bounded correction on revision verdict

- GIVEN an Orchestrator receiving a Phase Verdict with outcome `needs-revision`
- WHEN the Orchestrator handles the revision
- THEN the Orchestrator MUST dispatch exactly one bounded correction lane targeting the divergence

### Requirement: SDD Phase-Gated Verification Check Execution

Mechanical acceptance in `internal/accept` and integration checking in `internal/integrate` MUST gate `lucind-checks.sh` execution on `LaneMetadata.SDDPhase` (`internal/ledger/lanes_meta.go:20-47`, `internal/accept/accept.go:84-96`, `internal/accept/accept.go:125-137`, `internal/integrate/integrate.go:159-176`, `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-7`). When `SDDPhase` is `"apply"` or an explicit check override is provided, `internal/accept` MUST execute `lucind-checks.sh` (`internal/accept/accept.go:125-137`). For non-apply phases (`explore`, `propose`, `design`, `spec`, `tasks`, `verify`, `archive`), `internal/accept` MUST skip `lucind-checks.sh` and validate the candidate against schema, hard stops, done criteria, and `allowed_paths` scope (`openspec/specs/acceptance-verifier/spec.md:30-33`, `docs/sdd-phase-specialist.md:21-30`).

#### Scenario: Apply phase executes full verification suite

- GIVEN a lane candidate declaring `sdd_phase: apply`
- WHEN `internal/accept` verifies the candidate commit
- THEN `internal/accept` MUST execute `lucind-checks.sh` in the isolated worktree (`internal/accept/accept.go:125-137`)

#### Scenario: Planning phase skips full verification suite

- GIVEN a planning lane candidate declaring a non-apply phase (e.g. `propose` or `design`)
- WHEN `internal/accept` verifies the candidate commit
- THEN `internal/accept` MUST skip `lucind-checks.sh` and issue the receipt upon satisfying qualitative and scope criteria

#### Scenario: Explicit check override runs checks for non-apply lane

- GIVEN a non-apply lane candidate dispatched with an explicit check override flag
- WHEN `internal/accept` verifies the candidate
- THEN `internal/accept` MUST execute `lucind-checks.sh` despite the non-apply phase declaration

### Requirement: Specialist-Owned Synthesis Arbitration

The phase Specialist MUST inspect synthesis notes, arbitrate unresolved lens contradictions, and verify canonical artifact citations before accepting the phase artifact (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `openspec/specs/sdd-planning-fan-out/spec.md:9-12`, `docs/sdd-phase-specialist.md:21-30`). The top-level Orchestrator SHALL NOT read full synthesis notes or arbitrate lens contradictions during normal execution (`docs/sdd-phase-specialist.md:21-30`).

#### Scenario: Specialist arbitrates contradictions in synthesis notes

- GIVEN synthesis notes identifying conflicting proposals between lenses
- WHEN the Specialist reviews the synthesis lane result
- THEN the Specialist MUST arbitrate the contradiction and determine whether to accept or mark `needs-revision`

#### Scenario: Synthesis notes withheld from orchestrator context

- GIVEN an accepted synthesis lane producing canonical artifacts and detailed synthesis notes
- WHEN the Specialist concludes the phase
- THEN full synthesis notes MUST remain with the Specialist and not inflate the Orchestrator transcript

## Open Questions

- [ ] Whether `lucind-ai accept` CLI should expose an explicit `--force-checks` flag to override phase-gated skipping during manual operator audits.
- [ ] Whether the propose phase skill contract's standard whole-document generation instructions need an explicit note acknowledging asymmetric precedence when running in multi-lens fan-out lanes.

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:51-53` | Defines Acceptance as verified inclusion of a Lane's result into its Change without human confirmation. |
| `CONTEXT.md:91-93` | Defines Promotion as the human-confirmed integration of a completed Change into its Integration Target. |
| `CONTEXT.md:103-106` | Defines Specialist as a phase-scoped Agent that owns an SDD phase's strategy and accepts its own Lanes. |
| `CONTEXT.md:107-109` | Defines Phase Verdict as the compressed report returned from Specialist to Orchestrator. |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-7` | Records decision to give Specialists phase Acceptance authority and gate `lucind-checks.sh` on `sdd_phase: apply`. |
| `docs/sdd-phase-specialist.md:13-18` | Identifies existing deterministic phase adapter and unconditional check execution in `integrate.Check()`. |
| `docs/sdd-phase-specialist.md:21-30` | Details resolved decisions for agentic Specialist substrate, Acceptance carve-out, Phase Verdict structure, and check gating. |
| `internal/accept/accept.go:84-96` | Shows `GetLaneMetadata` loaded during authoring evidence validation without using `SDDPhase` for check gating. |
| `internal/accept/accept.go:125-137` | Executes `v.check` unconditionally on candidate worktrees during mechanical acceptance verification. |
| `internal/integrate/integrate.go:159-176` | Defines `Check` function running `lucind-checks.sh` unconditionally when present. |
| `internal/ledger/lanes_meta.go:20-47` | Declares `LaneMetadata` struct containing the `SDDPhase` field (`sdd_phase`). |
| `openspec/specs/acceptance-verifier/spec.md:30-33` | Specifies fail-closed mechanical criteria and check execution requirements for candidate acceptance. |
| `openspec/specs/acceptance-verifier/spec.md:124-127` | Specifies that acceptance must not mutate refs or invoke human Promotion. |
| `openspec/specs/phase-specialist-dispatch/spec.md:9-11` | Specifies deterministic phase specialist sequencing and canonical artifact generation superseded by agentic specialist. |
| `openspec/specs/sdd-planning-fan-out/spec.md:9-12` | Specifies two-wave planning fan-out protocol previously assigning orchestration and synthesis review to the orchestrator. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:18-19` | Contains the Hard Rule stating Agents own Lanes but not Acceptance or Promotion. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:16-30` | Defines the 10-step Acceptance protocol and checklist. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Defines Acceptance Subagent delegation pattern currently restricted to evidence gathering. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43` | Defines Dual-Judge acceptance requirement for Tier A Changes. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | Defines human-confirmed Promotion gate for completed Changes. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Assigns reading synthesis notes and arbitrating contradictions directly to the Orchestrator. |
| `plugin/opencode/skills/lucind-ai/SKILL.md:18-19` | Mirrors the Hard Rule stating Agents own Lanes but not Acceptance or Promotion in OpenCode skill tree. |
