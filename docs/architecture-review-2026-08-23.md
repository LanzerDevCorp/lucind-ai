# Architecture review — internal/run, internal/integrate

2026-08-23. Scope chosen from git history: uncommitted work and recent commits
concentrate in `internal/run` and `internal/integrate` (the lane-status-observability
feature). No `CONTEXT.md` or `docs/adr/` exist yet, so nothing below contradicts a
recorded decision.

See `architecture-review-2026-08-23.html` for the visual report (before/after
diagrams, recommendation strengths). This file is the raw exploration findings
it was built from.

## Friction points

### 1. `Deps` is a god-interface, and its size is why the real git/CAS wiring is never tested through orchestration

`internal/run/run.go:155-229`. `Deps` has ~25 function-valued fields
(`CreateWorktree`, `CombineTree`, `RunChecks`, `PromoteTarget`, `PromoteCAS`,
`ResolveRefSHA`, `ResolveCandidateSHA`, `EvaluateOverlap`, `PersistEnvelope`,
`HasUniqueLaneCommits`, `PorcelainEmpty`, …) — the "interface" to package `run`
is effectively every side effect the whole system performs, hidden behind
nothing. Consequence, verified directly: every test that exercises `Integrate`,
`driveAttemptFromLeased`, or the overlap gate (`internal/run/integrate_test.go`,
`attempt_test.go`, `gate_test.go`, and even the CLI-level
`cmd/lucind-ai/cli_test.go:3529-3574` via `featureDispatchDeps`) fakes
`CombineTree`, `RunChecks`, and `PromoteCAS` completely. The only place they're
wired to the real `internal/integrate.Combine/Check/PromoteTarget/PromoteCAS` is
`cmd/lucind-ai/cli.go:664-667`, and no test exercises that wiring —
`internal/integrate`'s real git-merge/CAS logic is unit-tested in isolation
(with real repos in `t.TempDir()`), and `internal/run`'s bisection/lease/gate
logic is unit-tested in isolation (with fakes), but the *composition* — real
bisection driving real `git merge --no-ff` and real CAS — has zero coverage.
Deletion test: deleting `Deps` and hard-wiring the real functions would force
exactly the composed test that's currently missing to exist. Confidence: **Strong**.

### 2. Two independent "what happens when integration fails" implementations, selected implicitly by whether a batch is legacy or feature-targeted

