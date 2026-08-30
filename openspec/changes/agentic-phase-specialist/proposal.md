# Proposal: Agentic Phase Specialist

**Chosen candidate: Phase-Scoped Agentic Specialist.** Existing `sdd-*` subagents administer their phase's lucind-ai fan-out+synthesis, independently accept that phase's Lanes, and return only a Phase Verdict. Promotion stays human-confirmed. Supersedes the archived deterministic `internal/phasespec.Adapter` definition of Specialist; that adapter remains callable.

## Intent

SDD planning already runs 3-lens fan-out+synthesis (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:7-16`), but the Orchestrator still reads synthesis notes, arbitrates contradictions, and judges Acceptance (`fan-out.md:47-48`). That loads full Lane evidence into the top-level conversation (`docs/sdd-phase-specialist.md:7-9`). A phase-scoped Specialist owns the phase and returns only a Phase Verdict (`CONTEXT.md:103-109`).

## Scope

### In Scope
- Reconfigure `sdd-explore`…`sdd-archive` into phase Specialists (`CONTEXT.md:103-106`, `docs/sdd-phase-specialist.md:21-24`).
- Phase Verdict: outcome, artifact path, unresolved divergence (`CONTEXT.md:107-109`).
- Hard Rule carve-out for named `sdd-*` Specialist Acceptance; Promotion stays forbidden (`plugin/claude-code/skills/lucind-ai/SKILL.md:19`, `plugin/opencode/skills/lucind-ai/SKILL.md:19`).
- Gate `lucind-checks.sh` on `sdd_phase == "apply"` (or exception / unlabeled fail-safe) at `internal/accept` and `lucind-ai run`; `allowed_paths` stays unconditional.
- Dogfood this Change's remaining planning phases via lucind-ai fan-out+synthesis under the matching `sdd-*` Specialist.

### Out of Scope
- Bash/Agent tools for `sdd-*`. Orchestrator dispatches `lucind-ai run`; Specialist authors packets, reads synthesis, and judges Acceptance (`docs/sdd-phase-specialist.md:21`, `openspec/changes/agentic-phase-specialist/explore.md`).
- Delegating Promotion (`CONTEXT.md:91-93`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`).
- Changing `AuthoringEvidence` / `AuthoringEvidenceVersion` (`internal/ledger/authoring.go:14,26`) or SQLite migrations (`internal/ledger/schema.go:425-445,584-592`).
- Changing `integrate.Check` itself (`internal/integrate/integrate.go:159-200`); the gate is at callers.
- Altering `allowed_paths` enforcement or hard-stop demotion (`internal/run/run.go:841-845,856-878`, `internal/accept/accept.go:214-261`).
- Multi-repository coordination (`CONTEXT.md:23-26`).

## Capabilities

### New Capabilities
- `phase-verdict-reporting`: structured Phase Verdict from Specialist to Orchestrator.

### Modified Capabilities
- `phase-specialist-dispatch`: agentic Specialist is the decision-maker; the Go adapter stays the status/eligibility/dispatch tool (`openspec/specs/phase-specialist-dispatch/spec.md:9-11`).
- `acceptance-verifier`: phase-gate `lucind-checks.sh`; Acceptance still must not mutate refs or invoke Promotion (`openspec/specs/acceptance-verifier/spec.md:30-33,124-127`).
- `sdd-planning-fan-out`: synthesis review and contradiction arbitration move to the Specialist (`openspec/specs/sdd-planning-fan-out/spec.md:9-12`).

## Selected Candidate & Approach

Lane-lifecycle hooks: `Verifier.Verify` (`internal/accept/accept.go:120-137`) and `executeAttempt` checking (`internal/run/attempt.go:431-435`).

1. **Phase Verdict.** Report `accepted` / `needs-revision`, artifact path, and unresolved divergence (`CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30`). Raw envelopes, diffs, and synthesis notes stay with the Specialist unless asked. `needs-revision` triggers one bounded correction, never a full re-fan-out (`docs/sdd-phase-specialist.md:26`).

2. **Tool-constrained dispatch.** `sdd-*` lack Bash and Agent dispatch. Near-term the Specialist authors packets and judges Acceptance; the Orchestrator runs `lucind-ai run`. `internal/phasespec.Adapter` (`internal/phasespec/phasespec.go:338-350`, `cmd/lucind-ai/cli.go:2517-2649`) and `CLIStatusQuerier` (`phasespec.go:308-333`) remain the tool.

