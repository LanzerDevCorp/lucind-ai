---
id: propose-agentic-phase-specialist-lens-c
executor: agy
routed_by: risks, rollback, and test impact lens of the three-lens propose fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/propose-lens-c.md"]
---

# Packet propose-agentic-phase-specialist-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/propose-agentic-phase-specialist-lens-c  ·  **Branch:** lucind/propose-agentic-phase-specialist-lens-c

## Goal

Produce `openspec/changes/agentic-phase-specialist/propose-lens-c.md`: risk assessment, rollback strategy, additivity, and test impact for the **Agentic Phase Specialist** change.

This is one of three parallel propose lenses. It is feedstock for a synthesis lane, not the final proposal document. Do not write a complete `proposal.md`.

## Why this is safe to dispatch now

The proposal phase for `agentic-phase-specialist` is initiating. Lens A and lens B run in parallel against the same codebase and write to different files, so no lane races another.

## Preconditions

- `openspec/changes/agentic-phase-specialist/` exists, with `explore.md` present.
- `openspec/changes/agentic-phase-specialist/propose-lens-c.md` does not yet exist.

## Required reading (this lens only)

Read these before writing a single line. This lens is scoped to risks, rollback, and test impact:

1. The real `gentle-ai` propose skill (delivered under `## Required skills`). It is the phase contract this draft feeds; read it rather than trusting this packet's paraphrase of it.
2. `openspec/changes/agentic-phase-specialist/explore.md` — the accepted exploration. Treat its findings as settled fact.
3. `internal/integrate/integrate.go` (`Check()`, line ~159) and `internal/accept/accept.go` (`Verifier.Verify`, lines ~54-127) and `internal/run/attempt.go` (line ~433) — the exact current unconditional check-invocation call sites this change proposes to gate by `sdd_phase`.
4. `internal/ledger/lanes_meta.go` (`LaneMetadata.SDDPhase`, line ~25) — the existing field this change reuses rather than introducing a new one.
5. Existing test suites for `internal/accept`, `internal/integrate`, and `internal/run` (`go test ./internal/accept/... ./internal/integrate/... ./internal/run/...` targets) — what regression coverage already exists around the check-invocation call sites you are proposing to touch.
6. `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md` — the Tier A Dual-Judge requirement, to assess what changes if Specialist Acceptance authority is granted for lower tiers.

Never guess at seams or failure modes. Every row in your tables carries a `file:line` citation.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/propose-lens-c.md`:

```markdown
# Proposal Lens C — Risks, Rollback & Test Impact: Agentic Phase Specialist

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|

## Rollback & Additivity

**Rollback Plan**: <exact mechanism for reversal, git revert vs schema rollback>
**Additivity**: <state explicitly whether formats, schemas, or ledgers change additively or destructively, citing file:line>

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|

## Out of Scope

<Work explicitly excluded from this proposal.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`propose-lens-c.md` MUST be under 1000 words. Tables over prose.

## Out of scope

Owned by the sibling lenses. Do NOT write these — duplicated content is discarded by the synthesizer and wastes the lane:

- **Lens A owns**: candidate selection, technical approach, and conceptual changes.
- **Lens B owns**: capability impact table, delta specification requirements, and scenarios.

Rollback and test impact are yours. Conceptual design and delta spec requirements belong to lenses A and B.

## Allowed paths

`openspec/changes/agentic-phase-specialist/propose-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` propose skill and its `references/` (delivered under `## Required skills`). Read the contract as written, not as this packet paraphrases it.

Precedence between the two is **not symmetric**, so read this carefully.

The skill is authority on *what a proposal document must contain*: its required sections, the risk assessment, rollback plan, and test impact shape of a proposal. Where this packet paraphrases any of that and drifts, the skill wins and the drift belongs in `## Open Questions`.

This packet is authority on *how this phase is being executed here*. The skill describes one sub-agent writing a whole `proposal.md` by itself, so parts of it will read as instructing you to do what this packet forbids — write the complete document, persist it to Engram, return the phase summary block. Those are superseded here on purpose. Do not correct yourself toward them; note the conflict in `## Open Questions` and follow this packet.

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
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/propose-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Technical Risks & Failure Modes" --require-section "Rollback & Additivity" \
  --require-section "Test & Validation Impact" --require-section "Out of Scope" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/propose-lens-c.md
```

Paste the report's PASS/FAIL lines into `done_criteria[].evidence` in your envelope instead of narrating the same facts in prose.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every risk, test seam, and rollback claim carries `file:line` citations to real code in this worktree.**
- [ ] **`propose-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope, whether or not it fired.

- Whether a schema or format change is additive cannot be determined, making rollback a guess.
- A critical failure mode has no identifiable mitigation or test seam.
- Satisfying one instruction in this packet would require violating another.

## Context

**Change**: `agentic-phase-specialist`. Problem: SDD planning phases run a 3-lens fan-out + synthesis via lucind-ai, but today the top-level Orchestrator reads every phase's synthesis notes and judges Acceptance directly.

**Decision (already made by the human, do not relitigate)**: insert a phase-scoped **Specialist** — the existing `sdd-*` Claude Code subagent — that administers its phase's fan-out+synthesis dispatch, reads synthesis notes itself, and independently accepts its own phase's Lanes without additional human confirmation. It reports only a compressed **Phase Verdict** to the Orchestrator. **Promotion** stays human-confirmed, unchanged.

**Known hard constraint**: `sdd-propose`, `sdd-spec`, `sdd-design`, `sdd-tasks` have no Bash/Agent tool access and cannot dispatch `lucind-ai run` themselves. The human decided (for bootstrapping this Change) that the Orchestrator performs the mechanical dispatch while the Specialist authors packets, reads synthesis, and judges Acceptance.

**Risk surface to model (own this fully — this is your lens)**:
1. **Giving an LLM subagent real Acceptance authority without human confirmation** — today only a deterministic/qualitative checklist gate exists via the evidence-only "Acceptance Subagent delegation" pattern; this change lets the subagent's own judgment decide accept/reject for non-Tier-A phases. Assess what independent safety net remains (scope enforcement via `allowed_paths` is unconditional and unaffected; Dual-Judge stays for Tier A).
2. **`lucind-checks.sh` gating regression risk**: widening the `metadata.SDDPhase` load in `internal/accept/accept.go:84-96` outside its current `AuthoringEvidenceVersion` conditional, and adding an equivalent gate around `internal/run/attempt.go:433`, changes when the full Go test suite (`go test ./... -race -count=1`) runs. A bug in the gate could silently skip checks for a Lane that actually touched Go code (mislabeled `sdd_phase`), or force checks to keep running for planning phases (safe but non-functional regression).
3. **Additivity**: is `sdd_phase` in `LaneMetadata` additive (already exists per explore.md, just newly read) or does gating logic change existing accept/run behavior for lanes with no `sdd_phase` set at all (backward compatibility for non-SDD packets)?
4. **Hard Rule change**: relaxing "Agents own Lanes, not... Acceptance" is a security/authority-boundary change to a written contract two skill trees rely on (`plugin/claude-code/` and `plugin/opencode/`, byte-identical by test); assess blast radius of a carve-out being interpreted too broadly by a future agent.
5. **Test impact**: `internal/accept/accept_test.go`, `internal/integrate/integrate_test.go`, `internal/run/attempt_test.go` likely need new cases for phase-gated check skipping; `internal/packet/packet_test.go` already enforces byte-identity/containment on the glossary and skill trees touched by SKILL.md/fan-out.md edits.

## Required skills

- sdd-propose

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. In `findings`, report the counts that matter: risks identified, mitigations proposed. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
