---
id: spec-<change-id>-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel spec lenses into the canonical delta spec tree
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/<change-id>/specs/", "openspec/changes/<change-id>/spec-synthesis-notes.md"]
---

# Packet spec-<change-id>-synthesis

**Tier:** A (human merge)
**Worktree:** ../<repo>-worktrees/spec-<change-id>-synthesis  ·  **Branch:** lucind/spec-<change-id>-synthesis

## Goal

Read the three spec lens drafts for `<change-id>`, verify their claims against the real code and the real live specs, arbitrate where they disagree, and produce the canonical delta spec tree under `openspec/changes/<change-id>/specs/` plus a separate synthesis notes file recording everything that did not make it in and why.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the orchestrator reads only your notes file. Anything you accept without checking, ships into `openspec/specs/` at archive time.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` are all present here. Lens worktrees could not see each other; this one sees all three.

## Preconditions

- `spec-lens-a.md`, `spec-lens-b.md`, and `spec-lens-c.md` all exist in this worktree.
- `openspec/changes/<change-id>/specs/` does not yet exist.
- The proposal for `<change-id>` is present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `spec-lens-a.md` | Capability map; requirement statements; ADDED / MODIFIED / REMOVED / RENAMED classification |
| `spec-lens-b.md` | Given/When/Then scenarios; coverage table; untestable assertions |
| `spec-lens-c.md` | Live-spec inventory and conflicts; verbatim MODIFIED full blocks; removals, renames, consumers, migration |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

Do these in order. Skipping step 2 or step 3 makes the output worthless regardless of how good it reads.

### 1. Read all three drafts in full

Do not begin writing until you have read all three.

### 2. Citation verification pass

Every `file:line` citation in every draft is a claim about this repository. Open each one in this worktree and confirm it says what the draft says it says. Lens C's live-spec citations matter most: a MODIFIED block copied from a requirement that is not there is a silent deletion at archive time.

- A citation that resolves and supports the claim: keep it.
- A citation that does not resolve, points at unrelated code, or does not support the claim: **drop the claim from the delta spec** and record it under `## Dropped Citations` in the notes with what you found instead.

A lens draft is evidence, not authority. You have the code and the live specs; use them.

### 3. Requirement arbitration

The three drafts each opened with `## Assumed requirements`. Compare them.

- **Lens A's requirement set is authoritative.** It is the lens that owned it.
- A scenario from lens B keyed to a requirement lens A did not name does not go into the delta. Record it under `## Requirement Divergence` in the notes with the name lens B used.
- A conflict lens C found against a requirement lens A classified as `ADDED` means the classification is wrong: it is `MODIFIED`. Lens C's evidence wins on classification, because it is the lens that opened the live spec. Record the correction in the notes.
- If lens B or lens C converged independently on lens A's requirement set, say so. Independent convergence is corroboration and is worth recording.
- If lens A's requirement set is refuted by a live spec you verified in step 2, do not silently substitute your own. That is a hard stop.

### 4. Assemble, do not concatenate

Write one file per capability at `openspec/changes/<change-id>/specs/<capability>/spec.md`, following the delta format: `## ADDED Requirements`, `## MODIFIED Requirements`, `## REMOVED Requirements`, `## RENAMED Requirements`. Omit any section with no entries.

- Each requirement gets its statement from lens A and its scenarios from lens B, joined on the requirement name.
- Each `MODIFIED` requirement starts as lens C's verbatim full block, edited to the new behavior, with `(Previously: <one line>)` under the requirement text. **Never ship a partial MODIFIED block.** Archive replaces the live requirement with exactly what you write; a scenario you drop here is deleted from the capability.
- Each `REMOVED` and `RENAMED` requirement carries lens C's Reason and Migration.

### 5. Budget

The authored content of the delta tree MUST stay under 1800 words, **excluding scenarios copied verbatim from a live spec inside a MODIFIED block**. The three drafts total roughly 3000 authored words; compressing them to 1800 is what forces arbitration rather than stapling.

The exclusion is deliberate and is the one place this phase differs from the other planning fan-outs. Copied blocks are evidence, not prose, and truncating one to hit a word count is a silent capability deletion. Compress your own writing; never compress a copied block.

### 6. Coverage check

The delta tree must satisfy this repository's spec spine:

