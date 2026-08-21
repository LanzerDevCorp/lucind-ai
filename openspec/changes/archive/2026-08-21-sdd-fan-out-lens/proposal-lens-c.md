# Proposal Lens C — Risk, Rollback & Rejected Alternatives: sdd-fan-out-lens

## Assumed scope

Candidate 1 — Null option: convention and template hardening only (no Go).

## Risks

| Risk | Mechanism | Consequence | Mitigation |
|---|---|---|---|
| Silent admission failure | Orchestrator copies frontmatter omitting target keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha` or `legacy_main: true`) per stale `SKILL.md` table. | Packet rejected at admission with `status: failed` and empty worktree path before executor runs, stalling wave. | Update `SKILL.md` frontmatter table and all `assets/` templates with `legacy_main: true`; add contract tests in `internal/packet/packet_test.go`. |
| Path collision in wave 1 | Operator authors lens packets with overlapping `allowed_paths` or omits paths without `read_only: true`. | Admission rejects batch at `DisjointAllowedPaths` (`cli.go:243`) or `Integrate` fails on git merge conflicts (`integrate.go:62-79`). | Mandate disjoint draft filenames (`proposal-lens-a.md`, etc.) in templates and document wave 1 disjointness check in `SKILL.md`. |
| Feedstock overflow | Lenses produce verbose prose exceeding word ceilings without parse-time binary budget validation. | Feedstock exceeds synthesizer context window and compression target (~3000 words into 1800 words), dropping citations. | Hardcode word limits in lens packet headers and enforce eight-item compression spine in synthesis template. |
| Premature wave 2 dispatch | Operator launches wave 2 synthesizer before wave 1 barrier completes or after failed/deviated lens integration. | Synthesizer branches from stale base tree missing lens drafts, producing hallucinated or partial synthesis. | Document sequential wave barrier protocol in `SKILL.md` requiring verification of wave 1 integrated commit before wave 2 run. |

## Rollback Plan

Reversion requires only a standard `git revert` of the commits modifying `plugin/claude-code/skills/lucind-ai/SKILL.md` and `plugin/claude-code/skills/lucind-ai/assets/`.

- **Binary changes**: None. No Go binaries recompiled or deployed.
- **Schema versions**: No movement. Frontmatter parser (`internal/packet`), envelope schema (`.lucind/result.schema.json`), and DAG schema remain untouched.
- **Ledger & state**: No movement. No ledger migrations, database records, or disk formats altered.
- **Spec versions**: No movement. No specifications in `openspec/specs/` modified, added, or deleted.
- **Additivity**: Fully additive documentation and template adjustment; restoring prior revisions leaves existing running packets and worktrees unaffected.

## Rejected Alternatives

### Candidate 2 — Additive sidecar DAG extension (`read_only` and non-apply fan-out in `internal/dag`)

**Rejected because**: Phase fan-out (explore, propose, design, specs) is a qualitative LLM synthesis workflow requiring disjoint file drafts, not multi-wave topological DAG scheduling or cycle analysis. Modifying `internal/dag/parse.go`, `validate.go`, and `emit.go` to support `read_only` and empty `allowed_paths` adds binary churn and forces modification of `openspec/specs/apply-dag-dispatch/spec.md` without eliminating manual packet authoring.
**Would become right if**: SDD phase fan-out grew to require 3+ interdependent execution waves with non-trivial dynamic file dependencies, or if sidecar YAML files were adopted as the universal dispatch contract across all phases.

### Candidate 3 — Dedicated `lucind-ai fanout` scaffolding command

**Rejected because**: Hardcoding the 3-lens plus synthesizer topology, prompt templates, and model mappings into Go code (`internal/fanout`, `cmd/lucind-ai/cli.go`) rigidifies phase structures. It forecloses ad-hoc lens reconfiguration and lightweight prompt experimentation without recompiling and releasing binary updates.
**Would become right if**: The multi-lens topology stabilized into an immutable, permanently fixed pipeline across all phases, and manual template authoring caused high operator error rates across high-frequency dispatches.

## Review Burden Forecast

- **Estimated diff**: ~120–250 lines changed across `SKILL.md`, `assets/*.md` templates, and `internal/packet/packet_test.go`.
- **Session budget**: Consumes <5% of the 5,000-line session budget.
- **Complexity**: Low risk, documentation and test assertion updates only.

## Open questions left to design (not decided here)

- [ ] Should packet templates under `assets/` generalize across all fan-out phases (explore, propose, design, specs) or remain design-phase-specific?
- [ ] What specific test assertions should be added to `internal/packet/packet_test.go` to ensure `SKILL.md` frontmatter tables and `assets/` templates never rot against `internal/packet/packet.go`?
- [ ] Should the synthesis notes template (`design-synthesis-notes.md`) remain unstructured four-section Markdown or adopt a machine-parseable frontmatter summary?
- [ ] What failure-recovery protocol should `SKILL.md` prescribe if one of the three parallel lens lanes fails admission or execution during wave 1?

## Open Questions

- [ ] None
