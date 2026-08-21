# Tasks Lens B — Path Partition & DAG Shape: sdd-fan-out-lens

## Assumed work surface

Implementation spans 10 files across 3 surfaces: contract tests in `internal/packet/packet_test.go`, 8 new planning packet templates under `plugin/claude-code/skills/lucind-ai/assets/` (4 explore, 4 propose), and skill documentation in `plugin/claude-code/skills/lucind-ai/SKILL.md`. Existing design templates remain untouched; no Go binaries, DAG schemas, or verify flows are modified.

## Unit partition

| Unit | allowed_paths | Executor | Why this executor |
|---|---|---|---|
| Unit 1: Contract Tests | `["internal/packet/packet_test.go"]` | `cursor-agent` | Precision test authoring on a single Go test file for `TestSkillAssetContract` and table-driven template parsing/disjointness tests; matches "single-piece precision" aptitude. |
| Unit 2: Explore Templates | `["plugin/claude-code/skills/lucind-ai/assets/explore-lens-a-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/explore-lens-b-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/explore-lens-c-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/explore-synthesis-packet-template.md"]` | `agy` | Multi-file mechanical template generation following existing `design-*-packet-template.md` pattern; matches "sweeps and volume" aptitude. |
| Unit 3: Propose Templates | `["plugin/claude-code/skills/lucind-ai/assets/propose-lens-a-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/propose-lens-b-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/propose-lens-c-packet-template.md", "plugin/claude-code/skills/lucind-ai/assets/propose-synthesis-packet-template.md"]` | `agy` | Multi-file mechanical template generation following existing `design-*-packet-template.md` pattern; matches "sweeps and volume" aptitude. |
| Unit 4: Skill Contract Docs | `["plugin/claude-code/skills/lucind-ai/SKILL.md"]` | `cursor-agent` | High-precision editorial updates to a single canonical skill document (5 frontmatter keys, CLI subcommands/flags, branch ownership, 2-wave failure recovery); matches "single-piece precision" aptitude. |

## Disjointness

Same-wave units (Wave 2: Units 2, 3, 4) are strictly pairwise disjoint under component-boundary prefix matching (`internal/packet/disjoint.go:8-22`):
1. **Unit 2 vs Unit 3**: Unit 2 owns 4 distinct `explore-*.md` template paths; Unit 3 owns 4 distinct `propose-*.md` template paths. No file path overlaps or prefixes another.
2. **Units 2 & 3 vs Unit 4**: Units 2 and 3 operate within `plugin/claude-code/skills/lucind-ai/assets/`; Unit 4 operates on `plugin/claude-code/skills/lucind-ai/SKILL.md`. Neither prefixes the other.

Any accidental path overlap between concurrent units is caught upfront and rejected by `packet.DisjointAllowedPaths` (`internal/packet/disjoint.go:29-48`; invoked at `cmd/lucind-ai/cli.go:243` before `worktree.Create`), failing dispatch with `packet: overlapping allowed_paths between %q (%s) and %q (%s)` (`disjoint.go:41-42`). At split time, `ValidateGlobalOverlap` (`internal/dag/waves.go:68`) enforces global acyclic ordering for overlapping scopes.

## Dependency edges

| From | To | What breaks if concurrent |
|---|---|---|
| Unit 1 | Unit 2 | TDD contract inversion. Explore templates would be authored without the failing `packet.Parse` and `DisjointAllowedPaths` contract tests present in the worktree base, preventing in-lane test verification and risking unverified template syntax. |
| Unit 1 | Unit 3 | TDD contract inversion. Propose templates would be authored without the failing `packet.Parse` and `DisjointAllowedPaths` contract tests present in the worktree base, preventing in-lane test verification and risking unverified template syntax. |
| Unit 1 | Unit 4 | TDD contract inversion. `SKILL.md` updates would proceed without failing `TestSkillAssetContract` assertions present in the worktree base, preventing in-lane validation of frontmatter keys, CLI subcommands, and flags. |

## Waves

| Wave | Units | Mutually disjoint? |
|---|---|---|
| Wave 1 | Unit 1 | Yes (single unit) |
| Wave 2 | Unit 2, Unit 3, Unit 4 | Yes (proven via pairwise disjoint `allowed_paths` across all 3 units) |

## Open Questions

- [ ] None
