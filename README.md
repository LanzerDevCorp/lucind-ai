# lucind-ai

A deterministic contract for code-changing lanes run by Antigravity (`agy`) through herdr.
Claude orchestrates, agy implements, lucind-ai enforces only what needs no judgment: the result
contract, allowed paths, and test attestation.

**[`docs/product.md`](docs/product.md) is the source of truth.** This is the short version.

## Flow

```text
dispatch (lane + agy pane) -> agy edits within --allow -> attest run -> result.json -> accept (receipt)
```

## CLI

| Command | Purpose |
|---|---|
| `dispatch --cwd <dir> --allow <glob>... --brief <file\|-> [--detach] [--lane <id>]` | Open an agy lane and send the brief. |
| `wait <lane>` | Block on a detached lane and validate its result. |
| `accept --lane <id>` | Write a receipt if result, allowed paths and attestation all hold. |
| `attest run -- <cmd>` / `attest verify --command <cmd>` | HMAC tree-hash attestation of a test run. |
| `check` | Run `lucind-checks.sh`. |
| `hook pre-tool-use\|stop` | agy plugin handlers. |
| `plugin install` | Install the embedded agy plugin. |
| `--version` | Exact build. |

Lane state is plain JSON in `.lucind/lanes/<id>/`. Requires `HERDR_ENV=1` and `agy`.

## Install

```bash
make install
```

Installs the binary, the agy plugin (`~/.gemini/antigravity-cli/plugins/lucind/`) and links the
Claude skill into `~/.claude/skills/lucind`. Verify with `lucind-ai -v`.

## Docs

- [`docs/product.md`](docs/product.md): design, state files, plugin, what was removed.
- [`docs/ROADMAP.md`](docs/ROADMAP.md): what is next.
- [`docs/attestation.md`](docs/attestation.md): HMAC attestation details.
- [`plugin/claude-code/skills/lucind/SKILL.md`](plugin/claude-code/skills/lucind/SKILL.md): how Claude drives lanes.
