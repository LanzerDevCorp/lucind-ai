---
id: propose-skill-anchoring-guardrails-lens-b
executor: agy
routed_by: capability impact and delta specs lens of the three-lens propose fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/propose-lens-b.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: 1bcc9ff596b7a0fd9bf8e84ea4f7b5f8e755d5d7
expected_parent_sha: 1bcc9ff596b7a0fd9bf8e84ea4f7b5f8e755d5d7
---

# Packet propose-skill-anchoring-guardrails-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-skill-anchoring-guardrails-lens-b  ·  **Branch:** lucind/propose-skill-anchoring-guardrails-lens-b

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/propose-lens-b.md`: user/operator/agent capability impact, modified/added capabilities, and delta specification requirements and scenarios for this change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

`explore.md` is integrated and accepted. Lens A and lens C run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/explore.md` exists.
- `openspec/changes/skill-anchoring-guardrails/propose-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-propose/SKILL.md` — the real `gentle-ai` propose skill.
2. `openspec/changes/skill-anchoring-guardrails/explore.md` — the accepted exploration; ground scenarios in its problem statement and candidate.
3. `cmd/lucind-ai/cli.go` — the exact current functions that print operator-facing reports (blocked/timeout lane report, integrate report with `reverted_ids`, `accept` receipt output, `split` DAG output), and `internal/worktree/worktree.go` (`worktree cleanup` subcommand wiring).
4. `openspec/specs/` for existing delta spec precedent in this repository, if any exists.
5. `plugin/claude-code/skills/lucind-ai/references/operations/troubleshooting.md`, `references/coordination/recovery-reconciliation.md`, `references/contracts/acceptance-promotion.md` — the exact reference docs the new banners must point operators/agents to.

Never guess at signatures or spec shapes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/propose-lens-b.md`:

```markdown
# Proposal Lens B — Capability Impact & Specs: Skill Anchoring & Worktree Cleanup Guardrails

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|

## Delta Specifications

### Requirement: <Requirement Name>

<Requirement text using RFC 2119 keywords: MUST, SHALL, SHOULD, MAY.>

#### Scenario: <Scenario Name>

- GIVEN <initial condition>
- WHEN <trigger event>
- THEN <observable outcome>

## Open Questions

- [ ] <unresolved question, or "None">
```

Author one `### Requirement` block per distinct behavior change: (a) worktree cleanup dirty-guardrail + `--force`, (b) blocked/timeout report banner, (c) integrate-report reverted-IDs banner, (d) accept-receipt qualitative-review banner, (e) split multi-wave `base_sha` warning banner, (f) troubleshooting.md TDD rescue protocol content. At least one scenario each.

## Size budget

`propose-lens-b.md` MUST be under 1000 words. Tables and structured delta specs over prose.

## Out of scope

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write architecture rationale or rollback mechanisms here.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/propose-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-propose/` — the real `gentle-ai` propose skill and its `references/`. The skill is authority on *what* a proposal document must contain; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row per unique citation, grouped by file, files alphabetical, line numbers ascending.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/propose-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "User & Capability Impact" --require-section "Delta Specifications" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/propose-lens-b.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every capability impact and delta spec requirement carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- Delta requirements conflict with existing base specs without explicit migration path.
- Capability impacts cannot be determined from packet context or code inspection.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted exploration selected Candidate 1 (fail-closed worktree guardrail + banner anchoring + prescriptive TDD rescue). Six distinct behavior surfaces need delta requirements: worktree cleanup `--force` gate, blocked/timeout report banner, integrate report reverted-IDs banner, accept receipt qualitative-review banner, split multi-wave warning banner, and the troubleshooting.md TDD rescue protocol text. Execution for this Change: Isolated Mode, SDD with fan-out planning, `agy`-only executor throughout except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
