---
id: tasks-skill-provisioning-and-phase-specialist-lens-c
executor: agy
routed_by: proof and review-burden lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-c.md"]
---

# Packet tasks-skill-provisioning-and-phase-specialist-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-provisioning-and-phase-specialist-lens-c  ·  **Branch:** lucind/tasks-skill-provisioning-and-phase-specialist-lens-c

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-c.md`: what proves
each piece of this change works — the RED test task for every applicable threat-matrix row, the
acceptance evidence per task, and the Review Workload Forecast that says whether this ships as one
PR or a chain.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final
checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and all 8 capability spec deltas are accepted and frozen. Lens A and lens B run in
parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the task decomposition and is running concurrently, so you do not have it. Derive the
work from the design's File Changes table (`design.md:89-108`) and Testing Strategy
(`design.md:110-123`), declare it in `## Assumed decomposition`, and attach proof to that. The
synthesizer arbitrates divergence.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/design.md` exists and carries a threat
  matrix (`:125-133`) and a testing strategy (`:110-123`).
- `openspec/changes/skill-provisioning-and-phase-specialist/specs/` exists.
- `openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-c.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill, and its Review
   Workload Forecast table and threat-matrix rule in particular.
2. `openspec/changes/skill-provisioning-and-phase-specialist/design.md` — the Threat Matrix
   (`:125-133`, 5 rows: documentation-like paths, git repository selection, commit state, push
   state N/A, PR commands) and the Testing Strategy table (`:110-123`, unit/integration/E2E rows
   with concrete seams). Every row marked Applicable becomes an explicit RED-test task before its
   production task; rows marked N/A stay omitted.
3. `openspec/changes/skill-provisioning-and-phase-specialist/specs/` — the scenarios across all 8
   capability deltas, which are what the tests assert.
4. Existing test files: `internal/packet/packet_test.go`, `internal/accept/accept_test.go`,
   `internal/accept/authoring_evidence_test.go`, `internal/result/schema_test.go`, so a proposed
   test command is one this repository can actually run.

Never propose a test command you did not derive from a real test file, and never claim a test
proves something without saying which assertion fires.

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-c.md`:

```markdown
# Tasks Lens C — Proof & Review Burden: Skill Provisioning and the SDD Phase Specialist

## Assumed decomposition

<2–4 sentences naming the work breakdown you are attaching proof to. Lens A and
lens B write this same block independently; the synthesizer compares all three.>

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | <a number or a range, with the basis> |
| 400-line budget risk | Low / Medium / High |
| Chained PRs recommended | Yes / No |
| Suggested split | <single PR, or PR 1 → PR 2 → PR 3> |
| Delivery strategy | <ask-on-risk / auto-chain / single-pr / exception-ok> |
| Chain strategy | <stacked-to-main / feature-branch-chain / size-exception / pending> |

<State the basis for the line estimate: 4 new packages plus 11 modified files
(`design.md:89-108`) — name what you counted. A fan-out template forecast of
120-250 lines came in at 1730 for template-heavy work before; do not repeat that
underestimate.>

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|

<One row per threat-matrix row in `design.md:127-133`. Copy the Applicable/N/A
verdict; do not re-decide it.>

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|

## Verification Gaps

<Every behavior the specs require that no proposed test proves. "None" if none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`tasks-lens-c.md` MUST be under 1000 words. Tables over prose.

## Out of scope

- **Lens A owns**: the phased checklist, dependency-order table, requirement traceability.
- **Lens B owns**: the Suggested Work Units table, wave partition, `allowed_paths`, executor
  assignment, sidecar recommendation.

Do not write the task checklist and do not partition units into waves.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-c.md` only.

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
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Review Workload Forecast" \
  --require-section "RED Tests from the Threat Matrix" --require-section "Acceptance Evidence" \
  --require-section "Verification Gaps" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

After committing and writing `.lucind/result.json`:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-c.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL.**
- [ ] **Every threat-matrix row in `design.md:127-133` appears in `## RED Tests from the Threat Matrix` with the design's own Applicable/N/A verdict**, and no row was invented.
- [ ] **Every applicable row names a RED test and the assertion that fires.**
- [ ] **The changed-line estimate states the basis it was derived from.**
- [ ] **Every proving command is derived from a real test file in this worktree**, cited with `file:line`.
- [ ] **`tasks-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed decomposition`, `## Review Workload Forecast`, `## Acceptance Evidence`, `## Verification Gaps`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The design has no threat matrix, so there is nothing to derive RED tests from.
- A behavior the specs require cannot be proven through any test this repository can run, and no
  new seam is identified in the design.
- The changed-line estimate cannot be grounded in anything.
- Satisfying one instruction in this packet would require violating another.

## Context

Confirmed preflight: execution mode auto, artifact store hybrid, delivery strategy single-pr with
size:exception pre-accepted, review budget 10000 changed lines. Do not re-litigate these — the
Delivery strategy and Chain strategy rows should reflect the pre-accepted single-pr with
size:exception, not an independently chosen alternative, unless your forecast finds evidence that
contradicts it (say so if it does).

Threat Matrix (`design.md:125-133`): 5 rows — documentation-like paths (Applicable, RED test at
`internal/accept/accept_test.go:125-138`), git repository selection (Applicable, identical digest
across differing root prefixes), commit state (Applicable, accept passes/rejects on
match/mismatch), push state (N/A: no ref mutation), PR commands (Applicable, malformed status JSON
fails closed).

Testing Strategy (`design.md:110-123`): unit rows for `internal/packet/packet.go:122-179`, new
`internal/skillset/skillset_test.go`, new `internal/skillroots/skillroots_test.go`,
`internal/packetauthor/compile.go:45,171-183`, `internal/result/schema_test.go:10-33`; integration
rows for `internal/run/run.go:876-904`, `internal/accept/authoring_evidence_test.go:56-127`,
`internal/executor/executor.go:20-39`, `internal/ledger/authoring.go:44-75`; E2E row for new
`internal/phasespec/specialist_test.go`.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
