# Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

## Unresolved Contradictions

None. Where B or C differed from A, Lens A's architecture or a failed citation settled the text. Those are recorded below, not escalated.

## Coverage Gaps

- **`sdd_phase` membership of `remediate` — raised as a gap, now DECIDED.** `proposal-synthesis-notes.md` asked design to confirm it. Lens B listed `{explore, propose, spec, design, tasks, apply, verify, archive}` with no rationale for the omission, so synthesis escalated it rather than silently adopting B's set. Resolved after adversarial validation: see Open Questions below.
- **Specialist `sdd-attempt` bracketing** for apply/verify/remediate (one-attempt-one-worktree). Propose-phase leftover; no design lens covered it. Not invented into `design.md` (listed under Out of Scope).
- **Per-file template edits.** Proposal affected areas name `.agents/skills/lucind-*` and `plugin/.../assets/*.md`. No lens specified which files drop which hardcoded paths. `design.md` keeps summary rows only.
- **sdd-design heading names.** The skill requires `## Data Flow`, `## Interfaces / Contracts`, and `## Migration / Rollout`. `design.md` uses the packet spine names `## Flow and Invariants` and `## Rollback and Additivity`; type deltas live in File Changes rather than a separate Interfaces section. Substantively filled. The skill's 800-word budget lost to this packet's 1800-word execution rule; the correction pass below stayed inside it by trimming duplicated prose and citations rather than dropping evidence.

Spine items 1–8 are covered.

## Dropped Citations

Unique manifest ranges opened: 64. Dropped or retargeted: 12. All other unique ranges resolved and supported their claim. Propose-phase already-dropped citations (`authoring.go:23`, `compile.go:49-65` as a budget seam, `authoring_evidence_test.go:56-127` as a reflection pin, `skillcontent.go:90-100` as the full `HashDir` range) were not repeated by the design lenses.

1. **Dropped rationale — `internal/dag/parse.go:45-56` as the `KnownFields(true)` pattern (Lens A).** Lines 45–56 are `os.ReadFile` plus `yaml.Unmarshal`. `KnownFields` appears nowhere in this repository. `design.md` keeps A's `KnownFields(true)` choice for `lucind.yaml` and cites `:45-54` only as the permissive `Unmarshal` alternative.

2. **Dropped current-behavior — `internal/run/run.go:876-904` already demotes missing envelope skill declarations (Lens A Technical Approach).** The range is `enforceAllowedPaths` (git-diff demotion). `enforceRequiredSkills` does not exist. Kept as the insertion seam; call site is `:489-491`.

3. **Dropped current-behavior — `internal/packetauthor/compile.go:171-183` already renders `## Required skills` (Lens A Dual Delivery).** `renderBody` emits Goal, Done criteria, Hard stops, Return. Kept as the insertion point; the `- %s\n` list form is real.

4. **Dropped "without state mutation" — `cmd/lucind-ai/cli.go:684-687` (Lens A manifest).** `Verify` persists receipts. `accept.go:1-2` never promotes or mutates refs; it is not write-free. Kept as error-only, never demotes, exit 1.

5. **Retargeted — `cmd/lucind-ai/cli.go:150-168` as the full dispatch table (Lens A).** The switch starts at line 142 (`case "run"`). `design.md` cites `142-168`.

6. **Dropped — `internal/accept/authoring_evidence_test.go:13-44` as freeze/decode roundtrips (Lens C).** That range is `TestValidateTypedTargetBindingRequiresCompleteIdentity`, which opens at line 13. Correspondence mutations are `:56-127`.

7. **Dropped — `internal/candidatechange/collect_test.go:1-50` as four-way git-diff tests (Lens C).** `:1-50` is `TestCollectCanonicalCommittedChangesAndCopyScope`. Four-way union is `TestCollectFourWayUnionAndCanonicalRootSelectors` at `:56-100`. Not cited in `design.md`.

8. **Dropped terminal consumer — `cmd/lucind-ai/cli.go:684-687` as the `phase` subcommand (Lens B File Changes).** `:684-687` is the `accept` Verify error path. `phase` does not exist. `design.md` attaches `phase` to `cli.go:142-168`.

