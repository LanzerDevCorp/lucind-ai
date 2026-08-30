# Design: Deterministic lucind-ai Orchestrator

## Technical Approach

Standardize SDD execution across Claude Code and OpenCode with a two-layer contract (`proposal.md:30-34`, `explore.md:36-39`). Author the orchestrator state machine only under `plugin/claude-code/skills/lucind-ai/` and replicate it byte-identically into `plugin/opencode/skills/lucind-ai/` with parity checks (`specs/deterministic-orchestrator-contract/spec.md:5-26`). The Go binary enforces facts prompts cannot: preflight before worktree allocation, fail-closed admission after late target bind, wave-exit barriers, frozen envelope/diff/porcelain evidence, and idempotent CAS retry (`cmd/lucind-ai/cli.go:95-127`). No new lifecycle states, scheduler, flags, or replacements for Combine/Resolve/Check/bisect/CAS (`proposal.md:14-15`).

## Architecture Decisions

### Decision 1 — Two-layer split: canonical Claude skill, parity-verified OpenCode copy, enforcing runtime

**Choice**: Canonical contract lives only in `plugin/claude-code/skills/lucind-ai/`. `plugin/opencode/skills/lucind-ai/` is a verified byte-identical copy (that tree is absent on this base and is created here). Runtime invariants stay in existing `cmd/lucind-ai`, `internal/packet`, `internal/dag`, `internal/run`, `internal/ledger`, and `internal/result`.
**Alternatives considered**: Skill-only contract; dual-authored divergent skill trees; embedding orchestration policy in Go.
**Rationale**: Prompts cannot enforce filesystem, process, SQLite, or schema truth. Dual trees drift. The binary already owns dispatch (`cli.go:95-127`).

### Decision 2 — Preflight at existing CLI barriers, before any worktree

**Choice**: Extend the existing fail-closed checks at `resolvePrimaryRoot` + `worktree.IsLinkedWorktree` in `runDispatch` (`cli.go:267-280`) and `runFeatureCreate` (`cli.go:791-800`) with skill-parity and embedded-schema freshness. Halt before `worktree.Create` (`run.go:294-296,307-310`). Isolation is the worktree package's job (`worktree.go:1-14`). `lucind-ai check` remains `integrate.Check` / `lucind-checks.sh` (`cli.go:409-411`); no new subcommand.
**Alternatives considered**: Lazy validation at worktree creation; post-dispatch Integrate checks; a dedicated preflight subcommand.
**Rationale**: Worktrees created before a failed preflight orphan directories and burn quota. A new subcommand would be a new CLI verb, which this change does not add.

### Decision 3 — Target-free templates; bind onto dispatched packets; admission stays fail-closed

**Choice**: Author reusable templates without `feature` / `parent_ref` / `base_sha` / `expected_parent_sha` (`packet.go:63-72,114-138`). At wave dispatch the orchestrator writes all four fields onto the packet copies it passes to `lucind-ai run`. `validatePacketAdmission` (`run.go:270-285`) and `FeatureTarget` (`integrate_feature.go:26-77`) keep requiring a complete, homogeneous target; an unbound batch already fails before worktrees (`integrate_feature.go:58-65`). Omitted `allowed_paths` stays undeclared: `DisjointAllowedPaths` skips it (`disjoint.go:24-48`) and `enforceAllowedPaths` runs only when `len(AllowedPaths) > 0` (`run.go:408-410`).
**Alternatives considered**: Hardcoded targets in every template; defaulting omitted targets to `main`; new CLI flags to inject targets; admitting empty packets inside `Execute`.
**Rationale**: Hardcoded targets couple templates to branches. New flags are out of scope. Today's admission already fail-closes on missing targets; late bind is orchestrator fill-in, not a second admission mode.

### Decision 4 — Frozen evidence over narrative; demote on fired hard stops and undeclared paths

**Choice**: Acceptance stays in `internal/run` + `internal/result`, not a new package. `result.Read` validates `.lucind/result.json` against the embedded schema (`result.go:141-162`). `enforceAllowedPaths` demotes undeclared diffs (`run.go:582-654`). `enforceCompletionMode` requires unique commits + clean porcelain for write packets and the inverse for `read_only` (`run.go:663-690`). Extend `decideStatus` (`run.go:549-573`): after a schema-valid envelope, if any `HardStop.Fired` is true, demote to `lane.Blocked` regardless of `envelope.Status` (today `LaneStatus` maps status 1:1 at `result.go:122-135`; schema prose at `internal/result/result.schema.json:35` is not an `if`/`then`). Frozen identity is `Worktree.BaseSHA` and `Attempt.CandidateSHA`.
**Alternatives considered**: Trust executor exit codes or `done_criteria`; a new `internal/accept` package; qualitative approval overriding fired stops.
**Rationale**: Agents have claimed `done` with fired stops or extra paths. Path and porcelain checks already exist; hard-stop demotion is the missing machine check the spec requires (`specs/acceptance-verifier/spec.md:35-39`). The live `acceptance-verifier` capability (merged from `skill-provisioning-and-phase-specialist` after this design was drafted) already states this requirement in prose under "Fail-Closed Mechanical Criteria"; this MODIFIED delta makes the demotion mechanism explicit and machine-verifiable rather than adding new prose.

