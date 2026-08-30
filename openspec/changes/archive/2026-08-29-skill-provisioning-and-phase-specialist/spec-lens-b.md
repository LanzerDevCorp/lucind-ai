# Spec Lens B — Scenarios & Coverage: Skill Provisioning and the SDD Phase Specialist

## Assumed requirements

The proposal defines six core requirements: `Deterministic multi-tier derivation`, `Root resolution and fail-closed admission`, `Contract extension and rendered delivery`, `Closed-set lane_role`, `Demotion and acceptance correspondence`, and `Specialist sequencing`. These cover typed skill derivation, machine-local root resolution, frozen contract delivery, execution demotion, acceptance receipt verification, and non-intercepting SDD specialist sequencing.

## Scenarios

### Requirement: Deterministic multi-tier derivation

#### Scenario: Planning lens derivation

- GIVEN `sdd_phase: propose` and `lane_role: lens` without stack or ad-hoc additions
- WHEN derivation runs at batch admission
- THEN required skills MUST equal `["lucind-executor", "lucind-fan-out-lens", "sdd-propose"]`.

#### Scenario: Stack deduplication within budget

- GIVEN `lucind.yaml` stack listing `lucind-executor`, already in the derived set
- WHEN derivation unions derived, stack, and ad-hoc tiers
- THEN `lucind-executor` MUST deduplicate, counting as 1 toward budget default 3.

#### Scenario: Over-budget skill set rejected

- GIVEN combined derived, stack, and ad-hoc skills exceeding budget 3
- WHEN `admitDispatchBatch` executes
- THEN admission MUST fail closed before allocating any worktree.

### Requirement: Root resolution and fail-closed admission

#### Scenario: Tilde-expanded skill root resolution

- GIVEN `.lucind/skill-roots.yaml` with root `~/.claude/skills` and skill `sdd-propose` at `~/.claude/skills/sdd-propose/SKILL.md`
- WHEN `admitDispatchBatch` resolves the skill path
- THEN resolution MUST expand `~` to home directory and locate `SKILL.md`.

#### Scenario: Multi-root ordered resolution

- GIVEN `.lucind/skill-roots.yaml` with two roots where a skill exists only under the second root
- WHEN root resolution searches configured roots in order
- THEN resolution MUST locate the skill under the second root.

#### Scenario: Unresolvable required skill fails admission

- GIVEN a required skill missing from all configured roots in `.lucind/skill-roots.yaml`
- WHEN `admitDispatchBatch` validates the batch
- THEN admission MUST return a non-nil error naming the unresolvable skill and searched roots before creating worktrees.

### Requirement: Contract extension and rendered delivery

#### Scenario: Dual delivery in body and environment

- GIVEN a compiled contract with resolved skill `sdd-propose`
- WHEN `renderBody` formats markdown and executor prepares environment variables
- THEN body MUST contain `## Required skills` listing resolved paths and `requestEnv` MUST inject `LUCIND_REQUIRED_SKILLS`.

#### Scenario: Legacy authoring evidence hash stability

- GIVEN frozen `AuthoringEvidence` under `lane-authoring-evidence/v1` without `required_skills`
- WHEN `DecodeAuthoringEvidence` decodes the legacy record
- THEN hash verification MUST succeed without schema migration.

#### Scenario: Malformed contract payload rejected

- GIVEN `AuthoringEvidence` containing invalid JSON in `Contract`
- WHEN `validateVersionedEvidence` executes during acceptance
- THEN acceptance MUST fail with an authored contract integrity error.

### Requirement: Closed-set lane_role

#### Scenario: Valid lane_role and phase parsed

- GIVEN frontmatter declaring `lane_role: lens` and `sdd_phase: propose`
- WHEN `packet.Parse` executes
- THEN `p.LaneRole` MUST equal `"lens"` and `p.SDDPhase` MUST equal `"propose"`.

#### Scenario: Omitted lane_role preserves backward compatibility

- GIVEN frontmatter omitting `lane_role`
- WHEN `packet.Parse` executes
- THEN parsing MUST succeed with `p.LaneRole` empty and unvalidated `sdd_phase`.

#### Scenario: Unrecognized lane_role rejected

- GIVEN frontmatter declaring `lane_role: unsupported-role`
- WHEN `packet.Parse` runs its validation switch
- THEN `packet.Parse` MUST return a validation error.

### Requirement: Demotion and acceptance correspondence

#### Scenario: Complete skills loaded accepted

- GIVEN required skills `["lucind-executor", "lucind-fan-out-lens", "sdd-propose"]` and envelope declaring matching `skills_loaded` with `status: done`
- WHEN `decideStatus` and `validateVersionedEvidence` run
- THEN status MUST remain `lane.Done` and acceptance MUST succeed.

#### Scenario: Superfluous declared skills tolerated

- GIVEN required skill `["lucind-executor"]` and envelope declaring `skills_loaded: ["lucind-executor", "extra-skill"]`
- WHEN `enforceRequiredSkills` and acceptance run
- THEN status MUST remain `lane.Done` and acceptance MUST succeed.

