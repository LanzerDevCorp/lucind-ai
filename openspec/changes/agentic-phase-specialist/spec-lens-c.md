# Spec Lens C — Live-Spec Conflicts & Migration: Agentic Phase Specialist

## Assumed requirements

This lens audits live specifications against the accepted proposal for `agentic-phase-specialist` (`openspec/changes/agentic-phase-specialist/proposal.md:3-4`), which defines four core requirements: `Specialist Phase Acceptance and Authority Carve-Out` (`proposal.md:67-85`), `Structured Phase Verdict Reporting` (`proposal.md:86-99`), `SDD Phase-Gated Verification Check Execution` (`proposal.md:100-118`), and `Specialist-Owned Synthesis Arbitration` (`proposal.md:119-132`). We evaluate collisions and migration impacts across live specs in `openspec/specs/phase-specialist-dispatch/spec.md:1`, `openspec/specs/acceptance-verifier/spec.md:1`, and `openspec/specs/sdd-planning-fan-out/spec.md:1`, treating `phase-verdict-reporting` as an ADDED capability without existing live spec artifacts (`proposal.md:28-29`).

## Live Spec Inventory

| Capability | Live spec (file:line) | Requirements | Scenarios | Touched by this change |
|---|---|---|---|---|
| phase-specialist-dispatch | `openspec/specs/phase-specialist-dispatch/spec.md:1` | 1 | 3 | Yes (`Specialist sequencing and canonical artifact generation`) |
| acceptance-verifier | `openspec/specs/acceptance-verifier/spec.md:1` | 9 | 21 | Yes (`Fail-Closed Mechanical Criteria`) |
| sdd-planning-fan-out | `openspec/specs/sdd-planning-fan-out/spec.md:1` | 5 | 13 | Yes (`Two-Wave Planning Fan-Out Protocol`) |

## Conflicts

1. **Phase Specialist Execution Model vs. Agentic Decision-Maker (`phase-specialist-dispatch`)**:
   - `openspec/specs/phase-specialist-dispatch/spec.md:9-11` defines specialist dispatch as a deterministic sequence ingesting `sdd-status` and triggering child lanes.
   - `proposal.md:32,67-85` reconfigures `sdd-*` subagents into autonomous decision-bearing Specialists who independently judge Acceptance and author synthesis, while the Go adapter (`cmd/lucind-ai/cli.go:2517-2553`, `internal/phasespec/phasespec.go:338-350`) is retained strictly as the mechanical status/dispatch CLI tool.
   - *Resolution / Guidance*: The live requirement `Specialist sequencing and canonical artifact generation` must be updated to clarify that the Specialist is the agentic decision-maker, while the Go adapter provides eligibility inspection.

2. **Unconditional Verification Checks vs. SDD Phase Gating (`acceptance-verifier`)**:
   - `openspec/specs/acceptance-verifier/spec.md:30-33` mandates that acceptance MUST reject a candidate on a "failed required check", which currently executes `lucind-checks.sh` unconditionally across all phases (`internal/accept/accept.go:120-137`, `internal/integrate/integrate.go:159-200`).
   - `proposal.md:100-118` requires gating `lucind-checks.sh` on `LaneMetadata.SDDPhase` (`internal/ledger/lanes_meta.go:20-47`), skipping the Go test suite for non-apply planning phases while enforcing fail-closed schema, done criteria, and `allowed_paths` scope validation (`internal/accept/accept.go:97-98,214-261`).
   - *Resolution / Guidance*: Live requirement `Fail-Closed Mechanical Criteria` must be modified to define "required check" as conditional on `SDDPhase == "apply"`, empty/unlabeled metadata, or explicit check exceptions.

3. **Orchestrator Synthesis Review vs. Specialist-Owned Arbitration (`sdd-planning-fan-out`)**:
   - `openspec/specs/sdd-planning-fan-out/spec.md:9-12` mandates that the Orchestrator executes planning phases and wave 2 synthesis, and `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` specifies that the Orchestrator reads synthesis notes and arbitrates contradictions.
   - `proposal.md:119-132` shifts synthesis review, contradiction arbitration, and lens Acceptance to the phase Specialist; the Orchestrator receives only the compressed Phase Verdict (`CONTEXT.md:107-109`, `docs/sdd-phase-specialist.md:21-30`).
   - *Resolution / Guidance*: Live requirement `Two-Wave Planning Fan-Out Protocol` must be modified to assign synthesis arbitration and lane Acceptance to the Specialist.

