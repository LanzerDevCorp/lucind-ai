# Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

## Unresolved Contradictions

None. Open Questions 1–5 were each closed by the owning lens with a concrete choice. Where B or C differed from A, step 4 (A authoritative) or a failed citation settled the text; those are recorded below, not escalated.

## Coverage Gaps

- **`sdd_phase` membership of `remediate`.** `proposal-synthesis-notes.md` asked design to confirm whether `remediate` belongs in the closed set. No design lens discussed it. `design.md` uses Lens B's explicit set `{explore, propose, spec, design, tasks, apply, verify, archive}`, which omits `remediate` without rationale.
- **Specialist `sdd-attempt` bracketing** for apply/verify (one-attempt-one-worktree). Propose-synthesis leftover; no design lens covered it. Not invented into `design.md`.
- **How to drop hardcoded `~/.claude/skills/...` paths and executor-name coupling.** Proposal affected areas name `plugin/.../assets/*.md` and `.agents/skills/lucind-*`. No lens specified per-file edits. `design.md` keeps summary rows only.
- **sdd-design heading names.** The skill requires `## Data Flow` and `## Migration / Rollout`. `design.md` uses packet spine names `## Flow and Invariants` and `## Rollback and Additivity`; both are substantively filled, plus skill-required `## Interfaces / Contracts`. Skill's 800-word budget lost to this packet's 1800-word execution rule (1751 words).

## Dropped Citations

Unique manifest ranges opened: 64. Claims dropped or retargeted: 12. All other unique ranges resolved and supported their claim.

1. **Dropped rationale — `internal/dag/parse.go:45-56` as the `KnownFields(true)` pattern (Lens A).** Lines 45–56 are `os.ReadFile` plus `yaml.Unmarshal`. `KnownFields` appears nowhere in this repository. `Unmarshal` is permissive. `design.md` keeps A's `KnownFields(true)` choice for `lucind.yaml` and cites `:45-54` only as the roots-loading pattern.

2. **Dropped current-behavior — `internal/run/run.go:876-904` already demotes missing envelope skill declarations (Lens A Technical Approach).** The range is `enforceAllowedPaths`, which demotes out-of-scope git diffs. `enforceRequiredSkills` does not exist. Kept as the insertion seam.

3. **Dropped current-behavior — `internal/packetauthor/compile.go:171-183` already renders `## Required skills` (Lens A Dual Delivery).** `renderBody` emits Goal, Done criteria, Hard stops, Return. Kept as the insertion point; the `- %s\n` list form is real.

4. **Dropped "without state mutation" — `cmd/lucind-ai/cli.go:684-687` (Lens A manifest).** `Verify` persists receipts. `accept.go:1-2` never promotes or mutates refs; it is not write-free. Kept as error-only, never demotes, exit 1.

5. **Retargeted — `cmd/lucind-ai/cli.go:150-168` as the full dispatch table (Lens A).** The switch starts at line 142 (`case "run"`). `design.md` cites `142-168`.

6. **Dropped — `internal/accept/authoring_evidence_test.go:12-44` as freeze/decode roundtrips (Lens C).** That range is `TestValidateTypedTargetBindingRequiresCompleteIdentity`. Correspondence mutations are `:56-127`.

7. **Dropped — `internal/candidatechange/collect_test.go:1-50` as four-way git-diff tests (Lens C).** `:1-50` is `TestCollectCanonicalCommittedChangesAndCopyScope`. Four-way union is `TestCollectFourWayUnionAndCanonicalRootSelectors` at `:56-100`. Not cited in `design.md`.

8. **Dropped terminal consumer — `cmd/lucind-ai/cli.go:684-687` as the `phase` subcommand (Lens B File Changes).** `:684-687` is the `accept` Verify error path. `phase` does not exist. `design.md` attaches `phase` to `cli.go:142-168`.

9. **Dropped current-behavior — `internal/packet/packet.go:122-179` already closed-validates `lane_role`/`sdd_phase` (Lens C manifest).** The switch copies raw strings; `sdd_phase` at `:159-164` is unvalidated. Kept as the parse seam to extend.

10. **Dropped current-behavior — `cmd/lucind-ai/packet_authoring.go:32-54` already rejects missing required skills (Lens C manifest).** The function admits via `AdmitBatch` after target resolution; there is no skill existence check. Kept as the fail-closed seam to extend.

11. **Dropped name — `internal/run/run.go:882-904` as `enforceRequiredSkills` (Lens B).** `:882-904` is the body of `enforceAllowedPaths`. Same seam as item 2.

12. **Dropped applicability — PR commands row Applicable because the specialist runs `gentle-ai sdd-status` (Lens C).** The matrix row is PR argv (`--head`, composed PR commands). This repository has no PR automation. `design.md` marks the row `N/A`.

Lens A prose cited `internal/executor/executor.go:20-39` (missing from A's manifest). Opened: `requestEnv` strips and injects `LUCIND_READ_ONLY_PATHS`. Kept as the dual-delivery env seam.

## Architecture Divergence

All three independently converged on Candidate 1: three-tier `derived ∪ stack ∪ adhoc`, machine-local `.lucind/skill-roots.yaml`, dual delivery (`## Required skills` + `LUCIND_REQUIRED_SKILLS`), skills inside `Contract json.RawMessage` with no ledger migration, two-site enforcement (`run` demotes, `accept` re-verifies), and a non-intercepting specialist. That convergence is corroboration.

Content that did not survive Lens A's architecture:

- **Budget home.** A: `DefaultSkillBudget` in `internal/skillset`, optional `skill_budget` in `lucind.yaml` via `internal/lucindconfig`, evaluated at `admitDispatchBatch`. B's flow placed "Resolution & Budget" in `internal/skillroots`. C's new seam `skillset_test.go` matches A. B's budget-in-skillroots is out of `design.md`.
- **`AdhocSkills`.** A Decision 1 adds both a typed field and `adhoc_skills` frontmatter. B's surface table adds only `LaneRole` and `RequiredSkills`. `design.md` follows A.
- **`phasespec` consumer.** B pointed it at `cli.go:684-687` (`phase`). That line is `accept`. A adds `phase` to the dispatch switch. `design.md` follows A.
- **C "sequences DAG execution".** A's specialist is `gentle-ai sdd-status` → `admitDispatchBatch` → wait for lens accept+merge → synthesis. It does not drive `internal/dag` apply-dag.yaml. C's DAG wording is out; the lens-then-synthesis barrier remains (`fan-out.md:24`).
- **C PR-commands Applicable.** Out (Dropped Citations item 12).

Compatible fills, not divergences: C's Open Question 2 close (no `lucind-archive` / `lucind-ultrafixer` stubs; those roles derive only `lucind-executor`) fits A's pure `Derive` and the current `.agents/skills/` tree (`lucind-executor`, `lucind-fan-out-lens`, `lucind-apply`, `lucind-verify` only). C's rollback order matches A's additive packages. B's `lane_role` closed set of seven and `## Required skills` list format are A's missing surface detail, not a competing architecture.

Orchestrator note, not an A/B/C contradiction: C's "deriving only `lucind-executor`" omits proposal item 1's "`archive` derives `sdd-archive`". `~/.claude/skills/sdd-archive` exists on this machine. `design.md` follows C because C owned Open Question 2.
