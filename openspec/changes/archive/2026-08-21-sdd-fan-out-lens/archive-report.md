# Archive Report: sdd-fan-out-lens

**Archived**: 2026-08-21 · **Verdict**: PASSED · **Store**: hybrid

## What shipped

Candidate 1, the null option: harden the authoring contract, not the binary. The three-lens planning fan-out — three parallel `agy` lens lanes writing disjoint drafts, one `cursor-agent` synthesizer — is now the documented convention for planning phases, with templates for explore and propose alongside the existing design set, and contract tests that fail when the skill text or the templates drift from the parser.

No production Go changed. New capability `sdd-planning-fan-out`: 5 requirements, 13 scenarios, merged to `openspec/specs/` with exact parity.

Final commit `047c5b88`. Apply: `2fa46f5`, `842b88e`, `7409e5d`, `709c0dd`. Remediation: `e08f6e2`, `54a90fe`.

## The cycle ran on itself

Every planning phase was executed through the topology it specifies — explore, propose, spec, design, and tasks, each as three lens lanes plus a synthesizer. Twenty lanes, zero reverts, every word budget respected.

The synthesis citation-verification pass caught roughly fifty bad or stale citations across fifteen lens drafts. Nine were caused by a mid-flight commit shifting `SKILL.md` line numbers after lenses had branched, which produced a standing rule: a fan-out in flight forbids committing to files the lenses cite.

The `## Assumed X` anchor produced corroboration in four phases and one real divergence in the fifth. In spec, lens A returned a delta under a new capability while lenses B and C independently read the proposal's `Capabilities: None` literally and assumed no delta. Without the anchor that disagreement would have entered the canonical document invisibly — edge scenarios written for requirements their author believed did not exist, and a traceability matrix auditing coverage against an empty set.

## What the dual-dispatch verify caught that a single lane would not

Both verify lanes examined the same divergence between `tasks.md` and the templates. `agy` checked it against `tasks.md`, called the checklist stale, and did not block. `cursor-agent` checked it against the binding spec and found a live requirement violation.

Same evidence, different reference document, opposite verdicts. A single-lane verify that drew `agy` would have shipped the violation.

## Follow-ups

Nine, none fixed. Grouped by where they live.

### Binary — observability

1. **`mapLaneConstraintError` collapses every CHECK violation into `ErrRoutingConditionMissing`.** A rejected `executor` value reports a missing routing condition. This cost three layers of excavation — worktree, ledger, DDL — to diagnose what one accurate message would have named.
2. **`_ = ensureLaneFailed(...)` in `internal/run/batch.go` discards errors silently.** When the recovery path hits the same constraint that caused the original failure, the lane leaves no trace at all — not even the `lane_registered` event that an admission rejection produces.
3. **A failed lane reports an empty worktree path whether admission rejected it or the executor died.** Two very different failures with an identical signature. The reliable discriminator is whether a `lane_registered` event exists, which is not visible from the lane report.

### Skill and templates

4. **`.lucind/` is gitignored and undocumented as where sidecars lived.** No `apply-dag.yaml` appears in git history, which reads as "never used" when the truth is "used and never committed". `SKILL.md` says they belong under `openspec/changes/<id>/`.
5. **`## Architecture Divergence` is copy residue** in `explore-synthesis-packet-template.md:62,111` and `propose-synthesis-packet-template.md:61,112`. Neither phase arbitrates architecture; the explore template hedges it inline as "(or approach divergence)".
6. **`SKILL.md:173,192` generalize design-specific structure to every planning phase** — the `## Assumed architecture` anchor and the eight-item design spine — which explore's and propose's own canonical artifacts in this change contradict.
7. **The four design templates still bake `legacy_main: true`.** Compliant via path 1 and pinned by `TestDesignPacketTemplatesContract`, but they now use a path the amended spec names as not the default for a reusable template.
8. **`apply-dag.yaml` carries no in-file marker saying it must not be dispatched.** `state.yaml` explains it; the sidecar is the file `lucind-ai split` actually consumes, and a reader starting there can emit and run both wave commands.

### Method

9. **No tasks lens owns "does this partition survive the integration gate?"** The accepted `tasks.md` split the work RED-first across waves. `Integrate` gates every batch on `lucind-checks.sh`, so a wave whose accepted done criterion is that tests fail would be reverted before its successor could turn them green. Lens A ordered dependencies, lens B proved path disjointness, lens C paired RED tests, and the synthesizer re-verified disjointness — none asked the question. The DAG was authored faithfully and abandoned unrun.

Related, same phase: lens C forecast 120–250 lines against an actual 1730, and neither sibling lens nor the synthesizer challenged it. Eight templates at ~150 lines each were visible in the existing design set. Future fan-out template work should forecast ~150 lines per template.

### Also recorded

The explore synthesizer measured the lens partition itself and found it leaks on the sidecar-versus-hand-authored seam — all three lenses covered it. Its proposed tightening is in `explore-synthesis-notes.md` under `## Lens Overlap`.

## Defects found in adjacent work during this cycle

Fixed by the sibling session, not by this change:

- `opencode` dispatch omitted the `run` subcommand, so every opencode dispatch since it landed hit the default TUI command and died at exit 1 without ever reaching a model. Its test asserted `argv[0]` was the prompt — the bug encoded as a passing assertion. Fixed in `503f2f8`.
- The ledger's `lanes.executor` CHECK constraint did not admit `'opencode'`, so every opencode lane died at registration after its worktree was created. Invisible until the argv bug was fixed. Schema v5 in `3c03bb1`.
- The `lucind-dag` agent's mission excluded `.lucind/result.json`, so it could never reach a terminal status regardless of how good its work was. Fixed in the agent definition.

Three defects in layers, each reachable only by fixing the one before it.
