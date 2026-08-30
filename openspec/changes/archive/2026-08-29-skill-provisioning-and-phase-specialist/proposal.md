# Proposal: Skill Provisioning and the SDD Phase Specialist

**Chosen candidate: Candidate 1 — Deterministic Three-Tier Skill Provisioning with a Non-Intercepting SDD Phase Specialist.**

## Intent

A packet tells an agent what to do but never delivers the manuals that govern the work. Executor skills under `.agents/skills/` load only if a runtime scans that directory (`.agents/skills/lucind-executor/SKILL.md:4-17`). Templates hardcode `$HOME`-dependent `~/.claude/skills/sdd-*` paths. The `skill:` key (`internal/packet/packet.go:159-164`) is copied into lane metadata (`internal/run/run.go:382-393`) as telemetry; nothing reads it back. Separately, the orchestrator still authors every SDD packet by hand.

Make required skills a typed, binary-derived, frozen part of the packet contract — delivered, then verified against what the agent declares. Add a per-phase specialist that composes `gentle-ai sdd-status` with lucind-ai dispatch so the orchestrator states intent and reads results.

## Scope

### In Scope
- Three-tier derivation from `(sdd_phase, lane_role)`, machine-local resolution, admission existence and budget checks.
- Dual delivery (body plus environment) and two-site enforcement (`run` demotes, `accept` re-verifies).
- Per-phase specialist that plans, dispatches, and writes the canonical artifact without intercepting gentle-ai.
- Decouple `.agents/skills/lucind-*` from executor names; drop hardcoded skill paths from templates.
- Close the duplicated-contract, schema/struct, and whole-struct equality drift seams.

### Out of Scope
- Changing `AuthoringEvidence` shape or `AuthoringEvidenceVersion` (`internal/ledger/authoring.go:14,20-42`); any SQLite migration (`internal/ledger/schema.go:425-445,584-592`).
- Specialist-side skill selection (`.opencode/agent/lucind-packet-author.md:1-8` denies all tools).
- Wrapping or replacing gentle-ai; reading `openspec/config.yaml` in the binary.
- Authoring skill content; treating an external skill edit as a blocking gate (`internal/skillcontent/skillcontent.go:1-28`).
- Automatic cutover from manual packets; splitting into multiple PRs (`openspec/config.yaml:6-7` authorizes `single-pr` at 10000 lines).
- CLI failure-guidance banners (`cmd/lucind-ai/cli.go:699,737,759,2004`).

## Capabilities

### New Capabilities
- `skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`, `phase-specialist-dispatch`

### Modified Capabilities
- `packet-authoring-contract`, `lane-execution`, `acceptance-verifier`, `read-only-packet-schema`

## Selected Candidate & Approach

1. **Derivation.** `internal/skillset/` computes `derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)`. Derived skills are never dropped. Stack names come from tracked `lucind.yaml`; ad-hoc names are authored input, frozen like `goal`. Budget default 3: a set that does not fit is rejected at admission, not trimmed. Every lane derives `lucind-executor`. Planning roles derive `lucind-fan-out-lens` plus `sdd-<phase>`; `apply`/`verify` derive the matching lucind child plus `sdd-apply`/`sdd-verify`; `archive` derives `sdd-archive`.

2. **Resolution.** `internal/skillroots/` maps names to `SKILL.md` via ordered roots in `.lucind/skill-roots.yaml` (gitignored, `.gitignore:2`) with `~` expansion — Go has neither `os.UserHomeDir` nor multi-root lookup. Names enter the digest; absolute paths do not. YAML loading follows `internal/dag/parse.go:45-54`. `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) rejects the whole batch before worktree or quota if any required skill is missing, naming the skill and roots searched.

3. **Contract blob, no migration.** Skills ride in `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:26`), a `json.RawMessage`. Freeze/decode (`internal/ledger/authoring.go:44-75`) re-hashes the whole struct; a new struct field would break every frozen row. Bytes inside `Contract` change only for new rows. Version stays `lane-authoring-evidence/v1` (`internal/ledger/authoring.go:14`).

4. **Dual delivery.** `renderBody` (`internal/packetauthor/compile.go:171-183`) emits `## Required skills` with resolved paths. `requestEnv` (`internal/executor/executor.go:20-39`) already injects `LUCIND_READ_ONLY_PATHS`; required skills use the same channel as `LUCIND_REQUIRED_SKILLS`.