4. **Hard Rule Acceptance Authority vs. Strict Promotion Prohibition**:
   - `plugin/claude-code/skills/lucind-ai/SKILL.md:19` states "Agents own Lanes, not... Acceptance, or Promotion".
   - `proposal.md:14,67-85` carves out Acceptance authority for named `sdd-*` Specialists for their assigned phase's lanes (`CONTEXT.md:51-53,103-106`, `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8,16-18`). Promotion MUST remain human-confirmed (`CONTEXT.md:91-93`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`, `openspec/specs/acceptance-verifier/spec.md:124-134`).

## MODIFIED Full Blocks

### Requirement: Specialist sequencing and canonical artifact generation

**Source**: `openspec/specs/phase-specialist-dispatch/spec.md:9` — 3 scenarios

The phase specialist MUST ingest `gentle-ai sdd-status` JSON, dispatch child lanes through lucind-ai, MUST NOT start synthesis until all required planning lenses are accepted and merged, and MUST land canonical phase artifacts at `openspec/changes/<change>/` under the canonical per-phase filename (`proposal.md`, `spec.md`, `design.md`, `tasks.md`, `apply.md`, `verify.md`, `remediate.md`, `archive.md`).

#### Scenario: Fan-out lenses merged before synthesis dispatch

- GIVEN an active propose phase with all required lenses (`lens-a`, `lens-b`, `lens-c`) accepted and merged
- WHEN the phase specialist checks `gentle-ai sdd-status`
- THEN the specialist MUST dispatch the propose synthesis lane for `openspec/changes/<change>/proposal.md`.

#### Scenario: Unchanged phase state generates no dispatches

- GIVEN `gentle-ai sdd-status` reporting phase complete with canonical artifact present
- WHEN the phase specialist inspects lifecycle state
- THEN the specialist MUST complete without dispatching redundant lanes.

#### Scenario: Synthesis blocked while lenses unmerged

- GIVEN an active propose phase with an unmerged lens
- WHEN the phase specialist evaluates next action
- THEN the specialist MUST NOT dispatch synthesis and MUST wait.

### Requirement: Fail-Closed Mechanical Criteria

**Source**: `openspec/specs/acceptance-verifier/spec.md:30` — 6 scenarios

The verifier MUST reject a missing or invalid result schema, packet or candidate-commit mismatch, fired hard stop, unmet done criterion, undeclared or out-of-scope change, or failed required check. For versioned contracts it MUST also reject any missing, extra, duplicate, reordered, or altered authored criterion or hard stop; mode or commit disagreement; and any path or change-classification mismatch against the canonical frozen candidate change set. A rejected attempt MUST NOT create or reuse a receipt. The status-deciding step MUST explicitly evaluate every declared hard stop's `fired` value after schema validation and demote the lane to blocked when any is true, regardless of the envelope's claimed top-level status.

#### Scenario: Reject invalid result evidence
- GIVEN result evidence is missing, schema-invalid, mismatched, has a fired hard stop, or has an unmet done criterion
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Reject scope or check failure
- GIVEN the candidate contains an undeclared or disallowed change, or a required check fails
- WHEN acceptance is attempted
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Reject authored-result mismatch
- GIVEN a versioned result omits or changes an authored criterion or stop
- WHEN acceptance is attempted
- THEN acceptance MUST fail even when every reported entry is green

#### Scenario: Reject commit or path-class mismatch
- GIVEN a write result names another commit or misclassifies a deletion or rename endpoint
- WHEN acceptance compares it with the frozen candidate
- THEN acceptance MUST fail and no receipt exists

#### Scenario: Preserve explicit legacy behavior
- GIVEN an admitted manual candidate is explicitly marked legacy
- WHEN acceptance runs
- THEN universal schema, scope, commit-state, and check rules MUST apply without inventing versioned declaration correspondence

#### Scenario: Fired hard stop demotes regardless of claimed status

- GIVEN a schema-valid result envelope where at least one declared hard stop's `fired` value is true
- WHEN the verifier decides status
- THEN the lane MUST be demoted to blocked even when `envelope.Status` claims `done`

### Requirement: Two-Wave Planning Fan-Out Protocol

**Source**: `openspec/specs/sdd-planning-fan-out/spec.md:9` — 2 scenarios

