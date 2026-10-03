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
- [x] **T2. Packet contract fields.** Add `verification`, `known_environmental_failures`, `route`, `route_evidence`, injected skills by exact path to `internal/packet` and `packetauthor`; add `interaction_required` to `result.schema.json`. Route: delegated writer. Est. ~300 lines.
  - Route: delegated: read-only agy explorer (`gemini-3.8-flash-medium`) mapped the surfaces, then writer (`gemini-3.8-flash-high`, trigger: 20+ files across 6 packages). Shipped: optional frontmatter `route`, `route_evidence`, `verification`, `known_environmental_failures`, `named_skills_only` (+ `skillset.DeriveNamed`), `packetauthor.Contract` fields, result status `interaction_required` with an `interaction` payload (schema if/then; maps to `lane.Blocked`), `decideStatus` reason, plugin docs + bump to 2.0.15.
  - Evidence: orchestrator re-ran build, vet, full `go test ./...`, `make verify-plugin-content verify-opencode-plugin`; proved the pinned legacy digest literal against the BASE source (git archive of the feature branch before T2) so legacy packet digests are unchanged. Orchestrator fixes: gofmt, `route` validation in `packetauthor.validateContract` (+ test).
  - Commits: lane `73179d2` on `lane/t2-packet-fields`; integrated as `73179d2`. No blind review (not process/key/ledger work); RDD off.

- [x] **T3. Neutralize SDD gates.** Replace `SDDPhase == "" || == "apply"` in `internal/accept/accept.go:120` and `internal/run/attempt.go:391` with a lane-role / `read_only` predicate (fail closed). Route: delegated writer. Est. ~120 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: 8 files in 3 packages). Single predicate `LaneMetadata.RequiresMechanicalChecks()` (false only for `ReadOnly` or roles lens/synthesis/verify/archive/human; fail closed otherwise) used by `accept` and `shouldRunAttemptChecks`; `LaneMetadata` gained `lane_role`/`read_only` (omitempty, no schema change); both metadata writers populate them.
  - Evidence: orchestrator re-ran build, vet, full `go test ./...` in the lane and on the feature branch. Behavior change recorded as D3.
  - Commits: lane `d5d07f2`; integrated as `d5d07f2`. No blind review; RDD off.

- [x] **T4. SDD removal, docs and derivation.** Retire the 21 SDD packet templates and `references/strategies/sdd.md`; make `sdd-*` derivation in `internal/skillset` optional; add `odd.md` strategy. Plugin bump. Route: delegated writer. Est. ~400 lines (mostly deletions).
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: 50+ files incl. deletions in two plugin copies). Removed 21 SDD packet templates and `strategies/sdd.md` from both copies, added `strategies/odd.md`, router/SKILL.md rows, `skillset.Derive` now adds `sdd-apply|verify|archive` only with an explicit `sdd_phase` (role-only apply now derives `lucind-apply` + `lucind-executor`; decision D4), plugin bumped to 2.0.16.
  - Evidence: orchestrator re-ran build, vet, full tests, `make verify-plugin-content verify-opencode-plugin`; checked no dangling plugin references (fixed one in `strategies/fan-out.md` by hand, both copies, then re-ran the bump); read back `odd.md` and the SKILL.md diff.
  - Commits: lane `85b4a24`; integrated as `85b4a24`.

- [x] **T5. SDD removal, phase command.** Accept-and-ignore `sdd_phase`; remove the `phase` subcommand and `internal/phasespec`. `rg phasespec` before cutting. Route: delegated writer. Est. ~600 lines (deletions).
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: deletion of a package plus surgical edits to the large `cli.go`). `phasespec` had no consumers beyond `cmd/lucind-ai/cli.go` and `cli_test.go` (checked with `rg`). Removed the `phase` subcommand, usage line, `internal/phasespec`, `docs/sdd-phase-specialist.md`; ADR 0002 marked superseded. `SDDPhase`/`FanoutGroup` fields and `sdd_phase` parsing untouched.
  - Evidence: orchestrator re-ran build, vet, full tests; `lucind-ai phase propose` now exits non-zero as an unknown command; `rg phasespec cmd internal` empty.
  - Commits: lane `10ef695`; integrated as `10ef695`.

