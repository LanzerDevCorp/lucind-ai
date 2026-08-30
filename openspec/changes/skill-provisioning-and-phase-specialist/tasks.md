# Tasks: Skill Provisioning and the SDD Phase Specialist

`apply-dag.yaml` sidecar warranted (`internal/dag/parse.go:40-60`): 10 units, 4 waves, real parallelism in waves 1 (4 units) and 2 (3). `tasks.md` is the human checklist, not the parse source. Wave 3 keeps `packetDigest` (`internal/run/run.go:722-729`) and the accept decode struct (`internal/accept/accept.go:275-286`) in one unit per Decision 8 (`design.md:61-66`). No wave merge: each combined tree is green under `Integrate` (`internal/run/integrate.go:50-59`). All units: executor `agy`. Single PR.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1,200–1,600 lines (4 new packages: ~700–900 lines; 11 modified files: ~500–700 lines) |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | single PR (size:exception pre-accepted) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |
| review_budget_lines | 10000 |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| `result-envelope` | Optional `skills_loaded` on schema + `Envelope`; reflection pin | PR 1 | `go test -run TestSchema ./internal/result` | N/A: schema fixtures (`schema_test.go:10-33`) | `internal/result/` |
| `skillset-engine` | Pure `Derive`, `DefaultSkillBudget = 3`, `DigestBody` | PR 1 | `go test -run TestDerive ./internal/skillset` | N/A: pure function | `internal/skillset/` |
| `skillroots-resolution` | Ordered roots, `~` expand, load-as-data | PR 1 | `go test -run TestResolveRoots ./internal/skillroots` | N/A: temp dirs | `internal/skillroots/` |
| `lucindconfig-loader` | `lucind.yaml` + `KnownFields(true)` | PR 1 | `go test -run TestLoadConfig ./internal/lucindconfig` | N/A: testdata YAML | `internal/lucindconfig/` |
| `packet-contract` | Packet/Contract fields, parse, `renderBody`, hash `DigestBody` | PR 1 | `go test -run 'TestParseObservabilityFrontmatter\|TestCompile' ./internal/packet ./internal/packetauthor` | N/A: Parse/Compile fixtures | `internal/packet/`, `internal/packetauthor/` |
| `executor-env` | `Request.RequiredSkills`; inject `LUCIND_REQUIRED_SKILLS` | PR 1 | `go test -run 'ReadOnlyPaths\|RequiredSkills' ./internal/executor` | Subprocess stub (`read_only_paths_test.go:1-50`) | `internal/executor/` |
| `phasespec-adapter` | `sdd-status` adapter, lens-then-synthesis, artifact path | PR 1 | `go test -run TestPhaseSpecialist ./internal/phasespec` | N/A: mock `sdd-status` (`design.md:123`) | `internal/phasespec/` |
| `runtime-enforcement-accept` | `enforceRequiredSkills` + `packetDigest` lockstep with accept decode | PR 1 | `go test -run 'TestEnforceRequiredSkills\|TestValidateVersionedResult' ./internal/run ./internal/accept` | N/A: temp git fixtures | `internal/run/`, `internal/accept/` |
| `agent-prompts-assets` | Drop executor-named skills and dispatched hardcoded skill paths | PR 1 | `git grep -n 'lucind-archive\\|lucind-ultrafixer' .agents/skills plugin/claude-code/skills/lucind-ai/assets` | N/A: markdown | `.agents/skills/`, `plugin/claude-code/skills/lucind-ai/assets/` |
| `cli-packet-authoring` | `lucind-ai phase <name>`; admission derive/resolve/budget | PR 1 | `go test -run 'TestAdmit\|TestPhase' ./cmd/lucind-ai` | `lucind-ai phase <name>` on a fixture change | `cmd/lucind-ai/` |

## Wave plan

| Wave | Units | Parallel | Green on combined tree |
|------|-------|----------|------------------------|
| 1 | `result-envelope`, `skillset-engine`, `skillroots-resolution`, `lucindconfig-loader` | Yes | Yes: new packages + optional schema field; existing `TestSchemaJSON*` still pass |
| 2 | `packet-contract`, `executor-env`, `phasespec-adapter` | Yes | Yes: additive fields; `DigestBody` is identity when the heading is absent (`design.md:61-67`); `requestEnv` tests pin `LUCIND_READ_ONLY_PATHS` only; `phasespec` is a new package with mocks and no `cmd` import |
| 3 | `runtime-enforcement-accept`, `agent-prompts-assets` | Yes | Yes: digest/accept tests update in the same unit; markdown cannot fail `go test` |
| 4 | `cli-packet-authoring` | No | Yes: new `phase` case + fail-closed admission; existing valid batches still admit |

