# Research & Proposal: Skill Provisioning and the SDD Phase Specialist

**Status:** Pre-SDD research. No SDD change has been started for this work.
**Prepared:** 2026-08-28
**Intended consumer:** a fresh agent that will run the SDD cycle for this work, dispatched through `lucind-ai`.

This document is deliberately self-contained. Everything a fresh agent needs — the evidence,
the corrections to earlier assumptions, the decisions already taken by the maintainer, and the
proposal itself — is written here so no prior conversation has to be reconstructed.

Every factual claim below was verified against source at the cited `file:line` during the research
session. Claims about `gentle-ai` are against `/home/lanzerdev/git_root/gentle-ai` at HEAD
`cc4ed695` (v2.4.0, 2026-08-28). Anything not verified is marked **UNVERIFIED**.

---

## 0. How to use this document

1. Read §1 to know what the repository looks like right now, including the in-flight change.
2. Read §2 — the eight research findings. Four of them contradict assumptions a reasonable
   reader would otherwise make, and two of them would sink a naive implementation.
3. Read §3 for decisions the maintainer already made. These are settled; do not relitigate.
4. Read §4 for the proposal itself. It is written in the shape of this repository's
   `proposal.md` files and can seed one directly.
5. Read §5 for the questions still open, which belong to the SDD exploration phase.

---

## 1. Where the repository stands

### 1.1 The dependency is satisfied

`delegated-packet-authoring` **completed and archived on 2026-08-28**, at
`openspec/changes/archive/2026-08-28-delegated-packet-authoring`. `gentle-ai sdd-status` reports
`apply: all_done`, `tasks: 6/6 complete`, and no blocking reasons. There is no active SDD change
in `openspec/changes/`.

Its final remediation closed the read-only delivery gap described in §2.2: `internal/executor/`
`executor.go` gained `requestEnv` and the `LUCIND_READ_ONLY_PATHS` environment variable, now
committed. That mechanism is directly relevant here — see the first open question in §5.

**The contract seam this work builds on is therefore landed and stable. This work is unblocked.**

### 1.2 What `delegated-packet-authoring` built

Roughly 2,300 lines across new and modified packages, all uncommitted on `dev`:

| Component | Path | Role |
|---|---|---|
| Trusted compiler | `internal/packetauthor/{contract,compile}.go` | `Compile(Contract, TargetBinding) → Artifact`; deterministic body, contract JSON, manifest, length-prefixed digest |
| Manual admission | `internal/packetauthor/manual.go` | Preserves manual packet bytes; checks only universal safety |
| Diagnostics | `internal/packetauthor/diagnostic.go` | Rank-ordered `PA_*` codes |
| Specialist adapter | `internal/packetauthor/specialist.go` | Identity check, token-level duplicate-key rejection, `DisallowUnknownFields`, two forbidden-field maps |
| Shadow comparison | `internal/packetauthor/compare.go`, `internal/ledger/shadow.go` | Non-authoritative; one transaction per attempt |
| Canonical changes | `internal/candidatechange/collect.go` | Four-way union of Git change kinds |
| Frozen evidence | `internal/ledger/authoring.go` | `AuthoringEvidence` v1, hash-verified |
| Admission seam | `cmd/lucind-ai/packet_authoring.go` | `admitDispatchBatch` before worktree and quota |

Key invariants it established, which this work inherits:

- **Admission is all-or-nothing and runs before allocation.** `admitDispatchBatch`
  (`cmd/lucind-ai/packet_authoring.go:32`) rejects the whole batch before any worktree or quota
  is consumed.
- **Only the trusted compiler renders.** The specialist emits typed data; `forbiddenSpecialistRender`
  (`internal/packetauthor/specialist.go`) rejects any output field named `markdown`, `body`,
  `packet`, `rendered`.
- **The specialist cannot seize runtime authority.** `forbiddenSpecialistAuthority` rejects
  `target`, `commit`, `executor`, `worktree`, `quota`, `promote`, and siblings.
- **Targets bind late.** `dispatchTargetBinding` resolves the live parent SHA read-only and
  rejects with `PA_TARGET_STALE` if the parent moved.

---

## 2. Research findings

### 2.1 Executor skills exist but are never delivered

Two skill trees exist, for two different audiences. The split itself is correct:

- `plugin/claude-code/skills/lucind-ai/` — the **orchestrator** skill. `SKILL.md` plus 24 packet
  templates under `assets/` and disclosure modules under `references/`.
- `.agents/skills/lucind-{executor,apply,verify,fan-out-lens}/` — the **executor** skills, meant
  for the agent working inside a lane worktree.

The problem is delivery, not location:

- **Zero references to `.agents/skills` exist in Go, in the packet templates, or in the plugin.**
  Those skills load only if the executor's runtime happens to auto-discover `.agents/skills` in
  its working directory. That is an `agy` convention. `cursor-agent` looks in `.cursor/skills`,
  which does not exist in this repo; `opencode` looks in `.opencode/skills`, and while
  `.opencode/` exists it contains no `skills/`. **One of three supported executors finds them.**
