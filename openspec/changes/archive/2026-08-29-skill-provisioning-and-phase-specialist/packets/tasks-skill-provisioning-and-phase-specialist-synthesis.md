---
id: tasks-skill-provisioning-and-phase-specialist-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel tasks lenses into one canonical tasks.md
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/skill-provisioning-and-phase-specialist/tasks.md", "openspec/changes/skill-provisioning-and-phase-specialist/tasks-synthesis-notes.md"]
---

# Packet tasks-skill-provisioning-and-phase-specialist-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/tasks-skill-provisioning-and-phase-specialist-synthesis  ·  **Branch:** lucind/tasks-skill-provisioning-and-phase-specialist-synthesis

## Goal

Read the three tasks lens drafts for `skill-provisioning-and-phase-specialist`, verify their claims
against the real code, arbitrate where they disagree, and produce one canonical
`openspec/changes/skill-provisioning-and-phase-specialist/tasks.md` plus a separate synthesis notes
file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking, the apply phase
executes.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated (merge commits `1db8af2`,
`1d07c2a`, `5fbf2e7`). This worktree is branched from the integrated result, so `tasks-lens-a.md`,
`tasks-lens-b.md`, and `tasks-lens-c.md` are all present here. Lens worktrees could not see each
other; this one sees all three.

## Preconditions

- `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` all exist in this worktree.
- `openspec/changes/skill-provisioning-and-phase-specialist/tasks.md` does not yet exist.
- `design.md` and `specs/` for `skill-provisioning-and-phase-specialist` are present and accepted.

## What each lens owns

This fan-out used the repository's generic decomposition/partition/proof three-way split, not a
capability-sliced one.

| Draft | Owns |
|---|---|
| `tasks-lens-a.md` | Phased checklist (4 phases, 15 tasks); dependency order; requirement traceability against the 8 spec requirements |
| `tasks-lens-b.md` | Suggested Work Units (10 units); wave plan (4 waves); `allowed_paths`; executor assignment; disjointness check; sidecar recommendation (leans toward warranted) |
| `tasks-lens-c.md` | Review Workload Forecast (~1,200-1,600 lines, `size:exception`); RED tests from the threat matrix; acceptance evidence (proving commands); verification gaps |

All three also emit `## Open Questions`. Merge and deduplicate them. Lens A flagged one open question
about `sdd-tasks`'s single-writer forecast/work-unit convention being split across B and C in this
packet — that is this packet's known, deliberate shape, not a defect to resolve; note it and move on.

## Required procedure

Do these in order. Skipping step 2, step 3, or step 5 makes the output worthless regardless of how
good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this
worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop
  the claim from `tasks.md`** and record it under `## Dropped Citations` in the notes with what you
  found instead.

A lens draft is evidence, not authority. You have the code; use it. Two prior synthesis passes in
this same change (`design-synthesis-notes.md`, `spec-synthesis-notes.md`) each found real citation
errors in their lens drafts (dropped/retargeted current-behavior claims, wrong line ranges) — treat
that as the base rate for this fan-out, not a reason to sample less.

### 3. Decomposition arbitration

The three drafts each opened with `## Assumed decomposition`. Compare them.

- **Lens A's decomposition is authoritative.** It is the lens that owned it.
- A work unit from lens B or an acceptance row from lens C that maps to no task in lens A's checklist
  does not go into `tasks.md`. Record it under `## Decomposition Divergence` in the notes with what
  that lens assumed instead.
- If lens B or lens C converged independently on lens A's decomposition, say so. Independent
  convergence is corroboration and is worth recording. On a first read, all three appear to converge
  on the same four-package / eleven-file scope from `design.md`'s File Changes table — confirm this
  rather than assuming it.
- If lens A's decomposition is refuted by code you verified in step 2 — a task to modify a file that
  does not exist, a phase whose dependency is backwards — do not silently substitute your own. That
  is a hard stop.

### 4. Assemble, do not concatenate

`tasks.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job: a task
line from lens A, its unit from lens B, and its proof from lens C are one entry, not three.

### 5. Wave viability re-check

Do not take lens B's "green on its own" column on faith. For every wave, decide independently whether
it passes `lucind-checks.sh` on the combined tree by itself.

`Integrate` runs those checks on the combined tree and bisects a failing batch
(`internal/run/integrate.go:50-59`). A wave whose accepted outcome is that tests fail is reverted
before its successor can turn them green. Strict-TDD RED and GREEN for one unit therefore belong in
one lane.

Any wave that cannot be green alone must be merged into its successor before `tasks.md` ships. Record
what you merged and why under `## Coverage Gaps`.

