---
id: tasks-agentic-phase-specialist-lens-c
executor: agy
routed_by: proof and review-burden lens of the three-lens tasks fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/tasks-lens-c.md"]
---

# Packet tasks-agentic-phase-specialist-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/tasks-agentic-phase-specialist-lens-c  ·  **Branch:** lucind/tasks-agentic-phase-specialist-lens-c

## Goal

Produce `openspec/changes/agentic-phase-specialist/tasks-lens-c.md`: what proves each piece of this change works — the RED test task for every applicable threat-matrix row, the acceptance evidence per task, and the Review Workload Forecast that says whether this ships as one PR or a chain.

This is one of three parallel tasks lenses. It is feedstock for a synthesis lane, not the final checklist. Do not write a complete `tasks.md`.

## Why this is safe to dispatch now

The spec and design for `agentic-phase-specialist` are accepted and frozen. Lens A and lens B run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the task decomposition and is running concurrently, so you do not have it. Derive the work from the design's file-changes table and testing strategy, declare it in `## Assumed decomposition`, and attach proof to that. The synthesizer arbitrates divergence.

## Preconditions

- `openspec/changes/agentic-phase-specialist/design.md` exists and carries a threat matrix (all 5 rows `N/A`) and a testing strategy table.
- `openspec/changes/agentic-phase-specialist/specs/` exists (4 capability files).
- `openspec/changes/agentic-phase-specialist/tasks-lens-c.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *proof and cost* — not to what the work is and not to how it is dispatched:

1. The real `gentle-ai` tasks skill (delivered under `## Required skills`), and its **Review Workload Forecast** table and threat-matrix rule in particular. It is the phase contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/agentic-phase-specialist/design.md` — the `## Threat Matrix` (~lines 121-127: all 5 canonical boundary rows are `N/A` with cited evidence — this change adds no new classification, ref, or PR surface) and the `## Testing Strategy and Test Seams` table (~lines 112-117: unit tests for `internal/accept` and `internal/run`, regression tests for `internal/integrate` and `internal/packet`). Since every threat-matrix row is `N/A`, expect the RED Tests table to be correspondingly empty of "Applicable" rows — do not invent applicability the design didn't find.
3. `openspec/changes/agentic-phase-specialist/specs/` — the scenarios, which are what the tests assert. All four capability files.
4. Real existing test files: `internal/accept/accept_test.go` (in particular `newVerifierFixture`, ~lines 26-67), `internal/run/attempt_test.go` (in particular `attemptSpies`/`checkCalls`, ~lines 24-44, 83-92), `internal/integrate/integrate_test.go` (`TestCheck*`, ~line 471), `internal/packet/packet_test.go` (`TestSkillTreesByteIdentical` ~lines 943-967, `TestSkillAssetContract` ~lines 924-941) — so a proposed test command is one this repository can actually run and a proposed seam is one that exists.

Never propose a test command you did not derive from a real test file, and never claim a test proves something without saying which assertion fires.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/tasks-lens-c.md`:

```markdown
# Tasks Lens C — Proof & Review Burden: Agentic Phase Specialist

## Assumed decomposition

<2-4 sentences naming the work breakdown you are attaching proof to: how many
units, what each delivers, and what the critical path is. Lens A and lens B write
this same block independently; the synthesizer compares all three. Be specific
enough that a disagreement is visible.>

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | <a number or a range, with the basis for it> |
| 400-line budget risk | Low / Medium / High |
| Chained PRs recommended | Yes / No |
| Suggested split | <single PR, or PR 1 -> PR 2 -> PR 3> |
| Delivery strategy | auto-chain |
| Chain strategy | <stacked-to-main / feature-branch-chain / size-exception / pending> |

<State the basis for the line estimate. An estimate with no basis has been wrong
by an order of magnitude in this repository before — a fan-out template forecast
of 120-250 lines came in at 1730 — so name what you counted: count the actual
current line spans of `internal/accept/accept.go` (~lines 84-137 touched),
`internal/run/attempt.go` (~lines 431-448 touched), plus their test files, plus
the ~8-line Hard Rule / fan-out.md / acceptance-promotion.md doc edits × 2 trees
each. Note: the human's SDD session preflight for this Change already set the
review budget at 1500 lines — say explicitly whether your estimate is under or
over that, since it is stricter than this repository's `openspec/config.yaml`
default of 10000.>

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|

