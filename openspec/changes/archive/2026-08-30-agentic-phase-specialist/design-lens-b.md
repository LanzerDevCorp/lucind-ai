# Design Lens B — Surface & Flow: Agentic Phase Specialist

## Assumed architecture

The change establishes phase-scoped agentic Specialists (`sdd-*` subagents) that orchestrate planning fan-out and synthesis, evaluate lane evidence, and execute Acceptance without human confirmation, returning only a structured Phase Verdict. Deterministic `internal/phasespec.Adapter` remains the internal status and dispatch tool, while Promotion to the Integration Target remains strictly human-confirmed. Mechanical checks (`lucind-checks.sh`) are gated at callers (`internal/accept`, `internal/run/attempt.go`) to execute only for `LaneMetadata.SDDPhase == "apply"`, empty/missing, or explicit exception, while `allowed_paths` enforcement remains unconditional.

## Flow and Invariants

A phase's fan-out and synthesis lifecycle moves through six hops:

```
Orchestrator (triggers phase)
      │
      ▼
Specialist (`sdd-*`) ──[Authors Lens Packets]──→ `lucind-ai run` (Lens Lanes: A, B, C)
      │                                                        │
      │ ◄──[Validates Diff/Schema & Calls `lucind-ai accept`]──┘
      ▼
Specialist (`sdd-*`) ──[Authors Synthesis Packet]─→ `lucind-ai run` (Synthesis Lane)
      │                                                          │
      │ ◄──[Arbitrates Synthesis Notes & Calls `lucind-ai accept`]┘
      ▼
Phase Verdict (`accepted` | `needs-revision`) ──→ Orchestrator (Advances Phase or Bounded Fix)
```

1. **Phase Trigger**: Orchestrator invokes `sdd-*` Specialist with Change identifier and target metadata.
   - *Invariant*: Orchestrator delegates phase execution and Acceptance, but retains Promotion authority (`plugin/claude-code/skills/lucind-ai/SKILL.md:19`, `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`).
2. **Lens Fan-Out Dispatch**: Specialist authors three disjoint lens packets (`lens-a`, `lens-b`, `lens-c`) into `.lucind/packets/` declaring `sdd_phase: <phase>` and disjoint `allowed_paths`, triggering `lucind-ai run`.
   - *Invariant*: Lens scopes are pairwise disjoint (`openspec/specs/sdd-planning-fan-out/spec.md:9-12`, `internal/accept/accept.go:214-261`); non-apply metadata skips `lucind-checks.sh` while enforcing schema, done criteria, and hard stops (`internal/accept/accept.go:84-96`, `internal/accept/accept.go:120-137`, `internal/accept/accept.go:214-261`).
3. **Specialist Lens Acceptance**: Specialist validates diffs, envelopes, and receipts, running `lucind-ai accept` without human intervention (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`).
   - *Invariant*: Synthesis dispatch blocks until all required lens receipts exist and merge (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25`, `openspec/specs/phase-specialist-dispatch/spec.md:9-11`).
4. **Synthesis Dispatch & Arbitration**: Specialist authors synthesis packet, runs `lucind-ai run`, arbitrates contradictions from synthesis notes, and runs `lucind-ai accept`.
   - *Invariant*: Synthesis notes and contradiction debates remain with Specialist (`docs/sdd-phase-specialist.md:21-30`, `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`).
5. **Phase Verdict Emission**: Specialist returns compressed Phase Verdict (`outcome`, `canonical_artifact_path`, `unresolved_divergence`) (`docs/sdd-phase-specialist.md:21-30`).
   - *Invariant*: `accepted` advances phase; `needs-revision` initiates at most one bounded correction transaction (`docs/sdd-phase-specialist.md:21-30`).
