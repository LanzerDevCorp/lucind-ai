# Design Lens B — Surface & Flow: Skill Provisioning and the SDD Phase Specialist

## Assumed architecture

Candidate 1 deterministic 3-tier skill provisioning (`derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)`), capped by budget (default 3) where derived skills cannot drop. Machine-local roots in gitignored `.lucind/skill-roots.yaml` (`.gitignore:1-3`) map names to `SKILL.md` paths before worktree allocation. Dual delivery provisions skills via prompt body (`## Required skills`) and environment (`LUCIND_REQUIRED_SKILLS`). Two-site enforcement: `enforceRequiredSkills` demotes shortfalls to `deviated` at run time (`internal/run/run.go:882-904`); `validateVersionedEvidence` verifies correspondence at acceptance (`internal/accept/accept.go:263-328`).

## Flow and Invariants

```
(sdd_phase, lane_role) + lucind.yaml + adhoc
                     │
                     ▼
       internal/skillset (Union)
                     │
                     ▼
      internal/skillroots (Resolution & Budget)
                     │
       ┌─────────────┴─────────────┐
       ▼                           ▼
Rendered Body Prompt       Process Env
(## Required skills)   (LUCIND_REQUIRED_SKILLS)
       │                           │
       └─────────────┬─────────────┘
                     ▼
          Agent Execution & Result
          (Envelope.skills_loaded)
                     │
                     ▼
     internal/run (enforceRequiredSkills) ──[Shortfall]──► lane.Deviated
                     │
                     ▼ [Pass]
   internal/accept (validateVersionedEvidence) ──[Mismatch]──► Reject
```

