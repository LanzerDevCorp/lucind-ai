---
id: design-conflict-triage-fixture-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel design lenses into one canonical design
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/design.md", "openspec/changes/conflict-triage-fixture/design-synthesis-notes.md"]
---

# Packet design-conflict-triage-fixture-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/design-conflict-triage-fixture-synthesis  ·  **Branch:** lucind/design-conflict-triage-fixture-synthesis

## Goal

Read the three design lens drafts for `conflict-triage-fixture`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/conflict-triage-fixture/design.md` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `openspec/changes/conflict-triage-fixture/design-lens-a.md`, `-lens-b.md`, and `-lens-c.md` all exist in this worktree.
- `openspec/changes/conflict-triage-fixture/design.md` does not yet exist.
- The proposal and specs for `conflict-triage-fixture` are present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `design-lens-a.md` | Technical approach; every architecture decision except rollback, with alternatives and rationale |
| `design-lens-b.md` | Flow and invariants; surface deltas (types, schemas, frontmatter, CLI); file changes |
| `design-lens-c.md` | Testing strategy and test seams; threat matrix; rollback and additivity; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from `design.md`** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code; use it.

### 3. Architecture arbitration

The three drafts each opened with `## Assumed architecture`. Compare them.

- **Lens A's assumed architecture is authoritative.** It is the lens that owned the decision.
- Any content in lens B or lens C that does not survive lens A's architecture does not go into `design.md`. Record it under `## Architecture Divergence` in the notes, with what B or C assumed instead.
- If lens B or lens C converged independently on lens A's architecture, say so in the notes. Independent convergence is corroboration and is worth recording.
- If lens A's own architecture is refuted by code you verified in step 2, do not silently substitute your own. That is a hard stop.

### 4. Compress — do not concatenate

`design.md` MUST be under 1800 words. The three drafts total roughly 3000. Cutting is the job: merge overlapping statements, drop restatement, keep the specific sentence over the general one. A concatenation of three drafts is a failed synthesis even if every word in it is true.

### 5. Coverage check

`design.md` must cover this repository's actual design spine, derived from every archived design in `openspec/changes/archive/`:

1. Technical approach or recommendations at a glance
2. Architecture decisions, each with choice / alternatives considered / rationale
3. Flow and invariants
4. File changes, with terminal consumers
5. Testing strategy and test seams
6. Threat matrix — every row `Applicable` or `N/A: reason`
7. Rollback and additivity
8. Open questions and out of scope

Section headings may follow the change's own vocabulary — archived designs vary — but every one of the eight must be substantively present. Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/conflict-triage-fixture/design.md`

The canonical design. Under 1800 words. Covers all eight spine items. Contains only claims whose citations you verified in step 2 and which survive lens A's architecture.

### `openspec/changes/conflict-triage-fixture/design-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

<Where two drafts assert incompatible things and the code does not settle it.
State both positions and what evidence each has. Do NOT pick — this section is
the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code
actually says. "None" if there are none.>

## Architecture Divergence

<What lens B or lens C assumed that differed from lens A, what content that cost
them, and where they converged independently. "None — all three converged" if
that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write specs, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis lane.

## Allowed paths

`openspec/changes/conflict-triage-fixture/design.md` and `openspec/changes/conflict-triage-fixture/design-synthesis-notes.md` only.

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
your **verification worklist, never as evidence**. A manifest row was written by the same lane that
made the claim, so a wrong citation arrives with a confident row beside it. The property that makes
this fan-out trustworthy is that you open every cited range yourself and check it against the real
code in this worktree. That property is not negotiable and the manifests do not relax it.

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
deletes without warning — which has already cost this project one full synthesis run. Two commits
convert a timeout from lost work into resumable work.

Both commits are conventional, with no AI attribution.

## Done criteria