- The `skill:` frontmatter key is parsed (`internal/packet/packet.go:163-164`) and copied to
  `ledger.LaneMetadata` (`internal/run/run.go:390`), where it lands inside an opaque JSON audit
  blob. It is pure telemetry — nothing reads it back to load anything.
- **21 packet templates hardcode `~/.claude/skills/sdd-{explore,propose,design,spec,tasks,archive}/SKILL.md`**
  in prose. Absolute, `$HOME`-dependent, Claude-specific, and duplicated 21 times. This is the
  *de facto* skill-provisioning mechanism today.

The registry that would make this resolvable already exists but is unused by any code:
`.atl/skill-registry.md` (gitignored, generated by `gentle-ai skill-registry refresh`). Its own
Contract section states the right policy — *"pass paths so subagents load the full runtime
contract"* — and its "Sources scanned" section already enumerates the search roots this proposal
needs.

**Executor coupling to be removed:** `lucind-executor/SKILL.md:4,11,15,17` calls the reader "an
`agy` executor", lists only `agy` and `cursor-agent` as executors (omitting `opencode`), and
duplicates `agy`'s default model, which already lives in `executor.DefaultModel`.
`lucind-verify/SKILL.md:4` embeds the executor name in the lane naming convention
(`verify-*-agy`), which lies the moment such a packet is dispatched elsewhere. `lucind-apply`
paraphrases the TDD lifecycle, which is content owned by its parent skill `sdd-apply`.

### 2.2 `read_only_paths` was declared everywhere and delivered nowhere

At the time of research, `Packet.ReadOnlyPaths` was parsed, normalized in
`packetauthor.normalizePaths`, frozen into `ledger.AuthoringEvidence`, compared in `accept`, and
carried to `executor.Request.ReadOnlyPaths` — and **no executor consumed it**
(`rg "req.ReadOnlyPaths" internal/executor/` returned nothing). `packetauthor.renderBody` did not
print it either, so a compiled packet's body never mentioned it.

This is exactly the failure mode the skills work must avoid: a typed, validated, audited field
whose delivery channel to the agent's prompt is never closed. It is being remediated now
(§1.1), and the fix — an environment variable set by `requestEnv` — is a delivery mechanism this
proposal can reuse or deliberately diverge from.

### 2.3 `skill-anchoring-guardrails` does not force anything

A reasonable reader assumes the binary already compels skill loading. It does not. The
`failure-guidance-banners` capability of that archived change is four `fmt.Fprintln` calls:

```
cmd/lucind-ai/cli.go:699   "See .../contracts/acceptance-promotion.md for review steps."
cmd/lucind-ai/cli.go:737   "See .../operations/troubleshooting.md for recovery steps."
cmd/lucind-ai/cli.go:759   "See .../coordination/recovery-reconciliation.md for recovery steps."
cmd/lucind-ai/cli.go:2004  "See .../operations/troubleshooting.md for recovery steps."
```

They fire only on failure paths, print to the orchestrator's terminal rather than the dispatched
agent's prompt, point at the orchestrator's own reference docs rather than executor skills, and
nothing verifies they were followed. The real guardrail in that change was elsewhere:
`worktree.ErrWorktreeDirty` plus a `force bool` and the `--force` flag.

**There is no existing skill-enforcement mechanism to extend. This work builds the first one.**

### 2.4 Acceptance never demotes a lane

This corrects an assumption that would misplace the entire enforcement design.

- `internal/accept/accept.go` is a **pure receipt verifier**. Its package doc states it "never
  promotes a candidate, mutates refs, or represents its receipt as semantic approval"
  (`internal/accept/accept.go:1-2`). A mismatch returns an `error`, which
  `cmd/lucind-ai/cli.go:684-687` prints before exiting 1. It never calls `SetStatus`.
- **Demotion to `lane.Deviated` lives in `internal/run/run.go:875-902`** (`enforceAllowedPaths`),
  at post-dispatch admission. The status constant is `internal/lane/status.go:15`.

Any check that must demote a lane belongs beside `enforceAllowedPaths`, not in `accept`. The
established pattern is a pair: `run` enforces operationally, `accept` re-verifies for the record.

Fields compared today against frozen evidence, in `validateVersionedEvidence`
(`internal/accept/accept.go:213-259`): `done_criteria[].criterion`, `hard_stops[].hard_stop`,
`files_changed`, and `commit`. Not compared: `summary`, `session_id`, `deviations`, `questions`,
`findings`.

### 2.5 The `AuthoringEvidence` struct shape is frozen forever — and the escape hatch

This is the single sharpest constraint in the codebase, and the one that would sink a naive
implementation.

