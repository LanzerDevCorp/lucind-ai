---
id: design-agentic-phase-specialist-synthesis
executor: cursor-agent
routed_by: synthesis of three parallel design lenses into one canonical design
model: cursor-grok-4.6-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/design.md", "openspec/changes/agentic-phase-specialist/design-synthesis-notes.md"]
---

# Packet design-agentic-phase-specialist-synthesis

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/design-agentic-phase-specialist-synthesis  ·  **Branch:** lucind/design-agentic-phase-specialist-synthesis

## Goal

Read the three design lens drafts for `agentic-phase-specialist`, verify their claims against the real code, arbitrate where they disagree, and produce one canonical `openspec/changes/agentic-phase-specialist/design.md` plus a separate synthesis notes file.

You are the last judgment in this phase. Nobody re-reads the three drafts behind you — the phase-Specialist (`sdd-design`) and the Orchestrator read only your notes file.

## Why this is safe to dispatch now

All three lens lanes have reached terminal status and integrated. This worktree is branched from the integrated result, so `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` are all present here.

## Preconditions

- `design-lens-a.md`, `design-lens-b.md`, and `design-lens-c.md` all exist in this worktree.
- `openspec/changes/agentic-phase-specialist/design.md` does not yet exist.
- The proposal and specs for `agentic-phase-specialist` are present and accepted.

## What each lens owns

| Draft | Owns |
|---|---|
| `design-lens-a.md` | Technical approach; every architecture decision except rollback, with alternatives and rationale |
| `design-lens-b.md` | Flow and invariants; surface deltas; file changes |
| `design-lens-c.md` | Testing strategy and test seams; threat matrix; rollback and additivity; out of scope |

All three also emit `## Open Questions`. Merge and deduplicate them.

## Required procedure

### 1. Read all three drafts in full

### 2. Citation verification pass

Open every `file:line` citation in this worktree. Drop unsupported claims into `## Dropped Citations`.

### 3. Architecture arbitration

- **Lens A's assumed architecture is authoritative.**
- Content in lens B or lens C that does not survive lens A's architecture does not go into `design.md` — record under `## Architecture Divergence`.
- If lens A's own architecture is refuted by code you verified, do not silently substitute your own — that is a hard stop.
- **Specifically confirm lens A resolved the Phase Verdict shape** (JSON vs markdown, per the proposal's Open Question). If lens A left it open, decide it yourself using the same grounding rule lens A was given (favor markdown if no `internal/result/`-style package exists for this purpose; state your reasoning), and record that you made this call in `## Architecture Divergence`.

### 4. Compress — do not concatenate

`design.md` MUST be under 1800 words.

### 5. Coverage check

`design.md` must cover: (1) technical approach at a glance, (2) architecture decisions with choice/alternatives/rationale, (3) flow and invariants, (4) file changes with terminal consumers, (5) testing strategy and test seams, (6) threat matrix — every row Applicable or N/A, (7) rollback and additivity, (8) open questions and out of scope. Anything no draft covered goes under `## Coverage Gaps`.

## Output

### `openspec/changes/agentic-phase-specialist/design.md`

### `openspec/changes/agentic-phase-specialist/design-synthesis-notes.md`

Exactly these four sections, in this order:

```markdown
# Synthesis Notes: Agentic Phase Specialist

## Unresolved Contradictions

<"None" if there are none.>

## Coverage Gaps

<"None" if there are none.>

## Dropped Citations

<"None" if there are none.>

## Architecture Divergence

<Include whether the Phase Verdict shape decision was made by lens A or by you
during synthesis, and why. "None — all three converged" if that is the case.>
```

## Out of scope

- Do NOT modify the three lens drafts.
- Do NOT write specs, tasks, or implementation code.
- Do NOT resolve an unresolved contradiction by choosing.
- Do NOT run `go test`, `go build`, `go vet`, or `lucind-checks.sh`.

## Allowed paths

`openspec/changes/agentic-phase-specialist/design.md` and `openspec/changes/agentic-phase-specialist/design-synthesis-notes.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` design skill (delivered under `## Required skills`). It wins on required sections, choice/alternatives/rationale shape, and threat-matrix applicability rule. This packet sets the 1800-word budget (this repository's actual convention, per `openspec/changes/archive/`), the synthesis procedure, the notes file, and done criteria.

Write nothing outside this repository.

## Using the lens citation manifests

Treat the union of the three manifests as your verification worklist, never as evidence. Deduplicate across the three; batch by file; verify the claim, not just the line's existence. Record outcomes in `## Dropped Citations`.

## Commit discipline (REQUIRED — two commits, not one)

Commit `design.md` the moment it is written, before starting the notes file. Then commit the notes as a second conventional commit. No AI attribution; strip any injected `Co-authored-by:` trailer.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Right after the first commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design.md --budget 1800 --skip-result
```

**After the second commit and writing `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design-synthesis-notes.md \
  --require-section "Unresolved Contradictions" --require-section "Coverage Gaps" \
  --require-section "Dropped Citations" --require-section "Architecture Divergence"
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **`design.md` was committed as its own commit before `design-synthesis-notes.md` was started.**
- [ ] **Every `file:line` citation surviving into `design.md` was opened and confirmed**, dropped claims listed.
- [ ] **`design.md` exists, is under 1800 words, and substantively covers all eight spine items.**
- [ ] **The Phase Verdict shape is explicitly resolved in `design.md`** (either lens A's decision or your own, recorded in the notes).
- [ ] **`design-synthesis-notes.md` exists with exactly the four required sections.**
- [ ] **The work is committed with two conventional commits and no AI attribution.**

## Hard stops

- The three `## Assumed architecture` blocks are mutually irreconcilable and the proposal/specs do not choose.
- Lens A's architecture is refuted by code you verified.
- One or more lens drafts is missing.
- Covering all eight spine items honestly would require exceeding 1800 words.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist` — Phase-Scoped Agentic Specialist. Proposal accepted, specs accepted (`phase-verdict-reporting`, `phase-specialist-dispatch`, `acceptance-verifier`, `sdd-planning-fan-out`). Prior propose-phase Specialist verdict flagged: the check-gate design adds an empty/missing-`sdd_phase` fail-safe beyond the original settled decision (docs 1-9 in `docs/sdd-phase-specialist.md`/ADR-0002, which name only `apply` + explicit exception) — this is a legitimate, more-conservative extension; `design.md` should record it explicitly as an extension, not silently as if originally decided. Also flagged: `~/.claude/skills/sdd-*/SKILL.md` files are outside the repository, outside `allowed_paths`, and forbidden to Lanes by `fan-out.md`'s "nothing outside the repository is written" rule — `design.md` must state an explicit out-of-repository strategy for those edits (e.g., the Change documents required text and a human applies it outside any Lane).

## Required skills

- sdd-design

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
