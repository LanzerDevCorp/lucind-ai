# Proposal Lens C — Risks, Rollback & Test Impact: Skill Provisioning and the SDD Phase Specialist

## Technical Risks & Failure Modes

| Risk / Failure Mode | Impact | Mitigation | Existing seam (file:line) |
|---|---|---|---|
| Contract decode struct in `accept.go` drops `required_skills` / `lane_role` | Skills in frozen contracts are never verified at acceptance | Add fields to private struct in `internal/accept/accept.go:275-286`; add mutation cases in `internal/accept/authoring_evidence_test.go:56-127` | `internal/accept/accept.go:275-286` |
| Direct additions to `AuthoringEvidence` alter payload bytes | `DecodeAuthoringEvidence` fails on historical rows | Freeze struct at v1 (`internal/ledger/authoring.go:14`, `internal/ledger/authoring.go:20-42`); route skills through `Contract json.RawMessage` (`internal/ledger/authoring.go:44-60`, `internal/ledger/authoring.go:62-75`) | `internal/ledger/authoring.go:20-42` |
| Result schema and struct desync under `additionalProperties: false` | Lanes fail with unreadable envelope errors or drop `skills_loaded` | Add `skills_loaded` to `internal/result/result.schema.json:1-165` and `internal/result/result.go:103-116`; add reflection test in `internal/result/schema_test.go:10-33` | `internal/result/result.go:103-116` |
| Attempting operational demotion in acceptance verifier | `accept.go` is error-only (`internal/accept/accept.go:1-2`, `cmd/lucind-ai/cli.go:684-687`); never sets `lane.Deviated` | Place `enforceRequiredSkills` beside `enforceAllowedPaths` in `internal/run/run.go:882-904` to set `lane.Deviated` (`internal/lane/status.go:11-17`); keep `accept.go:263-328` receipt-only | `internal/run/run.go:882-904` |
| Skill roots or `~` expansion fail to resolve `SKILL.md` | Lanes dispatch with broken skill paths, causing hallucinations | Fail closed at admission in `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`); emit ranked diagnostics (`internal/packetauthor/diagnostic.go:8-39`) | `cmd/lucind-ai/packet_authoring.go:32-54` |
| Uncapped union of skill tiers exceeds token budget | Prompt bloat displaces task instructions | Enforce tier budget arithmetic in `internal/packetauthor/compile.go:49-65`; reject over-budget packets at admission | `internal/packetauthor/compile.go:49-65` |
| Dispatching synthesis before lens lanes merge into parent | Charges lens lines to gentle-ai changed-lines quota | Enforce sequencing invariant: await lens merge before synthesis worktree init (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:24`) | `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:24` |
| Skill hash changes treated as blocking execution gate | Parallel lanes fail from mutual overlap conflicts | Pin repo skills by `base_sha`; record external hashes via `skillcontent.HashDir` (`internal/skillcontent/skillcontent.go:73-100`) as telemetry (`internal/skillcontent/skillcontent.go:1-28`) | `internal/skillcontent/skillcontent.go:1-28` |
| Whole-struct `reflect.DeepEqual` failure in `SetDoneCandidate` | Spurious `ErrImmutableAcceptanceEvidence` on idempotent registration | Scan all candidate struct fields through SQL queries consistently (`internal/ledger/acceptance.go:96-103`) | `internal/ledger/acceptance.go:96-103` |

## Rollback & Additivity

**Rollback Plan**:
1. Revert `## Required skills` in `packetauthor.renderBody` (`internal/packetauthor/compile.go:171-183`) and admission derivation in `cmd/lucind-ai/packet_authoring.go:32-54`; packets compile as before, and `skills_loaded` becomes an ignored envelope field.
2. Revert post-dispatch enforcement in `internal/run/run.go:882-904` and acceptance check in `internal/accept/accept.go:263-328` (`internal/accept/accept.go:275-286`).
3. Revert `internal/result/result.schema.json:1-165` and `internal/result/result.go:103-116` fields.
4. Revert specialist/config packages (`internal/phasespec/`, `internal/skillset/`, `internal/skillroots/`, `internal/lucindconfig/`).
No database schema rollback or ledger row conversion is needed because `AuthoringEvidence` and SQLite tables remain untouched.

**Additivity**:
- `ledger.AuthoringEvidence` (`internal/ledger/authoring.go:20-42`): Additive via `Contract json.RawMessage` (`internal/ledger/authoring.go:20-42`). No fields added to struct; `AuthoringEvidenceVersion` remains `lane-authoring-evidence/v1` (`internal/ledger/authoring.go:14`). Existing frozen rows re-decode identically (`internal/ledger/authoring.go:62-75`).
- SQLite ledger database (`internal/ledger/schema.go:425-445`, `internal/ledger/schema.go:584-592`): 100% additive; schema version remains 10 with zero DDL migrations.
- Result Envelope (`internal/result/result.schema.json:1-165`, `internal/result/result.go:103-116`): Additive; `skills_loaded` is an optional array property (`omitempty`).
- Packet frontmatter (`internal/packet/packet.go:122-179`): Additive; `lane_role` is a new optional key; closed validation on `sdd_phase` triggers only when `lane_role` is present.
- Configuration files (`lucind.yaml` at root, `.lucind/skill-roots.yaml` in gitignored state): Additive new files following YAML parser precedent (`internal/dag/parse.go:45-54`).

## Test & Validation Impact

| Test Layer | Impact / Required Coverage | Existing seam (file:line) |
|---|---|---|
| Unit: Frontmatter & Validation | Test parsing `lane_role` and closed `(sdd_phase, lane_role)` validation in `internal/packet/packet.go:122-179` while preserving legacy compatibility | `internal/packet/packet.go:122-179` |
| Unit: Contract Compilation & Rendering | Test `Compile` (`internal/packetauthor/compile.go:32-47`, `internal/packetauthor/contract.go:45-56`) embeds `required_skills`, generates digests, enforces budget, and renders `## Required skills` (`internal/packetauthor/compile.go:171-183`) | `internal/packetauthor/compile.go:32-47` |
| Unit: Skill Resolution & Path Expansion | Test `skillroots` resolution with `~` expansion, multi-root priority order, and fail-closed error handling on missing files | `internal/dag/parse.go:45-54` |
| Unit: Result Schema & Struct Reflection Pinning | Reflection test ensuring `result.Envelope` fields (`internal/result/result.go:103-116`) match `result.schema.json` properties (`internal/result/result.schema.json:1-165`) under `additionalProperties: false` | `internal/result/schema_test.go:10-33` |
| Integration: Post-Dispatch Lane Enforcement | Test `enforceRequiredSkills` demotes skill shortfalls to `lane.Deviated` (`internal/lane/status.go:11-17`) while passing full sets to `lane.Done` in `internal/run/run.go:882-904` | `internal/run/run.go:882-904` |
| Integration: Acceptance Receipt Verification | Test `validateResultAndScope` (`internal/accept/accept.go:213-260`) and `validateVersionedEvidence` (`internal/accept/accept.go:263-328`) decode `required_skills` via private struct (`internal/accept/accept.go:275-286`) and reject mutations | `internal/accept/authoring_evidence_test.go:56-127` |
| Integration: Frozen Evidence Compatibility | Test `FreezeAuthoringEvidence` (`internal/ledger/authoring.go:44-60`) and `DecodeAuthoringEvidence` (`internal/ledger/authoring.go:62-75`) round-trip with new contracts while legacy v1 rows verify cleanly | `internal/ledger/authoring_evidence_test.go:12-44` |
| Integration: Executor Environment Delivery | Test `requestEnv` (`internal/executor/executor.go:25-39`) and `Request` dispatch (`internal/executor/executor.go:51-79`) isolate environment variables and deliver prompt context | `internal/executor/executor.go:25-39` |
| End-to-End: Phase Specialist Dispatch Loop | Test SDD phase loop (ingest `sdd-status` JSON, plan lanes, dispatch via `admitDispatchBatch` in `cmd/lucind-ai/packet_authoring.go:32-54`, merge lenses, execute synthesis, write artifact) in worktrees (`internal/worktree/worktree.go:159-163`, `internal/accept/accept.go:380-388`) | `cmd/lucind-ai/packet_authoring.go:32-54` |

## Out of Scope

- Modifying `ledger.AuthoringEvidence` struct shape (`internal/ledger/authoring.go:20-42`) or incrementing `AuthoringEvidenceVersion` from v1 (`internal/ledger/authoring.go:14`).
- SQLite ledger database migrations (`internal/ledger/schema.go:425-445`).
- Specialist-side skill selection or trigger matching from registry at compile time.
- Intercepting, wrapping, proxying, or replacing gentle-ai binary execution or lifecycle status authority.
- Reading `openspec/config.yaml` inside the Go binary.
- Authoring skill contents or treating external skill edits as blocking CI/lane gates (`internal/skillcontent/skillcontent.go:1-28`).
- Splitting the change into multiple PRs (superseded by maintainer single-PR size exception).
- Banners and failure guidance strings in CLI (`cmd/lucind-ai/cli.go:699`, `cmd/lucind-ai/cli.go:737`, `cmd/lucind-ai/cli.go:759`, `cmd/lucind-ai/cli.go:2004`).

## Open Questions

- [ ] Delivery channel: confirm whether required skills are delivered via rendered markdown body `## Required skills` (`internal/packetauthor/compile.go:171-183`) or also mirrored into an environment variable (`LUCIND_REQUIRED_SKILLS` via `internal/executor/executor.go:25-39`).
- [ ] Ad-hoc tier syntax: determine whether ad-hoc skills are authored via packet frontmatter key (`adhoc_skills: [...]` in `internal/packet/packet.go:122-179`) or contract field only.
- [ ] Missing role skills: decide whether to create placeholder role skills for `archive` and `ultrafixer` or declare their derived child skills empty.
- [ ] Budget default: validate whether default maximum skill limit of 3 matches empirical token usage across standard dispatches.
- [ ] Specialist granularity: finalize whether phase specialists use dedicated per-phase agent profiles or a single parameterized specialist agent profile.
- [ ] Repository configuration filename: confirm `lucind.yaml` at repo root avoids tooling conflicts with `.lucind/` state directory.

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/cli.go:684-687` | Accept command exit 1 on receipt validation failure |
| `cmd/lucind-ai/cli.go:699` | Acceptance promotion guidance banner |
| `cmd/lucind-ai/cli.go:737` | Troubleshooting guidance banner on lane non-completion |
| `cmd/lucind-ai/cli.go:759` | Reconciliation guidance banner on integrate retry failure |
| `cmd/lucind-ai/cli.go:2004` | Worktree dirty recovery troubleshooting banner |
| `cmd/lucind-ai/packet_authoring.go:32-54` | All-or-nothing pre-dispatch validation seam in `admitDispatchBatch` |
| `internal/accept/accept.go:1-2` | Package doc defining accept as pure receipt verifier without promotion authority |
| `internal/accept/accept.go:213-260` | `validateResultAndScope` verifying result status, criteria, hard stops, and diff scope |
| `internal/accept/accept.go:263-328` | `validateVersionedEvidence` comparing envelope and diff against frozen contract |
| `internal/accept/accept.go:275-286` | Hand-duplicated private contract decode struct in acceptance verifier |
| `internal/accept/accept.go:380-388` | Worktree isolation path calculation formula in acceptance verifier |
| `internal/accept/authoring_evidence_test.go:56-127` | Acceptance correspondence tests validating frozen contract matching and mutation rejection |
| `internal/dag/parse.go:45-54` | YAML sidecar parsing precedent for repository configuration |
| `internal/executor/executor.go:25-39` | `requestEnv` populating execution environment variables |
| `internal/executor/executor.go:51-79` | `Request` struct definition for lane execution |
| `internal/lane/status.go:11-17` | Lane status vocabulary defining `lane.Deviated` and `lane.Done` |
| `internal/ledger/acceptance.go:96-103` | `SetDoneCandidate` using `reflect.DeepEqual` across entire candidate struct |
| `internal/ledger/authoring.go:14` | `AuthoringEvidenceVersion` constant at v1 |
| `internal/ledger/authoring.go:20-42` | `AuthoringEvidence` struct definition with `Contract json.RawMessage` |
| `internal/ledger/authoring.go:44-60` | `FreezeAuthoringEvidence` computing length-prefixed domain hash |
| `internal/ledger/authoring.go:62-75` | `DecodeAuthoringEvidence` enforcing exact re-freeze hash matching |
| `internal/ledger/authoring_evidence_test.go:12-44` | Ledger freezing round-trip and candidate persistence test |
| `internal/ledger/schema.go:425-445` | `migrateV9ToV10DDL` additive migration template |
| `internal/ledger/schema.go:584-592` | Schema migration execution block for v10 |
| `internal/packet/packet.go:122-179` | Packet frontmatter key parser |
| `internal/packetauthor/compile.go:32-47` | `Compile` executing contract validation and artifact creation |
| `internal/packetauthor/compile.go:49-65` | `validateContract` enforcing contract constraints and budget |
| `internal/packetauthor/compile.go:171-183` | `renderBody` compiling markdown packet body with result contract |
| `internal/packetauthor/contract.go:45-56` | `Contract` struct definition |
| `internal/packetauthor/diagnostic.go:8-39` | Ranked diagnostic ordering and sorting |
| `internal/result/result.go:103-116` | `Envelope` Go struct mirroring result schema |
| `internal/result/result.schema.json:1-165` | JSON schema for packet result envelope with `additionalProperties: false` |
| `internal/result/schema_test.go:10-33` | Result schema JSON parse and defensive copy unit tests |
| `internal/run/run.go:882-904` | `enforceAllowedPaths` demoting out-of-scope diffs to `lane.Deviated` |
| `internal/skillcontent/skillcontent.go:1-28` | Package incident header documenting why shared content checks must not block lanes |
| `internal/skillcontent/skillcontent.go:73-100` | `HashDir` deterministic directory tree SHA-256 hashing |
| `internal/worktree/worktree.go:159-163` | `PathFor` deterministic linked worktree sibling path formula |
| `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:24` | Sequencing invariant requiring all lens lanes accepted before synthesis |
