# Synthesis Notes: sdd-fan-out-lens

## Unresolved Contradictions

None

## Coverage Gaps

- `~/.claude/skills/sdd-design/SKILL.md` Rules size budget is 800 words. This packet sets 1800; archived designs in this worktree run 764–2911 words. The skill does not win on the budget (packet execution rule).
- Skill template names the flow section `## Data Flow` and rollback `## Migration / Rollout`, and requires `## Interfaces / Contracts`. Canonical uses this repository’s spine names (`## Flow and Invariants`, `## Rollback and Additivity`) and keeps a compact `## Interfaces / Contracts` sourced from lens B’s surface deltas. Decision shape (choice / alternatives / rationale) and threat-matrix `Applicable` or `N/A: reason` follow the skill.

No packet spine item was missing from the drafts.

## Dropped Citations

- Lens A: `TestPlanningFanOutTemplateAssets` and `packet.Parse` at `internal/packet/packet_test.go:476`. Line 476 is `TestSkillAssetContract` (`os.ReadFile` + `strings.Contains` on `SKILL.md`). No test with that name exists. Canonical: extend `TestSkillAssetContract`; add a new Parse/disjoint test for planning templates.
- Lens A: `internal/packet/packet_test.go:476-594` as already guaranteeing template assets remain valid. Lines 518-594 are `TestVerifyPacketTemplateAssetStructure` for `assets/verify-packet-template.md` only. Recast as the Parse-from-file pattern (`:518-526`), not planning-fan-out coverage.
- Lens A: `internal/run/run.go:515-543` as the site of (rejected) Go word-count validation. That span is `decideStatus` (timeout / exit / envelope). Word-count-in-Go remains out of scope via `proposal.md:28`.
- Lens A: `cmd/lucind-ai/cli.go:686-700` as creating git refs `refs/heads/feature/<id>`. Line 686 is the comment; `runFeatureCreate` starts at 687 and calls ledger `feature.Service.Create` at `:744-745`. Canonical keeps orchestrator ownership and cites `:687-753` without claiming git-ref creation.
- Lens A: `openspec/changes/sdd-fan-out-lens/proposal.md:18-19` as “explore, propose, and design partitions empirically proven.” Those lines are in-scope bullets (promote the convention; add templates). They do not record empirical proof.
- Lens A: `SKILL.md:218-235` as already applying to every `~/.claude/skills/sdd-*`. Those lines name `sdd-design` only. Canonical: generalize that rule.
- Lens B: `SKILL.md:34-63` as today’s feature-branch ownership docs. Those lines are “Where to author packet files” and executor preference by SDD phase. Ownership copy is new.
- Lens B: `ErrOverlappingAllowedPaths` at `internal/packet/disjoint.go:41`. That line returns `fmt.Errorf("packet: overlapping allowed_paths between …")`. No sentinel with that name. Overlap still aborts before `worktree.Create` (`cli.go:243`).
- Lens B: `TestPlanningPacketTemplates` at `packet_test.go:518-594`. No such name. Same verify-template function as above.
- Lens B: `plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:1-157`. File has 143 lines (OOR end). Frontmatter at `:6` already has `legacy_main: true`.
- Lens B: `lucind-checks.sh:2-10`. File has 5 lines. `go test ./...` is line 4.
- Lens C: `internal/packet/packet.go:122-130` as admission fail-closed. Those lines parse the `legacy_main` boolean. Admission is `validatePacketAdmission` (`internal/run/run.go:250-265`).
- Lens C: `cmd/lucind-ai/cli.go:167-174` as packet admission. That block requires `--expected-parent-sha` when `--legacy-main` is set and frontmatter SHA is empty. Not `ErrMissingFeatureTarget`.
- Lens C: `cmd/lucind-ai/cli.go:548-599` as `depsFactory`. That span is `productionDeps`. `var depsFactory = productionDeps` is at `cli.go:50-52`. Dropped from this change’s test-seam list because new tests stay in `packet_test.go`.
- Lens C: `packet_test.go:737-887` as verifying `lucind-ai run` admission rejection. Those tests are `TestParseFeatureTargetFrontmatter` (`:737`) and `TestParseLegacyMainFrontmatter` (`:815`) — parser round-trips, not run-admission.

## Architecture Divergence

All three assumed Candidate 1 (null option: `SKILL.md` + `assets/` templates + `packet_test.go`; no `cmd/lucind-ai/*`, `internal/run/*`, or `internal/dag/*` runtime change). Independent convergence.

Content that did not survive Lens A:

- Lens B’s file-change table marked the four existing design templates **Modify** (“align with target frontmatter”). Lens A (and C) treat those files as the pattern to copy, already carrying `legacy_main: true`. Canonical leaves them in place and lets Parse/disjoint tests pin them. No design-template rewrite.
- Lens C’s testing table added a unit that drives `lucind-ai run` admission rejection, and listed `runDispatch` / `depsFactory` as required seams. Lens A confines new tests to `internal/packet/packet_test.go`. Existing parse tests stay; no new CLI/`internal/run` tests.
- Lens C’s open questions restated the two proposal-deferred items (shared vs phase-specific templates; operator copy on wave-1 failure) with recommendations that match Lens A. They are Decisions 1 and 2, not open questions.

Lens B’s flow invariants (admission → disjointness → isolation → integrate → synthesize → compress → cite) and Lens C’s threat matrix / rollback match Lens A’s architecture and are in `design.md`.
