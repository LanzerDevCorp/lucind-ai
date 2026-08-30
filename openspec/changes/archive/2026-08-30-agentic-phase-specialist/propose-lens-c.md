# Proposal Lens C — Risks, Rollback & Test Impact: Agentic Phase Specialist

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| Autonomous Specialist Acceptance admits hallucinated or defective planning artifacts | Low-quality or invalid planning documents enter repo history without human inspection | Restrict Specialist Acceptance to non-Tier-A planning phases. Keep deterministic schema/scope validation (`allowed_paths`), hard-stop demotion to `blocked`, Tier A Dual-Judge qualitative audit, and human-confirmed Promotion strictly intact | `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-50`, `internal/run/run.go:841-875`, `internal/accept/accept.go:97-98` |
| Widening `LaneMetadata.SDDPhase` check gate in `accept.go` silently skips Go test suite for code-modifying lanes or fails when metadata is absent | Breaking Go compilation or test regressions accepted into feature branches under mislabeled or missing `sdd_phase` | Fail-safe default: execute `lucind-checks.sh` if `sdd_phase == "apply"`, `sdd_phase == ""` (legacy/unlabeled), or any modified file in `files_changed` has `.go` extension. Extract `GetLaneMetadata` safely before verification | `internal/accept/accept.go:84-96`, `internal/accept/accept.go:120-137`, `internal/run/attempt.go:415-460`, `internal/ledger/lanes_meta.go:20-47` |
| Hard Rule carve-out in `SKILL.md` is misread as granting general Acceptance authority to standard executor subagents | Generic executor agents (`agy`, `cursor-agent`, `lucind-apply`) bypass verification checklist gates or attempt Promotion | Explicitly restrict carve-out strictly to named phase-scoped Specialists (`sdd-*`) for their own phase's lanes only; reinforce that standard executor lanes have zero acceptance authority and Promotion remains strictly forbidden to all AI agents | `plugin/claude-code/skills/lucind-ai/SKILL.md:19-20`, `plugin/opencode/skills/lucind-ai/SKILL.md:19-20`, `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` |
| Divergence between Claude Code and OpenCode skill trees during Specialist / Hard Rule edits | Preflight and worktree allocation fail due to skill tree mismatch check | Use byte-identity test suite (`TestSkillTreesByteIdentical`) and glossary projection test (`TestSkillDocumentsLanguageGlossary`) as mandatory verification gates | `internal/packet/packet_test.go:924-967`, `plugin/claude-code/skills/lucind-ai/SKILL.md:21-22`, `CONTEXT.md:103-109` |
| Specialist launches synthesis before all disjoint lenses are accepted and merged | Lens output diff charged against synthesis attempt objective line budget, causing attempt line-limit failure | Maintain strict fan-out ordering: specialist blocks synthesis dispatch until all required lens candidate receipts are recorded and branches merged | `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25`, `docs/sdd-phase-specialist.md:21-30` |
| Orchestrator triggers unbounded full re-fan-outs upon receiving a `needs-revision` Phase Verdict | Token churn, context exhaustion, and runaway lane allocations | Enforce bounded relaunch pattern: Orchestrator triggers at most one scoped correction transaction rather than re-running the entire phase | `docs/sdd-phase-specialist.md:21-30`, `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` |

## Rollback & Additivity

**Rollback Plan**: Complete reversal requires a standard `git revert` of code and documentation commits. No database schema rollback or ledger data migration is required because no table definitions or columns are altered (`internal/ledger/schema.go:425-445`, `internal/ledger/schema.go:584-592`). Reverting code restores unconditional `integrate.Check` execution in `internal/accept/accept.go:120-137` and `internal/run/attempt.go:415-460`, and restores Orchestrator-owned synthesis note review in `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48`.

