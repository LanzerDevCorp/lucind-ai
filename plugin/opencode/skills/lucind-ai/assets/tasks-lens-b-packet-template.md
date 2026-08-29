---
id: tasks-<change-id>-lens-b
executor: agy
routed_by: partition and dispatch-shape lens of the three-lens tasks fan-out
model: <model, e.g. gemini-3.7-flash-high>
allowed_paths: ["openspec/changes/<change-id>/tasks-lens-b.md"]
---

# Packet tasks-<change-id>-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../<repo>-worktrees/tasks-<change-id>-lens-b  ·  **Branch:** lucind/tasks-<change-id>-lens-b

## Goal

Produce `openspec/changes/<change-id>/tasks-lens-b.md`: how this change's work is partitioned into dispatchable units — the work-unit table, the wave plan, each unit's `allowed_paths`, its executor, and whether an `apply-dag.yaml` sidecar is warranted at all.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

The spec and design for `<change-id>` are accepted and frozen. Lens A and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the task decomposition and is running concurrently, so you do not have it. Derive the work from the design's file-changes table, declare it in `## Assumed decomposition`, and partition that. The synthesizer arbitrates divergence.

## Why this lens exists

A partition that looks parallel and is not costs a whole wave. Two failure modes are specific to this repository and both are checkable before dispatch:

- **The `allowed_paths` prefix trap.** Scope matching is a component-boundary prefix match (`internal/packet/disjoint.go`), so naming a directory covers everything beneath it. Two units that name the same directory are rejected as overlapping before any lane starts, even if the files they touch are disjoint. Name files, not directories, wherever two units share a parent.
- **The `Integrate` gate.** `Integrate` runs `lucind-checks.sh` on the combined tree and bisects a failing batch (`internal/run/integrate.go:50-59`). Every wave must pass those checks **on its own**. A wave whose accepted outcome is that tests fail is reverted before its successor can turn them green — which means strict-TDD RED and GREEN for one unit belong in one lane, never in two waves.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *how the work is dispatched* — not to what the work is and not to how it is proven:

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill, and its **Suggested
   Work Units** table in particular. It is the phase contract this draft feeds; read it rather than
   trusting this packet's paraphrase of it.
2. `openspec/changes/<change-id>/design.md` — the file-changes table is the partition's input.
3. `internal/packet/disjoint.go` — the real matching rule, so the disjointness you claim is the
   disjointness the binary checks.
4. `internal/run/integrate.go` — the gate and the bisection, so every wave you propose can survive it.
5. `internal/dag/` — the sidecar's node shape, including which fields a node actually carries.
6. `openspec/changes/archive/` for a prior change that used an `apply-dag.yaml`, and for
   `2026-08-20-apply-dag-dispatch-hardening/tasks.md`, which **declined** a DAG split because the
   units were too small to pay for sidecar orchestration. Declining is a legitimate outcome here.

Never claim two units are disjoint without checking their paths against the prefix rule by hand.

## Output format

Write exactly this skeleton to `openspec/changes/<change-id>/tasks-lens-b.md`:

