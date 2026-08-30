# Design: Skill Provisioning and the SDD Phase Specialist

## Technical Approach

Candidate 1 (`proposal.md:3,38-54`): four new packages — `internal/skillset`, `internal/skillroots`, `internal/lucindconfig`, `internal/phasespec`.

1. **Derivation.** `skillset.Derive` computes `derived(sdd_phase, lane_role) ∪ stack(lane_role) ∪ adhoc(packet)`. Every lane derives `lucind-executor`. Planning roles add `lucind-fan-out-lens` plus `sdd-<phase>`; `apply`/`verify` add the matching lucind child plus `sdd-apply`/`sdd-verify`. Stack names come from `lucind.yaml`; ad-hoc names are authored input.
2. **Resolution.** `skillroots` maps names to `<root>/<skill>/SKILL.md` (Decision 5).
3. **Contract blob.** Skills ride inside `AuthoringEvidence.Contract` (`internal/ledger/authoring.go:26`) with version `lane-authoring-evidence/v1` (`:14`). No struct-field addition, no version bump.
4. **Dual delivery.** `renderBody` (`internal/packetauthor/compile.go:171-183`) emits `## Required skills` as `- <resolved-path>` lines between `## Hard stops` and `## Return`, omitted when empty. Path lines are human-facing only (Decision 8). `requestEnv` (`internal/executor/executor.go:20-39`) strips inherited `LUCIND_REQUIRED_SKILLS` and injects a JSON array, same channel as `LUCIND_READ_ONLY_PATHS`.
5. **Two-site enforcement.** `enforceRequiredSkills` sits beside `enforceAllowedPaths` (`internal/run/run.go:489-491,876-904`) and demotes envelope `skills_loaded` shortfalls to `lane.Deviated` (`internal/lane/status.go:11-17`). `validateVersionedEvidence` (`internal/accept/accept.go:263-328`) re-checks frozen evidence.
6. **Specialist.** `phasespec` runs `gentle-ai sdd-status --json`, admits lens packets through `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`), then synthesizes `openspec/changes/<change>/<phase>.md`. Output stays inside forbidden-key maps (`internal/packetauthor/specialist.go:155-165`).

## Architecture Decisions

### Decision 1 — Ad-hoc authoring surface

**Choice**: Both: `AdhocSkills []string` on `packetauthor.Contract` and optional `adhoc_skills` frontmatter (JSON string array) on `packet.Packet`. `required_skills` is derived, never authored: `Compile` populates `Contract.RequiredSkills` and admission populates `Packet.RequiredSkills`, both from `Derive`. No `required_skills` parse case exists, so an authored one is ignored (`packet.go:122-179`).
**Alternatives considered**: Frontmatter only (nothing typed to hold ad-hoc names) vs `Contract` only (manual packets cannot declare them).
**Rationale**: Manual packets skip `Contract` compilation (`cmd/lucind-ai/packet_authoring.go:44-50`) and parse frontmatter via `packet.Parse`. Both surfaces keep typed and manual dispatch at parity.

### Decision 2 — Skill-budget default and override

