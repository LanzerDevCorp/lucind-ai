# Proposal Lens A — Candidate & Approach: Skill Provisioning and the SDD Phase Specialist

## Selected Candidate & Approach

We select Candidate 1 from exploration: **Deterministic Three-Tier Skill Provisioning with a Non-Intercepting SDD Phase Specialist**.

Today, executor skills under `.agents/skills/` are unreferenced in Go and discovered only if an agent runtime scans `.agents/skills` (`.agents/skills/lucind-executor/SKILL.md:4-17`), while packet templates hardcode 21 absolute `$HOME`-dependent paths in prose. The `skill:` frontmatter key (`internal/packet/packet.go:122-179`) is purely audit metadata, and nothing enforces skill loading.

This approach establishes a deterministic provisioning and enforcement pipeline paired with a per-phase specialist:

1. **Deterministic Derivation**: Required skills are computed as a pure function `derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)` in `internal/skillset/`. Derived skills are mandatory and never dropped; stack skills are loaded from tracked repo config `lucind.yaml`; ad-hoc skills are authored in the contract. A budget (default 3) rejects oversized sets at admission before allocation.
2. **Machine-Local Root Resolution**: `internal/skillroots/` resolves skill names to `SKILL.md` paths using ordered roots in `.lucind/skill-roots.yaml` with `~` expansion, keeping rendered contracts stable across environments.
3. **Contract Embedding Without Schema Migration**: Required skills ride inside `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:20-42`), a `json.RawMessage` escape hatch. `AuthoringEvidenceVersion` remains `lane-authoring-evidence/v1` (`internal/ledger/authoring.go:14`), preserving byte-identical hash verification for existing records (`internal/ledger/authoring.go:44-75`).
4. **Dual Delivery**: Skills are rendered in the packet markdown body via `packetauthor.renderBody` (`internal/packetauthor/compile.go:171-183`) and delivered to executor environments via `requestEnv` (`internal/executor/executor.go:20-39`).
5. **Two-Site Enforcement**: `run.enforceRequiredSkills` (`internal/run/run.go:875-904`) demotes shortfalls to `lane.Deviated` (`internal/lane/status.go:11-17`), while `accept.validateVersionedEvidence` (`internal/accept/accept.go:263-296`) enforces exact match against frozen evidence.
6. **Non-Intercepting Phase Specialist**: `internal/phasespec/` reads typed `gentle-ai sdd-status` JSON, plans fan-out lanes, dispatches via `cmd/lucind-ai`, and writes canonical artifacts. gentle-ai retains sole authority; specialist outputs are bounded by forbidden authority/render keys (`internal/packetauthor/specialist.go:155-165`).

## Conceptual Changes & Architecture Rationale

- **Lane Role Vocabulary**: Introduce `lane_role` (`lens`, `synthesis`, `apply`, `verify`, `archive`) to frontmatter (`internal/packet/packet.go:122-179`) and `Contract` (`internal/packetauthor/contract.go:45-56`). This enables closed-vocabulary validation and unambiguous mapping to role skills.
- **Repository Configuration**: Introduce `lucind.yaml` for versioned role-to-skill mappings and `.lucind/skill-roots.yaml` for machine-local search roots, following the YAML parser pattern in `internal/dag/parse.go:45-56`.
- **Result Envelope Evolution**: Add `skills_loaded: []string` to `internal/result/result.schema.json:1-16` and `Envelope` (`internal/result/result.go:103-116`). Pin schema and struct in `internal/accept/authoring_evidence_test.go:56-127`.
- **Contract Verification Synchronization**: Synchronize `internal/accept/accept.go:275-286` by adding `RequiredSkills` and `LaneRole` to the hand-duplicated contract decode struct, closing the verification bypass.
- **Receipt Verification Integrity**: Preserve `accept.go`'s invariant as a pure receipt verifier (`internal/accept/accept.go:1-2,213-261`) while pairing it with operational post-dispatch demotion in `run.go` (`internal/run/run.go:875-904`).
- **Executor Decoupling**: Remove runtime-specific naming from executor skills (`.agents/skills/lucind-executor/SKILL.md:4-17`, `.agents/skills/lucind-verify/SKILL.md:4`), aligning with `executor.Executor` abstraction (`internal/executor/executor.go:110-122`).

## Alternatives Considered & Rejected

- **Specialist-Side Skill Selection**: Rejected because `lucind-packet-author` has `permission: "*": deny` and no tools (`.opencode/agent/lucind-packet-author.md:1-8`). Specialist-side inference would introduce non-determinism and violate replay stability.
- **Ledger Schema Migration (v10)**: Rejected because `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:20-42`) can carry new JSON fields without invalidating hash checks in `DecodeAuthoringEvidence` (`internal/ledger/authoring.go:62-75`).
- **gentle-ai Command Interception / Wrapping**: Rejected because gentle-ai gates phases by content and path rather than process provenance. Wrapping would add over 1,500 lines of brittle glue without architectural benefit.
- **Multi-PR / Phased Rollout**: Rejected based on maintainer decision D4 and authorized single-PR review budget of 10,000 lines (`openspec/config.yaml:6-7`), which supersedes exploration recommendations.

