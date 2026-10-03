# herdr-agent-factory

Feature branch: `feature/herdr-agent-factory` (from `dev`). Mirror: Engram topic `odd/herdr-agent-factory/tasks`.
Design record (all decisions, rationale, open items, resume guide): `docs/herdr-agent-factory-design.md`.

## Objective

Turn lucind-ai into a multi-provider agent factory where Claude orchestrates and decides, `agy` runs the heavy lanes, `cursor-agent` runs blind judges, and `herdr` panes are the principal executor. Target usage split: about 60% agy, 15% Cursor, 25% Claude.

## Problem

- Executors launch each CLI with `exec.CommandContext`; there is no herdr integration.
- The packet has no machine-readable verification commands, known failures, route declaration or iteration budget.
- Acceptance receipts bind git tree hashes with plain SHA-256 (no secret), so a test result can be forged or silently repeated.
- SDD is hardwired (`sdd_phase`, `sdd-*` skill derivation, mechanical-check gating) although the owner does not want SDD.
- There is no escalation ladder, no risk tiering of verification, no per-model usage report.

## Scope

In: items T1-T15 below, all inside `lucind-ai`.
Out: forking gentle-ai; ledger schema changes (SDD fields live in `lane_metadata:v1:` event JSON, so none are needed); deleting `SDDPhase`/`FanoutGroup` fields (deferred, accepted-and-ignored); the `feature`, `split`, `integrate` machinery (left intact); any use of Jev with real authority (shadow mode only).

## Constraints