**Choice**: Package constant `DefaultSkillBudget = 3` in `internal/skillset`, overridable by optional integer `skill_budget` in `lucind.yaml`. Oversized sets are rejected at admission, not trimmed.
**Alternatives considered**: Hard-coded constant only vs config-only (breaks zero-config).
**Rationale**: `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) is the pre-worktree seam. Count the union there. Derived-only sets over budget also reject.

### Decision 3 — Specialist CLI and composition

**Choice**: One subcommand `lucind-ai phase <name>` in the existing string-switch (`cmd/lucind-ai/cli.go:142-168`) and one `.opencode/agent/lucind-packet-author.md` profile.

Content authority stays with gentle-ai's phase skills; execution authority — budgets, paths, done criteria, `.lucind/result.json` — stays with the dispatched packet (`plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:43`). The `sdd-design` skill's 800-word budget and Engram persistence therefore do not bind a synthesis lane.

**Alternatives considered**: Per-phase CLI subcommands and profiles; writing phase documents without lucind-ai lanes.
**Rationale**: Fan-out phases share the same mechanics and tool-denial profile (`.opencode/agent/lucind-packet-author.md:6-7`), so one parameterized command avoids handler duplication.

### Decision 4 — `skillset.Derive` shape

**Choice**: Pure function `func Derive(sddPhase, laneRole string, stackSkills, adhocSkills []string) ([]string, error)` returning a deduplicated, sorted name slice with derived skills guaranteed. Callers load stack names from `lucindconfig`.
**Alternatives considered**: Method on a stateful config struct; untyped variadic inputs.
**Rationale**: A pure function is testable from both `Compile` (`internal/packetauthor/compile.go:32-47`) and admission.

### Decision 5 — Roots, `lucind.yaml`, and file naming

**Choice**: Tracked `lucind.yaml` at repo root holds stack skills per role and optional `skill_budget`. Parsed with `yaml.NewDecoder` + `KnownFields(true)`. Machine-local roots live in gitignored `.lucind/skill-roots.yaml` (`.gitignore:1-3`); first matching `<root>/<skill>/SKILL.md` wins; `~` expands.
**Alternatives considered**: `.lucind.yaml` dotfile; merging roots into `lucind.yaml` (would version machine paths); unordered multi-match errors; permissive `yaml.Unmarshal`.
**Rationale**: `KnownFields(true)` is stricter than the DAG sidecar's `Unmarshal` (`internal/dag/parse.go:45-54`). Ordered roots give local overrides.

### Decision 6 — `lane_role` closed set and frontmatter

**Choice**: Optional `lane_role` with closed set `{lens, synthesis, apply, verify, archive, ultrafixer, human}`. Omission leaves `p.LaneRole == ""` and today's parse unchanged (`internal/packet/packet.go:122-179`). Unknown values return `ErrInvalidLaneRole`. When present, `sdd_phase` is closed-validated against `{explore, propose, spec, design, tasks, apply, verify, remediate, archive}`: the gentle-ai phase tokens lucind-ai dispatches lanes for (`explore.md:285`) plus orchestrator-owned `explore` (`explore.md:282-284`). `remediate` is in the set and derives no phase skill until `sdd-remediate` exists — Decision 7's derived-empty treatment.
**Alternatives considered**: Numeric enums; boolean flags.
**Rationale**: Matches existing lower-snake-case keys (`internal/packet/packet.go:159-164`). Closed validation makes `Derive` deterministic. The seven-value role set is the proposal union (`proposal.md:38-54`).

### Decision 7 — Missing archive/ultrafixer lucind children

**Choice**: Do not author stub `lucind-archive` or `lucind-ultrafixer` skills. Those roles have an empty lucind-child tier. They still derive `lucind-executor`; `archive` still derives existing `sdd-archive`.
**Alternatives considered**: Create stub skill files under `.agents/skills/` or external roots.
**Rationale**: Admission (`cmd/lucind-ai/packet_authoring.go:32-54`) fails closed on any required name it cannot resolve. `.agents/skills/` ships `lucind-executor`, `lucind-apply`, `lucind-verify`, and `lucind-fan-out-lens` only. Stubs are out-of-scope skill authoring (`proposal.md:20-27`). `sdd-archive` already exists as a gentle-ai skill; `sdd-remediate` does not.

### Decision 8 — Digest excludes resolved paths

**Choice**: `skillset.DigestBody(body string) string` elides the `## Required skills` section — the heading line through the line before the next `## ` heading or EOF — leaving the rest byte-identical; a body without that heading returns unchanged. `Compile` hashes `DigestBody(body)` at `compile.go:45`; names still hash through `contractJSON`, whose `normalizedContract` gains sorted `RequiredSkills`. `packetDigest` (`run.go:722-729`) substitutes `DigestBody(p.Body)` and appends `LaneRole` plus sorted `RequiredSkills`/`AdhocSkills`, each variable-length group preceded by a decimal count because `versionedHash` frames elements, not groups. That field list and the accept decode struct (`accept.go:275-286`) both enumerate packet fields literally, so a new `packet.Packet` field absent from either escapes silently: both change in the same commit.

**Alternatives considered**: Rendering canonical names instead of paths (loses readable paths); placeholder substitution (equivalent, more parsing).
**Rationale**: `packetDigest` is frozen once and compared only to the value written in that same transaction (`run.go:623-625`, `accept.go:268`), so extending the list converts no rows. Compiled packets override it with `p.Authoring.Digest` (`run.go:657`), so both sites must exclude equally or manual and compiled dispatch diverge.

## Flow and Invariants

