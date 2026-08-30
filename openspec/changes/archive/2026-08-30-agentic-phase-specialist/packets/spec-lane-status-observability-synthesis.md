---
id: spec-lane-status-observability-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel spec lenses into the canonical delta spec tree
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/lane-status-observability/specs/", "openspec/changes/lane-status-observability/spec-synthesis-notes.md"]
---

# Packet spec-lane-status-observability-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/spec-lane-status-observability-synthesis  ·  **Branch:** lucind/spec-lane-status-observability-synthesis

## Goal

Read the three spec lens drafts for `lane-status-observability`, verify their claims against the
real code and the real live specs, arbitrate where they disagree, and produce the canonical delta
spec tree under `openspec/changes/lane-status-observability/specs/` plus a separate synthesis notes
file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the
orchestrator reads only your notes file. Anything you accept without checking ships into
`openspec/specs/` at archive time.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from
the integrated result, so `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` are all present
here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` all exist in this worktree.
- `openspec/changes/lane-status-observability/specs/` does not yet exist.
- The proposal for `lane-status-observability` is present and accepted.

## What each lens owns

Unlike the standard capabilities-vs-scenarios-vs-conflicts spec fan-out, this change was split by
**capability domain** — each lens owns full vertical slices (requirement text + scenarios) for its
capabilities, not one aspect across all of them.

| Draft | Owns |
|---|---|
| `spec-lens-a.md` | `lane-execution` and `read-only-packet-schema` — requirement text, terminal consumers, and scenarios for **Lane metadata dispatch persistence** and **Extended packet frontmatter parsing**. Does NOT open the live specs for these two capabilities. |
| `spec-lens-b.md` | `dispatched-packet-body` and `lane-progress-telemetry` (both new) — requirement text and scenarios for **Dispatched packet body inspection** and **Structured progress telemetry**. |
| `spec-lens-c.md` | `orphan-lane-reconciliation` (new) — its own requirement and scenarios for **Orphaned lane reconciliation**. PLUS: opens `lane-execution/spec.md`, `read-only-packet-schema/spec.md`, and `batch-wave-view/spec.md` in full; corrects lens A's ADDED/MODIFIED classification if the live text disagrees; authors `batch-wave-view`'s own requirement from scratch (proposal.md has no draft for it); flags the schema-v7 cross-cutting overlap with lens B's `lane-progress-telemetry`. |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it
reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this
worktree and confirm it says what the draft says it says. Lens C's live-spec citations matter
most: a MODIFIED block copied from a requirement that is not there is a silent deletion at archive
time.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop
  the claim from the delta spec** and record it under `## Dropped Citations` in the notes with what
  you found instead.

A lens draft is evidence, not authority. You have the code and the live specs; use them.

**Known-wrong citations from the propose phase — confirm the drafts avoided them, and if any
resurfaced, drop them without re-verifying (they are already confirmed wrong):**
`internal/ledger/schema.go:310-330` (not v7 DDL); `internal/serve/handlers.go:33-60`/`:30-120` (not
the packet-route/sweep seam — `handlers.go:190` `NewHandlerWithConfig` is correct);
`internal/serve/server.go:1-60`/`:19-53` (not sweep/PID/ticker); `internal/ledger/lanes.go:35-50`
(does not exist — `SetStatus` is `ledger.go:452`); `internal/serve/server_test.go:42-93` (not
sweep coverage); `internal/ledger/schema.go:298-308` (is `lane_progress`, not `runs.pid`, which is
`:226-234`); `internal/ledger/runs.go:103-137` (not a backfill-policy citation);
`openspec/specs/lane-envelope-inspector` (wrong capability for packet-body — the correct new
capability is `dispatched-packet-body`).

### 3. Requirement arbitration

Because this fan-out split by capability domain rather than by aspect, most requirements have only
one lens's draft to arbitrate — arbitration here is mostly about the two capabilities lens A and
lens C both touch (`lane-execution`, `read-only-packet-schema`), and about `batch-wave-view`, which
only lens C drafted.

- **For `lane-execution` and `read-only-packet-schema`: lens A's requirement TEXT is authoritative.
  Lens C's live-spec evidence is authoritative on CLASSIFICATION only** (ADDED vs MODIFIED) — the
  same "one deliberate exception to lens A's authority" the standard fan-out convention uses. If
  lens C found the new behavior actually edits an existing requirement's text rather than adding a
  new one, the classification is MODIFIED, and lens A's requirement text becomes the edit applied
  to lens C's full live block (per the MODIFIED workflow) rather than a standalone ADDED section.
- **For `dispatched-packet-body` and `lane-progress-telemetry`: lens B is authoritative**, both text
  and scenarios — no other lens touched these.
- **For `orphan-lane-reconciliation`: lens C is authoritative**, both text and scenarios — no other
  lens touched this.
- **For `batch-wave-view`: lens C is the only source.** Verify its MODIFIED-or-ADDED authoring
  against the live spec yourself before accepting it — this is the one requirement in the whole
  change with no independent second opinion, so your own verification is the only check it gets.
