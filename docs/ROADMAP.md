# lucind-ai: roadmap

What exists is in [`product.md`](product.md).

## Done

- **Fail closed on auto-skills selection error** (`feature/auto-skills-fail-closed`): dispatch with
  `--auto-skills` fails closed (exit code 5) without creating or mutating any lane state when skills
  cannot be selected; orchestrator has clean fallback to hand-written skills section; API keys stay
  redacted; updated auto skill variant with exit 5 guidance.
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
- **Real-lane stability trials, first round** (throwaway repo `lucind-probes`, one worktree per
  probe, agy `Gemini 3.8 Flash (High)`): 6 probes, 7 completed turns, plus the implementation lanes of
  `turn-bound-results`. Worked as designed: a single turn (`result-1.json`); a continuation after
  `done` (turn 2, widened `--allow`, `result-1.json` kept); a continuation sent while turn 1 was
  still running (closed only by `result-2.json`, no stale file); a write outside `--allow` with a file
  tool (denied by PreToolUse, hard stop reported, `accept` rejects the `blocked` envelope); a shell
  write outside `--allow` (missed by the hook, rejected by `accept`); ending a turn without the
  envelope (the Stop retry nudged agy, which then wrote it). No timeouts and no crashes. Limits: one
  model, trivial tasks, and agy was told to probe the limits.
- **Install `lucind:dispatch` block into `~/.claude/CLAUDE.md`** (`feature/install-claude-md`): `lucind-ai install`
  (and `make install`) writes the dispatch block into `~/.claude/CLAUDE.md` idempotently, creates parent
  directories if needed, preserves surrounding content byte-for-byte, writes through symlinks, keeps
  one backup `<target>.lucind-ai.bak`, fails safely on malformed markers, and can be skipped with
  `--no-claude-md`.
- **Direct prompt dispatch** (`feature/direct-prompt-dispatch`): dispatch sends the task content itself
  directly to agy as the prompt (with the skills section placed first after the lane marker), dropped the
  `Read and follow <brief.md>` pointer and removed `--brief` in favor of `--prompt` across `dispatch` and
  `skills select`. The exact prompt is persisted to `prompt.md` for records, and the Stop hook classifies
  the main conversation using the `lucind-lane: <laneID>` marker line.
- **Global API key lookup, install-time key setup, and two skill variants** (`feature/global-api-key-and-skill-variants`):
  `TYPESAFE_API_KEY` lookup order checks the environment variable then `~/.config/lucind/env` (`$XDG_CONFIG_HOME/lucind/env`).
  `lucind-ai install` prompts for the key when missing (without echo using `stty -echo`) and stores it with mode 0600 in
  a 0700 dir, or skips when non-interactive. Renders the Claude skill as `auto` (orchestrator passes `--auto-skills`, no
  manual skills section needed) or `manual` (fallback where orchestrator writes the section; re-running install switches variants).
- **Invalid-key guidance and `install --reset-key`** (`fix/invalid-key-guidance`): a headless orchestrator test
  showed that for a stored but rejected key (HTTP 401) the skill and the exit 5 message pointed to
  `lucind-ai install`, which never replaces a key that resolves. Exit 5 now prints a different hint when the server
  answers 401 or 403, the skill says to correct `~/.config/lucind/env` or use the new
  `lucind-ai install --reset-key` (terminal only, empty answer keeps the current key, warns when the environment
  variable overrides the file).
- **Main conversation binding** (`feature/main-conversation-binding`): replaces the Stop hook's
  transcript parsing (`feature/lane-stop-main-conversation`) with a binding lucind-ai owns. A new
  `PreInvocation` hook (`lucind-ai hook pre-invocation`) writes the first conversation id of each
  turn once to `<laneDir>/main-turn-<turn>` (`O_CREATE|O_EXCL`, race-free, `lane.json` untouched).
  Stop compares its conversation id with that marker: equal is main, different is a worker (ignored
  without nudging or consuming retries), no marker falls back to `fullyIdle` alone. The
  `conversations/` cache and all transcript reads are gone. Measured on two real lanes (100 hook
  events): `PreInvocation` fires for workers too, the payload has no parent id or prompt text, the
  first event after dispatch is the main conversation, `ANTIGRAVITY_CONVERSATION_ID` equals the
  hook's own id, and herdr exposes nothing per conversation. Accepted risk: a worker of the
  previous turn still emitting `PreInvocation` before the new main conversation would be bound by
  mistake; not measured.
