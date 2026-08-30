---
id: tasks-lane-status-observability-synthesis
executor: cursor-agent
routed_by: synthesis of three capability-sliced tasks lenses into one canonical tasks.md
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/lane-status-observability/tasks.md", "openspec/changes/lane-status-observability/tasks-synthesis-notes.md"]
---

# Packet tasks-lane-status-observability-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/tasks-lane-status-observability-synthesis  ·  **Branch:** lucind/tasks-lane-status-observability-synthesis

## Goal

Read the three capability-sliced tasks lens drafts for `lane-status-observability`, verify their
claims against the real code, arbitrate where they disagree, and produce one canonical
`openspec/changes/lane-status-observability/tasks.md` plus a separate synthesis notes file
recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking, the apply phase
executes.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from
the integrated result, so `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` are all
present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `tasks-lens-a.md`, `tasks-lens-b.md`, and `tasks-lens-c.md` all exist in this worktree.
- `openspec/changes/lane-status-observability/tasks.md` does not yet exist.
- `design.md` and `specs/` for `lane-status-observability` are present and accepted.

## What each lens owns

Unlike the generic decomposition/partition/proof three-way split this repository's tasks fan-out
normally uses, this fan-out is sliced by **capability** — the same three groups `design.md` used —
because `lane-status-observability` has six capabilities clustering naturally into three groups.
Each lens therefore wrote its own self-contained checklist, dependency order, requirement
traceability, RED tests, and acceptance evidence for its own slice.

| Draft | Owns |
|---|---|
| `tasks-lens-a.md` | Lane dispatch metadata; extended packet frontmatter; packet-path persistence; dispatched packet-body HTTP endpoint (design.md Decisions 1-4) |
| `tasks-lens-b.md` | Structured progress telemetry (`ProgressEvent`/`LaneProgress` fields, per-decoder wiring, `tool_rate` derivation); the complete v7 STRICT-table migration DDL |
| `tasks-lens-c.md` | Orphan-lane reconciliation: PID capture on `RegisterRun`, the serve-side sweep/ticker, PID-liveness, and the "Process integration" threat-matrix row |

All three also emit `## Open Questions`. Merge and deduplicate them. Two cross-lens dependencies
were flagged deliberately for you to resolve here, not by any single lens: (1) lens C's
`internal/ledger/runs.go` `Run.PID` task depends on lens B's schema-v7 `runs.pid` column; (2) lens A
and lens C both touch `cmd/lucind-ai/cli.go` (different line ranges: `Packet.Path` near
`cli.go:160-174` for A, `RegisterRun`/`Sweeper` near `cli.go:314-324,770-774` for C), and lens A and
lens B both touch `internal/serve/model.go`/`app.js` (different field sets).

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

A lens draft is evidence, not authority. You have the code; use it.

### 3. Cross-capability dependency resolution

Resolve the two named cross-lens dependencies explicitly in the combined dependency order:

- Lens C's `Run.PID` insert/select/scan task must be ordered after lens B's schema-v7 migration
  task.
- Decide, and state, whether the shared-file touches on `cmd/lucind-ai/cli.go` and
  `internal/serve/model.go`/`app.js` need sequential ordering in `tasks.md`'s combined dependency
  table, or whether — since this change ships as a single accepted PR (`size:exception`) rather
  than parallel apply lanes — sequencing them within one packet is sufficient and no `allowed_paths`
  conflict actually blocks anything. Record the reasoning either way.

Any other genuine disagreement across the three `## Assumed decomposition` blocks (not just the two
flagged dependencies) goes under `## Unresolved Contradictions` — do not silently pick.

### 4. Sidecar / dispatch-shape recommendation

Weigh whether an `apply-dag.yaml` sidecar is warranted given: three capability slices, two
cross-lens file/column dependencies, and the user-accepted single-PR (`size:exception`) delivery.
State a final recommendation (single packet vs. DAG) with rationale, the same way lens B normally
would in the generic fan-out — no lens here owned that question, so you decide it. Precedent:
`openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md` and
`conflict-triage-fixture/tasks.md` both declined a DAG despite splittable work, because Strict-TDD
RED/GREEN for one unit belongs in one lane and `Integrate` bisects a failing combined tree
(`internal/run/integrate.go:50-59`). Weigh this change's two real cross-slice dependencies against
that same bar.

