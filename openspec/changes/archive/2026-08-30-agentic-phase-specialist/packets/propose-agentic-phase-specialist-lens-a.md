---
id: propose-agentic-phase-specialist-lens-a
executor: agy
routed_by: candidate and approach lens of the three-lens propose fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/propose-lens-a.md"]
---

# Packet propose-agentic-phase-specialist-lens-a

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-agentic-phase-specialist-lens-a  ·  **Branch:** lucind/propose-agentic-phase-specialist-lens-a

## Goal

Produce `openspec/changes/agentic-phase-specialist/propose-lens-a.md`: the candidate selection, proposed technical approach, changes to system concepts, and architecture rationale for the **Agentic Phase Specialist** change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `agentic-phase-specialist` is initiating. Lens B and lens C run in parallel against the same codebase and write to different files, so no lane races another. This lens owns candidate selection and core approach; the other two explore capability specs and risk/rollback.

## Preconditions

- `openspec/changes/agentic-phase-specialist/` exists, with `explore.md` present.
- `openspec/changes/agentic-phase-specialist/propose-lens-a.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to candidate selection and approach, not to spec delta authoring or test matrices:

1. The real `gentle-ai` propose skill (delivered under `## Required skills`). It is the phase contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/agentic-phase-specialist/explore.md` — the accepted exploration. Treat its "Current-state findings" (points 1-6) as settled fact, not open questions.
3. `docs/sdd-phase-specialist.md` and `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md` — the grilled-out design decisions this change formalizes. Decisions 1-9 there are settled; do not relitigate them, only translate them into proposal shape.
4. `internal/phasespec/phasespec.go` and `cmd/lucind-ai/cli.go` (`phaseDispatch`, ~line 2517) — the existing deterministic `internal/phasespec.Adapter` this change's Specialist supersedes for agentic phases (its own status/eligibility/dispatch mechanics remain a tool the new Specialist can call).
5. `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md` — current fan-out topology and the "Orchestrator reads synthesis notes" line that moves to the Specialist.
6. `~/.claude/agents/sdd-propose.md`, `sdd-spec.md`, `sdd-design.md`, `sdd-tasks.md` — confirm none of these subagents have Bash/Agent tool access today (explore.md already found this); this is a real constraint your approach must account for (candidate must explain, at the approach level, how a Specialist without Bash access still "administers" a fan-out dispatch it cannot itself execute).

Never guess at code. Every claim about existing structure carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/propose-lens-a.md`:

```markdown
# Proposal Lens A — Candidate & Approach: Agentic Phase Specialist

## Selected Candidate & Approach

<State the chosen candidate from exploration, summarize the core approach, and explain why this approach solves the problem. Cite file:line for existing code behavior.>

## Conceptual Changes & Architecture Rationale

<Describe additions, modifications, or deprecations to system concepts, interfaces, or architectural patterns. Cite file:line for existing concepts.>

## Alternatives Considered & Rejected

<What alternative approaches were considered during candidate selection and why they were rejected.>

## Open Questions

- [ ] <unresolved technical question, or "None">
```

## Size budget

`propose-lens-a.md` MUST be under 1000 words. Approach descriptions as compact blocks, not essays. Code snippets only for a non-obvious pattern, never to restate code the reader can open.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.
- **Lens C owns**: technical risks, failure modes, rollback plan, additivity assessment, and test impact.

Do not write delta spec requirements or a rollback plan here. They belong to lenses B and C.

## Allowed paths

`openspec/changes/agentic-phase-specialist/propose-lens-a.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` propose skill and its `references/` (delivered under `## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the candidate selection, approach, and conceptual change shape of a proposal. Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

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
- **The claim is what YOU assert that range shows** — one line, stated plainly, no hedging. Not a description of the file; the specific thing you are using it as evidence for.
- **This section does not count against the word budget.** Never trim analysis to make room for it, and never trim it to fit the budget.
- **The manifest is a worklist, not a certificate.** Listing a citation here asserts nothing about its correctness — the synthesizer opens and checks every single one. Writing a row does not spare you from getting the citation right; it makes getting it wrong cheaper to catch.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

