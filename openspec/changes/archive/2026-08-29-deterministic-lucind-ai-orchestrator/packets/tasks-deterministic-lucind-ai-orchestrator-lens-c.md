---
id: tasks-deterministic-lucind-ai-orchestrator-lens-c
executor: agy
routed_by: proof and review-burden lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-c.md"]
---

# Packet tasks-deterministic-lucind-ai-orchestrator-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/tasks-deterministic-lucind-ai-orchestrator-lens-c  ·  **Branch:** lucind/tasks-deterministic-lucind-ai-orchestrator-lens-c

## Goal

Produce `openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-c.md`: what proves each
piece of this change works — the RED test task for every applicable threat-matrix row, the
acceptance evidence per task, and the Review Workload Forecast that says whether this ships as one
PR or a chain.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final
checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

The spec and design for `deterministic-lucind-ai-orchestrator` are accepted and frozen. Lens A and
lens B run in parallel against the same frozen inputs and write to different files, so no lane
races another.

Lens A owns the task decomposition and is running concurrently, so you do not have it. Derive the
work from the design's file-changes table and testing strategy, declare it in
`## Assumed decomposition`, and attach proof to that. The synthesizer arbitrates divergence.

## Preconditions

- `openspec/changes/deterministic-lucind-ai-orchestrator/design.md` exists and carries a threat
  matrix and a testing strategy.
- `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` exists.
- `openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-c.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *proof and cost* — not to what the
work is and not to how it is dispatched:

1. `~/.claude/skills/sdd-tasks/SKILL.md` — the real `gentle-ai` tasks skill, and its **Review
   Workload Forecast** table and threat-matrix rule in particular. It is the phase contract this
   draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/deterministic-lucind-ai-orchestrator/design.md` — the threat matrix
   (`## Threat Matrix`) and the testing strategy (`## Testing Strategy and Test Seams`). Every row
   marked `Applicable` becomes an explicit RED-test task before its production task; rows marked
   `N/A` stay omitted. Invent no rows.
3. `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` — the scenarios, which are what
   the tests assert.
4. The existing test files for `cmd/lucind-ai`, `internal/packet`, `internal/dag`, `internal/run`,
   so a proposed test command is one this repository can actually run and a proposed seam is one
   that exists.

Never propose a test command you did not derive from a real test file, and never claim a test
proves something without saying which assertion fires.

## Output format

Write exactly this skeleton to
`openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-c.md`:

```markdown
# Tasks Lens C — Proof & Review Burden: Deterministic lucind-ai Orchestrator

## Assumed decomposition

<2–4 sentences naming the work breakdown you are attaching proof to: how many
units, what each delivers, and what the critical path is. Lens A and lens B write
this same block independently; the synthesizer compares all three. Be specific
enough that a disagreement is visible.>

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | <a number or a range, with the basis for it> |
| 400-line budget risk | Low / Medium / High |
| Chained PRs recommended | Yes / No |
| Suggested split | <single PR, or PR 1 → PR 2 → PR 3> |
| Delivery strategy | ask-on-risk |
| Chain strategy | <stacked-to-main / feature-branch-chain / size-exception / pending> |

<State the basis for the line estimate. An estimate with no basis has been wrong
by an order of magnitude in this repository before — a fan-out template forecast
of 120–250 lines came in at 1730 — so name what you counted: files times a
comparable file's size, or an archived change of similar shape.>

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|

<One row per threat-matrix row in the design. Copy the `Applicable` / `N/A`
verdict from the design; do not re-decide it and do not add rows the design does
not have. For every applicable row, the RED test names the concrete failure it
reproduces and the assertion that fires.>

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|

<The proving command is the smallest one that fails before the task and passes
after — a focused `go test -run` over a package, not the whole suite. The last
column is the honest one: a passing test is not evidence its assertion fires, so
say what still needs a mutation check or a real run.>

## Verification Gaps

<Every behavior the specs require that no proposed test proves, and what would
have to exist to prove it. "None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`tasks-lens-c.md` MUST be under 1000 words. Tables over prose. Keep every cell to a clause.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the
synthesizer and wastes the lane:

- **Lens A owns**: the phased checklist itself, the dependency-order table, and requirement
  traceability.
- **Lens B owns**: the Suggested Work Units table, the wave partition, per-unit `allowed_paths`,
  executor assignment, and the sidecar recommendation.

Do not write the task checklist and do not partition units into waves. You attach proof and cost
to the work; where a unit boundary lands is lens B's.

## Allowed paths

`openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-c.md` only. Create no other
file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-tasks/` — the real `gentle-ai` tasks skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a task file must contain*: the Review Workload Forecast table and
its fields, and the rule that every applicable threat-matrix case becomes an explicit RED-test task
before its production task while `N/A` rows stay omitted. Where this packet paraphrases any of
that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the task breakdown is
split across three parallel lanes, which slice this lane owns, its word budget, its output path and
skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing
the whole `tasks.md` by itself, so parts of it will read as instructing you to do what this packet
forbids — write the checklist, write the work-units table, persist to Engram, return the phase
summary block. Those are superseded here on purpose. Do not correct yourself toward them; note the
conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation, grouped by file, files alphabetical, ascending within each file.