- [x] **T6. Worker and explorer skills.** Port the `gentle-ai-worker` rules into `lucind-apply` (edit surfaces, tool safety, TDD discipline, escalate ambiguity; map states to the envelope); rewrite `lucind-fan-out-lens` as an SDD-free explorer-lens skill with structural/textual/historical lenses. Plugin bump. Route: delegated writer. Est. ~250 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: skill-document rewrite that needs reading the reference rules). Ported the worker rules into `.agents/skills/lucind-apply/SKILL.md` (edit surfaces, tool safety, TDD lifecycle, verification and known failures, `interaction_required` contract, envelope mapping) and rewrote `.agents/skills/lucind-fan-out-lens/SKILL.md` as an SDD-free read-only explorer with structural/textual/historical lenses and a synthesis protocol.
  - Evidence: structural readback of both files; `rg -i sdd` on both returns nothing; envelope field names checked against `result.schema.json` with `jq`; build, full tests and `make verify-plugin-content verify-opencode-plugin` green. No plugin bump was needed: `.agents/skills` is outside the hashed plugin tree (the verify target passed unchanged).
  - Commits: lane `3fe1e8c`; integrated as `3fe1e8c`.

- [x] **T7. Spike: herdr and agy facts.** Verify `herdr worktree open --path` on an externally created worktree, `herdr worktree remove` with live panes, `herdr agent wait` completion detection, exit-code sentinel and stream-json capture, and what `agy --sandbox` restricts. Output: findings note only. Route: delegated explorer (read-only plus scratch). No source writes.
  - Route: inline by the orchestrator (read-only plus throwaway repos/panes created for the spike; no source writes). Findings: `docs/herdr-spike-findings.md`. Key facts: `worktree open --path` needs `--cwd <repo>`; `worktree remove` kills live panes, refuses dirty trees, leaves the branch; `agent wait` is unreliable for headless runs (use the exit sentinel); stream-json final `result` equals the json output; usage has tokens but no cost; `--sandbox` made the add-dir read-only and did not restrict network or other writes.
  - Commit: `06d1a00` (docs only, directly on the feature branch).

- [x] **T8. HerdrExecutor.** Implement `executor.Executor` over herdr panes: lucind-ai creates the worktree, herdr opens it, headless CLI runs in the pane with stream-json tee and an exit sentinel. Depends on T7. Route: delegated writer. Est. ~500 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: new file + process boundary + plugin doc). Shipped `executor.HerdrAgy` registered as packet executor `herdr-agy` (same agy invocation as `executor.Agy`, run through a generated `run.sh` in a herdr pane; `worktree open --cwd --path`; nonce exit sentinel; `pane wait-output --regex`; C-c on timeout; stream-json progress polling), plugin executors doc rows, plugin bumped to 2.0.17.
  - Evidence: orchestrator re-ran build, vet, `go test -race` on executor/cmd, full suite, plugin checks; real end-to-end with real `herdr` and a fake `agy` script in a throwaway repo (prompt with quotes, `$()` and backticks arrives byte-identical; exit code 3 propagates; state dir kept on failure, removed on success). Found and fixed by hand: real herdr errors are `{"error":{"code","message"}}` objects (the writer's fake used a string), non-timeout `wait-output` failures were treated as timeouts.
  - Blind review (`gemini-3.1-pro-high`, `claude-opus-4-6-thinking`, sequential, read-only; worktree unchanged): both found the unparseable `exit.code` => success; one found `cd` failure falling through to run agy in the wrong directory. Both fixed with tests. Accepted risks and deferred items: see D5 and T16.
  - Commits: lane `dc3b55b`, `9633b37`; integrated as the two latest executor commits ending at `f721ce8`.

- [x] **T9. Dispatcher commit step.** After green attestation (T1) and judges, the dispatcher makes the Conventional Commit from the packet message. Depends on T1, T8. Route: delegated writer. Est. ~200 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: 18 files across 6 packages incl. git and process boundary). Shipped: `attest.RunAndRecord` (CLI `attest run` is now a thin caller), packet/contract field `commit_message` (Conventional Commit header, no AI trailers, requires `verification`), `dispatcher` commit obligation in packetauthor and accept, `run/commit_step.go` (`dispatcherCommit`: worker must not have committed, non-empty change set, attested verification under `sh -c`, tree-hash gate, optional `PreCommitGate` seam for T10, `git add -A` + unstage `.lucind`, commit), plugin docs + bump 2.0.18.
  - Evidence: orchestrator re-ran build, vet, full tests, `go test -race` on the touched packages, plugin checks; found and fixed a writer test bug (SHA taken from another fixture); real default-path e2e (real attestation + real git commit, verification output on stderr); `git add -A` + `git reset -- .lucind` verified in both the ignored and untracked cases (the exclude pathspec errors on an ignored dir).
  - Blind review (`gemini-3.1-pro-high`, `claude-opus-4-6-thinking`; worktree unchanged): both flagged (1) a verification command editing the tree gets its side effects attested and committed => fixed (tree hash compared before/after, fail closed) and (2) hooks running with dispatcher authority => fixed (`--no-verify`, D7); one flagged accept allowing `envelope.Commit == candidate` => fixed (must be empty). Deferred as T17.
  - Commits: lane `1eb4670`, `a82b7e0`; integrated as the two commits ending at `bed8d65`.

