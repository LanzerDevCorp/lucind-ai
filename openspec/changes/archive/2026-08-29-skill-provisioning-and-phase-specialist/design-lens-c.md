# Design Lens C — Failure, Test & Rollback: Skill Provisioning and the SDD Phase Specialist

## Assumed architecture

Three-tier derivation computes `derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)` capped at budget 3, resolving names via `.lucind/skill-roots.yaml` with tilde expansion. Dual delivery provides prompt `## Required skills` and `LUCIND_REQUIRED_SKILLS` environment variables, stored inside `AuthoringEvidence.Contract` JSON without ledger migrations. Two-site enforcement demotes shortfalls to `lane.Deviated` in `run` and re-verifies frozen evidence in `accept`. A phase specialist sequences DAG execution via `gentle-ai sdd-status` without intercepting gentle-ai authority.

## Decision 1 — Missing archive/ultrafixer child skills (resolves Open Question 2)

**Choice**: Declare `archive` and `ultrafixer` roles derived-empty for phase child skills (deriving only base `lucind-executor`), letting admission pass with zero required child skills for them.
**Alternatives considered**: Create stub `sdd-archive` or `lucind-ultrafixer` files. Operational risk: Out-of-scope skill authoring (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:20-27`), placeholder debt, and rejection when local stubs are missing.
**Rationale**: `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) fails closed before worktree allocation if any required skill cannot be resolved. Because neither `lucind-archive` nor `lucind-ultrafixer` exists in `.agents/skills/` or external roots, requiring stubs breaks archive/ultrafixer dispatches on environments lacking stubs. Declaring them derived-empty guarantees admission viability while preserving fail-closed enforcement for existing phase skills.
**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32-54`

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Unit | Closed-set `lane_role` and `sdd_phase` validation | Parse packets with valid/invalid roles and omitted role backward compatibility | `internal/packet/packet.go:122-179` |
| Unit | 3-tier skill derivation and budget cap | Compute derived, stack (`lucind.yaml`), ad-hoc sets; assert reject over budget 3 | new seam required (`internal/skillset/skillset_test.go`) |
| Unit | Root resolution with tilde expansion | Resolve relative, absolute, and `~` paths from `.lucind/skill-roots.yaml` | new seam required (`internal/skillroots/skillroots_test.go`) |
| Unit | Contract compilation and rendered body | Assert `required_skills` in `normalizedContract`, stable digest, `## Required skills` body | `internal/packetauthor/compile.go:171-183` |
| Unit | Envelope schema and reflection sync | Assert `skills_loaded` field in `Envelope` matches `result.schema.json` | `internal/result/schema_test.go:10-33` |
| Integration | Shortfall demotion via `enforceRequiredSkills` | Assert `lane.Deviated` when `skills_loaded` omits a required skill | `internal/run/run.go:876-904` |
| Integration | Strict acceptance correspondence | Assert `validateVersionedEvidence` rejects candidates with mutated or missing `required_skills` | `internal/accept/authoring_evidence_test.go:56-127` |
| Integration | Dual environment delivery | Assert `requestEnv` populates `LUCIND_REQUIRED_SKILLS` alongside `LUCIND_READ_ONLY_PATHS` | `internal/executor/read_only_paths_test.go:1-50` |
| Integration | Evidence hash stability | Verify `FreezeAuthoringEvidence`/`DecodeAuthoringEvidence` roundtrip with new contract fields | `internal/ledger/authoring.go:44-75` |
| E2E | Specialist orchestration | Propose fan-out: ingest `gentle-ai sdd-status`, admit batch, wait for lens merge, synthesize artifact | new seam required (`internal/phasespec/specialist_test.go`) |

*Mechanism distinction*: `enforceAllowedPaths` (`internal/run/run.go:876-904`) inspects git diffs via `candidatechange.Collect`, whereas `enforceRequiredSkills` compares derived `required_skills` against envelope `skills_loaded`.

## Test Seams

**Existing Seams**:
- *Acceptance mutation fixture* (`internal/accept/authoring_evidence_test.go:56-127`, `internal/accept/accept_test.go:24-65`): Mutates `Contract` JSON to verify rejection in `validateVersionedEvidence` (`internal/accept/accept.go:263-328`).
- *Isolated acceptance verification* (`internal/accept/accept_test.go:24-65`): Tests candidate tree verification (`internal/accept/accept_test.go:140-164`) and documentation scope (`internal/accept/accept_test.go:125-138`).
- *Post-dispatch evaluation* (`internal/run/run.go:876-904`): Demotes shortfalls to `lane.Deviated` (`internal/lane/status.go:11-17`).
- *Subprocess environment injection* (`internal/executor/executor.go:20-39`, `internal/executor/read_only_paths_test.go:1-50`): Tested via executor stubs.
- *Contract compiler* (`internal/packetauthor/compile.go:15-25,171-183`, `internal/packetauthor/compile_test.go:1-60`): Tests deterministic compilation.
- *Admission gate* (`cmd/lucind-ai/packet_authoring.go:32-54`): Validates batch admission before worktree allocation.

