---
id: tasks-skill-provisioning-and-phase-specialist-lens-a
executor: agy
routed_by: decomposition and ordering lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-a.md"]
---

# Packet tasks-skill-provisioning-and-phase-specialist-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-provisioning-and-phase-specialist-lens-a  ·  **Branch:** lucind/tasks-skill-provisioning-and-phase-specialist-lens-a

## Goal

Produce `openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-a.md`: the phased
implementation checklist for this change and the dependency order behind it — what must exist
before what, and why.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final
checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

`design.md` and all 8 capability spec deltas for `skill-provisioning-and-phase-specialist` are
accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to
different files, so no lane races another. This lens owns the decomposition; the other two
partition it for dispatch and attach proof to it.

## Preconditions

- `openspec/changes/skill-provisioning-and-phase-specialist/specs/` and
  `openspec/changes/skill-provisioning-and-phase-specialist/design.md` both exist.
- `openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/skill-provisioning-and-phase-specialist/design.md` — the File Changes table
   (`design.md:89-108`) lists 15 files across 4 new packages (`internal/skillset`,
   `internal/skillroots`, `internal/lucindconfig`, `internal/phasespec`) and 11 modified files; the
   Flow and Invariants block (`design.md:68-87`) and Decisions 1-8 (`design.md:16-67`) state why
   each exists.
3. `openspec/changes/skill-provisioning-and-phase-specialist/specs/` — all 8 capability deltas
   (`skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`,
   `phase-specialist-dispatch`, `packet-authoring-contract`, `lane-execution`,
   `acceptance-verifier`, `read-only-packet-schema`): every requirement, so no task exists that no
   requirement asks for and no requirement lacks a task.
4. `internal/packet/`, `internal/packetauthor/`, `internal/run/`, `internal/accept/`,
   `internal/result/`, `internal/ledger/` — enough to know what already exists. A task that says
   "create" a file that already exists is wrong, and a task that says "modify" one that does not is
   worse.

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line`
citation or is explicitly marked "new file".

## Output format

Write exactly this skeleton to
`openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-a.md`:

```markdown
# Tasks Lens A — Decomposition & Ordering: Skill Provisioning and the SDD Phase Specialist

## Assumed decomposition

<2–4 sentences naming the work breakdown you are ordering: how many phases, what
each phase delivers, and what the change's critical path is. Lens B and lens C
write this same block independently; the synthesizer compares all three. Be
specific enough that a disagreement is visible.>

## Phase 1: <Phase Name>

- [ ] 1.1 <Concrete action — which file, which change>
- [ ] 1.2 <Concrete action>

## Phase N: <Phase Name>

<same shape>

## Dependency Order

| Task | Depends on | Why |
|---|---|---|

<Only real dependencies. A task with no dependency can run in parallel — say so
with an em dash rather than omitting the row.>

## Requirement Traceability

| Requirement | Tasks |
|---|---|

<Every requirement across all 8 capability deltas maps to at least one task,
and every task maps back to at least one requirement.>

## Open Questions

- [ ] <unresolved question, or "None">
```

Every task MUST be specific, actionable, verifiable, and small (one file or one logical unit).

## Size budget

`tasks-lens-a.md` MUST be under 1000 words. This change touches 15 files across 4 new and 11
modified packages (`design.md:89-108`) — a prior fan-out forecast of 120-250 lines for
template-heavy work came in at 1730; do not under-scope the phasing to fit the budget. If the
breakdown does not fit, say so in `## Open Questions` rather than merging tasks to save words.

## Out of scope

- **Lens B owns**: the Suggested Work Units table, the wave partition, per-unit `allowed_paths`,
  executor assignment, and whether an `apply-dag.yaml` sidecar is warranted.
- **Lens C owns**: the Review Workload Forecast table, per-task acceptance evidence, and the
  RED-test task for every applicable threat-matrix row (`design.md:127-133`).

Do not estimate changed lines and do not name PR boundaries. Do not assign an executor to a task.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-a.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its
`references/`.

Precedence is not symmetric. The skill is authority on *what a task file must contain*. This packet
is authority on *how this phase is being executed here*: three parallel lanes, this lane's slice,
word budget, output path, out-of-scope list, done criteria. Where the skill's single-writer
instructions conflict (forecast table, work-units table, Engram persistence, phase summary block),
this packet supersedes; note the conflict in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, line numbers ascending.

## Mechanical self-check (REQUIRED)

Before committing:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" --require-section "Assumed decomposition" \
  --require-section "Dependency Order" --require-section "Requirement Traceability" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

After committing and writing `.lucind/result.json`:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks-lens-a.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL.**
- [ ] **Every task names a concrete path**, and every "modify" task's path resolves in this worktree.
- [ ] **Every requirement across all 8 capability deltas appears in the traceability table with at least one task.**
- [ ] **Every dependency row states why it is a real ordering constraint.**
- [ ] **`tasks-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed decomposition`, `## Dependency Order`, `## Requirement Traceability`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- A requirement in `specs/` cannot be decomposed into tasks because the design does not say how it
  is built.
- The design's File Changes table and the specs disagree about what this change does.
- Two tasks are mutually circular: each needs the other to land first.
- Satisfying one instruction in this packet would require violating another.

## Context

Confirmed preflight for this change: execution mode auto, artifact store hybrid, delivery strategy
single-pr with size:exception pre-accepted, review budget 10000 changed lines. Do not re-litigate
these.

Design decisions to decompose against (`design.md:16-67`): Decision 1 (ad-hoc skills on both
`Contract.AdhocSkills` and packet `adhoc_skills` frontmatter, `required_skills` always derived);
Decision 2 (`DefaultSkillBudget = 3`, overridable via `lucind.yaml`); Decision 3 (one
`lucind-ai phase <name>` subcommand plus one `.opencode/agent/lucind-packet-author.md` profile);
Decision 4 (`skillset.Derive` pure function signature); Decision 5 (tracked `lucind.yaml` +
gitignored `.lucind/skill-roots.yaml`); Decision 6 (closed `lane_role` set, closed-validated
`sdd_phase` only when `lane_role` present); Decision 7 (no stub `lucind-archive`/`lucind-ultrafixer`
skills); Decision 8 (`skillset.DigestBody` excludes `## Required skills` from both packet and
compile digests — both `packetDigest` in `run.go:722-729` and the accept decode struct in
`accept.go:275-286` must gain the same fields in the same commit).

File Changes table: `design.md:89-108`. Flow and Invariants: `design.md:68-87`.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
