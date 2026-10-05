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
- **Superseded:** the multi-provider herdr work (`herdr-agent-factory`, `herdr-interactive-agents`)
  predates the agy-only contract; its interactive-pane and Stop-hook ideas survive in it.

## Next

1. **Shell-write escape.** PreToolUse sees file-write tools, not shell writes. Probe P5 confirmed
   it: agy created `sneaky.txt` with `echo`, the hook did not object, and `accept` rejected the lane
   (`changed file sneaky.txt not in allowlist`), but the file stays in the working tree, so the
   orchestrator has to clean it. How often it matters is still unmeasured: agy only did it because
   the brief asked. Keep measuring on real tasks before adding anything.
2. **Stale agy trust entries after a crash** in `~/.gemini/antigravity-cli/settings.json`.
3. **Stops from other conversations** (fixed in `feature/lane-stop-main-conversation`). The Stop hook
   now inspects `transcriptPath` step 0: main conversations contain `Read and follow .../.lucind/lanes/<id>/brief.md`
   with source `USER_EXPLICIT` (or empty), while worker conversations contain `SYSTEM`/`SYSTEM_MESSAGE`
   with `sender=`. Role classifications are cached in `<laneDir>/conversations/<id>`. Worker Stops
   are ignored without nudging or consuming retries. Main conversation Stops with `fullyIdle=false` are
   ignored while worker subagents run; only main Stops with `fullyIdle=true` decide done/retry/failed.
   Continuation turns across differing main conversation IDs are resolved independently via the current
   turn's brief marker.
   **Owner review: this is a poor solution, replace it.** Classifying conversations by parsing step 0
   of agy's private transcript format couples lucind-ai to undocumented internals (`source`, `type`,
   the exact dispatch prompt wording) and breaks silently if any of them change. Look for a
   supported signal instead: a parent/child id in the Stop payload, an agy hook or API that marks
   subagents, or lucind-ai owning the main conversation id at dispatch time.
4. **Briefs do not make agy load skills first; evaluate sending the prompt directly.** Owner
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
   skill in `skills_loaded`, so that field alone is not trustworthy evidence. Plan, in order:
   2. If that is not enough, force it with a hook (for example a PreToolUse that blocks writes until
      every listed `SKILL.md` was read in that conversation).
   3. Measure in both the main and the worker conversations from the transcripts, not only from
      `skills_loaded`.
5. **RTK support.** Install RTK as part of the lucind-ai setup (today it is wired by hand in the
   global Claude config: `@RTK.md` include plus the `rtk hook claude` PreToolUse hook).
6. **Research gentle-ai reviews in depth.** Understand how receipt-driven development (RDD) works
   end to end: review lifecycle, receipts and lineage, consent, correction, and how it interacts
   with lucind-ai lanes and `accept`.

## Only if needed

- Cross-machine attestations (today the HMAC key and attestations are local).
- A second provider, only when a real need and a verified hook surface exist.
- More stability trials with other models or accounts and with larger real tasks.
