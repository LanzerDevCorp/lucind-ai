# Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

## Unresolved Contradictions

None. Where drafts disagreed, either the code settled the citation or Lens A's chosen candidate settled the approach. Those disagreements are recorded under Scope Divergence, not escalated.

## Coverage Gaps

- No lens specified how the specialist brackets gentle-ai `sdd-attempt` tokens for runtime-bearing phases (apply, verify, remediate), including the one-attempt-one-worktree constraint. That material lives in explore.md §4.4 A8. It is not invented into `proposal.md`.
- No lens specified whether `sdd_phase` closed-set membership includes `remediate` (Lens B listed it) independently of `lane_role`. Proposal.md follows Lens B's closed `sdd_phase` set only as the conditional check when `lane_role` is present; design should confirm `remediate` belongs.

Spine items 1–9 were all covered by at least one draft.

## Dropped Citations

Unique manifest ranges opened: 63. Verified as supporting their claim: 59. Dropped or retargeted: 4.

1. **Dropped / retargeted — `internal/ledger/authoring.go:23` (Lens B).** Claim: `Contract json.RawMessage` escape hatch. Line 23 is `PacketDigest string`. `Contract json.RawMessage` is line 26. The claim is true; the line is stale (same staleness the orchestrator already corrected from explore.md). Kept in `proposal.md` as `internal/ledger/authoring.go:26`, corroborated by Lens A/C `authoring.go:20-42`.

2. **Dropped claim — `internal/packetauthor/compile.go:49-65` (Lens C).** Claim: `validateContract` enforces contract constraints *and budget*. Lines 49-65 check version, goal/criteria/stops, result path/schema, and route intent. There is no budget arithmetic anywhere in `validateContract` (the function continues through path normalization and forbidden target claims at 66-91). Proposal.md places budget rejection at admission without citing this range as existing budget enforcement.

3. **Dropped claim — `internal/accept/authoring_evidence_test.go:56-127` as schema/struct pin (Lens A).** The test asserts exact correspondence between result, frozen evidence, and contract, and rejects mutations. It does not reflectively pin `result.Envelope` to `result.schema.json`. Proposal.md keeps the citation for a `required_skills` mutation case (Lens C's use) and locates the missing reflection pin at `internal/result/schema_test.go`, whose current tests (lines 10-33) only parse JSON and check a defensive copy.

4. **Retargeted — `internal/skillcontent/skillcontent.go:90-100` (Lens B).** Claim: `HashDir` walks a directory tree for a deterministic SHA-256. Lines 90-100 are the per-file hash writes and the return; the walk starts at line 75. Proposal.md cites `73-100` (Lens C's range), which is the full function.

All other unique ranges resolved and supported their claims, including `accept.go:275-286` (duplicated decode struct), `run.go:876-904` / `882-904` (`enforceAllowedPaths` demotion), `authoring.go:14,20-42,44-75`, `packet.go:122-179`, `executor.go:20-39`, `result.schema.json:5` (`additionalProperties: false`), and `fan-out.md:24`.

## Scope Divergence

All three independently converged on Candidate 1: three-tier derivation, skills inside `Contract json.RawMessage` with no ledger migration, fail-closed admission, two-site enforcement (`run` demotes, `accept` re-verifies), non-intercepting specialist, `skills_loaded` on the envelope, `HashDir` as observation not a gate, and a single PR under the 10000-line exception.

Divergences from Lens A's candidate — kept out of `proposal.md` except where noted:

- **Budget shedding vs reject.** Lens A: oversized sets are rejected at admission, not trimmed. Lens B: excess ad-hoc or stack skills MUST be shed, or admission fails if derived skills alone exceed budget. Explore.md §4.4 A3 is closer to B ("consumes ad-hoc first, then stack") but A is authoritative. Proposal.md uses reject-not-trim.
- **Delivery env var.** Lens A selected dual delivery via a dedicated `LUCIND_REQUIRED_SKILLS` on `requestEnv`. Lens B asked whether to reuse `LUCIND_READ_ONLY_PATHS`. Proposal.md follows A. The delivery-channel open question is therefore closed in the proposal; it is not relisted under Open Questions.
- **`lane_role` closed set.** Lens A listed `{lens, synthesis, apply, verify, archive}`. Lens B added `ultrafixer` and `human`. Proposal.md uses the union of seven because A's own open question treats `ultrafixer` as a role.
- **Schema/struct pin location.** Lens A pointed at `authoring_evidence_test.go`; Lens C pointed at `schema_test.go`. Code matches C. Proposal.md follows C for the pin file; this does not change A's approach.
- **Budget check site.** Lens C named `compile.go:49-65` as the budget seam. Compatible as a proposed extension of `validateContract`, not as current behavior (see Dropped Citations).

Open questions merged from all three, minus delivery channel (resolved by A's approach): ad-hoc authoring surface; archive/ultrafixer child skills; budget default of 3 and whether `lucind.yaml` may override it; specialist CLI/profile granularity; `lucind.yaml` filename collision.
