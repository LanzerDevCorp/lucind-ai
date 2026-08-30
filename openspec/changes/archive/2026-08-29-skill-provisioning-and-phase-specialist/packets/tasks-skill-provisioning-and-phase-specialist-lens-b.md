---
id: tasks-skill-provisioning-and-phase-specialist-lens-b
executor: agy
routed_by: partition and dispatch-shape lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-b.md"]
---

# Packet tasks-skill-provisioning-and-phase-specialist-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-provisioning-and-phase-specialist-lens-b  ·  **Branch:** lucind/tasks-skill-provisioning-and-phase-specialist-lens-b

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-b.md`: how this
change's work is partitioned into dispatchable units — the work-unit table, the wave plan, each
unit's `allowed_paths`, its executor, and whether an `apply-dag.yaml` sidecar is warranted at all.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final
checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and all 8 capability spec deltas are accepted and frozen. Lens A and lens C run in
parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the task decomposition and is running concurrently, so you do not have it. Derive the
work from the design's File Changes table (`design.md:89-108`), declare it in
`## Assumed decomposition`, and partition that. The synthesizer arbitrates divergence.

## Why this lens exists

A partition that looks parallel and is not costs a whole wave. Two failure modes are specific to
this repository and both are checkable before dispatch:

- **The `allowed_paths` prefix trap.** Scope matching is a component-boundary prefix match
  (`internal/packet/disjoint.go`), so naming a directory covers everything beneath it. This change
  creates four sibling packages (`internal/skillset/`, `internal/skillroots/`,
  `internal/lucindconfig/`, `internal/phasespec/`, per `design.md:103-106`) that look independent —
  check each pair's paths by hand, not by directory name alone.
- **The `Integrate` gate.** `Integrate` runs `lucind-checks.sh` on the combined tree and bisects a
  failing batch (`internal/run/integrate.go:50-59`). Every wave must pass those checks on its own.
  Decision 8 (`design.md:61-66`) requires `packetDigest` (`internal/run/run.go:722-729`) and the
  accept decode struct (`internal/accept/accept.go:275-286`) to gain the same field list in the same
  commit — splitting those two edits across waves breaks correspondence for everything dispatched in
  between. Strict-TDD RED and GREEN for one unit belong in one lane, never in two waves.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill, and its Suggested Work
   Units table in particular.
2. `openspec/changes/skill-provisioning-and-phase-specialist/design.md` — the File Changes table
   (`:89-108`) is the partition's input; Decision 8 (`:61-66`) names the correspondence pairs that
   must not split across waves.
3. `internal/packet/disjoint.go` — the real matching rule.
4. `internal/run/integrate.go` — the gate and the bisection.
5. `internal/dag/` — the sidecar's node shape.
6. `openspec/changes/archive/` for a prior change that used an `apply-dag.yaml`, and for
   `2026-08-20-apply-dag-dispatch-hardening/tasks.md`, which declined a DAG split because the units
   were too small to pay for sidecar orchestration. Declining is a legitimate outcome here.

Never claim two units are disjoint without checking their paths against the prefix rule by hand.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-b.md`:

```markdown
# Tasks Lens B — Partition & Dispatch Shape: Skill Provisioning and the SDD Phase Specialist

## Assumed decomposition

<2–4 sentences naming the work breakdown you are partitioning. Lens A and lens C
write this same block independently; the synthesizer compares all three.>

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|

## Disjointness Check

<For every pair of units in the same wave, the two path sets and the verdict
under the component-boundary prefix rule.>

## Sidecar Recommendation

**Recommendation**: <sidecar warranted / single packet, no sidecar>
**Rationale**: <why, citing the archived precedent if relevant.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`tasks-lens-b.md` MUST be under 1000 words. Tables over prose.

## Out of scope

- **Lens A owns**: the phased checklist, dependency-order table, requirement traceability.
- **Lens C owns**: the Review Workload Forecast table, per-task acceptance evidence, the RED-test
  task for every applicable threat-matrix row (`design.md:127-133`).

Do not write the task checklist. Do not estimate changed lines or name a PR split.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-b.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/`.

Precedence is not symmetric. The skill is authority on *what a task file must contain*. This packet
is authority on *how this phase is being executed here*. Where the skill's single-writer
instructions conflict, this packet supersedes; note the conflict in `## Open Questions`.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, line numbers ascending.

## Mechanical self-check (REQUIRED)

Before committing:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Suggested Work Units" \
  --require-section "Wave Plan" --require-section "Disjointness Check" \
  --require-section "Sidecar Recommendation" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

After committing and writing `.lucind/result.json`:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-b.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL.**
- [ ] **Every pair of units sharing a wave was checked by hand against the component-boundary prefix rule**, recorded in `## Disjointness Check`.
- [ ] **Every wave's "green on its own" column is a yes with a reason**, or the wave was merged into its successor.
- [ ] **Every unit names concrete `allowed_paths` and an executor**, resolving in this worktree or marked "new file".
- [ ] **`tasks-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed decomposition`, `## Wave Plan`, `## Disjointness Check`, `## Sidecar Recommendation`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- No partition exists in which every wave is green on its own. Say so; a single packet is not a
  failure.
- Two units that must run in the same wave cannot have disjoint `allowed_paths`.
- The design's File Changes table does not determine which unit owns a file.
- Satisfying one instruction in this packet would require violating another.

## Context

Confirmed preflight: execution mode auto, artifact store hybrid, delivery strategy single-pr with
size:exception pre-accepted, review budget 10000 changed lines. Do not re-litigate these.

File Changes table (`design.md:89-108`): 4 new packages (`internal/skillset/`,
`internal/skillroots/`, `internal/lucindconfig/`, `internal/phasespec/`) and 11 modified files
(`internal/packet/packet.go`, `internal/packetauthor/contract.go`, `internal/packetauthor/compile.go`,
`internal/executor/executor.go`, `internal/result/result.go`, `internal/result/result.schema.json`,
`internal/result/schema_test.go`, `internal/run/run.go`, `internal/accept/accept.go`,
`internal/accept/authoring_evidence_test.go`, `cmd/lucind-ai/cli.go`/`packet_authoring.go`,
`.agents/skills/lucind-*`, `plugin/.../assets/*.md`).

Decision 8 (`design.md:61-66`): `packetDigest` (`internal/run/run.go:722-729`) field list and the
accept decode struct (`internal/accept/accept.go:275-286`) both enumerate packet fields literally
and both change in the same commit — a wave that splits them breaks correspondence for anything
dispatched between the two commits.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