### Decision 5 — Wave N+1 only after wave N exits 0

**Choice**: The orchestrator runs one `lucind-ai run` per `dag.Split` stdout line (`split.go:18-46`). Advance iff that process exits 0 — every lane `done` and none reverted (`cli.go:361-370`). `dag.Waves` groups by Kahn and `ValidateGlobalOverlap` rejects unordered path overlap (`waves.go:16-72`, `overlap.go:52-79`). In-batch join remains existing `barrier.Evaluate` (`barrier.go:36-60`).
**Alternatives considered**: Speculative downstream waves; skipping failed nodes; a scheduler inside `ExecuteBatch`.
**Rationale**: Wave N+1's worktrees branch from the tree wave N promoted. Partial advancement corrupts the parent.

### Decision 6 — Idempotent attempts, CAS, fail-closed recovery

**Choice**: Reuse `ExecuteAttempt` / `RecoverAttempt` (`attempt.go:217-256,508-570,576-682`) and `IntegrateFeature` (`integrate_feature.go:80-140`). Terminal replay returns the stored row without redispatch. Recovery compares expected vs current parent SHA; mismatch stays `blocked` and preserves worktrees and ledger rows. No schema migration: `schemaVersion` stays 5; `integration_attempts` / `lanes` / `events` columns unchanged (`internal/ledger/schema.go:9,106-120`).
**Alternatives considered**: Blind redispatch; unconditional ref update; deleting worktrees on recovery failure.
**Rationale**: Redispatch wastes quota and is non-deterministic. CAS mismatch must not clobber history.

### Decision 7 — Independent revert of skill, parity, and runtime commits

**Choice**: Revert skill/reference, parity-test, and additive runtime commits independently; keep existing packet, ledger, lifecycle, and CAS behavior; never migrate or rewrite prior evidence (`proposal.md:51-53`).
**Alternatives considered**: Ledger rewrite; monolithic all-or-nothing revert.
**Rationale**: Format changes are additive. Target-free templates remain valid once bound. Reverting runtime restores earlier preflight without touching SQLite.

## Flow and Invariants

```
[1. Preflight] → [2. Bind + Admit] → [3. Split/Waves]
                                          │
[6. CAS / Recover] ← [5. Combine/Check] ← [4. Barrier + Evidence]
```

1. **Preflight.** Primary root resolved and not a linked worktree (`cli.go:267-280`, `worktree.go:278-292`). Skill trees byte-identical; embedded schema current. Break: stale skill/schema still dispatches.
2. **Bind + admit.** Orchestrator fills the four target fields. Parse rejects non-array `allowed_paths` (`packet.go:29,131-137`). Unbound or mixed targets fail before `CreateWorktree` (`run.go:294-296`, `integrate_feature.go:58-65`).
3. **Split/waves.** `dag.Parse` reads `apply-dag.yaml` (`parse.go:47`); `Waves` + `ValidateGlobalOverlap` order and reject unordered overlap; `Split` writes packets and prints `lucind-ai run` lines with no plan file (`split.go:17-46`). Node target fields stay optional (`parse.go:22-37`).
4. **Barrier.** Lanes run in linked worktrees (`worktree.go:168-177`). `barrier.Evaluate` releases when every lane is terminal (`barrier.go:36-60`, `lane/status.go:21-28`); siblings are not cancelled.
5. **Evidence.** Schema-valid envelope (`result.go:141-162`); fired hard-stop demotion (Decision 4); four-way diff vs `AllowedPaths`; porcelain/unique-commit mode (`run.go:402-415,549-691`). `read_only` uses `HasUniqueCommits` / `PorcelainEmpty` (`worktree.go:297-325`).
6. **Integrate.** `integrate.Combine` (`integrate.go:45-47`); checks; `PromoteCAS` only on expected SHA. Retry/recover as Decision 6. Break: stale parent overwrite, redispatch, deleted diagnostics.

## File Changes