- If any two drafts' `## Assumed requirements` blocks converged independently on the same
  boundary or classification, record that as corroboration.
- If lens A's classification is refuted by a live spec you verified yourself, do not silently
  substitute your own text — only the classification may move; the requirement wording stays lens
  A's unless it is factually wrong about a citation, in which case fix the citation and record it
  under `## Dropped Citations`.

### 4. Assemble, do not concatenate

Write one file per capability at `openspec/changes/lane-status-observability/specs/<capability>/spec.md`:

- New capabilities (`dispatched-packet-body`, `lane-progress-telemetry`, `orphan-lane-reconciliation`):
  a full spec (`## Purpose` + `## Requirements`), not a delta.
- Existing/modified capabilities (`lane-execution`, `read-only-packet-schema`, `batch-wave-view`): a
  delta (`## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements`,
  `## RENAMED Requirements`). Omit any section with no entries.

Each requirement gets its statement from the owning lens and its scenarios from the same lens
(this fan-out did not split scenarios into a separate lane). Each `MODIFIED` requirement (if any)
starts as lens C's verbatim full block, edited to the new behavior, with
`(Previously: <one line>)` under the requirement text. **Never ship a partial MODIFIED block.**
Archive replaces the live requirement with exactly what you write; a scenario you drop here is
deleted from the capability.

### 5. Resolve the schema-v7 cross-cutting note