`internal/run/integrate.go:31-184` (`Integrate`, ff-merge + recursive bisection
`bisect`/`tryCombine` at 187-276, `revertLanes` at 278-301) vs.
`internal/run/integrate_feature.go:100-140` (`IntegrateFeature`) which drives
`attempt.go`'s `ExecuteAttempt`/`driveAttemptFromLeased`/`performCASPromotion`
(CAS promotion, feature lease, overlap gate, **no bisection** — "Integrate's
clean-subset isolation has no equivalent here" per its own doc comment). Both
ultimately call the same `internal/integrate` git primitives, but each owns
separate revert/failure semantics, separate `IntegrateReport` construction,
separate discard/cleanup ordering. Understanding "what happens when integration
fails" requires first knowing which of the two paths you're in, then reading 3
files (`integrate.go`, `integrate_feature.go`, `attempt.go`) plus
`internal/integrate/integrate.go` for the git mechanics underneath either path.
Confidence: **Strong**.

### 3. `ledger.Ledger.DB()` leaks the raw `*sql.DB`, making `internal/run/attempt.go` a second owner of the `integration_attempts` schema

`internal/ledger/ledger.go:817` (`DB()`) vs. `internal/run/attempt.go:122-211`
(`getAttempt`, `getAttemptByIdempotencyKey`, `insertAttemptWithAudit`,
`updateAttemptWithAudit`) which hand-write `SELECT`/`INSERT`/`UPDATE ...
integration_attempts` directly. `ledger`'s real interface here is "here is the
whole database" — it hides nothing about that table, so `run` ends up
re-implementing CRUD and column mapping that arguably belongs behind a
`ledger`-owned API (the way `SetStatus`/`AppendEvent`/`RegisterLane` already are
for lanes). A schema change to `integration_attempts` has to be made correctly
in a package that isn't `ledger`. Confidence: **Strong**.

### 4. `evaluateOverlapGate` is a ~210-line unexported function that can only be tested by driving the entire attempt state machine

`internal/run/attempt.go:687-894`. It queries active features, resolves
per-feature SHAs, calls `overlap.Evaluate`, looks up/creates `reconcile`
requests, and handles the N-way-conflict case, all in one function with deep
nesting (three-way switch inside a loop inside a switch). It's unexported and
`gate_test.go` lives in the external `run_test` package, so testing "what does
the lane gate check" means constructing a full `ExecuteAttempt`/`RecoverAttempt`
call with a real ledger and real `feature`/`reconcile` services (confirmed in
`gate_test.go:41-117` — only `EvaluateOverlap` and the git-ref resolvers are
spied; ledger and `reconcile.Service` are real). High fan-out (3 packages:
`overlap`, `reconcile`, `ledger`) behind one function that can't be reached
directly. Confidence: **Worth exploring**.

### 5. `internal/integrate/candidate.go`'s `ResolveAndPromoteCandidate` duplicates the pre-flight/merge/check/re-validate/CAS sequence already implemented independently in `internal/run/attempt.go`

`candidate.go:48-275` vs. `attempt.go:376-570` (`driveAttemptFromLeased` +
`performCASPromotion`). Both: validate expected refs before starting, create a
worktree at a known base, merge/combine untrusted content, run mandatory
checks, re-validate both refs immediately before CAS (to close the same TOCTOU
window), then call `PromoteCASWithRunner`/`PromoteCAS`. These are two
hand-written copies of the same "combine, check, re-validate, CAS-promote,
fail-closed-and-preserve-evidence" protocol — one for feature attempts, one for
reconciliation candidates — with no shared code path. Deletion test: removing
either and trying to reuse the other fails without a rewrite, meaning a
correctness fix to the staleness-recheck (e.g. widening the re-validate window)
has to be found and applied twice by inspection, not by the compiler.
Confidence: **Worth exploring**.

### 6. `bisect`/`tryCombine`'s recursive divide-and-conquer is thoroughly unit-tested only against faked `CombineTree`/`RunChecks`

`internal/run/integrate.go:187-276`. `TestBisect*` in `integrate_test.go:832-960`
prove the *algorithm's* branching logic (which subset gets promoted vs.
reverted) but never exercise it against a real multi-branch merge conflict
through real git, so the actual "isolate the clean subset via bisection"
behavior promised in `Integrate`'s doc comment (line 24-30) is unverified
end-to-end — same class of gap as finding 1, scoped specifically to the legacy
path. Confidence: **Speculative** (real git bisection is plausible to be
correct given the components are separately well-tested, but there's no direct
evidence). Fixing finding 1 (collapsing `Deps`) also closes this one.

## Checked, not flagged

`parseDiffNameStatusZ`/`parseLSFilesZ` (`internal/run/gitpaths.go`) look like
pure helpers extracted purely for testability, but
`TestParseDiffNameStatusZ_EmbeddedNewlineAndRenamePair` (`run_test.go:2570`) and
the `TestExecuteScopeCheck*` family do exercise both the parsing edge cases and
the real git-backed caller (`enforceAllowedPaths`) — this one passes the
deletion test cleanly.

## Top recommendation

Start with finding 1 (collapse `Deps`). It's the root cause behind the biggest
coverage gap in the package — every composed behavior in `internal/run` is
currently proven only against fakes — and fixing it also closes finding 6 for
free. Finding 3 (`ledger.DB()`) is the cheapest independent win and can land in
parallel. Findings 2, 4, and 5 all get easier to see clearly once finding 1
forces the real seams into existence.