`FreezeAuthoringEvidence` (`internal/ledger/authoring.go:44-60`) marshals the whole struct with
`Hash` blanked, hashes the payload length-prefixed under domain
`"lucind:lane-authoring-evidence/v1"`, then re-marshals with the hash included.
`DecodeAuthoringEvidence` (`:62-75`) decodes a stored row, **re-runs the freeze**, and demands
that both `wantEncoded` and `wantHash` be byte-identical to what was stored.

Consequences, verified:

- Adding **any** field to `AuthoringEvidence` changes the marshaled payload, so every
  already-frozen row fails to decode. `omitempty` does not help: the hash is over payload bytes
  that no longer match.
- `internal/ledger/authoring.go:63` hard-rejects any `version != AuthoringEvidenceVersion`.
  There is **no decoder for a prior shape**, so bumping the version does not rescue old rows
  either — it orphans them.

**The escape hatch, verified directly:**

```go
// internal/ledger/authoring.go:23
Contract  json.RawMessage  `json:"contract"`
```

`Contract` is a raw blob. Adding a field to the *contract* changes only the bytes inside that
blob, and only for new rows. An old row re-marshals its own old bytes verbatim, so
`wantEncoded == encoded` still holds and its hash still verifies.

**Therefore: skills ride inside the contract blob. The `AuthoringEvidence` struct is not touched,
`AuthoringEvidenceVersion` stays at v1, and no ledger migration is required at all.**

For reference if a migration ever *is* needed, v9→v10 is a clean additive template:
`migrateV9ToV10DDL` (`internal/ledger/schema.go:425-445`) uses
`ALTER TABLE ... ADD COLUMN ... NOT NULL DEFAULT` plus new tables, gated by an
`if currentVersion < 10` block (`:584-592`).

### 2.6 Three silent-drift seams to close in the same pass

1. **Hand-duplicated decode struct.** `internal/accept/accept.go:224-238` decodes
   `evidence.Contract` into a private anonymous struct duplicated from
   `packetauthor.normalizedContract`. Go ignores unknown JSON keys, so a new contract field is
   frozen correctly and **silently never verified**. It must be added there in the same commit.
2. **No schema/struct pin.** Nothing reflectively ties `internal/result/result.schema.json`
   to the `Envelope` struct (`internal/result/result.go:103-116`). A field added to one side
   only will not fail CI. The envelope has **no version field** and uses
   `additionalProperties: false`.
3. **Whole-struct equality.** `SetDoneCandidate` uses `reflect.DeepEqual` on the entire
   `LaneCandidate` (`internal/ledger/acceptance.go:100`). A new Go field not wired into the SQL
   `SELECT`/`Scan` produces spurious `ErrImmutableAcceptanceEvidence` on identical re-registration.

### 2.7 What the binary does not have

Verified absences, each of which is a "build from scratch", not an "extend":

- **No repository configuration reader of any kind.** `openspec/config.yaml` is inert to the
  binary; it is consumed only by the Markdown skill workflow. The sole YAML precedent is the
  `--dag` sidecar (`internal/dag/parse.go:9,45-54`), a per-invocation CLI input.
- **No lane-role vocabulary.** The strings "lens" and "synthesis" appear in **zero** `.go` files.
  Role exists only in packet-template filenames and in `references/strategies/fan-out.md` prose.
- **No `~` expansion and no multi-root path resolution.** No `os.UserHomeDir`, no
  `os.UserConfigDir`, no XDG handling. The one outside-repo rule is `worktree.PathFor`
  (`internal/worktree/worktree.go:155-162`), a hardcoded
  `<parent>/<repo>-worktrees/<lane-id>` sibling formula, duplicated at
  `internal/accept/accept.go:329-335`.
- **No closed-set validation on any frontmatter key.** `sdd_phase`, `fanout_group`, and `skill`
  are free strings whose only consumers are `ledger.LaneMetadata` pass-through
  (`internal/run/batch.go:196-206`, `internal/run/run.go:380-393`) and `packetDigest`
  (`internal/run/run.go:723-726`). Nothing branches on their values.

What **can** be extended: `internal/skillcontent.HashDir` already computes a deterministic
SHA-256 over a directory tree, folding each file's relative path so renames and additions change
the hash exactly as much as edits. It is single-root today but the function takes a `dir`
argument and is otherwise generic.

### 2.8 gentle-ai: what it actually enforces

Verified against `/home/lanzerdev/git_root/gentle-ai` at HEAD `cc4ed695`. Three findings reframe
the entire specialist design.

**(a) The "acquire before runtime-bearing work" invariant is prose only.**
There is no `gentle-ai apply`, `verify`, or `remediate` command; the only SDD entrypoints are
`sdd_status.go`, `sdd_attempt*.go`, and `sdd_verify_validate.go` (`internal/app/app.go:101-108`).
The phase resolvers (`internal/sddstatus/status.go:1558-1592`) never consult
`RuntimeStatus.ActiveAttempt`. The rule lives in
`internal/assets/skills/_shared/sdd-status-contract.md:22,125`. **gentle-ai cannot detect,
reject, or even log runtime-bearing work done without a token.**