#### Scenario: Skill shortfall demoted and rejected

- GIVEN required skills `["lucind-executor", "sdd-propose"]` and envelope omitting `sdd-propose` from `skills_loaded`
- WHEN `decideStatus` evaluates the lane result
- THEN `enforceRequiredSkills` MUST demote to `lane.Deviated`, and acceptance MUST fail.

### Requirement: Specialist sequencing

#### Scenario: Fan-out lenses merged before synthesis dispatch

- GIVEN an active propose phase with all required lenses (`lens-a`, `lens-b`, `lens-c`) accepted and merged
- WHEN SDD phase specialist checks `gentle-ai sdd-status`
- THEN specialist MUST dispatch `sdd-propose` synthesis lane for `openspec/changes/<change>/propose.md`.

#### Scenario: Unchanged phase state generates no dispatches

- GIVEN `gentle-ai sdd-status` reporting phase complete with canonical artifact present
- WHEN phase specialist inspects lifecycle state
- THEN specialist MUST complete without dispatching redundant lanes.

#### Scenario: Synthesis blocked while lenses unmerged

- GIVEN an active propose phase with an unmerged lens
- WHEN phase specialist evaluates next action
- THEN specialist MUST NOT dispatch synthesis and MUST wait.

## Coverage

| Requirement | Happy path | Edge case | Error state | Testable through (file:line) |
|---|---|---|---|---|
| Deterministic multi-tier derivation | Planning lens derivation | Stack deduplication within budget | Over-budget skill set rejected | `cmd/lucind-ai/packet_authoring.go:32-54` |
| Root resolution and fail-closed admission | Tilde-expanded skill root resolution | Multi-root ordered resolution | Unresolvable required skill fails admission | `cmd/lucind-ai/packet_authoring.go:32-54` |
| Contract extension and rendered delivery | Dual delivery in body and environment | Legacy authoring evidence hash stability | Malformed contract payload rejected | `internal/packetauthor/compile.go:171-183`, `internal/executor/executor.go:20-39`, `internal/ledger/authoring.go:44-75` |
| Closed-set lane_role | Valid lane_role and phase parsed | Omitted lane_role preserves backward compatibility | Unrecognized lane_role rejected | `internal/packet/packet.go:122-179`, `internal/packet/packet.go:196-205` |
| Demotion and acceptance correspondence | Complete skills loaded accepted | Superfluous declared skills tolerated | Skill shortfall demoted and rejected | `internal/run/run.go:876-904`, `internal/accept/accept.go:263-328` |
| Specialist sequencing | Fan-out lenses merged before synthesis dispatch | Unchanged phase state generates no dispatches | Synthesis blocked while lenses unmerged | New seam required (`internal/phasespec/` specialist loop) |

## Untestable Assertions

None. Every scenario asserts an observable outcome testable through admission errors, packet parsing, hash verification, execution demotion, acceptance checks, or specialist state.

## Open Questions

- [ ] Should stack skills in `lucind.yaml` support machine-local root overrides or resolve strictly through `.lucind/skill-roots.yaml`?

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/packet_authoring.go:32-54` | `admitDispatchBatch` executes pre-worktree batch validation and admission rejection |
| `internal/accept/accept.go:263-328` | `validateVersionedEvidence` verifies candidate changes and envelope correspondence against frozen evidence |
| `internal/accept/accept.go:275-286` | Decode contract struct in acceptance verifier requiring `LaneRole` and `RequiredSkills` synchronization |
| `internal/accept/authoring_evidence_test.go:56-127` | Contract mutation and correspondence test patterns verifying evidence rejection |
| `internal/executor/executor.go:20-39` | `requestEnv` constructs execution environment variables for dispatched lanes |
| `internal/ledger/authoring.go:14` | `AuthoringEvidenceVersion` constant defines version string `lane-authoring-evidence/v1` |
| `internal/ledger/authoring.go:20-42` | `AuthoringEvidence` struct embeds `Contract json.RawMessage` without ledger schema migration |
| `internal/ledger/authoring.go:44-75` | `FreezeAuthoringEvidence` and `DecodeAuthoringEvidence` compute and verify hash integrity |
| `internal/packet/packet.go:122-179` | `packet.Parse` parses frontmatter key-value pairs |
| `internal/packet/packet.go:196-205` | `packet.Parse` validation switch evaluates required frontmatter and content invariants |
| `internal/packetauthor/compile.go:15-25` | `normalizedContract` struct definition for authored packet contracts |
| `internal/packetauthor/compile.go:171-183` | `renderBody` formats markdown headers and contract details into the packet body |
| `internal/result/result.go:103-116` | `Envelope` struct definition representing the result JSON payload |
| `internal/result/result.schema.json:1-30` | Result envelope JSON schema enforcing strict properties with `additionalProperties: false` |
| `internal/result/schema_test.go:10-33` | Envelope schema JSON unmarshaling and defensive copy tests |
| `internal/run/run.go:876-904` | `enforceAllowedPaths` demotes out-of-scope diff changes to `lane.Deviated` |
