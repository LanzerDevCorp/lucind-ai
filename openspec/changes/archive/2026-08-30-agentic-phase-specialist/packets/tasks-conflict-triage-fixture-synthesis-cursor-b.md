---
id: tasks-conflict-triage-fixture-synthesis-cursor-b
executor: cursor-agent
routed_by: synthesis of three parallel tasks lenses into one canonical tasks-cursor-b.md
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/tasks-cursor-b.md", "openspec/changes/conflict-triage-fixture/tasks-cursor-b-synthesis-notes.md"]
---

# Packet tasks-conflict-triage-fixture-synthesis-cursor-b

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/tasks-conflict-triage-fixture-synthesis-cursor-b  ·  **Branch:** lucind/tasks-conflict-triage-fixture-synthesis-cursor-b

## Goal

Read the three tasks lens drafts for `conflict-triage-fixture`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/conflict-triage-fixture/tasks-cursor-b.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, the apply phase executes.

## Output path

This lane's canonical tasks file is `openspec/changes/conflict-triage-fixture/tasks-cursor-b.md`,
not `tasks.md`. Every instruction below -- the word budget, the spine requirements, the commit
discipline, the self-checks -- applies to it unchanged; read "the canonical tasks file" wherever
the shape of a rule matters more than its filename. Do not read, write, create, or reference
`openspec/changes/conflict-triage-fixture/tasks.md` or `tasks-opencode.md`: both are outside this
lane's `allowed_paths`, and a lane that touches a path it was not granted is rejected at
integration regardless of how good its content is. Synthesize from the three lens drafts only.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` all exist in this worktree.
- `openspec/changes/conflict-triage-fixture/tasks-cursor-b.md` does not yet exist.
- The spec and design for `conflict-triage-fixture` are present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `tasks-lens-a.md` | Phased checklist; dependency order; requirement traceability; a `## Citation Manifest` |
| `tasks-lens-b.md` | Suggested Work Units; wave plan; `allowed_paths`; executor assignment; disjointness check; sidecar recommendation; a `## Citation Manifest` |
| `tasks-lens-c.md` | Review Workload Forecast; RED tests from the threat matrix; acceptance evidence; verification gaps; a `## Citation Manifest` |

All three also emit `## Open Questions`. Merge and deduplicate them. All three also emit a `##
Citation Manifest` and were each required to run a cheap pre-commit existence check over their own
citations before committing — that check is not a substitute for yours; see `## Using the lens
citation manifests` below.

## Required procedure

Do these in order. Skipping step 2, step 3, or step 5 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says. Lens C's proving commands and lens B's path claims matter most: a task whose command does not exist wastes an apply lane.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `tasks-cursor-b.md`** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Decomposition arbitration

The three drafts each opened with `## Assumed decomposition`. Compare them.

- **Lens A's decomposition is authoritative.** It is the lens that owned it.
- A work unit from lens B or an acceptance row from lens C that maps to no task in lens A's checklist does not go into `tasks-cursor-b.md`. Record it under `## Decomposition Divergence` in the notes with what that lens assumed instead.
- If lens B or lens C converged independently on lens A's decomposition, say so. Independent convergence is corroboration and is worth recording.
- If lens A's decomposition is refuted by code you verified in step 2 — a task to modify a file that does not exist, a phase whose dependency is backwards — do not silently substitute your own. That is a hard stop.

### 4. Assemble, do not concatenate

`tasks-cursor-b.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job: a task line from lens A, its unit from lens B, and its proof from lens C are one entry, not three.

### 5. Wave viability re-check

Do not take lens B's "green on its own" column on faith. For every wave, decide independently whether it passes `lucind-checks.sh` on the combined tree by itself.

`Integrate` runs those checks on the combined tree and bisects a failing batch (`internal/run/integrate.go:50-59`). A wave whose accepted outcome is that tests fail is reverted before its successor can turn them green. Strict-TDD RED and GREEN for one unit therefore belong in one lane.

Any wave that cannot be green alone must be merged into its successor before `tasks-cursor-b.md` ships. Record what you merged and why under `## Coverage Gaps`.

Re-check the disjointness claim the same way: for every pair of units sharing a wave, apply the component-boundary prefix rule (`internal/packet/disjoint.go`) yourself. A directory named in `allowed_paths` covers everything beneath it, so two units under one directory collide even when their files do not.

### 6. Coverage check

`tasks-cursor-b.md` must satisfy this repository's tasks spine:

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

### `openspec/changes/conflict-triage-fixture/tasks-cursor-b.md`

The canonical checklist. Under 1800 words. Covers all eight spine items. Contains only claims whose citations you verified in step 2 and which survive lens A's decomposition.

`tasks-cursor-b.md` stays the human checklist. It is not the parse source for dispatch — an `apply-dag.yaml` sidecar is, when one is warranted. If lens B recommended no sidecar, say so in `tasks-cursor-b.md` and name the single-packet shape instead.