- [x] **T10. Risk classifier and judges (classifier and tier mapping only).** Port a minimal `ClassifyRisk` (path tokens, risk signals, byte-proven passive content; failure is high); map tiers to verification (passive: readback; medium: attestation + 1 judge; high: attestation + 2 blind judges + Claude). Prove `cursor-agent` end to end. Route: delegated writer. Est. ~450 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: new package). Scope tonight (mission): the classifier and the tier-to-verification mapping only. Shipped `internal/risk`: `Classify(Input) Result` (path tokens auth|update|security|webhook|payments, sensitive locations, Go process/network/permission/deletion signals by quoted-import and call substrings, deletions and agent-instruction content as medium floor, passive only for byte-proven ordinary docs/images, everything else medium, any failure/empty/panic high) and `PlanFor(Tier) Plan` (passive readback; medium attestation + 1 judge; high attestation + 2 blind judges + Claude; unknown tier = high). Not wired into run/accept.
  - Evidence: orchestrator re-ran build, vet, `go test -race`, full suite; found and fixed a writer over-match (`net` substring matched ordinary words; now the quoted import literal) with a near-miss test; classified real repo commits as a sanity check (attest hardening: high; gates change: high; skills: medium; docs-only: passive).
  - Commit: lane `5563143`; integrated as `5563143`. The `cursor-agent` end-to-end proof and judge execution are carved out into T18.

- [x] **T11. Dispatch-threshold validator.** Validate the declared route against computable signals (allowed_paths count, new-file flag, risk tier); upgrade or reject mismatches. Route: delegated writer. Est. ~200 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: new package + CLI wiring + plugin doc). Shipped `risk.ClassifyPaths` (path-only tiers, never passive), `internal/dispatchcheck` (`ComputeSignals`, `Check` returning accept/upgrade/reject), wired into `lucind-ai run` before admission via `validateDispatchThresholds` (git `cat-file` against the packet base or HEAD; locale pinned): `inline` + 2+ paths / new file (or dir/glob) / high tier => upgraded to `worker` (printed), `worker`/`fanout` need `route_evidence`, `fanout` needs a read-only lane or 2+ paths, packets without `route` untouched. Plugin `odd.md` documents it; bumped to 2.0.19.
  - Evidence: orchestrator re-ran build, vet, `go test -race`, full suite, plugin checks; real binary rejects a `route: worker` packet without evidence with exit 1 before any dispatch; checked that admission keeps the manual packet's `Route` (the contract-copy block only runs for typed contracts).
  - Commits: lane `ebcd33c`; integrated as `ebcd33c`. Decision D9.

