# Design Lens A — Decisions: Skill Provisioning and the SDD Phase Specialist

## Assumed architecture

This design adds four new internal packages (`internal/skillset`, `internal/skillroots`, `internal/lucindconfig`, `internal/phasespec`) to implement three-tier skill derivation, machine-local root resolution, repository configuration parsing, and SDD phase orchestration. Existing packet frontmatter parsing in `internal/packet/packet.go` and typed contracts in `internal/packetauthor/contract.go` and `internal/packetauthor/compile.go` gain `lane_role`, `adhoc_skills`, and `required_skills` fields. Runtime demotion is added to `internal/run/run.go` beside allowed paths enforcement, receipt verification in `internal/accept/accept.go` decodes the extended contract JSON from unchanged v1 authoring evidence, and CLI dispatch in `cmd/lucind-ai/cli.go` and `cmd/lucind-ai/packet_authoring.go` gains admission budget validation and the `phase` subcommand.

## Technical Approach

We implement Candidate 1 from the proposal:
1. **Three-Tier Derivation** (Proposal Item 1): `internal/skillset` computes `derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)`. Derived skills (`lucind-executor`, lens/phase skills) are mandatory and never dropped. Stack skills are loaded from `lucind.yaml` via `internal/lucindconfig`, and ad-hoc skills come from authored input.
2. **Local Root Resolution** (Proposal Item 2): `internal/skillroots` maps skill names to `<root>/<skill>/SKILL.md` using ordered search paths from `.lucind/skill-roots.yaml` with tilde (`~`) expansion.
3. **Contract Embedding** (Proposal Item 3): Required skills ride inside `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:20-42`) as JSON without modifying `AuthoringEvidence` struct fields or incrementing `AuthoringEvidenceVersion` from v1 (`internal/ledger/authoring.go:14`).
4. **Dual Delivery** (Proposal Item 4): `internal/packetauthor/compile.go:171-183` renders resolved paths in packet bodies under `## Required skills`, while `internal/executor/executor.go:20-39` injects `LUCIND_REQUIRED_SKILLS`.
5. **Two-Site Enforcement** (Proposal Item 5): `internal/run/run.go:876-904` demotes missing envelope skill declarations to `lane.Deviated`, and `internal/accept/accept.go:263-328` enforces exact receipt correspondence against frozen evidence.
6. **Non-Intercepting Specialist** (Proposal Item 6): `internal/phasespec` queries `gentle-ai sdd-status --json`, dispatches phase packets through existing admission and execution machinery, and writes canonical artifacts to `openspec/changes/<change>/<phase>.md` while gentle-ai retains phase gate authority.

## Decision 1 — Ad-hoc authoring surface shape (resolves Open Question 1)

**Choice**: Both: a typed `AdhocSkills []string` field on `packetauthor.Contract` and an optional `adhoc_skills` frontmatter key (JSON array of strings) in `packet.Packet`.
**Alternatives considered**: Frontmatter key only (forces typed authoring to serialize ad-hoc skills into unstructured comments) vs typed `Contract` field only (leaves manual Markdown packets unable to declare ad-hoc skills).
**Rationale**: Manual Markdown packets bypass `packetauthor.Contract` compilation during batch admission (`cmd/lucind-ai/packet_authoring.go:44-50`), parsing frontmatter directly via `packet.Parse` (`internal/packet/packet.go:122-179`). Providing both surfaces ensures typed authoring compilers (`internal/packetauthor/compile.go:15-25`) and manual packet dispatches maintain parity without dropping ad-hoc skill capabilities.
**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:44-50`

## Decision 2 — Budget default and override shape (resolves Open Question 3)

**Choice**: Both: a package constant `DefaultSkillBudget = 3` in `internal/skillset`, overridable per repository via an optional integer `skill_budget` in `lucind.yaml` parsed by `internal/lucindconfig`.
**Alternatives considered**: Hard-coded constant only (prevents customization for complex multi-stack projects) vs config-only specification (breaks zero-config operation by requiring `lucind.yaml` in every repo).
**Rationale**: `admitDispatchBatch` in `cmd/lucind-ai/packet_authoring.go:32-54` operates as the single pre-worktree admission seam. Evaluating total skill count against the effective budget before lane allocation fails closed deterministically, preventing prompt bloat without creating orphan worktrees or consuming lane quotas.
**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32-54`

## Decision 3 — Specialist CLI shape (resolves Open Question 4)

**Choice**: One parameterized subcommand `lucind-ai phase <name>` in `cmd/lucind-ai/cli.go` and one unified `.opencode/agent/lucind-packet-author.md` agent profile.
**Alternatives considered**: Six individual per-phase CLI subcommands (`lucind-ai explore`, `lucind-ai propose`, etc.) and six distinct opencode agent profiles.
**Rationale**: The SDD lifecycle consists of six fixed phases sharing identical fan-out/synthesis execution mechanics and strict tool-denial boundaries (`.opencode/agent/lucind-packet-author.md:6-7`). Parameterizing `lucind-ai phase <name>` integrates directly into the existing string-switch dispatch table (`cmd/lucind-ai/cli.go:150-168`) without duplicating CLI handlers or multiplying identical agent profiles.
**Terminal consumer**: `cmd/lucind-ai/cli.go:150-168`

## Decision 4 — `internal/skillset` derivation function shape