5. **Two-site enforcement.** Operational demotion lives in `enforceAllowedPaths` (`internal/run/run.go:876-904`) → `lane.Deviated` (`internal/lane/status.go:11-17`). `enforceRequiredSkills` is added beside it. `accept` is a pure receipt verifier (`internal/accept/accept.go:1-2,213-261`); it errors and never demotes (`cmd/lucind-ai/cli.go:684-687`). `validateVersionedEvidence` (`internal/accept/accept.go:263-328`) re-checks frozen evidence. The duplicated decode struct (`internal/accept/accept.go:275-286`) must gain `LaneRole` and `RequiredSkills` in the same commit, or the field is frozen and silently never verified.

6. **Specialist composes.** `internal/phasespec/` reads typed `gentle-ai sdd-status` JSON, plans lanes, dispatches through existing machinery, and writes `openspec/changes/<change>/<phase>.md`. gentle-ai keeps authority. Specialist output stays inside forbidden-key maps (`internal/packetauthor/specialist.go:155-165`). Synthesis MUST NOT start until every required lens is accepted and merged (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:24`).

7. **Identities.** A repo skill is pinned by the lane `base_sha`. An external skill is hashed at admission via `HashDir` (`internal/skillcontent/skillcontent.go:73-100`) as observation only, never a gate (`internal/skillcontent/skillcontent.go:1-28`).

8. **`lane_role`.** New optional frontmatter key, closed set `{lens, synthesis, apply, verify, archive, ultrafixer, human}`. `sdd_phase` is closed-validated only when `lane_role` is present (`internal/packet/packet.go:122-179`). Packets that omit it keep parsing. Added to `packetauthor.Contract` (`internal/packetauthor/contract.go:45-56`) and `normalizedContract` (`internal/packetauthor/compile.go:15-25`).

Lane-lifecycle hooks: `admitDispatchBatch`; demotion beside `enforceAllowedPaths` (`internal/run/run.go:882-904`); receipt check in `validateVersionedEvidence`.

## Conceptual Changes

- First repo-tracked config the binary reads: `lucind.yaml` (skill names per role; unknown keys rejected). Roots stay machine-local and out of the digest.
- First closed-set frontmatter validation.
- Envelope gains optional `skills_loaded` (`internal/result/result.go:103-116`). The schema sets `additionalProperties: false` (`internal/result/result.schema.json:5`); `schema_test.go:10-33` only parses JSON and checks a defensive copy — add a reflection pin tying `Envelope` to the schema.
- Correspondence tests (`internal/accept/authoring_evidence_test.go:56-127`) gain a `required_skills` mutation case.
- `SetDoneCandidate` uses `reflect.DeepEqual` on the whole candidate (`internal/ledger/acceptance.go:96-103`); any new Go field must stay in the SQL SELECT/Scan.
- Executor skills drop `agy`-specific naming (`.agents/skills/lucind-executor/SKILL.md:4-17`, `.agents/skills/lucind-verify/SKILL.md:4`).

## User and Capability Impact

| Capability | Impact | Existing seam |
|---|---|---|
| `skill-derivation` | Added | `internal/packet/packet.go:159-164` |
| `skill-root-resolution` | Added | `internal/dag/parse.go:45-54` |
| `skill-load-correspondence` | Added | `internal/result/result.schema.json:1-165` |
| `phase-specialist-dispatch` | Added | `cmd/lucind-ai/packet_authoring.go:32-54` |
| `packet-authoring-contract` | Modified | `internal/packetauthor/compile.go:171-183` |
| `lane-execution` | Modified | `internal/run/run.go:876-904` |
| `acceptance-verifier` | Modified | `internal/accept/accept.go:275-286` |
| `read-only-packet-schema` | Modified | `internal/packet/packet.go:122-179` |

## Delta Specifications

### Requirement: Deterministic multi-tier derivation
The system MUST derive required skills from `(sdd_phase, lane_role)`, unioning derived, stack (`lucind.yaml`), and ad-hoc tiers. Derived skills MUST never be dropped. A set exceeding budget (default 3) MUST be rejected at admission.

#### Scenario: Planning lens set
GIVEN `sdd_phase: propose` and `lane_role: lens`, derivation MUST include `lucind-executor`, `lucind-fan-out-lens`, and `sdd-propose`.

#### Scenario: Over budget
GIVEN derived plus stack or ad-hoc names exceeding budget 3, admission MUST reject the batch before worktree allocation.

### Requirement: Root resolution and fail-closed admission
The system MUST resolve names through `.lucind/skill-roots.yaml` with `~` expansion. An unresolvable required skill MUST fail `admitDispatchBatch` before allocation, naming the skill.

#### Scenario: Tilde expansion
GIVEN root `~/.claude/skills` and skill `sdd-propose`, resolution MUST locate `~/.claude/skills/sdd-propose/SKILL.md`.

### Requirement: Contract extension and rendered delivery
`lane_role` and `required_skills` MUST travel inside `Contract json.RawMessage`. `renderBody` MUST include `## Required skills` with resolved paths. `AuthoringEvidence` shape and version MUST stay v1.

#### Scenario: Hash stability
GIVEN a contract containing `required_skills`, freeze/decode MUST keep version `lane-authoring-evidence/v1` and existing rows MUST still verify.

### Requirement: Closed-set `lane_role`
`packet.Parse` MUST validate `lane_role` against `{lens, synthesis, apply, verify, archive, ultrafixer, human}`. When present, `sdd_phase` MUST be closed-validated. Packets omitting `lane_role` MUST still parse.

#### Scenario: Unknown role rejected
GIVEN an unrecognized `lane_role`, `packet.Parse` MUST return a validation error.

### Requirement: Demotion and acceptance correspondence
Envelopes MUST accept `skills_loaded`. `enforceRequiredSkills` MUST demote shortfalls to `lane.Deviated`. Acceptance MUST decode `required_skills` from frozen evidence and reject shortfalls with no receipt.

#### Scenario: Shortfall
GIVEN an envelope omitting a required skill, post-dispatch status MUST be `lane.Deviated` and later acceptance MUST fail.

### Requirement: Specialist sequencing
The specialist MUST ingest `gentle-ai sdd-status` JSON, dispatch through lucind-ai, and MUST NOT start synthesis until all required lenses are accepted and merged. The canonical artifact MUST land at `openspec/changes/<change>/<phase>.md`.

#### Scenario: Fan-out then synthesis
GIVEN an active propose phase, lenses A, B, and C MUST be accepted before synthesis launches.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/skillset/`, `skillroots/`, `lucindconfig/`, `phasespec/` | New | Derivation, roots, config, specialist adapter |
| `internal/packetauthor/`, `packet/`, `run/`, `result/`, `accept/` | Modified | Contract, frontmatter, demotion, envelope, correspondence |
| `cmd/lucind-ai/` | Modified | Admission adapter, phase subcommand |
| `.opencode/agent/`, `.agents/skills/lucind-*`, `plugin/.../assets/*.md` | Modified | Profiles, executor decoupling, drop hardcoded paths |
| `internal/ledger/` | Untouched | Skills ride in the contract blob |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| New contract field frozen, never verified (`accept.go:275-286`) | High | Same-commit decode-struct update; mutation case in `authoring_evidence_test.go:56-127` |
| Schema/struct desync (`result.schema.json:5`, Envelope `result.go:103-116`) | High | Add `skills_loaded` to both; add a reflection pin in `schema_test.go` |
| Demotion attempted inside `accept` | Med | Keep accept error-only; demote beside `enforceAllowedPaths` (`run.go:882-904`) |
| Unresolved skill path dispatched | Med | Fail closed in `admitDispatchBatch` (`packet_authoring.go:32-54`); ranked diagnostics (`diagnostic.go:8-39`) |
| Over-budget prompt bloat | Med | Reject at admission; derived never dropped |
| Synthesis before lens merge charges lens lines | Med | Encode `fan-out.md:24` as a dispatch precondition |
| External skill hash used as a gate recreates overlap conflicts | Med | Observation only (`skillcontent.go:1-28`) |
| `SetDoneCandidate` DeepEqual (`acceptance.go:96-103`) | Med | Keep SELECT/Scan aligned with struct fields |
| `lucind.yaml` accretes unrelated keys | Med | Accept only `skills`; reject unknown keys |

Correspondence proves the agent *declared* the skill, not that it opened the file.

## Rollback Plan

Additive throughout. No row conversion.

1. Stop rendering `## Required skills` (`compile.go:171-183`) and stop deriving at admission (`packet_authoring.go:32-54`). Packets compile as before; `skills_loaded` is ignored.
2. Revert `enforceRequiredSkills` (`run.go:882-904`) and the accept comparison (`accept.go:263-328,275-286`).
3. Revert envelope schema/struct (`result.schema.json:1-165`, `result.go:103-116`).
4. Revert `internal/phasespec/`, `skillset/`, `skillroots/`, `lucindconfig/`.

`AuthoringEvidence` and schema v10 stay untouched; old rows re-decode identically (`authoring.go:62-75`).

## Test and Validation Impact

| Layer | Coverage |
|---|---|
| Unit | Closed-set `lane_role` parse and legacy omit (`packet.go:122-179`); Compile embeds `required_skills`, digest, budget reject, `## Required skills` (`compile.go:32-47,171-183`); `~` resolution and missing-file fail-closed; Envelope↔schema reflection pin |
| Integration | `enforceRequiredSkills` demotes shortfalls (`run.go:882-904`); accept rejects skill mutations (`accept.go:275-286`, `authoring_evidence_test.go:56-127`); freeze/decode with new contracts and legacy v1 rows (`authoring.go:44-75`, `authoring_evidence_test.go:12-44`); `requestEnv` delivers the skills env (`executor.go:25-39`) |
| E2E | Specialist loop: ingest status, `admitDispatchBatch`, lens merge, synthesis, artifact write (`packet_authoring.go:32-54`, `worktree.go:159-163`) |

## Dependencies

- `delegated-packet-authoring` is satisfied (archived). This work reuses `requestEnv` (`internal/executor/executor.go:20-39`).

## Open Questions

- [ ] Ad-hoc authoring surface: new frontmatter key, typed contract field only, or both?
- [ ] Missing child skills for `archive` and `ultrafixer`: create stubs, or declare those roles derived-empty?
- [ ] Budget default of 3: measure against real dispatches; should `lucind.yaml` allow a repo override?
- [ ] Specialist shape: one parameterized `lucind-ai phase <name>` (and one profile) versus per-phase subcommands and profiles?
- [ ] Confirm `lucind.yaml` at repo root does not collide with other toolchain filenames.

## Success Criteria

- [ ] Same `(phase, role)` yields a byte-identical required set and stable digest across machines with different roots.
- [ ] Unresolvable required skill rejects the batch before worktree allocation, naming the skill.
- [ ] Dispatched body lists resolved paths for every required skill.
- [ ] Envelope shortfall lands as `deviated` and is rejected at acceptance.
- [ ] Pre-existing frozen candidates still decode; evidence version stays v1; no schema migration.
- [ ] One fan-out phase completes (lenses, accept, merge, synthesis, canonical artifact) and `gentle-ai sdd-status` advances.
- [ ] No `.agents/skills/lucind-*` file names an executor; hardcoded `~/.claude/skills/...` paths are gone from `assets/`.

## Alternatives Considered

- Specialist-side skill selection — rejected: packet-author has no tools (`.opencode/agent/lucind-packet-author.md:1-8`).
- Ledger migration — rejected: `Contract` already carries new JSON (`authoring.go:20-42,62-75`).
- Intercepting gentle-ai — rejected: phase gates are content and path, not provenance.
- Multi-PR rollout — rejected by D4 and `openspec/config.yaml:6-7`.
