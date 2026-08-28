# Proposal Lens B — Capability Impact & Specs: Skill Provisioning and the SDD Phase Specialist

## User & Capability Impact

| Capability | Impact (Added / Modified / Removed) | Description | Existing seam (file:line) |
|---|---|---|---|
| `skill-derivation` | Added | Deterministic derivation from `(sdd_phase, lane_role)` across three tiers (derived, stack, ad-hoc) under budget. | `internal/packet/packet.go:159-164` |
| `skill-root-resolution` | Added | Multi-root resolution of skill names to `SKILL.md` paths with `~` expansion and admission checks. | `internal/dag/parse.go:45-54` |
| `skill-load-correspondence` | Added | Demotes shortfall lanes to `lane.Deviated` and rejects acceptance when declared `skills_loaded` omits required skills. | `internal/result/result.schema.json:1-165` |
| `phase-specialist-dispatch` | Added | Translates typed `gentle-ai sdd-status` into fan-out and synthesis planning without intercepting gentle-ai authority. | `cmd/lucind-ai/packet_authoring.go:32-60` |
| `packet-authoring-contract` | Modified | Extends contract with `lane_role` and `required_skills`, rendering `## Required skills` with v1 hash stability. | `internal/packetauthor/compile.go:171-183` |
| `lane-execution` | Modified | Admission verifies skills and budget before allocation; post-dispatch runs `enforceRequiredSkills`. | `internal/run/run.go:876-904` |
| `acceptance-verifier` | Modified | Decodes `required_skills` from contract JSON and rejects candidates failing skill correspondence. | `internal/accept/accept.go:275-286` |
| `read-only-packet-schema` | Modified | Adds closed validation for `lane_role` and conditional closed validation for `sdd_phase`. | `internal/packet/packet.go:122-179` |

## Delta Specifications

### Requirement: Deterministic Multi-Tier Skill Derivation

The system MUST derive required skills deterministically from declared `(sdd_phase, lane_role)` pairs. Derivation MUST union three additive tiers: mandatory derived skills (`lucind-executor`, `lucind-fan-out-lens` + `sdd-<phase>`, `lucind-apply` + `sdd-apply`, `lucind-verify` + `sdd-verify`, `sdd-archive`), repo stack skills (`lucind.yaml`), and packet ad-hoc skills. Budget arithmetic (default: 3) MUST NOT shed derived skills; excess ad-hoc or stack skills MUST be shed in order, or admission MUST fail if derived skills exceed budget.

#### Scenario: Mandatory planning skills
- GIVEN `sdd_phase: propose` and `lane_role: lens`
- WHEN skill derivation executes
- THEN the set MUST contain `lucind-executor`, `lucind-fan-out-lens`, and `sdd-propose`

#### Scenario: Tier union within budget
- GIVEN stack and ad-hoc skills within budget 3
- WHEN derivation executes
- THEN all tiers MUST be unioned and ordered deterministically

#### Scenario: Over-budget shedding
- GIVEN derived and stack skills exceeding budget
- WHEN budget arithmetic executes
- THEN stack skills MUST be shed to fit budget, or admission MUST fail

### Requirement: Machine-Local Root Resolution and Admission Gate

The system MUST resolve skill names to `SKILL.md` paths using ordered search roots in `.lucind/skill-roots.yaml` with tilde (`~`) expansion. Admission MUST fail closed before worktree or quota allocation if any required skill cannot be resolved on disk.

#### Scenario: Resolve path with tilde expansion
- GIVEN root `~/.claude/skills` and skill `sdd-propose`
- WHEN path resolution executes
- THEN `~` MUST be expanded to locate `~/.claude/skills/sdd-propose/SKILL.md`

#### Scenario: Missing skill blocks batch admission
- GIVEN an unresolvable required skill
- WHEN `admitDispatchBatch` executes
- THEN admission MUST reject the batch before allocation, naming the missing skill

### Requirement: Contract Extension and Rendered Skill Delivery

The compiled contract MUST carry `lane_role` and `required_skills` inside the `Contract json.RawMessage` blob. The compiler MUST render a `## Required skills` section listing resolved `SKILL.md` paths in the packet body. The `AuthoringEvidence` struct shape and version (`AuthoringEvidenceVersion = "lane-authoring-evidence/v1"`) MUST remain unchanged.

#### Scenario: Render required skills
- GIVEN a contract with resolved required skills
- WHEN `renderBody` compiles markdown
- THEN the body MUST include a `## Required skills` section with resolved file paths

#### Scenario: Evidence hash stability
- GIVEN a contract containing `required_skills`
- WHEN `FreezeAuthoringEvidence` and `DecodeAuthoringEvidence` execute
- THEN evidence version MUST remain `lane-authoring-evidence/v1` and verify without migrations

### Requirement: Closed-Set Validation of Lane Role and SDD Phase

Packet parsing MUST validate `lane_role` against the closed set `{"lens", "synthesis", "apply", "verify", "archive", "ultrafixer", "human"}`. When `lane_role` is present, `sdd_phase` MUST be validated against `{"propose", "spec", "design", "tasks", "apply", "verify", "remediate", "archive"}`. Legacy packets omitting `lane_role` MUST continue parsing without error.

#### Scenario: Valid lane_role and sdd_phase
- GIVEN `lane_role: lens` and `sdd_phase: propose`
- WHEN `packet.Parse` executes
- THEN parsing MUST succeed and assign `Packet.LaneRole` and `Packet.SDDPhase`