9. **Dropped current-behavior — `internal/packet/packet.go:122-179` already closed-validates `lane_role`/`sdd_phase` (Lens C manifest).** The switch copies raw strings; `sdd_phase` at `:159-164` is unvalidated. Kept as the parse seam to extend.

10. **Dropped current-behavior — `cmd/lucind-ai/packet_authoring.go:32-54` already rejects missing required skills (Lens C manifest).** The function admits via `AdmitBatch` after target resolution; there is no skill existence check. Kept as the fail-closed seam to extend.

11. **Dropped name — `internal/run/run.go:882-904` as `enforceRequiredSkills` (Lens B).** `:882-904` is the body of `enforceAllowedPaths`. Same seam as item 2.

12. **Retargeted — `internal/run/run.go:870-874` as `decideStatus` mapping Envelope to lane.Status / `SkillsLoaded` consumer (Lens B).** `decideStatus` starts at `:849`; `:870-874` is the empty-status fallback. `SkillsLoaded` does not exist yet; its consumer is the proposed `enforceRequiredSkills`, not `decideStatus`.

Lens A prose cited `internal/executor/executor.go:20-39` (missing from A's manifest). Opened: `requestEnv` strips and injects `LUCIND_READ_ONLY_PATHS`. Kept as the dual-delivery env seam.

## Architecture Divergence

All three independently converged on Candidate 1: three-tier `derived ∪ stack ∪ adhoc`, machine-local `.lucind/skill-roots.yaml`, dual delivery (`## Required skills` + `LUCIND_REQUIRED_SKILLS`), skills inside `Contract json.RawMessage` with no ledger migration, two-site enforcement (`run` demotes, `accept` re-verifies), and a non-intercepting specialist. That convergence is corroboration.

Content that did not survive Lens A's architecture:

- **Budget home.** A: `DefaultSkillBudget` in `internal/skillset`, optional `skill_budget` in `lucind.yaml` via `internal/lucindconfig`, evaluated at `admitDispatchBatch`. B's flow placed "Resolution & Budget" in `internal/skillroots`. C's new seam `skillset_test.go` matches A. B's budget-in-skillroots is out of `design.md`.
- **`AdhocSkills`.** A Decision 1 adds both a typed field and `adhoc_skills` frontmatter. B's surface table adds only `LaneRole` and `RequiredSkills`. `design.md` follows A.
- **`lucindconfig` consumer.** B pointed it at `skillset.Derive`. A's `Derive` is pure — callers load stack names and pass them in. `design.md` attaches `lucindconfig` to `admitDispatchBatch`.
- **`phasespec` consumer.** B pointed it at `cli.go:684-687` (`phase`). That line is `accept`. A adds `phase` to the dispatch switch. `design.md` follows A (`cli.go:142-168`).
- **C "sequences DAG execution".** A's specialist is `gentle-ai sdd-status` → `admitDispatchBatch` → wait for lens accept+merge → synthesis. It does not drive `internal/dag` apply-dag.yaml. C's DAG wording is out; the lens-then-synthesis barrier remains (`fan-out.md:24`).

Compatible fills, not divergences: C's Open Question 2 close (no `lucind-archive` / `lucind-ultrafixer` stubs) fits A's pure `Derive` and the current `.agents/skills/` tree (`lucind-executor`, `lucind-fan-out-lens`, `lucind-apply`, `lucind-verify` only). C's rollback order matches A's additive packages. B's seven-value `lane_role` set and `## Required skills` list format are A's missing surface detail, not a competing architecture. C's threat-matrix PR-commands row stays `Applicable` (C owned the matrix); the RED test is malformed `sdd-status` JSON, not `gh pr` argv.

**Carried-forward item 1 (`sdd_phase` / `remediate`).** Lens B gave no rationale for omitting `remediate`; explore.md records it as a gentle-ai phase token. Synthesis refused to silently invent it into B's set and escalated instead. Now decided — see Open Questions.

**Carried-forward item 2 (real `sdd-design` skill vs packet parameters).** Closed inside Decision 3 (A's specialist composition), not relisted under Open Questions. `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:43`: content authority stays with gentle-ai's phase skills; execution authority (budgets, paths, done criteria, `.lucind/result.json`) stays with the dispatched packet. That is what `internal/phasespec` must encode.

**C "deriving only `lucind-executor`" vs proposal item 1 "`archive` derives `sdd-archive`".** Not a B/C vs A contradiction. C owned Open Question 2 (missing *lucind* children: stubs vs derived-empty). `~/.claude/skills/sdd-archive/SKILL.md` exists; `lucind-archive` does not. `design.md` Decision 7: no lucind stubs; `archive` still derives existing `sdd-archive`. C's alternatives line naming a stub `sdd-archive` was an existence miss, recorded here rather than escalated.

## Open Questions

### 1. `sdd_phase` membership of `remediate` — DECIDED (include it)

**History.** Raised by `proposal-synthesis-notes.md:10`, which asked design to confirm the set. Lens B listed `{explore, propose, spec, design, tasks, apply, verify, archive}` and gave no rationale for omitting `remediate`. Synthesis verified the question was real rather than fabricated — `explore.md:285` records `propose|spec|design|tasks|apply|verify|remediate|archive|sdd-new|select-change|resolve-blockers` as the gentle-ai phase tokens — and left it open in `design.md` rather than adopting B's set as if the omission had been considered. Adversarial validation confirmed the flag as real.

**Decision.** Include `remediate`. The closed set in Decision 6 is `{explore, propose, spec, design, tasks, apply, verify, remediate, archive}`.

**Rationale.** `remediate` is a real phase token (`explore.md:285`), so a closed set that rejects it would reject a legitimate lane. No `sdd-remediate` skill exists under `~/.claude/skills/`, but that is a derivation question, not a membership one: Decision 7 already established the derived-empty treatment for a token whose skill tier is absent (`archive`/`ultrafixer` lucind children), so `remediate` is admitted into the set and resolves to no phase skill until `sdd-remediate` exists. The alternative — omitting it — would have made today's unvalidated raw string (`internal/packet/packet.go:159-160`, consumed as telemetry at `internal/ledger/lanes_meta.go:25`, `internal/run/run.go:387`, `internal/run/batch.go:199`) stricter than the phase vocabulary it mirrors.

`explore` stays in the set on different grounds: `explore.md:282-284` records that it is *not* a gentle-ai phase — pre-proposal research is orchestrator-owned, outside native status. It is a lucind-ai-local token. Decision 6 now attributes the two halves of the set separately rather than citing `explore.md:285` for both.

## Post-Validation Corrections

Applied after a fresh-context adversarial validation returned PASS-WITH-CORRECTIONS. `design.md` Decision 8 has no lens ancestor: it originates from that validation, not from Lens A, B, or C.

1. **Blocking — the digest hashed resolved paths.** `renderBody` emits `- <resolved-path>` lines into the body, and both digest sites hash the body: `Compile` at `internal/packetauthor/compile.go:45` and `packetDigest` at `internal/run/run.go:725` (via `p.Body`). That contradicted the `design.md` invariant "paths never hash" and defeated `proposal.md:178`'s stable-digest-across-machines requirement. Human decision: keep the paths in the rendered body for readability, exclude them from the hash. That is the only reading that satisfies both success criteria at once — `proposal.md:178` demands a stable digest across differing roots, and `proposal.md:180` demands the dispatched body list resolved paths. Decision 8 specifies `skillset.DigestBody`, a positional elision of the `## Required skills` section applied at both sites, with canonical names re-entering the hash through `contractJSON` and through explicit `packetDigest` field-list entries.
2. **Non-blocking — undisclosed drift seam.** `packetDigest` (`run.go:722-729`) enumerates packet fields literally, so `LaneRole`, `AdhocSkills`, and `RequiredSkills` would have escaped the digest silently. `design.md` disclosed the analogous `accept.go:275-286` decode struct but not this one. Both are now named together in Decision 8. Also noted there: `versionedHash` (`run.go:731-740`) length-prefixes elements but not groups, so appending further variable-length slices without an explicit count would make the existing `paths` / `ReadOnlyPaths` adjacency ambiguous.
3. **Non-blocking — `required_skills` under-specified.** `design.md` added it to `packet.Packet` while Decision 1 defined only `adhoc_skills` frontmatter. Decision 1 now states it is derived, never authored; the parse switch (`internal/packet/packet.go:122-179`) has no `required_skills` case and ignores unknown keys.
4. **Cosmetic.** Dropped-citation item 6 cited `authoring_evidence_test.go:12-44`; the function opens at line 13.