Lens C flags that schema v7 is one migration touching `runs.pid`
(`orphan-lane-reconciliation`) and `lane_progress` usage columns (`lane-progress-telemetry`). These
are two separate capability files but one migration. Do not duplicate the migration description in
both — state it once, in whichever capability's requirement the drafts describe it more concretely
(check both `spec-lens-b.md` and `spec-lens-c.md`), and have the other capability's requirement
reference it by name ("as part of the same schema v7 migration described under
`orphan-lane-reconciliation`" or the reverse) rather than re-deriving a second migration story.

### 6. Budget

The authored content of the delta tree MUST stay under 1800 words, **excluding scenarios copied
verbatim from a live spec inside a MODIFIED block**. The three drafts total roughly 3000 authored
words; compressing them to 1800 is what forces arbitration rather than stapling.

The exclusion is deliberate. Copied blocks are evidence, not prose, and truncating one to hit a
word count is a silent capability deletion. Compress your own writing; never compress a copied
block.

### 7. Coverage check

The delta tree must satisfy this repository's spec spine:

1. Every capability the proposal names has a file, at the right path for new versus existing
   (six capabilities: `dispatched-packet-body`, `lane-progress-telemetry`,
   `orphan-lane-reconciliation` new; `lane-execution`, `read-only-packet-schema`, `batch-wave-view`
   modified)
2. Every requirement classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`
3. Every requirement text carries an RFC 2119 keyword — MUST, SHALL, SHOULD, MAY
4. Every requirement carries at least one scenario
5. Scenarios cover happy path and edge cases, in GIVEN / WHEN / THEN form
6. Every `MODIFIED` block is the complete live block, edited — never partial
7. No implementation detail — the delta says WHAT, not HOW
8. None of the five still-open proposal questions (frontmatter key names, packet-path persistence
   mechanism, sweep ticker interval, PID-liveness syscall/portability, DAG-wave scope) is answered
   by a requirement or scenario as if decided. If any lens draft quietly picked one, correct it back
   to the open, mechanism-agnostic phrasing and record the correction under `## Coverage Gaps`.

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill
a gap; report it.

## Output

### `openspec/changes/lane-status-observability/specs/<capability>/spec.md`

Six files, one per capability. Contains only requirements whose citations you verified in step 2.

### `openspec/changes/lane-status-observability/spec-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Spec Synthesis Notes: Lane Status Observability

## Unresolved Contradictions

<Where two drafts assert incompatible things and neither the code nor the live
specs settle it. State both positions and what evidence each has. Do NOT pick —
this section is the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered — a requirement with no scenario, a capability
with no file, a still-open proposal question a draft answered as if decided.
"None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
or live spec actually says. "None" if there are none.>

## Requirement Divergence

<The classification corrections lens C's live-spec evidence made to lens A's
ADDED/MODIFIED calls (or "confirmed, no correction needed"), how
`batch-wave-view`'s requirement was authored with no independent second
opinion, and where the schema-v7 cross-cutting note landed. "None — all
converged" if genuinely nothing diverged.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write into `openspec/specs/`. Archive merges the delta there; this phase never touches the
  live tree.
- Do NOT write design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT answer any of the five still-open proposal questions. A requirement that hardcodes one is
  a defect to fix, not content to preserve.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis
  lane.

## Allowed paths

`openspec/changes/lane-status-observability/specs/` and
`openspec/changes/lane-status-observability/spec-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill. Check the delta tree
against the contract as written: the ADDED/MODIFIED/REMOVED/RENAMED format, the RFC 2119 rule, the
one-scenario-minimum rule, and the MODIFIED copy-full-then-edit workflow. On those, the skill wins
over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word authored budget and its verbatim-block
exclusion, the synthesis procedure, the notes file, and the done criteria. The skill's Step 5 Engram
persistence and Step 6 return block are superseded: your output is the delta tree, the notes file,
and `.lucind/result.json`.

Write nothing outside this repository.

## Using the lens citation manifests

Each lens draft ends with a `## Citation Manifest` table. Treat the union of the three manifests as
your **verification worklist, never as evidence**. A manifest row was written by the same lane that
made the claim, so a wrong citation arrives with a confident row beside it. The property that makes
this fan-out trustworthy is that you open every cited range yourself and check it against the real
code in this worktree. That property is not negotiable and the manifests do not relax it.

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

**Commit the canonical document the moment it is written, before you begin the notes file.** Then
write the notes and commit them as a second conventional commit.

This is not bookkeeping. A synthesis lane that dies on the wall clock after its first commit leaves
finished work on its branch, recoverable by whoever re-dispatches it. A lane that dies before a
single end-of-run commit leaves an untracked file that `lucind-ai worktree cleanup` deletes without
warning. Two commits convert a timeout from lost work into resumable work.

Both commits are conventional, with no AI attribution.

## Done criteria

- [ ] **The canonical document was committed BEFORE the notes file was started, as its own commit.**
- [ ] **Every citation in every lens manifest was opened and checked in this worktree, and its
      outcome recorded.**
- [ ] **Every `file:line` citation surviving into the delta tree was opened and confirmed in this
      worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every `MODIFIED` block (if any) matches the live requirement scenario for scenario**, with
      only the intended edits applied — verified by opening the live spec, not by trusting lens C.
- [ ] **Every requirement carries an RFC 2119 keyword and at least one GIVEN / WHEN / THEN scenario.**
- [ ] **`batch-wave-view` has a verified requirement**, checked against its live spec by you, not
      merely copied from lens C.
- [ ] **None of the five still-open proposal questions is answered as if decided** anywhere in the
      delta tree.
- [ ] **The delta tree's authored content is under 1800 words excluding verbatim copied blocks**,
      and every spine item is satisfied or reported under `## Coverage Gaps`.
- [ ] **`spec-synthesis-notes.md` exists with exactly the four required sections**, each either
      populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The three `## Assumed requirements` blocks are mutually irreconcilable and the proposal does not
  choose between them. Write the notes file, leave `specs/` uncreated, and block.
- Lens A's classification is refuted by a live spec you verified, but its requirement text is also
  wrong in a way you cannot fix by editing the classification alone.
- A `MODIFIED` requirement's live block cannot be recovered from lens C or from `openspec/specs/`,
  so the block would have to be partial. Never write a partial block.
- One or more lens drafts is missing from this worktree.
- Satisfying the spine honestly would require exceeding the authored budget. Report which
  requirement forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/lane-status-observability/proposal.md` is committed in this worktree and is the
accepted proposal.** Read it first.

**User-approved decisions, already final — do not re-litigate:**

- Full six-item scope ships as **one PR**, accepting `size:exception` for the review-budget risk.
- "Skill" observability is static-only (a new `skill:` frontmatter key); live runtime skill
  telemetry is out of scope. Generic tool-call counts are the live proxy.
- `delivery_strategy` is `exception-ok` for this change.

**Five open questions from proposal.md's `## Open Questions` remain OPEN — do not let any survive
into the delta tree as a decision:**

1. Exact frontmatter key names (`sdd_phase` vs `phase`, etc.)
2. Packet path persistence mechanism (new field vs. real column)
3. Sweep ticker interval
4. PID-liveness syscall/portability
5. DAG-wave `Node`/`EmitPacketContent` scope

**Ground truth for the six capabilities — cite it yourself, do not trust a lens's paraphrase:**

- `ledger.LaneMetadata` (`internal/ledger/lanes_meta.go:20-32`); `UpdateLaneMetadata`/
  `GetLaneMetadata` (`:39,89`). `Execute`/`ensureLaneFailed` (`run.go:334`, `batch.go:184`).
- `packet.Parse` (`internal/packet/packet.go:78-167`); `Packet` struct (`:33-75`).
- `NewHandlerWithConfig` (`internal/serve/handlers.go:190`) — the real mux registration point.
  `cli.go:160-174` — packet path mapping.
- `ProgressEvent` (`internal/executor/executor.go:17-21`); `LaneProgress`
  (`internal/ledger/progress.go:15-20`); decoders in `agy_stream.go`, `claude_stream.go`,
  `opencode_stream.go`; `cursor_agent.go`/`cursor_agent_stream.go` have no usage struct.
- `RegisterRun` (`internal/ledger/runs.go:29-40`, `cmd/lucind-ai/cli.go:314-321`); `SetStatus`
  (`internal/ledger/ledger.go:452`); running-transition seam (`internal/run/run.go:355`).
- Schema: `runs`/`lane_progress` STRICT (`schema.go:182-189,221-224`); v4→v5/v5→v6 pattern to
  follow (`schema.go:182-219,221-308`); `migrate` transactional/idempotent (`schema.go:310-409`).

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations
verified, citations dropped, classification corrections made, MODIFIED blocks confirmed complete,
contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