**Additivity**: Formats, schemas, and ledgers change strictly additively:
- **Ledger Schema**: Zero DDL changes or migrations. Reuses existing `LaneMetadata.SDDPhase` column/audit field in schema-v6 event log (`internal/ledger/lanes_meta.go:20-47`, `internal/ledger/lanes_meta.go:49-60`).
- **Authoring Evidence & Contracts**: Additive. `AuthoringEvidenceVersion` remains `"lane-authoring-evidence/v1"` (`internal/ledger/authoring.go:14`) with contract data carried in `Contract json.RawMessage` (`internal/ledger/authoring.go:23`, `internal/ledger/authoring.go:44-75`). Pre-existing frozen candidates decode byte-identically.
- **Check-Gating Logic**: Additive and backward-compatible. Packets lacking `sdd_phase` (empty string) continue running full mechanical checks by default, matching legacy behavior.

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| Acceptance Verifier (`internal/accept`) | Verify `Verifier.Verify` skips `lucind-checks.sh` when `metadata.SDDPhase` is a non-apply planning phase (`propose`, `spec`, `design`, `tasks`), executes checks when `SDDPhase == "apply"` or `""`, safely handles missing metadata, and executes checks if Go files were modified | `internal/accept/accept.go:84-140`, `internal/accept/accept_test.go:26-67`, `internal/accept/accept_test.go:80-120` |
| Integration Attempt (`internal/run`) | Verify `executeAttempt` respects phase-gated execution of `RunChecks` without compromising lease renewal, checking state transitions, or error logging | `internal/run/attempt.go:415-460`, `internal/run/attempt_test.go:24-80` |
| Mechanical Verification (`internal/integrate`) | Verify `integrate.Check` remains unchanged as a reusable verification execution primitive | `internal/integrate/integrate.go:159-200`, `internal/integrate/integrate_test.go:21-50` |
| Skill & Contract Parity (`internal/packet`) | Verify `TestSkillTreesByteIdentical` and `TestSkillDocumentsLanguageGlossary` pass against updated Hard Rules, fan-out strategy, and domain glossary projections | `internal/packet/packet_test.go:924-967` |

## Out of Scope