- [ ] **The canonical document was committed BEFORE the notes file was started, as its own commit.**
- [ ] **Every citation in every lens manifest was opened and checked in this worktree, and its outcome recorded.**
- [ ] **Every `file:line` citation surviving into `design.md` was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **`design.md` exists, is under 1800 words, and substantively covers all eight spine items**, with anything missing reported under `## Coverage Gaps`.
- [ ] **`design-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The three `## Assumed architecture` blocks are mutually irreconcilable and the proposal and specs do not choose between them. Write the notes file, leave `design.md` uncreated, and block.
- Lens A's architecture is refuted by code you verified. Do not substitute your own.
- One or more lens drafts is missing from this worktree.
- Covering all eight spine items honestly would require exceeding 1800 words. Report which item forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/conflict-triage-fixture/proposal.md` is committed in this worktree and is the
accepted proposal.** Read it first. `proposal-synthesis-notes.md` beside it records twelve
citations the propose synthesizer opened and dropped — do NOT resurrect any of them. Two examples,
so the shape is clear: `internal/reconcile/reconcile.go:163-165` is `NewService` assigning the
default evaluator, NOT the triage-JSON slot (that is `Candidate.Output` at `:105`);
`internal/serve/handlers.go:95-109` is `/api/state` pagination constants, NOT the read-only GET
reconcile routes.

**Four product questions are DECIDED. Do not re-open, re-offer, or widen them:**

1. Verify-budget unit is **wall clock plus the concrete command** (e.g. "~4 min:
   `./lucind-checks.sh` on the combined tree"). Token/pricing and test-weight units were rejected.
2. Risk is **three bands** — low / medium / high — with a business conflict pinned to `high` and
   the agent unable to lower it. A continuous 0-100 score was rejected.
3. The fixture generator lives at **`internal/conflicttriage/fixture/`**. `test/fixture/` and a
   public CLI subcommand were rejected.
4. The A/B win criterion is **correct classification of the three hunks** — business separated
   from the two mechanical controls, arbitrariness declared where it belongs. Grading the prepared
   resolution, and timing a human to thirty seconds, were both rejected.

**Two questions MUST stay open. Answering either by guess is a lane failure:**

- the exact non-decreasing risk formula and its thresholds, including mixed business+mechanical
  hunks;
- which executor/model runs production triage.

**One real ambiguity the proposal left unresolved — the orchestrator found it in review, and it
belongs to design, not specs.** The proposal's triage requirement says the agent must "leave a
prepared SHA (`internal/reconcile/reconcile.go:107`)", while its Approach says "a human resolves
out of band and registers the SHA" via `reconcile resolve --candidate --sha`. Both cannot hold. If
the agent writes `CandidateSHA` into the ledger itself it bypasses the human registration step
that makes the whole resolution safe. Design must decide explicitly who writes that field and say
why; specs must not assume an answer.

**Ground truth — cite it, do not re-derive it:**

- `evaluateOverlapGate` (`internal/run/attempt.go:687`) classifies via `overlap.Classify`
  (`internal/overlap/overlap.go:623`); `ClassRequired` at `:658-659`; thresholds at `:93-98`.
- Evidence for `ClassRequired` is inserted inside `CreateRequest`
  (`internal/reconcile/reconcile.go:266`), never on the warning branch.
- Without a registered shared `base_sha`, `ErrNoMergeBase` makes the gate `continue`
  (`internal/run/attempt.go:743-747`) and `ClassRequired` is unreachable.
- Triage does NOT fail closed, unlike `internal/resolve` (`internal/resolve/candidate.go:26`,
  prompt at `:303-312`). Keep the two disciplines separate.
- Ledger today: 36 `integration_attempts`, 0 `overlap_evidence`, 0 `reconciliation_requests`,
  0 `reconciliation_candidates`.

**Out of scope, and including any of it is wrong:** reconcile POST on the web surface (read-only
GET stays), overlap thresholds, and production dispatch paths.

**`openspec/changes/conflict-triage-fixture/specs/` does not exist yet.** The specs fan-out is
running concurrently with this one, in its own worktrees you cannot see. Reason from the
proposal's `## Capabilities` section, never from requirement ids: a design that cites a
requirement name it never read is citing something it invented.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
