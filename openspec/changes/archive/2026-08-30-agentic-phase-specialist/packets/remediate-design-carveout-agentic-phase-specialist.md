---
id: remediate-design-carveout-agentic-phase-specialist
executor: cursor-agent
routed_by: bounded correction on design.md only, per the design-phase Specialist's Acceptance verdict
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/design.md"]
---

# Packet remediate-design-carveout-agentic-phase-specialist

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-design-carveout-agentic-phase-specialist  ·  **Branch:** lucind/remediate-design-carveout-agentic-phase-specialist

## Goal

This is a single bounded correction lane, not a re-fan-out. The `design` phase-Specialist (an `sdd-design` subagent acting as this phase's Acceptance authority) reviewed the already-integrated, already-accepted-by-synthesis `openspec/changes/agentic-phase-specialist/design.md` and returned `needs-revision` with exactly two named gaps. Fix only those two gaps. Do not touch anything else in the file, do not re-litigate any of the five architecture decisions, and do not second-guess the Specialist's citation verification (it independently spot-checked 5 of the synthesis lane's 8 dropped/retargeted citations and confirmed all correct).

## Why this is safe to dispatch now

`design.md` is already integrated on the primary branch. This lane branches from that integrated tip, edits the same file in place, and is the only lane touching it — no concurrent lens or synthesis lane is running.

## Preconditions

- `openspec/changes/agentic-phase-specialist/design.md` exists and contains `### Decision 3 — Hard Rule carve-out; Promotion stays human` and a `## File Changes` table.

## The two gaps to fix (verbatim from the Specialist's Phase Verdict)

### Gap 1 — no exact replacement wording for the Hard Rule carve-out, and no named in-repo carrier for the out-of-repo SKILL.md text

Decision 3 (around line 33-36) currently describes the carve-out semantically ("named `sdd-*` Specialists may Accept their own phase's Lanes; Promotion remains forbidden to all Agents") but never gives the literal replacement sentence for the Hard Rule. The current live text at `plugin/claude-code/skills/lucind-ai/SKILL.md:19` (byte-identical in the OpenCode mirror, enforced by `TestSkillTreesByteIdentical`) is:

> Keep one Orchestrator authoritative for the Change. Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion.

Add to Decision 3 (or a new short subsection immediately under it) the exact verbatim replacement sentence — a single sentence or short pair of sentences, in the same terse Hard-Rule style as the rest of `SKILL.md`, that:
- Keeps "Agents own Lanes, not scope, priorities, or Dependencies" for ordinary Agents.
- Explicitly carves out that a named `sdd-*` phase-Specialist may independently Accept its own phase's Lanes.
- Keeps Promotion forbidden to every Agent, Specialist included, with no exception.

Present it as a literal quoted replacement block (old → new), so `sdd-tasks`/`sdd-apply` can copy it verbatim into both `plugin/claude-code/skills/lucind-ai/SKILL.md:19` and its OpenCode mirror without having to invent wording themselves.

Separately, the out-of-repository note (around line 92, "`~/.claude/skills/sdd-*/SKILL.md` must stop doing the phase's work...") says "This Change documents the required text" but never names where. Fix this by stating explicitly that `design.md` itself (this section) is the in-repo carrier of that required text — i.e., add the literal instruction text a human (or a future in-repo authoring mechanism) should paste into each `~/.claude/skills/sdd-*/SKILL.md` file, directly in this section, quoted. Keep it short: the substance is "stop doing the phase's work directly; instead author/dispatch this phase's fan-out+synthesis packets via lucind-ai, read the synthesis notes, and independently judge Acceptance, returning only a Phase Verdict to the Orchestrator" — phrase it precisely enough that `sdd-apply` can use it as source text, not a paraphrase to redo.

### Gap 2 — File Changes table omits `acceptance-promotion.md` steps 1 and 8, which become false for non-apply phases

The `## File Changes` table (around line 86) only names lines 31-36 of `acceptance-promotion.md` for modification. But the checklist's step 1 (currently at `acceptance-promotion.md:20`) states acceptance "then runs the repository checks (`lucind-checks.sh`) inside a verifier-owned detached worktree", and step 8 (currently at `:27`) states "**Full-repo suite pass**: Ensure mechanical checks cover the whole repository." Both become unconditionally false once Decision 2's `sdd_phase`-gated check-skip lands for declared non-apply lanes.

Add a new row (or extend the existing `acceptance-promotion.md` row) to the File Changes table covering lines 18-30 (the numbered checklist itself, not just 31-36), describing that steps 1 and 8 need a phase-conditional caveat: checks run only when the lane's `sdd_phase` is `"apply"`, empty/missing, or carries an explicit exception (per Decision 2) — otherwise steps 1 and 8 are skipped and the checklist should say so rather than assert unconditional full-suite execution.

## Out of scope

- Do not modify any other section of `design.md` (Decisions 1, 2, 4, 5; Testing Strategy; Threat Matrix; Rollback and Additivity; Open Questions and Out of Scope) beyond what Gap 1 and Gap 2 require.
- Do not modify `design-synthesis-notes.md`, any lens draft, `proposal.md`, `explore.md`, or any spec file.
- Do not edit the real `plugin/*/skills/lucind-ai/SKILL.md` or `acceptance-promotion.md` files themselves — this is a design-document correction, not the implementation. Those edits happen in `sdd-apply`.
- Do not run `go test`, `go build`, or `lucind-checks.sh` — this phase only touches `openspec/changes/agentic-phase-specialist/**`.

## Allowed paths

`openspec/changes/agentic-phase-specialist/design.md` only. Create no other file.

## Citation manifest (REQUIRED — excluded from any word count)

If you add any new file:line citation not already present in `design.md`, close your edit with a `## Citation Manifest (Remediation)` appendix section listing it, one row per unique new citation, same format as the rest of this Change's packets (`| citation | claim |`). If you add no new citations (likely, since this is quoting existing text), state "No new citations added" instead.

## Mechanical self-check (REQUIRED)

Run `./lucind-lane-check.sh` from the repo root before and after committing:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design.md --skip-result
```

Confirm the file still parses as valid markdown and the word count did not balloon — this is a targeted addition of roughly 2-4 short paragraphs, not a rewrite. Paste the PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **Decision 3 (or an immediately adjacent subsection) contains the literal, verbatim replacement sentence(s) for `SKILL.md:19`**, presented as an old→new quoted block, ready to copy into both skill trees without further wordsmithing.
- [ ] **The out-of-repository note names `design.md` itself as the in-repo carrier and includes the literal instruction text to paste into each `~/.claude/skills/sdd-*/SKILL.md` file.**
- [ ] **The `## File Changes` table (or an adjacent note) explicitly covers `acceptance-promotion.md` checklist steps 1 and 8 (lines 18-30), stating they need the same `sdd_phase`-conditional caveat as the rest of the gate.**
- [ ] **No other section of `design.md` was modified.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by `lucind-lane-check.sh` reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Crafting the exact Hard Rule replacement sentence would require a security/authority judgment call beyond what Decision 3's existing semantic description already settled (e.g. if it seems to require deciding something Decision 3 left genuinely open, not just wording something it already decided).
- `design.md`'s current structure has changed since this packet was written such that the named sections/line numbers no longer exist.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`. This is the propose→spec/design→tasks SDD chain for inserting a phase-scoped agentic Specialist. `design` phase was synthesized and integrated, then judged by its own phase-Specialist (`sdd-design` subagent), which returned `needs-revision` with exactly the two gaps above — full verdict already summarized in this packet. The `spec` phase (sibling) was independently judged `accepted` with no revision needed. Both `tasks` waits on this correction landing.

## Required skills

- sdd-design

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