- [ ] **T12. Fan-out and loops.** Parallel independent tasks (cap 3 workers), explorer fan-out with 3 lenses plus an agy synthesizer (about 2k-token handoff), write/test/fix loop (cap 4 iterations), escalation ladder declared in the packet and executed deterministically. Delivered in three slices, each its own work unit:
  - [x] **T12a. Parallel lane cap.** `ExecuteBatch` currently starts every lane at once with no limit; add `Deps.MaxParallelLanes` (default 3) and a `--max-parallel` flag. Est. ~120 lines.
    - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: concurrency change in `run` + CLI). `Deps.MaxParallelLanes` (default `DefaultMaxParallelLanes = 3`) with a counting semaphore in `ExecuteBatch`, `--max-parallel <n>` (n >= 1). Queued lanes do not burn their own timeout, report order is preserved, cancelled queued lanes are recorded as failed.
    - Evidence: orchestrator re-ran build, vet, full suite and `go test -race` on run/cmd; real binary rejects `--max-parallel 0` and `-3`; fixed by hand: queued-lane failure persistence used the cancelled ctx (now `context.WithoutCancel`), with a ledger assertion proven by mutation (fails without the fix).
    - Commit: lane `be120f5`; integrated as `be120f5`. Decision D10.

  - [x] **T12b. Write/test/fix loop and escalation ladder.** Packet fields `max_iterations` (default 1 = today's behavior, cap 4) and `escalation` (ordered executor/model rungs); the dispatcher re-dispatches the worker with the failing verification output, then climbs the ladder deterministically; exhaustion ends blocked. Depends on T9. Est. ~350 lines.
    - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: 19 files in 5 packages, core dispatcher logic). Shipped packet/contract fields `max_iterations` (1..4) and `escalation` (<= 3 rungs; both need `verification` AND `commit_message`), `internal/run/loop.go` (attempt plan, total cap 4, feedback), `dispatcherVerify` split from the commit step, `Deps.RunAttested` now returns the output tail, ladder rung validation in the CLI, `Report.Attempts`, plugin docs bumped to 2.0.20.
    - Evidence: orchestrator re-ran build, vet, full suite, `go test -race` on packet/packetauthor/run/cmd, plugin checks; real default-path e2e (real attestation + git; the real failing output reached the 2nd prompt; commit made; lane done after 2 attempts). Fixed by hand and proven by mutation: stale previous-attempt result envelope and stale verification result.
    - Blind review: reviewer A (`gemini-3.1-pro-high`) found 5 issues, all reproduced and fixed with tests (loop packets without `commit_message` could end done with an uncommitted tree => now rejected at parse/compile; context ended between attempts kept the last failure status => now blocked with a reason; hostile verification output could close the markdown fence and was unbounded => longer fence + 8 KiB cap; a failed envelope removal was ignored => lane fails; progress errors of earlier attempts were dropped => accumulated). Reviewer B (`claude-opus-4-6-thinking`) first hit 429 quota (D11), then reviewed both T12b commits after the Claude/GPT bucket reset (14:06Z): no high-severity finding; the two medium items it reported (use of `ctx` instead of `persistCtx` in the verification and completion-mode paths) are patterns that already existed before this change and were not applied.
    - Commits: lane `7f12d0e`, fix commit on the lane; integrated as the two commits ending at `f919168`.

  - [x] **T12c. Explorer fan-out.** `lucind-ai explore`: 3 read-only lens lanes (structural, textual, historical) in parallel, then one synthesis lane fed with the bounded lens outputs; prints the handoff. Depends on T12a, T6. Est. ~350 lines.
    - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: new package + CLI + refactor of the dispatch pipeline). Shipped `internal/explorefan` (validation, deterministic lens and synthesis packets, backtick-safe fences, tail truncation) and `lucind-ai explore --objective --scope --id --timeout`: three read-only lens lanes in one parallel batch, then one synthesis lane fed with the bounded lens outputs; the handoff is printed and written to `handoff.md`. `runDispatch` was refactored into `executePacketBatch` (same output and exit codes) so both batches reuse the whole pipeline. `docs/explore.md`.
    - Evidence: orchestrator re-ran build, vet, full suite, `-race`; real end-to-end with a fake `agy` on PATH in a throwaway repo: 3 lens lanes done, synthesis packet with 3 `## Lens:` sections, handoff printed and saved, primary HEAD unchanged. The e2e found two real bugs the unit tests missed, both fixed by hand: (1) `routed_by: explore` equals an SDD phase name, so admission derived the missing skill `sdd-explore` (now `explorer-fanout`, with a test); (2) the worker had replaced `KnownModels()` of agy and herdr-agy in the CLI with a 2-model list, hiding that the real `Agy.KnownModels()` only allowed `gemini-3.7-flash-high` (decision D17: the real list now has the design's 5 models, wrappers removed).
    - Commits: lane `52c9549` (agy models), `7aa93ac` (explore); integrated as the same two commits (the feature branch had not moved), head `7aa93ac`.

- [x] **T13. Usage logging and report.** Per-call JSONL (provider, model, tokens, lane) and a report command against the 60/15/25 target. Route: delegated writer. Est. ~250 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: new package + dispatcher wiring + CLI). Shipped `internal/usagelog` (append-only JSONL at `$XDG_STATE_HOME/lucind-ai/usage.jsonl`, 0600; `ExtractUsage` from agy/claude-style stdout JSON, fallback to progress events, else `tokens_known=false`; tolerant reader that skips and counts corrupt lines), one record per executor attempt in `run.Execute` (`Deps.RecordUsage` seam), `lucind-ai usage report [--since] [--file] [--json]` with the 60/15/25 comparison, `docs/usage-log.md`.
  - Evidence: orchestrator re-ran build, vet, full suite and `-race` on usagelog/run/cmd; real binary report on a fixture (shares match hand calculation: 357630/517630 = 69.1%); found by hand that the cmd tests wrote 18 records of fake lanes into the real state file (the run package had its own guard, cmd did not): added an `init()` guard (`LUCIND_USAGE_LOG=off`) in `cmd/lucind-ai/usage_test.go`, removed the polluted file (every record was a test lane/model) and verified two full test runs create no file.
  - Commit: lane `47bd901`; integrated as `47bd901`. Limitation documented: orchestrator Claude usage outside `claude` lanes is not visible.