| citation | claim |
|---|---|
| `openspec/changes/deterministic-lucind-ai-orchestrator/design.md:106-107` | design's Threat Matrix marks Git repository selection and Commit state Applicable with named RED tests |

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Review Workload Forecast" \
  --require-section "RED Tests from the Threat Matrix" --require-section "Acceptance Evidence" \
  --require-section "Verification Gaps" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/deterministic-lucind-ai-orchestrator/tasks-lens-c.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of
narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the
  claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL
  against this draft's own manifest.**
- [ ] **Every threat-matrix row in the design appears in `## RED Tests from the Threat Matrix` with
  the design's own `Applicable` / `N/A` verdict**, and no row was invented.
- [ ] **Every applicable row names a RED test and the assertion that fires.**
- [ ] **The changed-line estimate states the basis it was derived from**, not a bare number.
- [ ] **Every proving command is derived from a real test file in this worktree**, cited with
  `file:line`.
- [ ] **`tasks-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries
  `## Assumed decomposition`, `## Review Workload Forecast`, `## Acceptance Evidence`,
  `## Verification Gaps`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- The design has no threat matrix, so there is nothing to derive RED tests from.
- A behavior the specs require cannot be proven through any test this repository can run, and no
  new seam is identified in the design.
- The changed-line estimate cannot be grounded in anything — no comparable file, no archived
  change of similar shape.
- Satisfying one instruction in this packet would require violating another.

## Context

**Ground truth — cite it, do not re-derive it. Verified directly in this worktree before this
packet was authored:**

- `design.md`'s Threat Matrix (`design.md:101-109`) has exactly four rows: Documentation-like
  paths (N/A — packet markdown is prompt text, not executable classification), Git repository
  selection (Applicable — `resolvePrimaryRoot`, linked/sibling worktrees fail closed), Commit state
  (Applicable — `enforceCompletionMode`/`enforceAllowedPaths`, clean porcelain, unique-commit
  rules), Push state (N/A — local branches/CAS only), PR commands (N/A — PR/review are human-owned).
- `design.md`'s Testing Strategy table (`design.md:84-99`) already names ten seam rows spanning
  unit, integration, and E2E layers, each with an existing or new seam citation — use these as the
  basis for proving commands rather than inventing new ones.
- The delivery strategy the human selected this session is `ask-on-risk` with a 5000-changed-line
  review budget: populate the Delivery strategy field with exactly `ask-on-risk`, and only mark
  400-line budget risk `High`/chained-PRs `Yes` if your own honest estimate exceeds roughly
  5000 lines, not the smaller 400-line default this repository's generic skill assumes.
- Five accepted spec requirements exist, one per capability, under
  `openspec/changes/deterministic-lucind-ai-orchestrator/specs/`.

**Decided already — do not re-litigate:** no new lifecycle states, scheduler/wave engine, flags,
routing mechanism, or replacement for existing Combine/Resolve/Check/bisect/CAS primitives
(`proposal.md:14-15`).

## Required skills

- ~/.claude/skills/sdd-tasks/SKILL.md

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
