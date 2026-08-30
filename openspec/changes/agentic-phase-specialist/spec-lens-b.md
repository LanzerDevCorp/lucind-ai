# Spec Lens B — Scenarios & Coverage: Agentic Phase Specialist

## Assumed requirements

The agentic phase specialist capability introduces four core requirements derived from the frozen proposal. Phase execution and lane acceptance authority are delegated to named phase specialists with an explicit hard rule carve-out, while promotion remains strictly human-confirmed. Specialists report compressed phase verdicts to the orchestrator rather than raw lane transcripts, verification checks are gated by SDD phase metadata so non-apply phases skip mechanical suites, and synthesis arbitration is owned exclusively by the specialist.

## Scenarios

### Requirement: Specialist Phase Acceptance and Authority Carve-Out

#### Scenario: Specialist independently accepts phase planning lanes
- GIVEN a completed planning lane with a schema-valid result envelope, passing qualitative checks, and clean declared scope
- WHEN the assigned phase Specialist executes the acceptance protocol
- THEN an acceptance receipt is persisted in the ledger and the lane is accepted without requesting human confirmation

#### Scenario: Tier A change requires dual-judge evaluation for specialist acceptance
- GIVEN a completed lane in a Tier A Change evaluated independently by two distinct model architectures
- WHEN the two judges disagree on compliance or safety during acceptance evaluation
- THEN acceptance is blocked and the lane is not accepted until the evaluation divergence is resolved

#### Scenario: Ordinary worker agent denied acceptance authority
- GIVEN an ordinary delegated worker executing a lane
- WHEN the worker completes lane execution and attempts to issue an acceptance decision
- THEN acceptance authority is denied and the worker is prohibited from accepting the lane

#### Scenario: Specialist prohibited from executing change promotion
- GIVEN a completed Change with all required phase artifacts accepted
- WHEN the Change is ready for integration into its declared Integration Target
- THEN automated promotion is blocked and explicit human confirmation is required

### Requirement: Structured Phase Verdict Reporting

#### Scenario: Successful phase completion returns compressed verdict
- GIVEN an accepted phase synthesis producing a canonical planning artifact
- WHEN the Specialist reports phase completion to the Orchestrator
- THEN the Phase Verdict contains outcome `accepted`, the canonical artifact path, and empty unresolved divergence without raw transcripts or full synthesis notes

#### Scenario: Synthesis divergence triggers bounded correction
- GIVEN synthesis notes identifying an unresolvable contradiction between planning lens drafts
- WHEN the Specialist reports the phase outcome to the Orchestrator
- THEN the Phase Verdict contains outcome `needs-revision` with populated `unresolved_divergence`, triggering exactly one bounded correction lane instead of a full re-fan-out

#### Scenario: Unstructured evidence delivery rejected
- GIVEN a phase completion report containing raw multi-lane transcripts or unparsed logs instead of structured fields
- WHEN the report is received by the Orchestrator
- THEN the submission is rejected and structured phase verdict fields are required

### Requirement: SDD Phase-Gated Verification Check Execution

#### Scenario: Apply phase executes full verification suite
- GIVEN a candidate lane declaring `sdd_phase: apply` in its lane metadata
- WHEN acceptance verification is executed in `internal/accept` or `internal/run`
- THEN `lucind-checks.sh` is executed in the isolated worktree and passing checks are required for acceptance

#### Scenario: Planning phase skips verification script execution
- GIVEN a planning lane declaring a non-apply phase such as `propose` or `spec` in its lane metadata
- WHEN acceptance verification is executed in `internal/accept`
- THEN `lucind-checks.sh` execution is skipped and acceptance is evaluated on schema, done criteria, and scope

#### Scenario: Unlabeled lane or explicit exception executes checks
- GIVEN a lane with an empty or missing `sdd_phase` value, or declaring an explicit check exception
- WHEN acceptance verification is executed in `internal/accept`
- THEN `lucind-checks.sh` is executed in the isolated worktree

