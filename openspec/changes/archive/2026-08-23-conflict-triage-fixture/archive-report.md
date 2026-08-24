# Archive Report: Conflict Triage Fixture

**Change**: conflict-triage-fixture
**Archive Date**: 2026-08-23
**Branch**: feature/conflict-triage-fixture
**Commit at close**: 22d1aeb (remediation verified)
**SDD Cycle Status**: Complete

## Executive Summary

The conflict-triage-fixture change has been fully planned, implemented, verified (with remediation), and archived. Four new capability specifications (conflict-triage, conflict-fixture, triage-evaluation-rubric, and modified reconciliation-approval) have been merged into the main openspec/specs/ tree. All 20 implementation tasks are checked and complete.

## Specifications Synced

| Domain | Action | Details |
|--------|--------|---------|
| conflict-triage | Created | New capability for advisory conflict triage agent; fail-open; 3-band risk; wall-clock verify budget |
| conflict-fixture | Created | New capability for 3-hunk deterministic fixture; shared base SHA; forces ClassRequired |
| triage-evaluation-rubric | Created | New capability for offline dual-judge grading; claude-opus-5 and openai/gpt-5.6-sol isolation |
| reconciliation-approval | Modified | Added new requirement "Two-step close and retry CAS" |

### Files Synced to Main Specs

- `openspec/specs/conflict-triage/spec.md` (new)
- `openspec/specs/conflict-fixture/spec.md` (new)
- `openspec/specs/triage-evaluation-rubric/spec.md` (new)
- `openspec/specs/reconciliation-approval/spec.md` (modified: 1 requirement added)

## Archive Contents

```
openspec/changes/archive/2026-08-23-conflict-triage-fixture/
├── proposal.md
├── design-lens-a.md
├── design-lens-b.md
├── design-lens-c.md
├── design-synthesis-notes.md
├── specs/
│   ├── conflict-triage/spec.md
│   ├── conflict-fixture/spec.md
│   ├── triage-evaluation-rubric/spec.md
│   └── reconciliation-approval/spec.md (delta)
├── tasks.md
├── verify.md
├── remediation.md
└── archive-report.md (this file)
```

## Task Completion

All 20 implementation tasks are checked and complete per the persisted `tasks.md`:

- **Phase 1 (Foundation Models & Output Persistence)**: 4 tasks complete
  - TriagePayload and HunkDecision types with risk bands
  - TriageInvoker function type
  - Persistent JSON to Candidate.Output without status change
  - Service.UpdateCandidateOutput implementation

- **Phase 2 (Advisory Triage Agent & Invariants)**: 3 tasks complete
  - ScanConflictMarkers and EnforceAllowedPaths regression tests
  - Business hunk ARBITRARY/high risk tests
  - RunTriage implementation with fail-open invariant checks

- **Phase 3 (Three-Hunk Fixture & Disjoint Packets)**: 4 tasks complete
  - Fixture generator forcing ClassRequired
  - Three-hunk toy file (1 business, 2 mechanical controls)
  - Packet templates with prefix-disjoint allowed_paths
  - CAS retry and tip-drift re-block tests

- **Phase 4 (Dual-Judge Rubric)**: 2 tasks complete
  - Rubric tests for distinct three-hunk classification
  - EvaluateRubric implementation on registered executors (claude-opus-5 and openai/gpt-5.6-sol)

**Note on Accepted Deviations** (recorded in proposal.md:153-194):

1. **Single feature delivery**: The proposal specified two path-disjoint features (conflict-triage-agent and conflict-fixture) promoted through separate lucind-ai run dispatches. The implementation delivered one change on a single feature (conflict-triage-fixture) using legacy mode (--legacy-main). This is accepted because the product does not require the multi-feature boundary; nothing depends on separate promotion. What it costs: the mixed-target refusal, cross-feature prefix-disjoint allowed_paths, and per-feature CAS promotion machinery remain unverified by this change's delivery.

