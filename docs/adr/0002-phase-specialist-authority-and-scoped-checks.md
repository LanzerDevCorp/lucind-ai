# SDD Phase Specialist Authority and Scoped Checks

Status: accepted

Each SDD phase (explore, propose, design, spec, tasks, apply, verify, archive) gets a phase-scoped **Specialist** — the existing `sdd-*` subagent, reconfigured to drive that phase's fan-out-and-synthesis dispatch through lucind-ai itself rather than doing the phase's work directly. The Specialist owns **Acceptance** of its own phase's Lanes without additional human confirmation (already glossary-legal); it reports only a **Phase Verdict** to its Orchestrator, which decides to accept it or trigger one bounded correction. **Promotion** of the whole Change into its Integration Target stays human-confirmed at the end of the full SDD cycle, unchanged. This supersedes the deterministic, non-agentic `internal/phasespec` "Phase Specialist" shipped in `2026-08-29-skill-provisioning-and-phase-specialist`: that adapter's status/eligibility/dispatch mechanics remain the tool the agentic Specialist calls, but its explicit "non-intercepting," tool-less design is reversed for this role.

`lucind-checks.sh` (`go build ./...` + `go test ./... -race`) now runs only for Lanes whose packet declares `sdd_phase: apply`, or when explicitly requested by exception; every other phase accepts on the qualitative checklist (schema, done criteria, scope, diff review) alone, since planning-phase Lanes only ever write to `openspec/changes/<change>/**` and the full Go suite carries no signal for that surface. Scope enforcement (`allowed_paths`) is unaffected — it is a separate, unconditional mechanism.

## Considered Options

- **Specialist as evidence-only delegate** (extend the existing "Acceptance Subagent delegation" pattern without decision authority): keeps every Acceptance judgment with the Orchestrator, but reintroduces the context-window cost this design exists to remove.
- **Always run `lucind-checks.sh` regardless of phase**: simplest and uniform, but burns a full race-enabled Go suite on every planning-phase dispatch that never touches Go code.

## Consequences

- The Hard Rule "Agents own Lanes, not... Acceptance, or Promotion" in `plugin/claude-code/skills/lucind-ai/SKILL.md` (mirrored in `plugin/opencode/`) needs a carve-out for phase-scoped Specialist Acceptance; Promotion stays forbidden to every Agent, Specialist included.
- `references/strategies/fan-out.md`'s "Orchestrator reads synthesis notes" step moves to the Specialist; only the Phase Verdict crosses back up.
- Tier A Changes (core engine, ledger, security, promotion paths) keep the Dual-Judge requirement for Specialist Acceptance; other phases don't need it.
- Not yet done: the SKILL.md Hard Rule text, `fan-out.md` rewrite, and the actual `sdd_phase`-gated check-skip logic in `internal/integrate`/`internal/accept` are still pending implementation.
