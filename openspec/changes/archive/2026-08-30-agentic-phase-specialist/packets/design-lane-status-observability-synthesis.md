---
id: design-lane-status-observability-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel design lenses into one canonical design
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/lane-status-observability/design.md", "openspec/changes/lane-status-observability/design-synthesis-notes.md"]
---

# Packet design-lane-status-observability-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/design-lane-status-observability-synthesis  ·  **Branch:** lucind/design-lane-status-observability-synthesis

## Goal

Read the three capability-sliced design lens drafts for `lane-status-observability`, verify their
claims against the real code, arbitrate where they disagree, and produce one canonical
`openspec/changes/lane-status-observability/design.md` plus a separate synthesis notes file
recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from
the integrated result, so `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` are all
present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `openspec/changes/lane-status-observability/design-lens-a.md`, `-lens-b.md`, and `-lens-c.md`
  all exist in this worktree.
- `openspec/changes/lane-status-observability/design.md` does not yet exist.
- `openspec/changes/lane-status-observability/proposal.md` and `specs/` are present and accepted.

## What each lens owns

Unlike the generic decisions/surface/testing three-way split used on other changes, this fan-out
is sliced by **capability**, because `lane-status-observability` has six capabilities that cluster
naturally into three groups. Each lens therefore covers its own decisions, surface deltas, file
changes, and testing notes together.

| Draft | Owns |
|---|---|
| `design-lens-a.md` | Lane metadata dispatch persistence; extended packet frontmatter (final `sdd_phase`/`fanout_group`/`skill` key names, resolving Open Question 1); packet-path persistence mechanism (resolving Open Question 2); dispatched packet-body HTTP endpoint; DAG-wave `Node`/`EmitPacketContent` scope call (resolving Open Question 5) |
| `design-lens-b.md` | Structured progress telemetry (`ProgressEvent`/`LaneProgress` fields, per-decoder wiring, the tool-count-vs-rate resolution); the **complete v7 STRICT-table migration DDL** for both `runs.pid` and `lane_progress`'s usage columns |
| `design-lens-c.md` | Orphan-lane reconciliation: PID capture on `RegisterRun`, the `serve`-side startup-sweep-plus-ticker architecture, ticker interval (resolving Open Question 3), PID-liveness mechanism (resolving Open Question 4), and the process-integration threat matrix |

All three also emit `## Open Questions`. Merge and deduplicate them. **A design lens's job here
was to CLOSE Open Questions 1-5, not carry them forward** — if a lens's `## Open Questions` still
lists one of the five numbered questions from `proposal.md`, that is a lens failure worth flagging
under `## Unresolved Contradictions`, not a quiet pass-through into `design.md`'s own open
questions.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it
reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this
worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop
  the claim from `design.md`** and record it under `## Dropped Citations` in the notes with what
  you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Open-question resolution check

