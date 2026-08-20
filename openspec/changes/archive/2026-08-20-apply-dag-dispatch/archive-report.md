# Archive Report: apply-dag-dispatch

**Date**: 2026-08-20  
**Change**: apply-dag-dispatch  
**Status**: archived, SDD cycle complete

## Executive Summary

The `apply-dag-dispatch` change has been fully planned, implemented, verified (PASSED), and archived. Three canonical capability specs (apply-dag-dispatch, allowed-paths-enforcement, sdd-apply) have been merged into `openspec/specs/`, and the change folder has been moved to `openspec/changes/archive/2026-08-20-apply-dag-dispatch/`.

## Final State Per Authority Hierarchy

### Orchestrator Facts (Rank 3)
- All phases marked `status: done` in `state.yaml` (explore, proposal, design, spec, tasks, apply, verify)
- Explicit user instruction: change is "fully complete"
- No blockers reported

### Native Review Authority (Rank 1)
- No review was conducted for this change (`reviewGate` absent per SDD lifecycle)
- Proceed under ordinary repository policy

### Persisted Tasks Artifact (Rank 2)
- **Reconciliation**: All 79 implementation checkboxes marked complete (from stale state at apply→verify transition)
- **Reason for reconciliation**: 
  - Apply phase status: done with detailed notes on every phase implemented (Phases 1-9)
  - Verify phase status: done with PASSED verdict from both lanes (agy, cursor-agent)
  - No CRITICAL issues in verification
  - All spec scenarios tested; verified findings are non-blocking gaps
- **Source**: openspec/changes/apply-dag-dispatch/tasks.md → now archived at openspec/changes/archive/2026-08-20-apply-dag-dispatch/tasks.md

### Verification Evidence (Rank 4 - Snapshot)
Per `openspec/changes/archive/2026-08-20-apply-dag-dispatch/verify.md`:
- Stage 1: lucind-ai check PASSED (commit b5488f0)
- Stage 2: Dual qualitative judgment (agy + cursor-agent) — both PASS, integrated cleanly
- Result: "Overall verdict: PASSED"
- Non-blocking findings: two identified design gaps (cross-wave overlap narrower than stated; staged-but-uncommitted index path missed by three-way diff) do not contradict tested scenarios

## Specs Synced to Main

Three delta capability specs merged into `openspec/specs/`:

| Spec | Location | Size | Notes |
|------|----------|------|-------|
| apply-dag-dispatch | `openspec/specs/apply-dag-dispatch/spec.md` | Full spec | DAG parse, Kahn waves, split mechanics, sequential per-wave dispatch |
| allowed-paths-enforcement | `openspec/specs/allowed-paths-enforcement/spec.md` | Full spec | Packet AllowedPaths field, upfront disjointness check, post-execution scope check, .lucind/ exclusion |
| sdd-apply | `openspec/specs/sdd-apply/spec.md` | Full spec | Apply orchestrator contract, additive rollback, stdout reporting, unmodified combine/resolve/bisect reuse |

Verification: Each spec copied via `cp -R` and verified with `diff -r` — all three are byte-identical to source.

## Archive Location

**Source**: `openspec/changes/apply-dag-dispatch/`  
**Destination**: `openspec/changes/archive/2026-08-20-apply-dag-dispatch/`  
**Method**: `git mv` (tracked in repository)  
**Verification**: Pre-move snapshot diffed against archived folder — empty diff (byte-identical)

### Archived Artifacts
- proposal.md (canonical, merged from agy + cursor-agent drafts)
- design.md (canonical, merged with design decisions 1-2 finalized)
- specs/ (three delta specs, identical to main specs copies)
- tasks.md (canonical, checkboxes reconciled to reflect apply completion)
- verify.md (final PASSED verdict with structural envelopes from both lanes)
- state.yaml (phase timeline with archive phase now status: done)
- apply-dag.yaml (sidecar used during apply, hand-split into wave packets)
- [draft siblings archived for reference]: proposal-agy.md, proposal-cursor-agent.md, design-agy.md, design-cursor-agent.md, tasks-agy.md, tasks-cursor-agent.md, specs-agy/, specs-cursor-agent/

## Final Checklist

- [x] Task Completion Gate: All 79 implementation tasks checked (reconciled stale state after apply completion)
- [x] Native Review Receipt Gate: No review conducted; proceeding under ordinary policy
- [x] Verification: No CRITICAL issues; PASSED verdict from dual lanes
- [x] Delta specs synced to main specs: apply-dag-dispatch, allowed-paths-enforcement, sdd-apply
- [x] Change folder moved to archive with date prefix
- [x] Archive verified byte-identical to source
- [x] state.yaml updated: archive phase now status: done
- [x] All artifacts present in archive folder

## Key Observations

1. **Stale Checkbox Reconciliation**: Tasks checkboxes were marked unchecked at the end of the spec/tasks phases but were completed during apply (phases 1-9). The reconciliation is backed by apply-phase notes explicitly documenting each phase implemented, and verify-phase PASSED verdict. This is an expected case per the Task Completion Gate — stale checkboxes after apply/verify completion are reconciled with proof.

2. **Dual-Executor Dispatch Experiment**: This change was part of a three-change umbrella exploration (read-only-packet-dispatch, apply-dag-dispatch, verify-dual-dispatch) using real `lucind-ai run` dispatch for propose/design phases with parallel agy and cursor-agent execution. The canonical artifacts were synthesized by the orchestrator from both lanes.

3. **Non-Blocking Verification Findings**: Two gaps identified during verify (cross-wave overlap check per-wave only, staged-but-uncommitted path outside three-way diff) are narrower than design guarantees but do not violate any tested spec scenario. Documented in verify.md as future hardening items.

4. **Review Budget Resolution**: Tasks phase flagged the change as over budget (2600-3800 lines estimated, delivery_strategy single-pr). User resolved by raising review_budget_lines to 5000 before apply; change proceeded as single-PR without chained split.

## Engram Topic Keys (For Traceability)

(Hybrid mode — topics persist separately; included for reference)
- sdd/dual-executor-dispatch/explore (umbrella exploration)
- sdd/apply-dag-dispatch/proposal
- sdd/apply-dag-dispatch/design
- sdd/apply-dag-dispatch/spec
- sdd/apply-dag-dispatch/tasks
- sdd/apply-dag-dispatch/verify
- sdd/apply-dag-dispatch/archive-report (this report)
