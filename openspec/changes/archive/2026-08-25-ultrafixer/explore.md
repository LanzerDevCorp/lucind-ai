# Exploration: ultrafixer feasibility evidence (against confirmed design)

Investigated from worktree `/home/lanzerdev/git_root/lucind-ai-worktrees/ultrafixer` (branch
`feature/ultrafixer`) via `codegraph_explore`, Grep/Glob, and a follow-on orchestrator pass with
`mem_search`/`Bash` to close the two gaps the sub-agent's toolset couldn't reach (prior Engram
context, and direct schema verification).

## 1. Packet/Lane/agy executor plumbing

- `internal/packet/packet.go:33` `Packet` struct: `ID, Executor, RoutedBy, Model, Agent, ReadOnly,
  AllowedPaths, Feature, ParentRef, BaseSHA, ExpectedParentSHA, LegacyMain, SDDPhase, FanoutGroup,
  Skill, Path, Body`. Frontmatter-parsed Markdown (`Parse`), `--packet` flag loads via `loadPacket`
  (`cmd/lucind-ai/cli.go:116`).
- `cmd/lucind-ai/cli.go:82` `supportedExecutors` map includes `"agy": func() executor.Executor {
  return executor.Agy{} }` alongside claude/cursor-agent/opencode. `internal/executor/agy.go:69`
  `Agy` struct dispatches the real `agy` CLI headlessly (`Run` at `agy.go:136`); `DefaultModel()` =
  `gemini-3.7-flash-high`.
- `runDispatch` (`cli.go:169`) is `lucind-ai run`: one `--packet` = one lane (repeatable flag,
  `packetPaths.Set`), executor/agent/model validated per-packet before any dispatch,
  `lucindrun.FeatureTarget(ps)` (`internal/run/integrate_feature.go:26`) derives the feature/parent
  target from `packet.Feature/ParentRef/BaseSHA/ExpectedParentSHA`, then `lucindrun.ExecuteBatch`
  (`internal/run/batch.go:66`) dispatches N lanes concurrently, and
  `lucindrun.IntegrateFeature`/`Integrate` handles CAS promotion. A single-packet batch is "one
  Lane" cleanly.
- Feature registration CLI: `lucind-ai feature create --id --parent/--parent-ref --base-sha
  --expected-parent-sha` (`runFeatureCreate`, `cli.go:885`) — exactly what was used to register
  `ultrafixer` itself.

## 2. `blocked` result contract

- `internal/result/result.go`: `Envelope` (line 102) is the schema-validated `.lucind/result.json`
  contract: `PacketID, Status, Summary, HardStops, FilesChanged, ExternalChanges, DoneCriteria,
  Commit, Questions, Deviations, Findings, SessionID`.
- `Question` struct (line 77): `Question, WhyBlocking, Options []string, Recommendation string` —
  exactly the "decision question and recommendation" `dependencies-defects.md` requires for a
  blocked result. Required when `Status == "blocked"`.
- `Envelope.LaneStatus()` (line 122) maps schema status `"blocked"` → `lane.Blocked`
  (`internal/lane/status.go:14`, one of 4 terminal states: Done/Blocked/Deviated/Failed).
- Assessment: the envelope already carries `Commit` (the fix commit), `FilesChanged`, `Findings`
  (evidence), and `Questions[].Recommendation`. It is rich enough for ONE branch's disposition
  (critical/blocking fix + recommendation). It has **no native per-branch fan-out field** — a
  single Envelope carries one `Commit`/one set of `Questions`, not "disposition per affected
  feature branch X, Y, Z". Multi-branch disposition would need to be encoded either as multiple
  `Question` entries (one per branch, using `WhyBlocking`/`Recommendation` text) or via a schema
  extension — `Finding.Affects string` (`result.go:98`) already exists and is usable today,
  informally, for "which branch this affects" without a schema change.

## 3. Feature discovery

- `internal/ledger/ledger.go:1342` `FeatureRow{ID, ParentRef, BaseSHA, ExpectedParentSHA, Status,
  CreatedAt, UpdatedAt}`; `ActiveFeatures` (`ledger.go:1353`) queries `WHERE status = 'active'` —
  Go-internal only, not directly callable from an agy subprocess.
- CLI surface confirmed: `lucind-ai feature status` (no `--id`) lists ALL features (not just
  active) via `serve.Model.ListFeatures` (`internal/serve/model.go:536`, JSON-facing `Feature`
  struct at `model.go:41`, same field shape) plus lease info (`runFeatureStatus`,
  `cli.go:954-1043`). This is genuinely shell-invocable by an agy-executor Lane subprocess
  (`lucind-ai feature status`) — closes the "is there a CLI surface" question, though it is
  literally named `feature status`, not `feature list`; there is no separate `list` subcommand
  (`featureDispatch`, `cli.go:867` only routes create/status/recover/renew/disable).
