---
id: tasks-<change-id>-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel tasks lenses into one canonical tasks.md
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/<change-id>/tasks.md", "openspec/changes/<change-id>/tasks-synthesis-notes.md"]
---

# Packet tasks-<change-id>-synthesis

**Tier:** A (human merge)
**Worktree:** ../<repo>-worktrees/tasks-<change-id>-synthesis  ·  **Branch:** lucind/tasks-<change-id>-synthesis

## Goal

Read the three tasks lens drafts for `<change-id>`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/<change-id>/tasks.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, the apply phase executes.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` all exist in this worktree.
- `openspec/changes/<change-id>/tasks.md` does not yet exist.
- The spec and design for `<change-id>` are present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `tasks-lens-a.md` | Phased checklist; dependency order; requirement traceability |
| `tasks-lens-b.md` | Suggested Work Units; wave plan; `allowed_paths`; executor assignment; disjointness check; sidecar recommendation |
| `tasks-lens-c.md` | Review Workload Forecast; RED tests from the threat matrix; acceptance evidence; verification gaps |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2, step 3, or step 5 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says. Lens C's proving commands and lens B's path claims matter most: a task whose command does not exist wastes an apply lane.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `tasks.md`** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Decomposition arbitration

The three drafts each opened with `## Assumed decomposition`. Compare them.

- **Lens A's decomposition is authoritative.** It is the lens that owned it.
- A work unit from lens B or an acceptance row from lens C that maps to no task in lens A's checklist does not go into `tasks.md`. Record it under `## Decomposition Divergence` in the notes with what that lens assumed instead.
- If lens B or lens C converged independently on lens A's decomposition, say so. Independent convergence is corroboration and is worth recording.
- If lens A's decomposition is refuted by code you verified in step 2 — a task to modify a file that does not exist, a phase whose dependency is backwards — do not silently substitute your own. That is a hard stop.

### 4. Assemble, do not concatenate

`tasks.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job: a task line from lens A, its unit from lens B, and its proof from lens C are one entry, not three.

### 5. Wave viability re-check

Do not take lens B's "green on its own" column on faith. For every wave, decide independently whether it passes `lucind-checks.sh` on the combined tree by itself.

`Integrate` runs those checks on the combined tree and bisects a failing batch (`internal/run/integrate.go:50-59`). A wave whose accepted outcome is that tests fail is reverted before its successor can turn them green. Strict-TDD RED and GREEN for one unit therefore belong in one lane.

Any wave that cannot be green alone must be merged into its successor before `tasks.md` ships. Record what you merged and why under `## Coverage Gaps`.

Re-check the disjointness claim the same way: for every pair of units sharing a wave, apply the component-boundary prefix rule (`internal/packet/disjoint.go`) yourself. A directory named in `allowed_paths` covers everything beneath it, so two units under one directory collide even when their files do not.

### 6. Coverage check

`tasks.md` must satisfy this repository's tasks spine:

1. Review Workload Forecast table, every field populated
2. Suggested Work Units table, each unit a standalone deliverable with a rollback boundary
3. Phased checklist, every task specific, actionable, verifiable, and small
4. A RED-test task before its production task for every threat-matrix row the design marked `Applicable`, and none for a row it marked `N/A`
5. Dependency order stated explicitly
6. Every wave green on its own under `Integrate`, and every same-wave unit pair path-disjoint
7. Executor named per unit wherever a DAG is intended
8. Every requirement in `specs/` traced to at least one task

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/<change-id>/tasks.md`

The canonical checklist. Under 1800 words. Covers all eight spine items. Contains only claims whose citations you verified in step 2 and which survive lens A's decomposition.

`tasks.md` stays the human checklist. It is not the parse source for dispatch — an `apply-dag.yaml` sidecar is, when one is warranted. If lens B recommended no sidecar, say so in `tasks.md` and name the single-packet shape instead.

### `openspec/changes/<change-id>/tasks-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Tasks Synthesis Notes: <Change Title>

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it —
lens B's partition needing an order lens A's dependencies forbid, lens C's
forecast implying a split lens B did not make. State both positions and what
evidence each has. Do NOT pick — this section is the escalation. "None" if there
are none.>

