# Synthesis Notes: sdd-fan-out-lens tasks

## Unresolved Contradictions

`~/.claude/skills/sdd-tasks/SKILL.md` Rules size budget is 530 words. This packet sets 1200 and forbids dropping a task, a guard line, or a threat-matrix obligation. The skill does not win on the budget (packet execution rule). Canonical `tasks.md` follows the packet cap and keeps every A production task, every C RED (including the Documentation-like-paths row), and the four guard lines.

No two lens drafts assert incompatible things that the code does not settle.

## Partition Gaps

None. Lens B’s four units re-key onto Lens A as: Unit 1 → 1.0-RED, 1.1-RED, 1.3-RED, 2.1-RED, 2.2-RED, 2.3-RED, 2.5-RED, 3.1-RED, 3.1, 3.2 (`packet_test.go`); Unit 2 → 1.1, 1.2 (explore templates); Unit 3 → 1.3, 1.4 (propose templates); Unit 4 → 2.1–2.5 (`SKILL.md`). A’s Unit 1 (all of `assets/`) is B Units 2 and 3 split by filename so Wave 2 stays disjoint.

## Test Pairing Gaps

None. Strict TDD pairing used:

| A production | Preceding RED |
|---|---|
| 1.1, 1.2 | 1.1-RED (explore Parse + `DisjointAllowedPaths`) |
| 1.3, 1.4 | 1.3-RED (propose Parse + `DisjointAllowedPaths`) |
| 2.1 | 2.1-RED (five feature-target keys) |
| 2.2 | 2.2-RED (protocol + asymmetric precedence + compression) |
| 2.3, 2.4 | 2.3-RED (feature ownership + two-tier wave-1 remediation) |
| 2.5 | 2.5-RED (CLI subcommands and `run` flags) |
| 3.1 | 3.1-RED (existing design templates) plus 1.1-RED / 1.3-RED |
| 3.2 | 2.1-RED, 2.2-RED, 2.3-RED, 2.5-RED (same `TestSkillAssetContract` seam) |

1.0-RED is the applicable threat-matrix row (Documentation-like paths). N/A rows (git selection, commit, push, PR) generated no task. 2.4 has no separate C row; C combined it with 2.3.

## Dropped Citations

- Lens B: `internal/dag/waves.go:68` as `ValidateGlobalOverlap` “enforces global acyclic ordering for overlapping scopes.” Line 68 calls `ValidateGlobalOverlap` after Kahn waves. That function is `internal/dag/overlap.go:54-79` and returns `ErrUnorderedOverlap` when overlapping paths have no reaches edge. Acyclicity is `ErrCycleDetected` at `waves.go:52`. Not carried into `tasks.md` (`internal/dag/*` is out of scope).
- Lens C threat-matrix: “empty worktree path and failing admission closed before dispatch” hung on `packet_test.go:207`, `:340`, `:815`. Those tests assert Parse typed errors (and an empty `Packet` on error at `:325-327`), not run-admission. Empty worktree is `validatePacketAdmission` in `internal/run/run.go` (out of scope). Canonical 1.0-RED keeps the typed-error seams only.

All other `file:line` cites in the three drafts opened as claimed: `disjoint.go:8-22` (`PathInScope`), `:29-48` (`DisjointAllowedPaths`), `:41-42` (overlap `fmt.Errorf`); `cli.go:243` (disjointness before `worktree.Create`); `packet.go:78` (`Parse`); `packet_test.go:476` (`TestSkillAssetContract`), `:518` (`TestVerifyPacketTemplateAssetStructure`, verify template only), `:207`, `:340`, `:815`.

## Surface Divergence

- **Units / waves.** A: three units, templates ∥ docs then tests (GREEN-order: tests parse files from disk). B: four units, tests Wave 1 then templates ∥ docs Wave 2. C: no units; RED-before-production. Packet TDD + B’s paths win for dispatch; A’s numbers stay. Independent convergence: Candidate 1, same three files, eight new explore/propose templates, design templates pinned not rewritten, new tests only in `packet_test.go`.
- **CLI delta vs full surface.** A 2.5 documents the missing subcommands/flags (`serve`/`feature`/`reconcile`/`renew`, `--approval-timeout`/`--legacy-main`/`--expected-parent-sha`). C’s CLI RED also asserts already-documented `split`/`check`/`--timeout` (`SKILL.md:288-298`; binary at `cli.go:100-111,133-137`). Canonical RED asserts the full surface; 2.5 still adds only the missing rows.
- **Work-unit placement.** Skill template nests work units under the forecast. Packet assembly order is forecast → phases → work units → out of scope. Canonical follows the packet.
- **A open question.** A deferred forecast and RED tasks to C and DAG/executor to B. Synthesis folded them; no remaining open question.

## Disjointness Verdict

Holds. Wave 1 is a single unit (`packet_test.go` only). Wave 2 paths are pairwise disjoint under `PathInScope` (`disjoint.go:8-22`): four distinct `explore-*.md` files vs four distinct `propose-*.md` files vs `SKILL.md`. None equals another; none is a component-boundary prefix of another (`SKILL.md` is a sibling of `assets/`, not a prefix of it). No same-wave path collision. Using directory `plugin/claude-code/skills/lucind-ai/assets/` for Unit 2 would prefix Unit 3 and reject the batch (`DisjointAllowedPaths` at `disjoint.go:29-48`; `cli.go:243`); the assembled list does not do that.