3. **Scoped checks.** `Check()` always runs `lucind-checks.sh` when called (`internal/integrate/integrate.go:159-176`). `accept.go` loads `LaneMetadata` only inside the authoring-evidence branch (`accept.go:84-96`), then always calls `CheckPolicySnapshot` and `v.check` (`accept.go:120-137`); `attempt.go` defaults `checkFunc` to `integrate.Check`. Load metadata unconditionally and skip those calls unless `SDDPhase == "apply"`, `sdd_phase` is empty/missing, or an explicit exception. Scope validation stays unconditional (`accept.go:97-98,214-261`).

4. **Fan-out dogfooding.** Synthesis starts only after all required lens receipts exist and branches are merged (`fan-out.md:21-25`).

## Conceptual Changes & Architecture Rationale

- **Supersede deterministic Specialist.** Archived Change `2026-08-29-skill-provisioning-and-phase-specialist` rejected tool access for a packet-author Specialist (`openspec/changes/archive/2026-08-29-skill-provisioning-and-phase-specialist/proposal.md:188`). Decision authority moves to the agentic `sdd-*` Specialist; the adapter stays callable (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8`).
- **Acceptance vs Promotion.** Acceptance may already occur without human confirmation (`CONTEXT.md:51-53`). Assign that to the Specialist for its own Lanes. Promotion stays human-confirmed (`CONTEXT.md:91-93`, `acceptance-promotion.md:44-50`).
- **Hard Rule carve-out.** `SKILL.md:19` (both trees) forbids Agents from owning Acceptance. Carve out named `sdd-*` Specialists for their own phase's Lanes only (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-17`).
- **Contracts.** `fan-out.md:47-48` moves synthesis-note review to the Specialist. `acceptance-promotion.md:31-36` upgrades the evidence-only Acceptance Subagent to decision-bearing Specialist Acceptance. Dual-Judge stays required for Tier A (`acceptance-promotion.md:38-43`, `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:18`).
- **Glossary already committed.** `CONTEXT.md:103-109`; `domain.md` projections must stay in lockstep (`internal/packet/packet_test.go:924-941,943-967`).

## User and Capability Impact

