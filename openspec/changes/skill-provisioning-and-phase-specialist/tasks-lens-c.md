# Tasks Lens C — Proof & Review Burden: Skill Provisioning and the SDD Phase Specialist

## Assumed decomposition

The work decomposes across four sequential operational phases: (1) Core skill derivation, root resolution, and config foundation in `internal/skillset`, `internal/skillroots`, and `internal/lucindconfig`; (2) Packet authoring, executor environment injection, envelope schema update, and runner status demotion in `internal/packet`, `internal/packetauthor`, `internal/executor`, `internal/result`, and `internal/run`; (3) Candidate evidence verification and acceptance enforcement in `internal/accept` and `internal/ledger`; and (4) SDD phase specialist orchestration and CLI command dispatch in `internal/phasespec` and `cmd/lucind-ai`. Proof obligations and test tasks attach to these four operational tiers.

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~1,200–1,600 lines (4 new packages: ~700–900 lines; 11 modified files: ~500–700 lines) |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | single PR (size:exception pre-accepted) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

The line estimate is grounded in `openspec/changes/skill-provisioning-and-phase-specialist/design.md:89-108`, comprising 4 new packages (`internal/skillset/`, `internal/skillroots/`, `internal/lucindconfig/`, `internal/phasespec/` contributing ~700–900 lines including unit/E2E test suites) and 11 modified files (`internal/packet/packet.go`, `internal/packetauthor/contract.go`, `internal/packetauthor/compile.go`, `internal/executor/executor.go`, `internal/result/result.go`, `internal/result/result.schema.json`, `internal/result/schema_test.go`, `internal/run/run.go`, `internal/accept/accept.go`, `internal/accept/authoring_evidence_test.go`, `cmd/lucind-ai/cli.go`, `cmd/lucind-ai/packet_authoring.go`, `.agents/skills/lucind-*` contributing ~500–700 lines). The preflight pre-accepts `single-pr` delivery with `size:exception` up to 10,000 lines.

## RED Tests from the Threat Matrix

