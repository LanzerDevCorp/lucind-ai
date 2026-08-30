---
id: design-agentic-phase-specialist-lens-c
executor: agy
routed_by: failure-test-rollback lens of the three-lens design fan-out
model: gemini-3.7-flash-high
allowed_paths: ["openspec/changes/agentic-phase-specialist/design-lens-c.md"]
---

# Packet design-agentic-phase-specialist-lens-c

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-agentic-phase-specialist-lens-c  ·  **Branch:** lucind/design-agentic-phase-specialist-lens-c

## Goal

Produce `openspec/changes/agentic-phase-specialist/design-lens-c.md`: how this change is tested, existing seams, the applicability-driven threat matrix, and the rollback/additivity decision.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

The proposal and specs for `agentic-phase-specialist` are accepted and frozen. Lens A and lens B run in parallel against the same frozen inputs and write to different files, so no lane races another.

Lens A owns the architecture decision and is running concurrently. Declare the architecture you are assuming in `## Assumed architecture`.

## Preconditions

- `openspec/changes/agentic-phase-specialist/proposal.md` exists and is accepted.
- `openspec/changes/agentic-phase-specialist/specs/` exists.
- `openspec/changes/agentic-phase-specialist/design-lens-c.md` does not yet exist.
- The threat-matrix reference table is embedded verbatim in this packet's `## Context`.

## Required reading (this lens only)

1. The real `gentle-ai` design skill (delivered under `## Required skills`).
2. `openspec/changes/agentic-phase-specialist/proposal.md` and `openspec/changes/agentic-phase-specialist/specs/`.
3. `internal/accept/accept_test.go`, `internal/run/attempt_test.go`, `internal/integrate/integrate_test.go` — how this repository actually tests these packages: what it asserts on, what it fakes.
4. Injection seams already in these packages — `Verifier` struct fields, `checkFunc`-style function fields, existing fakes/stubs. Name them by `file:line`.
5. The threat-matrix table in `## Context` of this packet.

Never guess at a test seam.

## Output format

Write exactly this skeleton to `openspec/changes/agentic-phase-specialist/design-lens-c.md`:

```markdown
# Design Lens C — Failure, Test & Rollback: Agentic Phase Specialist

## Assumed architecture

<2-4 sentences. Lens A and lens B write this same block independently.>

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|

<Must cover: accept.go's phase-gated check skip, attempt.go's equivalent gate,
and the byte-identity/glossary-containment tests already enforcing SKILL.md and
fan-out.md consistency (internal/packet/packet_test.go).>

## Test Seams

<What is injectable/fakeable today for testing the check-gate change specifically.>

## Threat Matrix

<The table from `## Context`, every row marked Applicable or N/A: reason. This
change touches no routing, shell, subprocess, VCS/PR automation, or executable-file
classification boundary in Go code — it is a conditional-skip added to an existing
function. Justify each N/A concretely rather than asserting it.>

## Rollback and Additivity

**Choice**: <what reverting looks like>
**Alternatives considered**: <what other reversal strategy was rejected>
**Rationale**: <why, grounded in what actually changes>

<State explicitly whether any schema, ledger, or envelope version moves.>

## Out of Scope

<Adjacent work this change explicitly does not do.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-c.md` MUST be under 1000 words.

## Out of scope

Owned by the sibling lenses:

- **Lens A owns**: technical approach and every architecture decision except rollback.
- **Lens B owns**: file-changes table, data-flow diagrams, invariants, signature deltas.

Rollback is yours.

## Allowed paths

`openspec/changes/agentic-phase-specialist/design-lens-c.md` only.

## Allowed paths outside the repository

**Read-only**: The real `gentle-ai` design skill and its `references/` (delivered under `## Required skills`). Not symmetric: skill governs *what*; this packet governs *how this phase is executed here*, including this repository's actual 1800-word canonical budget.

Write nothing outside this repository.

## Citation manifest (REQUIRED — excluded from the word budget)

One row per unique citation, grouped by file, ascending line numbers, plainly stated claim.

## Mechanical self-check (REQUIRED — replaces narrating these facts)

**Before you commit**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design-lens-c.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed architecture" --require-section "Testing Strategy" \
  --require-section "Test Seams" --require-section "Threat Matrix" \
  --require-section "Rollback and Additivity" --require-section "Out of Scope" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`**:

```
./lucind-lane-check.sh --file openspec/changes/agentic-phase-specialist/design-lens-c.md
```

Paste both reports' PASS/FAIL lines into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` reported no FAIL against this draft's own manifest.**
- [ ] **Every named test seam carries a `file:line` citation**, or is marked "new seam required".
- [ ] **Every threat-matrix row is marked Applicable or N/A with a reason.**
- [ ] **`design-lens-c.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section.**
- [ ] **The work is committed with a conventional commit and no AI attribution.**

## Hard stops

- A behavior the specs require cannot be tested through any existing or proposed seam.
- Whether a format delta is additive cannot be determined from the specs.
- The threat matrix is missing from `## Context`.
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

### Ground truth

**Change**: `agentic-phase-specialist`. Proposal's Technical Risks & Failure Modes table (verbatim summary): Specialist Acceptance admits defective planning artifacts → mitigated by fail-closed schema/scope (`accept.go:97-98,214-261`), hard-stop→blocked (`internal/run/run.go:841-845`), Dual-Judge for Tier A, Promotion stays human. Check gate skips Go suite on mislabeled/unlabeled lanes → mitigated by running checks if `sdd_phase` is `"apply"`, empty, or exception. Hard Rule carve-out misread as general executor Acceptance → mitigated by naming `sdd-*` Specialists only, own-phase Lanes only.

Proposal's Test and Validation Impact table: `internal/accept` needs coverage for skip-declared-non-apply / run-for-apply-empty-or-exception / missing-metadata-does-not-skip (`accept_test.go:26-67,80-100`); `internal/run` needs the same gate in `executeAttempt` (`attempt_test.go:24-80`); `internal/integrate` — `Check` remains ungated (`integrate_test.go:21-50`); `internal/packet` — byte-identical skill trees and glossary projection after SKILL.md/fan-out.md edits (`packet_test.go:924-967`).

**Do not relitigate**: no DDL, no schema migration; `internal/ledger/authoring.go` unchanged.

## Required skills

- sdd-design

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