2. **Verify on registered feature**: Verify is the first phase dispatched against the registered feature (feature/conflict-triage-fixture) rather than legacy mode, exercising partial CAS mechanics. This is also accepted and documented.

## Verification and Remediation Status

**Overall verdict**: PASSED (remediated) — per `verify.md` and verified by orchestrator.

The original verify phase identified two confirmed gaps and six carried-forward claims. All have been triaged and remediated:

### Confirmed Gaps (now closed)

1. **Fixture packets and toy file coherence** — Resolved by clarifying that packets are build-scope templates, not the dispatch shape for the toy collision. GenerateFixture writes toy.go independently via git. Test added: `TestFixturePackets_AreBuildScopeTemplatesNotToyWriters`.

2. **Missing negative admission test** — Resolved by adding `TestFixturePackets_OverlappingBuildScopesRefused` to verify the spec's MUST-fail scenario for non-disjoint allowed_paths.

### Carried-Forward Claims (all triaged)

1. **REFUTED** — RunTriage enforcement of mechanical resolutions: the prompt asks the invoker to resolve deterministically; only business hunks are pinned to ARBITRARY/high.

2. **CONFIRMED and FIXED** — EvaluateRubric grading canned JSON: now attaches fixture evidence (both sides of toy.go) to the rubric prompt. Test: `TestRubric_PresentsGenerateFixtureEvidence`.

3. **CONFIRMED and FIXED** — Invariant coverage gap through RunTriage: added `TestTriageAgent_OutOfScopeEditsFailCandidate` to verify out-of-scope edits fail.

4. **REFUTED** — Missing/divergent base SHA skip: verified at overlap.Evaluate layer (tasks.md:58), not gate layer; within change's contract.

5. **REFUTED** — Empty verify command bypass: validVerifyBudget already enforces " min: " (with trailing space); test locks replacement behavior.

6. **CONFIRMED and FIXED** — Business hunk compile/test validation: GenerateFixture now writes go.mod and per-side toy_test.go. Test: `TestFixtureGenerator_BothSidesCompileAndPassOwnTests`.

## Open Design Questions

Both questions remain open (deferred until fixture produces calibration data):

1. Exact non-decreasing risk formula and numeric thresholds, including mixed business+mechanical hunks.
2. Selection of production executor/model for live conflict triage (judges: claude-opus-5 and openai/gpt-5.6-sol; production runtime is distinct).

## Ancestry Note

This change's git ancestry on feature/conflict-triage-fixture contains commits from an unrelated change (lane-status-observability) due to a known bug in an earlier integration. This is pre-existing, documented in verify.md ("A note on this candidate's ancestry"), and is NOT a new defect to clean up or flag.

## Source of Truth Updated

All specifications now reflect the complete scope of the conflict-triage-fixture change:

- `openspec/specs/conflict-triage/spec.md` — advisory triage agent semantics, risk ratchet, fail-open contract
- `openspec/specs/conflict-fixture/spec.md` — 3-hunk fixture generation, shared base SHA, deterministic classification
- `openspec/specs/triage-evaluation-rubric/spec.md` — offline dual-judge grading, executor isolation, hunk classification quality
- `openspec/specs/reconciliation-approval/spec.md` — extended with "Two-step close and retry CAS" requirement covering human-in-the-loop approval and SHA registration flow

## Merge Verification

All delta specs copied mechanically via `cp -R` and verified via `diff -r`:
- New specs: empty diff (files match byte-for-byte)
- Reconciliation-approval: delta requirement appended to existing spec

Archive folder verified via `diff -r` snapshot comparison: empty diff (archive matches source at time of move).

## SDD Cycle Complete

The change has been fully planned (proposal.md, design.md across three lens deliverables), implemented (20 tasks across four phases), verified (mechanical + judgment lanes with remediation), and archived. The SDD cycle is now closed.

**Ready for the next change.**
