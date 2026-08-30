---
id: apply-deterministic-lucind-ai-orchestrator
executor: cursor-agent
routed_by: single-packet sequential apply, no apply-dag.yaml sidecar (tasks.md Sidecar Recommendation and Review Workload Forecast)
model: cursor-grok-4.6-high
allowed_paths: ["plugin/claude-code/skills/lucind-ai", "plugin/opencode/skills/lucind-ai", "internal/packet", "internal/dag", "internal/run", "cmd/lucind-ai", "openspec/changes/deterministic-lucind-ai-orchestrator/tasks.md"]
---

# Packet apply-deterministic-lucind-ai-orchestrator

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-deterministic-orchestrator-worktrees/apply-deterministic-lucind-ai-orchestrator  ·  **Branch:** lucind/apply-deterministic-lucind-ai-orchestrator

## Goal

Implement all five phases of `openspec/changes/deterministic-lucind-ai-orchestrator/tasks.md` in
one sequential lane: canonical skill/reference text and its byte-identical OpenCode re-sync,
target-free packet parsing and DAG split (pin existing, no change), fired-hard-stop demotion in
acceptance (RED then GREEN), idempotent attempt/CAS integration (pin existing, no change), and
skill-parity/schema-freshness CLI preflight (RED then GREEN).

This ships as **one PR** — the human confirmed this fits the session's 5000-line review budget
after the tasks synthesis forecast (700–1400 lines) came in well under it. Do not split this into
multiple lanes or PRs.

## Why this is safe to dispatch now

`tasks.md`, `design.md`, and all five capability deltas under
`openspec/changes/deterministic-lucind-ai-orchestrator/specs/` are accepted and frozen. The tasks
synthesis independently re-verified every citation against this exact branch (`61aa0cc`-merged),
corrected two stale claims (OpenCode tree already exists byte-identical; several CLI line numbers
shifted), and confirmed a single sequential packet — not a DAG — is correct: `Integrate` reverts a
wave whose own checks fail, so Strict-TDD RED and GREEN for one unit must land in one lane
(`internal/run/integrate.go:50-59`).

## Strict TDD Mode is ACTIVE

Test runner: `go test ./... -race -count=1`. For every task marked `-RED` in `tasks.md`, write the
named test first, run it, and confirm it fails **for the stated reason** — paste that failure
output as done-criterion evidence — before writing any production code. Only then implement the
paired `GREEN` task. Do not implement GREEN before its RED test exists and fails for the stated
reason. Do not skip a RED step because the behavior "obviously" doesn't exist yet.

## Required reading (read in full before writing anything)

1. `openspec/changes/deterministic-lucind-ai-orchestrator/tasks.md` — the canonical checklist.
   Follow its five phases, its Dependency Order table, and its Requirement Traceability table
   exactly. Task numbering (1.1, 1.2, 2.1, 2.2, 3.0-RED, 3.0b-RED, 3.1, 3.2, 4.1, 4.2, 5.0-RED,
   5.1) is authoritative; this packet does not repeat the per-task detail, `tasks.md` does.
2. `openspec/changes/deterministic-lucind-ai-orchestrator/design.md` — the seven architecture
   decisions and the file-changes table are the "how"; `tasks.md` cites it throughout.
3. `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` — all five capability deltas.
   Every task traces to at least one requirement; do not implement behavior the specs do not
   describe.
4. `openspec/changes/deterministic-lucind-ai-orchestrator/tasks-synthesis-notes.md` — read
   `## Dropped Citations` and `## Coverage Gaps` before trusting any line-number claim elsewhere;
   several citations in earlier drafts were wrong and this file has the corrected ones.

## Execution order

Follow `tasks.md`'s own Dependency Order table. In short: Phase 1 (skill text, then OpenCode
re-sync) and Phase 2 (packet/DAG — pin existing behavior, no change expected) have no
interdependency and may be done in either order; Phase 3 (RED hard-stop demotion, GREEN, batch
pin) depends on nothing else; Phase 4 (attempt/CAS — pin existing, no change expected) depends on
nothing else; Phase 5 (CLI preflight RED/GREEN) depends on Phases 1–4 all landing first, since its
preflight checks skill parity and calls existing run/packet APIs.