- **Derivation (`internal/skillset`)**: Computes `derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)`; derived skills cannot drop (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:38-54`). *Breaks*: Divergent derived sets.
- **Resolution & Admission (`internal/skillroots`, `cmd/lucind-ai/packet_authoring.go:32-54`)**: Resolves via `.lucind/skill-roots.yaml` (`~` expanded); `admitDispatchBatch` rejects missing skills or sets over budget 3. *Breaks*: Missing-skill execution failures; context bloat.
- **Dual Delivery (`internal/packetauthor/compile.go:171-183`, `internal/executor/executor.go:20-39`)**: Prompt renders `## Required skills`; process env sets `LUCIND_REQUIRED_SKILLS` JSON array. *Breaks*: Prompt or env-reliant agents miss skills.
- **Envelope Declaration (`internal/result/result.go:103-116`, `internal/result/result.schema.json:7-165`)**: Result declares `skills_loaded` (`additionalProperties: false` at `internal/result/result.schema.json:5`). *Breaks*: Schema errors or dropped fields.
- **Run-Time Demotion (`internal/run/run.go:882-904`)**: `enforceRequiredSkills` demotes missing declared skills to `lane.Deviated` (`internal/lane/status.go:11-17`). *Breaks*: Deficient lanes report `lane.Done`.
- **Acceptance Verification (`internal/accept/accept.go:263-328,275-286`)**: `validateVersionedEvidence` checks candidate against frozen contract; requires declared skills to satisfy required skills. *Breaks*: Mutated evidence accepted.

## Decision 1 — `lane_role` closed-set values and frontmatter wiring

**Choice**: Frontmatter key `lane_role` validated against `{lens, synthesis, apply, verify, archive, ultrafixer, human}` (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:38-54`). `packet.Parse` (`internal/packet/packet.go:122-179`) validates `lane_role`; when present, validates `sdd_phase` against `{explore, propose, spec, design, tasks, apply, verify, archive}`. Omission leaves `p.LaneRole` as `""`. Unknown values return `ErrInvalidLaneRole`.
**Alternatives considered**: Numeric enums (violates frontmatter conventions); boolean flags (risk combinatorial errors).
**Rationale**: Grounded in lower-snake-case parsing in `internal/packet/packet.go:122-179`. Closed validation ensures deterministic derivation in `internal/skillset`.
**Terminal consumer**: `internal/packet/packet.go:122-179` (`Parse`), consumed by `cmd/lucind-ai/packet_authoring.go:32-54` (`admitDispatchBatch`) and `internal/packetauthor/compile.go:15-25` (`normalizedContract`).

## Decision 2 — `## Required skills` rendered body format (resolves how dual delivery's body half looks)

**Choice**: In `internal/packetauthor/compile.go:171-183` (`renderBody`), when `contract.RequiredSkills` is non-empty, emit `## Required skills` before `## Return`:
```markdown
## Required skills
- <resolved-path-1>
- <resolved-path-2>
```
Rendered as `- %s\n` with resolved paths. Omitted when empty.
**Alternatives considered**: Markdown tables (token overhead); bare names (unusable without ambient resolution).
**Rationale**: Matches list formatting in `internal/packetauthor/compile.go:171-183`.
**Terminal consumer**: `internal/packetauthor/compile.go:171-183` (`renderBody`), consumed by agents parsing `Packet.Body` (`internal/packet/packet.go:43-103`).

## Decision 3 — `lucind.yaml` and `.lucind/skill-roots.yaml` file naming (resolves Open Question 5)

**Choice**: Retain `lucind.yaml` at repo root for tracked config (stack skills per role, unknown key rejection) and `.lucind/skill-roots.yaml` for machine-local roots (`openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:169-175`).
**Alternatives considered**: `.lucind.yaml` dotfile (hides tracked config); merging roots into `lucind.yaml` (violates `.gitignore:1-3` machine-local separation).
**Rationale**: Repo root inspection confirms no collision (`go.mod`, `Makefile`, `openspec/config.yaml`). In Go toolchains (`.golangci.yml`), `lucind.yaml` has no conflicts. Roots stay gitignored in `.lucind/` (`.gitignore:1-3`).
**Terminal consumer**: `internal/lucindconfig` (parses `lucind.yaml`) and `internal/skillroots` (parses `.lucind/skill-roots.yaml`).

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|
| `packet.Packet` | `internal/packet/packet.go:43-103` | Add `LaneRole string`, `RequiredSkills []string` | Yes (additive) |
| `packet.Parse` | `internal/packet/packet.go:122-179` | Add `lane_role` parse case with 7-value closed set, validate `sdd_phase`, add `ErrInvalidLaneRole` | Yes (omitted parses unchanged) |
| `packetauthor.Contract` | `internal/packetauthor/contract.go:45-56` | Add `LaneRole`, `RequiredSkills` fields | Yes (additive JSON) |
| `packetauthor.normalizedContract` | `internal/packetauthor/compile.go:15-25` | Add `LaneRole`, `RequiredSkills` fields | Yes (additive JSON) |
| `packetauthor.renderBody` | `internal/packetauthor/compile.go:171-183` | Emit `## Required skills` list when non-empty | Yes (omitted when empty) |
| `executor.requestEnv` | `internal/executor/executor.go:20-39` | Strip inherited `LUCIND_REQUIRED_SKILLS`, inject JSON array | Yes (additive env var) |
| `executor.Request` | `internal/executor/executor.go:50-75` | Add `RequiredSkills []string` | Yes (additive) |
| `result.Envelope` | `internal/result/result.go:103-116` | Add `SkillsLoaded []string `json:"skills_loaded,omitempty"`` | Yes (additive optional) |
| `result.schema.json` | `internal/result/result.schema.json:7-165` | Add `skills_loaded` string array under `properties` (`additionalProperties: false` at `:5`) | Yes (optional property) |
| `accept.validateVersionedEvidence` | `internal/accept/accept.go:275-286` | Add `LaneRole`, `RequiredSkills` to inline decode struct; assert correspondence | Yes (handles nil/empty legacy) |
| `run.enforceRequiredSkills` | `internal/run/run.go:882-904` | Add check beside `enforceAllowedPaths` demoting shortfalls to `lane.Deviated` | Yes (no-op when empty) |
| `ledger.LaneCandidate` & `SetDoneCandidate` | `internal/ledger/acceptance.go:20-33,83-92` | Untouched; `Contract` is `json.RawMessage` (`internal/ledger/authoring.go:26`), SQL columns unchanged | Yes (schema unchanged) |

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/packet/packet.go` | Modify | Add `LaneRole`, `RequiredSkills` to `Packet` and `Parse` (`internal/packet/packet.go:43-103,122-179`) | `cmd/lucind-ai/packet_authoring.go:32-54` (`admitDispatchBatch`) |
| `internal/packetauthor/contract.go` | Modify | Add `LaneRole`, `RequiredSkills` to `Contract` (`internal/packetauthor/contract.go:45-56`) | `internal/packetauthor/compile.go:32-47` (`Compile`) |
| `internal/packetauthor/compile.go` | Modify | Add fields to `normalizedContract` and `renderBody` (`internal/packetauthor/compile.go:15-25,171-183`) | `internal/executor/executor.go:20-39` (`requestEnv`) |
| `internal/executor/executor.go` | Modify | Add `RequiredSkills` to `Request` and `requestEnv` (`internal/executor/executor.go:20-39,50-75`) | Child agent environment (`os.Environ`) |
| `internal/result/result.go` | Modify | Add `SkillsLoaded` to `Envelope` (`internal/result/result.go:103-116`) | `internal/run/run.go:870-874` (`decideStatus`) |
| `internal/result/result.schema.json` | Modify | Add `skills_loaded` property (`internal/result/result.schema.json:7-165`) | `internal/result/result.go:138-145` (`result.Read`) |
| `internal/result/schema_test.go` | Modify | Add reflection test locking `Envelope` to schema (`internal/result/schema_test.go:1-33`) | `go test ./internal/result` |
| `internal/run/run.go` | Modify | Add `enforceRequiredSkills` demoting to `lane.Deviated` (`internal/run/run.go:882-904`) | `internal/ledger/acceptance.go:55-105` (`SetDoneCandidate`) |
| `internal/accept/accept.go` | Modify | Update inline decode struct in `validateVersionedEvidence` (`internal/accept/accept.go:275-286`) | `cmd/lucind-ai/cli.go:684-687` (`accept` command) |
| `internal/accept/authoring_evidence_test.go` | Modify | Add `required_skills` mutation test (`internal/accept/authoring_evidence_test.go:56-127`) | `go test ./internal/accept` |
| `internal/skillset/skillset.go` | Create | Implement 3-tier derivation union (`derived ∪ stack ∪ adhoc`) | `cmd/lucind-ai/packet_authoring.go:32-54` (`admitDispatchBatch`) |
| `internal/skillroots/skillroots.go` | Create | Implement `.lucind/skill-roots.yaml` resolution with `~` expansion | `cmd/lucind-ai/packet_authoring.go:32-54` (`admitDispatchBatch`) |
| `internal/lucindconfig/config.go` | Create | Implement `lucind.yaml` parser with unknown key rejection | `internal/skillset/skillset.go` (`Derive`) |
| `internal/phasespec/phasespec.go` | Create | Implement SDD phase specialist adapter reading `sdd-status` | `cmd/lucind-ai/cli.go:684-687` (`phase` subcommand) |

## Open Questions

- [ ] Precedence conflict: `~/.claude/skills/sdd-design/SKILL.md` prescribes an 800-word budget, Engram persistence, and a phase-summary return block; this packet's parameters followed.

## Citation Manifest

| citation | claim |
|---|---|
| `.gitignore:1-3` | `.lucind/` directory is gitignored, ensuring machine-local skill-roots.yaml is never tracked |
| `cmd/lucind-ai/cli.go:684-687` | accept command invokes acceptVerifier and reports acceptance receipt errors |
| `cmd/lucind-ai/packet_authoring.go:32-54` | admitDispatchBatch performs batch-level admission before worktree allocation |
| `internal/accept/accept.go:263-328` | validateVersionedEvidence verifies candidate evidence against frozen contract and results |
| `internal/accept/accept.go:275-286` | Duplicated inline struct decoding Contract from AuthoringEvidence |
| `internal/accept/authoring_evidence_test.go:56-127` | TestValidateVersionedResultRequiresExactFrozenCorrespondence tests frozen contract mutations |
| `internal/dag/parse.go:45-54` | YAML parsing pattern used for sidecar and config file decoding |
| `internal/executor/executor.go:20-39` | requestEnv strips and injects LUCIND_READ_ONLY_PATHS as JSON array env var |
| `internal/executor/executor.go:50-75` | Request struct defining prompt, worktree path, and input scope |
| `internal/lane/status.go:11-17` | Terminal lane status definitions including Done, Blocked, Deviated, and Failed |
| `internal/ledger/acceptance.go:20-33` | LaneCandidate struct definition frozen before acceptance verification |
| `internal/ledger/acceptance.go:55-105` | SetDoneCandidate atomically updates lane status and inserts lane_candidates |
| `internal/ledger/acceptance.go:83-92` | SQL INSERT statement for lane_candidates persistence |
| `internal/ledger/acceptance.go:96-103` | SetDoneCandidate falls back to reflect.DeepEqual on existing candidate record |
| `internal/ledger/authoring.go:14` | AuthoringEvidenceVersion constant lane-authoring-evidence/v1 |
| `internal/ledger/authoring.go:20-42` | AuthoringEvidence struct definition carrying versioned candidate contract |
| `internal/ledger/authoring.go:26` | Contract field in AuthoringEvidence is json.RawMessage, allowing schema-free blob storage |
| `internal/ledger/authoring.go:44-75` | FreezeAuthoringEvidence and DecodeAuthoringEvidence hashing and verification logic |
| `internal/packet/packet.go:43-103` | Packet struct definition fields representing frontmatter metadata |
| `internal/packet/packet.go:122-179` | Parse frontmatter key switch block and value unmarshaling |
| `internal/packetauthor/compile.go:15-25` | normalizedContract struct definition compiled from input Contract |
| `internal/packetauthor/compile.go:32-47` | Compile function validating contract, binding, and generating digest |
| `internal/packetauthor/compile.go:171-183` | renderBody function formatting packet Markdown prompt sections |
| `internal/packetauthor/contract.go:45-56` | Contract struct definition for packet authoring input |
| `internal/result/result.go:103-116` | Envelope struct definition mirroring result.schema.json |
| `internal/result/result.go:138-145` | Read function validating JSON result envelope against embedded schema |
| `internal/result/result.schema.json:5` | additionalProperties set to false enforcing strict result validation |
| `internal/result/result.schema.json:7-165` | Schema properties definitions for packet result envelope |
| `internal/result/schema_test.go:1-33` | Unit tests verifying schema parsing and defensive copying |
| `internal/run/run.go:870-874` | decideStatus mapping Envelope to lane.Status |
| `internal/run/run.go:876-904` | enforceAllowedPaths inspecting git diff and demoting out-of-scope changes |
| `internal/run/run.go:882-904` | enforceAllowedPaths implementation logic demoting to lane.Deviated |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:38-54` | Selected Candidate approach detailing derivation, resolution, contract blob, and lane_role |
| `openspec/changes/skill-provisioning-and-phase-specialist/proposal.md:169-175` | Open questions list including Open Question 5 on lucind.yaml file naming |
