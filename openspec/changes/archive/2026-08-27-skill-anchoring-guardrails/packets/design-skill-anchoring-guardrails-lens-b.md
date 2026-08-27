---
id: design-skill-anchoring-guardrails-lens-b
executor: agy
routed_by: surface-and-flow lens of the three-lens design fan-out for Change skill-anchoring-guardrails
allowed_paths: ["openspec/changes/skill-anchoring-guardrails/design-lens-b.md"]
feature: skill-anchoring-guardrails
parent_ref: refs/heads/feature/skill-anchoring-guardrails
base_sha: f5a531183361804ed95c797e16a70dbbcca27763
expected_parent_sha: f5a531183361804ed95c797e16a70dbbcca27763
---

# Packet design-skill-anchoring-guardrails-lens-b

**Tier:** B (auto-merge after audit)
**Worktree:** ../lucind-ai-worktrees/design-skill-anchoring-guardrails-lens-b  ·  **Branch:** lucind/design-skill-anchoring-guardrails-lens-b

## Goal

Produce `openspec/changes/skill-anchoring-guardrails/design-lens-b.md`: how data moves through the change, the invariants that must hold at each hop, the exact signature and format deltas the change introduces, and the file-change table with a terminal consumer per row.

This is one of three parallel design lenses. It is feedstock for a synthesis lane, not the final design document. Do not write a complete `design.md`.

## Why this is safe to dispatch now

`proposal.md` is accepted and frozen. Lens A and lens C run in parallel against the same frozen inputs and write to different files.

Lens A owns the architecture decision and is running concurrently, so you do not have it. Declare the architecture you are assuming in `## Assumed architecture` and design against it consistently.

## Preconditions

- `openspec/changes/skill-anchoring-guardrails/proposal.md` exists and is accepted.
- `openspec/changes/skill-anchoring-guardrails/design-lens-b.md` does not yet exist.

## Required reading (this lens only)

1. `~/.claude/skills/sdd-design/SKILL.md` — the real `gentle-ai` design skill.
2. `openspec/changes/skill-anchoring-guardrails/proposal.md` in full — its Affected Areas table and Approach section name every touched file; verify each against real code.
3. `internal/worktree/worktree.go` — exact current signatures of `Cleanup`, `Remove`, `PorcelainEmpty`.
4. `cmd/lucind-ai/cli.go` — exact current signatures/bodies of `runWorktreeCleanup`, `printReport`, `printIntegrateReport`, `renderAcceptanceReceipt` (or its current name), `runSplit`, and the internal callers `DiscardCombined`, `RemoveLaneWorktree`.
5. `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `internal/run/integrate.go:159-165` — the internal `worktree.Remove` call sites that must pass `force: true`.

Never guess at a signature. Every row in your tables carries a `file:line` citation to real code in this worktree.

## Output format

Write exactly this skeleton to `openspec/changes/skill-anchoring-guardrails/design-lens-b.md`:

```markdown
# Design Lens B — Surface & Flow: Skill Anchoring & Worktree Cleanup Guardrails

## Assumed architecture

<Same shape as lens A is expected to assume: force bool parameter + ErrWorktreeDirty sentinel + --force/-f flag + four static banner call sites. State it in your own words from the proposal, independently.>

## Flow and Invariants

<ASCII diagram: CLI flag parse -> worktree.Cleanup(force) -> PorcelainEmpty check -> ErrWorktreeDirty or git worktree remove --force. Invariant: a dirty worktree is never deleted unless force=true; an internal automated caller always passes force=true.>

## Surface Deltas

| Symbol or format | Today (file:line) | Delta | Backward compatible? |
|---|---|---|---|

