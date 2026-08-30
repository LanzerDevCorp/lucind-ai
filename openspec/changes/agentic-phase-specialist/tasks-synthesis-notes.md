# Tasks Synthesis Notes: Agentic Phase Specialist

## Unresolved Contradictions

None. Lens B put both Claude Code and OpenCode skill trees in one unit, and Lens A’s 1.1–1.3 each edit both mirrors of one file pair, so `TestSkillTreesByteIdentical` (`internal/packet/packet_test.go:943-967`) is not a two-lane race on this partition. Independent lanes writing the same logical text into two trees remain a real risk if a later DAG splits the trees; this plan does not.

## Coverage Gaps

- **No waves merged.** Lens B’s one parallel wave (units 1–3) was re-checked: each unit keeps RED+GREEN together; the combined tree would pass `lucind-checks.sh` (`lucind-checks.sh:1-4`) because unit 1 is docs-only (`TestSkillAssetContract` at `packet_test.go:778` does not pin the pre-carve-out Hard Rule), unit 2 is `internal/accept` only, and unit 3 is `internal/run` only. Pairwise PathInScope on exact files (`disjoint.go:8-22,24-47`): `plugin/…/SKILL.md` (and the two reference files) vs `internal/accept/accept.go` vs `internal/run/attempt.go` — none prefixes another. Dispatch shape is still one sequential packet because B recommended no sidecar and `Integrate` reverts a red batch (`internal/run/integrate.go:50-59`).
- **Exception carrier.** Spec scenario “explicit check exception” (`specs/acceptance-verifier/spec.md:53-57`) and Decision 2 (`design.md:21-27`) name an exception; `LaneMetadata` has `SDDPhase` and no exception field (`lanes_meta.go:20-47`). Design leaves `--force-checks` vs packet metadata open (`design.md:139-140`). 2.1/3.1 cover apply, empty, and missing. No draft named a carrier; none invented.
- **C’s ungated `integrate.Check` proving command** (`go test ./internal/integrate -run 'TestCheck'`, live names `TestCheckAbsentScript` / `TestCheckScriptPasses` at `integrate_test.go:471-500`) maps to no Lens A checklist line. Existing tests already pin ungated `Check` (`integrate.go:159-200`). Apply must not edit that primitive (`design.md:147`).
- **sdd-tasks contract vs this packet.** Skill word budget 530 is superseded by packet 1800 (`tasks.md` is 915). Skill Engram persist (`sdd/agentic-phase-specialist/tasks`) and return block are superseded by the two files plus `.lucind/result.json`. Forecast field names and the N/A threat-matrix rule follow the skill; session review budget 1500 (not `config.yaml:7` 10000) was used to judge risk. B’s 100–200 line estimate was lighter on tests; C’s 250–350 is the forecast.
- **Phase Verdict has no Go parser** (C verification gap; Decision 1, `design.md:14-19`). Prompt-level; not a missing apply task.

## Dropped Citations

Every unique `file:line` in the three drafts (80 tokens) was opened. Claims below did not survive into `tasks.md` as stated.

- **Lens A/C `Verifier.Accept` and proving command `-run 'TestAccept_SDDPhase'`.** Method is `Verify` (`internal/accept/accept.go:62`). Existing tests are `TestVerifier*`. 2.1 uses `TestVerifier*` names. `accept_test.go:26-67` is `newVerifierFixture`, not those tests.
- **Lens C `accept.go:84-137` as already gating on SDD phase.** `GetLaneMetadata` is still inside the `AuthoringEvidenceVersion` branch (`:84-96`). `CheckPolicySnapshot` and `v.check` always run (`:120-137`). Range kept as the 2.2 modification site.
- **Lens C `attempt.go:431-448` as already gating `checkFunc` on combined-lane SDD phases.** `:448` always calls `checkFunc`. Range kept as the 3.2 modification site.
- **Lens A `accept_test.go:80-140` as asserting check failures.** `:80-100` is receipt persistence; `:102-125` invalid evidence/scope; `:127-140` documentation-like paths. Check failure is `TestVerifierCheckFailureAndForeignIsolationPersistNoReceipt` (`:320`).
- **Lens A/C `packet_test.go:924-941` as the whole of `TestSkillAssetContract`.** Function starts at `:778`; `:924-941` is only the `domain.md` / `CONTEXT.md` lockstep tail. `TestSkillTreesByteIdentical` is `:943-967`. 1.4 cites `:778,924-941` and `:943-967`.
- **Lens A `design.md:102-108` as the paste-ready prompt.** Prompt is `:104-106`; `:108` is `domain.md` lockstep. 4.1 cites `:102-106`.
- **Lens C `run.go:377-394` as the full `UpdateLaneMetadata` call.** Literal ends ~394; error path is `:377-397` (Lens A). 3.2 uses `:377-397`.
- **Lens C `-run 'TestCheck'` as a single `TestCheck` func.** Live names are `TestCheckAbsentScript` and `TestCheckScriptPasses` (`integrate_test.go:471-500`). Prefix match would still run both; not used as an A task (see Coverage Gaps).
- **Lens B `openspec/config.yaml:7-8` as this session’s 10000-line review budget.** File does contain `review_budget_lines: 10000` and `strict_tdd: true`. Packet preflight is 1500 for risk; `strict_tdd` is kept.

Verified and kept (not listed row-by-row): Hard Rule and OpenCode mirrors at `SKILL.md:19`; `fan-out.md:47-48` Orchestrator synthesis review; `acceptance-promotion.md:18-36`; `CONTEXT.md:23-26,91-93,103-109`; ADR-0002 `:5-8`; `docs/sdd-phase-specialist.md:21-30`; all four spec deltas; design decisions, file-changes, threat matrix N/A, testing seams; `disjoint.go`; `integrate.go:50-59` and `:159-200`; `lanes_meta.go:20-47,49-60`; `run.go:165,208`; `lucind-checks.sh:1-4`; archived sidecar decline `:26-27`.

## Decomposition Divergence

Lens A’s four-phase split is the checklist in `tasks.md`. It was not refuted: every named production file exists; Phase 4 is out-of-repo by design (`design.md:102-106`).

**Independent convergence.** B and C also opened with three in-repo deliverables: skill/contract docs, `internal/accept` gating, `internal/run` attempt gating. That corroborates A’s phases 1–3.

**Lens B — three units, all independent, no Phase 4 work unit.** Cost: out-of-repo handoff omitted from Suggested Work Units. Kept as Phase 4 / 4.1, not a PR unit. B collapsing 1.1–1.3 into unit 1 is compatible with A’s dependency table (1.4 after all three); `tasks.md` keeps A’s three checklist lines under one work unit so both trees stay in one lane.

**Lens C — three-unit sequential path (skills + accept before attempt).** A’s table gives 3.1 no dependency on 2.x; B called the packages independent. Code: `accept.go` and `attempt.go` are separate callers of ungated `integrate.Check` (`design.md:21-27`). C’s sequential “critical path” is not required for greenness. Cost: C’s combined proving narrative; `tasks.md` keeps 2.x and 3.x as sibling units.

**Lens C Review Workload Forecast vs B’s sidecar rationale.** Forecast (C) and no-sidecar (B) both survive. B’s three-unit table lacked skill columns `Likely PR`, `Focused test command`, and `Runtime harness`; those were filled from C’s proving commands (retargeted) and A’s files.