The orchestrator MUST execute planning phases (explore, propose, design, specs, tasks) as a two-wave fan-out on generic `lucind-ai run --packet` (`cmd/lucind-ai/cli.go:121-149`; `openspec/changes/sdd-fan-out-lens/proposal.md:18,74`; `openspec/changes/sdd-fan-out-lens/explore.md:9`). Wave 1 MUST dispatch three parallel `agy` lens lanes to mutually disjoint draft paths. Wave 2 MUST dispatch one sequential `cursor-agent` synthesis lane branched from the integrated tree, producing the canonical artifact and synthesis notes (`plugin/claude-code/skills/lucind-ai/SKILL.md:153-176,184-186`). Sidecar `apply-dag.yaml` / `lucind-ai split` MUST NOT be required (`explore.md:38-40`).

#### Scenario: Planning phase dual-wave dispatch

- GIVEN an SDD change in a planning phase
- WHEN wave 1 completes and integrates all three lens drafts
- THEN the orchestrator MUST dispatch wave 2 to produce the canonical artifact and synthesis notes

#### Scenario: Wave-2 synthesizer dispatched before wave 1 integrates

- GIVEN wave-1 lanes finished in isolated worktrees but have not integrated into primary `HEAD` (`internal/run/integrate.go:31-81`; `explore.md:42`)
- WHEN wave 2 dispatches the synthesizer from that unintegrated `HEAD`
- THEN the synthesizer worktree MUST NOT contain the wave-1 draft files (`SKILL.md:184-186`)

## Removals and Renames

| Requirement | Removed or renamed | Reason | Consumers (file:line) | Migration |
|---|---|---|---|---|
| None | None | No requirements are removed or renamed by this change; existing live capabilities are modified in place. | None | None |

## Open Questions

