# Synthesis Notes: Agentic Phase Specialist

## Unresolved Contradictions

None. Where drafts disagreed, either the code or Lens A's chosen candidate settled the approach. Those disagreements are recorded under Scope Divergence, not escalated.

Lens A listed Dual-Judge for planning-phase Specialist Acceptance as an open question. Lens B required Dual-Judge for Tier A Specialist Acceptance. Lens C restricted Specialist Acceptance itself to non-Tier-A planning phases. That is not left open: Lens A selected `docs/adr/0002-phase-specialist-authority-and-scoped-checks.md` as the architecture, and that ADR's consequences already require Dual-Judge for Tier A Specialist Acceptance (`docs/adr/0002-phase-specialist-authority-and-scoped-checks.md:18`; `docs/sdd-phase-specialist.md:24`). Current contract text matches (`plugin/claude-code/skills/lucind-ai/references/contracts/acceptance-promotion.md:38-43`). Lens C's stronger restriction (Specialist must not accept Tier A at all) contradicts Lens A's candidate and is under Scope Divergence.

The `sdd-*` Bash/Agent dispatch constraint is not a contradiction. All three lenses and `openspec/changes/agentic-phase-specialist/explore.md` agree: the Orchestrator performs mechanical `lucind-ai run` while the Specialist authors packets and judges Acceptance.

## Coverage Gaps

Packet spine items 1–9 were all covered by at least one draft.

- No lens estimated review burden (changed-line count). `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:56` lists review burden on the propose spine; the synthesis packet's nine-item spine does not. Not invented into `proposal.md`.
- No lens wrote a dedicated Dependencies or Success Criteria section. Those headings in `proposal.md` are compressed from Lens C additivity (no schema migration) and Lens B scenarios, not new claims.
- No lens specified the Phase Verdict wire format (JSON schema vs markdown). Carried as an open question, not specified.

## Dropped Citations

Unique citation ranges opened from the three manifests plus prose-only cites: 72. Verified as supporting their claim: 69. Dropped or retargeted: 3.

1. **Retargeted — `internal/ledger/authoring.go:23` (Lens C).** Claim: `Contract` is `json.RawMessage` allowing additive contract extensions. Line 23 is `PacketDigest string`. `Contract json.RawMessage` is line 26. Claim kept in `proposal.md` as `authoring.go:14,26,44-75`.

2. **Retargeted — `internal/accept/accept.go:172-207` (Lens C).** Claim: enforces candidate commit presence, result existence, and `allowed_paths` containment (and, in Out of Scope, that those mechanisms plus hard-stop demotion must not be altered). Lines 172–196 are `validateTypedTargetBinding` (feature/legacy-main field match). Lines 198–212 are `validateObjects` (commit/tree identity). Result hash/schema and hard-stop/`allowed_paths` checks are `validateResultAndScope` at `accept.go:214-261` (hard stops 225–228, `allowed_paths` 258–260). Claim kept; `proposal.md` cites `accept.go:214-261` and `internal/run/run.go:841-845,856-878`.

3. **Retargeted — `TestSkillDocumentsLanguageGlossary` (Lens C test-impact prose).** Claim: glossary projection test plus `TestSkillTreesByteIdentical` are mandatory gates. No function named `TestSkillDocumentsLanguageGlossary` exists. The CONTEXT.md → `domain.md` projection lives at the end of `TestSkillAssetContract` (`internal/packet/packet_test.go:924-941`). `TestSkillTreesByteIdentical` is `packet_test.go:943-967`. Claim kept; `proposal.md` cites `packet_test.go:924-967` and names `TestSkillAssetContract`.

All other unique ranges resolved and supported their claims, including `accept.go:84-96` (metadata load only inside the authoring-evidence branch, unused for check gating), `accept.go:120-137` (unconditional `CheckPolicySnapshot` / `v.check`), `integrate.go:159-176` (`Check` has no phase parameter), `attempt.go:431-435` (default `integrate.Check`), `lanes_meta.go:25` (`SDDPhase`), `SKILL.md:19` in both trees, `fan-out.md:47-48`, `cli.go:2517-2649`, `phasespec.go:308-333,338-350`, and archived `proposal.md:188`.

Lens C `SKILL.md:21-22` for byte-identity is slightly wide (the sentence is line 21; line 22 is packet templates) but line 21 supports the claim. Kept.

## Scope Divergence

All three independently converged on Lens A's candidate: phase-scoped `sdd-*` Specialist, independent Lane Acceptance without human confirmation, compressed Phase Verdict, Orchestrator-mediated `lucind-ai run`, `sdd_phase == "apply"` check gating, Hard Rule carve-out, human-only Promotion, bounded correction instead of full re-fan-out, and `internal/phasespec.Adapter` retained as a callable tool.

What Lens B or C assumed that differed from Lens A — kept out of `proposal.md` except where noted:

- **Tier A Acceptance (Lens C).** Mitigation "Restrict Specialist Acceptance to non-Tier-A planning phases" contradicts Lens A's independent Acceptance of the phase's Lanes. Cost: that mitigation wording. `proposal.md` keeps Dual-Judge for Tier A Specialist Acceptance (ADR + Lens B) and does not withhold Acceptance from Tier A.
- **`.go` files_changed extra gate (Lens C).** Risk table and open question would run `lucind-checks.sh` if any changed file has a `.go` extension, even for a declared planning phase. Lens A's gate is `SDDPhase`, not file type. Cost: the extra fail-safe clause and C's corresponding open question. Empty/`missing` `sdd_phase` fail-safe from the same C row is compatible with A's unlabeled-legacy concern and was kept.
- **`acceptance-promotion` as a spec capability (Lens B).** No `openspec/specs/acceptance-promotion/` exists. Hard Rule and Acceptance-Subagent edits are skill-contract plus `acceptance-verifier`. Cost: B's capability-table row as a spec name.
- **`--force-checks` as a required CLI flag (Lens B).** ADR already allows an explicit exception. Flag name/shape is an open question, not a requirement.
- **Dual-Judge open question (Lens A).** Closed by the ADR A selected; not re-listed under Open Questions. Lens B independently specified the same rule.

Dogfooding this Change's own remaining planning phases through lucind-ai fan-out+synthesis is a human decision already made; it is in Scope, not an open question, even though explore.md had asked it.