```markdown
# Tasks Lens B — Partition & Dispatch Shape: <Change Title>

## Assumed decomposition

<2–4 sentences naming the work breakdown you are partitioning: how many units,
what each delivers, and what the critical path is. Lens A and lens C write this
same block independently; the synthesizer compares all three. Be specific enough
that a disagreement is visible.>

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|

<One row per standalone deliverable. `allowed_paths` lists concrete paths, not
prose. Executor is `agy` for broad mechanical sweeps, `cursor-agent` for one
bounded judgment-heavy artifact, `opencode` where the repository convention calls
for it. Rollback boundary names what reverting this unit alone restores.>

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|

<Units in the same wave dispatch together and must have pairwise-disjoint
`allowed_paths`. The last column is a yes/no claim that this wave passes
`lucind-checks.sh` on the combined tree by itself, with the reason. A "no"
invalidates the partition — merge that wave into its successor rather than
shipping a plan that Integrate will revert.>

## Disjointness Check

<For every pair of units in the same wave, the two path sets and the verdict
under the component-boundary prefix rule. Do this by hand, pair by pair. A wave
of one unit needs no check — say so.>

## Sidecar Recommendation

**Recommendation**: <sidecar warranted / single packet, no sidecar>
**Rationale**: <why. A two-node DAG whose first unit is trivial does not pay for
`apply-dag.yaml` plus `lucind-ai split` plus per-wave sequencing; cite the
archived precedent if that is the call here.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`tasks-lens-b.md` MUST be under 1000 words. Tables over prose. The disjointness check is one line per pair, not a paragraph per pair.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: the phased checklist itself, the dependency-order table, and requirement traceability.
- **Lens C owns**: the Review Workload Forecast table, per-task acceptance evidence, and the RED-test task for every applicable threat-matrix row.

Do not write the task checklist. You partition units, not tasks. Do not estimate changed lines or name a PR split — that is lens C's forecast, and your unit boundaries are its input, not its output.

## Allowed paths

`openspec/changes/<change-id>/tasks-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a task file must contain*: the Suggested Work Units table and its
columns, and the rule that a unit is a standalone deliverable with its own rollback boundary. Where
this packet paraphrases any of that and drifts, the skill wins and the drift belongs in
`## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the task breakdown is
split across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
the whole `tasks.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the checklist, write the forecast, persist to Engram, return the phase summary
block. Those are superseded here on purpose. Do not correct yourself toward them; note the conflict
in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft -- never copy this placeholder row verbatim> |

Rules:

- **One row per UNIQUE citation.** A range cited three times in the prose appears once here.
- **Group by file**, files alphabetical, line numbers ascending within each file.
- **The claim is what YOU assert that range shows** — one line, stated plainly, no hedging. Not a
  description of the file; the specific thing you are using it as evidence for.
- **This section does not count against the word budget.** Never trim analysis to make room for
  it, and never trim it to fit the budget.
- **The manifest is a worklist, not a certificate.** Listing a citation here asserts nothing about
  its correctness — the synthesizer opens and checks every single one. Writing a row does not
  spare you from getting the citation right; it makes getting it wrong cheaper to catch.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice. It is a deterministic script, not a judge:
it reports whether these facts hold; it does not decide whether your draft is good, and it does
not replace your own judgment against `## Done criteria` below.

**Before you commit**, while content is still cheap to fix:

```
./lucind-lane-check.sh --file openspec/changes/<change-id>/tasks-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Suggested Work Units" \
  --require-section "Wave Plan" --require-section "Disjointness Check" \
  --require-section "Sidecar Recommendation" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

`--verify-citations` is an existence/grep-level check over your own `## Citation Manifest`: does
the cited file exist, does it have enough lines to contain the cited range. It asserts nothing
about whether the range supports your claim — the synthesizer still opens and checks every
citation itself in the next phase. A FAIL here is cheaper to fix now than after synthesis catches
it.

**After you commit and write `.lucind/result.json`**, confirm the bookkeeping:

```
./lucind-lane-check.sh --file openspec/changes/<change-id>/tasks-lens-b.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every pair of units sharing a wave was checked by hand against the component-boundary prefix rule**, and the verdict is recorded in `## Disjointness Check`.
- [ ] **Every wave's "green on its own" column is a yes with a reason**, or the wave was merged into its successor.
- [ ] **Every unit names concrete `allowed_paths` and an executor**, and every path claim resolves in this worktree or is marked "new file".
- [ ] **`tasks-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed decomposition`, `## Wave Plan`, `## Disjointness Check`, `## Sidecar Recommendation`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final
      `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.
      Check `git log -1 --format=%B` after committing: some executors' commit wrappers append a
      `Co-authored-by:` trailer the message never contained. Strip it if present.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- No partition exists in which every wave is green on its own, so the change cannot be dispatched as waves at all. Say so; a single packet is the correct answer and is not a failure.
- Two units that must run in the same wave cannot have disjoint `allowed_paths` because they edit the same file.
- The design's file-changes table does not determine which unit owns a file.
- Satisfying one instruction in this packet would require violating another.

## Context

<Ground-truth facts with file:line references: the design's file-changes table,
the current `internal/dag` node fields, the archived sidecar precedent, and any
decision the human has already made in conversation and does not want
re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
