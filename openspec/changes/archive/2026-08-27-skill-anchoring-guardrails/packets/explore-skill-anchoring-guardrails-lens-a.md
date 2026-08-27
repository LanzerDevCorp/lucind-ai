---
id: explore-skill-anchoring-guardrails-lens-a
executor: agy
routed_by: problem and candidates lens of the three-lens explore fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/explore-lens-a.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: ade62b5e45f7def0c99d84825257a3a612b9ebdb
expected_parent_sha: ade62b5e45f7def0c99d84825257a3a612b9ebdb
---

# Packet explore-skill-anchoring-guardrails-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/explore-skill-anchoring-guardrails-lens-a  ·  **Branch:** lucind/explore-skill-anchoring-guardrails-lens-a

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/explore-lens-a.md`: the problem space definition, motivation, background, and candidate approaches for this change, each with its description, pros, cons, and feasibility assessment.

This is one of three parallel explore lenses. It is feedstock for a synthesis lane, not the final explore document. Do not write a complete `explore.md`.

## Why this is safe to dispatch now

The exploration for `skill-anchoring-guardrails` is initiating. Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns the problem definition and candidate solutions; the other two explore capabilities and risks.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/explore-lens-a.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-explore/SKILL.md` — the real `gentle-ai` explore skill. It is the phase contract this draft feeds.
2. `docs/plan_1_audit_and_skill_anchoring.md` — a human-authored audit/draft plan proposing this change. Treat it as a strong lead only: every claim in it (file:line references, code snippets, described gaps) MUST be independently re-verified against the real code in this worktree before you repeat it. It has not been through any citation verification.
3. `internal/worktree/worktree.go` — the current `Cleanup`/`Remove` implementation and its callers.
4. `cmd/lucind-ai/cli.go` — the `worktree cleanup`, `run`, `split`, `accept`, and `integrate retry` subcommand wiring, and their report-printing functions (`printReport`, `printIntegrateReport`, `runAccept`, `runSplit` or their current equivalents).
5. `openspec/changes/archive/` for prior explorations or changes that addressed similar problem spaces, if one exists.

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/explore-lens-a.md`:

```markdown
# Explore Lens A — Problem & Candidates: Skill Anchoring & Worktree Cleanup Guardrails

## Problem Space

<Concise description of the problem, background, current limitations, and motivation for the change. Cite file:line for existing code behavior.>

## Candidate Approaches

### Candidate 1 — <title>

**Approach**: <summary of candidate approach>
**Pros**: <advantages>
**Cons**: <disadvantages and costs>
**Feasibility**: <assessment grounded in this codebase with file:line citations>

### Candidate N — <title>

<same four fields>

## Initial Recommendations

<Preliminary recommendation among candidates, with technical rationale.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`explore-lens-a.md` MUST be under 1000 words. Candidate descriptions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: capabilities, user scenarios, and success criteria.
- **Lens C owns**: technical risks, unknowns, trade-offs matrix, and potential spikes/proof-of-concepts.

Do not write a risks matrix or detailed user scenarios here. They belong to lenses B and C.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/explore-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill and its `references/`. Read the contract as written, not as this packet paraphrases it.

The skill is authority on *what* an explore document must contain. This packet is authority on *how this phase is executed here*: three parallel lanes, this lane's slice, word budget, output path/skeleton, out-of-scope list, and done criteria. Where the skill describes one agent writing a whole `explore.md` alone, persisting to Engram, or returning a phase summary block, that is superseded here on purpose — note the conflict in `## Open Questions`, follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row per unique citation, grouped by file, files alphabetical, line numbers ascending. The claim is what YOU assert that range shows.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Problem Space" --require-section "Candidate Approaches" \
  --require-section "Initial Recommendations" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore-lens-a.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every candidate and problem claim carries `file:line` citations to real code in this worktree.**
- [ ] **`explore-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The problem scope cannot be identified from codebase inspection or packet context.
- Candidate approaches cannot be formulated without designing complete implementation details (which belongs to design phase).
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. The human wants to stop agents/operators from acting on reflex or intuition when a lucind-ai Lane blocks, times out, or leaves an intermediate state — specifically: (1) `internal/worktree.Cleanup`/`Remove` currently calls `git worktree remove --force` unconditionally with no dirty-tree guardrail, risking silent loss of uncommitted TDD progress; (2) the orchestrator/agent-facing skill references under `plugin/claude-code/skills/lucind-ai/references/` already document recovery/qualitative-review guidance but the CLI's own terminal output never points at them at the moment it matters; (3) there is no prescriptive protocol for rescuing partial TDD (RED-written / partial-GREEN) work before a timeout-triggered cleanup. `docs/plan_1_audit_and_skill_anchoring.md` is the human's own draft audit of this — a lead to verify, not a spec to copy. Execution for this Change: Isolated Mode (feature `skill-anchoring-guardrails`, parent `refs/heads/feature/skill-anchoring-guardrails`, base `ade62b5`), SDD with fan-out planning, `agy`-only executor throughout except verify's second qualitative judge (kept on `cursor-agent` for adversarial cross-check) — this is already decided; do not re-litigate it, only note if you find a technical reason it's unworkable.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