| Capability | Impact | Description | Seam |
|---|---|---|---|
| `phase-specialist-dispatch` | Modified | Agentic Specialist owns phase execution and Lane Acceptance; adapter stays the dispatch tool. | `openspec/specs/phase-specialist-dispatch/spec.md:9-11`, `cmd/lucind-ai/cli.go:2517-2649` |
| `acceptance-verifier` | Modified | Gate `lucind-checks.sh` on apply / exception / unlabeled; qualitative + scope remain. | `internal/accept/accept.go:84-96,120-137`, `internal/ledger/lanes_meta.go:20-47` |
| `sdd-planning-fan-out` | Modified | Specialist reads synthesis notes and arbitrates contradictions. | `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `openspec/specs/sdd-planning-fan-out/spec.md:9-12` |
| `phase-verdict-reporting` | Added | Outcome, artifact path, unresolved divergence. | `CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30` |

## Delta Specifications

### Requirement: Specialist Phase Acceptance and Authority Carve-Out

The Specialist MUST execute `lucind-ai accept` for Lanes in its assigned phase without human confirmation (`CONTEXT.md:51-53,103-106`). The Hard Rule (`SKILL.md:19` both trees) MUST carve out named `sdd-*` Specialists for their own phase's Lanes only. Ordinary workers MUST NOT judge Acceptance. Promotion MUST remain human-confirmed (`CONTEXT.md:91-93`, `acceptance-promotion.md:44-50`, `openspec/specs/acceptance-verifier/spec.md:124-127`). For Tier A Changes, Specialist Acceptance MUST keep Dual-Judge (`acceptance-promotion.md:38-43`).

#### Scenario: Specialist independently accepts phase planning lanes
- GIVEN a completed planning lane with valid schema, passing qualitative checks, and clean scope
- WHEN the Specialist executes the Acceptance protocol (`acceptance-promotion.md:16-30`)
- THEN it MUST issue the receipt without requesting human confirmation

#### Scenario: Specialist prohibited from executing change promotion
- GIVEN a completed Change with all phase artifacts accepted
- WHEN it is ready for its Integration Target
- THEN Promotion MUST require explicit human confirmation and MUST NOT be executed by any Specialist

#### Scenario: Non-specialist agent denied acceptance authority
- GIVEN an ordinary delegated worker completing a lane
- WHEN the lane reaches completion
- THEN the worker MUST NOT judge Acceptance or execute `lucind-ai accept`

### Requirement: Structured Phase Verdict Reporting

The Specialist MUST return `outcome` (`accepted` or `needs-revision`), `canonical_artifact_path`, and `unresolved_divergence` (`CONTEXT.md:107-109`). It MUST NOT send raw envelopes, diffs, or full synthesis notes unless asked (`docs/sdd-phase-specialist.md:21-30`). On `needs-revision`, the Orchestrator MUST dispatch exactly one bounded correction.

#### Scenario: Successful phase completion returns compressed verdict
- GIVEN a synthesized artifact accepted by the Specialist
- WHEN it reports to the Orchestrator
- THEN the verdict MUST have outcome `accepted`, the artifact path, and empty divergence, without raw lane transcripts

#### Scenario: Synthesis divergence reported in verdict
- GIVEN unresolvable contradictions between lens drafts
- WHEN the Specialist prepares the verdict
- THEN outcome MUST be `needs-revision` and `unresolved_divergence` MUST be populated

### Requirement: SDD Phase-Gated Verification Check Execution

`internal/accept` and the `lucind-ai run` attempt path MUST gate `lucind-checks.sh` on `LaneMetadata.SDDPhase` (`internal/ledger/lanes_meta.go:20-47`, `internal/accept/accept.go:84-96,120-137`, `internal/run/attempt.go:431-435`). Execute checks when `SDDPhase` is `"apply"`, `sdd_phase` is empty/missing, or an explicit exception is set; otherwise skip and validate schema, hard stops, done criteria, and `allowed_paths` (`openspec/specs/acceptance-verifier/spec.md:30-33`). `integrate.Check` stays an ungated primitive (`internal/integrate/integrate.go:159-200`).

#### Scenario: Apply phase executes full verification suite
- GIVEN a lane declaring `sdd_phase: apply`
- WHEN `internal/accept` verifies the candidate
- THEN it MUST execute `lucind-checks.sh` in the isolated worktree

#### Scenario: Planning phase skips full verification suite
- GIVEN a planning lane declaring a non-apply phase (e.g. `propose`)
- WHEN `internal/accept` verifies the candidate
- THEN it MUST skip `lucind-checks.sh` and accept on qualitative and scope criteria

#### Scenario: Unlabeled or exception path still runs checks
- GIVEN a lane with empty `sdd_phase` or an explicit check exception
- WHEN `internal/accept` verifies the candidate
- THEN it MUST execute `lucind-checks.sh`

### Requirement: Specialist-Owned Synthesis Arbitration

The Specialist MUST inspect synthesis notes, arbitrate contradictions, and verify canonical citations before accepting (`fan-out.md:47-48`, `openspec/specs/sdd-planning-fan-out/spec.md:9-12`). The Orchestrator SHALL NOT do that during normal execution.

#### Scenario: Specialist arbitrates contradictions in synthesis notes
- GIVEN synthesis notes identifying conflicting lens proposals
- WHEN the Specialist reviews the synthesis result
- THEN it MUST arbitrate and either accept or mark `needs-revision`

#### Scenario: Synthesis notes withheld from orchestrator context
- GIVEN accepted synthesis producing canonical artifacts and detailed notes
- WHEN the Specialist concludes the phase
- THEN full synthesis notes MUST remain with the Specialist

## Technical Risks & Failure Modes

| Risk | Mitigation | Seam |
|---|---|---|
| Specialist Acceptance admits defective planning artifacts | Fail-closed schema/scope (`accept.go:97-98,214-261`); hard-stop → `blocked` (`internal/run/run.go:841-845`); Dual-Judge for Tier A; Promotion stays human | `acceptance-promotion.md:38-50` |
| Check gate skips Go suite on mislabeled/unlabeled lanes | Run checks if `sdd_phase` is `"apply"`, empty, or exception; load metadata before verification | `accept.go:84-96,120-137`, `attempt.go:415-460`, `lanes_meta.go:20-47` |
| Hard Rule carve-out misread as general executor Acceptance | Carve-out named `sdd-*` Specialists, own-phase Lanes only; Promotion forbidden to all Agents | `SKILL.md:19` (both trees) |
| Claude/OpenCode skill-tree drift | `TestSkillTreesByteIdentical` and glossary projection in `TestSkillAssetContract` | `internal/packet/packet_test.go:924-967`, `SKILL.md:21` |
| Synthesis before all lenses accepted | Block until all required lens receipts exist | `fan-out.md:21-25` |
| Unbounded re-fan-out on `needs-revision` | At most one scoped correction | `docs/sdd-phase-specialist.md:21-30` |

## Rollback Plan and Additivity

`git revert` of the code and documentation commits. No DDL (`internal/ledger/schema.go:425-445,584-592`). Revert restores unconditional `v.check` / `integrate.Check` (`accept.go:120-137`, `attempt.go:415-460`) and Orchestrator-owned synthesis-note review (`fan-out.md:47-48`).

Additivity: reuse `LaneMetadata.SDDPhase` (`lanes_meta.go:20-47,49-60`). Evidence version stays `"lane-authoring-evidence/v1"`; `Contract` remains `json.RawMessage` (`authoring.go:14,26,44-75`). Empty `sdd_phase` keeps running full checks.

## Test and Validation Impact

| Layer | Required coverage | Seam |
|---|---|---|
| `internal/accept` | Skip declared non-apply; run for `"apply"`, empty, or exception; missing metadata does not skip | `accept.go:84-140`, `accept_test.go:26-67,80-100` |
| `internal/run` | Same gate in `executeAttempt`; lease renewal and checking transitions unchanged | `attempt.go:415-460`, `attempt_test.go:24-80` |
| `internal/integrate` | `Check` remains ungated | `integrate.go:159-200`, `integrate_test.go:21-50` |
| `internal/packet` | Byte-identical skill trees and glossary projection after Hard Rule / fan-out edits | `packet_test.go:924-967` |

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `plugin/*/skills/lucind-ai/SKILL.md:19` | Modified | Hard Rule carve-out for `sdd-*` Specialist Acceptance |
| `plugin/*/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Modified | Synthesis-note review moves to Specialist |
| `plugin/*/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Modified | Acceptance Subagent becomes decision-bearing for Specialists |
| `internal/accept/accept.go:84-137` | Modified | Unconditional metadata load; phase-gate `v.check` |
| `internal/run/attempt.go:431-435` | Modified | Equivalent gate on `checkFunc` |
| `~/.claude/skills/sdd-*/SKILL.md` | Modified | Drive fan-out+synthesis and Acceptance instead of doing phase work directly |

## Alternatives Considered

- **Evidence-only Acceptance delegate.** Rejected: recreates Orchestrator context cost (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:11`).
- **Unconditional `lucind-checks.sh`.** Rejected: planning lanes write `openspec/changes/**` (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:12`).
- **Deterministic Go Specialist as sole coordinator.** Rejected: cannot judge qualitative done criteria (`phasespec.go:338-350`).
- **Full phase relaunch on disagreement.** Rejected: bounded correction is cheaper (`docs/sdd-phase-specialist.md:26`).

## Dependencies

Existing `lucind-ai run` / `accept` / `phase`, `LaneMetadata.SDDPhase`, and Claude Code `sdd-*` subagents. No new schema version.

## Success Criteria

- [ ] Named `sdd-*` Specialists accept their own phase's Lanes without human confirmation; ordinary agents cannot; Promotion stays human-confirmed.
- [ ] Orchestrator receives a Phase Verdict only; synthesis notes stay with the Specialist unless asked.
- [ ] `lucind-checks.sh` runs for apply, empty/missing, or exception; declared non-apply phases skip it; `allowed_paths` still fails closed.
- [ ] Both skill trees stay byte-identical; glossary projections still match `CONTEXT.md`.
- [ ] `git revert` restores prior Hard Rule, fan-out assignment, and unconditional checks with no ledger migration.

## Open Questions

- [ ] What tool or CLI bridge, in a later Change, lets `sdd-*` Specialists invoke `lucind-ai run` without Orchestrator mediation?
- [ ] Should `lucind-ai accept` expose a named exception flag (e.g. `--force-checks`), or is packet-level exception metadata enough?
- [ ] Should the Phase Verdict be a JSON schema under `internal/result/` or a structured markdown section returned to the Orchestrator?
- [ ] Does the propose-phase skill contract need an explicit note that packet topology/budget outranks the skill's whole-document size budget in multi-lens fan-out?
