# Spec Synthesis Notes: Skill Provisioning and the SDD Phase Specialist

## Unresolved Contradictions

None. Lens A split the proposal's six requirements across eight capability-scoped names; B and C assumed the six proposal names. That is granularity, not an irreconcilable conflict. Classification disagreements are recorded under Requirement Divergence.

## Coverage Gaps

- Dual delivery via `LUCIND_REQUIRED_SKILLS` is in the proposal and in lens B's Dual delivery scenario. A's `Versioned Contract and Late Target Binding` statement covers body rendering only; the env-channel half is not in the delta.
- A's `Result envelope skills loaded declaration` requires rejecting unexpected envelope properties. No lens wrote that scenario; it is not invented here.
- Lens C flagged four live requirements as colliding whose names A did not put in the requirement set, so they are not in the delta: `Universal Admission and Manual Compatibility`, `Versioned Result Correspondence`, `Universal Pre-Dispatch Packet Admission`, `Exact Acceptance Binding`. After archive those enumerations still omit skills.
- Unresolved proposal/lens questions with no spec requirement: ad-hoc authoring surface; `archive`/`ultrafixer` child skills; budget override in `lucind.yaml`; `lucind.yaml` filename collision; whether stack names may override roots (lens B).
- `sdd-spec` wants new domains as a full spec (`## Purpose` + `## Requirements`) and a 650-word per-artifact budget. This packet required delta format under the change tree and an 1800-word authored tree budget; execution follows the packet. Authored tree: 1311 words excluding verbatim copied live scenarios.

## Dropped Citations

Unique ranges opened: 120 (manifest union). Verified as supporting their claim: 118. Retargeted: 2. Dropped (claim removed from the delta): 0.

1. **Retargeted — `internal/result/result.schema.json:1` (Lens A).** Claim: schema root enforces `additionalProperties: false`. Line 1 is `"$schema"`. The flag is line 4. Claim kept; the line is imprecise.
2. **Retargeted — `openspec/specs/read-only-packet-schema/spec.md:28` (Lens C).** Claim: unrecognized keys remain ignored. Line 28 is `Default Value and Backward Compatibility`; the unknown-key scenario is line 42. Claim kept.

All four of A's live `MODIFIED` blocks exist at the cited headings with the cited scenario counts. C's eight copied blocks match the live files scenario for scenario. `openspec/specs/` has no `skill-derivation`, `skill-root-resolution`, `skill-load-correspondence`, or `phase-specialist-dispatch` directories. `accept.go:275-286` is the decode struct with no `LaneRole`/`RequiredSkills` yet (the seam, not current fields). `requestEnv` (`executor.go:20-39`) currently injects only `LUCIND_READ_ONLY_PATHS`. `packet.go:159-164` copies `sdd_phase`/`fanout_group`/`skill` with no `lane_role`. `run.go:876-904` is `enforceAllowedPaths` demotion to `lane.Deviated`. `authoring.go:14,20-42,44-75` is v1 `Contract json.RawMessage` freeze/decode. `fan-out.md:24` forbids synthesis before all required lens IDs are accepted.

## Requirement Divergence

Lens A's eight names are the requirement set. B and C independently assumed the proposal's six names and mapped onto A's eight as follows.

**Independent convergence.** All three: three-tier derivation with reject-not-trim; fail-closed missing-skill admission; closed `lane_role` set of seven; freeze `required_skills`/`lane_role` inside the v1 contract blob; `run` demotes shortfalls and `accept` re-verifies without a receipt; specialist does not start synthesis until lenses are accepted and merged; no removals or renames. C's live inventory (4 / 6 / 8 / 8 requirements on the four modified capabilities) matches the live files.

**Name joins (B content kept, A's names used).** `Contract extension and rendered delivery` → `Versioned Contract and Late Target Binding` (body-render and v1 hash-stability scenarios). `Closed-set lane_role` → `Extended packet frontmatter parsing`. `Demotion and acceptance correspondence` split onto `Result envelope skills loaded declaration`, `Frozen Authored Candidate Evidence` (demotion), and `Fail-Closed Mechanical Criteria` (accept reject / extra-name tolerate). `Specialist sequencing` → `Specialist sequencing and canonical artifact generation`.

**B content not in the delta.** Dual-delivery env-channel (see Coverage Gaps). `Malformed contract payload rejected` — keyed to B's contract-extension name; live `Exact Acceptance Binding` already rejects integrity mismatch and was not in A's set.

**C extras not in the delta.** C would MODIFY four live requirements A did not name (listed under Coverage Gaps). That is not an ADDED→MODIFIED reclassification: A's four ADDED requirements live on capabilities that do not exist under `openspec/specs/`. C correctly left `Frozen Candidate Verification` untouched. Live telemetry sentence on `Extended packet frontmatter parsing` is preserved; A had omitted it.

**Classification.** No ADDED requirement was reclassified MODIFIED. C's evidence did not refute A's set.
