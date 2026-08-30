# Synthesis Notes: Deterministic lucind-ai Orchestrator

## Unresolved Contradictions

None.

## Coverage Gaps

- Spec Frozen Evidence names tree hashes. No lens specified a tree-hash type or consumer. This worktree has `Worktree.BaseSHA` and `Attempt.CandidateSHA` only. Not invented; design uses those commit SHAs as frozen identity.
- Proposal in-scope TypeScript wrapper (`lucind-ai.ts` / `process.mjs` / `install.sh`). `plugin/opencode/` is absent on this base and no lens designed those files. Not invented; design creates only the skill-tree copy A required.
- `sdd-design` SKILL.md size budget is 800 words and Step 4/5 persist to Engram plus a return block. This packet sets 1800 words, two files, and `.lucind/result.json`. Process drift only; all eight spine items fit under 1800. Skill required decision shape (choice / alternatives / rationale) and threat-matrix Applicable/N/A rule are followed.

## Dropped Citations

Every `file:line` below was opened in this worktree. Claims that used them are out of `design.md`.

- **Lens A Decision 4 / Lens B Envelope row / Lens C hard-stop unit row as current auto-demotion on `HardStop.Fired`.** `result.LaneStatus` maps `envelope.Status` 1:1 (`internal/result/result.go:122-135`). `result.Read` (`:141-162`) validates schema structure only. `internal/result/result.schema.json:35` says fired stops must be `blocked` in description text; there is no JSON Schema `if`/`then` and no Go loop over `HardStops[i].Fired`. Path demotion at `enforceAllowedPaths` (`internal/run/run.go:582-654`) is real and kept. Hard-stop demotion stays in design as a `decideStatus` extension required by `specs/acceptance-verifier/spec.md:21-25`, not as existing behavior of the cited lines.
- **Lens A `plugin/claude-code/skills/lucind-ai/SKILL.md:1-8` as the executing contract.** Lines 1-8 are YAML frontmatter (`name`, `description`, `license`, `metadata`). The skill body starts at line 10. Canonical skill path is kept; that range is not the consumer.
- **Lens B `internal/worktree/worktree.go:278-292` for "primary root is clean".** Range is `IsLinkedWorktree` only. Clean porcelain is `PorcelainEmpty` at `:319-325`.
- **Lens B `internal/dag/parse.go:40-43` as `dag.Parse`.** Range is `type DAG struct`. `Parse` starts at `:47`.
- **Lens B `internal/dag/split.go:18-46` "with bound targets".** `Split` prints `lucind-ai run --packet …` (`:40`); it does not write `feature` / `parent_ref` / `base_sha` / `expected_parent_sha`.
- **Lens B `internal/run/attempt.go:66-77` as idempotent replay / no-redispatch retry.** Range is the `Attempt` struct (`ID`, `IdempotencyKey`, `Status`, …). Replay is `ExecuteAttempt` at `:233-255`.
- **Lens B `.lucind/result.schema.json:1-159`.** Path does not exist in this worktree (runtime file written from embed). Authoritative schema is `internal/result/result.schema.json`. Same file does not machine-enforce `Fired` → `blocked`.
- **Lens B `internal/result/result.go:30-34` as demotion on fired hard stops.** Range is the `HardStop` struct definition.
- **Lens C `internal/run/attempt.go:245` (`recoverAttemptInternal`).** Line 245 is `getAttemptByIdempotencyKey` inside `ExecuteAttempt`. `recoverAttemptInternal` starts at `:593`.

No lens citation resolved only on `feature/skill-provisioning-and-phase-specialist`. This worktree's `cmd/lucind-ai/cli.go` usage lists `run`, `split`, `check`, `serve`, `feature`, `reconcile`, `worktree` — not `integrate retry` or `defect record/list/resolve/decline/defer`. No `LUCIND_REQUIRED_SKILLS` or `required_skills` symbol in `cmd/lucind-ai/cli.go` or `internal/`. `internal/accept/`, `internal/run/integrate_retry.go`, and `internal/ledger/acceptance.go` are absent.

## Architecture Divergence

Lens A's assumed architecture is the one in `design.md`: two-layer Claude canonical skill plus byte-identical OpenCode copy; extend existing `cmd/lucind-ai`, `internal/packet`, `internal/dag`, `internal/run`, `internal/ledger`, `internal/result`; no new lifecycle, scheduler, or routing engine. A's architecture was not refuted by the code on this base.

**Independent convergence.** B and C each opened with the same two-layer split, byte-identical OpenCode copy, fail-closed preflight, late target binding, omitted-`allowed_paths` skip-checks, orchestrator wave barriers, frozen envelope evidence, and idempotent CAS retry without new lifecycle/scheduler/flags. That overlap is corroboration.

**Lens B — new `internal/accept` package.** B assumed a dedicated `internal/accept` consolidating commits, tree hashes, envelopes, and clean worktrees, and listed `internal/accept/accept.go` as Create. That package does not exist here. A (and C) put frozen evidence in existing `internal/run` (`decideStatus`, `enforceAllowedPaths`, `enforceCompletionMode`) and `internal/result`. Cost: B's Create row, flow step 5 `internal/accept → internal/ledger`, and any "tree hash" consumer B implied. Not in `design.md`.

**Lens B — Split binds targets.** B's surface delta said `dag.Split` emits commands "with bound targets". A binds at wave dispatch onto packet copies; today's `Split` only prints `--packet` paths. Cost: that delta sentence.

**Lens B — `validatePacketAdmission` admits target-free templates.** Live `validatePacketAdmission` (`run.go:270-285`) still rejects missing targets. A consumes it after bind, not as a new empty-packet mode. Cost: reading B's delta as a current admission change.

**Lens C — `Deps` / `FeatureTarget` as the late-bind mechanism.** C's assumed architecture extends `internal/run` with "`FeatureTarget` late binding". Live `FeatureTarget` (`integrate_feature.go:26-77`) validates homogeneity and fail-closes on an all-empty batch (`:58-65`); it does not inject targets. A binds in the orchestrator, then existing admission. Cost: C's integration test wording "runtime-supplied target" if read as new CLI flags (forbidden) or as a `Deps` field. Kept as: tests drive already-bound packets through `runDispatch`.

**Lens C — packages extended vs reused.** C extends `cmd/lucind-ai`, `internal/packet`, `internal/run` and reuses `internal/dag`, `internal/ledger`, `internal/integrate`, `internal/worktree`. Compatible with A; `internal/result` (A) is the envelope owner C already cited via `decideStatus`. No content dropped for that difference.

**Open questions closed by A, not left open.** C asked whether preflight lives in `runDispatch` / `lucind-ai check` or a new subcommand — Decision 2: existing dispatch barriers; `check` stays `lucind-checks.sh`. B and C asked about the skill's 800-word/Engram design phase — packet process, not this change.
