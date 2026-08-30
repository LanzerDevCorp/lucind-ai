---
id: design-skill-anchoring-guardrails-lens-c
executor: agy
routed_by: failure-test-rollback lens of the three-lens design fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/design-lens-c.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: f5a531183361804ed95c797e16a70dbbcca27763
expected_parent_sha: f5a531183361804ed95c797e16a70dbbcca27763
---

# Packet design-skill-anchoring-guardrails-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-skill-anchoring-guardrails-lens-c  ·  **Branch:** lucind/design-skill-anchoring-guardrails-lens-c

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/design-lens-c.md`: how this change is tested, which seams already exist to test it through, the applicability-driven threat matrix, and the rollback/additivity decision.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` is accepted and frozen. Lens A and lens B run in parallel against the same frozen inputs and write to different files.

Lens A owns the architecture decision and is running concurrently, so you do not have it. Declare the architecture you are assuming in `## Assumed architecture` and design against it consistently.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/proposal.md` exists and is accepted.
- `openspec/changes/skill-anchoring-guardrails/design-lens-c.md` does not yet exist.
- The threat-matrix reference table is embedded verbatim in this packet's `## Context`.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill.
2. `openspec/changes/skill-anchoring-guardrails/proposal.md` in full — its Test and Validation Impact table already names the tests to update; verify each and expand with any missing seam.
3. `internal/worktree/worktree_test.go` and `cmd/lucind-ai/cli_test.go` — read how this repository actually tests worktree cleanup and CLI banners: what it asserts on, what it fakes.
4. The threat-matrix table in `## Context` of this packet, and `~/.claude/skills/sdd-design/references/threat-matrix.md` behind it. The embedded copy is the frozen evidence; the reference is the authority. Report any drift.

Never guess at a test seam. A seam you cannot cite does not exist yet, and saying so is the useful answer.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/design-lens-c.md`:

```markdown
# Design Lens C — Failure, Test & Rollback: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed architecture

<Same shape as lens A is expected to assume: force bool parameter + ErrWorktreeDirty sentinel + --force/-f flag + four static banner call sites. State it in your own words from the proposal, independently.>

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|

<Unit (worktree.Cleanup/Remove force behavior), CLI integration (worktree cleanup exit codes/output), banner-content tests for each of the four milestones.>

## Test Seams

<What is injectable/fakeable today for testing dirty-vs-clean worktree state and CLI stdout/stderr capture, and what this change would have to add.>

## Threat Matrix

<The table from `## Context`, every row marked `Applicable` or `N/A: <reason>`. For every applicable row: the expected safe behavior, the expected failure behavior, and the concrete RED test that proves it. This change wraps `git worktree remove --force` and `git status --porcelain` — evaluate "Git repository selection" and "Commit state" rows carefully before marking others N/A.>

## Rollback and Additivity

**Choice**: <what reverting looks like — single git revert per proposal>
**Alternatives considered**: <what other reversal strategy was rejected>
**Rationale**: <why, grounded in what the format deltas actually move>

<State explicitly whether any schema, ledger, or envelope version moves.>

## Out of Scope

<Adjacent work this change explicitly does not do, per the proposal's Out of Scope section.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-c.md` MUST be under 1000 words. Tables over prose. The threat matrix rows count toward the budget — keep the reasons to one clause.

## Out of scope

- **Lens A owns**: the technical approach and every architecture decision except rollback.
- **Lens B owns**: the file-changes table, data-flow diagrams, invariants, and every exact type/schema/CLI signature delta.

Rollback is yours even though it is shaped like an architecture decision.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/design-lens-c.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its `references/`. The skill is authority on *what* a design document must contain, including the threat-matrix applicability rule; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed architecture" --require-section "Testing Strategy" \
  --require-section "Test Seams" --require-section "Threat Matrix" \
  --require-section "Rollback and Additivity" --require-section "Out of Scope" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design-lens-c.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every named test seam carries a `file:line` citation that points at real code in this worktree**, or is explicitly marked "new seam required".
- [ ] **Every threat-matrix row is marked `Applicable` or `N/A` with a reason**, and every applicable row names a planned RED test.
- [ ] **`design-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section including `## Assumed architecture`, plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- A behavior the specs require cannot be tested through any existing or proposed seam.
- Whether a format delta is additive cannot be determined from the specs, so the rollback decision would be a guess.
- The threat matrix is missing from both `## Context` and the skill reference.
- Satisfying one instruction in this packet would require violating another.

## Context

### Threat-matrix reference table

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | Applicable / N/A: reason | Classification and execution boundary | One test per applicable class |
| Git repository selection | `git -C`, relative paths, absolute paths | Applicable / N/A: reason | Repository/cwd authority | One test per applicable selector |
| Commit state | staged, `commit -a`, empty index | Applicable / N/A: reason | Index/worktree semantics | One test per applicable state |
| Push state | tracking branch, first push, explicit refspec | Applicable / N/A: reason | Destination/ref resolution | One test per applicable state |
| PR commands | explicit `--head`, environment prefix, composed commands | Applicable / N/A: reason | Argument composition and ownership | One test per applicable form |

For every applicable row, define the expected safe behavior, failure behavior, and concrete test boundary. If the change has no routing/shell/process boundary, record the matrix as not applicable rather than expanding it.

### Ground truth

Change: **skill-anchoring-guardrails**. Accepted proposal's Test and Validation Impact table names: `TestCleanupRemovesExistingWorktree` (`internal/worktree/worktree_test.go:1034-1057`), `TestCleanupOnLaneWithNoWorktreeIsNoOp` (`internal/worktree/worktree_test.go:1059-1069`), `TestRemove` (`internal/worktree/worktree_test.go:255-266`), `TestWorktreeCleanupCLI` (`cmd/lucind-ai/cli_test.go:2974-3010`), `TestAcceptRequiresExactReceiptAndRendersMechanicalEvidenceOnly` (`cmd/lucind-ai/cli_test.go:4503-4545`), `TestPrintReportOmitsDiagnosisBlockWhenNoneCaptured` (`cmd/lucind-ai/cli_test.go:685-724`), `TestPrintIntegrateReportIncludesIntegratedAndRevertedIDs` (`cmd/lucind-ai/cli_test.go:729-777`), `TestSplit_TwoWaveDAGSuccess` (`internal/dag/split_test.go:13-111`). This change wraps `git worktree remove --force` (a shell/subprocess boundary) inside `internal/worktree/worktree.go`. Purely additive, single `git revert` rollback per proposal. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