```
(sdd_phase, lane_role) + lucind.yaml + adhoc
        → skillset.Derive (union)
        → skillroots (resolve; ~ expand)
        → admitDispatchBatch (existence + budget)
        → renderBody  ## Required skills   AND   LUCIND_REQUIRED_SKILLS
        → Envelope.skills_loaded
        → enforceRequiredSkills ── shortfall → lane.Deviated
        → validateVersionedEvidence ── mismatch → reject, no receipt
```

- **Derivation**: same `(phase, role)` → byte-identical names. Breaks: divergent derived tables.
- **Admission**: missing skill or over budget rejects the batch before allocation, naming skill and roots (`cmd/lucind-ai/packet_authoring.go:32-54`).
- **Digest**: canonical names only; resolved paths render but never hash (Decision 8).
- **Envelope**: optional `skills_loaded`; `additionalProperties: false` (`internal/result/result.schema.json:5`).
- **Demotion vs accept**: `run` demotes; `accept` never does (`cmd/lucind-ai/cli.go:684-687`). Correspondence proves declaration, not that the file was opened.
- **Specialist**: synthesis starts only after every required lens ID is accepted and merged (`fan-out.md:24`).
- **Legacy**: omitted `lane_role` still parses (`internal/packet/packet.go:122-179`). Frozen rows still decode (`internal/ledger/authoring.go:62-75`). `SetDoneCandidate` DeepEqual (`internal/ledger/acceptance.go:96-103`) — no new `LaneCandidate` fields or columns.

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|
| `internal/packet/packet.go` | Modify | Add `LaneRole`, `AdhocSkills`, `RequiredSkills`; `lane_role`/`adhoc_skills` parse; `ErrInvalidLaneRole` (`:43-103,122-179`) | `admitDispatchBatch` (`cmd/lucind-ai/packet_authoring.go:32-54`) |
| `internal/packetauthor/contract.go` | Modify | Add `LaneRole`, `AdhocSkills`, `RequiredSkills` (`:45-56`) | `Compile` (`internal/packetauthor/compile.go:32-47`) |
| `internal/packetauthor/compile.go` | Modify | Same fields on `normalizedContract`; emit `## Required skills`; hash `DigestBody` (`:15-25,45,171-183`) | `requestEnv` (`internal/executor/executor.go:20-39`); agents reading `Packet.Body` |
| `internal/executor/executor.go` | Modify | `RequiredSkills` on `Request`; inject `LUCIND_REQUIRED_SKILLS` (`:20-39,50-75`) | Child `os.Environ` |
| `internal/result/result.go` | Modify | Add optional `SkillsLoaded` (`:103-116`) | `enforceRequiredSkills` (`internal/run/run.go:876-904`) |
| `internal/result/result.schema.json` | Modify | Add optional `skills_loaded` string array (`:7-165`) | `result.Read` (`internal/result/result.go:138-145`) |
| `internal/result/schema_test.go` | Modify | Add Envelope↔schema reflection pin (`:10-33`) | `go test ./internal/result` |
| `internal/run/run.go` | Modify | `enforceRequiredSkills` beside `enforceAllowedPaths`; `packetDigest` field list (`:489-491,722-729,876-904`) | `SetDoneCandidate` (`internal/ledger/acceptance.go:55-105`) |
| `internal/accept/accept.go` | Modify | Decode-struct fields + correspondence (`:275-286`) | `accept` CLI (`cmd/lucind-ai/cli.go:684-687`) |
| `internal/accept/authoring_evidence_test.go` | Modify | `required_skills` mutation case (`:56-127`) | `go test ./internal/accept` |
| `internal/skillset/` | Create | `Derive` + `DefaultSkillBudget` + `DigestBody` | `admitDispatchBatch`; `Compile`; `packetDigest` |
| `internal/skillroots/` | Create | Ordered root resolution, `~` expansion | `admitDispatchBatch` |
| `internal/lucindconfig/` | Create | `lucind.yaml` + `KnownFields(true)` | `admitDispatchBatch` (feeds `Derive`) |
| `internal/phasespec/` | Create | `sdd-status` adapter, lens then synthesis | `lucind-ai phase` (`cmd/lucind-ai/cli.go:142-168`) |
| `cmd/lucind-ai/cli.go`, `packet_authoring.go` | Modify | `phase` case; admission budget + resolution | Operators; `AdmitBatch` |
| `.agents/skills/lucind-*`, `plugin/.../assets/*.md` | Modify | Drop executor-named skills and hardcoded `~/.claude/skills/...` paths | Dispatched agents |

