# Tasks Lens B — Partition & Dispatch Shape: Skill Provisioning and the SDD Phase Specialist

## Assumed decomposition

Derived from the frozen design File Changes table (`design.md:89-108`), this change decomposes into ten dispatchable units across four implementation tiers: schema/envelope contracts, core skill derivation/resolution engines, packet compilation and execution runtime plumbing, and CLI specialist wiring. The partition isolates four new sibling packages (`internal/skillset/`, `internal/skillroots/`, `internal/lucindconfig/`, `internal/phasespec/`) while grouping tightly coupled runtime digest and acceptance verification pairs into unified work units to preserve correspondence invariants (`design.md:61-66`).

## Suggested Work Units

| Unit | Goal | allowed_paths | Executor | Rollback boundary |
|---|---|---|---|---|
| `result-envelope` | Add optional `skills_loaded` string array to `result.schema.json`, `SkillsLoaded` to `result.Envelope`, and verify reflection pin in `schema_test.go` (`result.go:103-116`, `result.schema.json:7-165`, `schema_test.go:10-33`) | `internal/result/` | `agy` | `internal/result/` |
| `skillset-engine` | Implement `internal/skillset` with pure `Derive`, `DefaultSkillBudget = 3`, `DigestBody`, and unit tests (`design.md:37-42,103`) | `internal/skillset/` (new file) | `agy` | `internal/skillset/` |
| `skillroots-resolution` | Implement `internal/skillroots` with ordered root resolution, `~` path expansion, missing-skill diagnostics, and unit tests (`design.md:43-48,104`) | `internal/skillroots/` (new file) | `agy` | `internal/skillroots/` |
| `lucindconfig-loader` | Implement `internal/lucindconfig` loading `lucind.yaml` via `yaml.NewDecoder` with `KnownFields(true)` and unit tests (`design.md:43-48,105`) | `internal/lucindconfig/` (new file) | `agy` | `internal/lucindconfig/` |
| `packet-contract` | Add `LaneRole`, `AdhocSkills`, `RequiredSkills` to `packet.Packet` and `packetauthor.Contract`/`normalizedContract`; parse `lane_role`/`adhoc_skills`; validate closed role set; render `## Required skills` and hash `DigestBody` (`packet.go:43-103,122-179`, `contract.go:45-56`, `compile.go:15-25,171-183`) | `internal/packet/`, `internal/packetauthor/` | `agy` | `internal/packet/`, `internal/packetauthor/` |
| `executor-env` | Add `RequiredSkills` to executor `Request` and inject `LUCIND_REQUIRED_SKILLS` JSON array in `requestEnv` (`executor.go:20-39,50-75`) | `internal/executor/` | `agy` | `internal/executor/` |
| `phasespec-adapter` | Implement `internal/phasespec` with `sdd-status` adapter, batch admission, and synthesis orchestration (`design.md:106`, `explore.md:282-285`) | `internal/phasespec/` (new file) | `agy` | `internal/phasespec/` |
| `runtime-enforcement-accept` | Implement `enforceRequiredSkills` beside `enforceAllowedPaths` in `run.go`; synchronize `packetDigest` field list with `accept.go` decode struct per Decision 8; add acceptance evidence verification and mutation tests (`run.go:489-491,722-729,876-904`, `accept.go:263-328,275-286`, `authoring_evidence_test.go:56-127`) | `internal/run/`, `internal/accept/` | `agy` | `internal/run/`, `internal/accept/` |
| `agent-prompts-assets` | Update `.agents/skills/` and `plugin/` markdown assets to drop executor-named skills and hardcoded `~/.claude/skills/...` paths (`design.md:108`) | `.agents/skills/`, `plugin/` | `agy` | `.agents/skills/`, `plugin/` |
| `cli-packet-authoring` | Add `lucind-ai phase <name>` CLI subcommand in `cli.go` and pre-worktree `admitDispatchBatch` resolution + budget enforcement in `packet_authoring.go` (`cli.go:142-168`, `packet_authoring.go:32-54`) | `cmd/lucind-ai/` | `agy` | `cmd/lucind-ai/` |

