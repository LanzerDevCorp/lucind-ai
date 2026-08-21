# Tasks: sdd-fan-out-lens

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 120–250 |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

120–250 lines across `SKILL.md`, eight explore/propose templates, and `packet_test.go`. Docs and assertions only; below the 400-line review budget (`delivery_strategy: single-pr`).

## Phase 1: Planning Packet Template Assets

- [ ] 1.0-RED Pin malformed-frontmatter typed errors in `internal/packet/packet_test.go` (`:207` `ErrInvalidAllowedPaths`; `:340` `ErrMissingID` / `ErrMissingExecutor` / `ErrMissingRoutedBy` / `ErrEmptyBody`; `:815` `ErrInvalidLegacyMain`). `packet.Parse` (`packet.go:78-166`) copies the body; it does not execute it.
- [ ] 1.1-RED Write a failing table-driven `packet.Parse` + pairwise `DisjointAllowedPaths` (`disjoint.go:29`) test for explore templates (copy `packet_test.go:518-526`).
- [ ] 1.1 Create `plugin/claude-code/skills/lucind-ai/assets/explore-lens-{a,b,c}-packet-template.md` with disjoint draft paths, slice ownership, and `legacy_main: true`.
- [ ] 1.2 Create `assets/explore-synthesis-packet-template.md` targeting canonical `explore.md` with notes skeleton, compression ceiling, and citation verification.
- [ ] 1.3-RED Write a failing Parse + `DisjointAllowedPaths` test for propose templates (same pattern).
- [ ] 1.3 Create `assets/propose-lens-{a,b,c}-packet-template.md` with disjoint draft paths, slice ownership, and `legacy_main: true`.
- [ ] 1.4 Create `assets/propose-synthesis-packet-template.md` targeting canonical `proposal.md` with notes skeleton, compression ceiling, and citation verification.

## Phase 2: Skill Authoring Contract & Documentation

- [ ] 2.1-RED Extend `TestSkillAssetContract` (`packet_test.go:476`) so the frontmatter table must list `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`.
- [ ] 2.1 Update that table in `plugin/claude-code/skills/lucind-ai/SKILL.md` to document those five keys.
- [ ] 2.2-RED Same seam: fan-out is the planning convention (explore, propose, design, specs, tasks); wave 2 branches from integrated `HEAD`; skill governs schema, packet governs topology/slices/ceilings; canonical ceiling stays strictly below the lens sum.
- [ ] 2.2 Promote multi-lens fan-out in `SKILL.md` to that convention with asymmetric precedence and compression ceilings.
- [ ] 2.3-RED Same seam: orchestrator runs `feature create` before dispatch; lanes do not create or move parent refs; two-tier wave-1 remediation (admission repair vs single-lane re-dispatch).
- [ ] 2.3 Document feature-branch ownership in `SKILL.md` (`feature create` before dispatch; lanes do not create/move parent refs).
- [ ] 2.4 Document two-tier operator remediation in `SKILL.md` (silent admission repair vs execution single-lane re-dispatch).
- [ ] 2.5-RED Same seam: subcommands `serve`, `feature`, `reconcile`, `renew` (keep `split`, `check`) and `run` flags `--approval-timeout`, `--legacy-main`, `--expected-parent-sha` (keep `--timeout`).
- [ ] 2.5 Document those shipped subcommands and `run` flags in `SKILL.md`.

## Phase 3: Contract Test Coverage

- [ ] 3.1-RED Extend the table-driven Parse + `DisjointAllowedPaths` test to existing `assets/design-*-packet-template.md` (already `legacy_main: true` at `design-lens-a-packet-template.md:6`).
- [ ] 3.1 Add the table-driven contract test in `internal/packet/packet_test.go` parsing explore, propose, and design templates with `packet.Parse` and pairwise `DisjointAllowedPaths`.
- [ ] 3.2 Extend `TestSkillAssetContract` in `internal/packet/packet_test.go` with assertions for the five keys, planning fan-out convention, CLI subcommands/flags, and feature-branch ownership.

## Suggested Work Units

Wave 1 (TDD RED): Unit 1. Wave 2 (GREEN): Units 2, 3, 4. Unit 1 precedes 2, 3, and 4. List Wave 2 paths as files, never `assets/` as a prefix — `PathInScope` (`disjoint.go:8-22`) would then collide explore with propose.

| Unit | A tasks | allowed_paths | Executor | Focused test | Runtime harness | Rollback |
|------|---------|---------------|----------|--------------|-----------------|----------|
| 1 Contract tests | 1.0-RED, 1.1-RED, 1.3-RED, 2.1-RED, 2.2-RED, 2.3-RED, 2.5-RED, 3.1-RED, 3.1, 3.2 | `["internal/packet/packet_test.go"]` | `cursor-agent` | `go test ./internal/packet` | N/A: Candidate 1 does not change dispatch runtime | revert `packet_test.go` |
| 2 Explore templates | 1.1, 1.2 | `["plugin/claude-code/skills/lucind-ai/assets/explore-lens-a-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/explore-lens-b-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/explore-lens-c-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/explore-synthesis-packet-template.md"]` | `agy` | `go test ./internal/packet` | N/A: templates are passive text | delete those four files |
| 3 Propose templates | 1.3, 1.4 | `["plugin/claude-code/skills/lucind-ai/assets/propose-lens-a-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/propose-lens-b-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/propose-lens-c-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/propose-synthesis-packet-template.md"]` | `agy` | `go test ./internal/packet` | N/A: templates are passive text | delete those four files |
| 4 Skill docs | 2.1, 2.2, 2.3, 2.4, 2.5 | `["plugin/claude-code/skills/lucind-ai/SKILL.md"]` | `cursor-agent` | `go test ./internal/packet -run TestSkillAssetContract` | N/A: documentation only | revert `SKILL.md` |

## Out of Scope

- `cmd/lucind-ai/*`, `internal/run/*`, `internal/dag/*`
- `lucind-ai fanout` generator; Go word-count enforcement
- Verify dual-dispatch; specs/tasks templates until those partitions stabilize
- Rewriting existing `assets/design-*-packet-template.md` (pin only)
- `apply-dag.yaml` (downstream `lucind-dag` owns it)
