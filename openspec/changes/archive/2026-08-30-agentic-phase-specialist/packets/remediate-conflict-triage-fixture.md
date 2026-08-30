---
id: remediate-conflict-triage-fixture
executor: cursor-agent
routed_by: bounded remediation of two confirmed verify gaps plus evidence-backed triage of six carried-forward claims
model: cursor-grok-4.6-high
feature: conflict-triage-fixture
parent_ref: feature/conflict-triage-fixture
base_sha: 22405f73191bf701d4822193d864242b252e02f9
expected_parent_sha: 22405f73191bf701d4822193d864242b252e02f9
allowed_paths: ["internal/conflicttriage/types.go", "internal/conflicttriage/triage.go", "internal/conflicttriage/triage_test.go", "internal/conflicttriage/fixture/fixture.go", "internal/conflicttriage/fixture/fixture_test.go", "internal/conflicttriage/fixture/rubric.go", "internal/conflicttriage/fixture/rubric_test.go", "internal/conflicttriage/fixture/packets/", "openspec/changes/conflict-triage-fixture/remediation.md"]
---

# Packet remediate-conflict-triage-fixture

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-conflict-triage-fixture  ·  **Branch:** lucind/remediate-conflict-triage-fixture

## Goal

Close the two confirmed gaps in `openspec/changes/conflict-triage-fixture/verify.md`, and triage its
six carried-forward claims with evidence — fixing the ones that are real and refuting the ones that
are not. When you are finished, `verify.md`'s remediation list is answered item by item and
`./lucind-checks.sh` is green.

## Why this is safe to dispatch now

Verify has already run and its verdict is committed at `openspec/changes/conflict-triage-fixture/verify.md`.
Its two CONFIRMED findings were independently checked against the code by the orchestrator, not taken
from a lane's word. The six carried-forward claims were NOT independently checked — that is precisely
what this packet is for, and why refuting one is as acceptable an outcome as fixing it.

The two design questions in `design.md:118-123` stay open and no task here may close them.

## Preconditions

- `openspec/changes/conflict-triage-fixture/verify.md` exists in this worktree.
- `./lucind-checks.sh` is green before you start. If not, return `blocked` — you did not break it.
- `openspec/changes/conflict-triage-fixture/remediation.md` does not yet exist.

## Required procedure

Read `verify.md` first, in full. It is the specification for this packet.

### Part 1 — the two CONFIRMED gaps (both must be closed)

**1. The fixture packets cannot write the file they must collide on.** `ToyPath` is `"toy.go"` at the
repository root (`internal/conflicttriage/fixture/fixture.go:17-18`) and `GenerateFixture` writes it
there; `packets/feat_a.md` grants only `["fixture/feat-a/"]` and `packets/feat_b.md` only
`["fixture/feat-b/"]`. A lane dispatched from either cannot write `toy.go`, and the binary demotes a
`done` envelope to `deviated` when the diff leaves `allowed_paths`.

Decide which of the two worlds is true and make the tree say so:

- If `feat_a.md`/`feat_b.md` are meant to be the dispatch shape that produces the toy collision,
  their `allowed_paths` must actually cover `ToyPath`, and the two must still be non-disjoint on it —
  that overlap is the point of the fixture, not a bug in it.
- If they are build-scope templates unrelated to the toy, say so **inside the packets themselves**,
  in a sentence a reader hits before the frontmatter's meaning is assumed, and make
  `fixture_test.go` assert the relationship you claim.

Do not split the difference by leaving both readings available.

**2. The refusal scenario has no test.** `specs/conflict-fixture/spec.md:24-30` requires that two
build features whose allowed paths are **not** prefix-disjoint MUST fail admission when placed in one
batch. `TestFixturePackets_DisjointAndValidParentRef` (`fixture_test.go:122-155`) asserts only that
the shipped packets *are* disjoint. Add the negative test the scenario actually states, driving the
real admission path rather than asserting on a hand-built error value.

### Part 2 — triage the six carried-forward claims

`verify.md`'s "Findings carried forward as claims, not verdicts" lists six, each with citations.
For **every one** of them, reach a verdict backed by evidence:

- **CONFIRMED** — reproduce it, fix it, and add the test that would have caught it.
- **REFUTED** — name the code or test that already covers it, with `file:line`. A refutation with no
  citation is not a refutation.