- [x] **T14. Router interface with Jev in shadow mode.** Router interface; deterministic implementation as baseline and permanent fallback; Jev adapter over plain HTTP (no Go SDK) that logs disagreements to the T13 JSONL and has no authority. Read `docs.typesafe.ai/api.md` and `legal.md` first. Route: delegated writer. Est. ~350 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: new package + usagelog extension + CLI wiring + network client). Read the public Jev docs first (D15). Shipped `internal/router` (`Signals` = numbers and booleans only, enforced by a reflection test; `Router`; `Deterministic` baseline and permanent fallback; `Jev` plain-HTTP adapter with typed errors, 1 MiB body cap, https required except loopback, no retries; `Shadow` that always returns the primary decision and logs `router_disagreement`/`router_error`), `usagelog` router events (ignored by usage totals), opt-in wiring in the dispatch-threshold validator (needs BOTH `LUCIND_JEV_API_KEY` and `LUCIND_JEV_SHADOW=on`), `docs/router.md`.
  - Evidence: orchestrator re-ran build, vet, full suite, `-race`; real CLI e2e against a loopback fake Jev server: the request carried only `{allowed_path_count, new_file, risk_tier_level, read_only}`, a disagreement was logged, the validator verdict stayed authoritative, and with one env var no request was made. Hardened by hand: HTTP clients never follow redirects (the bearer key must go only to the validated URL; test with a redirecting server), `NewJev` failures are surfaced on stderr without the key. No real call to api.typesafe.ai was made.
  - Commit: lane `2cd9e78`; integrated as `2cd9e78`. Open owner item: Jev's retention terms (Data Processing Agreement) not read (D15); declared fuzzy signals deferred to T19.

