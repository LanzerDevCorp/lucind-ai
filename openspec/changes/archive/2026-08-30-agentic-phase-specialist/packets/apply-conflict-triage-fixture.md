---
id: apply-conflict-triage-fixture
executor: cursor-agent
routed_by: single-packet sequential apply of an accepted four-phase tasks checklist under strict TDD
model: cursor-grok-4.6-high
allowed_paths: ["internal/conflicttriage/types.go", "internal/conflicttriage/invoker.go", "internal/conflicttriage/triage.go", "internal/conflicttriage/triage_test.go", "internal/conflicttriage/fixture/fixture.go", "internal/conflicttriage/fixture/fixture_test.go", "internal/conflicttriage/fixture/rubric.go", "internal/conflicttriage/fixture/rubric_test.go", "internal/conflicttriage/fixture/packets/", "internal/reconcile/reconcile.go", "internal/reconcile/reconcile_test.go", "internal/resolve/candidate_test.go", "internal/run/attempt_test.go", "cmd/lucind-ai/cli_test.go", "openspec/changes/conflict-triage-fixture/tasks.md"]
---

# Packet apply-conflict-triage-fixture

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/apply-conflict-triage-fixture  ·  **Branch:** lucind/apply-conflict-triage-fixture

## Goal

Implement every task in `openspec/changes/conflict-triage-fixture/tasks.md`, in its stated
dependency order, as four sequential work-unit commits. When you are finished the repository
contains an advisory conflict-triage agent, a deterministic three-hunk fixture that forces
`overlap.ClassRequired`, and an offline dual-judge rubric — with `./lucind-checks.sh` green on
the combined tree and every checkbox in `tasks.md` ticked.

## Why this is safe to dispatch now

The proposal, spec, design and tasks for this change are all accepted and present in this
worktree. `tasks.md` is the canonical checklist, synthesized from three lens drafts and
promoted after a human comparison; its citations were verified against this repository.

Two design questions remain deliberately open (`design.md:118-123`) and **neither blocks this
work**, because `tasks.md` assigns no task to either: the exact non-decreasing risk formula
and thresholds, and which executor/model runs *production* triage. The judges are pinned; the
production runtime is not. Do not resolve either.

## Preconditions

- `openspec/changes/conflict-triage-fixture/tasks.md`, `design.md`, `proposal.md` and
  `specs/` all exist in this worktree.
- `./lucind-checks.sh` is green before you start. If it is not, return `blocked` — you did
  not break it and must not repair someone else's failure inside this packet.
- `internal/conflicttriage/` does not yet exist.

**A precondition satisfied by one of this packet's own later steps is a misordered packet.**
Return `blocked` and say so; do not work around it.

## Required procedure

`tasks.md` is the specification for this packet. Read it first, in full, and follow its
Dependency Order table. Do not re-derive its decisions.

### Strict TDD is mandatory

Every task marked RED writes a failing test **before** the production code that satisfies it.
For each RED task you must actually observe the failure — run the focused test command, see it
fail for the intended reason, and only then write the GREEN production code.

A test that passes the moment you write it is not RED. If a RED task's test passes
immediately, that is a finding, not a formality: say so in the envelope's notes and explain
what already satisfied it. Do not weaken the test to manufacture a failure, and do not silently
proceed as if it had failed.

Tasks 2.1 and 3.4 include cases that lock **existing** helpers. Those are regression tests and
are expected to pass without touching production code — that is correct, not a strict-TDD
violation, and `tasks.md` says so. Do not edit `internal/resolve/candidate.go` production to
make them fail.

### Four commits, not one

One commit per work unit, in order 1 → 2 → 3 → 4. Each unit contains its RED tests and the
GREEN production that satisfies them, so the combined tree is green at every commit — Integrate
checks the combined tree (`internal/run/integrate.go:50-59`), and a unit that is red on its own
is a unit that cannot be reverted cleanly.

Before each commit, run the unit's focused test command from the Suggested Work Units table,
then `./lucind-checks.sh` on the full tree. Conventional commit messages. **No
Co-Authored-By and no AI attribution of any kind.**

Tick each task's checkbox in `tasks.md` as you complete it. `tasks.md` is in `allowed_paths`
for exactly this reason and for no other — do not edit its content, ordering, or citations.

## Done criteria

- [ ] **Every indirection introduced is demonstrably consumed by a terminal consumer.** For
      each type, function, flag or config key added, name the program, test or file that
      *reads* it and attach the output that proves it. Another mention of the name is not
      consumption. `TriageInvoker` in particular must be consumed by `RunTriage`, and
      `TriagePayload` by both `RunTriage` and `EvaluateRubric`.
- [ ] **The work is committed.** Evidence: `git status --porcelain` empty and
      `git log --oneline -4`. Four conventional commits, one per work unit, no AI attribution.