## Testing Strategy and Test Seams

| Layer | What | Seam |
|---|---|---|
| Unit | Closed-set `lane_role`; omitted role still parses | `internal/packet/packet.go:122-179`, `internal/packet/packet_test.go:133-195` |
| Unit | Derive union; derived never drop; budget reject | new `internal/skillset/skillset_test.go` |
| Unit | Root order, `~`, missing-skill diagnostic | new `internal/skillroots/skillroots_test.go` |
| Unit | `required_skills` in `normalizedContract`, root-independent digest, `## Required skills` body, `packetDigest` field-list pin | `internal/packetauthor/compile.go:45,171-183`, `compile_test.go:1-60`, `run_test.go` |
| Unit | Envelope↔schema reflection pin for `skills_loaded` | `internal/result/schema_test.go:10-33` |
| Integration | Shortfall → `lane.Deviated` | `internal/run/run.go:876-904` |
| Integration | Mutated/missing `required_skills` rejected, no receipt | `internal/accept/authoring_evidence_test.go:56-127`, `accept.go:263-328` |
| Integration | `LUCIND_REQUIRED_SKILLS` beside `LUCIND_READ_ONLY_PATHS` | `internal/executor/executor.go:20-39`, `read_only_paths_test.go:1-50` |
| Integration | Freeze/decode with new contract fields; legacy v1 rows | `internal/ledger/authoring.go:44-75` |
| E2E | Status ingest, admit, lens merge, synthesis, artifact path | new `internal/phasespec/specialist_test.go` (mock `sdd-status`) |

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | Applicable | Load `SKILL.md` and skill-roots YAML as data; `HashDir` is observation-only (`internal/skillcontent/skillcontent.go:1-28,73-100`) | Admit/hash `SKILL.md` without executing it (`internal/accept/accept_test.go:125-138`) |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable | Roots resolve locally; only canonical names enter either digest (Decision 8) | Identical `Compile` digest and `packetDigest` across differing root prefixes and `~` expansions, bodies differing |
| Commit state | staged, `commit -a`, empty index | Applicable | Demotion reads `skills_loaded`; accept verifies the detached candidate (`internal/accept/accept.go:263-328`, `accept_test.go:140-164`) | Accept passes on a dirty primary when skills match; rejects on mismatch |
| Push state | tracking branch, first push, explicit refspec | N/A: no ref mutation | N/A | N/A |
| PR commands | explicit `--head`, environment prefix, composed commands | Applicable | Specialist runs read-only `gentle-ai sdd-status`; never mutates PR state | Malformed status JSON and CLI errors fail closed without side effects |

## Rollback and Additivity

Additive; no row conversion. Reverse:

1. Stop rendering `## Required skills` and hashing `DigestBody` (`compile.go:45,171-183`); stop deriving at admission (`packet_authoring.go:32-54`).
2. Revert `enforceRequiredSkills` and the `packetDigest` fields (`run.go:722-729,876-904`), and accept comparison (`accept.go:263-328,275-286`).
3. Revert envelope schema/struct (`result.schema.json:1-165`, `result.go:103-116`).
4. Revert `internal/phasespec/`, `skillset/`, `skillroots/`, `lucindconfig/`.

Rejected: ledger v11 and evidence v2 — would break frozen hashes (`internal/ledger/authoring.go:62-75`). Schema stays v10 (`internal/ledger/schema.go:425-445,584-592`).

## Open Questions and Out of Scope

### Open Questions

None. `remediate` is closed in Decision 6.

### Out of Scope

- Changing `AuthoringEvidence` shape or version (`internal/ledger/authoring.go:14,20-42`); SQLite migration (`internal/ledger/schema.go:425-445,584-592`).
- Specialist-side skill selection (`.opencode/agent/lucind-packet-author.md:1-8`).
- Intercepting gentle-ai; reading `openspec/config.yaml` in Go (`proposal.md:20-27`).
- Authoring skill markdown; `HashDir` as a gate (`internal/skillcontent/skillcontent.go:1-28`).
- Automatic cutover from manual packets; multi-PR split (`openspec/config.yaml:6-7`).
- CLI failure-guidance banners (`cmd/lucind-ai/cli.go:699,737,759,2004`).
- Specialist bracketing of `sdd-attempt` tokens for apply/verify/remediate (propose-phase gap; not invented here).