- [x] **T15. Rules source and generated files.** Single rules source generating `CLAUDE.md`, `GEMINI.md`, `AGENTS.md` per workspace; workspace `CLAUDE.md` states that delegation goes through the dispatcher. Route: delegated writer. Est. ~200 lines.
  - Route: delegated writer (agy `gemini-3.8-flash-high`; trigger: new package + CLI + docs). Shipped `internal/rules` and `lucind-ai rules init|generate [--root] [--check]`: one versioned source `lucind-rules.md` with audience-tagged sections renders `CLAUDE.md` (orchestrator + all; contains the sentence "Delegation goes through the dispatcher"), `GEMINI.md` and `AGENTS.md` (worker + all). Safety: refuses roots inside `~/.claude` or `~/.gemini` (symlinks resolved), never touches hand-written files or symlink targets (reports `skipped_hand_written`), atomic writes, `--check` for staleness. `docs/agent-rules.md`.
  - Evidence: orchestrator re-ran build, vet, full suite, `-race`; global `~/.claude/CLAUDE.md` and `~/.gemini/GEMINI.md` mtimes unchanged; real CLI e2e in temp repos (hand-written CLAUDE.md skipped, check exit codes 0/1, forbidden root refused, delegation sentence only in the generated CLAUDE.md); fixed by hand a `..foo` directory-name hole in `isInsideOrEqual` with a test.
  - Commit: lane `218053c`; integrated as `218053c`. This repo's own CLAUDE.md is hand-written and stays untouched (decision D14).

- [ ] **T16. HerdrAgy lifecycle: hard stop and cleanup.** Found by the T8 blind review: (a) after a timeout the C-c grace period can expire and the pane process keeps running (no hard-kill fallback); (b) state dirs `$XDG_STATE_HOME/lucind-ai/herdr/run-*` are kept on failure and never reaped; (c) `pane wait-output` followed by reading `exit.code` has no cross-check that agy really exited. Add a bounded hard-stop policy (second C-c, then close only the pane/workspace that this executor itself opened, never reused ones) and a reaper for old run dirs with a retention window. Route: delegated writer. Est. ~200 lines. Depends on T8.
- [x] **T17. Accept re-verifies dispatcher-commit candidates.** From the T9 review: `accept` trusts the frozen evidence for the `dispatcher` obligation and does not re-check that every declared `verification` command has a valid attestation for `CandidateTree`. Add that check (reusing `attest.HasValidAttestation` and the injectable seam in `accept.Verifier`), failing closed when the attestation is missing, and decide how it interacts with the `lucind-checks.sh` reuse path. Route: delegated writer. Est. ~150 lines. Depends on T9.
  - Route: inline (small, security-critical, ~60 lines; recorded deviation from the planned delegated writer). RED: 3 tests failed before the change. GREEN: `requireDispatcherAttestations` in `internal/accept/accept.go` checks each `verification` command in the frozen contract with `hasAttestation` against `CandidateTree`; no commands, lookup error or missing attestation all reject (fail closed). Independent from the `lucind-checks.sh` reuse path, which is unchanged. Full `go test ./...` green. Commit `ab515df`.
- [ ] **T18. Judges via cursor-agent, end to end.** Carved out of T10: execute blind judges for the tier plan from `internal/risk` (two different model families, same frozen target, only findings both confirm are fixed, at most two fix rounds), prove `cursor-agent` end to end in lucind-ai (never run so far), and plug the judge runner into the `PreCommitGate` seam of the dispatcher commit step (T9). Not tonight (no Cursor). Route: delegated writer. Est. ~450 lines. Depends on T9, T10.
- [x] **T19. Declared fuzzy signals for the router.** From T14: the design splits router inputs into computable signals (done) and signals declared by the orchestrator (the fix is understood, open design, estimated lookups). Add them as optional packet frontmatter (`understood`, `open_design`, `estimated_lookups`), parse/validate/digest them like the T2 fields, and feed them to `router.Signals` and the dispatch-threshold validator (T11). Route: delegated writer. Est. ~200 lines. Depends on T14.
  - Route: inline. Manual-packet frontmatter `understood`, `open_design`, `estimated_lookups` parse/validate (typed errors), enter the packet digest, upgrade an `inline` route to `worker` in the threshold validator (reasons `understood:false`, `open_design:true`, `estimated_lookups:N` for N>5), and feed `router.Signals` (deterministic baseline and Jev shadow). Documented in both plugin trees. Not done: typed-contract (`packetauthor`) pass-through of the three fields; tracked as T23. Commit `635d55d`.
