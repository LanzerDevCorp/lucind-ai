---
id: remediate-canonical-filename-and-skills-section
executor: agy
routed_by: fix 2 confirmed residual findings from the second BLOCKED verify pass
model: gemini-3.7-flash-high
---

# Packet remediate-canonical-filename-and-skills-section

**Tier:** A (human merge)
**Worktree:** ../lucind-ai-worktrees/remediate-canonical-filename-and-skills-section  ·  **Branch:** lucind/remediate-canonical-filename-and-skills-section

## Goal

Fix two confirmed findings from `openspec/changes/skill-provisioning-and-phase-specialist/verify.md`'s second (current) BLOCKED verdict, findings 9 and 10:

**Finding 9 (canonical filename mismatch):** `internal/phasespec/phasespec.go`'s `CanonicalArtifactFilename("propose")` returns `"propose.md"`, but this repository's own live `gentle-ai sdd-status` output uses `proposal.md` as the actual artifact name for the propose phase (confirmed directly against this very change's own `sdd-status` output during this session, and against `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md:12` and existing packet templates' `allowed_paths`, all of which say `proposal.md`). `isPhaseComplete`'s `os.Stat` check only looks for the canonical path it computes, so it will never find this repo's real `proposal.md` and will never correctly report the propose phase complete.

**Finding 10 (missing Required skills section):** The specialist's dynamically-generated synthesis packet body — the `packetContent := fmt.Sprintf(...)` string literal in `cmd/lucind-ai/cli.go` (around line 2466, inside the phase-dispatch branch) — has no `## Required skills` section. Dual delivery on this specific path is env-var-only (`LUCIND_REQUIRED_SKILLS`); the compiled-contract path (`internal/packetauthor/compile.go`'s `renderBody`) already correctly emits this section for regular (non-specialist-generated) packets.

## Preconditions

- Read `internal/phasespec/phasespec.go`'s `CanonicalArtifactFilename` and `CanonicalArtifactPath` functions (currently lines ~229-260) and every caller of both.
- Read `openspec/changes/skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md` — decide whether to change the code to match the repo's real `proposal.md` convention, or whether the spec's scenario text is itself the thing that needs correcting. Prefer matching this repo's actual, currently-in-use convention (`proposal.md`, `apply.md`/`apply-progress.md`, `verify.md`/`verify-report.md`, `archive.md`/`archive-report.md` — check what this exact change's own `openspec/changes/skill-provisioning-and-phase-specialist/` directory contains for precedent) over the literal spec scenario text, since the goal is a working specialist against this repo's real state, not a technically-spec-compliant-but-non-functional one. If genuinely ambiguous, declare the hard stop below instead of guessing.
- Read `cmd/lucind-ai/cli.go` around the `packetContent := fmt.Sprintf(...)` block (~line 2440-2500) and `internal/packetauthor/compile.go`'s `renderBody` (~lines 220-238) to see exactly what a `## Required skills` section looks like and how resolved skill paths are rendered there, so the specialist's hand-built packet matches that same rendered format.
- Read `internal/phasespec/phasespec_test.go` and `cmd/lucind-ai/cli_test.go` for the existing tests covering these two code paths (canonical filenames, specialist packet generation) so you extend rather than duplicate coverage.

## Allowed paths

- `internal/phasespec/`
- `cmd/lucind-ai/cli.go`
- `cmd/lucind-ai/cli_test.go`
- `openspec/changes/skill-provisioning-and-phase-specialist/specs/phase-specialist-dispatch/spec.md` (only if the spec's own scenario text needs updating to match the corrected, repo-real filename convention — do not touch other specs)

## Read-only inputs

- `internal/packetauthor/`
- `internal/skillroots/`
- `plugin/claude-code/skills/lucind-ai/references/strategies/fan-out.md`

## Out of scope

Do not touch `internal/run`, `internal/accept`, `internal/skillset`, or any plugin/asset files. Do not change how the specialist derives which skills are required (that logic is already correct and tested) — only how they are rendered into the hand-built packet body. Do not add a `cmd` import into `internal/phasespec`.

## Done criteria

- [ ] `CanonicalArtifactFilename`/`CanonicalArtifactPath` for the propose phase now matches this repo's actual, currently-used artifact filename (verify by checking what name this very change's own directory or `gentle-ai sdd-status` output uses).
- [ ] A test proves `isPhaseComplete` now correctly recognizes a `proposal.md` (or whatever the corrected convention is) file as satisfying the propose-phase completeness check.
- [ ] The specialist's dynamically-generated packet body now includes a `## Required skills` section listing the resolved skill paths, matching the format used by `compile.go`'s `renderBody` for consistency.
- [ ] A test proves the specialist-generated packet body contains the `## Required skills` section with the expected resolved paths.
- [ ] `go build ./...` and `go test ./... -race -count=1` are green.
- [ ] Commit conventionally with no AI attribution; clean status and latest commit evidence are recorded.

## Hard stops

- Stop `blocked` if it's genuinely ambiguous which filename convention (`propose.md` vs `proposal.md`, etc.) is correct after checking all the evidence sources listed in Preconditions — describe the conflicting evidence instead of guessing.
- Stop `blocked` if rendering `## Required skills` into the specialist's packet requires resolved skill paths that aren't already available at that point in `cli.go` without duplicating `skillroots` resolution logic that belongs elsewhere.

## Return

Write the result envelope to **.lucind/result.json in this worktree**. Validate it against `.lucind/result.schema.json` before writing.