## Wave Plan

| Wave | Units | Runs in parallel | Green on its own |
|---|---|---|---|
| Wave 1 | `result-envelope`, `skillset-engine`, `skillroots-resolution`, `lucindconfig-loader` | Yes | Yes: 4 independent foundation packages and schemas with zero cross-dependencies; compiles and passes `lucind-checks.sh` |
| Wave 2 | `packet-contract`, `executor-env`, `phasespec-adapter` | Yes | Yes: packet/compiler, executor env injection, and phase specialist compile against Wave 1 foundations and pass package test suites |
| Wave 3 | `runtime-enforcement-accept`, `agent-prompts-assets` | Yes | Yes: `run.go` and `accept.go` update in lockstep to satisfy Decision 8 correspondence; markdown prompt updates carry no code breaks |
| Wave 4 | `cli-packet-authoring` | No (single unit) | Yes: wires all completed subsystems into CLI commands and passes admission batch test suites |

## Disjointness Check

Every pair of units sharing a wave was checked by hand against the component-boundary prefix rule (`internal/packet/disjoint.go:8-22`, `internal/packet/disjoint.go:24-48`):

- **Wave 1 — `result-envelope` vs `skillset-engine`**: `["internal/result/"]` vs `["internal/skillset/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 1 — `result-envelope` vs `skillroots-resolution`**: `["internal/result/"]` vs `["internal/skillroots/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 1 — `result-envelope` vs `lucindconfig-loader`**: `["internal/result/"]` vs `["internal/lucindconfig/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 1 — `skillset-engine` vs `skillroots-resolution`**: `["internal/skillset/"]` vs `["internal/skillroots/"]`. While sharing the string prefix `skill`, component-boundary matching normalizes directory segments (`internal/skillset/` vs `internal/skillroots/`), so `PathInScope` evaluates to false. Verdict: Disjoint.
- **Wave 1 — `skillset-engine` vs `lucindconfig-loader`**: `["internal/skillset/"]` vs `["internal/lucindconfig/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 1 — `skillroots-resolution` vs `lucindconfig-loader`**: `["internal/skillroots/"]` vs `["internal/lucindconfig/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 2 — `packet-contract` vs `executor-env`**: `["internal/packet/", "internal/packetauthor/"]` vs `["internal/executor/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 2 — `packet-contract` vs `phasespec-adapter`**: `["internal/packet/", "internal/packetauthor/"]` vs `["internal/phasespec/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 2 — `executor-env` vs `phasespec-adapter`**: `["internal/executor/"]` vs `["internal/phasespec/"]`. Distinct component directories under `internal/`. Verdict: Disjoint.
- **Wave 3 — `runtime-enforcement-accept` vs `agent-prompts-assets`**: `["internal/run/", "internal/accept/"]` vs `[".agents/skills/", "plugin/"]`. Subtrees are entirely disjoint across root and internal directories. Verdict: Disjoint.

## Sidecar Recommendation

**Recommendation**: sidecar warranted
**Rationale**: This change creates 4 new Go packages (`skillset`, `skillroots`, `lucindconfig`, `phasespec`) and modifies 11 files across 16 canonical locations, partitioned into 10 cohesive work units across 4 waves. Wave 1 (4 parallel foundation units) and Wave 2 (3 parallel subsystem units) offer substantial parallel execution throughput across isolated worktrees. Unlike `2026-08-20-apply-dag-dispatch-hardening/tasks.md:26`, which declined an `apply-dag.yaml` sidecar because the change was small (~650–1200 lines across 5 existing files) and Unit 1 was too small to justify orchestration, this change's multi-package scope justifies an `apply-dag.yaml` sidecar (`internal/dag/parse.go:40-60`) for `lucind-ai split` orchestration. Alternatively, if the synthesizer opts for sequential single-packet apply to avoid bisection overhead (`internal/run/integrate.go:50-59`), the 10 units map directly to 10 sequential work-unit commits.

## Open Questions

- [ ] Should the synthesizer collapse Wave 3 (`runtime-enforcement-accept` and `agent-prompts-assets`) and Wave 4 (`cli-packet-authoring`) into a single sequential terminal wave if parallel dispatch of non-code prompt assets is deemed unnecessary?

## Citation Manifest

| Citation | Claim |
|---|---|
| `cmd/lucind-ai/cli.go:142-168` | `lucind-ai` subcommand string switch handling `run`, `split`, `check`, `accept`, and adding `phase` |
| `cmd/lucind-ai/packet_authoring.go:32-54` | `admitDispatchBatch` executes pre-worktree admission batch validation and rejection |
| `internal/accept/accept.go:263-328` | `validateVersionedEvidence` verifies candidate changes and envelope correspondence against frozen evidence |
| `internal/accept/accept.go:275-286` | Decode contract struct in acceptance verifier requiring `LaneRole` and `RequiredSkills` synchronization |
| `internal/accept/authoring_evidence_test.go:56-127` | Authoring evidence test suite verifying contract mutation rejection |
| `internal/dag/parse.go:40-60` | `DAG` struct definition and `Parse` function reading `apply-dag.yaml` sidecar |
| `internal/executor/executor.go:20-39` | `requestEnv` constructs execution environment variables for dispatched lanes |
| `internal/executor/executor.go:50-75` | `Request` struct definition defining prompt, worktree path, and input scope |
| `internal/packet/disjoint.go:8-22` | `PathInScope` implements component-boundary prefix matching rule |
| `internal/packet/disjoint.go:24-48` | `DisjointAllowedPaths` verifies pairwise path disjointness across packets |
| `internal/packet/packet.go:43-103` | `Packet` struct definition fields representing frontmatter metadata |
| `internal/packet/packet.go:122-179` | `packet.Parse` frontmatter switch evaluating key-value pairs |
| `internal/packetauthor/compile.go:15-25` | `normalizedContract` struct definition compiled from input `Contract` |
| `internal/packetauthor/compile.go:171-183` | `renderBody` formats markdown headers and contract details into packet body |
| `internal/packetauthor/contract.go:45-56` | `Contract` struct definition for packet authoring input |
| `internal/result/result.go:103-116` | `Envelope` struct definition representing result JSON payload |
| `internal/result/result.schema.json:7-165` | Result envelope JSON schema property definitions |
| `internal/result/schema_test.go:10-33` | Unit tests verifying schema parsing and defensive copying |
| `internal/run/integrate.go:50-59` | `Integrate` runs checks on combined tree and triggers bisection on failure |
| `internal/run/run.go:489-491` | `runDispatch` invokes `enforceAllowedPaths` and `enforceRequiredSkills` |
| `internal/run/run.go:722-729` | `packetDigest` field list requiring lockstep synchronization with accept decode struct |
| `internal/run/run.go:876-904` | `enforceAllowedPaths` inspects git diff and demotes out-of-scope changes |
| `openspec/changes/archive/2026-08-20-apply-dag-dispatch-hardening/tasks.md:26` | Precedent declining `apply-dag.yaml` sidecar for small changes |
| `openspec/changes/archive/2026-08-20-apply-dag-dispatch/design.md:19-21` | Architecture decision establishing `apply-dag.yaml` sidecar format |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:37-42` | Decision 4 defining `skillset.Derive` pure function signature and behavior |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:43-48` | Decision 5 defining root resolution, `lucind.yaml`, and `.lucind/skill-roots.yaml` |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:61-66` | Decision 8 requiring lockstep synchronization between `packetDigest` and accept decode struct |
| `openspec/changes/skill-provisioning-and-phase-specialist/design.md:89-108` | File Changes table specifying modified files, actions, and terminal consumers |
| `openspec/changes/skill-provisioning-and-phase-specialist/explore.md:282-285` | Phase Specialist token mappings for SDD lifecycle phases |