- [ ] **`./lucind-checks.sh` exits 0 on the combined tree.** Attach the tail of its output.
- [ ] **Every RED task was observed failing before its GREEN.** For each, name the focused
      test command and the failure message you saw. Where a RED test passed immediately,
      say which one and what already satisfied it.
- [ ] **The fixture forces `ClassRequired`.** Evidence: the output of
      `go test ./internal/conflicttriage/fixture -run '^TestFixtureGenerator_ForcesClassRequired$' -count=1 -v`.
- [ ] **Every checkbox in `openspec/changes/conflict-triage-fixture/tasks.md` is ticked**, and
      no other line of that file changed. Evidence: `git diff` of that file against the lane's
      birth point showing only `- [ ]` → `- [x]`.
- [ ] **Both open design questions are still open.** No task closed the risk formula or named
      a production triage executor/model.

## Allowed paths

Only these may be created or modified. Touching anything else is a **deviation** — finish
nothing further, report it, and stop.

Paths are enumerated as files rather than as the `internal/conflicttriage/` directory on
purpose: a directory prefix would also grant `fixture/`, which `tasks.md` calls out as the trap
that would silently break the work units' disjointness (`internal/packet/disjoint.go:13-22`).

- `internal/conflicttriage/types.go`, `invoker.go`, `triage.go`, `triage_test.go`
- `internal/conflicttriage/fixture/fixture.go`, `fixture_test.go`, `rubric.go`, `rubric_test.go`, `packets/`
- `internal/reconcile/reconcile.go`, `internal/reconcile/reconcile_test.go`
- `internal/resolve/candidate_test.go`
- `internal/run/attempt_test.go`
- `cmd/lucind-ai/cli_test.go`
- `openspec/changes/conflict-triage-fixture/tasks.md` (checkboxes only)

## Allowed paths outside the repository

None. This packet may touch nothing outside the repository.

## Out of scope

- `internal/resolve/candidate.go` production code. Tasks 2.1 and 3.4 lock its existing
  behavior with tests; they must pass without editing it.
- `internal/overlap/overlap.go`. The fixture must force `ClassRequired` through the existing
  thresholds (`overlap.go:93-98,623-659`), never by changing them.
- Any live LLM call. Both judges run offline against stub executors
  (`internal/executor/claude_test.go:18-26`, `opencode_test.go:19-27`).
- `apply-dag.yaml`. `tasks.md` decided against a sidecar; this is one sequential packet.
- The two open design questions.

## Hard stops

Stop and return `status: blocked` — do not guess. **Declare every one of these in the
envelope**, whether or not it fired.

- Any credential value would need to be chosen, generated, or written.
- A done-criterion turns out to be impossible, or already true for a reason the packet did
  not anticipate.
- The change would break something outside `allowed_paths`.
- Two reasonable implementations exist and `tasks.md` does not say which.
- Satisfying one instruction in this packet would require violating another.
- The fixture cannot be made to reach `ClassRequired` without altering
  `internal/overlap/overlap.go` thresholds.
- A task would require closing one of the two open design questions.

## Context

Established facts, already verified — do not re-derive them.

- `tasks.md` is canonical and its citations were verified against this repository during
  synthesis. `tasks-synthesis-notes.md` records dropped citations, coverage gaps and
  decomposition divergence; read it if a task seems to contradict a lens draft.
- `evaluateOverlapGate` (`internal/run/attempt.go:687`) calls `overlap.Classify`
  (`internal/overlap/overlap.go:622-659`); `ClassRequired` is what creates a reconciliation
  request. `DefaultThresholds` is at `overlap.go:93-98`.
- Reconciliation closes in two steps: `reconcile approve` authorizes a candidate only; a human
  resolves out of band and registers with `reconcile resolve --candidate --sha`.
- `UpdateCandidateStatus`'s SQL (`internal/reconcile/reconcile.go:873-876`) does not touch
  `output`, which is why task 1.4 needs a separate output-only path through
  `ledger.UpdateReconciliationCandidate` (`internal/ledger/ledger.go:1314-1338`).
- Executors are pinned one model per provider family: `agy`/gemini-3.7-flash-high,
  `cursor-agent`/cursor-grok-4.6-high, `opencode`/openai gpt-5.6-sol and gpt-5.6-luna,
  `claude`/claude-opus-5 (`cmd/lucind-ai/cli.go:65-70`).
- `./lucind-checks.sh` is the full-tree check. `./lucind-lane-check.sh` is the mechanical
  self-check for lane artifacts and is not needed here.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. That file is what the
dispatching binary reads. Printed output alone will be read as a lane that produced nothing.

The schema is at `.lucind/result.schema.json` in this worktree. Validate against it before
writing — an envelope that fails schema validation makes the lane `blocked` regardless of how
well the work went.

Report `done` only when every done-criterion carries evidence and every hard stop is declared.
