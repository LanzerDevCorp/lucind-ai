# Tasks Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

None.

## Coverage Gaps

Waves merged for Integrate viability:

- Lens B Wave 2 (Unit 2 ∥ Unit 4) was not shipped. Under B’s mapping, payload types live in Unit 2 (`internal/conflicttriage/types.go`) while Unit 4’s rubric sits in `internal/conflicttriage/fixture/rubric.go`. Lens A’s 4.1 depends on 1.1: rubric tests consume `TriagePayload`. An isolated Unit 4 worktree on a post-Wave-1 base would not contain `types.go`, so `go test` in that lane fails; Integrate bisection of a failing Unit 2 would then revert Unit 4 as well (`internal/run/integrate.go:50-59`). Strict-TDD RED/GREEN for one unit already belong in one lane. Canonical plan is sequential Units 1 → 2 → 3 → 4 in a single packet. B had already recommended no sidecar; the viability merge is why a DAG is not an alternate dispatch.

- Lens B Wave 1 (Unit 1 ∥ Unit 3) is path-disjoint and would be green alone (additive reconcile persist ∥ fixture generator). It was not dispatched because the sidecar was declined, not because it fails the green-on-its-own test.

Skill vs packet (packet wins on execution; recorded as drift):

- Skill size budget is 530 words; packet budget is 1800. `tasks.md` is 990 words. Skill 530 was not applied.
- Skill “400-line budget risk” field name is kept. The value is judged against the human 2000-line review budget (packet). Against a literal 400-line trigger this change would be High with chained PRs and `Decision needed before apply: Yes` (`single-pr` → `size:exception`). Against 2000 lines, 1100–1700 is Low, chained PRs No, decision No.

Spine fill that no single lens owned, assembled not invented:

- Skill work-unit columns Likely PR, Focused test command, and Runtime harness were absent from lens B; proving commands and N/A harness reasons come from lens C.
- Lens B `allowed_paths` omitted lens A 2.1 (`internal/resolve/candidate_test.go`) and 3.4 (`cmd/lucind-ai/cli_test.go`, `internal/run/attempt_test.go`). Those paths are on Units 2 and 3 in canonical `tasks.md`.
- Live cloud LLM invocation is not in CI (lens C verification gap). Runtime harness is N/A with that reason; apply must not treat unstubbed `claude-opus-5` / `openai/gpt-5.6-sol` network calls as the proving command.

Not a gap: Push state and PR commands stay N/A with no RED tasks. Risk formula and production triage executor stay open (`design.md:118-123`) and are not resolved by any task.

## Dropped Citations

Retargeted:

- Lens A 3.4 placed `TestReconcileResolve_RejectsLinkedWorktree` in `internal/run/attempt_test.go`, citing `worktree.go:278-292` and `cli.go:1478-1481`. Those citations support the behavior, not the file. `IsLinkedWorktree` is called from `runReconcileResolve` (`cli.go:1478-1481`); `evaluateOverlapGate` does not. Sibling CLI test `TestReconcileResolveCLI` is at `cli_test.go:3126`. Canonical task 3.4 writes the linked-worktree RED in `cmd/lucind-ai/cli_test.go`. CAS retry tests stay in `internal/run/attempt_test.go` (`attempt.go:821-828,870`).

Dropped (citation does not support the claim):

- Lens A dependency “3.4 depends on 1.4”. `evaluateOverlapGate` adopts `CandidateSHA` when the other tip still matches (`attempt.go:821-828,870`); it does not read `Candidate.Output`. Output-only persist is not a CAS prerequisite. Canonical 3.4 depends on 3.2 only. 1.3/1.4 remain on the triage requirement, not two-step close.
- Lens A manifest `internal/reconcile/reconcile.go:496-504` as “Approve audit event logging”. That span is the candidate `INSERT`; the audit event is `ledger.IntegrationEvent` at `:508-513`. Claim not used in `tasks.md`.
- Lens A/B `cmd/lucind-ai/cli.go:56` as “subcommand routing”. Line 56 is the usage string that *lists* reconcile verbs; it is not the dispatcher. Listing claim kept as context only; no task cites it as routing.
- Lens C proving command `go test -run ^TestReconcileResolveCLI$ ./cmd/lucind-ai` as proof of CAS retry. `TestReconcileResolveCLI` (`cli_test.go:3126`, subtest ~3199–3228) asserts `reconcile resolve --sha` marks the candidate integrated and stores the SHA. It does not call `evaluateOverlapGate` or `PromoteCASWithRunner`. Kept as SHA-registration evidence only; CAS proof is new `attempt_test.go` tests in 3.4.

Not dropped (judges, not production):

- Lens A 4.2 names `claude`/`claude-opus-5` and `opencode`/`openai/gpt-5.6-sol`. Design pins those as A/B judges (`design.md:37-42`; `claude.go:35-52`; `opencode.go:53-65`). Production triage executor stays open. Canonical 4.2 keeps judge names and forbids naming production.

Verified (unique citations opened in this worktree; not repeated per prose mention): all remaining manifest rows resolved and supported their claims, including `ScanConflictMarkers` / `EnforceAllowedPaths` (`candidate.go:48-145`, NUL skip `:80`, `git -C` `:107-133`), `UpdateCandidateStatus` SQL omitting `output` (`reconcile.go:873-876`), `ledger.UpdateReconciliationCandidate` including `output` (`ledger.go:1314-1338`), `output TEXT NOT NULL DEFAULT ''` (`schema.go:163`), `Classify` → `ClassRequired` (`overlap.go:623-659`), `ErrNoMergeBase` continue (`attempt.go:743-747`), `FeatureTarget` / `ErrMixedFeatureTargets` (`integrate_feature.go:17,26-78`), `PathInScope` / `DisjointAllowedPaths` (`disjoint.go:13-22,29-48`), `IsLinkedWorktree` (`worktree.go:278-292`), `PromoteCASWithRunner` (`integrate.go:151-173`), `Integrate` check-then-bisect (`integrate.go:50-59`), executor stubs (`claude_test.go:18-26`, `opencode_test.go:19-27`), and the four spec files. Planned test names (`TestUpdateCandidateOutputOnly`, `TestTriageAgent_*`, `TestFixtureGenerator_*`, `TestRubric_*`) do not exist yet; they are apply-time proving commands, not failed citations. Package `internal/conflicttriage` does not exist yet; that is expected (create tasks).

## Decomposition Divergence

Lens A (authoritative): four sequential phases — (1) types + invoker + output persist, (2) threat RED + fail-open agent, (3) fixture + disjoint packets + CAS/linked-worktree tests, (4) dual-judge rubric.

Lens B differed: Unit 1 = reconcile persist only; types and invoker bundled with the agent in Unit 2; packets bundled with the rubric in Unit 4; no work-unit home for A 2.1 or A 3.4. Cost: Wave 2 parallel of agent and rubric collides with A 4.1→1.1; extra `types_test.go` / `invoker_test.go` are not in A’s checklist and did not enter `tasks.md`.

Lens C differed: invoker with the agent (C unit 2) rather than A phase 1; packets with the rubric (C unit 4) rather than A 3.3. Extra proving rows (missing `base_sha`, uniform hunk scores, invariant violations) map onto A 3.1, 4.1, and 2.2 and were folded in as assertions, not extra phases.

Independent convergence: all three lenses used the same four capabilities (persist, agent, fixture, rubric) and the same critical path persist → agent and fixture → rubric. A and C independently placed payload types with output persist, against B’s types-with-agent split. That A+C agreement is corroboration; canonical units follow A (types and invoker in Unit 1, packets in Unit 3).
