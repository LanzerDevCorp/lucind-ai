# lucind-ai

A deterministic contract for code-changing lanes run by Antigravity (`agy`) through herdr.
Claude orchestrates, agy implements, lucind-ai enforces only what needs no judgment: the result
contract, allowed paths, and test attestation.

**[`docs/product.md`](docs/product.md) is the source of truth.** This is the short version.

## Flow

```text
dispatch (lane + agy pane) -> agy edits within --allow -> attest run -> result-<turn>.json -> accept (receipt)
```

## CLI

| Command | Purpose |
|---|---|
| `dispatch --cwd <dir> --allow <glob>... --prompt <file\|-> [--auto-skills] [--check <cmd>]... [--detach] [--lane <id>]` | Open an agy lane and send the prompt. |
| `wait <lane>` | Block on a detached lane and validate its result. |
| `accept --lane <id>` | Write a receipt if result, allowed paths and lane check attestations all hold. |
| `attest run -- <cmd>` / `attest verify --command <cmd>` | HMAC tree-hash attestation of a test run. |
| `hook pre-tool-use\|stop` | agy plugin handlers. |
| `skills select --prompt <file\|-> [--allow <glob>]... [--cwd <dir>] [--registry <path>] [--threshold <float>]` | Ask Jev which skills to load for a prompt. |
| `plugin install` | Install the embedded agy plugin. |
| `install` | Install the Claude skill (auto or manual variant), the `lucind` agy plugin, the `lucind-roles` agy plugin, and write the lucind dispatch block into `~/.claude/CLAUDE.md` (`--no-claude-md` to skip). |
| `--version` | Exact build. |

Lane state is plain JSON in `.lucind/lanes/<id>/`. Requires `HERDR_ENV=1` and `agy`.

## Install

```bash
make install
```

Installs the binary and runs `lucind-ai install`, which:
- Checks for `TYPESAFE_API_KEY`: lookup checks the `TYPESAFE_API_KEY` environment variable first, then `$XDG_CONFIG_HOME/lucind/env` (defaults to `~/.config/lucind/env`).
- If no key is configured and standard input is a terminal, prompts for `TYPESAFE_API_KEY` (hidden without echo via `stty -echo`) and saves it to `~/.config/lucind/env` (mode 0600 in a mode 0700 directory). If input is empty or non-interactive (e.g. CI), prompting is skipped.
- Never replaces a key that already resolves, even an invalid one. `lucind-ai install --reset-key` (terminal only) asks for a new key and replaces the stored one; an empty answer keeps the current key, and a warning appears when the `TYPESAFE_API_KEY` environment variable is set because it overrides the file.
- Installs the Claude skill into `~/.claude/skills/lucind` rendered to match your setup:
  - **`auto` variant** (key configured): the orchestrator dispatches with `--auto-skills` and does not need to write `## Skills to load before work` manually.
  - **`manual` variant** (no key): the orchestrator manually resolves skill paths and writes the section.
  Re-running `lucind-ai install` switches variants when a key is configured later.
- Installs the `lucind` agy plugin and the `lucind-roles` agy plugin.
- Writes the lucind dispatch block into `~/.claude/CLAUDE.md` (idempotently, skipped with `--no-claude-md`).

Verify with `lucind-ai -v`.

## Docs

- [`docs/product.md`](docs/product.md): design, state files, plugin, what was removed.
- [`docs/ROADMAP.md`](docs/ROADMAP.md): what is next.
- [`docs/attestation.md`](docs/attestation.md): HMAC attestation details.
- [`docs/skill-selection.md`](docs/skill-selection.md): Jev skill selection subcommand and contract.
- [`plugin/claude-code/skills/lucind/SKILL.md`](plugin/claude-code/skills/lucind/SKILL.md): how Claude drives lanes.
