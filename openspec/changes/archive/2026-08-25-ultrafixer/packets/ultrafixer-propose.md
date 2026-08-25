---
id: ultrafixer-propose
executor: agy
routed_by: SDD simple (no fan-out) phase dispatch for feature/ultrafixer — user explicitly requested every phase from propose through archive run via lucind-ai + agy dispatch, not Claude Code SDD sub-agents.
allowed_paths: ["openspec/changes/ultrafixer/proposal.md", "openspec/changes/ultrafixer/state.yaml"]
feature: ultrafixer
parent_ref: refs/heads/feature/ultrafixer
base_sha: 3ad2f2a84c5d509a1a9457d5b6cc98c1eb445538
expected_parent_sha: 3ad2f2a84c5d509a1a9457d5b6cc98c1eb445538
---

# Packet ultrafixer-propose

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/ultrafixer-propose  ·  **Branch:** lucind/ultrafixer-propose

## Goal

Write `openspec/changes/ultrafixer/proposal.md`, the SDD proposal for the `ultrafixer`
capability, following this repo's own proposal convention (see
`openspec/changes/control-room-capture/proposal.md` in this worktree for the exact shape: title +
one-paragraph thesis, `## Intent`, `## Selected Candidate and Approach`, `## Conceptual Changes`,
`## Capabilities` split into New/Modified, `## User and Capability Impact` table, `## Delta
Specifications` with Requirement/Scenario GIVEN-WHEN-THEN blocks, `## Alternatives Considered`,
`## Technical Risks and Failure Modes` table, `## Rollback Plan and Additivity`, `## Test and
Validation Impact` table, `## Out of Scope`, `## Open Questions`, `## Success Criteria`). Update
`openspec/changes/ultrafixer/state.yaml`'s `phases.proposal.status` to `done`.

## Why this is safe to dispatch now