- **Superseded:** the multi-provider herdr work (`herdr-agent-factory`, `herdr-interactive-agents`)
  predates the agy-only contract; its interactive-pane and Stop-hook ideas survive in it.

## Next

1. **Shell-write escape.** PreToolUse sees file-write tools, not shell writes. Probe P5 confirmed
   it: agy created `sneaky.txt` with `echo`, the hook did not object, and `accept` rejected the lane
   (`changed file sneaky.txt not in allowlist`), but the file stays in the working tree, so the
   orchestrator has to clean it. How often it matters is still unmeasured: agy only did it because
   the brief asked. Keep measuring on real tasks before adding anything.
2. **Stale agy trust entries after a crash** in `~/.gemini/antigravity-cli/settings.json`.
3. **Make agy load skills first and pass them to workers.** Step 1 (send the prompt directly) is
   done and measured, see the result below; what is left is worker propagation and the
   `skills_loaded` field. History of the problem, owner
   observation: agy does not follow the brief literally. Dispatch sends `Read and follow <brief.md>`,
   and agy does not read the `## Skills to load before work` files before starting, even with the
   section right after the title. Evaluate sending the brief content itself as the prompt (instead
   of a pointer to a file to read), and measure skill loading with the envelope's `skills_loaded`
   (it came back `null` in the first `--auto-skills` lane, so the worker contract must require it).
   Confirmed in lane `20261005-050638-3cba` (owner screenshots): main agy read the lane rule, the
   brief, `lane.json`, the feature document and `lucind-result`, then went straight to code without
   opening any listed `SKILL.md`. Asked afterwards, agy admitted it read only `golang-cli` and did
   **not pass the skill paths to its two worker subagents**, so the brief fails twice: the main
   conversation does not load skills first, and it does not propagate them to workers (although
   `roles/agents/worker.md` step 1 tells workers to read them). The envelope still listed every
   skill in `skills_loaded`, so that field alone is not trustworthy evidence.
   **Result of step 1** (lanes `20261005-174913-ea48` with a hand-written section and
   `20261005-175854-e2e0` with `--auto-skills`, measured from the agy transcript, not from the
   envelope): the main conversation opened the lane rule and `lucind-result`, then every listed
   `SKILL.md` (5 of 5 with Jev) with `view_file` before reading any source file and before its first
   edit. Jev ran with `TYPESAFE_API_KEY` taken from `.env` by the caller: the binary only reads the
   environment and does not load `.env` itself. Still open:
   - Worker propagation is unmeasured: neither probe launched a worker subagent (one conversation
     only), so the "does not pass the skills to workers" failure was not exercised.
   - `skills_loaded` still came back `null` although the skills were really read (agy listed them
     in `done_criteria` instead), so the worker contract must require that field.
   - If worker propagation fails when a lane does launch workers, force it with a hook (for example
     a PreToolUse that blocks writes until every listed `SKILL.md` was read in that conversation).
   - Measure the main and worker conversations from the transcripts on a lane that launches workers.
4. **RTK support.** Install RTK as part of the lucind-ai setup (today it is wired by hand in the
   global Claude config: `@RTK.md` include plus the `rtk hook claude` PreToolUse hook).
5. **Research gentle-ai reviews in depth.** Understand how receipt-driven development (RDD) works
   end to end: review lifecycle, receipts and lineage, consent, correction, and how it interacts
   with lucind-ai lanes and `accept`.

## Only if needed

- Cross-machine attestations (today the HMAC key and attestations are local).
- A second provider, only when a real need and a verified hook surface exist.
- More stability trials with other models or accounts and with larger real tasks.