<One row per threat-matrix row in the design. Copy the `Applicable` / `N/A`
verdict from the design; do not re-decide it and do not add rows the design does
not have. All 5 rows in this design are `N/A` — if that holds, this table is
5 rows, all `N/A`, with no RED test column populated for any of them, and you
must still separately name the RED tests this change needs for its actual
behavior change (gating in `accept.go`/`attempt.go`) under `## Acceptance
Evidence` below, since the threat matrix's `N/A` rows are about the *fixed*
security boundaries (git/commit/push/PR/doc-path handling), not about whether
the new phase-gating logic itself needs tests — it does, and belongs in
Acceptance Evidence, not here.>

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|

<The proving command is the smallest one that fails before the task and passes
after — a focused `go test -run` over a package, not the whole suite. The last
column is the honest one: a passing test is not evidence its assertion fires, so
say what still needs a mutation check or a real run. Cover at minimum: the
accept.go gate (apply/empty/exception/missing/mixed cases), the attempt.go gate,
and the byte-identity/glossary regression tests that any skill-doc edit risks
breaking.>

## Verification Gaps

<Every behavior the specs require that no proposed test proves, and what would
have to exist to prove it. "None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`tasks-lens-c.md` MUST be under 1000 words. Tables over prose. Keep every cell to a clause.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: the phased checklist itself, the dependency-order table, and requirement traceability.
- **Lens B owns**: the Suggested Work Units table, the wave partition, per-unit `allowed_paths`, executor assignment, and the sidecar recommendation.

Do not write the task checklist and do not partition units into waves. You attach proof and cost to the work; where a unit boundary lands is lens B's.

## Allowed paths

`openspec/changes/agentic-phase-specialist/tasks-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` tasks skill and its `references/` (delivered under `## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a task file must contain*: the Review Workload Forecast table and its fields, and the rule that every applicable threat-matrix case becomes an explicit RED-test task before its production task while `N/A` rows stay omitted. Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the task breakdown is split across three parallel lanes, which slice this lane owns, its word budget, its output path and skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing the whole `tasks.md` by itself, so parts of it will read as instructing you to do what this packet forbids — write the checklist, write the work-units table, persist to Engram, return the phase summary block. Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row per unique citation.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft -- never copy this placeholder row verbatim> |

Rules:

- **One row per UNIQUE citation.** Group by file, files alphabetical, line numbers ascending.
- **The claim is what YOU assert that range shows** — one line, no hedging.
- **This section does not count against the word budget.**
- **The manifest is a worklist, not a certificate.** The synthesizer opens and checks every single one.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/tasks-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed decomposition" --require-section "Review Workload Forecast" \
  --require-section "RED Tests from the Threat Matrix" --require-section "Acceptance Evidence" \
  --require-section "Verification Gaps" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/tasks-lens-c.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every threat-matrix row in the design appears in `## RED Tests from the Threat Matrix` with the design's own `Applicable` / `N/A` verdict**, and no row was invented.
- [ ] **Every applicable row names a RED test and the assertion that fires.**
- [ ] **The changed-line estimate states the basis it was derived from**, not a bare number, and states whether it is under or over the human's 1500-line session review budget.
- [ ] **Every proving command is derived from a real test file in this worktree**, cited with `file:line`.
- [ ] **`tasks-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries `## Assumed decomposition`, `## Review Workload Forecast`, `## Acceptance Evidence`, `## Verification Gaps`, and `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The design has no threat matrix, so there is nothing to derive RED tests from.
- A behavior the specs require cannot be proven through any test this repository can run, and no new seam is identified in the design.
- The changed-line estimate cannot be grounded in anything — no comparable file, no archived change of similar shape.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`. Design's Threat Matrix (`design.md` ~lines 119-127): 5 rows, all `N/A` (documentation-like paths, git repository selection, commit state, push state, PR commands) — this change is a phase-gating logic change plus doc edits, not a new security surface. Testing Strategy table (~lines 112-117) names the real seams: `Verifier.check` (`accept.go:49,55`), `newVerifierFixture` (`accept_test.go:26-67`), `UpdateLaneMetadata` (`lanes_meta.go:49-60`); `Deps.RunChecks` (`run.go:208`), `attemptSpies.checkFunc` (`attempt_test.go:41,83-92`); `integrate.Check` regression (`integrate.go:159-200`, `TestCheck*` at `integrate_test.go:471`); `TestSkillTreesByteIdentical` (`packet_test.go:943-967`) and `TestSkillAssetContract` (`:924-941`) for the doc edits.

**Human-chosen SDD session preflight for this Change**: delivery strategy `auto-chain`, review budget **1500 lines** (stricter than the repo's `openspec/config.yaml` default of 10000 — use 1500 as your actual risk threshold, not 10000).

## Required skills

- sdd-tasks

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: threat-matrix rows covered, RED tests named. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
