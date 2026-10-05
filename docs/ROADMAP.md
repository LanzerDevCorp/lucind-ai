# lucind-ai: roadmap

What exists is in [`product.md`](product.md).

## Done

- **agy-only contract** (`feature/agy-only-contract`): single provider, `dispatch`/`wait`,
  per-lane JSON state, embedded agy plugin (PreToolUse/Stop hooks), `accept` with allowed-path and
  attestation checks, Claude skill `lucind`. Orchestration, ledger and other providers removed.
  Delivered as a chain of PRs (#5–#11) merged through #12.
- **Leftover cleanup** (`lucind-cleanup`, `chore/remove-packet-leftovers`, #13): static agy model
  list and the other non-blocking leftovers of the contract; packet-era rules, skills, backups,
  stray files and stale cursor-agent proposals removed.
- **Per-lane checks and `lucind-ai install`** (`feature/lane-checks-and-install`, #14): the
  orchestrator picks the attested checks per lane with `--check` (the hardcoded
  `lucind-checks.sh` is deprecated); one flagless `lucind-ai install` sets up the agy plugin and
  the Claude skill.
- **Stop-hook retries** (`feature/lane-stop-retries`): hook payload logging, `wait` revalidates
  the result before reporting `failed`, the retry budget resets after a quiet window and is
  capped, and the stop counters reset on lane continuation. Verified with real probe lanes.
- **golangci-lint** (`chore/golangci-lint`): minimal profile (`errcheck`, `govet`, `ineffassign`,
  `staticcheck`, `unused`) with the reporting caps lifted, repo clean (the capped default hid most of
  the real count), and `golangci-lint run` added to `lucind-checks.sh`. Pass it as
  `--check 'golangci-lint run'` for lanes.
- **`--allow` on lane continuations** (`fix(dispatch)`): a continuation now persists the new globs in
  `lane.json`, which the agy hook and `accept` enforce, instead of only rewriting the brief footer.
- **Per-turn result files** (`feature/turn-bound-results`): a lane is done only when the current
  turn delivered its own result file (`result-<turn>.json`); earlier turns stay as history;
  Stop hook payload logging in `hook.log`; deterministic `TestLaneIDFormat` uniqueness test using
  distinct seconds.
- **Superseded:** the multi-provider herdr work (`herdr-agent-factory`, `herdr-interactive-agents`)
  predates the agy-only contract; its interactive-pane and Stop-hook ideas survive in it.

## Next

1. **Real-lane stability trials.** Run several real lanes end to end (Claude, dispatch, agy, attest,
   accept) across tasks and models; record failures of the Stop-retry loop, timeouts and
   allowed-path denials.
2. **Shell-write escape.** PreToolUse sees file-write tools; shell writes are caught only at
   `accept`. Measure how often it matters before adding anything.
3. **Stale agy trust entries after a crash** in `~/.gemini/antigravity-cli/settings.json`.
4. **Stops from other conversations.** Completion no longer depends on herdr idle (decided by the
   current turn's result file plus the Stop hook). The raw Stop payload carries `conversationId` and
   `transcriptPath`, but no turn id. One real lane showed three conversations, two of them with
   `fullyIdle=true`, and both retries were spent by Stops that arrived before the result existed
   (`hook.log` of lane `20261005-011507-22a2`). Hypothesis, not verified: Stops from sub-conversations
   spend the retry budget and inject the nudge. Next: record the lane's main `conversationId`
   (first PreToolUse or Stop) and ignore Stops from the others; check it over a few real lanes.5. **RTK support.** Install RTK as part of the lucind-ai setup (today it is wired by hand in the
   global Claude config: `@RTK.md` include plus the `rtk hook claude` PreToolUse hook).
6. **Research gentle-ai reviews in depth.** Understand how receipt-driven development (RDD) works
   end to end: review lifecycle, receipts and lineage, consent, correction, and how it interacts
   with lucind-ai lanes and `accept`.
7. **Inject the `lucind:dispatch` block into the global `~/.claude/CLAUDE.md`.** `lucind-ai install`
   should write it (idempotent, between its own markers, outside the gentle-ai ones) so the
   dispatch precedence rules stop being hand-maintained.

## Only if needed

- Cross-machine attestations (today the HMAC key and attestations are local).
- A second provider, only when a real need and a verified hook surface exist.
