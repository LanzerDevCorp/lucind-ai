---
id: spec-conflict-triage-fixture-lens-b
executor: agy
routed_by: scenarios and coverage lens of the three-lens spec fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/conflict-triage-fixture/spec-lens-b.md"]
---

# Packet spec-conflict-triage-fixture-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/spec-conflict-triage-fixture-lens-b  ·  **Branch:** lucind/spec-conflict-triage-fixture-lens-b

## Goal

Produce `openspec/changes/conflict-triage-fixture/spec-lens-b.md`: a Given/When/Then scenario set for every requirement this change introduces or changes, plus the coverage argument that says which happy paths, edge cases, and error states are proven and which are not.

This is one of three parallel spec lenses. It is feedstock for a synthesis lane, not the final delta spec. Do not write anything under `openspec/changes/conflict-triage-fixture/specs/`.

## Why this is safe to dispatch now

The proposal for `conflict-triage-fixture` is accepted and frozen. Lens A and lens C run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the requirement set and is running concurrently, so you do not have it. Derive the requirements you are writing scenarios for from the proposal itself, declare them in `## Assumed requirements`, and key every scenario to one of them by name. The synthesizer arbitrates divergence; scenarios keyed to a requirement nobody else named are dropped, so name them the way the proposal does.

## Preconditions

- `openspec/changes/conflict-triage-fixture/proposal.md` exists and is accepted.
- `openspec/changes/conflict-triage-fixture/spec-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to *proof of behavior* — not to which requirements exist and not to migration:

1. `~/.claude/skills/sdd-spec/SKILL.md` — the real `gentle-ai` spec skill. It is the phase
   contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/conflict-triage-fixture/proposal.md`, and its **Capabilities section** in particular —
   that is where you derive the requirement names your scenarios key to.
3. Two or three archived delta specs under `openspec/changes/archive/*/specs/` — read how this
   repository actually writes a scenario: the granularity of a GIVEN, whether a THEN names a
   concrete value or a vague outcome, how many scenarios a requirement typically carries.
4. The code paths the proposal names, enough to know what a precondition and an observable
   outcome actually are here. A scenario whose THEN cannot be observed is not testable.

Never invent a state the system cannot be in. A precondition you cannot reach is a scenario nobody can write a test from.

## Output format

Write exactly this skeleton to `openspec/changes/conflict-triage-fixture/spec-lens-b.md`:

```markdown
# Spec Lens B — Scenarios & Coverage: Conflict Triage Fixture

## Assumed requirements

<2–4 sentences naming the requirement set you are writing scenarios for: which
capabilities this change touches and what each requirement asserts. Lens A and
lens C write this same block independently; the synthesizer compares all three.
Be specific enough that a disagreement is visible.>

## Scenarios

### Requirement: <Name, as the proposal names it>

#### Scenario: <Happy path>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>
- AND <additional outcome, if any>

#### Scenario: <Edge case>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable outcome>

#### Scenario: <Error state>

- GIVEN <precondition>
- WHEN <action>
- THEN <observable failure — the error, the exit code, the refusal>

### Requirement: <Next name>

<same shape>

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|

<One row per requirement. Mark each column covered or missing — "missing" is a
legitimate and useful answer. The last column cites the seam a test would assert
through, or states "new seam required".>

## Untestable Assertions

<Every scenario you wanted to write but could not, because its THEN is not
observable through anything that exists. Name the requirement and what would have
to exist. "None" if there are none.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`spec-lens-b.md` MUST be under 1000 words. Scenarios are the bulk of it, so keep every GIVEN / WHEN / THEN to one line. If the scenario set does not fit, cover every requirement's happy path first, then edge cases, then error states, and record what you had to leave out under `## Open Questions`.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: the capability map, the requirement statements themselves, and their ADDED / MODIFIED / REMOVED / RENAMED classification.
- **Lens C owns**: conflicts against live specs, the full-block copy of each MODIFIED requirement, and every Migration note.

Do not restate a requirement's text above its scenarios beyond the `### Requirement: <Name>` heading. The name is the join key; the text is lens A's.

Do NOT create or write any file under `openspec/changes/conflict-triage-fixture/specs/`. That tree belongs to the synthesizer.

## Allowed paths

`openspec/changes/conflict-triage-fixture/spec-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-spec/` — the real `gentle-ai` spec skill and its
`references/`. Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a delta spec must contain*: the Given/When/Then scenario format,
the "every requirement has at least one scenario" rule, the happy-path-and-edge-case rule, and the
"specs describe WHAT, not HOW" rule. Where this packet paraphrases any of that and drifts, the
skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the spec is split across
three parallel lanes, which slice this lane owns, its word budget, its output path and skeleton,
its out-of-scope list, and its done criteria. The skill describes one sub-agent writing the whole
delta spec tree by itself, so parts of it will read as instructing you to do what this packet
forbids — write requirement statements, write files under `specs/`, persist to Engram, return the
phase summary block. Those are superseded here on purpose. Do not correct yourself toward them;
note the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row
per unique citation.

