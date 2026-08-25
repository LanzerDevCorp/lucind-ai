---
id: ultrafixer-design
executor: agy
routed_by: SDD simple (no fan-out) phase dispatch for feature/ultrafixer — design phase, following propose (integrated at af38e1e).
allowed_paths: ["openspec/changes/ultrafixer/design.md", "openspec/changes/ultrafixer/state.yaml"]
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: af38e1edc716600204e821df6037d0696ee168bf
expected_parent_sha: af38e1edc716600204e821df6037d0696ee168bf
---

# Packet ultrafixer-design

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/ultrafixer-design  ·  **Branch:** lucind/ultrafixer-design

## Goal

Write `openspec/changes/ultrafixer/design.md`, the technical design for the `ultrafixer`
capability, following this repo's own design convention (see
`openspec/changes/control-room-capture/design.md` in this worktree for the exact shape:
`## Technical Approach`, `## Architecture Decisions` (each a `### Decision:` with
Choice/Alternatives considered/Rationale/Terminal consumer), `## Flow and Invariants` (ASCII
diagram + numbered invariants), `## Interfaces / Contracts` (Go signatures for anything genuinely
new), `## File Changes` table, `## Testing Strategy and Test Seams` table, `## Threat Matrix`
table, `## Rollback and Additivity`, `## Open Questions and Out of Scope`). Update
`openspec/changes/ultrafixer/state.yaml`'s `phases.design.status` to `done`.

## Why this is safe to dispatch now

