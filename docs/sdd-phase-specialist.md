# SDD Phase Specialist — Pre-SDD Design Note

Status: **pre-SDD** — this is the grilled-out design context, not a formal openspec proposal. Formalize it through `sdd-propose` only when the SDD cycle for this Change actually starts.

## Problem

Every SDD planning phase (explore, propose, design, spec, tasks) already runs a 3-lens fan-out + synthesis through lucind-ai (`references/strategies/fan-out.md`), but today the **Orchestrator** — the top-level conversation — is the one who reads the synthesis notes, arbitrates unresolved contradictions, and judges Acceptance directly. That keeps the Orchestrator's context loaded with full Lane evidence for every phase of every Change.

Goal: insert a phase-scoped **Specialist** that owns its phase end to end and hands the Orchestrator only a compressed **Phase Verdict** — never the raw fan-out evidence.

## What already exists (verified in code, not assumed)

- **`internal/phasespec.Adapter`** (`cmd/lucind-ai/cli.go:2517` `phaseDispatch`, shipped in the archived Change `2026-08-29-skill-provisioning-and-phase-specialist`) is already called the "SDD Phase Specialist" — but it's deterministic Go with zero agency: it reads `gentle-ai sdd-status` JSON, checks lens-merge eligibility, and dispatches exactly one synthesis lane. Its own proposal explicitly rejected giving it skill selection or tools ("packet-author has no tools").
- **`references/strategies/fan-out.md`** already specifies the 3-lens + synthesis topology per phase, lens ownership, divergence sections, and word budgets — but assigns "read synthesis notes, arbitrate contradictions" to the Orchestrator directly.
- **Claude Code subagents `sdd-explore`, `sdd-propose`, `sdd-design`, `sdd-spec`, `sdd-tasks`, `sdd-apply`, `sdd-verify`, `sdd-archive`** already exist with tools/memory, but today do their phase's work directly rather than orchestrating a fan-out.
- **`acceptance-promotion.md`** already permits Acceptance without human confirmation, and already describes an "Acceptance Subagent delegation" pattern — but that pattern only gathers evidence today; the actual accept/reject judgment still sits with the Orchestrator. This contradicts the Hard Rule text "Agents own Lanes, not... Acceptance, or Promotion," which is itself an existing inconsistency independent of this design.
- **`internal/integrate.Check()`** (`internal/integrate/integrate.go:159`) runs `lucind-checks.sh` (`go build ./...` + `go test ./... -race`) unconditionally on every `lucind-ai run`/`accept`, regardless of what the batch touched — including planning-phase batches that only write `openspec/changes/<change>/**`.

## Resolved decisions

1. **Runtime substrate**: the phase-Specialist is the existing `sdd-*` Claude Code subagent, reconfigured from "does the phase's work" to "orchestrates its phase's fan-out + synthesis via lucind-ai, then accepts the result." No new execution mechanism.
2. **Acceptance authority**: the Specialist owns real Acceptance of its own phase's Lanes, without human confirmation — already glossary-legal (`Acceptance: ... can occur without additional human confirmation`). This is Acceptance, not Promotion.
3. **Promotion stays out of scope**: merging the whole Change into its Integration Target remains human-confirmed at the end of the full SDD cycle, unchanged. Promotion is defined as human-confirmed in three separate places in the current docs — nothing here touches that.
4. **Mechanism**: the Specialist runs the full Acceptance checklist itself and calls `lucind-ai accept`/`lucind-ai run` with decision authority — effectively promoting the existing "Acceptance Subagent delegation" pattern from evidence-gatherer to decision-maker, scoped to phase-Specialists. Dual-Judge stays required only for Tier A Changes (core engine, ledger, security, promotion paths); most planning phases don't need it.
5. **Phase Verdict contents** (what crosses back to the Orchestrator): outcome (accepted / needs-revision), the canonical artifact's path, and any unresolved divergence/contradiction section. Raw `result.json`, diffs, and full synthesis notes stay with the Specialist unless the Orchestrator explicitly asks.
6. **Bounded relaunch**: when the Orchestrator doesn't accept a Phase Verdict, it triggers one scoped correction (same "single correction transaction" pattern already used by the Review Execution Contract elsewhere in this repo) — never a full re-fan-out of the phase.
7. **`fan-out.md` gets rewritten** as part of this work: "Orchestrator reads synthesis notes" moves to the Specialist; the Orchestrator only sees the Phase Verdict.
8. **Naming**: this supersedes the archived `phase-specialist-dispatch` Change's definition of "Specialist" (deterministic, non-agentic) rather than coexisting under the same name. `internal/phasespec.Adapter`'s status/eligibility/dispatch mechanics remain the tool the new agentic Specialist calls internally.
9. **`lucind-checks.sh` gating**: runs only when the packet declares `sdd_phase: apply`, or when explicitly requested by exception. Every other phase accepts on the qualitative checklist alone (schema, done criteria, scope, diff review) — scope enforcement (`allowed_paths`) is a separate, unconditional mechanism and is unaffected.

## Already captured elsewhere

- Glossary: `CONTEXT.md` (+ synced projections in both `plugin/claude-code/` and `plugin/opencode/` skill trees) now define **Specialist** and **Phase Verdict**.
- Rationale record: `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md`.

## Not yet done

- Rewrite the Hard Rule "Agents own Lanes, not... Acceptance, or Promotion" in `plugin/claude-code/skills/lucind-ai/SKILL.md` (and its OpenCode mirror) to carve out phase-scoped Specialist Acceptance.
- Rewrite `references/strategies/fan-out.md` to move synthesis-note arbitration to the Specialist.
- Implement the `sdd_phase == "apply"` gate in `internal/integrate`/`internal/accept` (currently `Check()` runs unconditionally).
- Decide whether this becomes a formal SDD Change (proposal → spec → design → tasks) or a smaller direct implementation — not decided yet; this document exists so that decision doesn't require re-deriving the above from scratch.
