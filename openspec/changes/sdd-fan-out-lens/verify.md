# Verify: sdd-fan-out-lens

**Overall verdict: BLOCKED**

One confirmed violation of an accepted requirement. Three confirmed non-blocking findings. Mechanical checks passed.

## Stage 1 — Mechanical check

`lucind-ai check` on candidate `6abd702`: **passed**, 22.18s, exit 0. Every package green under `go build ./... && go test ./... -race -count=1`. Frozen transcript: `verify-mechanical.log`. Judgment lanes did not re-run it.

## Stage 2 — Dual qualitative dispatch

| Lane | Status |
|---|---|
| `verify-sdd-fan-out-lens-agy` | done |
| `verify-sdd-fan-out-lens-cursor-agent` | done |

Both integrated cleanly, 0 reverted. Both reported `done`, but their findings diverged materially on the one issue that decides this gate.

## BLOCKING — Templates violate the Planning Fan-Out Template Assets requirement

**Raised by**: `cursor-agent` only. `agy` inspected the same divergence and classified it as stale checklist text, missing that the binding spec — not just `tasks.md` — carries the requirement.

**Independently confirmed by the orchestrator** against the spec text and the assets.

`specs/sdd-planning-fan-out/spec.md:53` states:

> Each MUST parse under `packet.Parse`, **MUST set `legacy_main: true` or the four feature-target fields**, and MUST assign mutually disjoint draft paths for parallel lens lanes.

Its scenario at `:57-60` repeats it:

> THEN parsing MUST succeed, **`Packet.LegacyMain` MUST be true or feature-target fields MUST be set**, and declared draft paths MUST be pairwise disjoint

All eight new templates set neither. Verified: zero occurrences of `legacy_main` across `assets/explore-*.md` and `assets/propose-*.md`.

`tasks.md:25,28` are checked as done while stating the templates were created "with ... `legacy_main: true`". Those two checkboxes are factually false.

### Root cause is orchestrator error, not implementation error

The apply packet instructed the agent to omit every feature-parent field. That instruction came from adopting the `lucind-dag` agent's authoring convention mid-cycle, after the spec was already accepted. The binding contract was never re-checked before the convention changed. The implementing agent followed the packet correctly.

### The implementation is right and the spec is wrong

Remediation is to amend the spec, not the templates. A reusable template that bakes `legacy_main: true` silently targets `main` even on feature-branch SDD work — and the next SDD change is already planned to run in feature mode, so the templates would be wrong from their first use. `SKILL.md:150` already documents `--legacy-main` at dispatch as an equivalent admission path.

### Remediation

1. Amend the `Planning Fan-Out Template Assets` requirement and its `Fan-out templates parse as valid packets` scenario to permit a third path: a template declaring neither, with the legacy declaration supplied at dispatch as `--legacy-main`. Follow the MODIFIED workflow — copy the complete existing requirement with every scenario before editing, or archive silently deletes the untouched ones.
2. Correct the text of `tasks.md:25,28` so the checked boxes describe what was actually built.
3. Re-run verify.

## Non-blocking findings — all independently confirmed

### 1. `## Architecture Divergence` is copy residue in both new synthesizer templates

`explore-synthesis-packet-template.md:62,111` and `propose-synthesis-packet-template.md:61,112` both use the design phase's heading. Explore arbitrates problem boundaries and candidate viability; propose arbitrates a chosen candidate. Neither divergence is architectural. The explore template even hedges inline — "`## Architecture Divergence` (or approach divergence)" — which is the copy showing through.

Raised by `cursor-agent`. Not raised by `agy`, whose finding said all eight templates "strictly mirror" the design models — true, and that is precisely the defect.

### 2. `SKILL.md` generalizes design-specific structure to every planning phase

`SKILL.md:173` describes `## Assumed architecture` as the anchor for all phases, hedged as "(or approach)". `SKILL.md:192` presents the eight-item **design** spine — architecture decisions, flow and invariants, file changes, threat matrix, rollback — as "this repository's actual phase spine" for every phase. Explore and propose have entirely different spines, as their own canonical artifacts in this change demonstrate.

### 3. Contract test depth

- `forbidStrings` is declared in three test tables (`packet_test.go:939,1064,1189`) and iterated (`:1037,:1162`) but left empty, so design-model leftovers such as finding 1 would not fail a test.
- The `SKILL.md` CLI assertions use a bare `strings.Contains(content, cmd)` for `serve`, `feature`, `reconcile`, `renew` — near-tautological given how often those words appear in the document.
- Disjointness is asserted only among the three wave-1 lenses, never as a negative overlapping-path case on these assets.

## Forecast miss — assessed, not a defect

Both lanes independently reached the same conclusion, and the orchestrator agrees: the ~7× line-count miss is a planning error, not scope creep.

`tasks.md:7-19` forecasts 120–250 lines at `400-line budget risk: Low`. Actual: 1616 insertions, 114 deletions across 10 files. Every addition is in scope — eight templates at ~127–162 lines each, 427 lines of contract tests, and the `SKILL.md` changes. No production Go, no extra phases, no generator. It stays inside this session's 5000-line budget.

Against the skill's nominal 400-line rule the risk was High, not Low. Had the preflight budget been 400, a chaining decision would have been required and never requested. Future fan-out template work should forecast ~150 lines per template.

## Unrunnable sidecar — accepted with a caveat

`apply-dag.yaml` and `apply-bodies/` are committed and must never be dispatched. `state.yaml:137-161` explains this fully. Both lanes agreed `state.yaml` is clear; `cursor-agent` correctly noted the sidecar file itself carries no marker, and it is the file `lucind-ai split` actually consumes. A reader starting there can emit and run both wave commands.

Recommended with remediation, not blocking: add a header comment to `apply-dag.yaml` stating it must not be dispatched, and why.

## Where the dual dispatch earned its cost

`agy` and `cursor-agent` examined the same `legacy_main` divergence. `agy` checked it against `tasks.md` and called the checklist stale. `cursor-agent` checked it against `specs/sdd-planning-fan-out/spec.md` and found a live requirement violation.

Same evidence, different reference document, opposite verdicts — and only one of them gates the change. This is the case the dual dispatch exists for.