### 5. Assemble, do not concatenate

`tasks.md` MUST be under 1800 words (the verbatim v7 DDL excluded, same as it was excluded from
lens B's and `design.md`'s own budgets). The three drafts total roughly 3000 words. Cutting is the
job: a task line from one lens, its RED test, and its acceptance evidence are one entry, not three.

### 6. Coverage check

`tasks.md` must satisfy this repository's tasks spine:

1. Review Workload Forecast table, every field populated (assembled from all three slices'
   acceptance-evidence tables and file counts — no single lens wrote this)
2. Suggested Work Units table, each a standalone deliverable with a rollback boundary (one row per
   lens slice at minimum; split further only if a real sub-boundary exists)
3. Phased checklist, every task specific, actionable, verifiable, and small
4. A RED-test task before its production task for every threat-matrix row `design.md` marked
   `Applicable` (only "Process integration," lens C's) and none for an `N/A` row
5. Explicit dependency order, including the two cross-lens dependencies from step 3
6. Every wave green on its own under `Integrate`, and every same-wave unit pair path-disjoint (only
   relevant if step 4's recommendation is a DAG)
7. Executor named per unit where a DAG is intended
8. Every requirement in `specs/` traced to at least one task

Anything no draft covered goes under `## Coverage Gaps`. Do not invent content to fill a gap; report
it.

## Output

### `openspec/changes/lane-status-observability/tasks.md`

The canonical checklist. Under 1800 words (v7 DDL excluded). Covers all eight spine items. Contains
only claims whose citations you verified in step 2 and which survive the cross-lens dependency
resolution in step 3.

Set the Review Workload Forecast's plain-text guard lines from the already-decided delivery
strategy: `Decision needed before apply: No` (delivery strategy is `exception-ok`), `Chained PRs
recommended: No`, `Chain strategy: size-exception`, `400-line budget risk: High` (six capabilities,
a schema migration, and a new sweeper file — do not understate it; the human has already accepted
the exception).

### `openspec/changes/lane-status-observability/tasks-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Tasks Synthesis Notes: Lane Status Observability

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it.
State both positions and what evidence each has. Do NOT pick — this section is
the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
actually says. "None" if there are none.>

## Decomposition Divergence

<Where the three slices' Assumed decomposition blocks disagreed on ownership or
ordering, including how you resolved the two named cross-lens dependencies and
the sidecar recommendation from step 4. "None — all three converged" only if
genuinely true — the cross-lens dependencies are expected content here, not an
absence.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write `apply-dag.yaml` itself. The sidecar (if recommended) is authored at apply time;
  this phase recommends its shape.
- Do NOT write specs, design, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane; step 3 and step 4 are reading judgments, not executions.

## Allowed paths

`openspec/changes/lane-status-observability/tasks.md` and
`openspec/changes/lane-status-observability/tasks-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill. Check the canonical
checklist against the contract as written: the Review Workload Forecast fields, the Suggested Work
Units columns, the specific/actionable/verifiable/small rule, and the threat-matrix RED-test rule.
On those, the skill wins over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word budget, the capability-sliced
synthesis procedure, the cross-lens dependency resolution, the notes file, and the done criteria.
The skill's Engram persistence step and its return block are superseded: your output is the two
files named above plus `.lucind/result.json`.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as
your **verification worklist, never as evidence**. A manifest row was written by the same lane that
made the claim, so a wrong citation arrives with a confident row beside it. The property that makes
this fan-out trustworthy is that you open every cited range yourself and check it against the real
code in this worktree. That property is not negotiable and the manifests do not relax it.

Each lens also ran a cheap pre-commit existence check over its own manifest before it committed.
That check catches a citation that cannot possibly be right; it says nothing about whether the range
actually supports the claim. Do not treat a lens having run that check as a reason to verify its
citations any less thoroughly.

What the manifests are for is speed without loss:

- **Deduplicate across the three.** Verify each unique citation exactly once, not once per prose
  mention.
- **Batch by file.** Open each cited file once and check every citation into it.
- **Verify the claim, not the line's existence.** A row states what the lens asserts that range
  shows. A range that exists but does not support the claim is a dropped citation.
- **A citation in a lens's prose but missing from its manifest is still yours to verify.** An
  incomplete manifest does not shrink your obligation.

Record each entry's outcome — verified, dropped, or retargeted — in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

**Commit `tasks.md` the moment it is written, before you begin the notes file.** Then write the
notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit leaves
finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies before a
single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup` deletes without
warning — which has already cost this project one full synthesis run. Two commits convert a timeout
from lost work into resumable work.

Both commits are conventional, with no AI attribution. Check `git log -1 --format=%B` after each
one: some executors' commit wrappers append a `Co-authored-by:` trailer the message never
contained. Strip it if present.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root, bracketing each commit. It is a deterministic
script, not a judge: it reports whether these facts hold; it does not decide whether your synthesis
is good, and it does not replace your own judgment against `## Done criteria` below.

**Right after the first commit** (`tasks.md`, before you start the notes file):

```
./lucind-lane-check.sh --file openspec/changes/lane-status-observability/tasks.md --budget 1800 --skip-result
```

A `git status --porcelain` FAIL here (the default check, not skipped) means the first commit did
not actually land everything it should have — catch that before you start the notes file, not after
the second commit buries it.

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/lane-status-observability/tasks-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Decomposition Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence` instead of narrating the same
facts in prose. The canonical document's own spine coverage is substantive, not a fixed set of
heading strings, so the script does not and cannot check it — that judgment stays yours.

## Done criteria

- [ ] **`tasks.md` was committed as its own commit before `tasks-synthesis-notes.md` was started**, confirmed by the mid-flow `lucind-lane-check.sh` run reporting a clean `git status --porcelain`.
- [ ] **Every `file:line` citation surviving into `tasks.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Both named cross-lens dependencies (runs.go-after-schema-v7; the shared `cli.go`/`model.go`/`app.js` touches) are explicitly resolved in the combined dependency order**, not silently dropped.
- [ ] **A final sidecar-vs-single-packet recommendation is stated with rationale**, weighed against the two real cross-slice dependencies and the accepted single-PR delivery.
- [ ] **Every wave (if any DAG is recommended) was independently judged green-on-its-own.**
- [ ] **`tasks.md` exists, is under 1800 words (v7 DDL excluded), and substantively covers all eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`tasks-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None", confirmed by the final `lucind-lane-check.sh` run.
- [ ] **The work is committed with two conventional commits and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The three `## Assumed decomposition` blocks are mutually irreconcilable and `design.md`/`specs/`
  do not choose between them. Write the notes file, leave `tasks.md` uncreated, and block.
- One or more lens drafts is missing from this worktree.
- Lens B's v7 DDL is missing, incomplete, or does not compile as plausible SQL in `tasks-lens-b.md`
  — do not invent or repair it yourself.
- Covering all eight spine items honestly would require exceeding 1800 words (DDL excluded). Report
  which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/lane-status-observability/design.md` and the full `specs/` tree (six
capabilities) are committed in this worktree.** `design-synthesis-notes.md`,
`spec-synthesis-notes.md`, and `proposal-synthesis-notes.md` beside them record citations earlier
synthesis lanes opened and dropped, and known-wrong citations that must not resurface — read all
three before trusting any citation a lens draft repeats from an earlier phase.

**User-approved decisions, already final — do not re-litigate:**

1. Full six-item scope ships as **one PR** (`size:exception`, 1200-line review budget deliberately
   exceeded).
2. Static `skill:` frontmatter only, never live runtime telemetry; generic `tool_calls` is the live
   proxy.
3. `delivery_strategy` is `exception-ok`.
4. Strict TDD is active for this project — every task list must be test-first where this project's
   testing conventions apply.
5. No historical-row backfill anywhere in this change.
6. Linux-only deployment; no cross-platform PID-liveness requirement.

**Out of scope, and including any of it is wrong:** live executor Skill telemetry decoding,
`internal/resolve`/`internal/conflicttriage` (an unrelated change already merged), process
supervision or auto-restart, cross-platform PID-liveness beyond Linux.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter:
citations verified, citations dropped, cross-lens dependencies resolved, contradictions escalated,
coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is
declared.
