# Design: sdd-fan-out-lens

## Technical Approach

Candidate 1 (null option): harden the authoring contract, not the binary. Planning phases run as two sequential `lucind-ai run --packet` invocations already implemented by `runDispatch` (`cmd/lucind-ai/cli.go:121-149`). Wave 1 dispatches three `agy` lanes to disjoint draft paths; wave 2 dispatches one `cursor-agent` synthesizer from the integrated tree (`specs/sdd-planning-fan-out/spec.md:5-20`).

This change updates `plugin/claude-code/skills/lucind-ai/SKILL.md`, adds explore and propose packet templates under `plugin/claude-code/skills/lucind-ai/assets/` beside the existing design templates, and extends `internal/packet/packet_test.go`. No edits to `cmd/lucind-ai/*`, `internal/run/*`, or `internal/dag/*`.

## Architecture Decisions

### Decision: Phase-specific template files

**Choice**: Dedicated files per phase and lane: `explore-lens-{a,b,c}-packet-template.md`, `explore-synthesis-packet-template.md`, `propose-lens-{a,b,c}-packet-template.md`, `propose-synthesis-packet-template.md`, next to existing `design-lens-{a,b,c}-packet-template.md` and `design-synthesis-packet-template.md`.
**Alternatives considered**: One parameterized `planning-lens` / `planning-synthesis` family; speculative `specs`/`tasks` templates before those partitions stabilize.
**Rationale**: Design already needs distinct slice ownership, exclusive reading lists, and a compression gap (`plugin/claude-code/skills/lucind-ai/SKILL.md:143-148,199-207`). A generic family forces the orchestrator to reconstruct that prompt architecture. Explore and propose need the same treatment; design templates already exist and parse (`plugin/claude-code/skills/lucind-ai/assets/design-lens-a-packet-template.md:6` carries `legacy_main: true`).
**Terminal consumer**: New contract tests calling `packet.Parse` (`internal/packet/packet.go:78-166`) on each planning template (`spec.md:53-74`).

### Decision: Two-tier operator remediation for wave-1 failure

**Choice**: `SKILL.md` tells the operator: (1) admission failure (`status: failed`, empty worktree path) → inspect and repair `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, or `legacy_main: true`; (2) execution `blocked`/`failed`/`deviated` → remediate and re-dispatch only the failed lane. Dispatch wave 2 only after `integrated_ids` contains all three lens IDs. Unresolvable blockage stays with the operator; do not start synthesis.
**Alternatives considered**: Wave 2 on 2-of-3 drafts; re-run the whole 3-lane batch; fall back to one monolithic agent.
**Rationale**: Admission already fails closed and silent (`SKILL.md:178-182`; `internal/run/run.go:250-265`). The synthesizer worktree is the first tree that contains all three drafts (`SKILL.md:184-186`). Partial feedstock collapses the 3-slice partition.
**Terminal consumer**: `SKILL.md` operator copy beside the existing silent-admission paragraph (`SKILL.md:178-186`).

### Decision: Substring contract tests, not a Markdown AST

**Choice**: Extend `TestSkillAssetContract` with `strings.Contains` / targeted table-row checks on `SKILL.md`. Add a table-driven test that `packet.Parse`s each planning template and checks `LegacyMain` or feature-target fields plus pairwise `DisjointAllowedPaths` on each phase’s three lens templates.
**Alternatives considered**: Import `goldmark` (or similar) into `internal/packet`; whole-file exact diffs; shell-only template checks.
**Rationale**: `internal/packet` imports only the standard library (`internal/packet/packet.go:6-12`). `TestSkillAssetContract` already reads `SKILL.md` via `os.ReadFile` (`internal/packet/packet_test.go:476-516`). `TestVerifyPacketTemplateAssetStructure` shows the Parse-from-file pattern (`internal/packet/packet_test.go:518-526`) but covers only `verify-packet-template.md` — do not treat that range as planning-fan-out coverage.
**Terminal consumer**: `go test ./internal/packet` (`lucind-checks.sh:4`).

### Decision: Sectioned Markdown synthesis notes

**Choice**: Keep human-read notes with the fixed spine (`## Unresolved Contradictions`, `## Coverage Gaps`, `## Dropped Citations`, `## Architecture Divergence` / phase-equivalent). No YAML/JSON schema on notes files.
**Alternatives considered**: Machine-parseable frontmatter; embedding notes in the canonical artifact; discarding notes after merge.
**Rationale**: The orchestrator reads only the notes file (`SKILL.md:237-241`). `runDispatch` parses packets, not notes (`cmd/lucind-ai/cli.go:121-149`).
**Terminal consumer**: Human orchestrator.

