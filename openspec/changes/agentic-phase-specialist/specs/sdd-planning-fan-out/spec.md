# Delta for sdd-planning-fan-out

## MODIFIED Requirements

### Requirement: Two-Wave Planning Fan-Out Protocol

The phase Specialist MUST direct planning phases (explore, propose, design, specs, tasks) as a two-wave fan-out on generic `lucind-ai run --packet`. The Orchestrator MUST mechanically perform those dispatches on the Specialist's direction. Wave 1 MUST dispatch three parallel `agy` lens lanes to mutually disjoint draft paths. Wave 2 MUST dispatch one sequential `cursor-agent` synthesis lane branched from the integrated tree, producing the canonical artifact and synthesis notes. The phase Specialist MUST inspect synthesis notes, arbitrate unresolved contradictions across lens drafts, and verify canonical citations before deciding phase Acceptance. The Orchestrator SHALL NOT inspect raw synthesis notes or perform contradiction arbitration during normal phase execution. Sidecar `apply-dag.yaml` / `lucind-ai split` MUST NOT be required.
(Previously: The Orchestrator executed both waves and read synthesis notes to arbitrate contradictions.)

#### Scenario: Planning phase dual-wave dispatch

- GIVEN an SDD change in a planning phase
- WHEN wave 1 completes and integrates all three lens drafts
- THEN the orchestrator MUST dispatch wave 2 to produce the canonical artifact and synthesis notes

#### Scenario: Wave-2 synthesizer dispatched before wave 1 integrates

- GIVEN wave-1 lanes finished in isolated worktrees but have not integrated into primary `HEAD` (`internal/run/integrate.go:31-81`; `explore.md:42`)
- WHEN wave 2 dispatches the synthesizer from that unintegrated `HEAD`
- THEN the synthesizer worktree MUST NOT contain the wave-1 draft files (`SKILL.md:184-186`)

#### Scenario: Specialist arbitrates contradictions in synthesis notes

- GIVEN synthesis notes identifying conflicting recommendations across lens drafts
- WHEN the Specialist reviews the synthesis result
- THEN the Specialist arbitrates the conflict and decides whether to accept the canonical artifact or mark the verdict `needs-revision`

#### Scenario: Synthesis notes withheld from orchestrator context

- GIVEN an accepted synthesis producing canonical artifacts and detailed working notes
- WHEN the Specialist reports the phase outcome to the Orchestrator
- THEN detailed synthesis notes and draft comparison matrices remain with the Specialist and are omitted from the Orchestrator conversation
