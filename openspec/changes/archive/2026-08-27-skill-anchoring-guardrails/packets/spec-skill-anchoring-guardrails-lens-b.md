---
id: spec-skill-anchoring-guardrails-lens-b
executor: agy
routed_by: scenarios and coverage lens of the three-lens spec fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/spec-lens-b.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: f5a531183361804ed95c797e16a70dbbcca27763
expected_parent_sha: f5a531183361804ed95c797e16a70dbbcca27763
---

# Packet spec-skill-anchoring-guardrails-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-skill-anchoring-guardrails-lens-b  ·  **Branch:** lucind/spec-skill-anchoring-guardrails-lens-b

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/spec-lens-b.md`: a Given/When/Then scenario set for every requirement this change introduces or changes, plus the coverage argument.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/skill-anchoring-guardrails/specs/`.

## Why this is safe to dispatch now

`proposal.md` is accepted and frozen, and its Delta Specifications section already drafts one scenario per requirement. Lens A and lens C run in parallel and write to different files.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the requirements from the proposal itself, declare them in `## Assumed requirements`, and key every scenario to one by name.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/proposal.md` exists and is accepted.
- `openspec/changes/skill-anchoring-guardrails/spec-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill.
2. `openspec/changes/skill-anchoring-guardrails/proposal.md`, its **Capabilities** and **Delta Specifications** sections in full — it drafted one happy-path scenario per requirement; your job is to add the missing edge cases and error states, not just restate the happy path.
3. Two or three archived delta specs under `openspec/changes/archive/*/specs/` for this repository's scenario granularity conventions.
4. `internal/worktree/worktree.go`, `cmd/lucind-ai/cli.go` — enough to know what a precondition (dirty vs. clean worktree, `--force` present/absent) and an observable outcome (exit code, stdout/stderr content, file presence) actually are here.

Never invent a state the system cannot be in.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/spec-lens-b.md`:

```markdown
# Spec Lens B — Scenarios & Coverage: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed requirements

<2–4 sentences naming the six requirements from the proposal's Delta Specifications section.>

## Scenarios

### Requirement: Worktree cleanup dirty guardrail and force flag

#### Scenario: Refuse cleanup on dirty worktree without force

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>

#### Scenario: Force cleanup removes dirty worktree

<same shape — cover the happy path from the proposal PLUS at least one edge case, e.g. untracked-only dirtiness, or nonexistent worktree path>

#### Scenario: <error state, if any — e.g. force cleanup on already-integrated lane>

### Requirement: Blocked and timeout lane report guidance banner

<happy path + edge case, e.g. lane blocked with no prior commits>

### Requirement: Integration report reverted IDs recovery banner

<happy path + edge case, e.g. mixed integrated and reverted IDs in one batch>

### Requirement: Acceptance receipt qualitative review banner

<happy path + edge case>

### Requirement: DAG split multi-wave base SHA warning banner

<happy path + edge case, e.g. single-wave DAG should NOT emit the warning>

### Requirement: Prescriptive TDD WIP-rescue protocol documentation

<happy path — this one is documentation-only; note in Untestable Assertions if no automated scenario applies>

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|

## Untestable Assertions

<"None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-b.md` MUST be under 1000 words.

## Out of scope

- **Lens A owns**: the capability map, the requirement statements themselves, and their classification.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement, and every Migration note.

Do NOT create or write any file under `openspec/changes/skill-anchoring-guardrails/specs/`. That tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/spec-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its `references/`. The skill is authority on *what* a delta spec must contain; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/spec-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed requirements" --require-section "Scenarios" \
  --require-section "Coverage" --require-section "Untestable Assertions" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/spec-lens-b.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every requirement named in `## Assumed requirements` has at least one scenario**, and every scenario is in GIVEN / WHEN / THEN form.
- [ ] **Every scenario's THEN names an observable outcome**, and the coverage table cites the seam it is observable through or marks it "new seam required".
- [ ] **`spec-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed requirements`, `## Coverage`, `## Untestable Assertions`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- The proposal does not determine what the system should do in a case, so the scenario would assert an outcome nobody chose.
- Every scenario for a requirement would be untestable, meaning the requirement as proposed is unobservable.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted proposal's Delta Specifications section drafted these six requirements with one scenario each: worktree cleanup dirty guardrail + force flag (3 scenarios already drafted: refuse without force, force removes, clean succeeds idempotently); blocked/timeout report banner; integration report reverted-IDs banner; acceptance receipt qualitative-review banner; DAG split multi-wave base-SHA banner; TDD WIP-rescue protocol documentation. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