The design below was already fully grilled and confirmed with the human across four rounds of
questioning (scope, dispatch mechanism, critical/blocking semantics, cross-branch detection,
permission gate, integration mechanism, decline handling). Nothing in it is an open product
question for this packet to resolve — your job is to **formalize and ground it in code evidence**,
not re-derive or re-litigate it. `openspec/changes/ultrafixer/explore.md` (already committed at
this packet's `base_sha`) already did the codebase investigation; use it as your primary evidence
source, plus your own `codegraph_explore`/`git`/`grep` verification of anything you cite that
explore.md doesn't already cover.

## Preconditions

- `openspec/changes/ultrafixer/explore.md` and `state.yaml` already exist at this packet's
  `base_sha` (committed `3ad2f2a`). Read them first — do not re-run exploration from scratch.
- Feature `ultrafixer` is registered and active in the ledger (`lucind-ai feature status --id
  ultrafixer` from the primary root, if you need to confirm).

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** (Not
      applicable to a docs-only packet in the usual code sense — instead: every new
      capability/concept named in `proposal.md`'s Capabilities section must be traceable to a
      concrete file/mechanism named in its own Impact table or Delta Specifications, not left as
      an abstract label with nothing pointing at it.)
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and `git log --oneline
      -1`. Conventional commit, no AI attribution.
- [ ] `openspec/changes/ultrafixer/proposal.md` exists, follows the local convention cited above,
      and every `file:line` citation in it is real (you verified it, not copied blind from
      explore.md without checking it still applies at this SHA).
- [ ] `openspec/changes/ultrafixer/state.yaml`'s `phases.proposal` is updated to `status: done`
      with a short `note:` summarizing what was written (do not touch any other phase's entry).
- [ ] The proposal's `## Alternatives Considered` section reflects the ACTUAL alternatives
      rejected during design grilling (see Context below — e.g. full autonomy vs. human-gated
      integration, standalone Claude-Code agent vs. real Lane dispatch, single vs. independent
      critical/blocking axes) — not fabricated generic alternatives.

## Allowed paths

- `openspec/changes/ultrafixer/proposal.md`
- `openspec/changes/ultrafixer/state.yaml`

## Allowed paths outside the repository

None.

## Out of scope

- Writing spec, design, or tasks artifacts — those are later phases, dispatched separately.
- Any code change under `internal/`, `cmd/`, or `plugin/`.
- Modifying `explore.md` (read-only input) or `/home/lanzerdev/.claude/agents/lucind-ai-fixer.md`
  (a separate, already-existing agent — out of scope entirely, never touch it).
- Re-litigating any of the confirmed design decisions below. If you believe one is genuinely
  wrong or infeasible given explore.md's evidence, say so explicitly as a flagged risk/open
  question in the proposal — do not silently change it.

## Hard stops

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did not
  anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and the packet does not say which.
- Satisfying one instruction in this packet would require violating another.
- The confirmed design below turns out to contradict hard evidence in `explore.md` in a way that
  cannot be reconciled without a human product decision (e.g. a load-bearing assumption the
  design relies on turns out to be false). Name the contradiction precisely; do not paper over it.

## Context

### Confirmed design for `ultrafixer` (grilled and human-approved — formalize, do not re-derive)

`ultrafixer` is an agy subagent responsible for fixing errors that surface during the development
of ANY project lucind-ai orchestrates. It is separate and exclusive from the already-existing
`lucind-ai-fixer` Claude Code agent, which stays the sole owner of defects in lucind-ai's own repo
(never modify or duplicate it).

1. **Dispatch mechanism**: a real Lane inside lucind-ai's own packet/dispatch system (Change →
   packet → Lane, `executor: agy`) — not a standalone Claude Code SendMessage agent. One fresh
   Lane per detected error (never persistent/long-lived). Trigger is manual: the Orchestrator of
   the affected feature arms the packet when it sees a failed check.
2. **Step 1 — origin classification** (done by ultrafixer's own Lane, first thing): diff the
   error against the feature's own `base_sha`/`parent_ref` to determine whether it was introduced
   during this feature's own development (→ not ultrafixer's job, the feature handles it,
   ultrafixer's Lane exits) or pre-existed before the feature started (→ continue). Rejected
   alternative: a baseline test/lint snapshot captured at feature-creation time — rejected because
   `base_sha`/`parent_ref` is already a persisted anchor every feature declares (see explore.md
   item 4, `internal/overlap`'s `Evaluate`/`NormalizeChanges` reuse candidate), no new
   infrastructure needed.
3. **Step 2 — two independent evaluation axes** for a pre-existing error: (a) **critical**
   (security risk, data loss/corruption, or CI/build breakage — deliberately NOT "any bug"), and
   (b) **blocking**, evaluated *separately per feature branch* — own branch, and independently for
   each other active feature branch discovered via `lucind-ai feature status` (no `--id`; see
   explore.md item 3). These two axes were explicitly confirmed as independent (a critical,
   non-blocking security defect is a real, distinct case, not folded into "blocking").
4. **Step 3 — disposition per branch**: if critical OR blocking (for that specific branch) →
   ultrafixer's Lane repairs in its own isolated worktree, runs tests, commits, and the Lane
   terminates in a `blocked` result (using `internal/result/result.go`'s `Question`
   struct — `Question`/`WhyBlocking`/`Options`/`Recommendation`, already confirmed rich enough per
   explore.md item 2) carrying the fix + evidence. It does **not** integrate itself — no
   `--approval-timeout`-gated auto-integrate. If neither critical nor blocking for that branch →
   only a Defect Record (evidence in durable ledger/memory — new schema, explore.md item 5/9
   confirms current schema is v7, so this would be a v8 migration), no code touched, no Change
   generated.
5. **Cross-branch impact detection**: CodeGraph (`codegraph impact`/`codegraph affected` —
   confirmed real but an *external* tool/index dependency, not lucind-ai-internal; see explore.md
   item 6) as the candidate filter for "which other active features might this touch," then the
   error's actual signal (failing test/lint/build) must be reproduced in that specific branch
   before it counts as genuinely affected — file/path overlap alone is explicitly insufficient.
6. **Integration**: always manual and human-initiated. The SAME human/Orchestrator who dispatched
   ultrafixer decides, for every affected branch (own + others), whether/when to run `lucind-ai
   integrate` / `lucind-ai integrate retry` themselves — reusing the CAS-safe integration machinery
   already fixed for multi-wave features (explore.md item 9, the `expected_parent_sha`/`base_sha`
   stale-baseline bugfix). Rejected alternative: a live `--approval-timeout` + `serve --approver`
   gate — rejected as unnecessary background-process complexity for v1, since the manual-trigger
   philosophy (item 1) is already conservative.
7. **On decline**: the fix worktree/branch is preserved (never deleted without explicit
   instruction — mirrors `lucind-ai-fixer`'s own "cheap to keep, expensive to lose" rule), with a
   "declined" disposition recorded in ledger/memory.

### Key evidence from `explore.md` (already committed — read the full file, this is a condensed pointer, not a substitute)

- Packet/Lane/agy plumbing: `internal/packet/packet.go:33`, `cmd/lucind-ai/cli.go:82` (executor
  map), `internal/executor/agy.go:69/136`, `internal/run/integrate_feature.go:26`
  (`FeatureTarget`), `internal/run/batch.go:66` (`ExecuteBatch`).
- `blocked` result contract: `internal/result/result.go:77` (`Question`), `:102` (`Envelope`),
  `:122` (`LaneStatus`) — no native per-branch fan-out field; `Finding.Affects` (`:98`) is the
  informal per-branch carrier available today without a schema change.
- Feature discovery: `internal/ledger/ledger.go:1342` (`FeatureRow`), `:1353` (`ActiveFeatures`,
  Go-internal only); CLI surface is `lucind-ai feature status` with no `--id`
  (`internal/serve/model.go:536`, `cli.go:954-1043`) — there is no separate `feature list` verb.
- Origin classification: `internal/overlap/overlap.go:1007` (`Evaluate`) is a full base_sha-diff
  classification engine, strong reuse candidate. `internal/resolve/candidate.go:97`
  (`EnforceAllowedPaths`) confirms direct `git` subprocess shellout is the codebase's own
  convention — no wrapped git library, no existing `git bisect` helper.
- Defect Record: no existing table; closest is `approvals.defect_surfaced_later`
  (`internal/ledger/ledger.go:574/643`), a narrow, unrelated concept. **Current schema version is
  7** (`internal/ledger/schema.go:10`, verified directly — not the stale "v4" figure an earlier
  pass of this exploration briefly cited). A new Defect Record table would be schema **v8**.
- CodeGraph: real and invocable, but zero references to `codegraph impact`/`codegraph affected`
  anywhere in lucind-ai's own Go code — confirmed external-tool/index dependency.
- Real multi-feature-parallel evidence right now: `feature/agy-quota-wave-gate`,
  `feature/opencode-customizations`, `feature/skill-modularization`,
  `feature/integration-target-isolation`, plus in-flight `native-stability-campaign` lens/apply
  worktrees and two `lucind-ai-fixer` worktrees — concrete proof the "evaluate blocking
  independently per active branch" scenario is real, not hypothetical.
- `dependencies-defects.md` (`plugin/claude-code/skills/lucind-ai/references/coordination/
  dependencies-defects.md`) confirms today's manual contract: no automatic Defect Assessment, no
  automatic remediation, human confirmation required before any remediation activation — this is
  precisely the gap `ultrafixer` fills, with the human-gate-on-integrate design (item 6 above)
  staying deliberately conservative relative to full automation.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how well
the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
