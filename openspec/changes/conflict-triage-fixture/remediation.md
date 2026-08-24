# Remediation: Conflict Triage Fixture

## Confirmed gaps closed

### 1. Fixture packets cannot write the file they must collide on

The tree had already settled this: the two build features MUST keep
prefix-disjoint allowed paths (`specs/conflict-fixture/spec.md`), sequential
disjoint dispatch is a design decision (`design.md`), and colliding the two
*build* features is out of scope. `GenerateFixture` writes `toy.go` via git
independently of the packets (`fixture.go:78-130`).

World chosen: `feat_a.md` / `feat_b.md` are **build-scope templates**, not the
dispatch shape that produces the toy collision. Both packets now open with a
`#` comment naming `toy.go` before any frontmatter key, `routed_by` no longer
describes them as the toy's dispatch shape, and the body states that
`GenerateFixture` writes the toy independently.

Test: `TestFixturePackets_AreBuildScopeTemplatesNotToyWriters`
(`fixture_test.go:186`). RED:

```
feat_a.md: want a # comment before frontmatter keys so a reader hits it before assuming allowed_paths, got "id: fixture-feat-a"
feat_a.md body must say GenerateFixture writes the toy independently
feat_a.md routed_by still describes these packets as the toy's dispatch shape
```

(and the same three lines for `feat_b.md`).

### 2. The refusal scenario the spec states has no test

Added `TestFixturePackets_OverlappingBuildScopesRefused` (`fixture_test.go:215`).
It parses the shipped feat packets, then drives the real admission check
`packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:29`) with B's
allowed paths extended to overlap A's. Separate dispatch of the original
disjoint pair still succeeds.

This test **passed the moment it was written**. `DisjointAllowedPaths` already
refuses overlap; the gap was coverage of the spec's MUST-fail, not missing
production behavior. Not weakened to manufacture red.

## Carried-forward claims

### Claim 1 — REFUTED

`RunTriage` is not supposed to overwrite mechanical resolutions. The prompt
asks the invoker to resolve them deterministically (`triage.go:135`). The only
code-level pin is business hunks to ARBITRARY / high (`pinBusinessHigh`,
`triage.go:112-123`) — the case the invoker must not be trusted on.
`TestTriageAgent_BusinessHunkPinsHighRisk` already includes a mechanical
`HunkKindSliceUnion` with invoker-supplied `"union"` and asserts fail-open on
`resolve.ErrSemanticAmbiguity` (`triage_test.go:131-147`). The rubric grades
classification (three hunks, business distinguished as ARBITRARY, two
mechanical), not mechanical resolution tokens (`gradePayload`,
`rubric.go:112-138`). Encoding `"union"` / `"keep-rename"` as enforced tokens
would be a product call the spec does not name.

### Claim 2 — CONFIRMED

`EvaluateRubric` graded canned `TriagePayload` JSON from stub binaries and
never presented `GenerateFixture` evidence. The prompt was a generic
instruction; tests passed `t.TempDir()` as the worktree.

**What the rubric proved before this fix:** `gradePayload` (`rubric.go:112-138`)
checks three hunks, non-uniform scoring, business distinguished as ARBITRARY,
and two mechanical kinds. `TestRubric_GradesDistinctThreeHunkClassification`
(`rubric_test.go:66-110`) is the executor-argv isolation check: each judge
gets its pinned model and no cross-provider leak. That isolation result is
real and unchanged.

**What it did not prove:** that a judge looking at the three-hunk toy would
classify it. An A/B win under the old rubric meant "the stub emitted the
right JSON shape," not "the model separated the fixture's hunks."

Fix: `rubricPrompt` now attaches both sides of `toy.go` from the fixture
worktree (`toyEvidence`, `rubric.go:155-184`). Test:
`TestRubric_PresentsGenerateFixtureEvidence` (`rubric_test.go:136`). RED:

```
claude prompt missing fixture evidence "tier A"
```

Isolation tests still pass with empty worktrees (no git → no evidence attached).

### Claim 3 — CONFIRMED

Invariant coverage through `RunTriage` was markers-only
(`TestTriageAgent_InvariantViolationsFailCandidate`, `triage_test.go:248`).
Production already called `resolve.EnforceAllowedPaths` (`triage.go:70-74`).
Added `TestTriageAgent_OutOfScopeEditsFailCandidate` (`triage_test.go:306`):
the invoker writes `outside.go`, `RunTriage` must fail with
`resolve.ErrOutOfScopeEdits`.

This test **passed the moment it was written**. The production path already
enforced the invariant; the gap was a missing `RunTriage` test. Not weakened.

### Claim 4 — REFUTED

The missing/divergent-base skip is this change's contract at
`overlap.Evaluate`, not a defect of the fixture. `tasks.md:58` says task 3.1
proves `Classify`, not `evaluateOverlapGate`.
`TestFixtureGenerator_MissingBaseSHASkipsClassRequired` (`fixture_test.go:90`)
drives `overlap.Evaluate` with an empty common base and asserts
`overlap.ErrNoMergeBase`. The gate already continues on that error
(`evaluateOverlapGate`, `internal/run/attempt.go:745-747`). Proving it
through the gate would require editing `internal/run/`, which this packet
forbids.

### Claim 5 — REFUTED

`validVerifyBudget` (`triage.go:125-131`) TrimSpaces first, then requires a
`~` prefix and a `" min: "` substring. `"~4 min: "` becomes `"~4 min:"`,
which does not contain `" min: "` (the substring needs the space after the
colon), so `RunTriage` already replaces it with `VerifyBudgetExample`
(`triage.go:45-47`). `TestTriageAgent_EmptyVerifyCommandIsReplaced`
(`triage_test.go:205`) locks that replacement and **passed the moment it was
written**.

### Claim 6 — CONFIRMED

The business hunk's defining property — both sides compile and pass their own
tests — was never built or run. `TestFixtureGenerator_ForcesClassRequired`
(`fixture_test.go:31`) asserted only `ClassRequired` and a hunk count of
three.

Fix: `GenerateFixture` writes `go.mod` on the shared base and per-side
`toy_test.go` (`fixture.go:78-130, 237-280`). Test:
`TestFixtureGenerator_BothSidesCompileAndPassOwnTests` (`fixture_test.go:227`)
checks out each feature branch and runs `go test .`. RED:

```
go test on feature A: exit status 1
go: cannot find main module, but found .git/config
```

`toy.go` still has exactly three hunks; the tests live in a separate file.

## Open design questions

Both remain open. No change encoded a risk formula or named a production
triage executor (`design.md:120-123`).
