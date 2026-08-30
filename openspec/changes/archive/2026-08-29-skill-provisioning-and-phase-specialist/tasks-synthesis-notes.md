# Tasks Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

## Unresolved Contradictions

None.

## Coverage Gaps

No waves merged. Each combined wave was judged green on its own under `Integrate` (`internal/run/integrate.go:50-59`):

- Wave 1: new packages plus an optional schema field. Existing `TestSchemaJSON*` (`internal/result/schema_test.go:10-33`) still pass.
- Wave 2: additive struct fields. `DigestBody` is identity when `## Required skills` is absent (`design.md:61-67`), so `compile.go:45` switching to `DigestBody(body)` does not change existing artifact bytes. `requestEnv` tests pin `LUCIND_READ_ONLY_PATHS` only (`internal/executor/read_only_paths_test.go:1-50`). `phasespec` is a new package with mocked `sdd-status` and no `cmd` import.
- Wave 3: `packetDigest` (`internal/run/run.go:722-729`) and the accept decode struct (`internal/accept/accept.go:275-286`) stay in `runtime-enforcement-accept` (Decision 8, `design.md:61-66`). Digest/accept tests update in that same unit. Markdown cannot fail `go test`.
- Wave 4: new `phase` case plus fail-closed admission. Existing valid batches still admit.

Lens B's Wave 3+4 collapse question is declined for the same reason: both waves are green alone and path-disjoint.

The `sdd-tasks` skill's 530-word size budget is superseded by this packet's 1800-word budget (packet wins on execution). Checklist contract from the skill is met: forecast fields, plain-text guard lines, work-unit columns (focused test, runtime harness, rollback), specific/actionable/verifiable/small tasks, and RED-before-GREEN for the four Applicable threat rows with none for push state.

No spine item was missing from the drafts after arbitration. Lens C's "Verification Gaps: None" stands after splitting two over-broad proving commands (see Dropped Citations).

## Dropped Citations

**Dropped claims** (removed from `tasks.md`):

- Lens C Acceptance Evidence: `go test -run TestCompile ./internal/packetauthor` (`compile_test.go:1-60`) proves `requestEnv` injects `LUCIND_REQUIRED_SKILLS`. `TestCompileDeterministicReplayAndCanonicalOrdering` and `TestCompileDigestChangesForEveryRelevantInputClass` cover Compile replay and digest sensitivity only. `requestEnv` lives in `internal/executor/executor.go:20-39`. Executor proof is a separate command on `./internal/executor`.
- Lens C: `go test -run TestResolveRoots ./internal/skillroots` proves `lucind.yaml` + `KnownFields(true)`. That command's package is `skillroots` only. Config proof is `go test -run TestLoadConfig ./internal/lucindconfig`.
- Lens C RED row: `TestVerifierTreatsDocumentationLikeFilesAsScopeOnly` (`internal/accept/accept_test.go:125-138`) asserts loading `SKILL.md` or skill-roots YAML as data. The test touches `requirements.txt`, `CMakeLists.txt`, `guide.md`, `guide.mdx`, `README.sh` and checks accept does not execute them. It does not load `SKILL.md`. The RED task is a new `skillroots` test (`1.2-R`).
- Lens A 4.2: modify `.opencode/agent/lucind-packet-author.md` to remove hardcoded `~/.claude/skills/...` paths. `:14-32` is the JSON output contract with no skill paths. File Changes (`design.md:108`) lists `.agents/skills/lucind-*` and `plugin/.../assets/*.md` only.
- Lens A 2.1: hash `DigestBody` in **contract and** artifact digests. `compile.go:40` hashes `contractJSON` (names via `normalizedContract`). `:45` hashes `body` and is the `DigestBody` seam (`design.md:61-67`).
- Lens A dependency: 2.3 depends on 2.2. `enforceRequiredSkills` sits beside `enforceAllowedPaths` (`run.go:489-491,876-904`) and compares envelope `skills_loaded` to packet names, not `executor.Request`. 2.2 remains independent.
- Lens C: `result.schema.json:7-165` is `additionalProperties: false`. That keyword is at line 5. `:7-165` is `properties`, where `skills_loaded` is added.
- Lens B: `run.go:489-491` is `runDispatch` invoking `enforceRequiredSkills`. That hunk is Execute after `decideStatus`, currently `enforceAllowedPaths` only. `enforceRequiredSkills` does not exist yet. Kept as the call-site seam, not as a CLI function.

**Retargeted** (claim kept, citation corrected):

- Lens C `TestAcceptDirtyPrimaryWithSkillsMatch` citing `accept_test.go:140-164`: that function is `TestVerifierUsesFrozenDetachedCandidateDespitePrimaryState` (dirty primary, no skills). Kept as the pattern; the new test is `2.4-R`.
- Lens C `TestEnforceRequiredSkills` citing `scope_test.go:15-110`: that file is `TestEnforceAllowedPathsUsesCanonicalFourWayCopyAwareChanges`. Sibling seam is `run.go:876-904`. Prospective `TestEnforceRequiredSkills` stays the proving command.
- Lens C `TestCompileDigestExcludesResolvedPaths` citing `compile.go:45,171-183` and `run.go:722-729`: those are production seams. The RED test belongs in `compile_test.go` / `internal/run` (`2.1-R`, `2.3-R`).
- Lens C `TestSpecialistFailsClosedOnMalformedStatusJSON` citing `cli.go:142-168`: that range is the subcommand switch. The RED test belongs in `internal/phasespec` (`3.2-R`).
- Lens C `TestParseObservabilityFrontmatter` (`packet_test.go:133-195`) already parses `lane_role` / `adhoc_skills`. Today it covers `sdd_phase`, `fanout_group`, `skill`. Command kept as the extension seam for 1.4.
- Lens C `TestSchema` (`schema_test.go:10-33`) already pins `skills_loaded`. Today: `TestSchemaJSONParsesAsJSON` and `TestSchemaJSONReturnsDefensiveCopy`. Command kept; 1.6 adds the reflection pin.

