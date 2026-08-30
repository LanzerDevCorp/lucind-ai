---
id: remediate-design-carveout-2-agentic-phase-specialist
executor: cursor-agent
routed_by: second bounded correction on design.md, scoped to one clause, per the design-phase Specialist's post-correction Acceptance verdict
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/design.md"]
---

# Packet remediate-design-carveout-2-agentic-phase-specialist

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-design-carveout-2-agentic-phase-specialist  ·  **Branch:** lucind/remediate-design-carveout-2-agentic-phase-specialist

## Goal

This is a single-clause bounded correction, narrower than the previous remediation on this same file. The `design` phase-Specialist re-checked the prior correction and found two of three gaps genuinely closed, but flagged a real defect in the third: the `New:` replacement text for the `SKILL.md:19` Hard Rule silently **deletes** `Acceptance` from the closed prohibition list instead of carving an **exception** out of it. Fix only this one clause. Do not touch anything else in `design.md`, including the rest of Decision 3, the `Old:` string, or the two other already-closed gaps (the out-of-repo carrier note and the `acceptance-promotion.md` File Changes coverage).

## Why this is safe to dispatch now

`design.md` is already integrated with the first correction. This lane branches from that integrated tip and touches exactly one line inside the Decision 3 replacement block.

## Preconditions

- `openspec/changes/agentic-phase-specialist/design.md` contains a `New:` replacement block for `SKILL.md:19` inside or near Decision 3 (around line 46 at time of writing, but locate it by content, not by line number, since a prior commit may have shifted lines).

## The exact defect to fix

The current `New:` text reads:

> Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, or Dependencies; a named `sdd-*` phase-Specialist may independently Accept its own phase's Lanes; Promotion remains forbidden to every Agent, Specialist included.

This drops `Acceptance` from the prohibition list entirely, then grants it only affirmatively to Specialists — which means the sentence no longer tells an ordinary, non-Specialist Agent that it does NOT own Acceptance. That is precisely the alternative Decision 3's own `:36` line already rejects ("Authorize all agents (rejected: workers lack cross-lane context)"). A lane applying this text verbatim to `SKILL.md:19` would ship a broad de-restriction, not a narrow carve-out.

Rewrite the `New:` text so it keeps the prohibition list closed (`scope, priorities, Dependencies, Acceptance, or Promotion` — same five items as the current live `Old:` text) and expresses the Specialist carve-out as an explicit exception clause, not a subtraction. A shape like this is acceptable (adapt wording to read naturally, but preserve exactly this semantic structure — closed list intact, exception stated separately, Promotion unconditionally still forbidden to Specialists too):

> Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion — except that a named `sdd-*` phase-Specialist may independently Accept its own phase's Lanes; Promotion remains forbidden to every Agent, Specialist included.

Do not just copy that sentence blindly if it does not fit gracefully with the surrounding text you find — preserve the semantic requirement (closed list + explicit exception + Promotion unconditionally forbidden), adjust only the prose to fit.

## Out of scope

- Do not touch the `Old:` string, the rest of Decision 3, or any other Decision (1, 2, 4, 5).
- Do not touch the out-of-repo carrier note or the `acceptance-promotion.md` File Changes coverage added by the prior correction — both were independently verified correct.
- Do not touch Testing Strategy, Threat Matrix, Rollback and Additivity, or Open Questions and Out of Scope.
- Do not run `go test`, `go build`, or `lucind-checks.sh` — this phase only touches `openspec/changes/agentic-phase-specialist/**`.

## Allowed paths

`openspec/changes/agentic-phase-specialist/design.md` only. Create no other file.

## Mechanical self-check (REQUIRED)

Run `./lucind-lane-check.sh` from the repo root before and after committing:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design.md --skip-result
```

Confirm the diff is a single-clause edit (a handful of words changed inside one existing sentence), not a rewrite of the surrounding block. Paste the PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **The `New:` replacement text keeps the prohibition list closed** (`scope, priorities, Dependencies, Acceptance, or Promotion` — same five items, in that order or a natural equivalent), with the Specialist carve-out expressed as an explicit exception to `Acceptance` specifically, not a deletion of it from the list.
- [ ] **Promotion remains stated as unconditionally forbidden to every Agent, Specialist included**, unchanged from the prior correction's intent.
- [ ] **No other line, section, or word of `design.md` was modified** beyond this one clause.
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by `lucind-lane-check.sh` reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The `New:` replacement block described above no longer exists in `design.md` in a form you can locate by content.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`. This is the second bounded correction on the same `design.md` file. The first correction (already integrated) fixed two of three gaps from the design-phase Specialist's original `needs-revision` verdict, but its own fix for the third gap (the `SKILL.md:19` Hard Rule replacement text) introduced a new, narrower defect, caught by the same Specialist re-checking its own prior correction. This is exactly the "one scoped correction" bounded-relaunch pattern working as designed — fix precisely what was named, nothing more.

## Required skills

- sdd-design

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