**New Seams Required**:
- `internal/skillset`: Unit tests for 3-tier derivation and budget cap (`internal/skillset/skillset_test.go`).
- `internal/skillroots`: Unit tests for path resolution, tilde expansion, missing diagnostics (`internal/skillroots/skillroots_test.go`).
- `internal/phasespec`: Fixtures mocking `gentle-ai sdd-status` CLI output (`internal/phasespec/specialist_test.go`).

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | Applicable | Load `SKILL.md` and `.lucind/skill-roots.yaml` as static data; external skill content hashing (`HashDir`) is observation-only metadata, never a blocking gate (`internal/skillcontent/skillcontent.go:1-28,73-100`) | Assert `admitDispatchBatch` and `HashDir` process `SKILL.md` without shell execution or blocking admission gates |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | Machine-local roots resolve locally; only canonical skill names enter packet digest (`internal/packetauthor/compile.go:15-25`) | Assert identical digest across differing root path prefixes and `~` expansions |
| Commit state | staged, `commit -a`, empty index | Applicable | `enforceRequiredSkills` reads `skills_loaded`; `validateVersionedEvidence` (`internal/accept/accept.go:263-328`) verifies candidate tree independently of dirty primary state (`internal/accept/accept_test.go:140-164`) | Verify acceptance passes detached candidate tree despite dirty primary index when skills match, rejects on mismatch |
| Push state | tracking branch, first push, explicit refspec | N/A: No remote push commands or ref mutations | N/A | N/A |
| PR commands | explicit `--head`, environment prefix, composed commands | Applicable | Specialist executes `gentle-ai sdd-status` via read-only non-intercepting subprocess; never mutates PR state | Verify specialist handles malformed `gentle-ai sdd-status` JSON or CLI errors gracefully |

## Rollback and Additivity

**Choice**: Changes are additive; rollback proceeds in reverse dependency:
1. Stop rendering `## Required skills` in packet bodies (`internal/packetauthor/compile.go:171-183`) and stop deriving skills in `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`).
2. Revert `enforceRequiredSkills` demotion (`internal/run/run.go:876-904`) and acceptance validation (`internal/accept/accept.go:263-328,275-286`).
3. Revert envelope schema and struct fields (`internal/result/result.schema.json:1-165`, `internal/result/result.go:103-116`).
4. Revert new packages (`internal/phasespec/`, `internal/skillset/`, `internal/skillroots/`, `internal/lucindconfig/`).

**Alternatives considered**: Ledger migration v11 and bumping `AuthoringEvidenceVersion` to `v2`. Operational risk: Ledger incompatibility, broken hash verification on frozen candidate rows (`internal/ledger/authoring.go:62-75`), and active run deadlock.
**Rationale**: Embedding `lane_role` and `required_skills` in `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:20-42`) preserves version `lane-authoring-evidence/v1` (`internal/ledger/authoring.go:14`) and table schema v10 (`internal/ledger/schema.go:425-445,584-592`). Pre-existing rows decode identically without mutation (`internal/ledger/acceptance.go:96-103`).

## Out of Scope

- Modifying `AuthoringEvidence` struct or bumping `AuthoringEvidenceVersion` (`internal/ledger/authoring.go:14,20-42`).
- SQLite migrations or historical candidate row backfills (`internal/ledger/schema.go:425-445,584-592`).
- Specialist-side skill selection or tool usage (`.opencode/agent/lucind-packet-author.md:1-8`).
- Intercepting `gentle-ai` execution authority or reading `openspec/config.yaml` in Go (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:20-27`).
- Authoring skill markdown content or enforcing external skill content hash as a blocking gate (`internal/skillcontent/skillcontent.go:1-28`).
- Multi-PR release splitting (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:20-27`).
- CLI failure-guidance banners (`cmd/lucind-ai/cli.go:699,737,759,2004`).
- Derivation algorithm internals, budget shape, and specialist CLI syntax (owned by Lens A).
- `lane_role` closed-set enumeration details and `skills_loaded` JSON syntax (owned by Lens B).

## Open Questions