- [ ] What CLI tool bridge or protocol should allow `sdd-*` Specialists to trigger `lucind-ai run` without Orchestrator mediation in future changes? (`proposal.md:191`)
- [ ] Should `lucind-ai accept` support an explicit `--force-checks` CLI flag, or is packet-level exception frontmatter sufficient for check bypass overrides? (`proposal.md:192`)
- [ ] Should the Phase Verdict schema be codified as a typed JSON envelope under `internal/result/` or structured markdown returned to the Orchestrator? (`proposal.md:193`)
- [ ] Does `sdd-propose` need explicit skill documentation stating that packet word ceilings take precedence over general skill length guidelines during fan-out? (`proposal.md:194`)
- [ ] How will delta spec synthesis reconcile the boundary where the Specialist holds Acceptance decision authority while the Orchestrator mechanically executes `lucind-ai run` and `lucind-ai accept`?

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:51-53` | Acceptance glossary definition permitting acceptance without additional human confirmation |
| `CONTEXT.md:91-93` | Promotion glossary definition requiring human-confirmed integration into Integration Target |
| `CONTEXT.md:103-106` | Specialist glossary definition as a phase-scoped agent owning execution strategy and lane acceptance |
| `CONTEXT.md:107-109` | Phase Verdict glossary definition covering outcome, artifact path, and unresolved divergence |
| `cmd/lucind-ai/cli.go:2517-2553` | `phaseDispatch` CLI command implementation in Go adapter |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | ADR decision establishing phase-scoped Specialist authority and scoped checks |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-18` | ADR consequences for Hard Rule carve-out and fan-out arbitration |
| `docs/sdd-phase-specialist.md:7-9` | Design note defining the problem of Orchestrator context window inflation |
| `docs/sdd-phase-specialist.md:21-30` | Resolved decisions on Specialist runtime substrate, Acceptance authority, and Phase Verdict |
| `internal/accept/accept.go:84-96` | `accept.go` loading lane metadata within authoring evidence branch |
| `internal/accept/accept.go:97-98` | `accept.go` validating result schema and path scope |
| `internal/accept/accept.go:120-137` | `accept.go` evaluating check policy and executing mechanical verification checks `v.check` |
| `internal/accept/accept.go:214-261` | `validateResultAndScope` fail-closed scope enforcement |
| `internal/integrate/integrate.go:159-200` | `integrate.Check` running `lucind-checks.sh` verification suite |
| `internal/ledger/lanes_meta.go:20-47` | `LaneMetadata` struct definition carrying `SDDPhase` field |
| `internal/packet/packet_test.go:924-941` | Test verifying glossary projection in `references/core/domain.md` matches `CONTEXT.md` |
| `internal/packet/packet_test.go:943-967` | `TestSkillTreesByteIdentical` ensuring Claude and OpenCode skill trees match |
| `internal/phasespec/phasespec.go:308-333` | `CLIStatusQuerier` querying `gentle-ai sdd-status` |
| `internal/phasespec/phasespec.go:338-350` | `Adapter` struct coordinating status and dispatch |
| `internal/run/attempt.go:431-435` | `executeAttempt` defaulting `checkFunc` to `integrate.Check` |
| `openspec/changes/agentic-phase-specialist/proposal.md:3-4` | Chosen candidate establishing phase-scoped agentic Specialist |
| `openspec/changes/agentic-phase-specialist/proposal.md:28-35` | Capabilities breakdown in accepted proposal |
| `openspec/changes/agentic-phase-specialist/proposal.md:67-85` | Proposed requirement: Specialist Phase Acceptance and Authority Carve-Out |
| `openspec/changes/agentic-phase-specialist/proposal.md:86-99` | Proposed requirement: Structured Phase Verdict Reporting |
| `openspec/changes/agentic-phase-specialist/proposal.md:100-118` | Proposed requirement: SDD Phase-Gated Verification Check Execution |
| `openspec/changes/agentic-phase-specialist/proposal.md:119-132` | Proposed requirement: Specialist-Owned Synthesis Arbitration |
| `openspec/changes/agentic-phase-specialist/proposal.md:191` | Proposal open question on Specialist direct execution bridge |
| `openspec/changes/agentic-phase-specialist/proposal.md:192` | Proposal open question on `--force-checks` CLI flag vs frontmatter metadata |
| `openspec/changes/agentic-phase-specialist/proposal.md:193` | Proposal open question on Phase Verdict JSON schema vs markdown |
| `openspec/changes/agentic-phase-specialist/proposal.md:194` | Proposal open question on fan-out word ceilings vs skill documentation |
| `openspec/specs/acceptance-verifier/spec.md:1` | Acceptance Verifier live specification header |
| `openspec/specs/acceptance-verifier/spec.md:9-29` | Live requirement: Exact Acceptance Binding |
| `openspec/specs/acceptance-verifier/spec.md:30-64` | Live requirement: Fail-Closed Mechanical Criteria |
| `openspec/specs/acceptance-verifier/spec.md:65-74` | Live requirement: Frozen Candidate Verification |
| `openspec/specs/acceptance-verifier/spec.md:75-90` | Live requirement: Owned Isolation and Cleanup |
| `openspec/specs/acceptance-verifier/spec.md:91-107` | Live requirement: Durable Receipt and Exact Cache Reuse |
| `openspec/specs/acceptance-verifier/spec.md:108-123` | Live requirement: Receipt-Gated CLI Success |
| `openspec/specs/acceptance-verifier/spec.md:124-134` | Live requirement: No Promotion Authority |
| `openspec/specs/acceptance-verifier/spec.md:135-145` | Live requirement: Mechanical Evidence Is Not Semantic Approval |
| `openspec/specs/acceptance-verifier/spec.md:146-167` | Live requirement: Fail-Closed Mechanical Criteria Validation |
| `openspec/specs/phase-specialist-dispatch/spec.md:1` | Phase Specialist Dispatch live specification header |
| `openspec/specs/phase-specialist-dispatch/spec.md:9-29` | Live requirement: Specialist sequencing and canonical artifact generation |
| `openspec/specs/sdd-planning-fan-out/spec.md:1` | SDD Planning Fan-Out live specification header |
| `openspec/specs/sdd-planning-fan-out/spec.md:9-24` | Live requirement: Two-Wave Planning Fan-Out Protocol |
| `openspec/specs/sdd-planning-fan-out/spec.md:25-40` | Live requirement: Asymmetric Precedence and Compression Ceiling |
| `openspec/specs/sdd-planning-fan-out/spec.md:41-56` | Live requirement: Frontmatter Admission and CLI Documentation |
| `openspec/specs/sdd-planning-fan-out/spec.md:57-92` | Live requirement: Planning Fan-Out Template Assets |
| `openspec/specs/sdd-planning-fan-out/spec.md:93-107` | Live requirement: Skill and Asset Contract Tests |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19` | Hard Rule declaring agents own lanes rather than Acceptance or Promotion |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:16-30` | Canonical 10-step Acceptance protocol and checklist |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Acceptance subagent delegation model |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43` | Dual-Judge requirement for Tier A Changes |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | Promotion gate and human confirmation requirements |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:7-16` | Three-lens planning fan-out topology matrix |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25` | Dispatch sequencing requiring all lens receipts before synthesis |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Live rule assigning synthesis note review and contradiction arbitration to Orchestrator |