- `lucind-ai feature status --id <id>` also returns lease + `ListAttempts` info for one feature.

## 4. Origin classification primitives (diff base_sha vs current)

- Strong reusable precedent exists: `internal/overlap/overlap.go` — `Evaluate(ctx, repoDir,
  baseSHA, shaA, shaB, ...)` (line 1007) runs `CaptureRaw`, `NormalizeChanges` (line 353),
  `ExtractSignals`, `Classify` producing an `Evidence` struct with `BaseSHA`, per-branch
  `PathChange` lists (labels: rename/delete, binary, mode-only, generated, symlink/submodule,
  executable), line-add/delete metrics, and a content-hash (`ComputeHash`). This is a genuine
  base_sha-anchored git-diff classification engine already built for cross-feature overlap
  detection — directly reusable (or a close pattern to imitate) for ultrafixer's "diff against
  feature's own base_sha" origin-classification step.
- `FindUniqueMergeBase` (`overlap.go`) computes merge-base when no `baseSHA` given.
- `internal/resolve/candidate.go:97` `EnforceAllowedPaths` shells `git diff --name-status -z
  --diff-filter=ACDMRT -M <baseSHA> HEAD` plus unstaged/staged/untracked variants directly via
  `exec.CommandContext(ctx, "git", ...)` — confirms the codebase's own convention for base_sha
  diffing is a **direct git subprocess shellout**, not a dedicated Go git library. No existing
  "bisect" helper found — nothing wraps `git bisect`; if ultrafixer needs bisection (not just
  base_sha diff), it would shell out to plain `git bisect`/`git log` itself, following this exact
  pattern.

## 5. Defect Record precedent

- No standalone "Defect Record" table exists. Closest precedent: `approvals` table
  (`internal/ledger/ledger.go:574` `Approval` struct) has a `DefectSurfacedLater bool` column
  (`defect_surfaced_later`), set via `MarkDefectSurfaced` (`ledger.go:643`) and read via
  `ApproverRate` (`ledger.go:797`, flagged/approved ratio for approver accuracy tracking). This is
  scoped to "a defect surfaced after a human approved a lane" — a narrow, different concept from
  "ultrafixer recorded a non-critical/non-blocking pre-existing defect with evidence." **This
  would be new schema** (new table, e.g. `defects`/`defect_records`) — no reusable table exists
  today for evidence-only, no-code-touched dispositions.
- `dependencies-defects.md` confirms explicitly: "Issue #4 automation is absent today. `lucind-ai`
  does not automatically perform Defect Assessment, create a Defect Record, prepare a fix Change,
  create an External Work Item, or launch remediation." Manual contract is entirely
  human/Orchestrator-driven today.
- **Correction to the original sub-agent finding**: it claimed "schema is already at v4 per
  feature-parent-integration's archived `state.yaml`". Verified directly against
  `internal/ledger/schema.go:10` (`const schemaVersion = 7`) — the **actual current schema version
  is 7**, not 4. `feature-parent-integration`'s v4 note was accurate only as of 2026-08-20; the
  now-archived `2026-08-24-lane-status-observability` change shipped the v7 migration
  (`runs.pid` + `lane_progress` telemetry) after that. Any new Defect Record migration must be
  **v8**, and must be sequenced after v7, not v5.

## 6. CodeGraph impact/affected commands

- Confirmed genuinely real and invocable — this exploration's own `codegraph_explore` calls
  returned real "blast radius" data (caller counts, files, test coverage) computed from the
  pre-built `.codegraph/` SQLite index, e.g. "`Agy` (`internal/executor/agy.go:69`) — 16 callers in
  `cmd/lucind-ai/cli.go`". The underlying `codegraph impact`/`codegraph affected` CLI subcommands
  are documented as read-only intelligence commands in this project's own CodeGraph guidance
  (alongside status/query/explore/node/files/callers/callees).
- Important scoping fact: **lucind-ai's own Go codebase has ZERO references to "codegraph
  impact"/"codegraph affected"** (grep across the whole worktree found no matches) — CodeGraph is
  an *external* CLI tool from the Gentle AI ecosystem, not something lucind-ai wraps or calls
  internally. Ultrafixer's Lane (an agy subprocess) would need the `codegraph` binary + its own
  `.codegraph/` index available in each candidate feature branch's worktree/PATH to shell out to it
  directly — this is an environment/tooling dependency, not existing lucind-ai code.

