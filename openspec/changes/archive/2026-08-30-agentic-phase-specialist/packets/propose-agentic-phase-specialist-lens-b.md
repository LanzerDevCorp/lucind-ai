---
id: propose-agentic-phase-specialist-lens-b
executor: agy
routed_by: capability impact and delta specs lens of the three-lens propose fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/propose-lens-b.md"]
---

# Packet propose-agentic-phase-specialist-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-agentic-phase-specialist-lens-b  ·  **Branch:** lucind/propose-agentic-phase-specialist-lens-b

## Goal

Produce `openspec/changes/agentic-phase-specialist/propose-lens-b.md`: user/capability impact, modified/added capabilities, and delta specification requirements and scenarios for the **Agentic Phase Specialist** change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `agentic-phase-specialist` is initiating. Lens A and lens C run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/agentic-phase-specialist/` exists, with `explore.md` present.
- `openspec/changes/agentic-phase-specialist/propose-lens-b.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to capability impact and delta specs:

1. The real `gentle-ai` propose skill (delivered under `## Required skills`). It is the phase contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/agentic-phase-specialist/explore.md` — the accepted exploration. Treat its findings as settled fact.
3. Existing delta/base specs at `openspec/specs/` and prior delta specs under `openspec/changes/archive/*/specs/` for structure precedent — in particular `openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md` (the capability this change supersedes) and `.../specs/acceptance-verifier/spec.md`.
4. `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md` — current Acceptance protocol and the existing "Acceptance Subagent delegation" section (evidence-only today).
5. `plugin/claude-code/skills/lucind-ai/SKILL.md` — exact current Hard Rule text at line 19 ("Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion").
6. `internal/ledger/lanes_meta.go` (`LaneMetadata.SDDPhase`) and `internal/accept/accept.go` (lines ~54-127) — the existing field and call sites relevant to gating checks by phase.

Never guess at signatures or spec shapes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/propose-lens-b.md`:

```markdown
# Proposal Lens B — Capability Impact & Specs: Agentic Phase Specialist

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

## Size budget

`propose-lens-b.md` MUST be under 1000 words. Tables and structured delta specs over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write architecture rationale or rollback mechanisms here. They belong to lenses A and C.

## Allowed paths

`openspec/changes/agentic-phase-specialist/propose-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` propose skill and its `references/` (delivered under `## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the capability impact table, and delta spec formatting conventions. Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*: that the proposal is split across three parallel lanes, which slice this lane owns, its word budget, its output path and skeleton, its out-of-scope list, and its done criteria. The skill describes one sub-agent writing a whole `proposal.md` by itself, so parts of it will read as instructing you to do what this packet forbids — write the complete document, persist it to Engram, return the phase summary block. Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in `## Open Questions` and follow this packet.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

Close this draft with a `## Citation Manifest` section: every `file:line` the draft cites, one row per unique citation.

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft -- never copy this placeholder row verbatim> |

Rules:

- **One row per UNIQUE citation.** A range cited three times in the prose appears once here.
- **Group by file**, files alphabetical, line numbers ascending within each file.
- **The claim is what YOU assert that range shows** — one line, stated plainly, no hedging.
- **This section does not count against the word budget.**
- **The manifest is a worklist, not a certificate.** The synthesizer opens and checks every single one.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice.

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/propose-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "User & Capability Impact" --require-section "Delta Specifications" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/propose-lens-b.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every capability impact and delta spec requirement carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing and strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Delta requirements conflict with existing base specs without explicit migration path.
- Capability impacts cannot be determined from packet context or code inspection.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`. Problem: SDD planning phases run a 3-lens fan-out + synthesis via lucind-ai, but today the top-level Orchestrator reads every phase's synthesis notes and judges Acceptance directly.

**Decision (already made by the human, do not relitigate)**: insert a phase-scoped **Specialist** — the existing `sdd-*` Claude Code subagent — that administers its phase's fan-out+synthesis dispatch, reads synthesis notes itself, and independently accepts its own phase's Lanes without additional human confirmation (glossary-legal: `CONTEXT.md` "Acceptance ... can occur without additional human confirmation"). It reports only a compressed **Phase Verdict** (outcome, canonical artifact path, unresolved divergence) to the Orchestrator. **Promotion** stays human-confirmed, unchanged.

**Known hard constraint**: `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks` have no Bash/Agent tool access and cannot dispatch `lucind-ai run` themselves. The human decided (for bootstrapping this Change) that the Orchestrator performs the mechanical dispatch while the Specialist authors packets, reads synthesis, and judges Acceptance.

**Capability surface to model in your table/specs** (own this fully — this is your lens):
1. **Specialist Acceptance carve-out** in the Hard Rule at `plugin/claude-code/skills/lucind-ai/SKILL.md:19` (and OpenCode mirror) — currently unconditional: "Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion."
2. **Phase Verdict** as a new returned artifact shape from Specialist to Orchestrator (already glossary-defined in `CONTEXT.md`: outcome, canonical artifact path, unresolved divergence).
3. **`sdd_phase`-gated checks**: `lucind-checks.sh` (`go build ./...` + `go test ./... -race`) should run only when the packet declares `sdd_phase: apply`, or by explicit exception — today `internal/integrate.Check()` (`internal/integrate/integrate.go:159`) and `internal/accept/accept.go` run it unconditionally. `LaneMetadata.SDDPhase` (`internal/ledger/lanes_meta.go:25`) already exists and is already loaded in `accept.go:89`, but only inside the `AuthoringEvidenceVersion` conditional (`accept.go:84-96`), not used for gating.
4. **`fan-out.md` rewrite**: `references/strategies/fan-out.md:47` ("The Orchestrator reads synthesis notes...") moves to the Specialist — model this as a modified capability of the existing fan-out strategy contract.
5. **Acceptance Subagent delegation** (`references/contracts/acceptance-promotion.md:31-36`) is promoted from evidence-gatherer to decision-maker, scoped to phase-Specialists; Dual-Judge stays required only for Tier A Changes.

**Supersession**: this change supersedes the archived Change `2026-08-29-skill-provisioning-and-phase-specialist`'s definition of "Specialist" — see its `specs/phase-specialist-dispatch/spec.md` for the capability spec you are now modifying/superseding.

## Required skills

- sdd-propose

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
