# Spec Synthesis Notes: Agentic Phase Specialist

## Unresolved Contradictions

None. All three assumed-requirements blocks describe the same four behaviors (phase-scoped Acceptance carve-out, compressed Phase Verdict, SDD-phase check gating, Specialist-owned synthesis arbitration). The ADDED-versus-MODIFIED disagreement is classification, resolved by Lens C's live-spec evidence per the packet rule, not an irreconcilable product fork.

## Coverage Gaps

- `sdd-spec` wants `## Purpose` + `## Requirements` for new domains and a 650-word per-artifact budget. This packet requires change-folder deltas, an 1800-word authored tree budget, and (for the new capability) a full spec without ADDED/MODIFIED framing. `phase-verdict-reporting` uses the skill's full-spec headers; the three existing capabilities use delta `## MODIFIED Requirements`. Packet 1800 applied (1187 authored words excluding 11 verbatim live scenarios). Skill 650 not applied per file.
- Live `No Promotion Authority` already forbids Acceptance from invoking Promotion. C listed Hard Rule versus carve-out as conflict 4 but did not copy that live block; Promotion stays human-confirmed inside the modified specialist-dispatch requirement. The live promotion requirement is untouched.
- Proposal open questions with no spec requirement: later CLI bridge so `sdd-*` can invoke `lucind-ai run` without Orchestrator mediation; `--force-checks` versus packet-level exception metadata; Phase Verdict as JSON under `internal/result/` versus structured markdown; whether propose-phase skill text must restate that packet budgets outrank skill length in fan-out (live `Asymmetric Precedence and Compression Ceiling` already states packet-vs-skill precedence and was not in A's set).
- REMOVED/RENAMED: none.

## Dropped Citations

Unique manifest ranges opened: 69. Requirement claims kept: all four of A's statements. Live MODIFIED blocks recovered from `openspec/specs/` and match C's copies scenario-for-scenario (3 / 6 / 2).

Retargeted or stripped (claim kept; citation not used as proof in authored requirement text):

1. **`internal/run/attempt.go:431-435` (Lens B).** Claimed "checking phase and default checkFunc." Range only defaults `checkFunc` to `integrate.Check`. `AttemptStatusChecking` is 415–416; there is no `SDDPhase` read. Seam for the new gate remains this default.
2. **Live Two-Wave parentheticals (stripped while editing the requirement paragraph).** `cmd/lucind-ai/cli.go:121-149` is `loadPacket` plus the start of `run()`, not two-wave protocol (`runDispatch` starts at 181). `plugin/claude-code/skills/lucind-ai/SKILL.md:153-176,184-186` does not resolve: current `SKILL.md` is 66 lines (topology is `references/strategies/fan-out.md:7-25`). `openspec/changes/sdd-fan-out-lens/proposal.md` and `explore.md` are absent from this worktree.
3. **Verbatim live scenario still contains unresolved citations.** Wave-2-before-integrate keeps `explore.md:42` and `SKILL.md:184-186` as copied live text. `internal/run/integrate.go:31-81` does resolve (`Integrate` combines worktrees). The scenario claim is independently supported by `fan-out.md:21`.

No other manifest range failed to support its claim. `accept.go:84-96` loads metadata only inside the authoring-evidence branch, then `120-137` always runs checks — that is current behavior, the seam the gate modifies.

## Requirement Divergence

Lens A's four names are the requirement set. B independently used the same four names and supplied happy/edge/error scenarios for each. C independently used the same four names and mapped three of them onto live requirements that already exist.

**Classification correction (C wins).** A classified all four as ADDED and stated none were modified in-place. C's live inventory conflicts on three existing requirements, so those three are MODIFIED. Name joins (A's title → live heading kept so archive replaces the right block):

- `Specialist Phase Acceptance and Authority Carve-Out` → `Specialist sequencing and canonical artifact generation`
- `SDD Phase-Gated Verification Check Execution` → `Fail-Closed Mechanical Criteria`
- `Specialist-Owned Synthesis Arbitration` → `Two-Wave Planning Fan-Out Protocol`
- `Structured Phase Verdict Reporting` remains ADDED on new `phase-verdict-reporting`

**B content not added as new scenarios.** `Check failure in apply phase rejects acceptance` is already live `Reject scope or check failure`. `Synthesis blocked when lens receipts are missing` is already live `Synthesis blocked while lenses unmerged`. Both preserved verbatim in the MODIFIED blocks.

**Specialist Acceptance authority-versus-execution.** A phrased decision authority (`decide Acceptance` and `direct the Orchestrator to execute`). C phrased independent judgment with the Go adapter as the mechanical tool. B's happy-path WHEN said the Specialist "executes the acceptance protocol" — execution language, corrected in the canonical spec. Canonical text: named `sdd-*` Specialists independently **decide** Acceptance for their own phase's Lanes; the Orchestrator **mechanically** invokes `lucind-ai run` and `lucind-ai accept` on that decision. C's leftover open question on that boundary is answered by this phrasing and is not left open.