## 7. Existing multi-feature-parallel evidence (real, right now)

Confirmed via `git worktree list` on the primary checkout: multiple live parallel worktrees exist
today, including `feature/agy-quota-wave-gate`, `feature/opencode-customizations`,
`feature/skill-modularization`, `feature/integration-target-isolation`, plus several in-flight
`native-stability-campaign` lens/apply worktrees and two `lucind-ai-fixer` worktrees
(`fix/ecosystem-defects-abc`, `fix/ecosystem-integrate-retry-cas-baseline`). This is concrete,
current, real-world proof that ultrafixer's "evaluate blocking independently per active feature
branch" scenario is not hypothetical — several real active branches exist simultaneously right now
that a pre-existing defect could plausibly affect.

## 8. Prior "issue #4 defect automation" groundwork

Confirmed nothing beyond the manual contract in `dependencies-defects.md` exists. No
defect-automation Go code, no TODOs found matching that specifically. The
`approvals.defect_surfaced_later` column (item 5) is the only defect-adjacent schema/state in the
entire ledger, and it is unrelated in scope (post-approval regret tracking, not
pre-existing-defect evidence recording). Confirms the design's premise cleanly: ultrafixer would
be greenfield for the Defect Record and disposition-per-branch mechanics; the only reuse
candidates are `internal/overlap` (diff/classification engine, item 4) and the
`result.Envelope`/`Question`/`Finding` types (item 2) as carriers.

## 9. Prior Engram context (closed by orchestrator follow-on search)

- `sdd-init/lucind-ai` (#1707): Go 1.24.2, strict TDD, testing capabilities on file — already
  satisfied the SDD init guard for this session.
- `#2490` (bugfix): "Fixed integrate retry stale CAS baseline for multi-wave features" —
  `lucind-ai integrate retry` was using the feature row's immutable `expected_parent_sha`/
  `base_sha` (set once at `feature.Service.Create`, never updated) instead of the original
  packet's own dispatch-time value, breaking retry for any multi-wave feature past wave 1. Fixed
  in `fix/ecosystem-integrate-retry-cas-baseline` — merged. Directly relevant: ultrafixer's Lanes
  will call `lucind-ai integrate`/`integrate retry` against feature targets, so this fix is a
  precondition for ultrafixer's integration step working correctly on any multi-wave feature.
- `lane-status-observability` (archived `2026-08-24-lane-status-observability`): shipped
  structured Lane progress telemetry, orphan-lane reconciliation via a `serve` Sweeper, and the
  schema v7 migration (`runs.pid` + `lane_progress` tables). No direct conflict with ultrafixer's
  design, but confirms schema is at v7 (see item 5 correction) and that the `serve`/Sweeper
  surface is an active, evolving part of the codebase ultrafixer's design phase should be aware of
  if it ever needs orphan-Lane-style liveness checks for its own fixer Lanes.

## Hard blockers / surprises for downstream sdd-propose/sdd-design

1. **No `codegraph` wrapper in lucind-ai** — ultrafixer's cross-branch impact filter depends
   entirely on an external binary/index being present per-worktree; this is an environment
   precondition to design around, not existing capability.
2. **`blocked` result envelope has no native multi-branch disposition field** — usable today via
   repeated `Question` entries or `Finding.Affects`, but a clean multi-branch design will want to
   decide whether to extend the schema (`internal/result/result.schema.json`) or reuse existing
   free-text fields.
3. **No Defect Record schema exists** — new ledger table/migration required. Current schema
   version is **7** (verified at `internal/ledger/schema.go:10`); a new migration must be **v8**.
4. **No CLI `feature list`** — only `feature status` (no `--id`) serves that purpose; fine
   functionally, but naming may confuse a spec/design reader expecting `feature list`.
5. **`internal/overlap` is the strongest reuse candidate** for origin classification — evaluate
   reusing its `Evaluate`/`NormalizeChanges`/`Classify` machinery (or its exact base_sha-diff
   pattern) rather than inventing a new git-diff helper.

## Ready for Proposal

Yes. Evidence is complete enough for `sdd-propose`/`sdd-design` to make informed extension
decisions on: (a) whether to extend `result.schema.json` for multi-branch blocked dispositions,
(b) new ledger schema v8 for Defect Records, (c) how ultrafixer's Lane shells out to
`codegraph`/`git` (both confirmed as direct-subprocess patterns already used elsewhere in this
codebase, e.g. `internal/resolve/candidate.go`), and (d) exact CLI verbs ultrafixer would shell
out to (`lucind-ai feature status`, `lucind-ai feature create`, `lucind-ai run --packet ...`,
`lucind-ai integrate`/`integrate retry`).
