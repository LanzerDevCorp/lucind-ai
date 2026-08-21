# Spec Lens C — Non-Regression & Traceability: sdd-fan-out-lens

## Assumed capability

none — no delta

## Non-regression invariants

| Must stay true | Evidence |
|---|---|
| **Generic `--packet` dispatch & barrier join**: `lucind-ai run --packet <path>` continues to accept repeatable packet paths, provision isolated worktrees per lane, execute lanes concurrently with independent deadlines, enforce upfront path disjointness, and join at the barrier without sidecar DAG dependency. | `cmd/lucind-ai/cli.go:121-149,241-246,282-298`; `internal/run/batch.go:66-113`; `internal/barrier/barrier.go:49-57`; `internal/worktree/worktree.go:168-237` |
| **CLI subcommand & flag routing**: Subcommand routing (`run`, `split`, `check`, `serve`, `feature`, `reconcile`, `renew`, `--version`) and flags (`--timeout`, `--approval-timeout`, `--legacy-main`, `--expected-parent-sha`) remain unchanged; no new subcommands or flags are added to the Go binary. | `cmd/lucind-ai/cli.go:100-119,132-138` |
| **Sidecar DAG validation & scope**: `apply-dag.yaml` sidecar parsing and `lucind-ai split` remain apply-phase only; `dag.Node` requires `body_path` and non-empty `allowed_paths`, does not support `read_only`, and rejects duplicate IDs or self-dependencies. | `internal/dag/parse.go:21-36,44-86`; `internal/dag/validate.go:19-52`; `internal/dag/emit.go:23-53`; `openspec/specs/apply-dag-dispatch/spec.md:9-27,51-59` |
| **Verify dual-dispatch & mechanical checks**: The two-stage verification workflow (Stage 1 mechanical check committing `verify-mechanical.log`, Stage 2 dual read-only judgment dispatch to `agy` + `cursor-agent`, Stage 3 envelope reconciliation in `verify.md`) executes unmodified. | `openspec/specs/verify-dual-dispatch/spec.md:11-28,31-45,145-151`; `openspec/specs/verify-mechanical-check/spec.md:9-30`; `cmd/lucind-ai/cli.go:102-103` |
| **No dedicated fan-out generator**: Multi-lens fan-out packet authoring remains an orchestrator prompt and template convention; no binary generator subcommand (`lucind-ai fanout`) is compiled into Go. | `cmd/lucind-ai/cli.go:100-118`; `openspec/changes/sdd-fan-out-lens/explore.md:62-67` |
| **No Go-level word count or prompt schema enforcement**: Word limits (~1000 words per lens, ~1800 words for synthesis) and Markdown skeletons remain prompt conventions compressed during synthesis, not parsed or validated by Go binary runtime. | `internal/packet/packet.go:33-74,94-165`; `internal/run/run.go:515-543` |
| **Read-only packet schema and zero-commit invariant**: `read_only: true` frontmatter flag allows omitting `allowed_paths`, skips path-diff enforcement, and requires zero unique commits and a clean worktree at completion. | `internal/packet/packet.go:55-58,105-113`; `internal/run/run.go:654-662`; `openspec/specs/read-only-packet-schema/spec.md:9-25`; `openspec/specs/read-only-done-criterion/spec.md:9-25` |
| **Allowed-paths enforcement & completion modes**: Post-run 4-way git diff against `BaseSHA` demotes out-of-scope modifications to `Deviated`; write packets require at least one commit and a clean worktree. | `internal/run/run.go:590-626,634-653`; `openspec/specs/allowed-paths-enforcement/spec.md:9-30`; `openspec/specs/completion-mode-enforcement/spec.md:9-30` |
| **Parent feature target admission**: Admission fails closed (`status: failed`, empty worktree path) unless all four target keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`) are present or `legacy_main: true` is set. | `internal/packet/packet.go:63-72,114-130`; `openspec/specs/parent-feature-integration/spec.md:9-30` |

## Traceability

| Proposal commitment | Lands in | Gap? |
|---|---|---|
| **SC-1**: `SKILL.md` documents multi-lens convention, 5 feature-target keys, shipped subcommands/flags, and feature-branch ownership. | `plugin/claude-code/skills/lucind-ai/SKILL.md` (lines 22-30, 126-235, 284-303), pinned by `internal/packet/packet_test.go:TestSkillAssetContract`. | No |
| **SC-2**: Fan-out packet templates exist under `assets/` and parse as packets. | `plugin/claude-code/skills/lucind-ai/assets/` templates, pinned by `internal/packet/packet_test.go:TestPacketTemplateAssetStructure`. | No |
| **SC-3**: Asset and skill contract tests fail if tables or templates drift. | `internal/packet/packet_test.go` test suite extension. | No |
| **SC-4**: No changes to Go dispatch logic under `cmd/` or `internal/run/`. | Non-regression invariant owned by existing Go dispatch (`cmd/lucind-ai/cli.go:121-149`, `internal/run/run.go:515-543`). | No |
| **SC-5**: A hand-authored two-wave fan-out still runs on generic `lucind-ai run --packet`. | Existing dispatch capabilities (`read-only-packet-schema`, `parent-feature-integration`, `cmd/lucind-ai/cli.go:121-149,285-298`, `internal/run/batch.go:66-113`). | No |
| **AA-1**: `plugin/claude-code/skills/lucind-ai/SKILL.md` (Fan-out convention, feature-target keys, CLI surface, feature-branch ownership, wave protocol). | `plugin/claude-code/skills/lucind-ai/SKILL.md` documentation updates, pinned by `internal/packet/packet_test.go`. | No |
| **AA-2**: `plugin/claude-code/skills/lucind-ai/assets/` (Lens and synthesizer templates with required frontmatter and disjoint draft paths). | `plugin/claude-code/skills/lucind-ai/assets/` markdown templates, pinned by `internal/packet/packet_test.go`. | No |
| **AA-3**: `internal/packet/packet_test.go` (Contract tests for skill text and templates versus the parser). | `internal/packet/packet_test.go` contract test assertions. | No |

## Design questions: legitimately deferred, or requirement in disguise?

| Question | Verdict | Why |
|---|---|---|
| **Q1**: One shared template family for every fan-out phase, or phase-specific templates beyond design / explore / propose? | **Legitimately deferred to design** | Asset file organization and template inheritance structure are internal implementation choices that do not alter system behavioral contracts or runtime dispatch semantics. |
| **Q2**: Exact assertions in `packet_test.go` (which `SKILL.md` strings, which template keys). | **Requirement in disguise** | Defining which frontmatter keys (e.g. `legacy_main: true` vs. 4 feature-target fields) and CLI doc strings must mechanically pass parser validation defines the system contract boundary, not an internal design choice. |
| **Q3**: Whether synthesis-notes stay sectioned Markdown or gain machine-parseable frontmatter. | **Requirement in disguise** | Downstream artifact consumption contracts (by operators, orchestrators, or verification tools) establish behavioral interfaces; deciding whether notes are free-form markdown or machine-structured frontmatter is a requirement on artifact schema. |
| **Q4**: What `SKILL.md` tells the operator if a wave-1 lens fails admission or execution. | **Legitimately deferred to design** | Binary admission rejection and barrier failure behaviors are already specified in Go (`run.ErrMissingFeatureTarget`, ledger status); crafting operator recovery guidance and troubleshooting prose in `SKILL.md` is documentation design. |

## Additivity

Reverting the commits touching `plugin/claude-code/skills/lucind-ai/SKILL.md`, `plugin/claude-code/skills/lucind-ai/assets/`, and `internal/packet/packet_test.go` cleanly restores the prior repository state with zero side effects:

- **Schema versions**: Frontmatter parser (`internal/packet/packet.go:33-74`), result envelope (`.lucind/result.schema.json`), and sidecar DAG schema (`internal/dag/parse.go:21-36`) remain bit-for-bit identical.
- **Ledger state**: SQLite database schema and persistence logic (`internal/ledger/`) remain untouched with no database migrations.
- **Spec versions**: No specifications in `openspec/specs/` are modified, added, or deleted.
- **Runtime binary**: No Go runtime recompilation or deployment required. In-flight packets and existing worktrees continue execution unaffected.

## Open Questions

- [ ] None
