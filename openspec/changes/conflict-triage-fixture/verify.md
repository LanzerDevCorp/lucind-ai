# Verify: Conflict Triage Fixture

**Overall verdict: BLOCKED** — two confirmed coverage gaps, no confirmed behavioral defect.

Stage 1 (mechanical) passed: `lucind-checks.sh` exit 0 in 51.3s at `ab478b7`, transcript at
`verify-mechanical.log`. Stage 2 dispatched two read-only judgment lanes against feature
`conflict-triage-fixture` (`run 021c89c6`, promoted by CAS).

## Reconciliation

The two lanes disagreed, and the disagreement is the finding.

`agy` reported every requirement and scenario "fully covered by implementation and verified by
targeted unit and integration tests". `cursor-agent` returned nine cited findings against the same
tree. **A blanket all-clear against nine specific citations is not a second opinion; it is an
absent one.** The lanes were not weighted identically — `cursor-agent` was told to emphasize edge
cases and test quality — but emphasis orders effort, it does not license the other lane to miss a
scenario that has no test at all.

I verified the two most consequential findings myself rather than taking either lane's word.

### CONFIRMED — the shipped fixture packets cannot write the file they must collide on

`ToyPath` is `"toy.go"` at the repository root (`internal/conflicttriage/fixture/fixture.go:17-18`),
and `GenerateFixture` writes it there three times to build the base and the two conflicting sides
(`:79,99,116`). The two feature packets grant `["fixture/feat-a/"]` and `["fixture/feat-b/"]`
(`packets/feat_a.md:9`, `packets/feat_b.md:9`).

`toy.go` is outside both. A lane dispatched from either packet cannot write it, and the binary
demotes a `done` envelope to `deviated` when the worktree diff leaves `allowed_paths`.

This is a coherence gap, not a spec violation: the generator produces the conflict directly through
git and does not need the packets to do it. But the two halves of the fixture describe different
worlds, and nothing in the tree reconciles them. Either the packets are the dispatch shape the
fixture demonstrates — in which case they are wrong — or they are build-scope templates unrelated
to the toy, in which case that must be said where a reader will find it.

### CONFIRMED — the refusal scenario the spec states has no test

`specs/conflict-fixture/spec.md:24-30` requires: GIVEN two build features whose allowed paths are
**not** prefix-disjoint, WHEN admitted in a single batch, THEN admission MUST fail.

`TestFixturePackets_DisjointAndValidParentRef` (`fixture_test.go:122-155`) asserts only the positive
direction — that the four shipped packets *are* disjoint (`:153-154`). Nothing anywhere asserts that
a non-disjoint pair is refused. The scenario's actual requirement, the MUST fail, is unproven.

## Findings carried forward as claims, not verdicts

Reported by `cursor-agent` with citations; I did not independently confirm these, and they should be
checked before they are acted on or dismissed:

1. `RunTriage` pins business hunks to ARBITRARY/`high` but never enforces the deterministic
   mechanical resolution the prompt asks for — the invoker's answer passes through unchecked
   (`triage.go:112-123,133-138`).
2. `EvaluateRubric` grades canned `TriagePayload` JSON from stub binaries and never presents
   `GenerateFixture` evidence to a judge (`rubric.go:46-47,75-80,112-138`). The executor-argv
   isolation check is real and is called the strongest rubric test.
3. Invariant coverage through `RunTriage` is markers-only; out-of-scope edits are locked on the
   helpers but not through `RunTriage` itself (`triage.go:63-74,98-109`).
4. The missing/divergent-base skip is proven at `overlap.Evaluate`, not through
   `evaluateOverlapGate` (`fixture_test.go:88-120` vs `attempt.go:738-747`).
5. `validVerifyBudget` checks only a `~` prefix and a `" min: "` substring, so `"~4 min: "` with an
   empty command would pass (`triage.go:45-47`).
6. The business hunk's defining property — both sides compile and pass their own tests — is never
   built or run; the toy is asserted only for `ClassRequired` and a hunk count of three
   (`fixture_test.go:56-85`).

## Confirmed sound

- **Tasks 2.1 and 3.4 lock real terminal behavior.** The apply lane claimed these were regression
  tests that passed without production edits; `cursor-agent` checked and agrees, with evidence:
  `e949c95` touched no `candidate.go`, the marker test asserts a script was *not* executed via an
  absent sentinel, and the path tests chdir away to prove `git -C` is explicit.
- **Both open design questions are still open.** No task encoded a risk formula or named a
  production triage executor. Confirmed by both lanes independently.
- **The accepted deviation was respected.** Neither lane reported the single-feature delivery
  topology as a defect.

## Remediation

1. Reconcile the fixture packets with `ToyPath`, or state plainly in the packets what they are for.
2. Add the negative admission test the `conflict-fixture` refusal scenario requires.
3. Triage findings 1-6 above: confirm or refute each with evidence before archive.