**(b) Phase gates check content, never provenance.**
`singleArtifactState` / `multiArtifactState` (`internal/sddstatus/status.go:1355-1378`) check that
the file exists and is non-empty; `tasks` additionally counts checkboxes. No hash, no signature,
no attempt reference. **An artifact written by an external tool, with the right shape at the right
path, marks the phase complete.** The one partial exception is `verify`, validated by content
shape via `ValidateVerifyReportAdmission` (`internal/sddstatus/verification.go:163+`) — still not
by producer identity. `AttestedVerifyReportDigest` (`internal/sddstatus/runtime_ledger.go:878-881`)
proves *these bytes were in this candidate*, not *this process wrote them*.

**(c) One attempt per change, one worktree at a time.**
`RuntimeStatus.ActiveAttempt` is a singular `*RuntimeAttempt`
(`internal/sddstatus/runtime_ledger.go:306`). `Handoff` (`:958-989`) requires a distinct,
registered linked worktree sharing the same Git common directory
(`validateRuntimeHandoffDestination`, `:2684-2723`) and refuses a second move
(`ErrRuntimeHandoffAlreadyPerformed`, `:134`). `Settle` delegates to `Finish`
(`internal/sddstatus/runtime_compact.go:272,310`), which enforces
`EffectiveWorktree == store.Workspace` (`runtime_ledger.go:811-816`). **No child attempts, no
sub-attempts, no concurrent tokens exist.**

Supporting facts:

- The ledger lives at `<git-common-dir>/gentle-ai/sdd-runtime/v1/<change>`, keyed by change name
  alone. Every linked worktree of one clone shares it.
- `--max-changed-lines` is **measured by gentle-ai**, not declared by the caller: a whole-worktree
  overlay diff against the base tree captured at `Begin`, computed at `Finish`
  (`runtime_ledger.go:842-851,886`). Everything present in that worktree at settle time is
  charged, regardless of who wrote it. Budget is per-objective, where objective hashes
  `change + work_unit + evidence_goal + candidate_identity + generation` (`:2838-2846`),
  enforced at `internal/sddstatus/runtime_admission.go:136-140`.
- **`explore` is not a gentle-ai phase at all.** Pre-proposal research is explicitly
  "orchestrator-owned", outside the native status and ledger
  (`sdd-status-contract.md:18`). The phase tokens are
  `propose|spec|design|tasks|apply|verify|remediate|archive|sdd-new|select-change|resolve-blockers`.
- **No lens/synthesis concept exists.** `artifact_states.go:5-30` lists exactly one canonical
  artifact per phase. The `lens` in `internal/reviewerprovider` is an RDD reviewer role over a
  frozen diff, unrelated to SDD documents.
- **No externally-callable validator exists for any phase but verify.** There is no
  `sdd-propose-validate`, `sdd-design-validate`, `sdd-spec-validate`, or `sdd-tasks-validate`.
- **lucind-ai's worktrees are already in the supported topology.** `grant`
  (`runtime_ledger.go:1230-1250`) stores absolute, symlink-resolved roots
  (`normalizeGrantRootsRequest`, `:2539-2545`), and `gitRootOf`
  (`internal/sddstatus/edit_authority.go:141-143`) recognizes a worktree's `.git` *file* as a
  first-class repository root. Commit `f654764e` (2026-08-27) added a topology guard
  (`edit_authority.go:201-207`) blocking runtime targets in a *different* Git common directory,
  with the refusal text: *"keep runtime work in the planning repository or a shared linked
  worktree with the same Git common directory"*. A `git worktree add` of the same repo passes;
  a separate clone is blocked. **gentle-ai moved toward lucind-ai's model, not against it.**

**Consequence for the design: there is nothing to intercept.** The specialist does not need to
wrap or replace gentle-ai. It reads typed status, dispatches, and writes the canonical artifact
to the canonical path. gentle-ai advances the phase without asking who produced it. This removes
an integration layer previously estimated at 1,600–1,800 lines — and moves the entire burden of
correctness onto lucind-ai, because gentle-ai cannot enforce it.

### 2.9 The specialist cannot choose skills, by construction

`.opencode/agent/lucind-packet-author.md` declares `permission: "*": deny`, `steps: 1`,
`temperature: 0`. It has **no tools**. It cannot read a skill registry or the filesystem even if
instructed to. Determinism here is enforced by a permission boundary, not by an instruction
someone could ignore.

### 2.10 Lens lanes are write lanes, not read-only

A natural assumption is that planning lenses are read-only. They are not:

```yaml
# plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:2,6
id: design-<change-id>-lens-a
allowed_paths: ["openspec/changes/<change-id>/design-lens-a.md"]
```

