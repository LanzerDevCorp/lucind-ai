# Design: Skill Provisioning and the SDD Phase Specialist

## Technical Approach

Candidate 1. Four new packages — `internal/skillset`, `internal/skillroots`, `internal/lucindconfig`, `internal/phasespec` — plus additive fields on existing packet, contract, executor, result, run, and accept surfaces.

`skillset.Derive` computes `derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)`. Derived skills never drop. Stack names come from tracked `lucind.yaml`; ad-hoc names are authored input. `skillroots` maps names to `<root>/<skill>/SKILL.md` via ordered `.lucind/skill-roots.yaml` (gitignored, `.gitignore:2`) with `~` expansion. Names enter the packet digest; absolute paths do not.

`lane_role` and `required_skills` ride inside `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:26`), a `json.RawMessage`. Freeze/decode (`internal/ledger/authoring.go:44-75`) re-hashes the whole struct; version stays `lane-authoring-evidence/v1` (`internal/ledger/authoring.go:14`). Dual delivery: `renderBody` (`internal/packetauthor/compile.go:171-183`) emits `## Required skills`; `requestEnv` (`internal/executor/executor.go:20-39`) injects `LUCIND_REQUIRED_SKILLS` the same way it already injects `LUCIND_READ_ONLY_PATHS`. Two-site enforcement: `enforceRequiredSkills` is added beside `enforceAllowedPaths` (`internal/run/run.go:876-904`) and demotes shortfalls to `lane.Deviated` (`internal/lane/status.go:11-17`); `accept` stays error-only (`internal/accept/accept.go:1-2`, `cmd/lucind-ai/cli.go:684-687`) and `validateVersionedEvidence` (`internal/accept/accept.go:263-328`) re-checks frozen evidence. The duplicated decode struct (`internal/accept/accept.go:275-286`) gains `LaneRole` and `RequiredSkills` in the same commit.

