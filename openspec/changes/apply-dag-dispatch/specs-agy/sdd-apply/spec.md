# SDD Apply Specification

## Purpose

Shift the SDD apply lifecycle from direct in-process file modifications to declarative DAG specification authoring, wave-based packet dispatch via `lucind-ai run`, and integration report handling.

## Requirements

### Requirement: Declarative Apply Artifact Authoring

The apply phase MUST author `apply-dag.yaml` defining packet nodes, allowed path scopes, and dependencies, alongside packet markdown bodies, rather than modifying source files directly within the orchestrator session. (Trace: Proposal §Impact on existing sdd-apply flow, Decision 1 — DAG artifact and exact format)

#### Scenario: Author apply DAG sidecar and bodies
- GIVEN an approved design and tasks checklist
- WHEN the apply phase begins
- THEN the orchestrator MUST author `apply-dag.yaml` and body files under `bodies/` without directly editing code in the primary worktree

#### Scenario: Human task list tasks.md preserved as checklist
- GIVEN existing `tasks.md` format produced by `sdd-tasks`
- WHEN `apply-dag.yaml` is authored
- THEN `tasks.md` MUST remain an unstructured human checklist without embedded machine metadata

### Requirement: Orchestrator-Driven Wave Dispatch Loop

The apply orchestrator MUST invoke `lucind-ai split` to generate packet files and wave commands, then iteratively execute each `lucind-ai run` command in sequence. (Trace: Proposal §Impact on existing sdd-apply flow, Decision 1 — DAG artifact and exact format, Decision 3 — sequential lucind-ai run per wave)

#### Scenario: Split tasks into wave commands and execute sequentially
- GIVEN a valid `apply-dag.yaml`
- WHEN the orchestrator runs `lucind-ai split`
- THEN it MUST parse the emitted wave lines and dispatch each wave sequentially with `lucind-ai run`

#### Scenario: Stop on wave failure for human review
- GIVEN a wave dispatch that exits with a non-zero code
- WHEN the orchestrator receives the error
- THEN it MUST halt the apply loop immediately for human review or replanning

### Requirement: Integration Outcome Handling

The apply orchestrator MUST inspect the stdout report and exit status of each wave dispatch to confirm which lane IDs were integrated or reverted before proceeding or surfacing blocks. (Trace: Proposal §Impact on existing sdd-apply flow, Decision 4 — partial-failure surfacing)

#### Scenario: Verify integrated lane IDs before next wave
- GIVEN a completed wave dispatch exiting with code 0
- WHEN the orchestrator inspects stdout
- THEN it MUST verify all wave lane IDs are listed under `integrated_ids` before triggering the next wave

#### Scenario: Halt and report reverted lane IDs on bisection failure
- GIVEN a wave where one or more lanes are listed under `reverted_ids`
- WHEN `lucind-ai run` exits non-zero
- THEN the orchestrator MUST surface the reverted lane IDs and halt further wave execution