Two carry a caution. Claim 6 (the business hunk's "both sides compile and pass their own tests" is
never built or run) is the fixture's defining property — if it is real, locking it is worth real
work, not a comment. Claim 2 (the rubric grades canned JSON rather than fixture evidence) touches the
A/B that justifies this whole change; if you confirm it, say clearly what the rubric currently proves
and what it does not, because the answer changes what the A/B result will mean.

Record every verdict in `openspec/changes/conflict-triage-fixture/remediation.md`:

```markdown
# Remediation: Conflict Triage Fixture

## Confirmed gaps closed

### 1. <gap>
<what you changed, and the test that now covers it>

## Carried-forward claims

### Claim <n> — <CONFIRMED | REFUTED>
<evidence with file:line. If confirmed: what you changed. If refuted: what already covers it.>
```

### Strict TDD

Every fix lands test-first. Observe the failure before writing the production change, and say in the
envelope what failure message you saw. A test that passes the moment you write it is a finding worth
reporting, not a formality to wave through — say so rather than weakening the test to manufacture red.

### Commits

One conventional commit per closed gap or confirmed claim; a refutation that changes no code needs no
commit of its own. Check `git log -1 --format=%B` after each: this environment's commit wrapper
appends a `Co-authored-by:` trailer the message never contained. Strip it.

## Done criteria

- [ ] **Both CONFIRMED gaps are closed**, each with a test that fails without the fix.
- [ ] **All six carried-forward claims carry a verdict** in `remediation.md`, each CONFIRMED with a
      fix and test, or REFUTED with a `file:line` citation to what already covers it.
- [ ] **`./lucind-checks.sh` exits 0** on the combined tree. Attach the tail.
- [ ] **Every RED was observed failing before its GREEN**, with the failure message named.
- [ ] **Both open design questions are still open.** No change encoded a risk formula or named a
      production triage executor.
- [ ] **The work is committed**, `git status --porcelain` empty, no AI attribution in any message.

## Allowed paths

Only these. Touching anything else is a **deviation** — stop and report.

Enumerated as files rather than as `internal/conflicttriage/`, because a directory prefix would also
grant `fixture/` and silently dissolve the boundary `tasks.md` drew.

- `internal/conflicttriage/types.go`, `triage.go`, `triage_test.go`
- `internal/conflicttriage/fixture/fixture.go`, `fixture_test.go`, `rubric.go`, `rubric_test.go`, `packets/`
- `openspec/changes/conflict-triage-fixture/remediation.md`

## Allowed paths outside the repository

None.

## Out of scope

- `internal/resolve/candidate.go`. Fail-closed behavior is not relaxed, and its tests must keep
  passing untouched.
- `internal/overlap/overlap.go`. The fixture forces `ClassRequired` through existing thresholds.
- `internal/run/`, `cmd/`. If a fix appears to need either, that is a hard stop, not a widening.
- `verify.md` and `tasks.md`. They are the record this packet answers, not files it edits.
- The two open design questions.

## Hard stops

Stop and return `status: blocked` — declare every one in the envelope whether or not it fired.

- A fix would require editing a path outside `allowed_paths`.
- Closing gap 1 would require choosing between the two readings on grounds the tree does not settle.
- A claim can be neither confirmed nor refuted from the code, and deciding it needs a product call.
- Any credential value would need to be chosen, generated, or written.
- Satisfying one instruction here would require violating another.
- A fix would require closing one of the two open design questions.

## Context

- `verify.md` is committed in this worktree and is the authoritative list. Its two CONFIRMED findings
  were verified against the code directly; its six claims were not.
- `GenerateFixture` builds the conflict with real git commits (`fixture.go:79,99,116`), independently
  of the packet templates. That is why gap 1 is a coherence gap rather than a broken fixture.
- `evaluateOverlapGate` (`internal/run/attempt.go:687`) calls `overlap.Classify`
  (`internal/overlap/overlap.go:622-659`); `ClassRequired` is what creates a reconciliation request.
  `DefaultThresholds` (`overlap.go:93-98`) sets `NearbyHunkLines`.
- `packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:13-48`) is the real admission check; a
  directory path covers its descendants.
- `./lucind-checks.sh` is the full-tree check.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. The schema is at
`.lucind/result.schema.json`; validate before writing. Report `done` only when every done-criterion
carries evidence and every hard stop is declared.