| Threat row | Applicable | RED test | Asserts | Production task it precedes |
|---|---|---|---|---|
| Documentation-like paths (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:129`) | Applicable | `TestVerifierTreatsDocumentationLikeFilesAsScopeOnly` (`internal/accept/accept_test.go:125-138`) | Loading `SKILL.md` or skill-roots YAML operates strictly as structured data without execution; doc-like changes remain observation-only. | Skill roots loading and batch admission (`internal/skillroots/`, `cmd/lucind-ai/packet_authoring.go:32-54`). |
| Git repository selection (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:130`) | Applicable | `TestCompileDigestExcludesResolvedPaths` (`internal/packetauthor/compile.go:45,171-183`, `internal/run/run.go:722-729`) | `Compile` digest and `packetDigest` remain identical across differing root prefixes and `~` expansions even when rendered bodies differ in resolved paths. | `skillset.DigestBody` and packet digest computation (`internal/skillset/`, `internal/packetauthor/compile.go:45`, `internal/run/run.go:722-729`). |
| Commit state (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:131`) | Applicable | `TestAcceptDirtyPrimaryWithSkillsMatch` (`internal/accept/accept.go:263-328`, `internal/accept/accept_test.go:140-164`) | Accept succeeds on a dirty primary repository when candidate changes match frozen evidence; rejects candidate when declared skills mismatch. | Acceptance verifier candidate evidence validation (`internal/accept/accept.go:263-328`). |
| Push state (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:132`) | N/A: no ref mutation | N/A | N/A | N/A |
| PR commands (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:133`) | Applicable | `TestSpecialistFailsClosedOnMalformedStatusJSON` (`cmd/lucind-ai/cli.go:142-168`) | Malformed `gentle-ai sdd-status` JSON or CLI invocation errors fail closed without mutating filesystem state or triggering redundant dispatches. | Phase specialist `sdd-status` adapter and synthesis sequencing (`internal/phasespec/`, `cmd/lucind-ai/cli.go:142-168`). |

## Acceptance Evidence

| Task | Proving command | What a pass proves | What it does not prove |
|---|---|---|---|
| Packet Frontmatter & Validation (`internal/packet/packet.go:122-179`) | `go test -run TestParseObservabilityFrontmatter ./internal/packet` (`internal/packet/packet_test.go:133-195`) | Parses `lane_role`, `adhoc_skills`, `sdd_phase`; enforces closed set validation on `lane_role` and `sdd_phase`; preserves backward compatibility when `lane_role` is omitted (`internal/packet/packet.go:122-179`). | Does not prove skill derivation union or batch admission enforcement. |
| Multi-Tier Skill Derivation & Digest (`internal/skillset/`) | `go test -run TestDerive ./internal/skillset` (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:115`) | Pure `Derive` deterministically computes `derived ∪ stack ∪ adhoc`, deduplicates names, enforces `DefaultSkillBudget = 3`, and `DigestBody` elides `## Required skills` heading (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:37-42,61-67`). | Does not prove disk resolution of `SKILL.md` files or runtime process environment injection. |
| Ordered Root Resolution & Config (`internal/skillroots/`, `internal/lucindconfig/`) | `go test -run TestResolveRoots ./internal/skillroots` (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:116`) | Ordered resolution across `.lucind/skill-roots.yaml`, tilde expansion to `$HOME`, `lucind.yaml` parsing with `KnownFields(true)`, and fail-closed diagnostics for unresolvable skills (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:43-48`). | Does not prove child process execution or CLI dispatch. |
| Contract Compilation & Executor Env (`internal/packetauthor/compile.go:15-25,45,171-183`, `internal/executor/executor.go:20-39`) | `go test -run TestCompile ./internal/packetauthor` (`internal/packetauthor/compile_test.go:1-60`) | `normalizedContract` embeds `required_skills`, `renderBody` renders `## Required skills` paths, `Compile` hashes `DigestBody`, and `requestEnv` injects `LUCIND_REQUIRED_SKILLS` (`internal/packetauthor/compile.go:45,171-183`, `internal/executor/executor.go:20-39`). | Does not prove agent loading inside external subagents or CLI tools. |
| Envelope Schema & Reflection Pin (`internal/result/result.go:103-116`, `internal/result/result.schema.json:7-165`, `internal/result/schema_test.go:10-33`) | `go test -run TestSchema ./internal/result` (`internal/result/schema_test.go:10-33`) | JSON schema validates optional `skills_loaded` string array, rejects unexpected fields (`additionalProperties: false`), and matches `Envelope` struct (`internal/result/result.go:103-116`, `internal/result/result.schema.json:7-165`). | Does not prove runner status demotion logic. |
| Runtime Status Demotion & Digest Pin (`internal/run/run.go:489-491,722-729,876-904`) | `go test -run TestEnforceRequiredSkills ./internal/run` (`internal/run/scope_test.go:15-110`, `internal/run/run.go:876-904`) | `enforceRequiredSkills` demotes missing `skills_loaded` entries from `lane.Done` to `lane.Deviated`; `packetDigest` pins field list order (`internal/run/run.go:489-491,722-729,876-904`). | Does not prove ledger candidate freezing or acceptance verification. |
| Acceptance Correspondence & Evidence Decode (`internal/accept/accept.go:263-328,275-286`, `internal/ledger/authoring.go:44-75`) | `go test -run TestValidateVersionedResult ./internal/accept` (`internal/accept/authoring_evidence_test.go:56-127`) | Verifier enforces exact correspondence between declared required skills and candidate evidence, rejects tampered/omitted skills, and decodes legacy v1 evidence (`internal/accept/accept.go:263-328`, `internal/ledger/authoring.go:44-75`). | Does not prove multi-lens orchestration sequencing. |
| SDD Phase Specialist Orchestration (`internal/phasespec/`, `cmd/lucind-ai/cli.go:142-168`, `cmd/lucind-ai/packet_authoring.go:32-54`) | `go test -run TestPhaseSpecialist ./internal/phasespec` (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:123`, `cmd/lucind-ai/cli.go:142-168`) | Ingests `gentle-ai sdd-status`, waits for all lens candidates to merge, admits batch via `admitDispatchBatch`, and writes canonical artifact to `openspec/changes/<change>/<phase>.md` (`cmd/lucind-ai/cli.go:142-168`, `cmd/lucind-ai/packet_authoring.go:32-54`). | Does not prove internal execution of `gentle-ai` external binary. |

## Verification Gaps

None. Every capability specification scenario across all 8 deltas is verifiable via unit tests, integration test fixtures, or mock-backed CLI tests identified in `openspec/changes/skill-provisioning-and-phase-specialist/design.md:110-123`.

## Open Questions

- [ ] None.

## Citation Manifest

| Citation | Claim supported |
|---|---|
| `cmd/lucind-ai/cli.go:142-168` | Subcommand dispatch string switch supporting `lucind-ai phase` execution. |
| `cmd/lucind-ai/cli.go:684-687` | Acceptance verifier CLI dispatch and receipt rendering seam. |
| `cmd/lucind-ai/packet_authoring.go:32-54` | Pre-worktree batch admission validating skill resolution and budget constraints. |
| `internal/accept/accept.go:263-328` | Versioned candidate authoring evidence and required skills correspondence verification. |
| `internal/accept/accept.go:275-286` | Decoded contract struct schema matching normalized contract fields. |
| `internal/accept/accept_test.go:125-138` | Verification test seam confirming documentation-like paths remain observation-only. |
| `internal/accept/accept_test.go:140-164` | Acceptance verification behavior against dirty primary working tree state. |
| `internal/accept/authoring_evidence_test.go:56-127` | Correspondence validation and frozen candidate evidence mutation test fixture. |
| `internal/executor/executor.go:20-39` | `requestEnv` helper stripping inherited values and injecting `LUCIND_REQUIRED_SKILLS`. |
| `internal/ledger/authoring.go:44-75` | `FreezeAuthoringEvidence` and `DecodeAuthoringEvidence` contract roundtrip. |
| `internal/packet/packet.go:43-103` | `Packet` struct fields for `LaneRole`, `AdhocSkills`, and `RequiredSkills`. |
| `internal/packet/packet.go:122-179` | Frontmatter parsing and closed-set validation for `lane_role` and `sdd_phase`. |
| `internal/packet/packet_test.go:133-195` | Frontmatter parsing test suite for optional observability keys. |
| `internal/packetauthor/compile.go:15-25` | `normalizedContract` struct definition holding contract fields. |
| `internal/packetauthor/compile.go:45` | Artifact compilation and body digest hashing seam. |
| `internal/packetauthor/compile.go:171-183` | `renderBody` implementation emitting `## Required skills` path block. |
| `internal/packetauthor/compile_test.go:1-60` | Deterministic compilation replay and contract hashing tests. |
| `internal/packetauthor/contract.go:45-56` | `Contract` struct definition for authored packet inputs. |
| `internal/result/result.go:103-116` | `Envelope` struct definition with optional `SkillsLoaded` field. |
| `internal/result/result.schema.json:7-165` | Result envelope JSON schema property definitions and validation constraints. |
| `internal/result/schema_test.go:10-33` | Result envelope schema JSON parsing and defensive copy tests. |
| `internal/run/run.go:489-491` | Runner status enforcement invocation in execution lifecycle. |
| `internal/run/run.go:722-729` | `packetDigest` field list composition including `LaneRole` and skill arrays. |
| `internal/run/run.go:876-904` | `enforceAllowedPaths` and runner status demotion seam. |
| `internal/run/scope_test.go:15-110` | Runner path and scope enforcement test cases. |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:89-108` | File changes table defining package additions and modifications for line estimation. |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:110-123` | Testing strategy and test seams across unit, integration, and E2E layers. |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:125-133` | Threat matrix rows, adversarial cases, design responses, and planned RED tests. |