| File | Action | Terminal consumer |
|------|--------|-------------------|
| `plugin/claude-code/skills/lucind-ai/` (SKILL + references) | Modify | Orchestrators; `specs/deterministic-orchestrator-contract/spec.md:5-26` |
| `plugin/opencode/skills/lucind-ai/` | Create (byte copy of Claude tree) | OpenCode orchestrator; same spec |
| `internal/packet/packet.go` | Modify | `Execute` admission (`run.go:292-296`); `specs/packet-authoring-contract/spec.md:5-29` |
| `internal/dag/split.go` | Modify | `runSplit` (`cli.go:402`); `specs/sdd-apply/spec.md:5-18` |
| `internal/run/run.go` | Modify | `decideStatus` / path / completion / hard-stop demotion; `specs/acceptance-verifier/spec.md:5-39` |
| `internal/run/batch.go` | Modify | `runDispatch` (`cli.go:132,361-370`) |
| `internal/run/attempt.go` | Modify | `IntegrateFeature` / `feature recover` (`cli.go:741-742`; `specs/parent-feature-integration/spec.md:5-24`) |
| `internal/run/integrate_feature.go` | Modify | `runDispatch` (`cli.go:322-326,345-354`) |
| `cmd/lucind-ai/cli.go` | Modify | Preflight + truthful `integrated_ids`/`reverted_ids` (`cli.go:267-280,542-554`) |

## Interfaces

No new CLI verbs or flags. `packet.Packet` target fields remain optional in templates and required on dispatched packets. `dag.Node` target YAML tags stay `omitempty`. Envelope schema unchanged (`internal/result/result.schema.json`); `HardStop.Fired` becomes a `decideStatus` input, not a schema migration. CLI `run` / `split` / `check` / `feature recover` keep current flags (`cli.go:56,105-119`).

## Testing Strategy and Test Seams

| Layer | What | Seam |
|-------|------|------|
| Unit | Target-free parse; omitted `allowed_paths` skipped | `packet.Parse` (`packet.go:78`) |
| Unit | Claude vs OpenCode byte-identical trees | New tree comparator |
| Unit | Kahn waves + unordered overlap | `dag.Waves` (`waves.go:16`), `ValidateGlobalOverlap` (`overlap.go:54`) |
| Unit | Fired hard stop + `status=done` demotes; undeclared path demotes | `decideStatus` (`run.go:549`), `enforceAllowedPaths` (`run.go:582`), `Deps.WorktreeFS` (`run.go:163`) |
| Unit | Write vs read-only commits/porcelain | `enforceCompletionMode` (`run.go:663`), `Deps.HasUniqueLaneCommits` / `PorcelainEmpty` (`run.go:181-182`) |
| Unit | Terminal replay without redispatch; recovery mismatch blocks | `ExecuteAttempt` (`attempt.go:217-256`), `RecoverAttempt` (`attempt.go:576-682`), `Deps.Ledger` (`run.go:152`) |
| Integration | Bound-target batch at `runDispatch` | `runDispatch` (`cli.go:132`), `depsFactory` (`cli.go:60`) |
| Integration | SQLite WAL under `-race` | `ledger.Open` (`ledger.go:146`), `openTestLedger` (`ledger_test.go:24`) |
| Integration | Combine + checks + CAS | `IntegrateFeature` (`integrate_feature.go:100`), `Deps.PromoteCAS` (`run.go:199`) |
| E2E | Linked-worktree and relative-cwd preflight | `resolvePrimaryRoot` (`cli.go:571`), `cli_test.go` (`cli_test.go:37`) |

Existing seams: `run.Deps` (`run.go:149-212`), `worktree.GitRunner` (`worktree.go:47-49`), `depsFactory` (`cli.go:60`), `resolve.Invoker` (`resolve.go:21`), `io.Reader` to `Parse` (`packet.go:78`). New: inject skill roots/schema bytes at the CLI preflight site; a cross-runtime tree comparator. Consumer tests owned by this change must assert the terminal consumers in File Changes, not producers alone.

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: packet markdown is prompt text; path scope is prefix containment, not executable classification | — | — |
| Git repository selection | Applicable | Authority is `resolvePrimaryRoot`; linked/sibling worktrees fail closed before allocation (`cli.go:267-280`, `worktree.go:278-292`) | Linked worktree refused; relative cwd resolves to primary; absolute sibling rejected |
| Commit state | Applicable | `enforceCompletionMode` / `enforceAllowedPaths` require clean porcelain; write needs unique commits; read-only needs zero (`run.go:663-690`, `worktree.go:297-325`) | Staged leftover fails; `commit -a` with untracked fails porcelain; empty index fails unique commits; read-only with commits fails |
| Push state | N/A: local branches, worktrees, and local CAS only | — | — |
| PR commands | N/A: PR/review are human-owned; binary promotes local refs | — | — |

## Rollback and Additivity

Revert skill, parity-test, and runtime commits independently (`proposal.md:51-53`). No v6 migration. Omitted `allowed_paths` keeps today's skip-checks behavior. Prior ledger rows and envelopes remain readable.

## Open Questions and Out of Scope

Open questions: none.

Out of scope: new lifecycle states, wave engines, CLI flags, or replacements for Combine/Resolve/Check/bisect/PromoteCAS (`proposal.md:14-15`); executor/model/provider/profile selection and semantic approval (`proposal.md:16`); cross-fork orchestration; anything from `feature/skill-provisioning-and-phase-specialist` (`LUCIND_REQUIRED_SKILLS`, `required_skills` frontmatter, `integrate retry`, `defect record/list/resolve/decline/defer`).
