# lucind-ai: product

A small Go binary that enforces a **deterministic contract** on code-changing lanes run by
Antigravity (`agy`) through [herdr](https://github.com/ogulcancelik/herdr). Claude orchestrates,
agy implements, lucind-ai checks only what needs no judgment.

How Claude uses it: [`plugin/claude-code/skills/lucind/SKILL.md`](../plugin/claude-code/skills/lucind/SKILL.md).

## Boundary rule

lucind-ai owns only what is reproducible without judgment:

- result-envelope validation against a schema
- tree-hash attestation of test runs (HMAC)
- allowed-path enforcement
- pass/fail lane checks

Everything else belongs to Claude: when to parallelize, merging, conflict resolution, retries,
review, model choice, branches and worktrees.

## Flow

```text
Claude --dispatch--> lane (new herdr pane, agy, LUCIND_LANE=<id>)
agy edits inside --allow globs --> attest run (per lane check) --> result-<turn>.json
agy Stop hook validates result-<turn>.json (max 2 re-entries) --> lane done|failed
Claude --accept--> receipt.json (accepted | rejected)
```

## CLI

| Command | What it does |
|---|---|
| `dispatch --cwd <dir> --allow <glob>... --prompt <file\|-> [--auto-skills] [--check <cmd>]... [--model M] [--timeout D] [--detach] [--lane <id>] [--min-quota F]` | Create a lane, record the base tree, open an agy pane, send the prompt directly. Blocks until the lane is `done`/`failed`/`timeout`, or returns immediately with `--detach`. `--lane <id>` continues a lane in the same pane. |
| `wait <lane> [--cwd <dir>] [--timeout D]` | Block on a detached lane and validate its result. |
| `accept --lane <id>` | Write `receipt.json`. Accepts only if the result is valid, every file changed since base tree matches `--allow`, and valid attestations match the final tree for all lane checks (otherwise running any missing checks); requires no attestation when zero checks were specified. |
| `attest run -- <cmd>` / `attest verify --command <cmd>` | Run a command and sign `{command, exit code, tree hash}`; verify a matching passing attestation. See [`attestation.md`](attestation.md). |
| `check [--out <path>]` | Run `lucind-checks.sh` (deprecated; scrubbed env, timeout, process-group kill). |
| `hook pre-tool-use\|stop` | Handlers called by the agy plugin. Pass-through without `LUCIND_LANE`. |
| `plugin install [--dir <staging root>]` | Render the embedded agy plugin into a staging dir (`$XDG_DATA_HOME/lucind-ai/agy-plugin/lucind`) and register it with `agy plugin install` (lands in `~/.gemini/config/plugins/lucind/`; requires `agy` on PATH). A plugin merely dropped into `~/.gemini/antigravity-cli/plugins/` validates but is never loaded, so the obsolete copy there is removed. |
| `install` | Flagless installer for Claude skill (auto/manual variant matching key setup), `lucind` agy plugin, and `lucind-roles` agy plugin. |
| `--version` | Exact build (`git describe`). |

`dispatch`/`wait` print one JSON object. Exit codes: 0 done, 1 error, 3 failed, 4 timeout, 5 auto-skills unavailable.
The pane is never closed or killed by lucind-ai. Requires `HERDR_ENV=1`.
Model precedence: `--model` > `LUCIND_AGY_MODEL` > agy default.

## Lane state

Plain JSON under `<git toplevel>/.lucind/lanes/<id>/` (id = `YYYYMMDD-HHMMSS-<4 hex>`, UTC). No database.

| File | Content |
|---|---|
| `prompt.md` | The exact prompt sent to agy (recorded for reference; not referenced from the prompt itself). |
| `lane.json` | Base tree hash, allowed globs, checks list, model, pane id, turn (1 on create, incremented by every continuation), status (`running\|done\|failed\|timeout\|accepted\|rejected`). |
| `result-<turn>.json` | Envelope written by agy for the current turn (earlier turns stay as history; legacy lanes without turn use `result.json`). |
| `receipt.json` | Base/final tree, changed files, verdict, reasons, evidence. |

Attestations live in `$XDG_STATE_HOME/lucind-ai/attestations/`, outside the repo.

## agy plugin

Embedded in the binary, installed globally by `plugin install`.

- **PreToolUse**: denies writes outside the lane's globs (except the current turn's result
  file), protects all other `.lucind/**` paths, and denies reading the HMAC key or writing the
  attestations directory.
- **Stop**: validates the current turn's result file; if missing or invalid it re-enters agy with
  the schema error (max 2 times), then marks the lane `failed`. Logs the raw Stop payload in
  `hook.log`.
- **Rule + skill** (`lucind-lane`, `lucind-result`): describe the result contract and
  `lucind-ai attest run`.

Without `LUCIND_LANE` every hook is a no-op, so free/manual agy sessions are unaffected.

## Setup

`make install` builds the binary and runs `lucind-ai install`, which:
1. Resolves `TYPESAFE_API_KEY`: checks the `TYPESAFE_API_KEY` environment variable first, then
   `~/.config/lucind/env` (`$XDG_CONFIG_HOME/lucind/env`).
2. Prompts on an interactive terminal when no key is set (hidden input via `stty -echo`), storing it
   in `~/.config/lucind/env` with mode 0600. When non-interactive, skips prompt without blocking.
3. Renders and installs the Claude skill into `~/.claude/skills/lucind` in one of two variants:
   - **`auto` variant** (key configured): orchestrator dispatches with `--auto-skills` and does not
     need to construct `## Skills to load before work`.
   - **`manual` variant** (no key): orchestrator resolves skill paths and writes the section manually.
   Re-running `lucind-ai install` switches the variant when a key is configured later.
4. Installs the `lucind` and `lucind-roles` agy plugins.
5. Writes the dispatch block into `~/.claude/CLAUDE.md` (idempotently, skipped with `--no-claude-md`).

Check the build with `lucind-ai -v`.

## Removed on purpose

This feature deleted judgment from the binary; Claude does it better and cheaper.

| Removed | Why |
|---|---|
| Executors `cursor-agent`, `opencode`, `claude`; the `Executor` interface | Single provider (agy). |
| `integrate`/`run` (merge, bisect, revert, CAS promote), `resolve`, `judges` | Merging and review are Claude's. Plain git. |
| `explore`, `split`/`dag` | Read-only work needs no contract. |
| `feature`, leases, `reconcile`, `defect`, SQLite ledger | Replaced by per-lane JSON files. |
| Packet files, `rules init\|generate` | The prompt is free Markdown; the plugin carries the rules. |
| Per-lane worktrees, `worktree cleanup` | Worktrees only for parallel writers, created by Claude. |
| `usage` log, `plugin/opencode` | No consumer. |

Historical design notes remain in `docs/provider-docs/`, `openspec/` and `odd/`.
