# Design Lens C — Failure, Test & Rollback: sdd-fan-out-lens

## Assumed architecture

Candidate 1 (the null option): Harden authoring contracts, documentation, and template assets without adding Go binary features, runtime dispatch alterations, or new CLI subcommands/flags (`cmd/lucind-ai/cli.go:121-149`, `internal/run/batch.go:66-113`, `internal/run/integrate.go:31-81`). In-scope code changes are strictly confined to `internal/packet/packet_test.go` to add contract tests for skill text, frontmatter keys, and template assets. `plugin/claude-code/skills/lucind-ai/SKILL.md` promotes the three-lens fan-out protocol to planning phases, documents the five feature-target keys and shipped CLI surface, and defines feature-branch ownership. `plugin/claude-code/skills/lucind-ai/assets/` adds explore and propose lens/synthesis templates alongside existing design templates.

## Testing Strategy

| Layer | What to test | Approach | Existing seam (file:line) |
|---|---|---|---|
| Contract | `SKILL.md` documents 5 feature-target keys (`feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, `legacy_main`) | Parse `SKILL.md` frontmatter table and assert each key is documented against parser fields | `internal/packet/packet_test.go:476-516`, `internal/packet/packet.go:63-72` |
| Contract | `SKILL.md` documents shipped CLI surface (`serve`, `feature`, `reconcile`, `renew`, `check`, `split`, `run` flags) | Assert exact subcommand and flag strings exist in `SKILL.md` | `internal/packet/packet_test.go:476-516`, `cmd/lucind-ai/cli.go:100-118,133-138` |
| Contract | `SKILL.md` documents two-wave planning fan-out protocol and feature-branch ownership | Assert planning fan-out protocol, wave sequencing, and branch ownership sections exist | `internal/packet/packet_test.go:476-516`, `plugin/claude-code/skills/lucind-ai/SKILL.md:126-198` |
| Contract | Planning fan-out templates under `assets/` parse as valid packets | Read each template file and invoke `packet.Parse`, asserting valid frontmatter, body, and executor | `internal/packet/packet_test.go:518-594`, `internal/packet/packet.go:77-166` |
| Contract | Wave-1 lens templates within each phase declare pairwise disjoint `allowed_paths` | Parse lens templates per phase and evaluate `packet.DisjointAllowedPaths` | `internal/packet/disjoint.go:29-48`, `internal/packet/disjoint_test.go:95-217` |
| Unit | Admission fails closed when packet omits 4 feature-target keys and omits `legacy_main: true` | Parse frontmatter lacking target keys and verify `lucind-ai run` admission rejection | `internal/packet/packet.go:122-130`, `cmd/lucind-ai/cli.go:167-174`, `internal/packet/packet_test.go:737-887` |
| Non-regression | Existing binary dispatch, barrier join, and integration paths remain intact | Execute full repository test suite (`go test ./...`) | `cmd/lucind-ai/cli_test.go:1-500`, `internal/run/run_test.go:1-400`, `internal/run/integrate.go:31-81` |

## Test Seams

Existing seams are fully sufficient; no new injection points or production interfaces are required:

- **Packet parsing (`packet.Parse`)**: `internal/packet/packet.go:77-166` accepts an `io.Reader`, allowing in-memory and file-based validation of template markdown and skill frontmatter snippets without spawning subprocesses.
- **Path disjointness (`packet.DisjointAllowedPaths`)**: `internal/packet/disjoint.go:29-48` evaluates slice-level path scope overlap purely in memory.
- **Skill contract suite (`TestSkillAssetContract`)**: `internal/packet/packet_test.go:476-516` reads `plugin/claude-code/skills/lucind-ai/SKILL.md` directly via `os.ReadFile` and asserts documentation integrity.
- **Template asset suite (`TestVerifyPacketTemplateAssetStructure`)**: `internal/packet/packet_test.go:518-594` demonstrates the established pattern for verifying template asset structure and frontmatter validity.
- **CLI dispatch harnesses (`runDispatch`)**: `cmd/lucind-ai/cli.go:124-330` accepts `io.Writer` streams for stdout/stderr and injects `depsFactory` (`cmd/lucind-ai/cli.go:548-599`), enabling complete mock/fake execution in existing tests.

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | `requirements.txt`, `CMakeLists.txt`, executable Markdown/MDX, `README.sh` | Applicable | Markdown templates and skill docs are treated as passive text; `packet.Parse` strictly extracts frontmatter delimiter blocks and never evaluates executable scripts or expressions. Malformed frontmatter fails closed with explicit parse errors. | Unit contract test in `internal/packet/packet_test.go` asserting all `assets/*.md` templates parse cleanly, and malformed frontmatter (unclosed delimiters, non-JSON `allowed_paths`, non-bool `legacy_main`) returns typed errors without evaluating body content. |
| Git repository selection | `git -C`, relative paths, absolute paths | N/A: reason | No git discovery or cwd resolution logic is modified; `cmd/lucind-ai/cli.go:530-544` and `internal/worktree/worktree.go:263-277` remain untouched. | None |
| Commit state | staged, `commit -a`, empty index | N/A: reason | No commit or worktree index semantics are altered; `internal/run/run.go:628-662` and `internal/worktree/worktree.go:304-310` remain untouched. | None |
| Push state | tracking branch, first push, explicit refspec | N/A: reason | No remote push commands or network VCS operations exist in `lucind-ai`; all operations execute against local worktrees. | None |
| PR commands | explicit `--head`, environment prefix, composed commands | N/A: reason | No PR creation, API calls, or GitHub automation commands exist in `lucind-ai`; PR management is user-owned. | None |

## Rollback and Additivity

**Choice**: Revert commits touching `plugin/claude-code/skills/lucind-ai/SKILL.md`, `plugin/claude-code/skills/lucind-ai/assets/`, and `internal/packet/packet_test.go` via standard `git revert`.
**Alternatives considered**: Binary rollback or database migration reversal. Rejected because no Go production binaries are rebuilt and no database/ledger schemas are modified.
**Rationale**: The change is purely additive documentation, asset templates, and test assertions. Zero schema, ledger, or envelope versions move (`.lucind/result.schema.json`, `internal/ledger/schema.go`, and `internal/packet/packet.go:33-75` remain untouched). Restoring previous git revisions restores prior skill text and template files with zero effect on in-flight packets, existing worktrees, or SQLite ledger databases.

## Out of Scope

- Go binary runtime dispatch modifications in `cmd/lucind-ai/*` and `internal/run/*`.
- Sidecar DAG extension in `internal/dag/*` (`read_only` and fan-out support in `apply-dag.yaml` rejected per Candidate 2).
- Dedicated scaffolding CLI (`lucind-ai fanout` rejected per Candidate 3).
- Go-level word budget parsing or enforcement (remains an editorial/synthesis contract).
- Verification phase dual-dispatch modifications (already frozen under `openspec/specs/verify-dual-dispatch/`).

## Open Questions

- [ ] Template structure: Should templates under `assets/` remain phase-specific (`explore-lens-*.md`, `proposal-lens-*.md`, `design-lens-*.md`) or use a single generalized template family? Recommendation: Maintain phase-specific templates because each phase carries distinct required reading, artifact goals, and slice ownership rules.
- [ ] Operator failure copy: What should `SKILL.md` instruct if a wave-1 lens fails admission or execution? Recommendation: Document a 3-part triage rule in `SKILL.md`: (1) on silent admission failure (empty worktree path), check and fix frontmatter; (2) on execution timeout or crash, re-dispatch the single failing lens lane; (3) on unresolvable blocker, escalate to the operator before wave 2.
