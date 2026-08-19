# Runtime surface

Verified 2026-08-13. Re-check with `agy --help` / `opencode --help` after upgrades.

## agy — Antigravity CLI 1.1.12

**It can have the same tools as everyone else.** The earlier belief that `agy` was toolless
was true about its state, not its capability.

Two ways in, and the first is simpler:

1. **Its own MCP config**, same shape as Claude's, at
   `~/.gemini/antigravity-cli/mcp_config.json`:

   ```json
   { "mcpServers": { "engram": { "command": "engram", "args": ["mcp"] } } }
   ```

   There is no `agy mcp` subcommand — that absence is why the surface looked missing. The
   binary carries the full MCP type system internally.

2. **Installing a Claude Code plugin**, which brings skills, MCP servers and hooks across
   (agents do not cross):

   ```bash
   agy plugin install ~/.claude/plugins/marketplaces/<name>/plugin/claude-code
   ```

   Confirmed working end to end: after installing engram this way, `agy` called `mem_search`
   and returned real observations.

Two traps:

- `agy plugin import claude` **does not work** — it answers "No claude extensions found" and
  never looks in `~/.claude/plugins/`. Install by path instead.
- `agy plugin validate <dir>` gives a **false negative**: it looks for `plugin.json` at the
  directory root instead of `.claude-plugin/plugin.json`. `install` on the same directory
  succeeds.
- `codegraph` and `context7` are not plugins — they live as plain MCP servers in
  `~/.claude.json`. Copy them into agy's `mcp_config.json`; there is nothing to install.

Non-interactive invocation:

```bash
agy -p "$(cat packet.md)" \
    --model gemini-3.7-flash-high \
    --output-format json \
    --json-schema .lucind/result.schema.json \
    --add-dir <worktree> \
    --mode accept-edits \
    --dangerously-skip-permissions \
    --print-timeout 20m
```

| Flag | Use |
|---|---|
| `-p` / `--print` | One-shot, non-interactive. The only mode for packets. |
| `--json-schema` | Enforce the result envelope. Never dispatch without it. |
| `--output-format json` | Machine-readable. `stream-json` only for live progress. |
| `--effort low\|medium\|high` | Match to packet size. |
| `--conversation <id>` | Resume a blocked packet with its context. |
| `--mode plan` | Dry run — produces a plan, writes nothing. |
| `--sandbox` | Terminal restrictions. Use for untrusted sweeps. |

**Default: `gemini-3.7-flash-high`** unless the user names another model.

Full roster: `gemini-3.7-flash-{low,medium,high}`, `gemini-3.6-flash-*`, `gemini-3.5-flash-*`,
`gemini-3.1-pro-{low,high}`, `claude-sonnet-4-6`, `claude-opus-4-6-thinking`,
`gpt-oss-120b-medium`. There is no 3.7 Pro — 3.1 is the only Pro tier, two generations older
than 3.7 Flash.

## opencode 1.18.18

Holds the audit lane. MCPs connected: **codegraph, context7, engram, exa, supabase-local**.

```bash
opencode run --agent build -m openai/gpt-5.6-sol --prompt "$(cat review.md)"
opencode run -s <sessionID> --prompt "<answer to the blocking question>"
opencode export <sessionID>          # harvest full result as JSON
```

Relevant models: `openai/gpt-5.6-sol` (default for judgment), `openai/gpt-5.6-sol-fast`,
`opencode-go/gpt-5.6-luna`, plus the `vercel/anthropic/*` roster.

`--auto` approves non-denied permissions. `--fork` branches a session instead of continuing.

## Blind Claude panel — audit fallback only

Used when `opencode` returns a quota or rate-limit error, never for any other failure.

```
Agent(subagent_type: "jd-judge-a", model: "opus", prompt: <frozen diff + packet, nothing else>)
Agent(subagent_type: "jd-judge-b", model: "opus", prompt: <same>)
```

Both carry `Read, Glob, Grep, engram, codegraph` and **no conversation context** — that is the
whole point. They cannot echo the orchestrator's reasoning because they never saw it.

Never route this to the orchestrator itself, and never to an agent that carries the
conversation. Record the result as `audit.family: "same"`; it degrades Tier B to human merge.

## Worktrees

Sibling of the repo, under the user's home — **never** `/tmp`, `/var/tmp`, or a system temp
dir. CodeGraph indexes are per-checkout.

```bash
git worktree add ../<repo>-worktrees/<packet-id> -b lucind/<packet-id>
git worktree remove ../<repo>-worktrees/<packet-id>
```

Each worktree needs its own `.codegraph/` index. Never copy, symlink, or reuse another
checkout's — the root and the checked-out bytes differ.

**A worktree stack publishes the same host ports as the main one.** Tear down a packet's
containers before bringing the main stack up, or the bind fails. Record running containers in
the ledger so the next packet knows what is holding a port.

## Windows notes

PowerShell is primary. From Git Bash, prefix a single command with `MSYS_NO_PATHCONV=1` when
passing POSIX-looking paths to a native `.exe`; never set it globally.

## Verification traps

Found the hard way; they produce evidence that looks real and proves nothing.

- **Docker freezes environment variables at container creation.** Editing `.env` afterwards
  does nothing to an existing container. Use `docker compose up -d --force-recreate <svc>`.
  The symptom is identical to a wrong password.
- **The official Postgres image trusts localhost.** `pg_hba.conf` ships
  `host all all 127.0.0.1/32 trust`, so any `psql` run inside the db container authenticates
  without checking the password. Test against the container's network IP instead.
- **`psql -c` does not interpolate `-v` variables.** For the injection-safe `:'var'` form,
  feed the SQL through stdin.