| citation | claim |
|---|---|
| `internal/run/attempt.go:743-747` | ErrNoMergeBase makes evaluateOverlapGate continue |

Rules:

- **One row per UNIQUE citation.** A range cited three times in the prose appears once here.
- **Group by file**, files alphabetical, line numbers ascending within each file.
- **The claim is what YOU assert that range shows** — one line, stated plainly, no hedging. Not a
  description of the file; the specific thing you are using it as evidence for.
- **This section does not count against the word budget.** Never trim analysis to make room for
  it, and never trim it to fit the budget.
- **The manifest is a worklist, not a certificate.** Listing a citation here asserts nothing about
  its correctness — the synthesizer opens and checks every single one. Writing a row does not
  spare you from getting the citation right; it makes getting it wrong cheaper to catch.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **Every requirement named in `## Assumed requirements` has at least one scenario**, and every scenario is in GIVEN / WHEN / THEN form.
- [ ] **Every scenario's THEN names an observable outcome**, and the coverage table cites the seam it is observable through or marks it "new seam required".
- [ ] **`spec-lens-b.md` exists, is under 1000 words, and carries `## Assumed requirements`, `## Coverage`, and `## Untestable Assertions`.**
- [ ] **The work is committed with a conventional commit and no AI attribution** (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The proposal does not determine what the system should do in a case, so the scenario would assert an outcome nobody chose.
- Every scenario for a requirement would be untestable, meaning the requirement as proposed is unobservable.
- Satisfying one instruction in this packet would require violating another.

## Context

**`openspec/changes/conflict-triage-fixture/proposal.md` is committed in this worktree and is the
accepted proposal.** Read it first. `proposal-synthesis-notes.md` beside it records twelve
citations the propose synthesizer opened and dropped — do NOT resurrect any of them. Two examples,
so the shape is clear: `internal/reconcile/reconcile.go:163-165` is `NewService` assigning the
default evaluator, NOT the triage-JSON slot (that is `Candidate.Output` at `:105`);
`internal/serve/handlers.go:95-109` is `/api/state` pagination constants, NOT the read-only GET
reconcile routes.

**Four product questions are DECIDED. Do not re-open, re-offer, or widen them:**

1. Verify-budget unit is **wall clock plus the concrete command** (e.g. "~4 min:
   `./lucind-checks.sh` on the combined tree"). Token/pricing and test-weight units were rejected.
2. Risk is **three bands** — low / medium / high — with a business conflict pinned to `high` and
   the agent unable to lower it. A continuous 0-100 score was rejected.
3. The fixture generator lives at **`internal/conflicttriage/fixture/`**. `test/fixture/` and a
   public CLI subcommand were rejected.
4. The A/B win criterion is **correct classification of the three hunks** — business separated
   from the two mechanical controls, arbitrariness declared where it belongs. Grading the prepared
   resolution, and timing a human to thirty seconds, were both rejected.

**Two questions MUST stay open. Answering either by guess is a lane failure:**

- the exact non-decreasing risk formula and its thresholds, including mixed business+mechanical
  hunks;
- which executor/model runs production triage.

**One real ambiguity the proposal left unresolved — the orchestrator found it in review, and it
belongs to design, not specs.** The proposal's triage requirement says the agent must "leave a
prepared SHA (`internal/reconcile/reconcile.go:107`)", while its Approach says "a human resolves
out of band and registers the SHA" via `reconcile resolve --candidate --sha`. Both cannot hold. If
the agent writes `CandidateSHA` into the ledger itself it bypasses the human registration step
that makes the whole resolution safe. Design must decide explicitly who writes that field and say
why; specs must not assume an answer.

**Ground truth — cite it, do not re-derive it:**

- `evaluateOverlapGate` (`internal/run/attempt.go:687`) classifies via `overlap.Classify`
  (`internal/overlap/overlap.go:623`); `ClassRequired` at `:658-659`; thresholds at `:93-98`.
- Evidence for `ClassRequired` is inserted inside `CreateRequest`
  (`internal/reconcile/reconcile.go:266`), never on the warning branch.
- Without a registered shared `base_sha`, `ErrNoMergeBase` makes the gate `continue`
  (`internal/run/attempt.go:743-747`) and `ClassRequired` is unreachable.
- Triage does NOT fail closed, unlike `internal/resolve` (`internal/resolve/candidate.go:26`,
  prompt at `:303-312`). Keep the two disciplines separate.
- Ledger today: 36 `integration_attempts`, 0 `overlap_evidence`, 0 `reconciliation_requests`,
  0 `reconciliation_candidates`.

**Out of scope, and including any of it is wrong:** reconcile POST on the web surface (read-only
GET stays), overlap thresholds, and production dispatch paths.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
