# Spec Lens A — Capabilities & Requirements: Skill Provisioning and the SDD Phase Specialist

## Assumed requirements

This specification defines eight capabilities spanning four new capabilities (`skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`, `phase-specialist-dispatch`) and four modified capabilities (`packet-authoring-contract`, `lane-execution`, `acceptance-verifier`, `read-only-packet-schema`). Each of the eight capabilities receives exactly one requirement statement: four newly added requirements defining multi-tier skill derivation, machine-local root resolution, envelope skill loading, and phase specialist sequencing; and four modified requirements updating contract extension, candidate evidence freezing and shortfall demotion, fail-closed acceptance criteria, and closed-set frontmatter parsing. No requirements are removed or renamed.

## Capability Map

| Capability | New or existing | Target file | Live spec (file:line) |
|---|---|---|---|
| `skill-derivation` | New | `openspec/specs/skill-derivation/spec.md` | |
| `skill-root-resolution` | New | `openspec/specs/skill-root-resolution/spec.md` | |
| `skill-load-correspondence` | New | `openspec/specs/skill-load-correspondence/spec.md` | |
| `phase-specialist-dispatch` | New | `openspec/specs/phase-specialist-dispatch/spec.md` | |
| `packet-authoring-contract` | Existing | `openspec/changes/skill-provisioning-and-phase-specialist/specs/packet-authoring-contract/spec.md` | `openspec/specs/packet-authoring-contract/spec.md:1` |
| `lane-execution` | Existing | `openspec/changes/skill-provisioning-and-phase-specialist/specs/lane-execution/spec.md` | `openspec/specs/lane-execution/spec.md:1` |
| `acceptance-verifier` | Existing | `openspec/changes/skill-provisioning-and-phase-specialist/specs/acceptance-verifier/spec.md` | `openspec/specs/acceptance-verifier/spec.md:1` |
| `read-only-packet-schema` | Existing | `openspec/changes/skill-provisioning-and-phase-specialist/specs/read-only-packet-schema/spec.md` | `openspec/specs/read-only-packet-schema/spec.md:1` |

## ADDED Requirements

### Requirement: Deterministic multi-tier derivation

The system MUST deterministically derive required skills from `(sdd_phase, lane_role)`, unioning derived, stack (`lucind.yaml`), and ad-hoc tiers without dropping derived skills, and MUST reject any candidate set exceeding budget (default 3) at admission before worktree or quota allocation.

**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32`

### Requirement: Root resolution and fail-closed admission

The system MUST resolve skill names to `SKILL.md` paths through `.lucind/skill-roots.yaml` with tilde expansion, and MUST fail batch admission with field-specific diagnostics identifying the missing skill and searched roots if any required skill cannot be resolved.

**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32`

### Requirement: Result envelope skills loaded declaration

The result envelope schema (`.lucind/result.schema.json`) and `result.Envelope` Go struct MUST accept an optional `skills_loaded` property declaring the list of skills loaded by the executing agent, and MUST reject unexpected properties under strict schema validation.

**Terminal consumer**: `internal/result/result.schema.json:1`

### Requirement: Specialist sequencing and canonical artifact generation

The phase specialist MUST ingest `gentle-ai sdd-status` JSON, dispatch child lanes through lucind-ai, MUST NOT start synthesis until all required planning lenses are accepted and merged, and MUST land canonical phase artifacts at `openspec/changes/<change>/<phase>.md`.

**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32`

## MODIFIED Requirements

### Requirement: Versioned Contract and Late Target Binding

An authored contract MUST declare its contract version, route intent, execution mode, write paths, read-only input paths, goal, ordered done criteria, ordered hard stops, result obligations, and optional `lane_role` and `required_skills` declarations carried within `Contract json.RawMessage`. `renderBody` MUST render `## Required skills` with resolved filesystem paths for every required skill. Compilation MUST accept exactly one validated typed target binding and keep `AuthoringEvidence` version at `v1` without live target authority.
(Previously: Authored contracts declared version, paths, criteria, stops, and result obligations without lane_role or rendered required skills.)

**Live block**: `openspec/specs/packet-authoring-contract/spec.md:9`, 3 scenarios

### Requirement: Frozen Authored Candidate Evidence

Before executor work can become a lane candidate, lane execution MUST freeze the exact admitted packet identity and digest, contract version or explicit legacy mode, normalized versioned contract evidence including `required_skills` and `lane_role` when present, typed target binding, execution mode, write paths, read-only paths, and result obligations, and MUST enforce that envelope skill shortfalls demote terminal status to `lane.Deviated`. Later packet, target, or checkout changes MUST NOT alter this evidence.
(Previously: Frozen candidate evidence recorded versioned contract fields and target bindings without required skills or skill shortfall demotion.)

**Live block**: `openspec/specs/lane-execution/spec.md:104`, 3 scenarios

### Requirement: Fail-Closed Mechanical Criteria

The verifier MUST reject a missing or invalid result schema, packet or candidate-commit mismatch, fired hard stop, unmet done criterion, undeclared or out-of-scope change, failed required check, or missing required skill declaration. For versioned contracts it MUST also reject any missing, extra, duplicate, reordered, or altered authored criterion, hard stop, or required skill; mode or commit disagreement; and any path or change-classification mismatch against the canonical frozen candidate change set. A rejected attempt MUST NOT create or reuse a receipt.
(Previously: Mechanical criteria verified criteria, hard stops, checks, and changed paths against frozen evidence without validating required skills correspondence.)

**Live block**: `openspec/specs/acceptance-verifier/spec.md:30`, 5 scenarios

### Requirement: Extended packet frontmatter parsing

Packet parsing MUST accept optional `sdd_phase`, `fanout_group`, `skill`, and `lane_role` frontmatter keys, mapping present values onto the corresponding packet fields, and MUST default omitted keys to empty values. When `lane_role` is present, `packet.Parse` MUST validate it against the closed set `{lens, synthesis, apply, verify, archive, ultrafixer, human}` and closed-validate `sdd_phase`; packets omitting `lane_role` MUST retain open `sdd_phase` parsing without failure.
(Previously: Extended packet frontmatter keys had open schema validation with exact key names left unresolved.)

**Live block**: `openspec/specs/read-only-packet-schema/spec.md:84`, 3 scenarios

## Open Questions

- [ ] None

## Citation Manifest

| citation | claim |
|---|---|
| `cmd/lucind-ai/packet_authoring.go:32` | admitDispatchBatch performs batch admission and target resolution before allocation |
| `internal/result/result.schema.json:1` | Result envelope schema root definition enforcing strict additionalProperties false |
| `openspec/specs/acceptance-verifier/spec.md:1` | Live specification root for existing acceptance-verifier capability |
| `openspec/specs/acceptance-verifier/spec.md:30` | Live requirement block for Fail-Closed Mechanical Criteria with 5 scenarios |
| `openspec/specs/lane-execution/spec.md:1` | Live specification root for existing lane-execution capability |
| `openspec/specs/lane-execution/spec.md:104` | Live requirement block for Frozen Authored Candidate Evidence with 3 scenarios |
| `openspec/specs/packet-authoring-contract/spec.md:1` | Live specification root for existing packet-authoring-contract capability |
| `openspec/specs/packet-authoring-contract/spec.md:9` | Live requirement block for Versioned Contract and Late Target Binding with 3 scenarios |
| `openspec/specs/read-only-packet-schema/spec.md:1` | Live specification root for existing read-only-packet-schema capability |
| `openspec/specs/read-only-packet-schema/spec.md:84` | Live requirement block for Extended packet frontmatter parsing with 3 scenarios |