- [x] **T20. Ledger concurrency tests under load.** Found while running the full suite with `-race` repeatedly: `TestConcurrentOpenOnFreshDatabase` and `TestConcurrentProgressAndSetStatus` (internal/ledger) failed once with SQLITE_BUSY (`busy_timeout` is 5 s) when all packages ran in parallel under `-race`; they pass in isolation and the ledger code was not changed by this work (only a metadata field was added in T3). Decide whether to raise the busy timeout, bound test parallelism or make those tests retry. Route: inline or one delegated writer after reproducing. Est. ~50 lines.
  - Route: inline. Reproduced under repeated `-race` runs on a loaded machine: `database is locked` from `Open`/`AppendProgress` and 3 of 12 progress rows lost to errors. Fix: `busy_timeout` 5 s -> 30 s (DSN and the pragma verification constant). After the fix 30/30 runs passed. Commit `627fe7b`.

- [x] **T21. Run Jev in shadow mode for real.** The adapter (T14) is verified against the live API (`86b6f0b`, opt-in live test). Make shadow mode usable day to day: accept `JEV_API_KEY` as a fallback for `LUCIND_JEV_API_KEY` (env only, never read from files or logged), keep Jev non-authoritative (the deterministic decision always wins), log every `router_disagreement` and `router_error` to the usage JSONL, and add a `usage report` section that summarizes agreement rate and confidence of disagreements so the owner can judge whether Jev should ever influence routing. First live evidence: Jev answered `inline` (0.45) for 4 paths + new file + high risk where the deterministic router says `worker`; T19 signals may fix that. Route: delegated writer. Est. ~150 lines. Depends on T14; benefits from T19.
  - Route: inline. Shadow now logs `router_agreement` as well as disagreements/errors; `usage report` shows agreement rate and mean confidence of disagreements; `JEV_API_KEY` is accepted when `LUCIND_JEV_API_KEY` is empty (env only, test proves key not leaked to stderr). Live-verified against the real API earlier (`86b6f0b`). Enable with `LUCIND_JEV_SHADOW=on` plus a key in the environment. Commit `3f2f6b4`.

- [ ] **T22. Read-only lanes should skip integration.** Found by the real `lucind-ai explore` run in a sandbox repo: the integrate step attempted to integrate the read-only synthesis lane and reported it as `reverted` (no `lucind-checks.sh` in the repo), which is noise and looks like a failure. Read-only lanes carry no commits, so integration should be skipped for them. Also record the fixes the same run forced: derived required skills were never shown to the worker (`fix(run)`), and a repo needs `.lucind/skill-roots.yaml` (document it). Route: inline. Est. ~60 lines.

- [ ] **T23. Declared router signals in typed packets.** T19 added `understood`, `open_design`, `estimated_lookups` to manual packet frontmatter only. Pass them through the typed contract (`internal/packetauthor` contract + compile + `cmd/lucind-ai/packet_authoring.go`) so typed packets can declare them too. Route: inline. Est. ~60 lines.

Order: T1, T1b, T2, T3, T4, T5, T6, T7, T8, T9, T10, T11, T12, T13, T14, T15, T16, T17, T18, T19, T20, T21, T22, T23.

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
- 2026-10-03: T12c closed (`7aa93ac`).
- 2026-10-03: T14 closed (`2cd9e78`).
- 2026-10-03: T15 closed (`218053c`).
- 2026-10-03: T13 closed (`47bd901`).
- 2026-10-03: T12b closed (`f919168`), review B pending (quota).
- 2026-10-03: T12a closed (`be120f5`).
- 2026-10-03: T11 closed (`ebcd33c`).
- 2026-10-03: T10 closed (`5563143`); T18 carved out.
- 2026-10-03: T9 closed (`bed8d65`). T17 added from the T9 review findings.
- 2026-10-03: T8 closed (`f721ce8`). T16 added from the T8 review findings.
- 2026-10-03: T7 closed (`06d1a00`).
- 2026-10-03: T6 closed (`3fe1e8c`).
- 2026-10-03: T5 closed (`10ef695`).
- 2026-10-03: T4 closed (`85b4a24`).
- 2026-10-03: T3 closed (`d5d07f2`).
- 2026-10-03: T2 closed (`73179d2`).
- 2026-10-03: T1b closed. Slice 1 now = `880665b`, `6b82c49`, `aa8a693`. Engram mirror update pending.