No `read_only: true`, and a declared **write** path. Git history confirms lens lanes commit and
merge (`docs(openspec): author tasks lens c ...` followed by its merge commit). The only
genuinely read-only lanes are verify lanes.

Lens output nonetheless stays out of the synthesis attempt's budget — by **ordering**, not by
permissions. Synthesis starts only after all lenses are accepted and merged
(`references/strategies/fan-out.md:24`: *"Never start synthesis until all required Lens IDs are
accepted"*). By then the lens documents are part of the synthesis worktree's base tree, so the
`Finish` diff against `BeginCandidateTree` counts them as zero. This is a sequencing invariant
that must be written down, because gentle-ai does not validate ordering — it only measures a diff.

---

## 3. Decisions already taken

Settled by the maintainer during research. Do not relitigate these in exploration.

| # | Decision | Rationale |
|---|---|---|
| D1 | All four enforcement levels ship at once, including the `result.schema.json` change and the acceptance comparison. No phased cut. | A declaration nobody verifies is another banner (§2.3). |
| D2 | `sdd-init` writes the per-role stack skill configuration. Once a repo is initialized, invoking a phase deterministically fixes which skills load. | gentle-ai already detects the stack there and writes `context:` in `openspec/config.yaml`. |
| D3 | Skills come from three additive tiers — derived, stack, ad-hoc. Only specialist-side inference at compile time is forbidden. | A list written into a contract is frozen input, like `goal`. What breaks replay stability is re-deciding per run. |
| D4 | The skills change and the dispatch-specialist change are **one** change. | The specialist is the real consumer of the machinery; both alter the packet contract, and one migration beats two. |
| D5 | Lens output stays outside gentle-ai's attempt budget; only synthesis is charged. | Correct outcome; see §2.10 for the actual mechanism, which is ordering rather than read-only mode. |

---

## 4. Proposal

### 4.1 Intent

A packet tells an agent what to do but never hands it the operating manuals that govern the work.
Twenty-one templates hardcode `$HOME`-dependent paths in prose; the four in-repo executor skills
reach an agent only if its runtime happens to auto-discover them, which one of three executors
does; and nothing anywhere confirms a manual was read. Separately, every SDD phase is dispatched
by an orchestrator that carries the full weight of authoring each packet.

Make required skills a typed, binary-derived, frozen part of the packet contract — delivered in
the rendered body and verified against what the agent declares. Then add a per-phase specialist
that composes gentle-ai's typed phase status with lucind-ai's dispatch machinery, so the
orchestrator states intent and reads results instead of authoring every lane.

### 4.2 Scope

**In scope**

- Derive required skills deterministically from `(sdd_phase, lane_role)` inside the binary. No
  author, human or specialist, selects them.
- Two additive tiers under one budget: repo-versioned stack skills and packet-level ad-hoc skills.
- Resolve skill names to `SKILL.md` files through an ordered machine-local root list with `~`
  expansion; verify existence at admission.
- Carry the resolved set inside the existing authoring contract blob; render a
  `## Required skills` section into the packet body.
- Declare `skills_loaded` in the result envelope; demote to `deviated` on shortfall post-dispatch
  and reject at acceptance.
- A per-phase specialist that reads `gentle-ai sdd-status` JSON, plans the phase's lane set,
  dispatches through existing machinery, and writes the canonical artifact to the canonical path.
- Decouple `.agents/skills/lucind-*` from any executor name; delete prose duplicating a parent skill.
- Close the three drift seams in §2.6.

**Out of scope**

- Any change to `ledger.AuthoringEvidence`'s struct shape or to `AuthoringEvidenceVersion`.
- Any ledger schema migration.
- Specialist-side skill selection of any kind, including trigger matching against a registry.
- Wrapping, intercepting, proxying, or replacing gentle-ai. The specialist composes; gentle-ai
  keeps authority.
- Making the binary read `openspec/config.yaml`, or any coupling to OpenSpec as an artifact store.
- Authoring skill content, or making gentle-ai skills a build-time dependency.
- Making an externally edited skill fail a running lane.
- Automatic cutover from manual packets; manual authoring stays canonical.

### 4.3 Capabilities

**New**

- `skill-derivation` — closed `(phase, role)` table, tier union, budget arithmetic, deterministic ordering.
- `skill-root-resolution` — ordered machine-local roots, `~` expansion, name → `SKILL.md`, admission-time existence check.
- `skill-load-correspondence` — `skills_loaded` in the envelope, compared against the frozen required set.
- `phase-specialist-dispatch` — typed `sdd-status` ingestion, phase lane planning, canonical artifact placement.

**Modified**

- `packet-authoring-contract` — `Contract` gains `lane_role` and `required_skills`; `renderBody` gains a section.
- `lane-execution` — admission fails closed on missing or over-budget skills before worktree and quota; post-dispatch enforcement demotes on shortfall.
- `acceptance-verifier` — the hand-duplicated decode struct gains the field and the comparison.
- `read-only-packet-schema` — `lane_role` joins the frontmatter vocabulary as the first closed-validated key.

### 4.4 Approach

**A1 — Skills ride inside the contract blob.** `AuthoringEvidence.Contract` is `json.RawMessage`
(`internal/ledger/authoring.go:23`), marshaled verbatim by `FreezeAuthoringEvidence`. Old rows
re-freeze byte-identically no matter what new contracts contain. The struct shape stays frozen,
the evidence version stays v1, and **no migration is required**. This is the difference between an
additive change and one that orphans every stored candidate (§2.5).

**A2 — Configuration splits by durability, into two files.**

| | Path | Tracked | In the digest |
|---|---|---|---|
| Skill **names** per role | `lucind.yaml` (repo root) | yes | **yes** |
| Ordered search **roots** | `.lucind/skill-roots.yaml` | no | **no** |

Two machines resolve the same name to different absolute paths and still produce one digest.
`.lucind/` is gitignored (`.gitignore:2`) and already holds machine-local state, so the roots file
belongs there; the names file must be versioned or two machines compile different contracts. The
loader follows `internal/dag/parse.go:45-54`, the binary's only YAML precedent. Roots are seeded
once from the "Sources scanned" section of `.atl/skill-registry.md`; `.atl` is never read on the
dispatch path.

**A3 — Derivation is a pure function of two declared values.**

```
required(lane) = derived(phase, role)   ← mandatory; the budget never drops these
               ∪ stack(role)            ← from lucind.yaml
               ∪ adhoc(packet)          ← authored, frozen as input
```

The parent/child relationship is a property of this table, not of any skill's frontmatter. Two
axes: gentle-ai's skill owns *what makes the artifact valid* (the phase); lucind-ai's skill owns
*how it executes in an isolated worktree* (the role). Five planning phases share one role skill
because the role is identical — only the phase differs, and the parent already says that.

| `sdd_phase` | `lane_role` | Child (lucind) | Parent (gentle-ai) |
|---|---|---|---|
| propose / spec / design / tasks | `lens` \| `synthesis` | `lucind-fan-out-lens` | `sdd-<phase>` |
| apply | `apply` | `lucind-apply` | `sdd-apply` |
| verify | `verify` | `lucind-verify` | `sdd-verify` |
| archive | `archive` | *(gap — none exists)* | `sdd-archive` |
| — | `ultrafixer` \| `human` | *(gap — none exists)* | — |

Every lane also gets `lucind-executor`. Budget default 3; derived skills are never dropped, the
budget consumes ad-hoc first, then stack, and a packet that still does not fit is rejected rather
than silently trimmed.

**A4 — `lane_role` is a new closed-vocabulary key.** Constraining an existing key would break
packets in flight, so `lane_role` is new and `sdd_phase` becomes closed-validated *only when
`lane_role` is present*. Every existing packet keeps parsing.

**A5 — Enforcement mirrors allowed-paths exactly.** `run.enforceRequiredSkills`, beside
`enforceAllowedPaths` (`internal/run/run.go:875-902`), demotes to `lane.Deviated`.
`accept.validateVersionedEvidence` re-checks at receipt time and errors. Two checks — one
operational, one for the record — which is how path scope already works (§2.4).

**A6 — In-repo and external skills have different identities.** A repo skill is pinned by the
lane's `base_sha`; the commit *is* the content reference. An external skill is content-hashed at
admission via a multi-root variant of `skillcontent.HashDir`, recorded as **observation only,
never a gate**. The `internal/skillcontent` package header documents why: a blocking shared-content
check once turned unrelated parallel lanes into mutual overlap conflicts, with no way to resolve
three at once. Nobody's lane should fail because someone else edited a manual.

**A7 — The specialist composes; it never intercepts.**

```
orchestrator
     │  "what phase are we in?"
     ▼
gentle-ai sdd-status ──► typed JSON ──► phase specialist
                                             │ 1. plan the lane set for this phase
                                             │ 2. lucind-ai run → fan-out in worktrees
                                             │ 3. lenses accepted and merged
                                             │ 4. synthesis → canonical artifact path
                                             │ 5. redispatch within its own limits
                                             ▼
                                   gentle-ai (authority intact)
                                             ▼
                                       orchestrator
```

gentle-ai never stops deciding; lucind-ai never decides, it executes and reports. This is the same
boundary that already separates the packet specialist from the trusted compiler: one proposes
data, the other is the authority.

**A8 — Attempt handling follows the evidence, not the intuition.** Planning phases need no
`sdd-attempt` bracket: gentle-ai scopes the requirement to apply, verify, and remediation, and
phase completion there is content-driven (§2.8a, §2.8b). Runtime-bearing phases do take a token,
and they have no fan-out, so the "one token, N worktrees" conflict never actually arises. Where a
token is taken, three constraints hold: one `--change` is one attempt bound to one worktree;
`acquire`/`handoff`/`settle` all run with `--cwd` at that worktree, which must already be a
registered linked worktree; and everything that must be charged has to be inside it before settle.

**A9 — Ordering invariant for fan-out budgeting.** The synthesis `acquire`, where one is taken,
happens **after** all lenses are accepted and merged into the parent. Before that point their
lines are charged to the synthesis objective. gentle-ai does not validate this ordering; lucind-ai
must (§2.10).

### 4.5 Affected areas

| Area | Impact | Description |
|---|---|---|
| `internal/skillset/` | New | `(phase, role)` table, tiers, budget, ordering |
| `internal/skillroots/` | New | Roots loader, `~` expansion, name → `SKILL.md` |
| `internal/lucindconfig/` | New | `lucind.yaml` reader; the binary's first repo config |
| `internal/phasespec/` | New | `sdd-status` JSON ingestion, phase lane planning |
| `internal/packetauthor/` | Modified | Contract fields, validation, `renderBody` section |
| `internal/packet/` | Modified | `lane_role` key, first closed-set validation |
| `internal/run/` | Modified | Admission wiring, `enforceRequiredSkills` |
| `internal/result/` | Modified | `skills_loaded` in schema and struct, plus the pinning test |
| `internal/accept/` | Modified | Decode struct and comparison |
| `cmd/lucind-ai/` | Modified | Admission adapter, phase subcommand |
| `.opencode/agent/` | Modified | Phase-specialist profiles |
| `.agents/skills/lucind-*` | Modified | Executor decoupling, parent-duplication removal |
| `plugin/.../assets/*.md` | Modified | Remove 21 hardcoded skill paths |
| `internal/ledger/` | **Untouched** | No migration; skills ride in the contract blob |

### 4.6 Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| A new contract field is frozen but never verified, because the decode struct is hand-duplicated (§2.6) | **High** | Add it there in the same commit; extend `TestValidateVersionedResultRequiresExactFrozenCorrespondence` with a mutation case |
| Scope exceeds the 3,000-line budget | **High** | Forecast §4.9 before apply; chained PRs sliced by tier |
| gentle-ai cannot enforce any of this, so a bug in lucind-ai advances phases with bad artifacts (§2.8b) | Med | Treat lucind-ai's own acceptance as the only gate; never claim gentle-ai validated provenance |
| Context bloat: seven manuals reach an agent needing two | Med | Budget checked at admission, fail-closed; derived tier never dropped |
| The binary's first config reader accretes unrelated settings | Med | `lucind.yaml` accepts only `skills` in this change; unknown keys rejected |
| Ordering invariant (§2.10) silently violated, mischarging the budget | Med | Encode it as a dispatch precondition, not documentation |

### 4.7 Rollback plan

Remove the `## Required skills` section from `renderBody` first: packets compile and dispatch
exactly as before, and `skills_loaded` becomes an ignored optional envelope field. Then revert
enforcement, then the contract fields, then the specialist. No stored row needs conversion at any
step, because nothing was migrated.

### 4.8 Dependencies

- `delegated-packet-authoring` — **satisfied**. Archived 2026-08-28 (§1.1). This work is unblocked.
- The `agy` decoupling and parent-duplication cleanup in `.agents/skills/` (§2.1) are independent,
  touch no code, and can land at any time before or during this work.

### 4.9 Size forecast

| Slice | Authored lines (prod + test) |
|---|---|
| `skillset` derivation and budget | ~450 |
| `skillroots` resolution and `~` expansion | ~400 |
| `lucind.yaml` config loader | ~270 |
| `packetauthor` contract and render | ~320 |
| `packet` `lane_role` and validation | ~180 |
| `run` enforcement | ~260 |
| `result` schema, struct, pinning test | ~220 |
| `accept` decode and comparison | ~210 |
| `phasespec` specialist adapter | ~550 |
| Skills, profiles, template cleanup | ~250 |
| **Total** | **~3,100 (range 2,900–3,500)** |

Above the 3,000 budget. Chained PRs are recommended, sliced by tier: derivation and resolution
first, enforcement second, specialist third. A `size:exception` is the alternative and needs an
explicit maintainer decision before apply.

### 4.10 Success criteria

- [ ] The same `(phase, role)` yields a byte-identical required set and a stable digest across two machines with different skill roots.
- [ ] A packet whose required skill cannot be resolved is rejected before worktree allocation, naming the skill and the roots searched.
- [ ] A dispatched packet body contains resolved, machine-correct paths for every required skill.
- [ ] A result declaring fewer skills than required lands as `deviated` and is rejected at acceptance.
- [ ] Every pre-existing frozen candidate still decodes and verifies, with `AuthoringEvidenceVersion` unchanged and no schema migration applied.
- [ ] A phase specialist completes one full fan-out phase — lens dispatch, acceptance, merge, synthesis, canonical artifact — and `gentle-ai sdd-status` advances the phase.
- [ ] No `.agents/skills/lucind-*` file names an executor, and none restates its parent skill's contract.
- [ ] The 21 hardcoded `~/.claude/skills/...` prose paths are gone from `assets/`.

### 4.11 Stated limitation

This work can prove a manual was **delivered** and that the agent **declared** it loaded. It
cannot prove the agent read or understood it. An executor can emit `skills_loaded` without
opening a file. That limit is inherent and is recorded here so nobody later mistakes the check for
something stronger than it is.

---

## 5. Open questions for SDD exploration

1. **Delivery channel.** The read-only remediation now in flight (§2.2) delivers declared inputs
   through the `LUCIND_READ_ONLY_PATHS` environment variable. Should required skills use the same
   channel, the rendered body, or both? The body is visible to the agent's reasoning; the
   environment variable is not, unless the skill runtime reads it.
2. **Ad-hoc tier surface.** Ad-hoc skills need an authoring surface. A new frontmatter key, a
   field only the typed contract carries, or both?
3. **Missing role skills.** §4.4 shows no child skill exists for `archive` or `ultrafixer`. Create
   them in this change, or declare those roles derived-empty?
4. **Budget default.** 3 is proposed on judgment, not evidence. Worth one measurement pass against
   real dispatches before fixing it.
5. **Specialist granularity.** One specialist per phase, or one specialist parameterized by phase?
   The `(phase, role)` table suggests the latter; the profile-per-agent convention in
   `.opencode/agent/` suggests the former.
6. **`lucind.yaml` naming.** `.gitignore:2` ignores `.lucind/` with a trailing slash, so a file
   named `.lucind.yaml` would not be ignored — but the near-collision is a readability trap.
   `lucind.yaml` is proposed for that reason. **UNVERIFIED:** whether any other tool in this
   toolchain already claims that filename.

---

## 6. Reference index

Verified citations gathered during research, for the fresh agent's convenience.

**lucind-ai**

- `internal/ledger/authoring.go:14` — `AuthoringEvidenceVersion`
- `internal/ledger/authoring.go:23` — `Contract json.RawMessage` (the escape hatch)
- `internal/ledger/authoring.go:44-75` — freeze/decode hash discipline
- `internal/ledger/acceptance.go:100` — whole-struct `reflect.DeepEqual`
- `internal/ledger/schema.go:425-445,584-592` — v9→v10 additive migration template
- `internal/accept/accept.go:1-2` — "never promotes a candidate"
- `internal/accept/accept.go:172-207,213-259` — result/scope validation
- `internal/accept/accept.go:224-238` — hand-duplicated decode struct
- `internal/accept/accept.go:329-335` — duplicated worktree path formula
- `internal/run/run.go:875-902` — `enforceAllowedPaths`, the only demotion site
- `internal/run/run.go:380-393,723-726` — metadata pass-through and packet digest
- `internal/lane/status.go:15` — `Deviated`
- `internal/packet/packet.go:122-179` — the full frontmatter vocabulary
- `internal/packetauthor/compile.go:171-183,192-208` — `renderBody`, digest
- `internal/skillcontent/skillcontent.go` — `HashDir`, and the incident header
- `internal/worktree/worktree.go:155-162` — `PathFor`
- `internal/dag/parse.go:45-54` — the only YAML precedent
- `cmd/lucind-ai/cli.go:684-687` — accept error to exit 1
- `cmd/lucind-ai/cli.go:699,737,759,2004` — the four banners
- `cmd/lucind-ai/packet_authoring.go:32` — `admitDispatchBatch`
- `.opencode/agent/lucind-packet-author.md` — `permission: "*": deny`
- `plugin/.../assets/design-lens-a-packet-template.md:2,6` — lens lanes are write lanes
- `plugin/.../references/strategies/fan-out.md:24` — synthesis ordering invariant

**gentle-ai** (HEAD `cc4ed695`)

- `internal/app/app.go:101-108` — the only SDD entrypoints
- `internal/assets/skills/_shared/sdd-status-contract.md:18,22,125` — prose-only invariant; explore is orchestrator-owned
- `internal/sddstatus/status.go:1355-1378,1558-1592` — content-only phase gates
- `internal/sddstatus/runtime_ledger.go:306` — singular `ActiveAttempt`
- `internal/sddstatus/runtime_ledger.go:811-816,842-851,886` — worktree binding and line measurement
- `internal/sddstatus/runtime_ledger.go:958-989,2684-2723` — handoff
- `internal/sddstatus/runtime_ledger.go:1230-1250,2539-2545` — grant roots
- `internal/sddstatus/runtime_admission.go:136-140` — budget enforcement
- `internal/sddstatus/edit_authority.go:141-143,201-207,295-297` — worktree roots and the topology guard
- `internal/sddstatus/artifact_states.go:5-30` — one artifact per phase
- `internal/cli/sdd_verify_validate.go:35-88` — the only external phase validator