## Open Questions

- [ ] **Delivery Channel Uniformity**: Should required skills be delivered solely via the rendered packet body (`packetauthor.renderBody`), via `LUCIND_REQUIRED_SKILLS` in `requestEnv` (`internal/executor/executor.go:20-39`), or dual-channel? (Recommended: dual-channel).
- [ ] **Ad-hoc Authoring Surface**: Should ad-hoc skills be specified via a frontmatter list (`skills: [...]`) or strictly via typed contract fields?
- [ ] **Missing Role Skills**: Should `archive` and `ultrafixer` child skills be created as stubs or declared derived-empty in the phase-role table?
- [ ] **Budget Default Calibration**: Confirm if the default limit of 3 skills should be configurable per project in `lucind.yaml`.
- [ ] **Specialist Granularity**: Implement as a single parameterized CLI subcommand (`lucind-ai phase <name>`) versus individual per-phase subcommands.

## Citation Manifest

| citation | claim |
|---|---|
| `.agents/skills/lucind-executor/SKILL.md:4-17` | Shows executor skill coupled to agy runtime conventions and omitting other executors. |
| `.agents/skills/lucind-verify/SKILL.md:4` | Shows verification lane documentation embedding executor naming in lane convention. |
| `.opencode/agent/lucind-packet-author.md:1-8` | Shows packet author agent configured with denied permissions and zero tools. |
| `internal/accept/accept.go:1-2` | States accept package is a receipt verifier that never promotes candidates or mutates refs. |
| `internal/accept/accept.go:213-261` | Validates result status, criteria, stops, external changes, and scopes candidate changes. |
| `internal/accept/accept.go:263-296` | Verifies frozen authoring evidence against decoded contract and binding integrity. |
| `internal/accept/accept.go:275-286` | Hand-duplicated anonymous contract struct used to decode and verify frozen authoring evidence. |
| `internal/accept/authoring_evidence_test.go:56-127` | Asserts exact correspondence between lane candidate result, frozen evidence, and contract. |
| `internal/dag/parse.go:45-56` | Demonstrates repository YAML unmarshaling precedent using yaml.Unmarshal. |
| `internal/executor/executor.go:20-39` | Injects read-only paths into subprocess environment via requestEnv and LUCIND_READ_ONLY_PATHS. |
| `internal/executor/executor.go:110-122` | Defines Executor interface with Run, DefaultModel, and KnownModels methods. |
| `internal/lane/status.go:11-17` | Defines lane lifecycle status constants including terminal states Done, Blocked, Deviated, and Failed. |
| `internal/ledger/authoring.go:14` | Declares constant AuthoringEvidenceVersion as lane-authoring-evidence/v1. |
| `internal/ledger/authoring.go:20-42` | Defines AuthoringEvidence struct containing Contract json.RawMessage escape hatch. |
| `internal/ledger/authoring.go:44-60` | Serializes and computes domain-separated SHA-256 hash over AuthoringEvidence payload. |
| `internal/ledger/authoring.go:62-75` | Re-runs freeze on stored authoring evidence and rejects any hash or version mismatch. |
| `internal/packet/packet.go:122-179` | Parses packet frontmatter keys with string switches without closed-set enum validation. |
| `internal/packetauthor/compile.go:15-25` | Defines normalizedContract struct matching the frozen contract JSON schema. |
| `internal/packetauthor/compile.go:32-47` | Compiles normalized contract and bound target into deterministic artifact with digest. |
| `internal/packetauthor/compile.go:49-91` | Validates contract fields, paths, modes, and rejects forbidden target claims. |
| `internal/packetauthor/compile.go:171-183` | Renders markdown packet body from normalized contract including Goal, Done criteria, and Hard stops. |
| `internal/packetauthor/contract.go:45-56` | Defines Contract authoring input struct containing Mode, WritePaths, ReadOnlyPaths, and obligations. |
| `internal/packetauthor/specialist.go:155-165` | Rejects specialist output containing forbidden authority keys or forbidden render keys. |
| `internal/result/result.go:103-116` | Defines result Envelope struct mirroring JSON schema fields. |
| `internal/result/result.schema.json:1-16` | Defines JSON schema for result envelope with additionalProperties set to false. |
| `internal/run/run.go:875-904` | Demotes lane to lane.Deviated when actual git changes touch paths outside declared allowed_paths. |
| `openspec/config.yaml:6-7` | Configures delivery_strategy single-pr and review_budget_lines 10000 for this change. |