#### Scenario: Reject unknown lane_role
- GIVEN an unrecognized `lane_role`
- WHEN `packet.Parse` executes
- THEN parsing MUST return a validation error

### Requirement: Runtime Demotion and Mechanical Acceptance Correspondence

Result envelopes MUST support declaring `skills_loaded`. Post-dispatch execution MUST evaluate `enforceRequiredSkills` and demote lanes to `lane.Deviated` when `skills_loaded` omits any required skill. Acceptance verification MUST decode `required_skills` from frozen contract evidence and reject candidates with skill shortfalls.

#### Scenario: Demote shortfall lane
- GIVEN an envelope omitting a required skill from `skills_loaded`
- WHEN `run.enforceRequiredSkills` evaluates the lane
- THEN the lane status MUST become `lane.Deviated`

#### Scenario: Acceptance rejects missing skill
- GIVEN frozen contract evidence requiring `sdd-propose` and an envelope omitting it
- WHEN `accept.validateVersionedEvidence` executes
- THEN acceptance MUST fail and create no receipt

### Requirement: Phase Specialist Dispatch and Fan-Out Sequencing

The phase specialist MUST ingest `gentle-ai sdd-status` JSON, plan phase lanes, and dispatch through `lucind-ai`. For fan-out phases, synthesis lanes MUST NOT dispatch or acquire an attempt until all required lens lanes are accepted and merged. Upon synthesis completion, the specialist MUST place the canonical artifact at `openspec/changes/<change>/<phase>.md`.

#### Scenario: Fan-out precedes synthesis
- GIVEN an active fan-out `propose` phase
- WHEN the specialist executes
- THEN Lens A, B, and C MUST be accepted before synthesis launches

#### Scenario: Canonical artifact advances phase
- GIVEN synthesis writes `openspec/changes/<change>/proposal.md`
- WHEN `gentle-ai sdd-status` inspects the repository
- THEN the phase gate MUST advance without provenance errors

## Open Questions

- [ ] Delivery channel: Pass required skills via `LUCIND_READ_ONLY_PATHS` in addition to rendered body?
- [ ] Ad-hoc surface: Author via `adhoc_skills` frontmatter key, typed contract field only, or both?
- [ ] Missing role skills: Author child skills for `archive` and `ultrafixer`, or declare derived-empty?
- [ ] Budget default: Is 3 sufficient or should `lucind.yaml` support repo-level budget overrides?
- [ ] Specialist CLI shape: Single parameterized command (`lucind-ai phase <phase>`) vs per-phase subcommands?

## Citation Manifest

| citation | claim |
|---|---|
| `.gitignore:2` | Ignores `.lucind/` directory holding machine-local state and search roots. |
| `.opencode/agent/lucind-packet-author.md:1-8` | Specialist profile with zero tools and step limits enforcing deterministic compilation. |
| `cmd/lucind-ai/packet_authoring.go:32-60` | `admitDispatchBatch` executes pre-dispatch admission before worktree or quota allocation. |
| `internal/accept/accept.go:1-2` | Package documentation establishing accept as a pure receipt verifier without mutation authority. |
| `internal/accept/accept.go:213-261` | `validateResultAndScope` verifies result hash, status, criteria, stops, and diff. |
| `internal/accept/accept.go:263-328` | `validateVersionedEvidence` compares frozen authoring evidence against candidate facts. |
| `internal/accept/accept.go:275-286` | Private hand-duplicated contract decode struct that must be updated for required skills. |
| `internal/dag/parse.go:45-54` | YAML parsing precedent in `Parse` unmarshaling sidecar configuration. |
| `internal/executor/executor.go:20-39` | `requestEnv` constructs execution environment passing `LUCIND_READ_ONLY_PATHS`. |
| `internal/ledger/authoring.go:23` | `Contract json.RawMessage` blob in `AuthoringEvidence` enabling unmigrated extension. |
| `internal/ledger/authoring.go:44-60` | `FreezeAuthoringEvidence` computes length-prefixed SHA-256 hash over evidence. |
| `internal/ledger/authoring.go:62-75` | `DecodeAuthoringEvidence` strictly verifies evidence version and hash re-freeze equality. |
| `internal/packet/packet.go:122-179` | Full frontmatter key parsing loop handling metadata and path arrays. |
| `internal/packet/packet.go:159-164` | Frontmatter parsing for `sdd_phase`, `fanout_group`, and `skill` pass-through keys. |
| `internal/packetauthor/compile.go:171-183` | `renderBody` formats markdown sections for goal, criteria, stops, and result contract. |
| `internal/packetauthor/compile.go:192-208` | `digest` computes deterministic SHA-256 hash over length-prefixed contract fields. |
| `internal/result/result.go:103-116` | `Envelope` struct representing `.lucind/result.json` payload. |
| `internal/result/result.schema.json:1-165` | JSON schema defining allowed result envelope properties with `additionalProperties: false`. |
| `internal/run/run.go:382-393` | `UpdateLaneMetadata` persists dispatched packet metadata snapshot to ledger. |
| `internal/run/run.go:722-729` | `packetDigest` hashes frontmatter and packet body into versioned hash. |
| `internal/run/run.go:876-904` | `enforceAllowedPaths` evaluates git diff and demotes out-of-scope runs to deviated. |
| `internal/skillcontent/skillcontent.go:90-100` | `HashDir` walks directory tree to compute deterministic SHA-256 content hash. |
| `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:2-6` | Packet template demonstrating that lens lanes are write lanes with declared allowed paths. |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:21-25` | Fan-out dispatch ordering invariant requiring all lens lanes accepted before synthesis. |