### Decision: Asymmetric precedence and editorial compression

**Choice**: Encode in packet templates and `SKILL.md`: phase skill (`~/.claude/skills/sdd-*/`) governs document schema and required sections; the packet governs topology, slice ownership, word ceilings, and done criteria. Canonical ceiling stays strictly below the sum of lens ceilings (1800 vs 3×1000 for design). No Go word-count check.
**Alternatives considered**: Skill always wins or packet always wins; compile word-count into Go (`proposal.md:28`).
**Rationale**: `sdd-design` describes one sub-agent writing a whole `design.md` (`SKILL.md:218-235`). Read as blanket authority it would collapse three lenses into three full designs. Today that rule names `sdd-design` only; this change generalizes it to every planning-phase fan-out packet (`spec.md:21-36`).
**Terminal consumer**: `SKILL.md` precedence block and template headers.

### Decision: Orchestrator owns the feature lifecycle

**Choice**: The human/orchestrator creates the feature (`lucind-ai feature create` → ledger `feature.Service.Create`, `cmd/lucind-ai/cli.go:687-753`) before wave dispatch. Packets name `feature`/`parent_ref`/`base_sha`/`expected_parent_sha` or set `legacy_main: true` (`internal/packet/packet.go:63-72,114-130`). Lanes do not create or move parent refs.
**Alternatives considered**: Lanes create feature branches at runtime; implicit `main` with no frontmatter or flags.
**Rationale**: Wave-1 lanes run concurrently (`internal/run/batch.go:66-113`). Parent-ref races belong outside the lanes. `feature create` records the feature in the ledger; it does not create `refs/heads/feature/<id>`.
**Terminal consumer**: `SKILL.md` feature-branch ownership copy (`spec.md:37-52`).

## Flow and Invariants

```
Wave 1: [agy lens A] [agy lens B] [agy lens C]  →  integrate  →  Wave 2: [cursor-agent synthesizer]
```

1. **Admission** — Frontmatter has `legacy_main: true` (plus expected SHA) or all four feature keys; body non-empty. Break: `validatePacketAdmission` returns `ErrMissingFeatureTarget` (`internal/run/run.go:250-265`); report is `status: failed` with an empty worktree (`SKILL.md:178-182`).
2. **Disjointness** — Wave-1 `allowed_paths` are pairwise disjoint. Break: `DisjointAllowedPaths` errors before `worktree.Create` (`internal/packet/disjoint.go:29-48`; `cmd/lucind-ai/cli.go:243`). Overlap is a formatted error at `disjoint.go:41`, not a named `ErrOverlappingAllowedPaths`.
3. **Isolation** — Each lens runs in its own worktree (`internal/worktree/worktree.go:168-171`; `internal/run/batch.go:66-89`). Break: out-of-scope diff → `lane.Deviated` (`internal/run/run.go:621-623`); dirty tree / missing commit → `lane.Failed` (`internal/run/run.go:634-662`).
4. **Integration** — All three `done` lanes merge; stdout lists `integrated_ids` / `reverted_ids` (`internal/run/integrate.go:31-81`).
5. **Synthesis branching** — Wave 2 starts only from integrated `HEAD` (`SKILL.md:171-176,184-186`).
6. **Compression** — Canonical word count strictly below the sum of lens ceilings (`spec.md:21-36`). Concatenation is a failed synthesis.
7. **Citation verification** — Synthesizer opens every `file:line` and logs drops (`SKILL.md:237-253`).

## Interfaces / Contracts

No new Go types, CLI flags, or schema versions. Documentation must match what the parser and CLI already accept:

| Surface | Today | Delta |
|---|---|---|
| Frontmatter table | `SKILL.md:22-30` lists `id`/`executor`/`routed_by`/`model`/`agent`/`read_only`/`allowed_paths`. Narrative at `SKILL.md:157-161` names the four feature keys plus legacy mode. | Add all five keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) to the table (`packet.go:63-72,114-130`). |
| Fan-out protocol | Design-only pilot (`SKILL.md:126-135`). | Promote to planning-phase convention (explore, propose, design, specs, tasks). |
| CLI subcommands | Invocation block shows `run`/`split`/`check`/`--version` (`SKILL.md:288-293`). Binary also has `serve`/`feature`/`reconcile`/`renew` (`cli.go:97-111`). | Document the four missing subcommands. |
| `run` flags | Table has `--packet`/`--timeout` (`SKILL.md:295-299`). Binary also has `--approval-timeout`/`--legacy-main`/`--expected-parent-sha` (`cli.go:133-137`). | Document the three missing flags. |