- [ ] Conflict between upstream `sdd-design` skill (800-word budget, Engram persistence, phase-summary return block) and Lucind-AI packet constraints (1000-word budget, `.lucind/result.json` return envelope): this lane followed packet precedence as instructed.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:684-687` | `accept` is an error-only receipt verifier that returns exit code 1 on verification failure and never demotes |
| `cmd/lucind-ai/cli.go:699` | Failure guidance banner for acceptance promotion out of scope |
| `cmd/lucind-ai/cli.go:737` | Failure guidance banner out of scope |
| `cmd/lucind-ai/cli.go:759` | Failure guidance banner out of scope |
| `cmd/lucind-ai/cli.go:2004` | Failure guidance banner out of scope |
| `cmd/lucind-ai/packet_authoring.go:32-54` | `admitDispatchBatch` is the fail-closed admission gate rejecting batches if required skills are missing |
| `internal/accept/accept.go:263-328` | `validateVersionedEvidence` re-verifies frozen contract evidence against result envelope and rejects mismatches |
| `internal/accept/accept.go:275-286` | Duplicated decode struct in `accept.go` that must gain `RequiredSkills` in the same commit to avoid unverified frozen fields |
| `internal/accept/accept_test.go:24-65` | `newVerifierFixture` creates an isolated test fixture with fake ledger candidate rows and check scripts |
| `internal/accept/accept_test.go:125-138` | `TestVerifierTreatsDocumentationLikeFilesAsScopeOnly` proves documentation-like paths are treated as scope-only and not executed |
| `internal/accept/accept_test.go:140-164` | `TestVerifierUsesFrozenDetachedCandidateDespitePrimaryState` proves acceptance verifies detached candidate tree independently of primary working tree state |
| `internal/accept/authoring_evidence_test.go:12-44` | Frozen authoring evidence test fixture verifying legacy and versioned candidate roundtrips |
| `internal/accept/authoring_evidence_test.go:56-127` | `TestValidateVersionedResultRequiresExactFrozenCorrespondence` provides the mutation test pattern extended for `required_skills` |
| `internal/candidatechange/collect_test.go:1-50` | Candidate change collection tests for four-way git diff inspection |
| `internal/dag/parse.go:45-54` | YAML parsing pattern for tracked configuration files |
| `internal/executor/executor.go:20-39` | `requestEnv` injects read-only path context and provides the injection seam for `LUCIND_REQUIRED_SKILLS` |
| `internal/executor/read_only_paths_test.go:1-50` | `TestConcreteExecutorsExposeReadOnlyPathsToChild` verifies subprocess environment variable delivery |
| `internal/lane/status.go:11-17` | Closed set of terminal lane statuses including `lane.Done` and `lane.Deviated` |
| `internal/ledger/acceptance.go:96-103` | `SetDoneCandidate` uses `reflect.DeepEqual` on candidate rows, requiring exact SQL SELECT/Scan alignment |
| `internal/ledger/authoring.go:14` | `AuthoringEvidenceVersion` constant `lane-authoring-evidence/v1` preserved without version bump |
| `internal/ledger/authoring.go:20-42` | `AuthoringEvidence` struct definition storing typed contracts in `Contract json.RawMessage` |
| `internal/ledger/authoring.go:44-75` | `FreezeAuthoringEvidence` and `DecodeAuthoringEvidence` hashing and decoding functions |
| `internal/ledger/schema.go:425-445` | SQLite schema migration DDL establishing v10 candidate and receipt tables |
| `internal/ledger/schema.go:584-592` | Schema migration runner ensuring schema remains at v10 without new migrations |
| `internal/packet/packet.go:122-179` | `packet.Parse` frontmatter parser supporting closed-set `lane_role` and `sdd_phase` validation |
| `internal/packet/packet_test.go:133-195` | `TestParseObservabilityFrontmatter` testing frontmatter parsing and omission defaults |
| `internal/packetauthor/compile.go:15-25` | `normalizedContract` struct definition embedding contract fields for canonical hashing |
| `internal/packetauthor/compile.go:171-183` | `renderBody` formats packet markdown, providing the insertion point for `## Required skills` |
| `internal/packetauthor/compile_test.go:1-60` | `TestCompileDeterministicReplayAndCanonicalOrdering` verifying deterministic compilation and digest stability |
| `internal/packetauthor/contract.go:45-56` | `Contract` struct definition for authoring inputs |
| `internal/packetauthor/specialist.go:155-165` | `forbiddenSpecialistAuthority` and `forbiddenSpecialistRender` preventing specialist privilege escalation |
| `internal/result/result.go:103-116` | `Envelope` struct definition mirroring `result.schema.json` |
| `internal/result/result.schema.json:1-165` | JSON schema for packet result envelopes with `additionalProperties: false` |
| `internal/result/schema_test.go:10-33` | `TestSchemaJSONParsesAsJSON` and `TestSchemaJSONReturnsDefensiveCopy` testing embedded result schema |
| `internal/run/run.go:876-904` | `enforceAllowedPaths` implements git-diff-based demotion to `lane.Deviated`, serving as structural pattern for `enforceRequiredSkills` |
| `internal/skillcontent/skillcontent.go:1-28` | Package documentation incident writeup explaining why skill content hashing must never be a blocking gate |
| `internal/skillcontent/skillcontent.go:73-100` | `HashDir` computes deterministic SHA-256 over directory trees for non-blocking observation |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:20-27` | Proposal scope definitions establishing in-scope and out-of-scope boundaries |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:20-30` | Planning fan-out strategy requiring all lenses to be accepted before synthesis dispatch |
