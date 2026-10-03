# Herdr and agy spike findings (T7)

Date: 2026-10-03. Versions: `herdr 0.9.3`; `agy` from `~/.local/bin/agy` (model `gemini-3.8-flash-medium` for the probes). All probes used temporary repositories and panes created for the spike; nothing in this repository was changed by it. Commands are quoted so each fact can be reproduced.

## 1. `herdr worktree open --path` on an externally created worktree

Setup: `git init repo`, `git worktree add -b lane/x ../wt-ext` (created by plain git, not by herdr).

| Probe | Result |
|---|---|
| `herdr worktree open --path <wt-ext>` run from a pane whose workspace belongs to another repository | Error `worktree_not_found` ("worktree path not found"). The path is resolved against the repository of the calling workspace. |
| `herdr worktree open --cwd <repo> --path <wt-ext>` | Works. Returns `already_open:false`, a new workspace (`w3`, label = directory name), one root pane (`w3:p1`) with `cwd` = the worktree, and `worktree.checkout_path/repo_root/repo_key`. A workspace for the source repo (`w2`) also appeared as a side effect. |
| `herdr worktree list --cwd <repo>` | Lists the main checkout and the external linked worktree with `open_workspace_id`. |

Implication for `HerdrExecutor`: always pass `--cwd <primary repo root>` together with `--path <lane worktree>`; never rely on the caller's workspace. Treat the extra source-repo workspace as a side effect to account for (close only workspaces the executor created).

## 2. `herdr worktree remove`

| Probe | Result |
|---|---|
| Worktree with a live pane running `sleep 600`, clean tree, `herdr worktree remove --workspace w3` (no `--force`) | Succeeded (`worktree_removed`, `forced:false`): checkout directory deleted, `git worktree list` no longer shows it, workspace `w3` closed (the live pane did not block it). The branch `lane/x` was NOT deleted (`git branch --list` still shows it). |
| Worktree with an untracked file, no `--force` | Error `dirty_worktree_requires_force` ("contains modified or untracked files, use --force to delete it"); the directory and the file were left intact. |

Implication: herdr's `remove` kills live panes without complaint and only protects uncommitted files. It does not know about unique commits (lucind-ai's `HasUniqueCommits` guard must stay in front of any removal) and it leaves the branch behind. Do not call `remove --force`. Removal stays an explicit, human-owned step in this project.

## 3. Completion detection: `herdr agent wait` vs the exit sentinel

- `herdr agent wait <pane> --until idle|done|working|blocked|unknown` only works for panes where herdr has detected an agent. For a headless `agy --print ...` started with `herdr pane run`, detection was unreliable: polling `herdr agent get w1:p5` every 8 s during a run returned `agent:null` in 4 of 5 samples (once `agy`/`idle`), and `herdr agent wait w1:p5 --until idle --until done` failed with `agent_not_found`.
- The exit sentinel is reliable: `...; echo $? > EXIT.code; echo LUCIND_EXIT=$(<EXIT.code)` plus `herdr pane wait-output <pane> --regex 'LUCIND_EXIT=[0-9]+' --timeout <ms>` matched on every run in this session (about 20 runs: writers, explorers, reviewers).
- `herdr agent get/wait` are meant for interactive agents started with `herdr agent start --kind <agent>`; not exercised here.

Decision input for T8: use `pane run` + exit-code file + `pane wait-output --regex` for headless lanes. Do not depend on `agent wait` for them.

## 4. `agy --output-format stream-json`

`agy --print '<prompt>' --output-format stream-json` writes NDJSON, one object per line, nothing on stderr for a successful run. Event types observed in one short run: `init` (`conversation_id`, `init.model`, `init.cwd`, `init.tools` (the tool list), `init.permission_mode`), several `step_update` (`step_index`, `step_type`, `state`, optional `text_delta`; the last one carries `duration_seconds` and `usage`), and a final `result` (`conversation_id`, `status`, `response`, `duration_seconds`, `num_turns`, `usage`). The final `result` event equals the object printed by `--output-format json`, so a tee of the stream gives live progress and the same final payload.

## 5. Token and cost fields in `agy` JSON