- Modifying `AuthoringEvidence` struct shape, `AuthoringEvidenceVersion`, or SQLite schema migrations (`internal/ledger/authoring.go:14-23`, `internal/ledger/schema.go:425-445`).
- Delegating Change-level Promotion authority to any AI agent or Specialist (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50`, `CONTEXT.md:91-94`).
- Altering unconditional post-dispatch `allowed_paths` boundary enforcement or hard stop demotion logic (`internal/run/run.go:841-878`, `internal/accept/accept.go:172-207`).
- Granting direct Bash/Agent dispatch tool access to `sdd-*` planning subagents (Orchestrator retains CLI dispatch invocation).
- Extending coordination scope across multi-repository or distributed topologies (`CONTEXT.md:23-26`).

## Open Questions

- [ ] Should check-skipping in `internal/accept/accept.go` explicitly inspect `files_changed` for `.go` source files in addition to `metadata.SDDPhase != "apply"` before bypassing checks?
- [ ] Should the Phase Verdict payload schema be formally validated via JSON schema in `internal/result/` or remain a structured markdown section returned to the Orchestrator?

## Citation Manifest

| citation | claim |
|---|---|
| `CONTEXT.md:23-26` | defines Coordination Scope as one local repository on one machine |
| `CONTEXT.md:51-54` | defines Acceptance as verified inclusion of a Lane result into its owning Change without human confirmation |
| `CONTEXT.md:91-94` | defines Promotion as the human-confirmed integration of a completed Change into its target |
| `CONTEXT.md:103-106` | defines Specialist as a phase-scoped Agent holding autonomous Acceptance authority for its own phase |
| `CONTEXT.md:107-109` | defines Phase Verdict as the compressed report returned by a Specialist to the Orchestrator |
| `cmd/lucind-ai/cli.go:2516-2517` | implements the deterministic phase subcommand dispatch entry point |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:5-8` | records architectural decision granting phase-scoped Acceptance authority and scoping checks to apply phase |
| `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:16-19` | specifies consequences on SKILL.md Hard Rule carve-out, fan-out strategy, and Dual-Judge for Tier A |
| `docs/sdd-phase-specialist.md:7-10` | defines problem of Orchestrator context bloat and goal of compressed Phase Verdict |
| `docs/sdd-phase-specialist.md:13-18` | inventories existing phasespec adapter, fan-out strategy, Claude Code subagents, and unconditional checks |
| `docs/sdd-phase-specialist.md:21-30` | records resolved decisions on runtime substrate, Acceptance authority, Promotion exclusion, and checks gating |
| `docs/sdd-phase-specialist.md:38-41` | lists pending implementation tasks for Hard Rule rewrite, fan-out strategy update, and checks gating |
| `internal/accept/accept.go:84-96` | loads LaneMetadata conditionally only when AuthoringEvidenceVersion matches |
| `internal/accept/accept.go:97-98` | invokes validateResultAndScope to enforce result schema and path boundaries before checks |
| `internal/accept/accept.go:120-137` | executes checks unconditionally in owned isolation and fails acceptance if checks do not pass |
| `internal/accept/accept.go:172-207` | enforces candidate commit presence, result existence, and allowed_paths containment |
| `internal/accept/accept_test.go:26-67` | sets up verifier fixture with isolated git repo, lucind-checks.sh, and candidate registration |
| `internal/accept/accept_test.go:80-120` | tests verifier persistence of complete receipt and exact binding reuse |
| `internal/integrate/integrate.go:159-200` | executes lucind-checks.sh at worktree root with timeout and captures combined output |
| `internal/integrate/integrate_test.go:21-50` | provides test helpers for git repo initialization and execution in integrate tests |
| `internal/ledger/authoring.go:14` | declares AuthoringEvidenceVersion constant frozen at v1 |
| `internal/ledger/authoring.go:23` | defines Contract as json.RawMessage allowing additive contract extensions without struct migration |
| `internal/ledger/authoring.go:44-60` | computes length-prefixed domain hash over serialized AuthoringEvidence payload |
| `internal/ledger/authoring.go:62-75` | decodes and verifies AuthoringEvidence against frozen version and hash |
| `internal/ledger/lanes_meta.go:20-47` | defines LaneMetadata struct carrying existing SDDPhase string field |
| `internal/ledger/lanes_meta.go:49-60` | updates lane metadata and appends snapshot to append-only event log transactionally |
| `internal/ledger/schema.go:425-445` | defines DDL migration v9 to v10 adding authoring evidence columns and shadow tables |
| `internal/ledger/schema.go:584-592` | applies schema migration transactionally when database version is below 10 |
| `internal/packet/packet_test.go:924-941` | validates CONTEXT.md glossary projections match references/core/domain.md |
| `internal/packet/packet_test.go:943-967` | asserts Claude Code and OpenCode skill trees are byte-identical |
| `internal/run/attempt.go:415-460` | transitions attempt to checking status and executes checkFunc unconditionally |
| `internal/run/attempt_test.go:24-44` | defines attemptSpies struct tracking createWorktree, combine, check, and promoteCAS calls |
| `internal/run/attempt_test.go:46-80` | constructs test dependencies, ledger, and feature service for attempt execution tests |
| `internal/run/run.go:841-845` | demotes candidate lane to Blocked if any declared hard stop fired |
| `internal/run/run.go:856-878` | demotes lane to Deviated if git diff touched paths outside declared allowed_paths |
| `internal/run/run.go:883-906` | demotes lane to Deviated if result envelope omitted declared required skills |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:19-20` | defines Hard Rule restricting Acceptance and Promotion authority to Orchestrator |
| `plugin/claude-code/skills/lucind-ai/SKILL.md:21-22` | enforces byte-identity between Claude Code and OpenCode skill trees before worktree allocation |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:16-30` | defines 10-step canonical acceptance protocol and checklist |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:31-37` | defines Acceptance subagent delegation protocol for evidence gathering |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43` | mandates Dual-Judge qualitative acceptance for Tier A Changes |
| `plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:44-50` | defines Promotion gate requiring explicit human confirmation |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:7-16` | defines 3-lens fan-out and synthesis planning topology across SDD phases |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25` | mandates lens wave dispatch before synthesis and forbids synthesis before lens acceptance |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:47-48` | assigns synthesis note review and contradiction arbitration directly to Orchestrator |
| `plugin/opencode/skills/lucind-ai/SKILL.md:19-20` | mirrors Hard Rule restricting Acceptance and Promotion authority to Orchestrator |