- No SDD. ODD flow: feature doc + task packets. Simple fixes stay inline.
- Worker never runs `git add/commit/push`; the dispatcher commits after green attestation and judges (Conventional Commits).
- Risk tier and hard routing signals are deterministic; classifier failure counts as high.
- Jev receives only numeric signals and boolean flags until its data-retention terms (`docs.typesafe.ai/legal.md`) are read.
- One agy account active at a time (rotation via the owner's `agy-pool`).
- Plugin skill edits need `make bump-plugin-version`; the OpenCode copy must stay byte-identical.
- After binary changes run `make install` (see `CLAUDE.md`).
- Generated artifacts (code, docs, skills) are in English.

## Delivery

Forecast: about 3,000+ authored changed lines across T1-T15, well above the ~400-line budget. Strategy: `feature-branch-chain` (owner's choice, 2026-10-03): each task lands on `feature/herdr-agent-factory` as work-unit commits; slice boundaries (which commits each pull request holds) are recorded under Progress. Each task runs in its own worktree on a `lane/<task>` branch under `~/git_root/lucind-ai-worktrees/` and is integrated into the feature branch after verification.

Execution (owner-authorized 2026-10-03): tasks are delegated to `agy` headless (`--dangerously-skip-permissions`, `--mode accept-edits`) inside herdr panes; the post-run `allowed_paths` diff check is the guard.

## Tasks

Each task closes with a work-unit commit and records its commit id and review tier.

- [x] **T1. HMAC test attestation.** `run-tests` wrapper writes `{command, exit code, git tree hash incl. uncommitted, timestamp}` signed with HMAC to a `chmod 444` log outside the worktree; `verify-attestation` accepts only a matching tree hash. Build on `internal/accept` receipts. Route: delegated writer (2+ non-trivial files). Est. ~350 lines.
  - Route: delegated (agy `gemini-3.8-flash-high`, headless in herdr pane, `--dangerously-skip-permissions`; trigger: 2+ non-trivial files plus new package). Shipped as `lucind-ai attest run|verify` (`internal/attest`, `cmd/lucind-ai/attest.go`) rather than a separate `run-tests` binary.
  - Evidence: worker exit 0, status completed, RED then GREEN reported; orchestrator re-ran `go build ./...`, `go vet`, `go test ./internal/attest/... ./cmd/lucind-ai/...` (ok) and an end-to-end run in a throwaway repo: verify passes after `attest run`, fails with `tree changed` after an edit or an untracked file, passes again after revert, `tests failed` for a failing command, real git index untouched, log file mode 0444. Diff stayed inside the allowed edit surfaces.
  - Commits: lane `87d2676` on `lane/t1-hmac-attestation`; integrated into the feature branch as `880665b` (cherry-pick).
  - Review: assessed `high` (`process_boundary`: `attest.go` starts processes; 1571 lines for the range since `dev`); consent declined for this candidate (owner, 2026-10-03). The accept-time binding of the attestation to `lucind-ai accept` is not wired yet.
  - Follow-ups (not blocking): key creation is not atomic if two first runs race (`os.WriteFile` then `Chmod`); log file is renamed before `chmod 0444`, leaving a short window with default mode; wire `attest verify` into `accept` and the dispatcher commit step (T9).
- [x] **T1b. Attestation hardening and accept wiring.** Closes the T1 follow-ups: atomic key creation, `0444` before rename, `lucind-ai accept` reuses a valid attestation (command `sh lucind-checks.sh`, exit 0, MAC ok, tree hash == frozen candidate tree) and falls back to running the checks; `RepoID` now derives from the git common dir so lane worktrees share one attestation namespace.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: 7 files across 3 packages). Then 2 sequential blind read-only reviewers (`gemini-3.1-pro-high`, `claude-opus-4-6-thinking`) because the task touches keys and processes.
  - Evidence: worker exit 0; orchestrator re-ran build, vet, tests; end-to-end in a throwaway repo (attest in primary, verify passes from a `git worktree`, `tree changed` after an edit, 6 concurrent first runs end with one 32-byte 0600 key). Reviewers agreed on one finding (a creator dying mid-write leaves a partial key file that bricks `LoadOrCreateKey`); fixed by temp file + `os.Link` (no retry loop). Also fixed: `RepoCommonDir` resolves symlinks, `HasValidAttestation` checks `RepoID` explicitly, shared `attestedCheckCommand` const. Regression tests added for each. Full `go build ./... && go test ./...` green on the feature branch (a single unexplained `FAIL` count of 3 appeared once right after `make install` and did not reproduce in 3 reruns).
  - Commits: lane `45f0443`, `f23a5df` on `lane/t1b-attest-hardening`; integrated as `6b82c49`, `aa8a693`.
  - Review: assessed by hand as high (keys, process, accept); RDD is off, so no native review; blind reviewers per the mission. Decision D1 in `docs/overnight-decisions.md`.
- [ ] **T2. Packet contract fields.** Add `verification`, `known_environmental_failures`, `route`, `route_evidence`, injected skills by exact path to `internal/packet` and `packetauthor`; add `interaction_required` to `result.schema.json`. Route: delegated writer. Est. ~300 lines.
- [ ] **T3. Neutralize SDD gates.** Replace `SDDPhase == "" || == "apply"` in `internal/accept/accept.go:120` and `internal/run/attempt.go:391` with a lane-role / `read_only` predicate (fail closed). Route: delegated writer. Est. ~120 lines.
- [ ] **T4. SDD removal, docs and derivation.** Retire the 21 SDD packet templates and `references/strategies/sdd.md`; make `sdd-*` derivation in `internal/skillset` optional; add `odd.md` strategy. Plugin bump. Route: delegated writer. Est. ~400 lines (mostly deletions).
- [ ] **T5. SDD removal, phase command.** Accept-and-ignore `sdd_phase`; remove the `phase` subcommand and `internal/phasespec`. `rg phasespec` before cutting. Route: delegated writer. Est. ~600 lines (deletions).
- [ ] **T6. Worker and explorer skills.** Port the `gentle-ai-worker` rules into `lucind-apply` (edit surfaces, tool safety, TDD discipline, escalate ambiguity; map states to the envelope); rewrite `lucind-fan-out-lens` as an SDD-free explorer-lens skill with structural/textual/historical lenses. Plugin bump. Route: delegated writer. Est. ~250 lines.
- [ ] **T7. Spike: herdr and agy facts.** Verify `herdr worktree open --path` on an externally created worktree, `herdr worktree remove` with live panes, `herdr agent wait` completion detection, exit-code sentinel and stream-json capture, and what `agy --sandbox` restricts. Output: findings note only. Route: delegated explorer (read-only plus scratch). No source writes.
- [ ] **T8. HerdrExecutor.** Implement `executor.Executor` over herdr panes: lucind-ai creates the worktree, herdr opens it, headless CLI runs in the pane with stream-json tee and an exit sentinel. Depends on T7. Route: delegated writer. Est. ~500 lines.
- [ ] **T9. Dispatcher commit step.** After green attestation (T1) and judges, the dispatcher makes the Conventional Commit from the packet message. Depends on T1, T8. Route: delegated writer. Est. ~200 lines.
- [ ] **T10. Risk classifier and judges.** Port a minimal `ClassifyRisk` (path tokens, risk signals, byte-proven passive content; failure is high); map tiers to verification (passive: readback; medium: attestation + 1 judge; high: attestation + 2 blind judges + Claude). Prove `cursor-agent` end to end. Route: delegated writer. Est. ~450 lines.
- [ ] **T11. Dispatch-threshold validator.** Validate the declared route against computable signals (allowed_paths count, new-file flag, risk tier); upgrade or reject mismatches. Route: delegated writer. Est. ~200 lines.
- [ ] **T12. Fan-out and loops.** Parallel independent tasks (cap 3 workers), explorer fan-out with 3 lenses plus an agy synthesizer (about 2k-token handoff), write/test/fix loop (cap 4 iterations), escalation ladder declared in the packet and executed deterministically. Route: delegated writer. Est. ~500 lines.
- [ ] **T13. Usage logging and report.** Per-call JSONL (provider, model, tokens, lane) and a report command against the 60/15/25 target. Route: delegated writer. Est. ~250 lines.
- [ ] **T14. Router interface with Jev in shadow mode.** Router interface; deterministic implementation as baseline and permanent fallback; Jev adapter over plain HTTP (no Go SDK) that logs disagreements to the T13 JSONL and has no authority. Read `docs.typesafe.ai/api.md` and `legal.md` first. Route: delegated writer. Est. ~350 lines.
- [ ] **T15. Rules source and generated files.** Single rules source generating `CLAUDE.md`, `GEMINI.md`, `AGENTS.md` per workspace; workspace `CLAUDE.md` states that delegation goes through the dispatcher. Route: delegated writer. Est. ~200 lines.

Order: T1, T1b, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15.

## Acceptance criteria

- `go build ./...` and `go test ./...` pass at every task close; `lucind-checks.sh` (`go build` plus `go test -race`) passes at task closure.
- Behavior changes follow RED then GREEN with the Go test runner; passive docs and skills use structural readback.
- No new `sdd-*` skill is required for any packet; legacy packets with `sdd_phase` still parse.
- An attestation is rejected whenever the tree hash differs from the working tree.

## Progress

- 2026-10-03: feature doc created on `feature/herdr-agent-factory`. No source changes yet.
- Review tiers and commit ids are recorded per task as they close.
- 2026-10-03: T1 closed (see task). Slice 1 of the `feature-branch-chain` = `880665b`.

## Next step

T2 (packet contract fields and `interaction_required`): delegate to agy in a new worktree `lane/t2-packet-fields` from `feature/herdr-agent-factory`. The T1 worktree `lane-t1-hmac-attestation` is kept until the follow-ups are decided; remove it only after confirming nothing unique remains (its commit was cherry-picked, so the SHA differs).
- 2026-10-03: T1b closed. Slice 1 now = `880665b`, `6b82c49`, `aa8a693`. Engram mirror update pending.