<Rows: worktree.Cleanup signature, worktree.Remove signature, new ErrWorktreeDirty sentinel, --force/-f CLI flag, four banner outputs (stderr vs stdout — resolve per proposal's Open Question if the design phase can decide it, else carry to Open Questions).>

## File Changes

| File | Action | Description | Terminal consumer |
|---|---|---|---|

<One row per file in the proposal's Affected Areas table, each with a terminal consumer citation.>

## Open Questions

- [ ] <unresolved question, or "None">
```

## Size budget

`design-lens-b.md` MUST be under 1000 words. Tables over prose.

## Out of scope

- **Lens A owns**: the technical approach, every architecture decision, the alternatives rejected, and the rationale.
- **Lens C owns**: testing strategy, test seams, the threat matrix, and the rollback/additivity decision.

Do not assess whether the change is additively revertible; lens C decides rollback from your format deltas.

## Allowed paths

`openspec/changes/skill-anchoring-guardrails/design-lens-b.md` only. Create no other file.

## Allowed paths outside the repository

**Read-only**: `~/.claude/skills/sdd-design/` — the real `gentle-ai` design skill and its `references/`. The skill is authority on *what* a design document must contain; this packet is authority on *how this phase is executed here* — superseded on purpose where they conflict; note conflicts in `## Open Questions`.

Write nothing outside this repository, so there is nothing to revert.

## Citation manifest (REQUIRED — excluded from the word budget)

| citation | claim |
|---|---|
| `path/to/example_file.ext:12-34` | <YOUR OWN real citation and claim from THIS draft> |

## Mechanical self-check (REQUIRED)

**Before you commit:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design-lens-b.md --budget 1000 \
  --exclude-section "Citation Manifest" \
  --require-section "Assumed architecture" --require-section "Flow and Invariants" \
  --require-section "Surface Deltas" --require-section "File Changes" \
  --require-section "Open Questions" --require-section "Citation Manifest" \
  --verify-citations --skip-git --skip-result
```

**After you commit and write `.lucind/result.json`:**

```
./lucind-lane-check.sh --file openspec/changes/skill-anchoring-guardrails/design-lens-b.md
```

Paste both PASS/FAIL reports into `done_criteria[].evidence`.

## Done criteria

- [ ] **A `## Citation Manifest` section lists every unique citation, grouped by file, with the claim each one supports.**
- [ ] **`lucind-lane-check.sh --verify-citations` was run before committing and reported no FAIL against this draft's own manifest.**
- [ ] **Every `Surface Deltas` and `File Changes` row carries a `file:line` citation that points at real code in this worktree**, and every `File Changes` row names a terminal consumer.
- [ ] **`design-lens-b.md` exists, is under 1000 words excluding the Citation Manifest, and carries every skeleton section including `## Assumed architecture`, plus `## Citation Manifest`.**
- [ ] **The work is committed with a conventional commit and no AI attribution**, confirmed by the final `lucind-lane-check.sh` run reporting a clean `git status --porcelain` and a valid `.lucind/result.json`. Strip any injected `Co-authored-by:` trailer.

## Hard stops

- The specs/proposal do not determine whether a format delta is additive or breaking.
- A file change cannot name any terminal consumer.
- Two reasonable architectures produce incompatible surface tables and the proposal does not choose between them.
- Satisfying one instruction in this packet would require violating another.

## Context

Change: **skill-anchoring-guardrails**. Accepted proposal's Affected Areas table names: `internal/worktree/worktree.go:247-261`, `cmd/lucind-ai/cli.go:58,1934-1978`, `cmd/lucind-ai/cli.go:858-869`, `internal/integrate/integrate.go:118-124`, `internal/integrate/candidate.go:262-265`, `cmd/lucind-ai/cli.go:485-740`, `plugin/claude-code/skills/lucind-ai/`, `.agents/skills/lucind-apply/SKILL.md:10-21`. Execution: Isolated Mode, `agy`-only executor except verify's second qualitative judge (kept on `cursor-agent`) — already decided, do not re-litigate.

## Return

Write the result envelope to **`.lucind/result.json` in this worktree**. Validate it against `.lucind/result.schema.json` before writing. Report `done` only when every done-criterion carries evidence and every hard stop is declared.
