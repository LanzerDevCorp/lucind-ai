# Tasks Lens A — Decomposition & Ordering: Skill Provisioning and the SDD Phase Specialist

## Assumed decomposition

This change is decomposed into 4 sequential phases comprising 15 actionable tasks across 4 new packages and 11 modified subsystems. Phase 1 establishes the foundational skill derivation, root resolution, configuration parsing, packet frontmatter models, and result envelope schemas. Phase 2 delivers contract compilation, body rendering, child execution environment injection, run-time shortfall demotion, and acceptance verification. Phase 3 implements admission-time batch validation, phase specialist sequencing, and CLI command dispatch. Phase 4 updates agent role skills and packet template references. The critical path runs through Phase 1 core packages into Phase 2 compilation/enforcement, gating Phase 3 admission and phase specialist execution.

## Phase 1: Foundation & Core Models

- [ ] 1.1 Create `internal/skillset/skillset.go` with pure `Derive`, package constant `DefaultSkillBudget = 3`, and `DigestBody` (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:37-42,61-67,103`)
- [ ] 1.2 Create `internal/skillroots/skillroots.go` implementing ordered root search from `.lucind/skill-roots.yaml`, tilde expansion, and fail-closed diagnostics (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:43-48,104`)
- [ ] 1.3 Create `internal/lucindconfig/config.go` loading tracked `lucind.yaml` with `KnownFields(true)`, stack skills per role, and optional `skill_budget` (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:43-48,105`)
- [ ] 1.4 Modify `internal/packet/packet.go` to add `LaneRole`, `AdhocSkills`, and `RequiredSkills` fields, parse `lane_role` against closed set `{lens, synthesis, apply, verify, archive, ultrafixer, human}`, closed-validate `sdd_phase`, parse `adhoc_skills`, and declare `ErrInvalidLaneRole` (`internal/packet/packet.go:43-103,122-179`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:49-54,93`)
- [ ] 1.5 Modify `internal/packetauthor/contract.go` adding `LaneRole`, `AdhocSkills`, and `RequiredSkills` to `Contract` (`internal/packetauthor/contract.go:45-56`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:16-21,94`)
- [ ] 1.6 Modify `internal/result/result.schema.json`, `internal/result/result.go`, and `internal/result/schema_test.go` to support optional `skills_loaded` property and add envelope reflection pin (`internal/result/result.go:103-116`, `internal/result/result.schema.json:7-165`, `internal/result/schema_test.go:10-33`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:97-99`)

## Phase 2: Compilation, Execution & Enforcement

- [ ] 2.1 Modify `internal/packetauthor/compile.go` to include `RequiredSkills` in `normalizedContract`, render `## Required skills` path block in `renderBody`, and hash `DigestBody` in contract and artifact digests (`internal/packetauthor/compile.go:15-25,45,171-183`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:61-67,95`)
- [ ] 2.2 Modify `internal/executor/executor.go` to add `RequiredSkills` to `Request` and inject `LUCIND_REQUIRED_SKILLS` JSON array in `requestEnv` alongside `LUCIND_READ_ONLY_PATHS` (`internal/executor/executor.go:20-39,50-75`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:96`)
- [ ] 2.3 Modify `internal/run/run.go` to add `enforceRequiredSkills` demoting shortfall to `lane.Deviated`, and update `packetDigest` to hash `DigestBody(p.Body)`, `LaneRole`, `RequiredSkills`, and `AdhocSkills` (`internal/run/run.go:489-491,722-729,876-904`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:61-67,100`)
- [ ] 2.4 Modify `internal/accept/accept.go` and `internal/accept/authoring_evidence_test.go` to add `RequiredSkills` to decode struct, enforce `skills_loaded` correspondence in `validateVersionedEvidence`, and add mutation test coverage (`internal/accept/accept.go:275-286`, `internal/accept/authoring_evidence_test.go:56-127`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:101-102`)

## Phase 3: Admission & Phase Specialist Subsystem