`--output-format json` returns `conversation_id`, `status` (`SUCCESS` seen), `response` (the text), `duration_seconds`, `num_turns`, and `usage` with `input_tokens`, `output_tokens`, `thinking_tokens`, `cache_read_tokens`, `total_tokens`. There is no cost or price field. Example from a real writer run: `input_tokens 331476, output_tokens 26154, thinking_tokens 19426, cache_read_tokens 1377879, total_tokens 357630`. Cost must be computed by lucind-ai from a price table if wanted (T13).

## 6. What `agy --sandbox` restricts

Probe: `--sandbox --mode accept-edits --dangerously-skip-permissions --add-dir <sbx>` with cwd `<sbx>`; three shell commands: write inside the add-dir, write to a sibling directory outside it, HTTPS request to example.com.

| Command | Result |
|---|---|
| `echo inside > <sbx>/inside.txt` | FAILED: `Read-only file system` |
| `echo outside > <outside-dir>/outside.txt` | Succeeded |
| `curl https://example.com` | Succeeded (HTTP 200) |

So in this probe `--sandbox` made the working/add-dir tree read-only, did not restrict writes elsewhere (here: under the same `/tmp` scratch area) and did not restrict the network. Caveat: one probe, scratch directory under `/tmp`; the sandbox may behave differently for other paths. Conclusion: `--sandbox` is not a usable guard for writer lanes (it blocks the writes they exist to make) and gives no network isolation; keep the post-run `allowed_paths` diff check (`enforceAllowedPaths`) as the guard, as already decided.

## 7. Other facts learned while running the factory overnight

- `agy` exits 0 and prints `root agent idle; waiting up to 30m0s for 1 background task(s)` on stderr; not an error.
- `agy-pool` profiles contain no `antigravity-oauth-token`, so account rotation is unavailable (decision D-ROT-0).
- `--mode plan` produced read-only explorer and reviewer runs that left the worktree untouched in every case checked (`git status` clean afterwards).

## 8. agy interactive mode and hooks (spike 2026-10-03)

Source: Antigravity hooks documentation plus probes in a scratch repo (`agy -i` inside a herdr pane, `gemini-3.8-flash-medium`).

- `agy -i "<prompt>"` (`--prompt-interactive`) runs the prompt in the TUI and keeps the session open. `--mode accept-edits --dangerously-skip-permissions --model ...` work as in print mode.
- **Trust prompt.** On a folder agy has not seen, interactive mode blocks on "Do you trust the contents of this project?". `--dangerously-skip-permissions` does not skip it. Trust is stored in `~/.gemini/antigravity-cli/settings.json` (`trustedWorkspaces`). Every new lane worktree would hit it; pre-trusting means editing that global file (owner decision).
- **Hooks.** `.agents/hooks.json` at the workspace root is loaded (`/hooks` in the TUI lists it). Schema: `{"<hook-name>": {"<Event>": [handlers]}}`; events `PreToolUse`, `PostToolUse` (with `matcher`), `PreInvocation`, `PostInvocation`, `Stop`. A handler is `{"type":"command","command":"<abs path>","timeout":N}`; input is JSON on stdin, output JSON on stdout. A Claude-style schema (`{"hooks":{"Stop":...}}`) is silently ignored.
- **`Stop` fires at the end of every turn in interactive mode**, not only on exit. Observed sequence for one turn: `PreInvocation 0`, `PostToolUse write_to_file`, `PostInvocation 0`, `PreInvocation 1`, `PostInvocation 1`, `Stop {"terminationReason":"NO_TOOL_CALL","fullyIdle":true}`. `fullyIdle:true` means no background task is pending.
- **`Stop` can force more work.** Returning `{"decision":"continue","reason":"..."}` re-enters the loop with `reason` injected; the agent then did the missing work and the next `Stop` (empty `{}`) let it finish. This can validate the result envelope in the hook and make the agent repair it itself, with a counter to bound the loop.
- A `Stop` probe in `--print` mode did not log; not investigated (print lanes use the process exit instead).
- Interactive runs give no `usage` JSON and no exit code; the end signal is the hook, the result is the envelope file.