6. **Promotion Gate**: Human confirms Promotion into Integration Target (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`).
   - *Invariant*: Agents/Specialists cannot execute Promotion (`plugin/claude-code/skills/lucind-ai/SKILL.md:19`, `openspec/specs/acceptance-verifier/spec.md:124-127`).

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| Hard Rule agent acceptance & promotion | `plugin/claude-code/skills/lucind-ai/SKILL.md:19`, `plugin/opencode/skills/lucind-ai/SKILL.md:19` | Carve out Acceptance for `sdd-*` Specialists over own-phase lanes; preserve strict Promotion ban. | Yes (preserves promotion prohibition and worker restrictions). |
| Synthesis note review & contradiction arbitration | `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`, `plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Reassign synthesis-note review and contradiction arbitration to Specialist; Orchestrator receives only Phase Verdict. | Yes (reduces context; preserves escalation on `needs-revision`). |
| Acceptance Subagent delegation authority | `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36`, `plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Upgrade Acceptance Subagent to reflect decision-bearing authority for Specialists; maintain human confirmation for Promotion. | Yes (formalizes authority without altering human promotion gate). |
| `accept.Verifier.Verify` check gating | `internal/accept/accept.go:84-96`, `internal/accept/accept.go:120-137` | Load `LaneMetadata` unconditionally; run `v.check` only for `SDDPhase == "apply"`, empty/missing, or exception; skip `v.check` for non-apply phases while enforcing schema and scope (`internal/accept/accept.go:214-261`). | Yes (unlabeled/legacy and apply lanes continue running checks; non-apply phases skip). |
| `run.driveAttemptFromLeased` check gating | `internal/run/attempt.go:431-435`, `internal/run/attempt.go:448-467` | Gate `checkFunc` on SDD phase: run checks for `"apply"`, empty, or exception; skip `checkFunc` for non-apply attempts before CAS pending. | Yes (apply lanes and unannotated feature runs continue executing checks). |
| `ledger.LaneMetadata` struct & DDL | `internal/ledger/lanes_meta.go:20-47`, `internal/ledger/schema.go:425-445`, `internal/ledger/schema.go:584-592` | No new fields added; reuse `SDDPhase` (`internal/ledger/lanes_meta.go:20-47`). No DDL migration. | Yes (100% additive reuse of existing schema-v6 column and audit JSON). |
| Phase Verdict response format | `docs/sdd-phase-specialist.md:21-30`, `openspec/changes/agentic-phase-specialist/proposal.md:87-99` | Structured payload containing `outcome` (`accepted` \| `needs-revision`), `canonical_artifact_path`, and `unresolved_divergence`. | Yes (new structured interface between Specialist and Orchestrator). |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modify | Update line 19 (`plugin/claude-code/skills/lucind-ai/SKILL.md:19`) to carve out Specialist Acceptance while retaining agent Promotion ban. | Claude CLI runtime and `sdd-*` subagents reading skill prompt (`plugin/claude-code/skills/lucind-ai/SKILL.md:14-21`). |
| `plugin/opencode/skills/lucind-ai/SKILL.md` | Modify | Update line 19 (`plugin/opencode/skills/lucind-ai/SKILL.md:19`) to mirror Hard Rule carve-out. | OpenCode agent runtime and `TestSkillTreesByteIdentical` in `internal/packet/packet_test.go:943-967`. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md` | Modify | Update lines 47-48 (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`) transferring synthesis review to Specialist. | `sdd-*` subagents orchestrating planning cycles (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:7-16`). |
| `plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md` | Modify | Update lines 47-48 (`plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47-48`) to mirror synthesis review transfer. | OpenCode runtime and `TestSkillTreesByteIdentical` in `internal/packet/packet_test.go:943-967`. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md` | Modify | Update lines 31-36 (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36`) defining Specialist Acceptance authority. | `sdd-*` subagents executing acceptance sequence (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30`). |
| `plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md` | Modify | Update lines 31-36 (`plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36`) to mirror Specialist Acceptance authority. | OpenCode runtime and `TestSkillTreesByteIdentical` in `internal/packet/packet_test.go:943-967`. |
| `internal/accept/accept.go` | Modify | Load `LaneMetadata` unconditionally at lines 84-96 (`internal/accept/accept.go:84-96`); gate `v.check` at lines 120-137 (`internal/accept/accept.go:120-137`) on `SDDPhase == "apply"`, empty, or exception. | `lucind-ai accept` CLI in `cmd/lucind-ai/cli.go:658-715` invoked by Specialists. |
| `internal/accept/accept_test.go` | Modify | Add unit tests at lines 80-100 (`internal/accept/accept_test.go:80-100`) and lines 102-125 (`internal/accept/accept_test.go:102-125`) asserting check skip for non-apply and check execution for apply/empty/exception. | `go test ./internal/accept` invoked by repository verification in `lucind-checks.sh:1-4`. |
| `internal/run/attempt.go` | Modify | Gate `checkFunc` at lines 431-435 (`internal/run/attempt.go:431-435`) and lines 448-467 (`internal/run/attempt.go:448-467`) during `AttemptStatusChecking` on SDD phase (`"apply"`, empty, or exception). | `ExecuteAttempt` state machine in `internal/run/attempt.go:217-328` driven during batch run. |
| `internal/run/attempt_test.go` | Modify | Add unit tests at lines 80-100 (`internal/run/attempt_test.go:80-100`) asserting non-apply attempts skip checks while completing transitions. | `go test ./internal/run` invoked by repository verification in `lucind-checks.sh:1-4`. |

## Open Questions

- [ ] None.

## Citation Manifest

| Citation | Claim |
|---|---|
| `cmd/lucind-ai/cli.go:658-715` | `runAccept` CLI handler parses arguments, invokes `accept.Verifier.Verify`, and renders receipts. |
| `cmd/lucind-ai/cli.go:2517-2649` | `phaseDispatch` command and deterministic Phase Specialist implementation dispatching synthesis lanes. |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | Architecture decision establishing agentic Specialist Acceptance authority and check scoping on `sdd_phase: apply`. |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-19` | Consequences requiring SKILL.md Hard Rule carve-out, fan-out synthesis review reassignment, and Dual-Judge retention for Tier A. |
| `docs/sdd-phase-specialist.md:21-30` | Resolved decisions on Specialist runtime substrate, Acceptance decision authority, bounded revision, and Phase Verdict structure. |
| `internal/accept/accept.go:84-96` | Loading `LaneMetadata` conditionally in `validateTypedTargetBinding` during candidate verification. |
| `internal/accept/accept.go:120-137` | Unconditional execution of `integrate.CheckPolicySnapshot` and `v.check` inside isolated worktree. |
| `internal/accept/accept.go:214-261` | Verification of result envelope status, hard stops, done criteria, and `allowed_paths` scope. |
| `internal/accept/accept_test.go:80-100` | Verifier tests checking receipt persistence, binding validation, and exact cache reuse. |
| `internal/accept/accept_test.go:102-125` | Verifier test suite verifying rejection of invalid evidence, schema errors, and scope violations without receipt persistence. |
| `internal/integrate/integrate.go:159-200` | `Check` function executing root `lucind-checks.sh` unconditionally when called. |
| `internal/ledger/authoring.go:14-26` | `AuthoringEvidenceVersion` constant and `AuthoringEvidence` struct definition. |
| `internal/ledger/lanes_meta.go:20-47` | `LaneMetadata` struct definition capturing dispatch context and existing `SDDPhase` string field. |
| `internal/ledger/schema.go:425-445` | Migration DDL creating evidence and receipt columns without modifying lane metadata schema. |
| `internal/ledger/schema.go:584-592` | Schema migration version 10 execution ensuring no DDL changes are needed for metadata reuse. |
| `internal/packet/packet_test.go:924-941` | Tests verifying canonical `references/core/domain.md` projections stay in sync with `CONTEXT.md`. |
| `internal/packet/packet_test.go:943-967` | `TestSkillTreesByteIdentical` verifying Claude Code and OpenCode skill files are byte-identical. |
| `internal/phasespec/phasespec.go:308-333` | `CLIStatusQuerier` executing `gentle-ai sdd-status --json` to inspect phase state. |
| `internal/phasespec/phasespec.go:338-350` | `Adapter` coordinating SDD status inspection and deterministic synthesis lane dispatch. |
| `internal/run/attempt.go:217-328` | `ExecuteAttempt` durable integration state machine resolving attempts and leases. |
| `internal/run/attempt.go:431-435` | Attempt execution default check function assignment before running checks. |
| `internal/run/attempt.go:448-467` | Execution of `checkFunc` in `AttemptStatusChecking` and failure handling. |
| `internal/run/attempt_test.go:80-100` | Integration attempt test suite verifying `RunChecks` spy execution and attempt state transitions. |
| `internal/run/run.go:162-235` | `Deps` struct defining external dependency injection hooks including `RunChecks`. |
| `internal/run/run.go:841-845` | Lane status evaluation demoting lanes to blocked when hard stops fire. |
| `internal/run/run.go:856-878` | `enforceAllowedPaths` verifying candidate diffs strictly stay within declared `allowed_paths`. |
| `lucind-checks.sh:1-4` | Verification shell script running Go build and race-enabled test suite. |
| `openspec/changes/agentic-phase-specialist/proposal.md:67-85` | Accepted proposal requirement for Specialist Phase Acceptance and Hard Rule carve-out. |
| `openspec/changes/agentic-phase-specialist/proposal.md:87-99` | Accepted proposal requirement for Structured Phase Verdict Reporting. |
| `openspec/changes/agentic-phase-specialist/proposal.md:100-118` | Accepted proposal requirement for SDD Phase-Gated Verification Check Execution. |
| `openspec/changes/agentic-phase-specialist/proposal.md:119-132` | Accepted proposal requirement for Specialist-Owned Synthesis Arbitration. |
| `openspec/specs/acceptance-verifier/spec.md:30-33` | Specification requiring fail-closed mechanical criteria and scope validation. |
| `openspec/specs/acceptance-verifier/spec.md:124-127` | Specification prohibiting Acceptance Verifier from mutating refs or invoking Promotion. |
| `openspec/specs/phase-specialist-dispatch/spec.md:9-11` | Specification requiring phase specialist status ingestion and synthesis sequencing. |
| `openspec/specs/sdd-planning-fan-out/spec.md:9-12` | Specification defining two-wave fan-out protocol and synthesis lane sequencing. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:14-21` | Claude Code skill activation contract, Hard Rules, and worktree verification rules. |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19` | Hard Rule restricting agent ownership of Acceptance and Promotion. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:18-30` | Canonical 10-step mechanical acceptance sequence. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | Acceptance Subagent delegation pattern description. |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | Promotion gate requiring human confirmation before merging into target ref. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:7-16` | SDD 3-lens planning topology table across phases. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25` | Planning fan-out dispatch rules requiring accepted lens receipts before synthesis. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | Fan-out authority rules specifying Orchestrator synthesis-note review and contradiction arbitration. |
| `plugin/opencode/skills/lucind-ai/SKILL.md:19` | OpenCode mirror of Hard Rule restricting agent ownership of Acceptance and Promotion. |
| `plugin/opencode/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-36` | OpenCode mirror of Acceptance Subagent delegation pattern description. |
| `plugin/opencode/skills/lucind-ai/references/strategies/fan-out.md:47-48` | OpenCode mirror of synthesis-note review and contradiction arbitration rule. |