"Pin existing" tasks (2.1, 2.2, 4.1, 4.2) mean: run the named existing tests, confirm they already
pass, and touch nothing in that file unless a test is missing per `tasks.md`'s own notes (e.g. add
a Claude-vs-OpenCode tree comparator test for 1.2 — `tasks-synthesis-notes.md` flags this as a
real gap no lens named). Do not refactor code that is not named in a task.

## Out of scope

- Do **not** build, wire, or reference `LUCIND_REQUIRED_SKILLS`, a `required_skills` packet
  frontmatter field, `integrate retry` as a CLI verb, or `defect record/list/resolve/decline/defer`.
  Those belong to the unrelated `feature/skill-provisioning-and-phase-specialist` deliverable,
  already merged into this branch's history but explicitly out of scope for this Change
  (`design.md:119`). `internal/run/run.go:496-498`'s existing `enforceRequiredSkills` call is
  pre-existing code from that merge — do not modify it.
- No new CLI subcommand, no new lifecycle state, no scheduler/wave engine, no replacement for
  existing Combine/Resolve/Check/bisect/CAS primitives (`proposal.md:14-15`).
- No `apply-dag.yaml` sidecar and no second lane.

## Allowed paths

Exactly: `plugin/claude-code/skills/lucind-ai`, `plugin/opencode/skills/lucind-ai`,
`internal/packet`, `internal/dag`, `internal/run`, `cmd/lucind-ai`, and
`openspec/changes/deterministic-lucind-ai-orchestrator/tasks.md` (checkbox updates only — check
off completed tasks as you go). Touch no other path.

## Context

**Ground truth — verified in this worktree before this packet was authored:**

- Delivery: single PR, `ask-on-risk` confirmed by the human as "single PR" this session (estimate
  700–1400 lines, comfortably under the session's 5000-line review budget).
- Requirement traceability (`tasks.md:76-84`): `deterministic-orchestrator-contract` → 1.1, 1.2,
  5.0-RED, 5.1. `packet-authoring-contract` → 2.1, 4.2, 5.1. `sdd-apply` → 1.1, 2.2, 3.2.
  `acceptance-verifier` → 3.0-RED, 3.1, 3.2. `parent-feature-integration` → 4.1, 4.2.
- `tasks-synthesis-notes.md` `## Coverage Gaps` already names the two real, non-invented gaps
  worth extra care: (1) no lens named a Claude-vs-OpenCode tree-comparator test — add one when
  doing 1.2; (2) Phases 2 and 4 are mostly pin-existing — the real net-new production work is
  Phase 1 skill text, Phase 1 OpenCode re-sync, Phase 3's `HardStop.Fired` demotion, and Phase 5's
  CLI preflight additions. Do not invent additional production changes in Phases 2 or 4.

## Done criteria

- [ ] **Every task in `tasks.md`'s five phases is complete**, in Dependency Order, with each
  `-RED` task's failing-for-the-stated-reason output captured as evidence before its paired GREEN
  task began.
- [ ] **Every focused test command named per unit in `tasks.md`'s Suggested Work Units table
  passes.**
- [ ] **Full suite passes**: `go build ./...`, `go test ./... -race -count=1`, `./lucind-checks.sh`,
  `gofmt -l .` reports nothing.
- [ ] **`diff -rq plugin/claude-code/skills/lucind-ai plugin/opencode/skills/lucind-ai` is empty**
  after 1.2 (byte-identical trees).
- [ ] **No path outside `## Allowed paths` was touched**, and nothing under Out of Scope was built.
- [ ] **Every requirement in `openspec/changes/deterministic-lucind-ai-orchestrator/specs/` traces
  to completed work** per `tasks.md`'s traceability table.
- [ ] **The work is committed with a conventional commit and no AI attribution**
  (`git status --porcelain` empty and `git log --oneline -1`).

## Hard stops

Stop and return `status: blocked` — do not guess. Declare every one of these in the envelope,
whether or not it fired.

- A named RED test cannot be written because the failure it should reproduce does not exist (the
  behavior is already correct) — report this rather than writing a test that fails for the wrong
  reason.
- A task's target file, function, or line range does not match `tasks.md`'s citation in this
  worktree.
- Two tasks require touching the same code in incompatible ways.
- Satisfying one instruction in this packet would require violating another, or would require
  building something under `## Out of scope`.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against
`.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries
evidence and every hard stop is declared.
