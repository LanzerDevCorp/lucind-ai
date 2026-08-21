---
id: tasks-<change-id>-lens-a
executor: agy
routed_by: decomposition and ordering lens of the three-lens tasks fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/tasks-lens-a.md"]
---

# Packet tasks-<change-id>-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/tasks-<change-id>-lens-a  ·  **Branch:** lucind/tasks-<change-id>-lens-a

## Goal

Produce `openspec/changes/<change-id>/tasks-lens-a.md`: the phased implementation checklist for this change and the dependency order behind it — what must exist before what, and why.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

The spec and design for `<change-id>` are accepted and frozen. Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another. This lens owns the decomposition; the other two partition it for dispatch and attach proof to it.

## Preconditions

- `openspec/changes/<change-id>/specs/` and `openspec/changes/<change-id>/design.md` both exist.
- `openspec/changes/<change-id>/tasks-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *what work exists and in what order* — not to how it is dispatched and not to how it is proven:

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/design.md`, and its file-changes table in particular — every file
   created, modified, or deleted, and the terminal consumer of each.
3. `openspec/changes/<change-id>/specs/` — every requirement, so no task exists that no requirement
   asks for and no requirement lacks a task.
4. The packages the change lands in, enough to know what already exists. A task that says "create"
   a file that already exists is wrong, and a task that says "modify" one that does not is worse.

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line` citation or is explicitly marked "new file".

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/tasks-lens-a.md`:

```markdown
# Tasks Lens A — Decomposition & Ordering: <Change Title>

## Assumed decomposition

<2–4 sentences naming the work breakdown you are ordering: how many phases, what
each phase delivers, and what the change's critical path is. Lens B and lens C
write this same block independently; the synthesizer compares all three. Be
specific enough that a disagreement is visible.>

## Phase 1: <Phase Name>

- [ ] 1.1 <Concrete action — which file, which change>
- [ ] 1.2 <Concrete action>

## Phase 2: <Phase Name>

- [ ] 2.1 <Concrete action>
- [ ] 2.2 <Concrete action>

## Phase N: <Phase Name>

<same shape>

## Dependency Order

| Task | Depends on | Why |
|---|---|---|

<Only real dependencies: task X cannot compile, cannot run, or cannot be
meaningful until task Y lands. "Reads better in this order" is not a dependency
and belongs nowhere. A task with no dependency is the useful signal that it can
run in parallel — say so with an em dash rather than omitting the row.>

## Requirement Traceability

| Requirement | Tasks |
|---|---|

<Every requirement in `specs/` maps to at least one task, and every task maps
back to at least one requirement. A task tracing to nothing is scope creep; a
requirement tracing to nothing is a gap. Report both rather than hiding either.>

## Open Questions

- [ ] <unresolved question, or "None">
```

Every task MUST be specific (names the file and the change), actionable (names the symbol or behavior), verifiable (someone else can tell whether it is done), and small (one file or one logical unit).

## Size budget

`tasks-lens-a.md` MUST be under 1000 words. A checklist is compact by nature — one line per task. If the breakdown does not fit, that is a signal the change is too large for one apply phase; say so in `## Open Questions` rather than merging tasks to save words.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: the Suggested Work Units table, the wave partition, per-unit `allowed_paths`, executor assignment, and whether an `apply-dag.yaml` sidecar is warranted at all.
- **Lens C owns**: the Review Workload Forecast table, per-task acceptance evidence, and the RED-test task for every applicable threat-matrix row.

Do not estimate changed lines and do not name PR boundaries — that is lens C's forecast and lens B's partition. Do not assign an executor to a task.

## Allowed paths

`openspec/changes/<change-id>/tasks-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a task file must contain*: the phased checklist format, the
specific / actionable / verifiable / small rule, and the requirement that every applicable
threat-matrix case becomes an explicit RED-test task before its production task. Where this packet
paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the task breakdown is
split across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
the whole `tasks.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the forecast table, write the work-units table, persist to Engram, return the phase
summary block. Those are superseded here on purpose. Do not correct yourself toward them; note the
conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Done criteria

- [ ] **Every task names a concrete path**, and every "modify" task's path resolves in this worktree.
- [ ] **Every requirement in `specs/` appears in the traceability table with at least one task**, and every gap in either direction is reported rather than hidden.
- [ ] **Every dependency row states why it is a real ordering constraint**, not a preference.
- [ ] **`tasks-lens-a.md` exists, is under 1000 words, and carries `## Assumed decomposition`, `## Dependency Order`, and `## Requirement Traceability`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- A requirement in `specs/` cannot be decomposed into tasks because the design does not say how it is built.
- The design's file-changes table and the specs disagree about what this change does.
- Two tasks are mutually circular: each needs the other to land first.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the design's file-changes table,
the requirement ids in `openspec/changes/<change-id>/specs/`, the packages in
scope, and any decision the human has already made in conversation and does not
want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