**Prospective test symbols** (packages or functions do not exist today; proving commands kept). `internal/skillset/`, `internal/skillroots/`, `internal/lucindconfig/`, and `internal/phasespec/` are File Changes "Create" rows (`design.md:103-106`), not Modify:

- `TestDerive`, `TestDigestBodyElidesRequiredSkills` in `internal/skillset`
- `TestResolveRoots`, `TestResolveRootsLoadsSkillMarkdownAsData` in `internal/skillroots`
- `TestLoadConfig` in `internal/lucindconfig`
- `TestPhaseSpecialist`, `TestSpecialistFailsClosedOnMalformedStatusJSON` in `internal/phasespec`
- `TestCompileDigestExcludesResolvedPaths` in `internal/packetauthor` (package exists; symbol does not)
- `TestEnforceRequiredSkills`, `TestPacketDigestExcludesResolvedPaths` in `internal/run` (package exists; symbols do not)
- `TestAcceptDirtyPrimaryWithSkillsMatch` in `internal/accept` (package exists; symbol does not)

**Verified** (unique manifest + prose citations opened; claim supported as current code or as the named seam): `cli.go:142-168`, `cli.go:684-687`, `packet_authoring.go:32-54`, `accept.go:263-328`, `accept.go:275-286`, `authoring_evidence_test.go:56-127`, `dag/parse.go:40-60`, `executor.go:20-39`, `executor.go:50-75`, `ledger/authoring.go:44-75` (existing Freeze/Decode; not a File Changes row), `disjoint.go:8-22`, `disjoint.go:24-48`, `packet.go:43-103`, `packet.go:122-179`, `packet.go:27-30` (error sentinels), `compile.go:15-25`, `compile.go:40`, `compile.go:45`, `compile.go:171-183`, `contract.go:45-56`, `result.go:103-116`, `result.schema.json:5`, `result.schema.json:7-165`, `schema_test.go:10-33`, `integrate.go:50-59`, `run.go:489-491`, `run.go:657`, `run.go:722-729`, `run.go:876-904`, `lane/status.go:11-17`, `read_only_paths_test.go:1-50`, `archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26`, `archive/2026-08-20-apply-dag-dispatch/design.md:19-21`, `explore.md:282-285`, `fan-out.md:43`, `.gitignore:1-3`, and `design.md` ranges 16-21, 22-27, 28-36, 37-42, 43-48, 49-54, 55-60, 61-67, 89-108, 93-108, 97-99, 101-102, 103-106, 107-108, 110-123, 115, 116, 123, 125-133, 129, 130, 131, 132, 133, 9, 154.

`.opencode/agent/lucind-packet-author.md:14-32` was opened and does not support a modify task; see dropped claims.

## Decomposition Divergence

All three drafts independently used a four-stage structure over the same File Changes set (`design.md:89-108`). That is corroboration, not identity.

- **Lens A (authoritative):** 4 phases, 15 production tasks. Phase 1 = skillset, skillroots, lucindconfig, packet, contract, result. Phase 2 = compile, executor, run, accept. Phase 3 = admission, phasespec, CLI. Phase 4 = agent skills and plugin assets.
- **Lens B:** 10 work units, 4 waves. Wave 1 matches A's foundation packages plus result, but moves packet/contract to Wave 2 with compile (`packet-contract`). `phasespec-adapter` is Wave 2 (library) rather than A's Phase 3; CLI admission is Wave 4. `runtime-enforcement-accept` correctly keeps `run.go` and `accept.go` in one unit. `agent-prompts-assets` is Wave 3, parallel with that unit. Content kept: unit table, wave plan, sidecar recommendation, disjointness. Content not copied wholesale: Wave 3+4 collapse (declined); `plugin/` allowed_paths narrowed to `plugin/claude-code/skills/lucind-ai/assets/`.
- **Lens C:** 4 operational tiers. Tier 1 = skillset/roots/config. Tier 2 = packet, packetauthor, executor, result, run. Tier 3 = accept **and ledger**. Tier 4 = phasespec and CLI. Phase 4 docs are absent from C's assumed decomposition. Content kept: forecast values (`single-pr` / `size:exception`, review budget 10000), four Applicable RED rows, N/A push-state omission, proving commands (after the splits above). Content not turned into tasks: a ledger work unit (`design.md:9,154` forbids `AuthoringEvidence` shape/version change). C's "batch admission" proof stays on `cli-packet-authoring` (Wave 4), not on the Wave 2 `phasespec` library.

A's 3.2-depends-on-3.1 is adjusted in `tasks.md` because `internal/phasespec` cannot import `cmd` (`admitDispatchBatch` is package `main`). 3.2 is a mock-tested library; 3.3 depends on 3.1 and 3.2. That is layering, not a substitute decomposition.

Nothing from B or C that mapped to no A task entered `tasks.md` except the four threat-matrix RED tasks required by the skill and the packet spine, attached immediately before A's production tasks.