1. Every capability the proposal names has a file, at the right path for new versus existing
2. Every requirement classified `ADDED` / `MODIFIED` / `REMOVED` / `RENAMED`
3. Every requirement text carries an RFC 2119 keyword — MUST, SHALL, SHOULD, MAY
4. Every requirement carries at least one scenario
5. Scenarios cover happy path and edge cases, in GIVEN / WHEN / THEN form
6. Every `MODIFIED` block is the complete live block, edited — never partial
7. Every `REMOVED` and `RENAMED` requirement carries a Reason, and a Migration where consumers exist
8. No implementation detail — the delta says WHAT, not HOW

Anything no draft covered goes under `## Coverage Gaps` in the notes. Do not invent content to fill a gap; report it.

## Output

### `openspec/changes/<change-id>/specs/<capability>/spec.md`

One file per capability. Contains only requirements whose citations you verified in step 2 and which survive lens A's requirement set.

### `openspec/changes/<change-id>/spec-synthesis-notes.md`

Exactly these four sections, in this order. This file is what the orchestrator reads:

```markdown
# Spec Synthesis Notes: <Change Title>

## Unresolved Contradictions

<Where two drafts assert incompatible things and neither the code nor the live
specs settle it. State both positions and what evidence each has. Do NOT pick —
this section is the escalation. "None" if there are none.>

## Coverage Gaps

<Spine items no draft covered — a requirement with no scenario, a capability with
no file, a removal with no migration. "None" if there are none.>

## Dropped Citations

<Every claim removed in step 2, with the citation that failed and what the code or
live spec actually says. "None" if there are none.>

## Requirement Divergence

<What lens B or lens C assumed that differed from lens A's requirement set, what
content that cost them, which classifications lens C's live-spec evidence
corrected, and where they converged independently. "None — all three converged"
if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts. They are the record of what each lens produced.
- Do NOT write into `openspec/specs/`. Archive merges the delta there; this phase never touches the live tree.
- Do NOT write design, tasks, or any implementation code.
- Do NOT resolve an unresolved contradiction by choosing. Escalating it is the correct output.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`. This is a document synthesis lane.

## Allowed paths

`openspec/changes/<change-id>/specs/` and `openspec/changes/<change-id>/spec-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill. Check the delta tree
against the contract as written: the ADDED / MODIFIED / REMOVED / RENAMED format, the RFC 2119
rule, the one-scenario-minimum rule, and the MODIFIED copy-full-then-edit workflow. On those, the
skill wins over this packet's paraphrase, and the drift goes in `## Coverage Gaps`.

It does not win on execution. This packet sets the 1800-word authored budget and its verbatim-block
exclusion, the synthesis procedure, the notes file, and the done criteria. The skill's Step 5
Engram persistence and Step 6 return block are superseded: your output is the delta tree, the notes
file, and `.lucind/result.json`.

Write nothing outside this repository.

## Done criteria

- [ ] **Every `file:line` citation surviving into the delta tree was opened and confirmed in this worktree**, and every dropped claim is listed under `## Dropped Citations`.
- [ ] **Every `MODIFIED` block matches the live requirement scenario for scenario**, with only the intended edits applied — verified by opening the live spec, not by trusting lens C.
- [ ] **Every requirement carries an RFC 2119 keyword and at least one GIVEN / WHEN / THEN scenario.**
- [ ] **The delta tree's authored content is under 1800 words excluding verbatim copied blocks**, and every spine item is satisfied or reported under `## Coverage Gaps`.
- [ ] **`spec-synthesis-notes.md` exists with exactly the four required sections**, each either populated or explicitly "None".
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The three `## Assumed requirements` blocks are mutually irreconcilable and the proposal does not choose between them. Write the notes file, leave `specs/` uncreated, and block.
- Lens A's requirement set is refuted by a live spec you verified. Do not substitute your own.
- A `MODIFIED` requirement's live block cannot be recovered from lens C or from `openspec/specs/`, so the block would have to be partial. Never write a partial block.
- One or more lens drafts is missing from this worktree.
- Satisfying the spine honestly would require exceeding the authored budget. Report which requirement forces it rather than silently overrunning or silently cutting.
- Satisfying one instruction in this packet would require violating another.

## Context

<The change title, the accepted proposal's Capabilities section, the live spec
paths for every modified capability, and any decision the human has already made
in conversation and does not want re-litigated.>

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: citations verified, citations dropped, MODIFIED blocks confirmed complete, contradictions escalated, coverage gaps. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
