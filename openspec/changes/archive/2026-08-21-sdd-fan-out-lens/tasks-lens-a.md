# Tasks Lens A — Decomposition & Order: sdd-fan-out-lens

## Assumed work surface

Touches three surfaces without Go binary modifications: `plugin/claude-code/skills/lucind-ai/SKILL.md` (authoring contract & CLI docs), `plugin/claude-code/skills/lucind-ai/assets/` (explore and propose packet templates), and `internal/packet/packet_test.go` (contract tests).

## Phase 1: Planning Packet Template Assets

- [ ] 1.1 Create explore lens templates `assets/explore-lens-{a,b,c}-packet-template.md` with disjoint draft paths, slice ownership, and `legacy_main: true`.
- [ ] 1.2 Create explore synthesis template `assets/explore-synthesis-packet-template.md` targeting canonical `explore.md` with notes skeleton, compression ceiling, and citation verification.
- [ ] 1.3 Create propose lens templates `assets/propose-lens-{a,b,c}-packet-template.md` with disjoint draft paths, slice ownership, and `legacy_main: true`.
- [ ] 1.4 Create propose synthesis template `assets/propose-synthesis-packet-template.md` targeting canonical `proposal.md` with notes skeleton, compression ceiling, and citation verification.

## Phase 2: Skill Authoring Contract & Documentation

- [ ] 2.1 Update the frontmatter table in `plugin/claude-code/skills/lucind-ai/SKILL.md` to document the five feature keys: `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`.
- [ ] 2.2 Update `plugin/claude-code/skills/lucind-ai/SKILL.md` to promote multi-lens fan-out to the planning convention (explore, propose, design, specs, tasks) with asymmetric precedence and compression ceilings.
- [ ] 2.3 Document feature-branch ownership in `plugin/claude-code/skills/lucind-ai/SKILL.md` (`feature create` before dispatch; lanes do not create/move parent refs).
- [ ] 2.4 Document two-tier operator remediation in `plugin/claude-code/skills/lucind-ai/SKILL.md` (silent admission repair vs execution single-lane re-dispatch).
- [ ] 2.5 Document shipped CLI subcommands (`serve`, `feature`, `reconcile`, `renew`) and `run` flags (`--approval-timeout`, `--legacy-main`, `--expected-parent-sha`) in `plugin/claude-code/skills/lucind-ai/SKILL.md`.

## Phase 3: Contract Test Coverage

- [ ] 3.1 Add table-driven contract test in `internal/packet/packet_test.go` parsing explore, propose, and design template assets with `packet.Parse` and asserting `legacy_main: true` and pairwise `DisjointAllowedPaths`.
- [ ] 3.2 Extend `TestSkillAssetContract` in `internal/packet/packet_test.go` with assertions for five feature keys, planning fan-out convention, CLI subcommands/flags, and feature-branch ownership.

## Ordering constraints

| This must precede this | Because |
|---|---|
| Phase 1 (Templates) must precede Phase 3 (Tests) | Contract tests in Phase 3 parse template files from disk and fail if templates are missing. |
| Phase 2 (Skill Docs) must precede Phase 3 (Tests) | `TestSkillAssetContract` asserts strings/tables in `SKILL.md` and fails against un-updated docs. |
| Phase 1 (Templates) and Phase 2 (Skill Docs) can run in parallel | Templates under `assets/` and documentation in `SKILL.md` are disjoint files with no authoring dependency. |

## Suggested work units

| Unit | Tasks | Commit boundary |
|---|---|---|
| Unit 1: Planning Templates | Tasks 1.1, 1.2, 1.3, 1.4 | `plugin/claude-code/skills/lucind-ai/assets/` |
| Unit 2: Skill Contract & Docs | Tasks 2.1, 2.2, 2.3, 2.4, 2.5 | `plugin/claude-code/skills/lucind-ai/SKILL.md` |
| Unit 3: Contract Test Suite | Tasks 3.1, 3.2 | `internal/packet/packet_test.go` |

## Traces

| Task | Spec requirement or design decision |
|---|---|
| 1.1 | Spec: Planning Fan-Out Template Assets; Design: Phase-specific template files |
| 1.2 | Spec: Two-Wave Planning Fan-Out Protocol; Design: Phase-specific template files & Sectioned Markdown synthesis notes |
| 1.3 | Spec: Planning Fan-Out Template Assets; Design: Phase-specific template files |
| 1.4 | Spec: Two-Wave Planning Fan-Out Protocol; Design: Phase-specific template files & Sectioned Markdown synthesis notes |
| 2.1 | Spec: Frontmatter Admission and CLI Documentation; Design: Frontmatter table delta |
| 2.2 | Spec: Asymmetric Precedence and Compression Ceiling; Design: Asymmetric precedence and editorial compression |
| 2.3 | Spec: Frontmatter Admission and CLI Documentation; Design: Orchestrator owns the feature lifecycle |
| 2.4 | Spec: Frontmatter Admission and CLI Documentation; Design: Two-tier operator remediation for wave-1 failure |
| 2.5 | Spec: Frontmatter Admission and CLI Documentation; Design: CLI subcommands and run flags deltas |
| 3.1 | Spec: Skill and Asset Contract Tests; Design: Substring contract tests, not a Markdown AST |
| 3.2 | Spec: Skill and Asset Contract Tests; Design: Substring contract tests, not a Markdown AST |

## Open Questions

- [ ] `sdd-tasks` standard schema prescribes inline review forecast and RED test tasks in `tasks.md`; this packet overrides that schema for Lens A, assigning forecast and RED test tasks to sibling Lens C and DAG/executor assignments to Lens B.