Same-wave pairs are component-boundary disjoint (`internal/packet/disjoint.go:8-22,24-48`). Wave 1: four distinct `internal/<pkg>/` dirs (`skillset` vs `skillroots` do not prefix-match). Wave 2: `packet`+`packetauthor` vs `executor` vs `phasespec`. Wave 3: `run`+`accept` vs `.agents/skills`+plugin assets.

Wave mapping onto the checklist: 1 → 1.1–1.3, 1.6; 2 → 1.4–1.5, 2.1–2.2, 3.2; 3 → 2.3–2.4, 4.1–4.2; 4 → 3.1, 3.3.

## Phase 1: Foundation & Core Models

- [x] 1.1-R RED: `TestDerive` / `TestDigestBodyElidesRequiredSkills` in new `internal/skillset/skillset_test.go` — union `derived ∪ stack ∪ adhoc`, `DefaultSkillBudget = 3`, `DigestBody` elides `## Required skills` through the next `## ` heading; bodies without that heading unchanged (`design.md:37-42,61-67,115`).
- [x] 1.1 Create `internal/skillset/skillset.go` with pure `Derive(sddPhase, laneRole string, stackSkills, adhocSkills []string) ([]string, error)`, `DefaultSkillBudget = 3`, and `DigestBody` (`design.md:37-42,103`).
- [x] 1.2-R RED: `TestResolveRootsLoadsSkillMarkdownAsData` in new `internal/skillroots/skillroots_test.go` — ordered roots, `~` → `$HOME`, missing-skill diagnostic; `SKILL.md` and skill-roots YAML load as data, never executed (`design.md:43-48,116,129`).
- [x] 1.2 Create `internal/skillroots/skillroots.go`: ordered search from `.lucind/skill-roots.yaml` (covered by `.gitignore:1-3`), tilde expansion, fail-closed diagnostics (`design.md:43-48,104`).
- [x] 1.3 Create `internal/lucindconfig/config.go` loading tracked `lucind.yaml` via `yaml.NewDecoder` + `KnownFields(true)`, stack skills per role, optional `skill_budget` (`design.md:43-48,105`). Prove: `go test -run TestLoadConfig ./internal/lucindconfig`.
- [x] 1.4 Modify `internal/packet/packet.go`: add `LaneRole`, `AdhocSkills`, `RequiredSkills`; parse `lane_role` against `{lens, synthesis, apply, verify, archive, ultrafixer, human}`; closed-validate `sdd_phase` when `lane_role` is present; parse `adhoc_skills`; declare `ErrInvalidLaneRole` beside `ErrInvalidReadOnly` (`packet.go:27-30,43-103,122-179`; `design.md:49-54,93`). Extend `TestParseObservabilityFrontmatter` (`packet_test.go:133-195`).
- [x] 1.5 Modify `internal/packetauthor/contract.go` adding `LaneRole`, `AdhocSkills`, `RequiredSkills` to `Contract` (`contract.go:45-56`; `design.md:16-21,94`).
- [x] 1.6 Modify `internal/result/result.schema.json` properties (`:7-165`) adding optional `skills_loaded` (`additionalProperties: false` at `:5`); add `SkillsLoaded` on `Envelope` (`result.go:103-116`); add Envelope↔schema reflection pin in `schema_test.go` (`:10-33`; `design.md:97-99`).

## Phase 2: Compilation, Execution & Enforcement

