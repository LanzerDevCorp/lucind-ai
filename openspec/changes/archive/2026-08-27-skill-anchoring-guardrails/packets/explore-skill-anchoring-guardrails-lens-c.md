---
id: explore-skill-anchoring-guardrails-lens-c
executor: agy
routed_by: risks, trade-offs, and spikes lens of the three-lens explore fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/explore-lens-c.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: ade62b5e45f7def0c99d84825257a3a612b9ebdb
expected_parent_sha: ade62b5e45f7def0c99d84825257a3a612b9ebdb
---

# Packet explore-skill-anchoring-guardrails-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/explore-skill-anchoring-guardrails-lens-c  ·  **Branch:** lucind/explore-skill-anchoring-guardrails-lens-c

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/explore-lens-c.md`: technical risks, trade-offs matrix, spike proposals, and out-of-scope boundaries for this change.

This is one of three parallel explore lenses. It is feedstock for a synthesis lane, not the final explore document. Do not write a complete `explore.md`.

## Why this is safe to dispatch now

Lens A and lens B run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/explore-lens-c.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-explore/SKILL.md` — the real `gentle-ai` explore skill. It is the phase contract this draft feeds.
2. `docs/plan_1_audit_and_skill_anchoring.md` — a human-authored audit/draft plan proposing this change. Treat it as a strong lead only: every claim in it (including its "Dificultades y Desafíos Técnicos" section) MUST be independently re-verified against the real code in this worktree before you repeat it.
3. `internal/worktree/worktree.go` and `internal/worktree/worktree_test.go` — dirty-detection risk surface, existing callers of `Cleanup`/`Remove` that assume unconditional deletion (backward-compat risk).
4. `cmd/lucind-ai/cli.go` and any script that parses its stdout/stderr (e.g. CI, `lucind-checks.sh`) — risk of banner text breaking structured/parsed output.
5. `openspec/changes/archive/` for historical trade-offs and edge-case postmortems, if any exist.

Never guess at code or risks. Every claim about existing code mechanisms carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/explore-lens-c.md`:

```markdown
# Explore Lens C — Risks, Trade-offs & Spikes: Skill Anchoring & Worktree Cleanup Guardrails

## Technical Risks & Unknowns

| Risk / Unknown | Severity | Mitigation / Investigation Strategy | Existing seam (file:line) |
|---|---|---|---|

## Trade-offs Matrix

| Dimension / Choice | Advantages | Disadvantages | Operational Cost |
|---|---|---|---|

## Potential Spikes / Proof of Concepts

<Targeted experiments or prototype spikes needed to de-risk technical unknowns, citing existing code seams with file:line.>

## Out of Scope

<Adjacent problems, features, or refactors explicitly excluded from this change.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`explore-lens-c.md` MUST be under 1000 words. Tables over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: problem space definition, candidate approaches, and initial recommendations.
- **Lens B owns**: capabilities, user scenarios, and success criteria.

Do not define user personas or author candidate architectural designs here. They belong to lenses A and B.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/explore-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-explore/` — the real `gentle-ai` explore skill and its `references/`. Read the contract as written, not as this packet paraphrases it.

The skill is authority on *what* an explore document must contain. This packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row per unique citation, grouped by file, files alphabetical, line numbers ascending.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Technical Risks & Unknowns" --require-section "Trade-offs Matrix" \
  --require-section "Potential Spikes / Proof of Concepts" --require-section "Out of Scope" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore-lens-c.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every risk and spike proposal carries `file:line` citations to real code in this worktree.**
- [ ] **`explore-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Critical unknown cannot be framed as a risk or spike with available codebase knowledge.
- The change requires touching fundamental invariant boundaries without possible mitigation.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. The human wants to stop agents/operators from acting on reflex or intuition when a lucind-ai Lane blocks, times out, or leaves an intermediate state — specifically: (1) `internal/worktree.Cleanup`/`Remove` currently calls `git worktree remove --force` unconditionally with no dirty-tree guardrail; (2) the orchestrator/agent-facing skill references under `plugin/claude-code/skills/lucind-ai/references/` already document recovery/qualitative-review guidance but the CLI's own terminal output never points at them; (3) there is no prescriptive protocol for rescuing partial TDD (RED-written / partial-GREEN) work before a timeout-triggered cleanup. `docs/plan_1_audit_and_skill_anchoring.md` is the human's own draft audit of this — a lead to verify, not a spec to copy; its "Dificultades y Desafíos Técnicos" section specifically names backward-compat test risk, robust dirty-detection, and not contaminating structured stdout as open technical risks worth scrutinizing. Execution for this Change: Isolated Mode (feature `skill-anchoring-guardrails`, parent `refs/heads/feature/skill-anchoring-guardrails`, base `ade62b5`), SDD with fan-out planning, `agy`-only executor throughout except verify's second qualitative judge (kept on `cursor-agent`) — already decided; do not re-litigate it.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
