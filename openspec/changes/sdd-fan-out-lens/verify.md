# Verify: sdd-fan-out-lens

**Overall verdict: PASSED**

Two rounds. Round 1 blocked on a confirmed requirement violation; round 2 confirmed the remediation was legitimate and surfaced three further defects, all now fixed. Three non-blocking findings remain open as follow-ups.

## Stage 1 — Mechanical check

Final run on `047c5b88`: **passed**, 23.73s, exit 0, every package green under `go build ./... && go test ./... -race -count=1`. Frozen transcript: `verify-mechanical.log`. Judgment lanes did not re-run it.

## Stage 2 — Dual qualitative dispatch, two rounds

| Round | Candidate | agy | cursor-agent |
|---|---|---|---|
| 1 | `6abd702` | done, no blocker raised | done, **blocker raised** |
| 2 | `e08f6e2` | done | done |

All four lanes integrated cleanly, 0 reverted.

## Round 1 — BLOCKED, and where the dual dispatch paid for itself

`specs/sdd-planning-fan-out/spec.md` required every planning template to set `legacy_main: true` or the four feature-target fields. All eight new templates set neither.

Both lanes examined that divergence. `agy` checked it against `tasks.md`, concluded the checklist was stale, and did not block. `cursor-agent` checked it against the binding spec and found a live requirement violation. Same evidence, different reference document, opposite verdicts — and only one of them gates the change. A single-lane verify that drew `agy` would have passed this.

**Root cause was orchestrator error.** The apply packet instructed the agent to omit every feature-parent field, after the `lucind-dag` authoring convention was adopted mid-cycle. The binding spec was never re-checked before the convention changed. The implementing agent followed its packet correctly.

## Remediation — the spec was wrong, not the templates

Amended in `e08f6e2`. A reusable template cannot know where a dispatch lands; baking `legacy_main: true` targets `main` even when the change runs against a named feature parent. The requirement now admits three paths and names the target-less one as the default for a template, with two scenarios pinning the dispatch-time path and the feature-parent reuse it protects.

Both round-2 lanes were asked directly whether this was a legitimate correction or a spec bent to fit an implementation. Both concluded legitimate, on the same grounds: the amendment removed a coupling without weakening the parse, disjointness, or compression constraints. `cursor-agent` — the lane that blocked in round 1 — was explicit: *"a legitimate correction to the already-shipped admission contract, not a spec bent to fit the templates."*

## Round 2 — three further defects, all confirmed and all fixed

Raised by `cursor-agent`; independently confirmed by the orchestrator against `internal/run/run.go` and `cmd/lucind-ai/cli.go`. Fixed in `54a90fe`.

1. **The amendment contradicted its own scenario.** Requirement prose said the orchestrator supplies the target with `--legacy-main` *or* `--expected-parent-sha`; both are required. `validatePacketAdmission` rejects legacy mode without an expected SHA (`run.go:251-253`), and an expected SHA without legacy mode falls through to the four-field branch and fails there (`run.go:261`). The pinning scenario already used both flags.

2. **`SKILL.md`'s fan-out dispatch recipe was broken by this change.** It passed only `--expected-parent-sha`, which worked while templates baked `legacy_main: true`. Against the target-less templates this change ships, following the skill's own recipe fails closed.

3. **The contract tests could not catch a regression of the amendment's distinctive claim.** `TestExplorePacketTemplatesContract` and `TestProposePacketTemplatesContract` parsed, checked ids, executors, paths and substrings, and asserted disjointness — but never asserted the absence of a dispatch target. Reintroducing `legacy_main: true` would have passed every existing assertion.

The fix for 3 was **mutation-verified**, not assumed: adding `legacy_main: true` to one template fails the new assertion with a specific message, removing it passes, and the template ends with no diff. A passing test is not evidence that its assertion fires.

## Non-blocking findings — confirmed, open as follow-ups

1. `## Architecture Divergence` is copy residue in both new synthesizer templates. Explore arbitrates problem boundaries; propose arbitrates a chosen candidate. Neither divergence is architectural, and the explore template hedges it inline as "(or approach divergence)".
2. `SKILL.md:173,192` generalize the design-specific `## Assumed architecture` anchor and the eight-item design spine to every planning phase, which explore's and propose's own canonical artifacts contradict.
3. Contract-test depth: `forbidStrings` declared in three tables and left empty, near-tautological `strings.Contains` CLI assertions, and no negative overlapping-path case.

## Assessed and accepted

**Forecast miss.** `tasks.md` forecast 120–250 lines at `400-line budget risk: Low`; actual was 1616 insertions and 114 deletions across 10 files. Both lanes and the orchestrator independently agreed: planning error, not scope creep. Every addition is in scope, no production Go, no extra phases. Inside this session's 5000-line budget, but High against the skill's nominal 400 — had the preflight been 400, a chaining decision would have been required and never requested.

**Unrunnable sidecar.** `apply-dag.yaml` and `apply-bodies/` are committed and must never be dispatched; `state.yaml` explains why. The sidecar file itself still carries no marker, and it is the file `lucind-ai split` consumes. Open as a follow-up.

**Design templates still bake `legacy_main: true`.** Compliant via path 1, pinned by `TestDesignPacketTemplatesContract`, out of scope to rewrite. They now use a path the amended spec names as not the default. Open as a follow-up.