#### Scenario: Check failure in apply phase rejects acceptance
- GIVEN a candidate lane declaring `sdd_phase: apply` where `lucind-checks.sh` exits non-zero
- WHEN acceptance verification is executed in `internal/accept`
- THEN verification fails with a mechanical checks error and no acceptance receipt is persisted

### Requirement: Specialist-Owned Synthesis Arbitration

#### Scenario: Specialist arbitrates contradictions in synthesis notes
- GIVEN synthesis notes identifying conflicting recommendations across lens drafts
- WHEN the Specialist reviews the synthesis result
- THEN the Specialist arbitrates the conflict and decides whether to accept the canonical artifact or mark the verdict `needs-revision`

#### Scenario: Synthesis notes withheld from orchestrator context
- GIVEN an accepted synthesis producing canonical artifacts and detailed working notes
- WHEN the Specialist reports the phase outcome to the Orchestrator
- THEN detailed synthesis notes and draft comparison matrices remain in the Specialist context and are omitted from the Orchestrator conversation

#### Scenario: Synthesis blocked when lens receipts are missing
- GIVEN a multi-lens planning phase where one required lens lane has failed or has not been accepted
- WHEN synthesis dispatch is evaluated
- THEN synthesis execution is blocked until all required lens acceptance receipts exist in the ledger

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Specialist Phase Acceptance and Authority Carve-Out | Specialist independently accepts phase planning lanes | Tier A change requires dual-judge evaluation for specialist acceptance | Ordinary worker agent denied acceptance authority | `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-50` |
| Structured Phase Verdict Reporting | Successful phase completion returns compressed verdict | Synthesis divergence triggers bounded correction | Unstructured evidence delivery rejected | `docs/sdd-phase-specialist.md:21-30` |
| SDD Phase-Gated Verification Check Execution | Apply phase executes full verification suite | Planning phase skips verification script execution | Check failure in apply phase rejects acceptance | `internal/accept/accept.go:84-137` |
| Specialist-Owned Synthesis Arbitration | Specialist arbitrates contradictions in synthesis notes | Synthesis notes withheld from orchestrator context | Synthesis blocked when lens receipts are missing | `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-48` |

## Untestable Assertions

None

## Open Questions

- [ ] None

## Citation Manifest

| Citation | Claim |
|---|---|
| `CONTEXT.md:51-53` | Acceptance definition permits inclusion of lane results without additional human confirmation. |
| `CONTEXT.md:91-93` | Promotion definition requires explicit human confirmation for integration into Integration Target. |
| `CONTEXT.md:103-106` | Specialist definition establishes phase-scoped acceptance authority and excludes orchestrator role. |
| `CONTEXT.md:107-109` | Phase Verdict definition specifies outcome, canonical artifact path, and unresolved divergence. |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | ADR records decision for specialist acceptance authority and scoped test check execution. |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-18` | ADR specifies Hard Rule carve-out and Dual-Judge requirement for Tier A Changes. |
| `docs/sdd-phase-specialist.md:21-30` | Design note details resolved decisions for specialist role, verdict contents, and check gating. |
| `internal/accept/accept.go:84-96` | Lane metadata retrieval and binding validation during acceptance verification. |
| `internal/accept/accept.go:120-137` | Check policy snapshot, verification check execution, and acceptance receipt persistence call sites. |
| `internal/integrate/integrate.go:159-200` | Ungated integrate.Check implementation executing lucind-checks.sh. |
| `internal/ledger/lanes_meta.go:20-47` | LaneMetadata struct definition containing the SDDPhase field. |
| `internal/run/attempt.go:431-435` | Attempt execution checking phase and default checkFunc assignment. |
| `openspec/changes/agentic-phase-specialist/proposal.md:67-132` | Delta specifications and draft scenarios for the four agentic specialist requirements. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:18-19` | Hard rules defining orchestrator authority and lane ownership constraints. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:20-20` | Mechanical acceptance automation protocol and check execution sequence. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43` | Dual-Judge acceptance requirements for Tier A changes. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:46-50` | Promotion gate requirements requiring explicit human confirmation. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25` | Fan-out dispatch rules requiring all lens receipts before starting synthesis. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Strategy specification for synthesis note review and contradiction arbitration. |