**Choice**: Pure package function `func Derive(sddPhase, laneRole string, stackSkills, adhocSkills []string) ([]string, error)`. Returns a deduplicated, sorted slice of required skill names with derived skills guaranteed.
**Alternatives considered**: Method on a stateful config struct vs an untyped variadic signature without distinct stack/adhoc inputs.
**Rationale**: Derivation is a pure deterministic mapping dependent solely on `(sdd_phase, lane_role)` and explicit stack/ad-hoc name lists. A pure function maximizes testability and allows direct integration into both compiler validation (`internal/packetauthor/compile.go:32-47`) and admission batch processing (`cmd/lucind-ai/packet_authoring.go:32-54`).
**Terminal consumer**: `internal/packetauthor/compile.go:32-47`

## Decision 5 — `internal/skillroots` resolution and `lucind.yaml`/`skill-roots.yaml` loading

**Choice**: `.lucind/skill-roots.yaml` specifies an ordered list of search roots with `~` expanded to user home directory; first matching `<root>/<skill>/SKILL.md` wins. `lucind.yaml` is parsed using `yaml.NewDecoder` with `KnownFields(true)` to fail closed on any unrecognized key.
**Alternatives considered**: Unordered multi-match ambiguity errors vs permissive `yaml.Unmarshal` for `lucind.yaml`.
**Rationale**: Following the YAML parser pattern in `internal/dag/parse.go:45-56`, `KnownFields(true)` prevents silent typos and unauthorized configuration drift in `lucind.yaml`. Ordered root resolution provides deterministic local overrides over global defaults while keeping machine-local paths out of versioned hashes.
**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32-54`

## Decision 6 — `internal/phasespec` specialist composition

**Choice**: `internal/phasespec` executes `gentle-ai sdd-status --json` via subprocess, parses active phase status, builds lens lane packets, submits them through `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`), waits for lens lanes to be accepted and merged, dispatches synthesis, and writes the canonical output to `openspec/changes/<change>/<phase>.md`.
**Alternatives considered**: Intercepting or wrapping `gentle-ai` CLI processes vs writing phase documents directly without lucind-ai lane execution.
**Rationale**: Preserves gentle-ai as the sole authority over phase gates and status lifecycles while leveraging lucind-ai's existing worktree isolation, ledger records, and acceptance receipts.
**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32-54`

## Open Questions

- None. (All architectural choices for skill derivation, root resolution, budget overrides, and specialist CLI integration are definitively resolved).

## Citation Manifest

| citation | claim |
|---|---|
| `.opencode/agent/lucind-packet-author.md:6-7` | Declares permission deny denying all agent tool access for packet authoring. |
| `cmd/lucind-ai/cli.go:150-168` | Subcommand dispatch table matching string arguments to CLI command handlers. |
| `cmd/lucind-ai/cli.go:684-687` | Executes acceptance verification and renders receipt or prints error without state mutation. |
| `cmd/lucind-ai/packet_authoring.go:32-54` | Admission seam validating batch items and resolving target references before worktree allocation. |
| `cmd/lucind-ai/packet_authoring.go:44-50` | Constructs manual packet batch items directly from packet frontmatter fields without typed contracts. |
| `internal/accept/accept.go:263-328` | Validates candidate changes, result status, criteria, and stops against decoded authoring evidence. |
| `internal/accept/accept.go:275-286` | Anonymous struct definition used to decode and verify contract JSON within authoring evidence. |
| `internal/dag/parse.go:22-37` | Defines DAG Node struct representing sidecar task packet declarations. |
| `internal/dag/parse.go:45-56` | Reads file data and unmarshals YAML configuration into typed data structures. |
| `internal/ledger/authoring.go:14` | Declares AuthoringEvidenceVersion constant as lane-authoring-evidence/v1. |
| `internal/ledger/authoring.go:20-42` | Defines AuthoringEvidence struct containing Contract json.RawMessage field. |
| `internal/ledger/authoring.go:44-60` | Computes SHA-256 hash across serialized AuthoringEvidence payload fields. |
| `internal/ledger/authoring.go:62-75` | Decodes stored authoring evidence and verifies payload against recorded SHA-256 hash. |
| `internal/packet/packet.go:43-103` | Defines Packet struct holding frontmatter metadata and Markdown prompt body. |
| `internal/packet/packet.go:122-179` | Parses frontmatter delimiter and keys via string switch without closed-set validation. |
| `internal/packet/packet.go:159-164` | Frontmatter switch cases extracting sdd_phase, fanout_group, and skill as raw strings. |
| `internal/packetauthor/compile.go:15-25` | Defines normalizedContract struct representing normalized contract JSON schema. |
| `internal/packetauthor/compile.go:32-47` | Compiles validated contract and bound target into deterministic artifact with digest. |
| `internal/packetauthor/compile.go:171-183` | Renders Markdown packet prompt body from normalized contract fields. |
| `internal/packetauthor/contract.go:45-56` | Defines Contract struct containing route intent, mode, path scopes, and result obligations. |
| `internal/packetauthor/contract.go:95-99` | Defines BatchItem struct representing either a manual packet or a typed contract. |
| `internal/run/run.go:876-904` | Enforces allowed paths by inspecting worktree diff and demoting unauthorized changes to lane.Deviated. |