### `openspec/changes/conflict-triage-fixture/tasks-cursor-b-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Tasks Synthesis Notes: Conflict Triage Fixture

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

`openspec/changes/conflict-triage-fixture/tasks-cursor-b.md` and `openspec/changes/conflict-triage-fixture/tasks-cursor-b-synthesis-notes.md` only.

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

**Commit `tasks-cursor-b.md` the moment it is written, before you begin the notes file.** Then write the
notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit
leaves finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies
before a single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup`
deletes without warning — which has already cost this project one full synthesis run. Two commits
convert a timeout from lost work into resumable work.

Both commits are conventional, with no AI attribution.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit. It is a deterministic
script, not a judge: it reports whether these facts hold; it does not decide whether your synthesis
is good, and it does not replace your own judgment against `## Done criteria` below.

**Right after the first commit** (`tasks-cursor-b.md`, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/conflict-triage-fixture/tasks-cursor-b.md \
  --budget 1800 --skip-result
```

A `git status --porcelain` FAIL here (the default check, not skipped) means the first commit did
not actually land everything it should have — catch that before you start the notes file, not
after the second commit buries it.

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/conflict-triage-fixture/tasks-cursor-b-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Decomposition Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose. `tasks-cursor-b.md`'s own spine coverage (the eight items in step 6) is
substantive, not a fixed set of heading strings, so the script does not and cannot check it — that
judgment stays yours.

## Done criteria

- [ ] **`tasks-cursor-b.md` was committed as its own commit before `tasks-cursor-b-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `tasks-cursor-b.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every wave was independently judged green-on-its-own**, and every wave that was not is merged and reported under `## Coverage Gaps`.
- [ ] **Every same-wave unit pair was re-checked against the component-boundary prefix rule** rather than taken from lens B's table.
- [ ] **`tasks-cursor-b.md` exists, is under 1800 words, and substantively covers all eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`tasks-cursor-b-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The three `## Assumed decomposition` blocks are mutually irreconcilable and the spec and design do not choose between them. Write the notes file, leave `tasks-cursor-b.md` uncreated, and block.
- Lens A's decomposition is refuted by code you verified. Do not substitute your own.
- No partition exists in which every wave is green on its own and every same-wave pair is disjoint. Report it; the correct answer may be a single packet, but say so rather than shipping a plan `Integrate` will revert.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: Conflict Triage Fixture. Four `ADDED` requirements: **Deterministic three-hunk
fixture**, **Semantic triage and risk ratchet**, **Two-step close and retry CAS**, **Dual-judge
rubric isolation** (`openspec/changes/conflict-triage-fixture/proposal.md:79-133`).

**File changes** (`openspec/changes/conflict-triage-fixture/design.md:79-87`):

| File | Action | Terminal consumer |
|------|--------|-------------------|
| `internal/conflicttriage/types.go` | Create payload types | `Candidate.Output` unmarshal (`reconcile.go:105`) |
| `internal/conflicttriage/triage.go` | Create fail-open agent + invariant calls | Operator `reconcile resolve` (`cli.go:56`) |
| `internal/conflicttriage/invoker.go` | Create `TriageInvoker` func field | Unit tests (stub LLM) |
| `internal/reconcile/reconcile.go` | Modify: output-only update | Agent persist; not `UpdateCandidateStatus` |
| `internal/conflicttriage/fixture/fixture.go` | Create 3-hunk generator + shared `base_sha` | `evaluateOverlapGate` (`attempt.go:687`) |
| `internal/conflicttriage/fixture/rubric.go` | Create A/B grader | `Claude` / `Opencode` |
| `internal/conflicttriage/fixture/packets/` | Create disjoint judge packets | `FeatureTarget` (`integrate_feature.go:17,26-78`) |

**Threat Matrix** (`design.md:104-110`): three rows `Applicable` (Documentation-like paths, Git
repository selection, Commit state), two `N/A` (Push state, PR commands). Every applicable row
needs a RED-test task before its production task in the final checklist; the two `N/A` rows get
none.

**Delivery is a single PR** with a **2000-changed-line review budget** — a human decision, not in
the proposal. The proposal carries no changed-line forecast; lens C's slice is where one was first
produced. Judge lens C's forecast, and the Review Workload Forecast table you assemble, against
this real 2000-line number, not the skill's nominal smaller default that the table's own field name
still carries.

**Strict TDD is active**; the test command is `./lucind-checks.sh`.

**Two questions stay open in the design and MUST NOT be resolved by a task you assemble**: the
exact non-decreasing risk formula and its thresholds, and which executor/model runs *production*
triage (`design.md:118-123`). If any lens smuggled a specific number or executor name into a task
as if it were decided, drop that specificity and record it under `## Dropped Citations` or `##
Decomposition Divergence` as appropriate — do not silently keep it and do not silently soften it
without saying so.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, waves merged for viability, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
