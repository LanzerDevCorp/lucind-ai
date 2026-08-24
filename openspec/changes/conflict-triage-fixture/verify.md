# Verify: Conflict Triage Fixture

**Overall verdict: PASSED (remediated)** — the two confirmed coverage gaps below were closed and the
six carried-forward claims were triaged with evidence; see `remediation.md` and
`## Remediation verified` at the bottom of this file. The original BLOCKED findings are kept below
verbatim as the record the remediation answers.

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

## Remediation verified

`remediation.md` (committed at `fe00a9f`/`22d1aeb`) answers all three items above. Redispatched a
read-only judgment lane (`agy`, packet `verify-conflict-triage-fixture-remediation`) against this
exact candidate to independently audit that response rather than accept it on its own word; its
result envelope (`status: done`, all done-criteria met, all hard stops clear) upheld all six claim
verdicts and both gap closures against their cited code. The lane's own dispatch was demoted to
`blocked` by an unattended Tier-A approval timeout — not a defect in its findings — so no new CAS
attempt was recorded; this section is the orchestrator's own synthesis of its (fully evidenced)
result, following the same convention as the reconciliation above.

I independently re-verified three of the most load-bearing claims myself before accepting the lane's
account:

- **Gap 1 closed for real.** `TestFixturePackets_AreBuildScopeTemplatesNotToyWriters`
  (`fixture_test.go:186-208`) parses `feat_a.md`/`feat_b.md` directly and asserts: `ToyPath` is
  excluded from `allowed_paths`, the first line after the frontmatter opener is a `#` comment naming
  `toy.go`, the body mentions `GenerateFixture`, and `routed_by` no longer describes the packets as
  the toy's dispatch shape. Confirmed against the actual test source, not the remediation summary.
- **Gap 2 closed for real.** `TestFixturePackets_OverlappingBuildScopesRefused` (`fixture_test.go:214-225`)
  drives the real admission function `packet.DisjointAllowedPaths` with B's `allowed_paths` extended
  to overlap A's and asserts `err == nil` is fatal (refusal required), while confirming the shipped
  disjoint pair still passes separately. This is the scenario's actual MUST-fail, not a hand-built
  stand-in.
- **Claim 5 (REFUTED) holds.** `validVerifyBudget` (`triage.go:125-131`) does `strings.TrimSpace`
  before checking for `" min: "`. `"~4 min: "` trims to `"~4 min:"`, which does not contain the
  trailing-space substring, so it already fails validation and gets replaced with
  `VerifyBudgetExample`. `TestTriageAgent_EmptyVerifyCommandIsReplaced` (`triage_test.go:205-245`)
  locks exactly this. The production behavior remediation.md describes was already correct before
  remediation touched this file — the claim was about missing test coverage, not a bug, and the
  verdict is accurate.

### A note on this candidate's ancestry

`feature/conflict-triage-fixture`'s tip (`35609ccd`) merges in roughly a dozen unrelated commits from
`lane-status-observability`. Root cause: the CAS-promotion integration worktree used to branch from
whatever the primary checkout's HEAD happened to be, rather than from the feature's own parent
(`internal/integrate/integrate.go:50`, fixed in commit `9a81aae` on a separate branch — not yet
present in this feature's history since the fix landed after the remediation attempt promoted). The
remediation attempt (`881b1a3e`) is where this struck: its diff against the true parent is exactly
the 8 files/502 insertions `remediation.md` describes, but its ancestry also carries the foreign
commits. This is historical contamination in this branch's git graph, not a defect in
conflict-triage-fixture's content — flagged here so archive does not mistake foreign commits for
part of this change.