- [x] 2.1-R RED: `TestCompileDigestExcludesResolvedPaths` in `compile_test.go` — `Compile` digest identical across differing root prefixes and `~` expansions when names match; rendered bodies may differ (`design.md:61-67,130`; seam `compile.go:45,171-183`).
- [x] 2.1 Modify `compile.go`: `RequiredSkills` on `normalizedContract` (`:15-25`); `renderBody` emits `## Required skills` as `- <resolved-path>` between `## Hard stops` and `## Return`, omitted when empty (`:171-183`); artifact digest hashes `DigestBody(body)` at `:45` (names still hash via `contractJSON` at `:40`) (`design.md:61-67,95`).
- [x] 2.2 Modify `internal/executor/executor.go`: `RequiredSkills` on `Request` (`:50-75`); `requestEnv` strips inherited `LUCIND_REQUIRED_SKILLS` and injects a JSON array beside `LUCIND_READ_ONLY_PATHS` (`:20-39`; `design.md:96`).
- [x] 2.3-R RED: `TestPacketDigestExcludesResolvedPaths` and `TestEnforceRequiredSkills` in `internal/run` — `packetDigest` uses `DigestBody(p.Body)` and is stable across root prefixes; envelope `skills_loaded` shortfall demotes `lane.Done` to `lane.Deviated` (`internal/lane/status.go:11-17`; seams `run.go:489-491,722-729,876-904`; `design.md:130`).
- [x] 2.3-R RED: `TestPacketDigestExcludesResolvedPaths` and `TestEnforceRequiredSkills` in `internal/run` — `packetDigest` uses `DigestBody(p.Body)` and is stable across root prefixes; envelope `skills_loaded` shortfall demotes `lane.Done` to `lane.Deviated` (`internal/lane/status.go:11-17`; seams `run.go:489-491,722-729,876-904`; `design.md:130`).
- [ ] 2.3 Modify `run.go`: call `enforceRequiredSkills` beside `enforceAllowedPaths` (`:489-491`); implement beside `:876-904`; `packetDigest` hashes `DigestBody(p.Body)`, `LaneRole`, and sorted `RequiredSkills`/`AdhocSkills` each preceded by a decimal count (`:722-729`; `design.md:61-67,100`). Compiled packets still override with `p.Authoring.Digest` (`:657`). **Reopened by verify.md finding 1**: the `executor.Request{...}` construction at `run.go:445-453` never copies `p.RequiredSkills`/`p.AdhocSkills`, so `LUCIND_REQUIRED_SKILLS` is never injected on the real dispatch path.
- [x] 2.4-R RED: `TestAcceptDirtyPrimaryWithSkillsMatch` — accept succeeds on a dirty primary when candidate skills match frozen evidence; rejects on declared-skills mismatch (`design.md:131`; pattern `accept_test.go:140-164`; seam `accept.go:263-328`).
- [ ] 2.4 Modify `accept.go` decode struct (`:275-286`) adding `RequiredSkills` in lockstep with `packetDigest`; enforce `skills_loaded` correspondence in `validateVersionedEvidence` (`:263-328`); add mutation case in `authoring_evidence_test.go` (`:56-127`; `design.md:101-102`). No `internal/ledger/authoring.go` edits (`design.md:9,154`). **Reopened by verify.md finding 6**: Design Decision 8 required `LaneRole` in the decode struct alongside `RequiredSkills`; only `RequiredSkills` landed, so `packetDigest`'s `LaneRole` component is never cross-checked at acceptance.

## Phase 3: Admission & Phase Specialist

