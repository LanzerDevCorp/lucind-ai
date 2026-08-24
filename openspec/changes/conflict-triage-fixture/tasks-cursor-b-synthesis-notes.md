# Tasks Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

None.

## Coverage Gaps

- **Wave 2 merged (viability).** Lens B’s Wave 2 ran Unit 2 (agent, including `types.go`) in parallel with Unit 4 (rubric). Lens A’s 4.1 depends on 1.1: rubric tests consume `TriagePayload`. An isolated Unit 4 worktree on a post-Wave-1 base would not contain `types.go`, so `go test` fails in that lane; Integrate then bisects a failing combined tree (`internal/run/integrate.go:50-59`). Strict-TDD RED/GREEN for one unit already belong in one lane. Canonical plan is sequential Units 1 → 2 → 3 → 4 in a single packet. B had already recommended no sidecar; the merge is why a DAG is not an alternate dispatch.
- **Wave 1 left unused, not merged for failure.** Persist (`internal/reconcile/*`) vs fixture (`internal/conflicttriage/fixture/fixture.go`) is prefix-disjoint (`disjoint.go:13-22`) and would be green alone (additive method + new package). It is not dispatched in parallel because there is no sidecar.
- **Skill contract vs this packet (not filled in).** `~/.claude/skills/sdd-tasks/SKILL.md` requires a 530-word artifact and treats >400 changed lines as High with chained PRs and `Decision needed before apply: Yes` under `single-pr`. The packet sets an 1800-word budget and a human 2000-line single-PR budget. Canonical forecast keeps the skill field *names* (including `400-line budget risk`) and the four guard lines, and evaluates the *value* against 2000 lines (Low, Chained PRs No, Decision needed No, `pending`). That drift is accepted, not silently patched by inventing a chain.
- **B-only test files not invented.** Lens B listed `types_test.go` and `invoker_test.go`. Lens A has no such tasks; coverage is `triage_test.go` via a stub `TriageInvoker`.
- **Live LLM out of CI.** Lens C’s verification gap stands: unit tests use `writeClaudeStub` / `writeOpencodeStub`. No task invented for credentialed cloud runs.
- **Linked-worktree RED placement.** `TestReconcileResolve_RejectsLinkedWorktree` stays in phase 3 (task 3.4) per Lens A, after agent GREEN. CLI production already exists (`cli.go:1478-1481`). The other Git-repository RED (`TestEnforceAllowedPaths_ExplicitWorktreeCwd`) is in 2.1 before 2.3.

## Dropped Citations

Union of the three manifests plus prose-only citations: each unique range opened in this worktree. Kept claims are those that resolved and supported the text in `tasks-cursor-b.md`. Removed or retargeted:

- **Lens B sidecar: `proposal.md:29-37` as the 2000-line review budget.** Those lines are Capabilities (`conflict-triage`, `conflict-fixture`, rubric, `reconciliation-approval`). The proposal has no changed-line forecast. Manifest row “capabilities” is true; the budget attribution is dropped. Budget source is the packet/human 2000-line decision.
- **Lens A 3.4: `TestReconcileResolve_RejectsLinkedWorktree` in `internal/run/attempt_test.go`.** `runReconcileResolve` is `cmd/lucind-ai/cli.go:1445-1511` (refuse at `:1478-1481`). Existing CLI tests are `cli_test.go:3126`. Retargeted to `cmd/lucind-ai/cli_test.go`. CAS retry remains in `attempt_test.go`.
- **Lens C: fixture package command proves `evaluateOverlapGate` (`attempt.go:687`).** `go test ./internal/conflicttriage/fixture` can prove `Classify` → `ClassRequired` (`overlap.go:623-659`). The gate needs `run.Deps`. Dropped the gate claim; Classify stays in 3.1; gate/CAS is 3.4.
- **Lens C: `TestReconcileResolveCLI` (`cli_test.go:3126-3150`) proves CAS retry.** That test registers a SHA (`cli.go:1501-1506`). It does not drive tip-match (`attempt.go:821-828,870`) or `PromoteCASWithRunner` (`integrate.go:151-173`). Dropped the CAS-retry claim. Stub-driven unblock already exists as `TestApprovedIntegratedCandidateUnblocksPromotion` in `gate_test.go`.
- **Lens A manifest: `reconcile.go:496-504` “Approve audit event logging”.** 496–504 is the candidate INSERT; `reconciliation_approved` is 508–513. Not used in the canonical file.
- **Lens A/B: `claude.go:106-122` as stream-json degrade fallback.** `Run` keeps raw stdout when there is no terminal record (`:116-121`). The degraded-timeline string is `claude_stream.go:10-16`. Not used in canonical tasks (judges via stubs).
- **Lens A: `opencode.go:53-65` “command runner”.** Range is `DefaultModel` / `KnownModels` (`openai/gpt-5.6-sol`, luna in-family). `Run` is later. Retargeted to pinned judge model only.
- **Lens A: `cli.go:56` “subcommand routing”.** Line 56 is the `usage` string listing reconcile verbs, not a dispatch switch. Not used as routing.

No citation failed to resolve (file missing or line out of range). Failures above are semantic: the range exists but does not support the claim as written.

## Decomposition Divergence

All three drafts used a four-stage spine: output persist → fail-open agent → 3-hunk fixture → dual-judge rubric. Independent convergence. Lens A’s phasing is canonical.

- **Types / invoker placement.** A: Phase 1 (1.1, 1.2). B: Unit 2 with the agent. C: types with persist (unit 1), invoker with the agent (unit 2). Canonical Unit 1 follows A (types + invoker + persist). B/C content still maps to A’s 1.1–1.4 and 2.2–2.3.
- **Packets.** A: 3.3 after the generator. B and C: Unit 4 with the rubric. Canonical keeps 3.3 (depends on 3.2, not 4.1). Judge filenames `claude_judge.md` / `opencode_judge.md` from B are kept inside 3.3.
- **Threat-matrix tests.** A: 2.1 on `internal/resolve/candidate_test.go`. B’s `allowed_paths` omitted that file. Canonical Unit 2 includes it so 2.1 is in-scope.
- **B extra files.** `types_test.go`, `invoker_test.go` map to no A task — omitted (see Coverage Gaps).
- **C acceptance rows** map onto A’s checklist once the two overclaims above are dropped. No C row lacked an A task.
- **Open questions.** All three left risk formula and production triage executor open (`design.md:118-123`). None smuggled a number or production executor into a task.
