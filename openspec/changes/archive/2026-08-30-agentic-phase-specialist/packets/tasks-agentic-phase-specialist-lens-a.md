---
id: tasks-agentic-phase-specialist-lens-a
executor: agy
routed_by: decomposition and ordering lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/tasks-lens-a.md"]
---

# Packet tasks-agentic-phase-specialist-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-agentic-phase-specialist-lens-a  ·  **Branch:** lucind/tasks-agentic-phase-specialist-lens-a

## Goal

Produce `openspec/changes/agentic-phase-specialist/tasks-lens-a.md`: the phased implementation checklist for the **Agentic Phase Specialist** change and the dependency order behind it — what must exist before what, and why.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

The spec and design for `agentic-phase-specialist` are accepted and frozen (design.md went through two bounded corrections and is now clean). Lens B and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another. This lens owns the decomposition; the other two partition it for dispatch and attach proof to it.

## Preconditions

- `openspec/changes/agentic-phase-specialist/specs/` and `openspec/changes/agentic-phase-specialist/design.md` both exist.
- `openspec/changes/agentic-phase-specialist/tasks-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *what work exists and in what order* — not to how it is dispatched and not to how it is proven:

1. The real `gentle-ai` tasks skill (delivered under `## Required skills`). It is the phase contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/agentic-phase-specialist/design.md`, and its `## File Changes` table in particular (lines ~92-108) — every file created, modified, or deleted, and the terminal consumer of each. Note the Decision 3 Hard Rule replacement block (old→new, lines ~39-47) is already literal and copy-ready — a task should reference applying it, not re-derive wording.
3. `openspec/changes/agentic-phase-specialist/specs/` — all four capability files (`phase-verdict-reporting`, `phase-specialist-dispatch`, `acceptance-verifier`, `sdd-planning-fan-out`) — every requirement, so no task exists that no requirement asks for and no requirement lacks a task.
4. `internal/accept/accept.go`, `internal/accept/accept_test.go`, `internal/run/attempt.go`, `internal/run/attempt_test.go` — confirm current line numbers/shape before writing a "modify" task; design.md's citations may have shifted slightly since it was written.
5. `plugin/claude-code/skills/lucind-ai/SKILL.md`, `references/strategies/fan-out.md`, `references/contracts/acceptance-promotion.md` (and their OpenCode mirrors) — confirm current line numbers for the doc-edit tasks.

## Known scope boundary (do not expand)

Per design.md's Out of Scope: no Bash/Agent tools for `sdd-*` this Change; no Promotion delegation; no `AuthoringEvidence`/schema changes; no `integrate.Check` internals changes; no `allowed_paths`/hard-stop changes. The `~/.claude/skills/sdd-*/SKILL.md` edits are **out of repository** — no task may target them with a Lane `allowed_paths`; design.md §"File Changes" already documents the paste-ready text as a human-applied, post-Change handoff. Phrase the corresponding task as "hand off the paste-ready text from design.md to a human/separate process," not as an in-Lane file write.

`openspec/config.yaml` sets `strict_tdd: true` — every code-touching task must be preceded by its own RED-test task (own that phrasing here at the checklist level; lens C separately maps RED tests to threat-matrix rows and acceptance evidence, don't duplicate its table).

Never guess at a file. Every task names a concrete path, and every path claim carries a `file:line` citation or is explicitly marked "new file".

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/tasks-lens-a.md`:

```markdown
# Tasks Lens A — Decomposition & Ordering: Agentic Phase Specialist

## Assumed decomposition

<2-4 sentences naming the work breakdown you are ordering: how many phases, what
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

`openspec/changes/agentic-phase-specialist/tasks-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` tasks skill and its `references/` (delivered under `## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a task file must contain*: the phased checklist format, the specific / actionable / verifiable / small rule, and the requirement that every applicable threat-matrix case becomes an explicit RED-test task before its production task. Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the task breakdown is split across three parallel lanes, which slice this lane owns, its word budget, its output path and skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing the whole `tasks.md` by itself, so parts of it will read as instructing you to do what this packet forbids — write the forecast table, write the work-units table, persist to Engram, return the phase summary block. Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row per unique citation.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft -- never copy this placeholder row verbatim> |

Rules:

- **One row per UNIQUE citation.** A range cited three times in the prose appears once here.
- **Group by file**, files alphabetical, line numbers ascending within each file.
- **The claim is what YOU assert that range shows** — one line, stated plainly, no hedging.
- **This section does not count against the word budget.**
- **The manifest is a worklist, not a certificate.** The synthesizer opens and checks every single one.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/tasks-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" --require-section "Assumed decomposition" \
  --require-section "Dependency Order" --require-section "Requirement Traceability" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/tasks-lens-a.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every task names a concrete path**, and every "modify" task's path resolves in this worktree.
- [ ] **Every requirement in `specs/` appears in the traceability table with at least one task**, and every gap in either direction is reported rather than hidden.
- [ ] **Every dependency row states why it is a real ordering constraint**, not a preference.
- [ ] **`tasks-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed decomposition`, `## Dependency Order`, `## Requirement Traceability`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- A requirement in `specs/` cannot be decomposed into tasks because the design does not say how it is built.
- The design's file-changes table and the specs disagree about what this change does.
- Two tasks are mutually circular: each needs the other to land first.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`, accepted `design.md` (7 File Changes rows: `SKILL.md` Hard Rule carve-out both trees; `fan-out.md` synthesis-review move both trees; `acceptance-promotion.md` decision-bearing Acceptance + steps 1/8 conditional caveat both trees; `internal/accept/accept.go` unconditional metadata load + gated `v.check`; `internal/accept/accept_test.go`; `internal/run/attempt.go` gated `checkFunc`; `internal/run/attempt_test.go`), plus one out-of-repository handoff item (`~/.claude/skills/sdd-*/SKILL.md`, human-applied, not a Lane task).

**Four accepted capabilities to trace** (`openspec/changes/agentic-phase-specialist/specs/`): `phase-verdict-reporting` (ADDED), `phase-specialist-dispatch` (MODIFIED), `acceptance-verifier` (MODIFIED), `sdd-planning-fan-out` (MODIFIED).

**strict_tdd: true`** (`openspec/config.yaml`) — every Go code change needs a preceding RED-test task in your phase breakdown, even though lens C owns the detailed RED-test-to-threat-matrix mapping; your phased checklist should still show test-before-implementation ordering as a phase-level pattern (e.g. "Phase N: RED tests for X" before "Phase N+1: implement X").

## Required skills

- sdd-tasks

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
