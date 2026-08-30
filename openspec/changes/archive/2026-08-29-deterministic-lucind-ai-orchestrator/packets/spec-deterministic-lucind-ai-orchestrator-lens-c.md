---
id: spec-deterministic-lucind-ai-orchestrator-lens-c
executor: agy
routed_by: live-spec conflict and migration lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-c.md"]
---

# Packet spec-deterministic-lucind-ai-orchestrator-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/spec-deterministic-lucind-ai-orchestrator-lens-c  ·  **Branch:** lucind/spec-deterministic-lucind-ai-orchestrator-lens-c

## Goal

Produce `openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-c.md`: what this change
collides with in the live specs under `openspec/specs/`, the verbatim full block of every
requirement it modifies, and the migration guidance for everything it removes or renames.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final
delta spec. Do not write anything under `openspec/changes/deterministic-lucind-ai-orchestrator/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted, frozen, and already committed on this branch. Lens A and lens B run in
parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the
requirements you are checking from the proposal itself, declare them in `## Assumed requirements`,
and key every finding to one of them by name. The synthesizer arbitrates divergence.

## Why this lens exists

Archive replaces a live requirement with whatever the MODIFIED block says. A partial MODIFIED
block silently deletes every scenario it failed to copy, and nothing catches it until the
capability is already wrong in `openspec/specs/`. This lens is the lane that opens the live spec
and copies the whole block forward.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill, and the **MODIFIED
   Requirements Workflow** section in particular.
2. `openspec/specs/parent-feature-integration/spec.md` and `openspec/specs/sdd-apply/spec.md`
   **in full** — the only two capabilities the proposal lists under *Modified Capabilities* on this
   Change's `main` base. Not the index, not a grep — the whole file each. Do not open
   `openspec/specs/acceptance-verifier/` or `openspec/specs/packet-authoring-contract/`: they do
   not exist on this base (see `## Context`).
3. `openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md`, for what the change intends
   to do to each of them.
4. Consumers of any requirement being removed or renamed: tests, docs, CLI help text, other specs
   that reference it by name. Cite each with `file:line`.

Never claim a live requirement says something without opening it. This lens is the only lane that
reads the live specs in full; a wrong claim here is not caught downstream.

## Output format

Write exactly this skeleton to
`openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-c.md`:

```markdown
# Spec Lens C — Live-Spec Conflicts & Migration: Deterministic lucind-ai Orchestrator

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

`spec-lens-c.md` MUST be under 1000 words **excluding the verbatim blocks under
`## MODIFIED Full Blocks`**. Those blocks are copied evidence, not authored prose, and truncating
one to fit a budget is the exact failure this lens exists to prevent.

## Out of scope

Owned by the sibling lenses. Do NOT write these:

- **Lens A owns**: the capability map, the new requirement statements, and their
  ADDED / MODIFIED / REMOVED / RENAMED classification.
- **Lens B owns**: every new `#### Scenario:` block and the coverage argument.

The scenarios inside a `## MODIFIED Full Blocks` entry are yours, because they are copied evidence
from the live spec. Any scenario that does not already exist in `openspec/specs/` is lens B's.

Do NOT create or write any file under
`openspec/changes/deterministic-lucind-ai-orchestrator/specs/`. That tree belongs to the
synthesizer.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-c.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Precedence is **not symmetric**: the skill wins on *what a delta spec must contain*
(MODIFIED copy-full-then-edit workflow, REMOVED Reason-and-Migration rule, RENAMED both-names
rule). This packet wins on *how this phase is executed here* (three-lane split, this lens's slice,
word budget, skeleton, done criteria). Note any conflict in `## Open Questions`, follow this
packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file.

| citation | claim |
|---|---|
| `openspec/specs/sdd-apply/spec.md:1` | sdd-apply spec's opening line, confirming the file exists and is where the requirement inventory count starts |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-c.md --budget 1000 \
  --exclude-section "MODIFIED Full Blocks" --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Live Spec Inventory" \
  --require-section "Conflicts" --require-section "MODIFIED Full Blocks" \
  --require-section "Removals and Renames" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/spec-lens-c.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the
  claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL
  against this draft's own manifest.**
- [ ] **Every capability listed as modified was opened in full**, and its inventory row's
  requirement and scenario counts came from the file rather than an estimate.
- [ ] **Every `## MODIFIED Full Blocks` entry is the complete live block**, scenario for scenario,
  with nothing summarized or elided.
- [ ] **Every removal or rename names its consumers with `file:line`** and carries a Reason.
- [ ] **`spec-lens-c.md` exists, is under 1000 words excluding the verbatim blocks and the
  Citation Manifest, and carries `## Assumed requirements`, `## Live Spec Inventory`, and
  `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- A capability the proposal lists as modified has no live spec to read.
- A requirement being removed has consumers the proposal never mentions, so removing it breaks
  behavior nobody agreed to break.
- Copying a MODIFIED block whole would exceed what you can write, so the copy would have to be
  partial. Report which requirement forces it; never write a partial block.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- The proposal's Capabilities section (`openspec/changes/deterministic-lucind-ai-orchestrator/proposal.md:21-28`,
  corrected in commit `55a1543` after a prior dispatch of this exact lens hard-stopped on a real
  baseline mismatch) lists exactly two Modified capabilities: `sdd-apply` and
  `parent-feature-integration`. Directory listing of `openspec/specs/` confirms both exist as live
  spec directories.
- `deterministic-orchestrator-contract`, `packet-authoring-contract`, and `acceptance-verifier` are
  the three New capabilities and have no live spec on this Change's `main` base — do not look for a
  MODIFIED block for any of them.
- **Do not be misled by identically-named specs elsewhere in this repository.** `packet-authoring-contract`
  and `acceptance-verifier` DO exist as live specs on the unrelated, still in-flight
  `feature/skill-provisioning-and-phase-specialist` branch (639 commits ahead of this Change's
  `main` base) — a different, deliberately isolated Change. Do not open or copy blocks from that
  branch; if your worktree somehow shows those two directories as already present under
  `openspec/specs/`, treat that as a worktree/target contamination hard stop, not as confirmation.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`); the Rollback Plan (`proposal.md:51-53`) commits to reverting skill/parity/
runtime commits independently while retaining existing packet, ledger, lifecycle, and CAS
behavior, and never migrating or rewriting prior evidence — treat any MODIFIED block you write as
bound by that rollback commitment.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