- [x] 3.1 Modify `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) to load `lucindconfig`, `skillset.Derive`, `skillroots` resolution, and fail-closed budget/missing-skill rejection before allocation (`design.md:22-27,107`).
- [x] 3.2-R RED: `TestSpecialistFailsClosedOnMalformedStatusJSON` in `internal/phasespec` — malformed `gentle-ai sdd-status --json` and CLI errors fail closed with no filesystem mutation and no extra dispatch (`design.md:133,123`).
- [ ] 3.2 Create `internal/phasespec/phasespec.go`: parse `sdd-status --json`, gate synthesis until required lens IDs are accepted and merged, write `openspec/changes/<change>/<phase>.md`. No `cmd` import; admission stays in `admitDispatchBatch` (`design.md:28-36,106`; `explore.md:282-285`). **Reopened by verify.md findings 3, 4, 8**: lens accepted/merged state from `gentle-ai sdd-status` is never ingested into `Synthesize` (`phasespec.Status` has no lens fields); canonical artifact filenames (`proposal.md`, `apply-progress.md`, `verify-report.md`, `archive-report.md`) diverge from the spec-named files (`propose.md` etc.); `isPhaseComplete` accepts a `done` status token without checking the artifact file exists on disk.
- [ ] 3.3 Modify `cli.go` string switch (`:142-168`) to register `lucind-ai phase <name>` and dispatch through `phasespec` (`design.md:28-36,107`). Accept still never demotes (`cli.go:684-687`). **Reopened by verify.md finding 2**: `phaseDispatch` (`cli.go:2452-2460`) calls `Adapter.Synthesize` with empty `LensStates`/`Content` and never calls `admitDispatchBatch`/`runDispatch` — `lucind-ai phase <name>` cannot dispatch a synthesis lane in production, only the already-complete no-op path is reachable.

## Phase 4: Skills, Templates & Documentation

- [x] 4.1 Modify `.agents/skills/lucind-*` to drop executor-named skills; do not author stub `lucind-archive` or `lucind-ultrafixer` (`design.md:55-60,108`).
- [x] 4.2 Modify `plugin/claude-code/skills/lucind-ai/assets/*.md` to drop hardcoded dispatched-skill `~/.claude/skills/...` paths and align templates with `## Required skills` (`design.md:108`). Do not edit `.opencode/agent/lucind-packet-author.md` (absent from File Changes; `:14-32` has no hardcoded skill paths). Fixed in both `plugin/claude-code` (first pass) and `plugin/opencode` (5.6); confirmed by re-verify, no remaining `~/.claude/skills/` matches in either tree.

## Remediation (post-verify round 1, see verify.md — all fixed, confirmed by dual re-verify judgment)

- [x] 5.1 `run.go`'s `executor.Request{...}` construction must copy `p.RequiredSkills`/`p.AdhocSkills` (or whatever the resolved required-skills field is named) so `requestEnv` actually injects `LUCIND_REQUIRED_SKILLS` on real dispatches, not just in `internal/executor` unit tests that set the field directly.
- [x] 5.2 `phaseDispatch` must populate `SynthesizeRequest.LensStates`/`Content` from parsed `sdd-status` JSON (or `phasespec.Status` must gain lens fields consumed by `Synthesize`), and `Synthesize` must actually call `admitDispatchBatch`/`runDispatch` for an incomplete phase — not only report success on the already-complete path.
- [x] 5.3 Reconcile canonical artifact filenames between `phasespec.go` (`proposal.md`, `apply-progress.md`, `verify-report.md`, `archive-report.md`) and the spec's named files (`propose.md` etc., `phase-specialist-dispatch/spec.md:7,13`) — pick one and update the other. Fixed against the delta spec's own naming; **but see 6.1 — this reopened a different, pre-existing mismatch against this repo's own live artifact convention.**
- [x] 5.4 Fix the `admitDispatchBatch` → `skillset.Derive` regression that fail-closes any legacy packet with a non-closed `sdd_phase` when `lane_role` is omitted, restoring the backward-compatibility scenario in `read-only-packet-schema`.
- [x] 5.5 Add `LaneRole` to the `accept.go` decode struct alongside `RequiredSkills`, per Design Decision 8's field-list lockstep with `packetDigest`.
- [x] 5.6 Drop hardcoded `~/.claude/skills/...` paths from `plugin/opencode/skills/lucind-ai/assets/*.md`, matching the `plugin/claude-code` sibling.
- [x] 5.7 `phasespec.isPhaseComplete` must check that the canonical artifact file exists on disk, not just that the status-JSON token is `done`.
- [x] 5.8 Re-run the verify sequence (`lucind-ai check` + dual `agy`/`cursor-agent` dispatch) once 5.1–5.7 land, and confirm the specific `file:line` citations above no longer reproduce. Done: both judges confirmed all 7 fixed with converging citations.

## Remediation round 2 (post-re-verify, see verify.md second-pass BLOCKED)

- [x] 6.1 `CanonicalArtifactFilename("propose")` returns `propose.md`, but this repo's own live `gentle-ai sdd-status` uses `proposal.md` (confirmed directly against this change's own `artifactPaths.proposal` in `sdd-status` output, and against `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:12` and existing packet templates' `allowed_paths`). `isPhaseComplete`'s disk check will never find this repo's real `proposal.md`. Fixed: `CanonicalArtifactFilename`/`CanonicalArtifactPath` now emit `proposal.md`; spec scenario text updated in `c226b6d`. Confirmed by dual re-verify (third pass).
- [x] 6.2 The specialist's dynamically-generated synthesis packet body (the `packetContent := fmt.Sprintf(...)` string in `cmd/lucind-ai/cli.go`'s dispatch branch) has no `## Required skills` section — dual delivery is env-only on this specific path, unlike the compiled-contract path (`internal/packetauthor/compile.go`'s `renderBody`, which is correct). Fixed: `resolveSynthesisSkillPaths` + interpolated `## Required skills` section added, exercised end-to-end by `TestPhaseSubcommandSpecialistPacketHasRequiredSkills`. Confirmed by dual re-verify (third pass).
- [x] 6.3 (Risk note, not required for this change) Lens eligibility on the production CLI path depends on status-JSON keys (`lenses`/`lensStates`/`phaseLenses`) with no checked-in live `gentle-ai sdd-status` contract sample proving they exist; only tests fabricate them. `design.md`'s own Testing Strategy names mock `sdd-status` as the intended E2E seam, so this is accepted residual schema-coupling risk — flag it in a follow-up change if a live contract sample becomes available, do not block this change on it. Re-confirmed still true and still non-blocking by the third-pass verify.
- [x] 6.4 Re-run the verify sequence once 6.1–6.2 land, and confirm neither reproduces. Done: third-pass dual verify PASSED, both judges converge with `file:line` citations.

## Minor follow-ups (non-blocking, noted by third-pass verify, not required for this change)

- [ ] 7.1 `TestPhaseSubcommandGatesPrematureSynthesis` (`cmd/lucind-ai/cli_test.go:5653-5657`) still negatively asserts the old `propose.md` filename is absent; should assert `proposal.md` is absent instead, to stay meaningful after the 6.1 rename.
- [ ] 7.2 `specs/phase-specialist-dispatch/spec.md`'s requirement prose (not its scenario, already updated) still generically says `<phase>.md`; tighten the wording to match the `proposal.md`/`spec.md`/`design.md`/`tasks.md`/`apply.md`/`verify.md`/`remediate.md`/`archive.md` convention actually implemented.
- [ ] 7.3 A synthesis packet already on disk in gitignored `.lucind/packets/` from before this fix is reused as-is (`cli.go:2447-2458`) and won't retroactively gain the `## Required skills` body section — env-var delivery still applies, so this is a cosmetic gap on stale local caches only, not a functional regression.

## Dependency order

| Task | Depends on | Why |
|---|---|---|
| 1.1-R, 1.1 | — | Pure `skillset` |
| 1.2-R, 1.2 | — | Independent `skillroots` |
| 1.3 | — | Independent `lucindconfig` |
| 1.4, 1.5, 1.6 | — | Additive fields/schema |
| 2.1-R, 2.1 | 1.1, 1.5 | `DigestBody` + `Contract.RequiredSkills` |
| 2.2 | — | Independent env injection |
| 2.3-R, 2.3 | 1.1, 1.4, 1.6 | `DigestBody`, Packet fields, `Envelope.SkillsLoaded` |
| 2.4-R, 2.4 | 1.1, 1.4, 1.6 | Decode struct mirrors `packetDigest`; correspondence uses `SkillsLoaded` |
| 3.1 | 1.1, 1.2, 1.3, 1.4, 2.1 | Admission calls Derive/roots/config and Compile |
| 3.2-R, 3.2 | — | New package; mock `sdd-status`; no `cmd` import |
| 3.3 | 3.1, 3.2 | CLI calls `phasespec` then `admitDispatchBatch` |
| 4.1 | — | Independent markdown |
| 4.2 | 2.1 | Templates match `renderBody` |

## Threat-matrix RED tests

Push state (`design.md:132`) is N/A: no RED task.

| Adversarial case | RED task | Asserts | Precedes |
|---|---|---|---|
| Documentation-like paths (`design.md:129`) | 1.2-R | `SKILL.md` and skill-roots YAML load as data; no execution | 1.2, 3.1 |
| Git repository selection (`design.md:130`) | 1.1-R, 2.1-R, 2.3-R | `DigestBody` / `Compile` digest / `packetDigest` identical across root prefixes and `~` | 1.1, 2.1, 2.3 |
| Commit state (`design.md:131`) | 2.4-R | Accept on dirty primary when skills match; reject on mismatch | 2.4 |
| PR commands (`design.md:133`) | 3.2-R | Malformed status JSON / CLI errors fail closed, no side effects | 3.2, 3.3 |

## Requirement traceability

| Requirement | Tasks |
|---|---|
| Deterministic multi-tier derivation | 1.1, 1.3, 3.1 |
| Root resolution and fail-closed admission | 1.2, 3.1 |
| Result envelope skills loaded declaration | 1.6, 2.3, 2.4 |
| Specialist sequencing and canonical artifact generation | 3.2, 3.3 |
| Versioned Contract and Late Target Binding | 1.5, 2.1, 2.4 |
| Frozen Authored Candidate Evidence | 2.2, 2.3 |
| Fail-Closed Mechanical Criteria | 2.3, 2.4 |
| Extended packet frontmatter parsing | 1.4, 4.1, 4.2 |

## Open questions

Lens A/B single-writer vs three-lens fan-out: resolved by `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:43` (skill governs content; packet governs topology). Not a gap.

Lens B Wave 3+4 collapse: declined. Both waves are green alone; markdown vs CLI are disjoint.