For each of Open Questions 1 through 5 (`proposal.md`'s `## Open Questions`), confirm the owning
lens actually made a final, concrete choice rather than restating the question. Record the
resolution explicitly in `design.md` (e.g. "Open Question 1 resolved: frontmatter keys are
`sdd_phase`, `fanout_group`, `skill`"). If a lens left one genuinely unresolved with real
justification (its hard stops fired), keep it open in `design.md` and say which lens's hard stop
caused it — do not silently invent a decision the lens declined to make.

### 4. Cross-lens consistency check

Lens C's `runs.pid` operational design assumes a column shape lens B actually owns. Lens A's
`design-lens-a.md` may reference a `LaneMetadata.Skill` field lens A itself is adding — confirm
lens B's and lens C's telemetry/PID additions to `LaneMetadata`/`Lane`/DTOs (if any) do not
silently duplicate or contradict lens A's own additions to the same structs. Record any real
contradiction under `## Unresolved Contradictions`; record confirmed consistency as corroboration.

### 5. Compress — do not concatenate

`design.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job:
merge overlapping statements, drop restatement, keep the specific sentence over the general one. A
concatenation of three drafts is a failed synthesis even if every word in it is true. The complete
v7 migration DDL from lens B's `## Interfaces / Contracts` is excluded from this count, same as it
was excluded from lens B's own budget — do not trim SQL to make room.

### 6. Coverage check

`design.md` must cover this repository's actual design spine, derived from every archived design
in `openspec/changes/archive/`:

1. Technical approach or recommendations at a glance
2. Architecture decisions, each with choice / alternatives considered / rationale
3. Flow and invariants
4. File changes, with terminal consumers
5. Testing strategy and test seams
6. Threat matrix — every row `Applicable` or `N/A: reason`
7. Rollback and additivity
8. Open questions and out of scope

Section headings may follow the change's own vocabulary — archived designs vary — but every one of
the eight must be substantively present. Anything no draft covered goes under `## Coverage Gaps`
in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/lane-status-observability/design.md`

The canonical design. Under 1800 words (v7 DDL excluded). Covers all eight spine items. Contains
only claims whose citations you verified in step 2 and which survive the cross-lens consistency
check in step 4. State each of Open Questions 1-5's resolution explicitly, or its remaining
open status with the reason.

### `openspec/changes/lane-status-observability/design-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: Lane Status Observability

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it, OR a lens failed to
close its assigned Open Question. State both positions and what evidence each has. Do NOT pick —
this section is the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code actually says.
"None" if there are none.>

## Architecture Divergence

<Where lens A, B, or C's assumed architecture disagreed with a sibling's, what content that cost
them, and where they converged independently. "None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write specs, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT invent a resolution for an Open Question a lens's hard stop genuinely left open.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane.

## Allowed paths

`openspec/changes/lane-status-observability/design.md` and
`openspec/changes/lane-status-observability/design-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill. Check the
canonical document against the contract as written: its required sections, the choice /
alternatives / rationale shape of a decision, and the threat-matrix applicability rule. On those,
the skill wins over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word budget — the skill's nominal 800 is
not honored in this repository, as `openspec/changes/archive/` shows — along with the synthesis
procedure, the notes file, and the done criteria. The skill's Step 4 Engram persistence and Step 5
return block are superseded: your output is the two files named above plus `.lucind/result.json`.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as
your **verification worklist, never as evidence**. A manifest row was written by the same lane
that made the claim, so a wrong citation arrives with a confident row beside it. The property that
makes this fan-out trustworthy is that you open every cited range yourself and check it against
the real code in this worktree. That property is not negotiable and the manifests do not relax it.

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

**Commit the canonical document the moment it is written, before you begin the notes file.** Then
write the notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit
leaves finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies
before a single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup`
deletes without warning. Two commits convert a timeout from lost work into resumable work.

Both commits are conventional, with no AI attribution.

## Done criteria

- [ ] **The canonical document was committed BEFORE the notes file was started, as its own
  commit.**
- [ ] **Every citation in every lens manifest was opened and checked in this worktree, and its
  outcome recorded.**
- [ ] **Every `file:line` citation surviving into `design.md` was opened and confirmed in this
  worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Each of Open Questions 1-5 is explicitly addressed in `design.md`** — resolved with the
  final choice stated, or left open with the reason.
- [ ] **`design.md` exists, is under 1800 words (v7 DDL excluded), and substantively covers all
  eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`design-synthesis-notes.md` exists with exactly the four required sections**, each either
  populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The three `## Assumed architecture` blocks are mutually irreconcilable and the proposal and
  specs do not choose between them. Write the notes file, leave `design.md` uncreated, and block.
- Lens B's v7 migration DDL is missing, incomplete, or does not compile as plausible SQL — do not
  invent or repair it yourself.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words (DDL excluded).
  Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/lane-status-observability/proposal.md` is committed in this worktree and is
the accepted proposal; the full `specs/` tree (six capabilities) is committed alongside it.** Read
both. `proposal-synthesis-notes.md` and `spec-synthesis-notes.md` beside them record citations the
earlier synthesis lanes opened and dropped, and known-wrong citations that must not resurface —
read both notes files before trusting any citation a lens draft repeats from the propose or spec
phase.

**Decided already — do not re-open, re-offer, or widen:**

1. Full six-item scope ships as **one PR**, accepting `size:exception`.
2. "Skill" observability is **static** frontmatter only (new `skill:` key), never live executor
   runtime telemetry. Generic "tool calls made" per lane is the live proxy.
3. `delivery_strategy` is `exception-ok`.
4. No historical-row backfill anywhere in this change.
5. `cursor-agent` reports zeroed telemetry, never omitted fields or a decode error.
6. The orphan sweep marks lanes `failed` only — no process supervision or auto-restart.
7. This is a Linux-only deployment; do not require cross-platform PID-liveness portability.

**Open Questions 1-5, from `proposal.md`, are what this phase exists to close.** Confirm each is
either closed with a stated final choice, or explicitly left open with the reason (see the
required procedure's step 3).

**Out of scope, and including any of it is wrong:** live executor Skill telemetry decoding,
`internal/resolve`/`internal/conflicttriage` (an unrelated change already merged to `main`),
process supervision or auto-restart, cross-platform PID-liveness beyond Linux.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter:
citations verified, citations dropped, Open Questions closed vs. left open, contradictions
escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every
hard stop is declared.