Re-check the disjointness claim the same way: for every pair of units sharing a wave, apply the
component-boundary prefix rule (`internal/packet/disjoint.go`) yourself. A directory named in
`allowed_paths` covers everything beneath it, so two units under one directory collide even when
their files do not. Lens B's own Wave 1 disjointness table already flags that `internal/skillset/`
and `internal/skillroots/` share the string prefix `skill` but are distinct component directories —
re-verify that reasoning against the real `disjoint.go` logic rather than trusting the table.

### 6. Sidecar / dispatch-shape decision

Lens B recommends an `apply-dag.yaml` sidecar (10 units, 4 waves) but also states the fallback: 10
units map directly to 10 sequential work-unit commits inside a single packet if the synthesizer opts
for sequential apply to avoid bisection overhead. Weigh this against `openspec/config.yaml:6-7`
(`delivery_strategy: single-pr`, `review_budget_lines: 10000`) — this change ships as one accepted PR
under a pre-approved size exception, the same posture as
`openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` (cited by lens B), which
declined a DAG sidecar for a smaller change. State a final recommendation with rationale; no lens
owned this decision outright, lens B only leaned toward one answer.

### 7. Coverage check

`tasks.md` must satisfy this repository's tasks spine:

1. Review Workload Forecast table, every field populated
2. Suggested Work Units table, each unit a standalone deliverable with a rollback boundary
3. Phased checklist, every task specific, actionable, verifiable, and small
4. A RED-test task before its production task for every threat-matrix row `design.md` marked
   `Applicable` (four of five rows; `Push state` is `N/A`), and none for the `N/A` row
5. Dependency order stated explicitly
6. Every wave green on its own under `Integrate`, and every same-wave unit pair path-disjoint
7. Executor named per unit wherever a DAG is intended
8. Every requirement in `specs/` traced to at least one task — the 8 requirements are: Deterministic
   multi-tier derivation, Root resolution and fail-closed admission, Versioned Contract and Late
   Target Binding, Extended packet frontmatter parsing, Result envelope skills loaded declaration,
   Frozen Authored Candidate Evidence, Fail-Closed Mechanical Criteria, Specialist sequencing and
   canonical artifact generation (`specs/*/spec.md:5` in each capability directory)

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a
gap; report it.

## Output

### `openspec/changes/skill-provisioning-and-phase-specialist/tasks.md`

The canonical checklist. Under 1800 words. Covers all eight spine items. Contains only claims whose
citations you verified in step 2 and which survive lens A's decomposition.

`tasks.md` stays the human checklist. It is not the parse source for dispatch — an `apply-dag.yaml`
sidecar is, when one is warranted. State your step-6 sidecar decision plainly and name the resulting
dispatch shape (single packet vs. DAG) either way.

### `openspec/changes/skill-provisioning-and-phase-specialist/tasks-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Tasks Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

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
content that cost them, and where they converged independently, plus the step-6
sidecar decision and rationale. "None — all three converged" only if genuinely
true.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write `apply-dag.yaml`. The sidecar is authored at apply time; this phase recommends its
  shape.
- Do NOT write specs, design, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane; step 5 and step 6 are reading judgments, not executions.

## Allowed paths

`openspec/changes/skill-provisioning-and-phase-specialist/tasks.md` and
`openspec/changes/skill-provisioning-and-phase-specialist/tasks-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill. Check the canonical
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

Each lens also ran a cheap pre-commit existence check over its own manifest (does the file exist, is
the line within range) before it committed. That check catches a citation that cannot possibly be
right; it says nothing about whether the range actually supports the claim. Do not treat a lens
having run that check as a reason to verify its citations any less thoroughly — it changes what kind
of wrong citation you are likely to find, not how many you must open.

What the manifests are for is speed without loss:

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose
  mention.
- **Batch by file.** Open each cited file once and check every citation into it, instead of jumping
  between files in prose order.
- **Verify the claim, not the line's existence.** A row states what the lens asserts that range
  shows. A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.** An
  incomplete manifest does not shrink your obligation.

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `tasks.md` the moment it is written, before you begin the notes file.** Then write the notes
and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit leaves
finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies before a
single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup` deletes without
warning — which has already cost this project one full synthesis run. Two commits convert a timeout
from lost work into resumable work.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each one:
some executors' commit wrappers append a `Co-authored-by:` trailer the message never contained. Strip
it if present.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit. It is a deterministic
script, not a judge: it reports whether these facts hold; it does not decide whether your synthesis
is good, and it does not replace your own judgment against `## Done criteria` below.

