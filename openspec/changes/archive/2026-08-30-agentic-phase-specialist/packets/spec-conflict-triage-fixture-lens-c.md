---
id: spec-conflict-triage-fixture-lens-c
executor: agy
routed_by: live-spec conflict and migration lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/spec-lens-c.md"]
---

# Packet spec-conflict-triage-fixture-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-conflict-triage-fixture-lens-c  ·  **Branch:** lucind/spec-conflict-triage-fixture-lens-c

## Goal

Produce `openspec/changes/conflict-triage-fixture/spec-lens-c.md`: what this change collides with in the live specs under `openspec/specs/`, the verbatim full block of every requirement it modifies, and the migration guidance for everything it removes or renames.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/conflict-triage-fixture/specs/`.

## Why this is safe to dispatch now

The proposal for `conflict-triage-fixture` is accepted and frozen. Lens A and lens B run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the requirements you are checking from the proposal itself, declare them in `## Assumed requirements`, and key every finding to one of them by name. The synthesizer arbitrates divergence.

## Why this lens exists

Archive replaces a live requirement with whatever the MODIFIED block says. A partial MODIFIED block silently deletes every scenario it failed to copy, and nothing catches it until the capability is already wrong in `openspec/specs/`. This lens is the lane that opens the live spec and copies the whole block forward.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *what already exists and what breaks* — not to new requirement text and not to scenarios:

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill, and the **MODIFIED
   Requirements Workflow** section in particular. It is the phase contract this draft feeds; read
   it rather than trusting this packet's paraphrase of it.
2. `openspec/specs/<capability>/spec.md` **in full**, for every capability the proposal lists under
   *Modified Capabilities*. Not the index, not a grep — the whole file. You cannot report what a
   change collides with from a search result.
3. `openspec/changes/conflict-triage-fixture/proposal.md`, for what the change intends to do to each of them.
4. Consumers of any requirement being removed or renamed: tests, docs, CLI help text, other specs
   that reference it by name. Cite each with `file:line`.

Never claim a live requirement says something without opening it. This lens is the only lane that reads the live specs in full; a wrong claim here is not caught downstream.

## Output format

Write exactly this skeleton to `openspec/changes/conflict-triage-fixture/spec-lens-c.md`:

```markdown
# Spec Lens C — Live-Spec Conflicts & Migration: Conflict Triage Fixture

## Assumed requirements

<2–4 sentences naming the requirement set you are checking against live specs:
which capabilities this change touches and what each requirement is expected to
assert. Lens A and lens B write this same block independently; the synthesizer
compares all three. Be specific enough that a disagreement is visible.>

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|

<One row per capability the proposal lists as modified. Counts come from opening
the file, not from estimating.>

## Conflicts

<Every place this change contradicts a live requirement rather than extending it:
the live requirement, what it currently guarantees, and what this change would
make untrue. A conflict is a MODIFIED requirement, not an ADDED one — say so.
"None" if there are none.>

## MODIFIED Full Blocks

### Requirement: <Live Requirement Name>

**Source**: `openspec/specs/<capability>/spec.md:<line>` — <N> scenarios

<The COMPLETE live block, copied verbatim: the requirement text and every one of
its scenarios, unedited. The synthesizer edits this copy; your job is to make sure
nothing is lost on the way. Do not summarize, do not elide a scenario, do not
write "(remaining scenarios unchanged)".>

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|

<One row per removal or rename. The Consumers column lists every test, doc, or
spec that references it by name — that list is what makes the Migration column
checkable rather than aspirational.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-c.md` MUST be under 1000 words **excluding the verbatim blocks under `## MODIFIED Full Blocks`**. Those blocks are copied evidence, not authored prose, and truncating one to fit a budget is the exact failure this lens exists to prevent. Everything you write in your own words stays under the cap.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: the capability map, the new requirement statements, and their ADDED / MODIFIED / REMOVED / RENAMED classification.
- **Lens B owns**: every new `#### Scenario:` block and the coverage argument.

The scenarios inside a `## MODIFIED Full Blocks` entry are yours, because they are copied evidence from the live spec. Any scenario that does not already exist in `openspec/specs/` is lens B's.

Do NOT create or write any file under `openspec/changes/conflict-triage-fixture/specs/`. That tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/conflict-triage-fixture/spec-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a delta spec must contain*: the MODIFIED copy-full-then-edit
workflow, the REMOVED Reason-and-Migration rule, and the RENAMED both-names rule. Where this packet
paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the spec is split across
three parallel lanes, which slice this lane owns, its word budget, its output path and skeleton,
its out-of-scope list, and its done criteria. The skill describes one sub-agent writing the whole
delta spec tree by itself, so parts of it will read as instructing you to do what this packet
forbids — write files under `specs/`, persist to Engram, return the phase summary block. Those are
superseded here on purpose. Do not correct yourself toward them; note the conflict in
`## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation.

| citation | claim |
|---|---|
| `internal/run/attempt.go:743-747` | ErrNoMergeBase makes evaluateOverlapGate continue |

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

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **Every capability listed as modified was opened in full**, and its inventory row's requirement and scenario counts came from the file rather than an estimate.
- [ ] **Every `## MODIFIED Full Blocks` entry is the complete live block**, scenario for scenario, with nothing summarized or elided.
- [ ] **Every removal or rename names its consumers with `file:line`** and carries a Reason.
- [ ] **`spec-lens-c.md` exists, is under 1000 words excluding the verbatim blocks, and carries `## Assumed requirements` and `## Live Spec Inventory`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- A capability the proposal lists as modified has no live spec to read.
- A requirement being removed has consumers the proposal never mentions, so removing it breaks behavior nobody agreed to break.
- Copying a MODIFIED block whole would exceed what you can write, so the copy would have to be partial. Report which requirement forces it; never write a partial block.
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

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