## Coverage Gaps

<Spine items no draft covered, plus every wave you merged in step 5 and why.
"None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
actually says. "None" if there are none.>

## Decomposition Divergence

<What lens B or lens C assumed that differed from lens A's decomposition, what
content that cost them, and where they converged independently. "None — all three
converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write `apply-dag.yaml`. The sidecar is authored at apply time; this phase recommends its shape.
- Do NOT write specs, design, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis lane; step 5 is a reading judgment, not an execution.

## Allowed paths

`openspec/changes/<change-id>/tasks.md` and `openspec/changes/<change-id>/tasks-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` tasks skill (delivered under `## Required skills`). Check the canonical
checklist against the contract as written: the Review Workload Forecast fields, the Suggested Work
Units columns, the specific / actionable / verifiable / small rule, and the threat-matrix RED-test
rule. On those, the skill wins over this packet's paraphrase, and the drift goes in
`## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word budget, the synthesis procedure, the
wave viability re-check, the notes file, and the done criteria. The skill's Engram persistence step
and its return block are superseded: your output is the two files named above plus
`.lucind/result.json`.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as
your **verification worklist, never as evidence**. A manifest row was written by the same lane that
made the claim, so a wrong citation arrives with a confident row beside it. The property that makes
this fan-out trustworthy is that you open every cited range yourself and check it against the real
code in this worktree. That property is not negotiable and the manifests do not relax it.

Each lens also ran a cheap pre-commit existence check over its own manifest (does the file exist,
is the line within range) before it committed. That check catches a citation that cannot possibly
be right; it says nothing about whether the range actually supports the claim. Do not treat a lens
having run that check as a reason to verify its citations any less thoroughly — it changes what
kind of wrong citation you are likely to find, not how many you must open.

What the manifests are for is speed without loss:

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose
  mention.
- **Batch by file.** Open each cited file once and check every citation into it, instead of
  jumping between files in prose order.
- **Verify the claim, not the line's existence.** A row states what the lens asserts that range
  shows. A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.** An
  incomplete manifest does not shrink your obligation.

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `tasks.md` the moment it is written, before you begin the notes file.** Then write the
notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit
leaves finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies
before a single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup`
deletes without warning — which has already cost this project one full synthesis run. Two commits
convert a timeout from lost work into resumable work.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each
one: some executors' commit wrappers append a `Co-authored-by:` trailer the message never
contained. Strip it if present.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit. It is a deterministic
script, not a judge: it reports whether these facts hold; it does not decide whether your synthesis
is good, and it does not replace your own judgment against `## Done criteria` below.

**Right after the first commit** (`tasks.md`, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/<change-id>/tasks.md --budget 1800 --skip-result
```

A `git status --porcelain` FAIL here (the default check, not skipped) means the first commit did
not actually land everything it should have — catch that before you start the notes file, not
after the second commit buries it.

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/<change-id>/tasks-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Decomposition Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose. The canonical document's own spine coverage is substantive, not
a fixed set of heading strings, so the script does not and cannot check it — that judgment stays
yours.

## Done criteria

- [ ] **`tasks.md` was committed as its own commit before `tasks-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `tasks.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every wave was independently judged green-on-its-own**, and every wave that was not is merged and reported under `## Coverage Gaps`.
- [ ] **Every same-wave unit pair was re-checked against the component-boundary prefix rule** rather than taken from lens B's table.
- [ ] **`tasks.md` exists, is under 1800 words, and substantively covers all eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`tasks-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final
      `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The three `## Assumed decomposition` blocks are mutually irreconcilable and the spec and design do not choose between them. Write the notes file, leave `tasks.md` uncreated, and block.
- Lens A's decomposition is refuted by code you verified. Do not substitute your own.
- No partition exists in which every wave is green on its own and every same-wave pair is disjoint. Report it; the correct answer may be a single packet, but say so rather than shipping a plan `Integrate` will revert.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

<The change title, the design's threat matrix and file-changes table, the
requirement ids in `openspec/changes/<change-id>/specs/`, the delivery strategy
the human chose, and any decision already made in conversation that should not be
re-litigated.>

## Required skills

- <sdd-tasks>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, waves merged for viability, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