- [ ] 3.1 Modify `cmd/lucind-ai/packet_authoring.go` to integrate `lucindconfig`, `skillset.Derive`, `skillroots` path resolution, and fail-closed budget rejection into `admitDispatchBatch` before allocation (`cmd/lucind-ai/packet_authoring.go:32-54`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:22-27,107`)
- [ ] 3.2 Create `internal/phasespec/phasespec.go` implementing `gentle-ai sdd-status --json` parsing, fan-out lens acceptance gating before synthesis dispatch, and canonical artifact generation at `openspec/changes/<change>/<phase>.md` (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:28-36,106`)
- [ ] 3.3 Modify `cmd/lucind-ai/cli.go` to register and dispatch the `lucind-ai phase <name>` subcommand via `phasespec` (`cmd/lucind-ai/cli.go:142-168`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:28-36,107`)

## Phase 4: Skills, Templates & Documentation Alignment

- [ ] 4.1 Modify `.agents/skills/lucind-*` dropping executor-named skills and aligning role guidance without stub skills (`openspec/changes/skill-provisioning-and-phase-specialist/design.md:55-60,108`)
- [ ] 4.2 Modify `plugin/.../assets/*.md` and `.opencode/agent/lucind-packet-author.md` to remove hardcoded `~/.claude/skills/...` paths and align templates with `## Required skills` (`.opencode/agent/lucind-packet-author.md:14-32`, `openspec/changes/skill-provisioning-and-phase-specialist/design.md:108`)

## Dependency Order

| Task | Depends on | Why |
|---|---|---|
| 1.1 | — | Pure function package without internal dependencies |
| 1.2 | — | Independent filesystem root resolution package |
| 1.3 | — | Independent configuration parser package |
| 1.4 | — | Frontmatter parsing modifications with no internal imports |
| 1.5 | — | Contract data structure field additions |
| 1.6 | — | Result envelope schema and Go struct additions |
| 2.1 | 1.1, 1.5 | Compiler requires `skillset.DigestBody` and `Contract.RequiredSkills` |
| 2.2 | — | Independent executor environment injection helper |
| 2.3 | 1.1, 1.4, 1.6, 2.2 | `packetDigest` requires `skillset.DigestBody` and `Packet` fields; `enforceRequiredSkills` requires `Envelope.SkillsLoaded` |
| 2.4 | 1.1, 1.4, 1.6 | Acceptance decode struct mirrors `packetDigest` field list and validates `Envelope.SkillsLoaded` correspondence |
| 3.1 | 1.1, 1.2, 1.3, 1.4, 2.1 | `admitDispatchBatch` calls `lucindconfig`, `skillset.Derive`, `skillroots`, and passes derived skills to `Compile` |
| 3.2 | 3.1 | Phase specialist constructs and admits dispatch batches via `admitDispatchBatch` |
| 3.3 | 3.2 | CLI entry point calls `phasespec` execution engine |
| 4.1 | — | Skill documentation updates are independent of Go package compilation |
| 4.2 | 2.1 | Packet templates match compiled `renderBody` format containing `## Required skills` |

## Requirement Traceability

| Requirement | Tasks |
|---|---|
| `Requirement: Deterministic multi-tier derivation` | 1.1, 1.3, 3.1 |
| `Requirement: Root resolution and fail-closed admission` | 1.2, 3.1 |
| `Requirement: Result envelope skills loaded declaration` | 1.6, 2.3, 2.4 |
| `Requirement: Specialist sequencing and canonical artifact generation` | 3.2, 3.3 |
| `Requirement: Versioned Contract and Late Target Binding` | 1.5, 2.1, 2.4 |
| `Requirement: Frozen Authored Candidate Evidence` | 2.2, 2.3 |
| `Requirement: Fail-Closed Mechanical Criteria` | 2.3, 2.4 |
| `Requirement: Extended packet frontmatter parsing` | 1.4, 4.1, 4.2 |

## Open Questions

- [ ] Single-writer skill instructions (`~/.claude/skills/sdd-tasks/SKILL.md`) specify single-writer workload forecast and work-unit tables, which are partitioned to Lens B and Lens C in this fan-out packet.

## Citation Manifest

| Citation | Claim |
|---|---|
| `.opencode/agent/lucind-packet-author.md:14-32` | Packet author profile output JSON contract structure without hardcoded skill paths |
| `cmd/lucind-ai/cli.go:142-168` | CLI string switch registering top-level subcommands where `phase` is added |
| `cmd/lucind-ai/packet_authoring.go:32-54` | Pre-worktree batch admission seam `admitDispatchBatch` enforcing derivation and budget |
| `internal/accept/accept.go:275-286` | Versioned evidence decode struct in `validateVersionedEvidence` gaining `RequiredSkills` |
| `internal/accept/authoring_evidence_test.go:56-127` | Acceptance test verifying exact frozen correspondence against tampered evidence |
| `internal/executor/executor.go:20-39` | Executor environment setup in `requestEnv` injecting `LUCIND_REQUIRED_SKILLS` |
| `internal/executor/executor.go:50-75` | Executor `Request` struct definition carrying dispatch configuration |
| `internal/packet/packet.go:43-103` | `Packet` struct definition holding frontmatter and derived metadata |
| `internal/packet/packet.go:122-179` | `Parse` frontmatter scanner parsing `lane_role`, `adhoc_skills`, and `sdd_phase` |
| `internal/packetauthor/compile.go:15-25` | `normalizedContract` struct definition carrying validated fields |
| `internal/packetauthor/compile.go:45` | Artifact digest calculation in `Compile` hashing contract JSON, manifest JSON, and body |
| `internal/packetauthor/compile.go:171-183` | `renderBody` rendering prompt Markdown with `## Required skills` section |
| `internal/packetauthor/contract.go:45-56` | Authoring `Contract` struct definition holding target-free declarations |
| `internal/result/result.go:103-116` | Result `Envelope` struct definition carrying `skills_loaded` |
| `internal/result/result.schema.json:7-165` | Result schema properties definition allowing optional `skills_loaded` string array |
| `internal/result/schema_test.go:10-33` | Result schema parse and copy integrity tests |
| `internal/run/run.go:489-491` | `Run` execution flow invoking scope and completion enforcement |
| `internal/run/run.go:722-729` | `packetDigest` calculation hashing packet metadata and body digest |
| `internal/run/run.go:876-904` | Post-execution diff scope check in `enforceAllowedPaths` |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:16-21` | Decision 1: Ad-hoc skills on `Contract` and `Packet` frontmatter with derived `RequiredSkills` |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:22-27` | Decision 2: `DefaultSkillBudget = 3` package constant overridable in `lucind.yaml` |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:28-36` | Decision 3: Single `lucind-ai phase <name>` subcommand and packet author profile |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:37-42` | Decision 4: Pure function signature for `skillset.Derive` |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:43-48` | Decision 5: Tracked `lucind.yaml` with `KnownFields(true)` and gitignored `.lucind/skill-roots.yaml` |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:49-54` | Decision 6: Closed `lane_role` set and conditional `sdd_phase` validation |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:55-60` | Decision 7: Omission of stub `lucind-archive` and `lucind-ultrafixer` skills |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:61-67` | Decision 8: `DigestBody` eliding `## Required skills` from packet and compile digests |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:93-108` | Design File Changes table specifying 15 touched files across 4 new and 11 modified packages |