`openspec/changes/ultrafixer/proposal.md` (committed at this packet's `base_sha`) already
formalized the confirmed, human-approved design at the product level and named three explicit
Open Questions this design phase must resolve (see Context below). This packet's job is to make
concrete engineering decisions answering those three questions and specifying exactly what code
changes (if any), schema migration, and packet template are needed — grounded in real
`file:line` evidence, not to revisit product-level decisions already settled in the proposal.

## Preconditions

- `openspec/changes/ultrafixer/proposal.md` and `state.yaml` already exist at this packet's
  `base_sha` (committed, integrated at `af38e1e`). Read `proposal.md` in full first.
- `openspec/changes/ultrafixer/explore.md` (committed earlier, still present) has the underlying
  codebase evidence — read it for the exact `file:line` citations already gathered.
- The repo's base (`35de9910f7`) was independently confirmed green via `lucind-ai check` before
  this feature was created — you do not need to re-verify that.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** Every new
      type, field, table, CLI verb, or config key you specify in the design must name the exact
      file/function that will read/consume it, per this repo's own design.md convention.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and `git log --oneline
      -1`. Conventional commit, no AI attribution.
- [ ] `openspec/changes/ultrafixer/design.md` exists, follows the local convention cited above,
      and every `file:line` citation is verified against the actual current worktree (not copied
      blind from `explore.md`/`proposal.md` without checking it still applies).
- [ ] `openspec/changes/ultrafixer/state.yaml`'s `phases.design` is updated to `status: done` with
      a short `note:` (do not touch any other phase's entry).
- [ ] The design explicitly resolves proposal.md's three Open Questions (Defect Record schema
      naming/columns, multi-branch question encoding, worktree pruning policy) with a stated
      Decision, not left as still-open.
- [ ] The design explicitly states whether `ultrafixer-dispatch` requires ANY new Go code beyond a
      packet template + skill documentation (it may turn out to need none, reusing existing
      packet/Lane/agy machinery entirely — say so plainly if that's the conclusion), versus
      `defect-records`, which the proposal already scoped as requiring new ledger schema (v8) and
      therefore new Go code.
- [ ] The design specifies the exact new packet template asset (e.g.
      `plugin/claude-code/skills/lucind-ai/assets/ultrafixer-packet-template.md`) an Orchestrator
      will use to dispatch a real ultrafixer Lane — its frontmatter shape, what Context it must
      carry (the failing check's exact command, since ultrafixer must work across ANY orchestrated
      project's toolchain, not just Go/lucind-ai's own), and how it differs from the generic
      `packet-template.md`.

## Allowed paths

- `openspec/changes/ultrafixer/design.md`
- `openspec/changes/ultrafixer/state.yaml`

## Allowed paths outside the repository

None.

## Out of scope

- Writing spec or tasks artifacts — later phases, dispatched separately.
- Any actual code change under `internal/`, `cmd/`, or `plugin/` — this phase is design-only, no
  implementation. Name the files that WILL change in a later apply phase; do not change them now.
- Modifying `explore.md`/`proposal.md` (read-only inputs) or
  `/home/lanzerdev/.claude/agents/lucind-ai-fixer.md` (a separate, already-existing agent — never
  touch it).
- Re-litigating the product-level design already confirmed in `proposal.md` (origin classification
  via `base_sha`, two independent critical/blocking axes, manual-only trigger, human-gated
  integration, decline preservation). If evidence genuinely contradicts one of these, flag it as a
  risk/open question — do not silently change it.

## Hard stops

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and the packet does not say which.
- Satisfying one instruction in this packet would require violating another.
- Resolving one of proposal.md's three Open Questions turns out to require a genuine product
  decision beyond engineering judgment (e.g. it materially changes user-facing behavior already
  confirmed with the human). Name the exact tension; do not decide it unilaterally.

## Context

### What proposal.md already decided (do not re-derive — read the full file for complete detail)

`ultrafixer` is an ephemeral, per-defect `agy` Lane dispatched manually by a feature's Orchestrator.
It classifies origin via `base_sha` diff (`internal/overlap/overlap.go:1007`), evaluates two
independent axes (critical: security/data-loss/CI-breakage; blocking: per active branch via
`lucind-ai feature status`), repairs and stops at a `blocked` result envelope
(`internal/result/result.go:77-115`) for critical-or-blocking cases, records a Defect Record
(new ledger schema v8) for neither, uses CodeGraph as a candidate filter plus mandatory failure
reproduction for cross-branch impact, and never self-integrates — the human always runs
`lucind-ai integrate`/`integrate retry` manually. Declined fixes preserve their worktree/branch.

### The three Open Questions this design phase must resolve

1. **Defect Record schema naming/columns**: proposal.md suggested `defect_records` or `defects`
   with columns roughly `(id, feature_id, run_id, lane_id, error_signature, evidence, disposition,
   created_at)` as a starting point, not a final decision. Design the actual DDL, following this
   repo's migration pattern (`internal/ledger/schema.go:59-78,221-308` — the create-copy-drop-rename
   pattern already used for prior migrations; verify the exact pattern by reading a recent
   migration in that file, e.g. whatever shipped schema v7).
2. **Multi-branch question encoding**: proposal.md's provisional answer was "repeated `Question`
   entries plus `Finding.Affects` (`internal/result/result.go:77-82,98`), no schema change" — verify
   this is genuinely sufficient by checking how `Question`/`Finding` are actually consumed
   downstream (search for where `Envelope.Questions`/`Envelope.Findings` are read — CLI printing,
   `serve` model, anything else) and confirm nothing chokes on multiple `Question` entries for one
   envelope. Decide definitively: reuse as-is, or does it need `result.schema.json`
   (`internal/result/schema.go:10-28`) extended? If extended, specify the exact new field.
3. **Worktree pruning policy for declined fixes**: proposal.md left this open between "a flag on
   `lucind-ai worktree cleanup`" and "strictly manual operator deletion." Check what
   `lucind-ai worktree cleanup` (`cmd/lucind-ai/cli.go:56`) already does today and decide whether
   ultrafixer needs anything new here, or whether the existing command already covers it (in which
   case: no new capability needed, just document the existing command in the design).

### Threat Matrix categories to address (per `plugin/claude-code/skills/lucind-ai/references/core/safety.md`)

Ultrafixer's Lane performs real git operations (diff against `base_sha`, commit a fix) and never
pushes or opens a PR (confirmed out of scope in proposal.md). Address, following
`control-room-capture/design.md`'s `## Threat Matrix` table shape: Documentation-like paths
(N/A or applicable — decide), Git repository selection (applicable — ultrafixer must never operate
outside its own isolated worktree/the specific candidate branch it's diffing against; name the
concrete guard), Commit state (applicable — the repair commit must follow this repo's own
conventional-commit-no-AI-attribution rule, same as every other Lane), Push state (N/A — ultrafixer
never pushes, confirmed out of scope), PR commands (N/A — ultrafixer never opens a PR, integration
is always the human running `lucind-ai integrate` manually).

### Key evidence already gathered (see `explore.md` for full detail with more citations)

- Packet/Lane/agy plumbing: `internal/packet/packet.go:33`, `cmd/lucind-ai/cli.go:82`,
  `internal/executor/agy.go:69/136`, `internal/run/integrate_feature.go:26`,
  `internal/run/batch.go:66`.
- `blocked` envelope: `internal/result/result.go:77` (`Question`), `:98` (`Finding.Affects`),
  `:102` (`Envelope`).
- Feature discovery: `lucind-ai feature status` (no `--id`), `internal/serve/model.go:536`,
  `cmd/lucind-ai/cli.go:954-1045`.
- Origin classification: `internal/overlap/overlap.go:1007` (`Evaluate`),
  `internal/resolve/candidate.go:97` (`EnforceAllowedPaths`, direct `git` subprocess convention).
- Current ledger schema version is **7** (`internal/ledger/schema.go:10`, directly verified) — any
  new migration is **v8**.
- CodeGraph (`codegraph impact`/`codegraph affected`) is a confirmed-real but external
  binary/index dependency — zero internal lucind-ai references to it.
- Integration: `lucind-ai integrate` / `integrate retry`
  (`internal/run/integrate.go:159-165`; `internal/run/integrate_retry.go`), including the
  recently-fixed CAS-baseline recovery for multi-wave features.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well
the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
