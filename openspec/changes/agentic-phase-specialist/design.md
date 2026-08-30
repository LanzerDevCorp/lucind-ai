# Design: Agentic Phase Specialist

## Technical Approach

Map the proposal and four capabilities onto existing seams. No new Go package, schema version, or result-envelope type.

1. **`phase-verdict-reporting`**: the Specialist returns a structured markdown Phase Verdict (`outcome`, `canonical_artifact_path`, `unresolved_divergence`) in chat; raw envelopes stay with the Specialist (`CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30`).
2. **`phase-specialist-dispatch`**: the `sdd-*` subagent owns sequencing and packet authoring; `internal/phasespec.Adapter` (`internal/phasespec/phasespec.go:338-350`) and `phaseDispatch` (`cmd/lucind-ai/cli.go:2517-2649`) stay deterministic tools. Synthesis waits for every required lens receipt (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25`, `openspec/specs/phase-specialist-dispatch/spec.md:9-12`).
3. **`acceptance-verifier`**: load `LaneMetadata` unconditionally; skip `lucind-checks.sh` unless `SDDPhase == "apply"`, `sdd_phase` is empty/missing, or an explicit exception is set. Schema, hard stops, done criteria, and `allowed_paths` stay unconditional (`internal/accept/accept.go:84-137,214-261`).
4. **`sdd-planning-fan-out`**: synthesis-note review and contradiction arbitration move to the Specialist (`fan-out.md:47-48`).

## Architecture Decisions

### Decision 1 — Phase Verdict is structured markdown

**Choice**: Specialist chat response with `Outcome`, `Canonical Artifact`, `Unresolved Divergence`. Not a JSON schema under `internal/result/`.
**Alternatives considered**: JSON in `internal/result/` (rejected: that package models worktree `.lucind/result.json` — `internal/result/result.go:1-12` — and the Specialist does not invoke the Go binary for the verdict). Free-form chat (rejected: the Orchestrator needs a parseable outcome).
**Rationale**: The proposal left JSON vs markdown open. No `internal/result/`-style package exists for conversation-level reports. Markdown matches `CONTEXT.md:107-109` without a schema bump.
**Terminal consumer**: Orchestrator conversation (`fan-out.md:47-48`).

### Decision 2 — Gate checks at callers, not inside `Check()`

**Choice**: Lift `GetLaneMetadata` out of the `AuthoringEvidenceVersion` branch (`accept.go:84-96`). Gate `CheckPolicySnapshot` + `v.check` (`accept.go:120-137`) and `checkFunc` (`internal/run/attempt.go:431-448`) to run when `SDDPhase == "apply"`, `sdd_phase` is empty/missing, or an explicit packet exception is set. `integrate.Check` stays ungated (`internal/integrate/integrate.go:159-200`).

Empty/missing `sdd_phase` is a **conservative extension** of ADR-0002 / docs decision 9, which named only `apply` + explicit exception (`docs/sdd-phase-specialist.md:29`, `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:7`). Unlabeled/legacy lanes keep running the full suite.

For `ExecuteAttempt`, resolve `SDDPhase` from combined lanes’ `LaneMetadata` via `Deps.Ledger` (written at dispatch, `internal/run/run.go:382`). Skip `checkFunc` only when every combined lane is a declared non-apply phase; mixed, missing, or exception → run checks.

**Alternatives considered**: Gate inside `Check()` (rejected: the primitive must not read the ledger). Gate on `.go` in the diff (rejected: fragile; `SDDPhase` is declared at dispatch — `internal/ledger/lanes_meta.go:20-47`). Unconditional suite (rejected: planning lanes write `openspec/changes/**` — ADR:11-12).
**Rationale**: Fail-closed on unlabeled lanes; skip only declared planning phases; scope validation stays (`accept.go:97-98,214-261`).
**Call sites**: `accept.go:84-137`, `attempt.go:431-448`. **Schema**: none; stays v10 (`internal/ledger/schema.go:425-445,584-592`).

### Decision 3 — Hard Rule carve-out; Promotion stays human

**Choice**: Both skill trees (`plugin/claude-code/skills/lucind-ai/SKILL.md:19` and the OpenCode mirror): named `sdd-*` Specialists may Accept their own phase’s Lanes; Promotion remains forbidden to all Agents. Upgrade `acceptance-promotion.md:31-36` from evidence-only subagent to decision-bearing Specialist Acceptance. Dual-Judge stays for Tier A (`:38-43`).
**Alternatives considered**: Authorize all agents (rejected: workers lack cross-lane context). Delegate Promotion (rejected: `CONTEXT.md:91-93`, `acceptance-promotion.md:44-50`). Edit only Claude Code (rejected: `TestSkillTreesByteIdentical` — `internal/packet/packet_test.go:943-967`).
**Rationale**: ADR:16-18.

### Decision 4 — Adapter coexistence; no Bash/Agent tools this Change

**Choice**: Keep `Adapter`, `CLIStatusQuerier` (`phasespec.go:308-333`), and `phaseDispatch`. The Specialist authors packets and owns the Acceptance judgment; the Orchestrator dispatches `lucind-ai run` (and the Specialist-directed `lucind-ai accept`) until a later Change adds a tool bridge.
**Alternatives considered**: Delete the Adapter (rejected: the CLI remains useful). Grant Bash/Agent now (rejected: out of scope — `proposal.md:18-19`).
**Rationale**: ADR:5-8.

### Decision 5 — Specialist arbitrates; one bounded correction

**Choice**: The Specialist reads synthesis notes, arbitrates, and verifies citations (`fan-out.md:47-48`). Persistent contradiction → `needs-revision` and exactly one bounded correction, not a re-fan-out (`docs/sdd-phase-specialist.md:21-30`).
**Alternatives considered**: Orchestrator reads notes (rejected: context exhaustion — `:7-9`). Full re-fan-out (rejected: churn).
**Rationale**: Encapsulate phase deliberation; Dual-Judge still applies for Tier A.

## Flow and Invariants

```
Orchestrator ──triggers──→ Specialist (sdd-*)
                               │ authors lens packets (disjoint allowed_paths, sdd_phase)
                               ▼
                    Orchestrator: lucind-ai run (lenses)
                               │ Specialist reviews, directs lucind-ai accept
                               ▼
                    Orchestrator: lucind-ai run (synthesis) after all lens receipts
                               │ Specialist arbitrates notes, directs accept
                               ▼
                    Phase Verdict ──→ Orchestrator (advance or one bounded fix)
                               │
                    Human Promotion ──→ Integration Target
```

Invariants: lens scopes pairwise disjoint; synthesis blocked until all required lens IDs are accepted (`fan-out.md:21-25`); non-apply skips `lucind-checks.sh` but not schema, hard stops, or `allowed_paths`; Agents/Specialists cannot Promote; a Lane writes nothing outside the repository (`fan-out.md:43`).

## Interfaces / Contracts

Phase Verdict (markdown in the Specialist completion; not `internal/result/` JSON):

```
Outcome: accepted | needs-revision
Canonical Artifact: openspec/changes/<id>/<phase>.md
Unresolved Divergence: <text or empty>
```

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `plugin/*/skills/lucind-ai/SKILL.md` | Modify | Hard Rule carve-out at line 19 | `sdd-*` runtimes; `TestSkillTreesByteIdentical` (`packet_test.go:943-967`) |
| `plugin/*/skills/lucind-ai/references/strategies/fan-out.md` | Modify | Lines 47–48: Specialist reads synthesis notes | Specialist conversation |
| `plugin/*/skills/lucind-ai/references/contracts/acceptance-promotion.md` | Modify | Lines 31–36: decision-bearing Specialist Acceptance | Specialist checklist (`:18-30`); `runAccept` (`cli.go:658-715`) |
| `internal/accept/accept.go` | Modify | Unconditional metadata load; gate `v.check` | `lucind-ai accept` |
| `internal/accept/accept_test.go` | Modify | Skip vs run for apply/empty/exception/missing; fixture `newVerifierFixture` (`:26-67`) | `go test ./internal/accept` |
| `internal/run/attempt.go` | Modify | Gate `checkFunc` in CHECKING | `ExecuteAttempt` (`:217-328`) |
| `internal/run/attempt_test.go` | Modify | Spy `checkCalls` via `attemptSpies` (`:24-44,83-92`) | `go test ./internal/run` |

**Out-of-repository (not Lane-writable):** `~/.claude/skills/sdd-*/SKILL.md` must stop doing the phase’s work and instead drive fan-out+synthesis and Acceptance. Lanes cannot write outside the repository (`fan-out.md:43`). This Change documents the required text; a human applies it outside any Lane after in-repo docs land.

No DDL. Reuse `LaneMetadata.SDDPhase`. `domain.md` stays in lockstep with `CONTEXT.md` (`packet_test.go:924-941`, `TestSkillAssetContract`).

## Testing Strategy and Test Seams

| Layer | What | Seam |
|---|---|---|
| Unit (`internal/accept`) | Skip declared non-apply; run for `"apply"`, `""`, missing metadata, exception; failing checks reject apply/unlabeled; scope still fails closed | `Verifier.check` (`accept.go:49,55`), `newVerifierFixture` (`accept_test.go:26-67`), `UpdateLaneMetadata` (`lanes_meta.go:49-60`) |
| Unit (`internal/run`) | Same gate in CHECKING; lease renewal and status transitions unchanged | `Deps.RunChecks` (`run.go:208`), `attemptSpies.checkFunc` (`attempt_test.go:41,83-92`), `Deps.Ledger` (`run.go:165`) |
| Regression (`internal/integrate`) | `Check` remains ungated | `integrate.Check` (`integrate.go:159-200`), `TestCheck*` (`integrate_test.go:471`) |
| Regression (`internal/packet`) | Byte-identical trees; glossary projection | `TestSkillTreesByteIdentical` (`:943-967`), `TestSkillAssetContract` (`:924-941`) |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no new classification; gating uses `SDDPhase`, not path heuristics | Existing scope-only treatment (`accept_test.go:127-140`) | None |
| Git repository selection | N/A: `canonicalRoot` (`accept.go:149-158`) unchanged | Isolated detached worktrees (`:406-428`) | None (`accept_test.go:297-318`) |
| Commit state | N/A: verifier ignores the primary index | Frozen detached candidate (`accept_test.go:142-166`) | None |
| Push state | N/A: no ref mutation (`accept.go:1-3`) | Receipts only | None (`accept_test.go:80-100`) |
| PR commands | N/A: no PR automation (`CONTEXT.md:23-26`) | Untouched | None |

## Rollback and Additivity

**Choice**: `git revert` of the code and skill-doc commits.
**Alternatives considered**: DDL down-scripts (no schema change); feature flags (unnecessary).
**Rationale**: Schema stays v10. `AuthoringEvidenceVersion` stays `"lane-authoring-evidence/v1"`; `Contract` stays `json.RawMessage` (`internal/ledger/authoring.go:14,26`). `BindingVersion` stays `"binding:v2"` for versioned evidence (`accept.go:364`). Frozen rows decode unchanged. Revert restores unconditional `v.check` / `checkFunc` and Orchestrator-owned synthesis review.

## Open Questions and Out of Scope

### Open Questions

- [ ] What tool or CLI bridge, in a later Change, lets `sdd-*` Specialists invoke `lucind-ai run` without Orchestrator mediation?
- [ ] Should `lucind-ai accept` expose `--force-checks`, or is packet-level exception metadata enough?

### Out of Scope

- Bash/Agent tools for `sdd-*` this Change (`proposal.md:18-19`).
- Delegating Promotion (`CONTEXT.md:91-93`).
- Changing `AuthoringEvidence` / evidence version / SQLite migrations.
- Changing `integrate.Check` internals.
- Altering `allowed_paths` or hard-stop demotion (`run.go:841-878`).
- Multi-repository coordination (`CONTEXT.md:23-26`).
- Gating on `.go` in `files_changed` (rejected, Decision 2).
- Lane writes to `~/.claude/skills/sdd-*` (human-applied; see File Changes).