**Right after the first commit** (`tasks.md`, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks.md --budget 1800 --skip-result
```

A `git status --porcelain` FAIL here (the default check, not skipped) means the first commit did not
actually land everything it should have — catch that before you start the notes file, not after the
second commit buries it.

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/skill-provisioning-and-phase-specialist/tasks-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Decomposition Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose. The canonical document's own spine coverage is substantive, not a
fixed set of heading strings, so the script does not and cannot check it — that judgment stays yours.

## Done criteria

- [ ] **`tasks.md` was committed as its own commit before `tasks-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `tasks.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every wave was independently judged green-on-its-own**, and every wave that was not is merged and reported under `## Coverage Gaps`.
- [ ] **Every same-wave unit pair was re-checked against the component-boundary prefix rule** rather than taken from lens B's table.
- [ ] **A final sidecar-vs-single-packet recommendation is stated with rationale**, weighed against `openspec/config.yaml:6-7` and the `apply-dag-dispatch-hardening` precedent.
- [ ] **`tasks.md` exists, is under 1800 words, and substantively covers all eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`tasks-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether
or not it fired.

- The three `## Assumed decomposition` blocks are mutually irreconcilable and `design.md`/`specs/` do
  not choose between them. Write the notes file, leave `tasks.md` uncreated, and block.
- Lens A's decomposition is refuted by code you verified. Do not substitute your own.
- No partition exists in which every wave is green on its own and every same-wave pair is disjoint.
  Report it; the correct answer may be a single packet, but say so rather than shipping a plan
  `Integrate` will revert.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words. Report which item
  forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change:** Skill Provisioning and the SDD Phase Specialist (`skill-provisioning-and-phase-specialist`).
Chosen candidate: deterministic three-tier skill derivation (`derived ∪ stack ∪ adhoc`) plus a
non-intercepting `phasespec` specialist composing `gentle-ai sdd-status` with lucind-ai dispatch
(`proposal.md:1-9`).

**Design's File Changes table** (`design.md:89-108`): 4 new packages — `internal/skillset/`,
`internal/skillroots/`, `internal/lucindconfig/`, `internal/phasespec/` — plus 11 modified files
across `internal/packet/`, `internal/packetauthor/`, `internal/executor/`, `internal/result/`,
`internal/run/`, `internal/accept/`, `cmd/lucind-ai/`, and `.agents/skills/`/`plugin/.../assets/`.

**Design's Threat Matrix** (`design.md:125-133`): 5 rows — Documentation-like paths, Git repository
selection, Commit state, PR commands all `Applicable`; Push state `N/A: no ref mutation`. Each
`Applicable` row names its Planned RED test.

**Requirement IDs in `specs/`** (8 total, one per capability directory, each at `spec.md:5`):
Deterministic multi-tier derivation (`specs/skill-derivation/`), Root resolution and fail-closed
admission (`specs/skill-root-resolution/`), Versioned Contract and Late Target Binding
(`specs/packet-authoring-contract/`), Extended packet frontmatter parsing
(`specs/read-only-packet-schema/`), Result envelope skills loaded declaration
(`specs/skill-load-correspondence/`), Frozen Authored Candidate Evidence (`specs/lane-execution/`),
Fail-Closed Mechanical Criteria (`specs/acceptance-verifier/`), Specialist sequencing and canonical
artifact generation (`specs/phase-specialist-dispatch/`).

**Delivery strategy already decided** (`openspec/config.yaml:6-7`): `delivery_strategy: single-pr`,
`review_budget_lines: 10000` — this change ships as one PR under a pre-accepted size exception.
Lens C's forecast (~1,200-1,600 changed lines, `400-line budget risk: High`) is expected and already
covered by that exception; do not treat it as a new decision to make.

**Decisions already made in design synthesis — do not re-litigate:**

1. `remediate` is included in the closed `sdd_phase` set; it derives no phase skill until
   `sdd-remediate` exists (`design.md:51`, `design-synthesis-notes.md:68-76`).
2. No stub `lucind-archive`/`lucind-ultrafixer` skills are authored; those roles have an empty
   lucind-child tier (`design.md:55-60`, Decision 7).
3. `skillset.DigestBody` elides the `## Required skills` section from both `Compile`'s digest and
   `packetDigest`; canonical names still hash through `contractJSON` and explicit field-list entries
   (`design.md:61-67`, Decision 8 — this is a post-validation correction with no lens ancestor, per
   `design-synthesis-notes.md:78-84`).
4. `packetDigest` (`run.go:722-729`) and the `accept.go:275-286` decode struct must change in the
   same commit whenever a new `packet.Packet` field is added — a repeated theme across all three
   lenses' Wave/Phase groupings; do not let the synthesis split them into different waves/units.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations
verified, citations dropped, waves merged for viability, contradictions escalated, coverage gaps.
Report `done` only when every done-criterion carries evidence and every hard stop is declared.
