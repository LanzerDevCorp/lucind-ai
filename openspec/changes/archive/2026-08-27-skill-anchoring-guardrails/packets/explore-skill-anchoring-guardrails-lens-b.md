---
id: explore-skill-anchoring-guardrails-lens-b
executor: agy
routed_by: capabilities and scenarios lens of the three-lens explore fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/explore-lens-b.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: ade62b5e45f7def0c99d84825257a3a612b9ebdb
expected_parent_sha: ade62b5e45f7def0c99d84825257a3a612b9ebdb
---

# Packet explore-skill-anchoring-guardrails-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/explore-skill-anchoring-guardrails-lens-b  ·  **Branch:** lucind/explore-skill-anchoring-guardrails-lens-b

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/explore-lens-b.md`: the capability impact, operator/agent journeys, concrete scenarios, and measurable success criteria for this change.

This is one of three parallel explore lenses. It is feedstock for a synthesis lane, not the final explore document. Do not write a complete `explore.md`.

## Why this is safe to dispatch now

Lens A and lens C run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/explore-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-explore/SKILL.md` — the real `gentle-ai` explore skill. It is the phase contract this draft feeds.
2. `docs/plan_1_audit_and_skill_anchoring.md` — a human-authored audit/draft plan proposing this change. Treat it as a strong lead only: every claim in it MUST be independently re-verified against the real code in this worktree before you repeat it.
3. `cmd/lucind-ai/cli.go` — the operator-facing CLI surface: `worktree cleanup`, `run`/`split`/`accept`/`integrate retry` output, current flags, and existing report-printing functions.
4. `plugin/claude-code/skills/lucind-ai/SKILL.md` and `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md` — the agent-facing skill surface this change is meant to anchor into CLI output.
5. `internal/worktree/worktree_test.go` and `cmd/lucind-ai/cli_test.go` — existing behavioral expectations around `worktree cleanup`.

Never guess at code. Every claim about existing behavior carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/explore-lens-b.md`:

```markdown
# Explore Lens B — Capabilities & Scenarios: Skill Anchoring & Worktree Cleanup Guardrails

## User & Capability Impact

<Who is affected (operators, dispatched agents), what new capabilities or modified capabilities are introduced. Ground claims in existing code with file:line citations.>

## Scenarios & Use Cases

### Scenario 1 — <title>

- **Context**: <starting condition or initial state>
- **Action**: <user action or system trigger>
- **Outcome**: <expected result and observable change>

### Scenario N — <title>

<same format>

## Success Criteria

- [ ] <measurable, observable criterion for success>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`explore-lens-b.md` MUST be under 1000 words. Scenarios and criteria as compact, testable items.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: problem space definition, candidate approaches, and initial recommendations.
- **Lens C owns**: technical risks, unknowns, trade-offs matrix, and potential spikes/proof-of-concepts.

Do not rank candidate architectures or write a technical risk matrix here. They belong to lenses A and C.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/explore-lens-b.md` only. Create no other file.

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
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "User & Capability Impact" --require-section "Scenarios & Use Cases" \
  --require-section "Success Criteria" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/explore-lens-b.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every scenario and capability claim carries `file:line` citations to real code in this worktree.**
- [ ] **`explore-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The capability impact cannot be determined from packet context or codebase inspection.
- Scenarios conflict with established system invariants and cannot be reconciled.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. The human wants to stop agents/operators from acting on reflex or intuition when a lucind-ai Lane blocks, times out, or leaves an intermediate state — specifically: (1) `internal/worktree.Cleanup`/`Remove` currently calls `git worktree remove --force` unconditionally with no dirty-tree guardrail; (2) the orchestrator/agent-facing skill references under `plugin/claude-code/skills/lucind-ai/references/` already document recovery/qualitative-review guidance but the CLI's own terminal output never points at them; (3) there is no prescriptive protocol for rescuing partial TDD (RED-written / partial-GREEN) work before a timeout-triggered cleanup. `docs/plan_1_audit_and_skill_anchoring.md` is the human's own draft audit of this — a lead to verify, not a spec to copy. Execution for this Change: Isolated Mode (feature `skill-anchoring-guardrails`, parent `refs/heads/feature/skill-anchoring-guardrails`, base `ade62b5`), SDD with fan-out planning, `agy`-only executor throughout except verify's second qualitative judge (kept on `cursor-agent`) — already decided; do not re-litigate it.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