Run `./lucind-lane-check.sh` from the repo root twice. It is a deterministic script, not a judge: it reports whether these facts hold; it does not decide whether your draft is good, and it does not replace your own judgment against `## Done criteria` below.

**Before you commit**, while content is still cheap to fix:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/propose-lens-a.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Selected Candidate & Approach" \
  --require-section "Conceptual Changes & Architecture Rationale" \
  --require-section "Alternatives Considered & Rejected" --require-section "Open Questions" \
  --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**, confirm the bookkeeping:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/propose-lens-a.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every candidate and approach claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-a.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Check `git log -1 --format=%B` after committing: some executors' commit wrappers append a `Co-authored-by:` trailer the message never contained. Strip it if present.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- The technical approach cannot be grounded in existing codebase patterns or proposal context.
- Candidate selection contradicts frozen exploration conclusions without justification.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`. Problem: SDD planning phases (explore/propose/design/spec/tasks) run a 3-lens fan-out + synthesis via lucind-ai, but today the top-level Orchestrator reads every phase's synthesis notes and judges Acceptance directly, keeping full Lane evidence in its context for every phase of every Change.

**Decision (already made by the human, do not relitigate)**: insert a phase-scoped **Specialist** — the existing `sdd-*` Claude Code subagent (`sdd-explore`...`sdd-archive`), reconfigured — that administers its phase's fan-out+synthesis dispatch, reads the synthesis notes itself, and independently accepts its own phase's Lanes without additional human confirmation (already glossary-legal: `CONTEXT.md` "Acceptance ... can occur without additional human confirmation"). It reports only a compressed **Phase Verdict** (outcome, canonical artifact path, unresolved divergence) to the Orchestrator — never raw Lane evidence unless asked. **Promotion** (merging the whole Change into its Integration Target) stays human-confirmed, unchanged, at the end of the full SDD cycle.

**Known hard constraint (confirmed in explore.md, must be addressed by the approach)**: `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks` have Read/Edit/Write/Grep/Glob/mem_*/codegraph_explore tools only — **no Bash, no Agent/Task tool**. They cannot themselves invoke `lucind-ai run` to dispatch lens Lanes. The human has decided (for bootstrapping this very Change) that the Orchestrator performs the mechanical `lucind-ai run` invocation while the phase-Specialist subagent authors the lens packet content, reads the resulting synthesis output, and renders the Acceptance judgment — describe this precisely as the near-term shape of "administers the flow," and note what would need to change (tool grants) for the Specialist to dispatch unassisted in the future.

**Supersession**: this change supersedes the archived Change `2026-08-29-skill-provisioning-and-phase-specialist`'s definition of "Specialist" (deterministic, non-agentic `internal/phasespec.Adapter`, `cmd/lucind-ai/cli.go:2517`) — that adapter's status/eligibility/dispatch mechanics remain a tool the new agentic Specialist can call internally, but its "non-intercepting," tool-less design is reversed for this role.

**Also in scope** (own it if Lens A judges it architectural; otherwise leave for B/C):
- `lucind-checks.sh` gating: runs only for Lanes whose packet declares `sdd_phase: apply`, or by explicit exception. `internal/ledger.LaneMetadata.SDDPhase` (`internal/ledger/lanes_meta.go:25`) already exists and is already loaded in `internal/accept/accept.go:89`, but only inside the `AuthoringEvidenceVersion` conditional (`accept.go:84-96`) and only for target-binding validation — the gate needs to widen that load and check it before `CheckPolicySnapshot`/`v.check` (`accept.go:120-126`). The equivalent gate for `lucind-ai run` sits around `internal/run/attempt.go:433`.
- Hard Rule carve-out needed at `plugin/claude-code/skills/lucind-ai/SKILL.md:19` (and OpenCode mirror): "Agents own Lanes, not scope, priorities, Dependencies, Acceptance, or Promotion" currently forbids any Agent from owning Acceptance outright; needs an explicit phase-scoped Specialist exception.
- `references/strategies/fan-out.md:47` "The Orchestrator reads synthesis notes..." moves to the Specialist.

## Required skills

- sdd-propose

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
