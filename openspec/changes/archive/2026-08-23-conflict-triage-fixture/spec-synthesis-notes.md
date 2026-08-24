# Spec Synthesis Notes: Conflict Triage Fixture

## Unresolved Contradictions

None. Lens C's Conflicts section is empty, so its optional MODIFIED/RENAME of `One Bounded Candidate` does not override Lens A's ADDED classification of `Two-step close and retry CAS`. Who writes `CandidateSHA`, the mixed-hunk risk formula, and the production triage executor remain open by instruction; all three lenses left them open, so they are not draft-versus-draft contradictions.

## Coverage Gaps

- Skill vs packet on new-domain format: `sdd-spec` wants a FULL spec (`Purpose` + `Requirements`) for new domains, and would place it at `openspec/specs/<capability>/spec.md`. This packet forbids writing the live tree and paraphrases “delta format for every file.” Skill wins on format for new domains; packet execution wins on path. New capabilities are full specs under `openspec/changes/conflict-triage-fixture/specs/`. Existing `reconciliation-approval` is an ADDED delta. Recorded here as format drift, not a missing spine item.
- Skill 650-word cap vs packet 1800-word authored budget: packet execution wins. Authored tree is 942 words; no verbatim MODIFIED blocks.
- Skill Step 5 Engram persistence of the spec artifact and Step 6 return block are superseded by this packet (delta tree, this notes file, `.lucind/result.json`).
- No MODIFIED, REMOVED, or RENAMED entries. Live `reconciliation-approval` guarantees are preserved; C found no conflict that would force a full-block edit. Spine items 6–7 are vacuously satisfied.
- Out of scope, not spine gaps: N-way (>2) reconciliation; Claude stream-decoder recovery; overlap threshold changes; web reconcile POST; production dispatch; mixed-hunk risk formula.

## Dropped Citations

1. **Lens C Conflicts — `internal/worktree/worktree.go:278-292` as candidate SHA registration.** Those lines are `IsLinkedWorktree`. SHA registration is `runReconcileResolve` at `cmd/lucind-ai/cli.go:1445-1485` (usage at `:56`). The SHA-registration attribution is dropped. The primary-root constraint is kept via Lens A's use of the same range.
2. **Lens A/B — `internal/resolve/candidate.go:100-145` as the allowed-paths comparison.** `EnforceAllowedPaths` starts at `:100`; `:100-145` only collects git diffs. `packet.PathInScope` runs at `:161-164`. The range is retargeted, not used as the comparison. The invariant requirement is kept: `ScanConflictMarkers` `:48-95` is complete, and `internal/resolve/candidate_test.go:51-80` exercises `EnforceAllowedPaths`.

All other unique citations in the three manifests, and the live-spec and proposal ranges Lens C cited, resolved and supported their claims. Truncated-but-correct ranges kept: `Service.Approve` `:406-435` (full function `:406-535`); `evaluateOverlapGate` `:687-856` (function continues to the N-way block); `TestCreateRequestFromRequiredOverlapDisplaysExactFields` cited `:52-100` (test starts at `:56`).

Proposal-synthesis dropped citations were not resurrected.

## Requirement Divergence

Lens A's four-requirement set is authoritative. Lens B and Lens C independently named the same four: `Deterministic three-hunk fixture`, `Semantic triage and risk ratchet`, `Two-step close and retry CAS`, `Dual-judge rubric isolation`. Independent convergence.

Lens C classified `One Bounded Candidate` as a potential MODIFIED (and optional rename to `Two-step close and retry CAS`) while also reporting **Conflicts: None**. The provided MODIFIED block is a verbatim copy of `openspec/specs/reconciliation-approval/spec.md:47-60` with no edits. Packet rule: C wins on classification only when it finds a conflict against an ADDED requirement. No conflict, so A's ADDED stands. Renaming would delete the live Sonnet-candidate CAS contract. Not taken.

Lens B scenarios keyed to A's names but not taken into the delta:

- **Concurrent multi-feature resolutions block** (under Two-step close). N-way (>2) reconciliation is out of scope. Code at `internal/run/attempt.go:873-891` already blocks that case; the delta does not specify it.
- **Streaming Claude decoder recovers terminal output** (under Dual-judge). A's requirement is rubric isolation and hunk classification, not executor stream recovery. Implementation detail; omitted.

Lens B's remaining scenarios joined A's statements: fixture ClassRequired + missing base SHA + disjoint dispatch; business ratchet + mechanical resolve + invariant failure; approve/retry CAS + tip drift; distinct hunk grading + uniform-score fail. Isolation (no cross-provider config) is taken from A's Dual-judge statement.

Classifications: four ADDED (three new capabilities, one new requirement on `reconciliation-approval`). Zero MODIFIED / REMOVED / RENAMED. C did not reclassify any ADDED requirement.

Open questions merged and left unanswered: mixed business+mechanical risk formula and thresholds; production triage executor/model; which actor writes `CandidateSHA` (design, not spec). Process fan-out vs `sdd-spec` single-agent skill is packet precedence, not a product question.