## File Changes

| File | Action | Terminal consumer |
|---|---|---|
| `plugin/claude-code/skills/lucind-ai/SKILL.md` | Modify: five keys, two-wave protocol, precedence, CLI surface, feature ownership, wave-1 failure copy | Orchestrator; `TestSkillAssetContract` (`packet_test.go:476`) |
| `assets/explore-lens-{a,b,c}-packet-template.md` | Create. A: problem & candidates. B: capabilities & scenarios. C: risks, trade-offs & spikes. Disjoint `allowed_paths` | `agy`; `packet.Parse` + `DisjointAllowedPaths` |
| `assets/explore-synthesis-packet-template.md` | Create: canonical `explore.md` + notes | `cursor-agent`; `packet.Parse` |
| `assets/propose-lens-{a,b,c}-packet-template.md` | Create. A: candidate & approach. B: capability impact & specs. C: risks, rollback & test impact | `agy`; `packet.Parse` + `DisjointAllowedPaths` |
| `assets/propose-synthesis-packet-template.md` | Create: canonical `proposal.md` + notes | `cursor-agent`; `packet.Parse` |
| Existing `assets/design-*-packet-template.md` | Leave in place (already `legacy_main: true`) | Same Parse/disjoint tests |
| `internal/packet/packet_test.go` | Extend skill contract; add planning-template Parse + disjointness | `lucind-checks.sh:4` |

## Testing Strategy and Test Seams

| Layer | What | Approach | Seam |
|---|---|---|---|
| Contract | `SKILL.md` documents five keys, two-wave protocol, shipped CLI, feature ownership | `strings.Contains` / table-row asserts | `TestSkillAssetContract` (`packet_test.go:476-516`) |
| Contract | Explore/propose/design templates parse; lens paths disjoint per phase | `packet.Parse` + `DisjointAllowedPaths` | `packet.go:78-166`; `disjoint.go:29-48`; pattern at `packet_test.go:518-526` |
| Non-regression | Dispatch, barrier, integrate unchanged | Existing `go test ./...` | `lucind-checks.sh:4` |

Do not add `cmd/lucind-ai` or `internal/run` tests. Parser coverage for feature keys and `legacy_main` already lives in `TestParseFeatureTargetFrontmatter` / `TestParseLegacyMainFrontmatter` (`packet_test.go:737,815`).

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | Applicable | Templates and `SKILL.md` are passive text. `packet.Parse` reads `---` frontmatter only and does not evaluate the body (`packet.go:78-166`). Malformed `allowed_paths` → `ErrInvalidAllowedPaths`; bad `legacy_main` → `ErrInvalidLegacyMain`. | Contract: every `assets/*.md` planning template Parses; malformed frontmatter returns typed errors without executing the body. |
| Git repository selection | N/A: reason | No git discovery change; `resolvePrimaryRoot` (`cli.go:530-544`) and `IsLinkedWorktree` (`worktree.go:263-277`) stay untouched. | None |
| Commit state | N/A: reason | No commit/index change; `enforceCompletionMode` (`run.go:634-662`) and `PorcelainEmpty` (`worktree.go:304-310`) stay untouched. | None |
| Push state | N/A: reason | `lucind-ai` has no push. | None |
| PR commands | N/A: reason | `lucind-ai` has no PR argv. | None |

## Rollback and Additivity

**Choice**: `git revert` of commits touching `SKILL.md`, `assets/`, and `packet_test.go`.
**Alternatives considered**: Binary rollback or ledger migration. Rejected — no production Go change, no schema move.
**Rationale**: Additive docs, templates, and tests. `.lucind/result.schema.json`, `internal/ledger/schema.go`, and `internal/packet/packet.go:33-75` stay put. Restoring those files restores prior authoring text; in-flight packets and SQLite ledgers are unaffected.

## Open Questions and Out of Scope

Open questions: none. Proposal items at `proposal.md:88,91` are Decisions 1 and 2. Assertion strategy and notes schema (`proposal.md:89-90`) are Decisions 3 and 4.

Out of scope: `cmd/lucind-ai/*` and `internal/run/*` runtime changes; `internal/dag/*`; `lucind-ai fanout`; Go word-count enforcement; verify dual-dispatch (`openspec/specs/verify-dual-dispatch/`); specs/tasks templates until those partitions stabilize.