`phasespec` reads `gentle-ai sdd-status --json`, plans lens packets, admits them through `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`), waits until every required lens is accepted and merged (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:24`), dispatches synthesis, and writes `openspec/changes/<change>/<phase>.md`. gentle-ai keeps phase-gate authority. Specialist output stays inside forbidden-key maps (`internal/packetauthor/specialist.go:155-165`).

## Architecture Decisions

### Decision: Ad-hoc authoring surface (Open Question 1)

**Choice**: Both a typed `AdhocSkills []string` on `packetauthor.Contract` (`internal/packetauthor/contract.go:45-56`) and an optional `adhoc_skills` JSON-array frontmatter key on `packet.Packet` (`internal/packet/packet.go:43-103`).
**Alternatives considered**: Frontmatter only; typed field only.
**Rationale**: Manual packets bypass typed compilation and parse frontmatter directly (`cmd/lucind-ai/packet_authoring.go:44-50`, `internal/packet/packet.go:122-179`). Both surfaces keep typed `Compile` (`internal/packetauthor/compile.go:32-47`) and manual dispatch at parity.
**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:44-50`.

### Decision: Budget default and override (Open Question 3)

**Choice**: Package constant `DefaultSkillBudget = 3` in `internal/skillset`, overridable by optional integer `skill_budget` in `lucind.yaml`.
**Alternatives considered**: Hard-coded constant only; config-only (breaks zero-config).
**Rationale**: `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) is the pre-worktree admission seam. Evaluate total skill count against the effective budget there and reject the batch; never trim. Derived skills still cannot drop.
**Terminal consumer**: `cmd/lucind-ai/packet_authoring.go:32-54`.

### Decision: Specialist CLI (Open Question 4)

**Choice**: One parameterized subcommand `lucind-ai phase <name>` in the existing string-switch (`cmd/lucind-ai/cli.go:142-168`) and the existing `.opencode/agent/lucind-packet-author.md` profile (`permission: "*": deny` at lines 6-7).
**Alternatives considered**: Six per-phase subcommands and six agent profiles.
**Rationale**: Six SDD phases share fan-out/synthesis mechanics and tool-denial. One case in the dispatch table avoids duplicated handlers.
**Terminal consumer**: `cmd/lucind-ai/cli.go:142-168`.

### Decision: `Derive` shape

**Choice**: Pure `func Derive(sddPhase, laneRole string, stackSkills, adhocSkills []string) ([]string, error)` returning a deduplicated, sorted name slice with derived skills guaranteed.
**Alternatives considered**: Stateful config method; untyped variadic mix of stack and ad-hoc.
**Rationale**: Mapping depends only on `(sdd_phase, lane_role)` plus explicit name lists. Same function serves `Compile` (`internal/packetauthor/compile.go:32-47`) and admission (`cmd/lucind-ai/packet_authoring.go:32-54`).
**Terminal consumer**: `internal/packetauthor/compile.go:32-47`.

### Decision: Root resolution and `lucind.yaml` loading (Open Question 5)

**Choice**: Tracked `lucind.yaml` at repo root (stack skills per role, optional `skill_budget`). Machine-local `.lucind/skill-roots.yaml` (`.gitignore:1-3`) holds an ordered root list; first matching `<root>/<skill>/SKILL.md` wins; `~` expands to the user home directory. Parse `lucind.yaml` with `yaml.NewDecoder` + `KnownFields(true)` so unknown keys fail closed. Roots load with the same `yaml.Unmarshal` pattern as `internal/dag/parse.go:45-54`.
**Alternatives considered**: `.lucind.yaml` dotfile; merging roots into `lucind.yaml` (would version machine-local paths); unordered multi-match errors; permissive `Unmarshal` for `lucind.yaml`.
**Rationale**: Repo root already has `go.mod`, `Makefile`, and `openspec/config.yaml`; no `lucind.yaml` collision. `KnownFields(true)` is stricter than sidecar `Unmarshal` because `lucind.yaml` is the first tracked config the binary reads and must not silently accept typos.
**Terminal consumers**: `internal/lucindconfig` (`lucind.yaml`), `internal/skillroots` (roots).

### Decision: `phasespec` composition

**Choice**: Subprocess `gentle-ai sdd-status --json`, build lens packets, `admitDispatchBatch`, wait for accept+merge, dispatch synthesis, write the canonical artifact. Never wrap or intercept gentle-ai.
**Alternatives considered**: Intercepting gentle-ai; writing phase documents without lucind-ai lanes.
**Rationale**: Reuses worktree isolation, ledger records, and receipts. Output remains inside `forbiddenSpecialistAuthority` / `forbiddenSpecialistRender` (`internal/packetauthor/specialist.go:155-165`).
**Terminal consumer**: `cmd/lucind-ai/cli.go:142-168` (`phase`).

### Decision: Missing archive/ultrafixer child skills (Open Question 2)

**Choice**: Do not create `lucind-archive` or `lucind-ultrafixer` stubs. Those roles derive only `lucind-executor`.
**Alternatives considered**: Placeholder SKILL.md files under `.agents/skills/`.
**Rationale**: `.agents/skills/` currently has `lucind-executor`, `lucind-fan-out-lens`, `lucind-apply`, and `lucind-verify` only. Admission will fail closed on unresolved required names (`cmd/lucind-ai/packet_authoring.go:32-54`). Stubs are out-of-scope skill authoring.
**Terminal consumer**: `internal/skillset.Derive`.

### Decision: `lane_role` closed set and `## Required skills` body

**Choice**: Optional frontmatter `lane_role` closed over `{lens, synthesis, apply, verify, archive, ultrafixer, human}`. Unknown values return `ErrInvalidLaneRole`. When `lane_role` is present, `sdd_phase` is closed over `{explore, propose, spec, design, tasks, apply, verify, archive}`. Omission leaves `p.LaneRole == ""` and does not closed-validate `sdd_phase` (`internal/packet/packet.go:122-179` today copies `sdd_phase` as a raw string at `:159-164`). `renderBody` emits, when `RequiredSkills` is non-empty, before `## Return`:

```markdown
## Required skills
- <resolved-path>
```

using the existing `- %s\n` list form (`internal/packetauthor/compile.go:171-183`). Omitted when empty.
**Alternatives considered**: Numeric enums; boolean flags; Markdown tables; bare skill names in the body.
**Rationale**: Closed validation makes `Derive` deterministic. Resolved paths are usable without a second lookup. Empty omission keeps legacy packets unchanged.
**Terminal consumers**: `internal/packet/packet.go:122-179`, `internal/packetauthor/compile.go:171-183`.

## Flow and Invariants

```
(sdd_phase, lane_role) + lucind.yaml + adhoc
        → skillset.Derive (union; derived never drop)
        → skillroots resolve + budget check in admitDispatchBatch
        → renderBody  AND  requestEnv(LUCIND_REQUIRED_SKILLS)
        → agent Envelope.skills_loaded
        → enforceRequiredSkills ──shortfall──► lane.Deviated
        → validateVersionedEvidence ──mismatch──► reject, no receipt
```

1. **Derivation.** Same `(sdd_phase, lane_role)` plus stack/ad-hoc names yields a byte-identical sorted set. Break: divergent derived tables.
2. **Admission.** `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) rejects the whole batch before worktree or quota if any required name is missing (naming the skill and roots searched) or the set exceeds budget. Break: missing-skill dispatch; prompt bloat.
3. **Digest.** Canonical skill names enter `normalizedContract` (`internal/packetauthor/compile.go:15-25`); resolved absolute paths do not. Break: machine-local roots changing the digest.
4. **Dual delivery.** Body lists resolved paths; env carries the JSON array. `requestEnv` strips a leaked inherited `LUCIND_REQUIRED_SKILLS` the same way it strips `LUCIND_READ_ONLY_PATHS` (`internal/executor/executor.go:20-39`).
5. **Demotion.** `enforceRequiredSkills` compares derived required names to envelope `skills_loaded`. It does not inspect git diffs (`enforceAllowedPaths` does, via `candidatechange.Collect` at `internal/run/run.go:876-904`). Shortfall → `lane.Deviated`, never `lane.Done`.
6. **Acceptance.** `validateVersionedEvidence` (`internal/accept/accept.go:263-328`) decodes `required_skills` from frozen `Contract` JSON. `accept` never demotes (`cmd/lucind-ai/cli.go:684-687` prints the error and exits 1). `SetDoneCandidate` uses `reflect.DeepEqual` (`internal/ledger/acceptance.go:96-103`); new Go fields on `LaneCandidate` stay out — skills remain in the blob.
7. **Specialist barrier.** Synthesis does not start until every required lens is accepted and merged (`fan-out.md:24`).

## File Changes

| File | Action | Terminal consumer |
|------|--------|-------------------|
| `internal/skillset/skillset.go` | Create `Derive` | `admitDispatchBatch` (`packet_authoring.go:32-54`) |
| `internal/skillroots/skillroots.go` | Create root resolution | `admitDispatchBatch` |
| `internal/lucindconfig/config.go` | Create `lucind.yaml` parser | `skillset.Derive` |
| `internal/phasespec/phasespec.go` | Create specialist adapter | `cli.go:142-168` (`phase`) |
| `internal/packet/packet.go` | Add `LaneRole`, `AdhocSkills`, `RequiredSkills`; parse `lane_role`/`adhoc_skills` | `packet_authoring.go:32-54` |
| `internal/packetauthor/contract.go` | Add `LaneRole`, `AdhocSkills`, `RequiredSkills` | `compile.go:32-47` |
| `internal/packetauthor/compile.go` | Fields on `normalizedContract`; `## Required skills` in `renderBody` | `executor.go:20-39` |
| `internal/executor/executor.go` | `RequiredSkills` on `Request`; inject `LUCIND_REQUIRED_SKILLS` | child `os.Environ` |
| `internal/result/result.go` | Optional `SkillsLoaded` | `enforceRequiredSkills` |
| `internal/result/result.schema.json` | Optional `skills_loaded` array (`additionalProperties: false` at `:5`) | `result.Read` (`result.go:138-145`) |
| `internal/result/schema_test.go` | Reflection pin `Envelope` ↔ schema (today `:10-33` only parse/copy) | `go test ./internal/result` |
| `internal/run/run.go` | `enforceRequiredSkills` beside `:876-904` | `SetDoneCandidate` (`acceptance.go:55-105`) |
| `internal/accept/accept.go` | Decode-struct fields at `:275-286`; correspondence | `cli.go:684-687` |
| `internal/accept/authoring_evidence_test.go` | `required_skills` mutation case (`:56-127` pattern) | `go test ./internal/accept` |
| `cmd/lucind-ai/cli.go` | `phase` case in `:142-168`; budget/admission wiring | operator CLI |
| `cmd/lucind-ai/packet_authoring.go` | Derive, resolve, budget inside `:32-54` | `runDispatch` |
| `.agents/skills/lucind-*` | Drop executor-name coupling | skill loaders |
| `plugin/claude-code/skills/lucind-ai/assets/*.md` | Drop hardcoded `~/.claude/skills/...` paths | `packet.Parse` |

Untouched: `internal/ledger/authoring.go` struct/version; schema v10 (`internal/ledger/schema.go:425-445,584-592`); `LaneCandidate` columns.

## Interfaces / Contracts

```go
func Derive(sddPhase, laneRole string, stackSkills, adhocSkills []string) ([]string, error)
```

Frontmatter: optional `lane_role` (closed set above), optional `adhoc_skills` (JSON string array). Envelope: `skills_loaded` optional string array. Env: `LUCIND_REQUIRED_SKILLS` JSON array. `normalizedContract` gains `lane_role` and `required_skills` (names, not paths).

## Testing Strategy

| Layer | What | Seam |
|-------|------|------|
| Unit | Closed-set `lane_role`/`sdd_phase`; omit-compat | `packet.go:122-179`, `packet_test.go:133-195` |
| Unit | Derive union, budget reject, derived never drop | new `internal/skillset/skillset_test.go` |
| Unit | `~` / relative / absolute roots; missing-name diagnostics | new `internal/skillroots/skillroots_test.go` |
| Unit | `required_skills` in `normalizedContract`, stable digest, `## Required skills` body | `compile.go:171-183`, `compile_test.go:10-34` |
| Unit | Envelope ↔ schema reflection pin | `schema_test.go:10-33` |
| Integration | Shortfall → `lane.Deviated` | `run.go:876-904` |
| Integration | Mutated/missing `required_skills` rejected, no receipt | `authoring_evidence_test.go:56-127` |
| Integration | `LUCIND_REQUIRED_SKILLS` beside `LUCIND_READ_ONLY_PATHS` | `read_only_paths_test.go:1-50` |
| Integration | Freeze/decode with new contract fields; legacy v1 rows still verify | `authoring.go:44-75` |
| E2E | `sdd-status` fixture → admit → lens merge → synthesis → artifact | new `internal/phasespec/specialist_test.go` |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | Applicable | Load `SKILL.md` and skill-roots YAML as data, never execute. `HashDir` (`internal/skillcontent/skillcontent.go:73-100`) is observation only (`:1-28`). Existing scope-only proof: `accept_test.go:125-138`. | Admit and `HashDir` process `SKILL.md` without shell execution or a blocking hash gate. |
| Git repository selection | Applicable | Roots resolve locally; only names enter the digest (`compile.go:15-25`). | Identical digest across differing root prefixes and `~` expansions. |
| Commit state | Applicable | `enforceRequiredSkills` reads the envelope; `validateVersionedEvidence` verifies the detached candidate (`accept_test.go:140-164`). | Accept passes a matching detached tree despite dirty primary; rejects skill mismatch. |
| Push state | N/A: no remote push or refspec mutation | N/A | N/A |
| PR commands | N/A: no PR argv; specialist runs read-only `gentle-ai sdd-status` | N/A | N/A |

Malformed `sdd-status` JSON or CLI errors fail the `phase` command closed without mutating PR or git refs.

## Rollback and Additivity

No migration. Schema stays v10. `AuthoringEvidence` shape and `lane-authoring-evidence/v1` stay put; old rows re-decode (`internal/ledger/authoring.go:62-75`).

1. Stop rendering `## Required skills` and stop deriving at admission. Packets compile as today; `skills_loaded` is ignored.
2. Revert `enforceRequiredSkills` and the accept comparison (`accept.go:263-328,275-286`).
3. Revert envelope schema/struct (`result.schema.json:1-165`, `result.go:103-116`).
4. Revert `internal/phasespec/`, `skillset/`, `skillroots/`, `lucindconfig/`.

Rejected: ledger v11 and `AuthoringEvidenceVersion` v2 — would break freeze/decode on frozen rows.

## Open Questions and Out of Scope

Open Questions 1–5 from `proposal.md:169-175` are closed by the decisions above: (1) both surfaces; (2) derived-empty lucind children for `archive`/`ultrafixer`; (3) default 3 plus `lucind.yaml` override; (4) `lucind-ai phase <name>` and one profile; (5) `lucind.yaml` at repo root, roots in `.lucind/skill-roots.yaml`. No remaining open questions.

Out of scope: changing `AuthoringEvidence` or bumping its version; SQLite migrations; specialist-side skill selection; wrapping gentle-ai or reading `openspec/config.yaml` in Go; authoring skill content; treating `HashDir` as a gate; CLI failure-guidance banners (`cli.go:699,737,759,2004`); live executor Skill telemetry decoding; multi-PR split (`openspec/config.yaml:6-7`).
