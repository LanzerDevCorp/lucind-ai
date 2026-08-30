---
id: tasks-deterministic-lucind-ai-orchestrator-lens-a
executor: agy
routed_by: decomposition and ordering lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-a.md"]
---

# Packet tasks-deterministic-lucind-ai-orchestrator-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/tasks-deterministic-lucind-ai-orchestrator-lens-a  ·  **Branch:** lucind/tasks-deterministic-lucind-ai-orchestrator-lens-a

## Goal

Produce `openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-a.md`: the phased
implementation checklist for this change and the dependency order behind it — what must exist
before what, and why.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final
checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

The spec and design for `deterministic-lucind-ai-orchestrator` are accepted and frozen. Lens B and
lens C run in parallel against the same frozen inputs and write to different files, so no lane
races another. This lens owns the decomposition; the other two partition it for dispatch and
attach proof to it.

## Preconditions

- `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` and
  `openspec/changes/deterministic-lucind-ai-orchestrator/design.md` both exist.
- `openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *what work exists and in what
order* — not to how it is dispatched and not to how it is proven:

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/deterministic-lucind-ai-orchestrator/design.md`, and its `## File Changes`
   table in particular — every file created, modified, or deleted, and the terminal consumer of
   each.
3. `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` — every requirement across the
   five capability deltas, so no task exists that no requirement asks for and no requirement lacks
   a task.
4. The packages the change lands in (`cmd/lucind-ai`, `internal/packet`, `internal/dag`,
   `internal/run`), enough to know what already exists. A task that says "create" a file that
   already exists is wrong, and a task that says "modify" one that does not is worse.

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line`
citation or is explicitly marked "new file".

## Output format

Write exactly this skeleton to
`openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-a.md`:

```markdown
# Tasks Lens A — Decomposition & Ordering: Deterministic lucind-ai Orchestrator

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

Every task MUST be specific (names the file and the change), actionable (names the symbol or
behavior), verifiable (someone else can tell whether it is done), and small (one file or one
logical unit).

## Size budget

`tasks-lens-a.md` MUST be under 1000 words. A checklist is compact by nature — one line per task.
If the breakdown does not fit, that is a signal the change is too large for one apply phase; say
so in `## Open Questions` rather than merging tasks to save words.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens B owns**: the Suggested Work Units table, the wave partition, per-unit `allowed_paths`,
  executor assignment, and whether an `apply-dag.yaml` sidecar is warranted at all.
- **Lens C owns**: the Review Workload Forecast table, per-task acceptance evidence, and the
  RED-test task for every applicable threat-matrix row.

Do not estimate changed lines and do not name PR boundaries — that is lens C's forecast and lens
B's partition. Do not assign an executor to a task.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-a.md` only. Create no other
file.

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

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file.

| citation | claim |
|---|---|
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:70` | design's File Changes table lists the OpenCode skill tree as Create, confirming it does not exist yet on this branch |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" --require-section "Assumed decomposition" \
  --require-section "Dependency Order" --require-section "Requirement Traceability" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-a.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the
  claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL
  against this draft's own manifest.**
- [ ] **Every task names a concrete path**, and every "modify" task's path resolves in this
  worktree.
- [ ] **Every requirement in `specs/` appears in the traceability table with at least one task**,
  and every gap in either direction is reported rather than hidden.
- [ ] **Every dependency row states why it is a real ordering constraint**, not a preference.
- [ ] **`tasks-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries
  `## Assumed decomposition`, `## Dependency Order`, `## Requirement Traceability`, and
  `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- A requirement in `specs/` cannot be decomposed into tasks because the design does not say how it
  is built.
- The design's file-changes table and the specs disagree about what this change does.
- Two tasks are mutually circular: each needs the other to land first.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- `design.md`'s File Changes table (`design.md:66-78`) names nine files/trees: the Claude skill
  tree (Modify), the OpenCode skill tree (Create, byte copy), `internal/packet/packet.go`,
  `internal/dag/split.go`, `internal/run/run.go`, `internal/run/batch.go`,
  `internal/run/attempt.go`, `internal/run/integrate_feature.go`, `cmd/lucind-ai/cli.go`.
- Five accepted spec requirements, one per capability
  (`openspec/changes/deterministic-lucind-ai-orchestrator/specs/`):
  `deterministic-orchestrator-contract` → *Cross-Runtime Orchestrator Preflight and Sequencing*;
  `sdd-apply` → *Orchestrator Advances Only on a Passing Wave*; `parent-feature-integration` →
  *Recoverable Idempotent Attempts*; `packet-authoring-contract` → *Versioned Contract and Late
  Target Binding* (MODIFIED, one new scenario for omitted `allowed_paths`); `acceptance-verifier`
  → *Fail-Closed Mechanical Criteria* (MODIFIED, one new scenario for fired-hard-stop demotion).
- `design.md`'s Flow (`design.md:53-64`) names six numbered steps: Preflight, Bind+Admit,
  Split/Waves, Barrier, Evidence, Integrate/CAS/Recover — a reasonable phase-ordering hint, not a
  binding decomposition.
- This Change's base includes the just-merged `feature/skill-provisioning-and-phase-specialist`
  (`61aa0cc`); `design.md`'s own `## Out of Scope` (`design.md:119`) explicitly excludes building
  or depending on that branch's `LUCIND_REQUIRED_SKILLS`, `required_skills` frontmatter,
  `integrate retry`, or `defect record/list/resolve/decline/defer` — do not schedule a task for
  any of them.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`); Delivery strategy is `ask-on-risk` with a 5000-changed-line review budget
before stopping for a chained-PR decision (human-selected this session) — lens C owns applying
this, you only need to know it exists so your phase breakdown does not silently assume a single PR.

## Required skills

- ~/.claude/skills/sdd-tasks/SKILL.md

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
