# Executors and Execution Routes

Load this module only when choosing or diagnosing an executor, model, provider, or agent profile.

## Supported routes

| Executor | Default model | Allowed models | Notes |
|---|---|---|---|
| `agy` | `gemini-3.7-flash-high` | `gemini-3.7-flash-high` | Broad mechanical work; result schema is also passed to the CLI, but the packet must still write the envelope file. |
| `cursor-agent` | `cursor-grok-4.6-high` | `cursor-grok-4.6-high` | Bounded editorial or precision work. Do not copy an external-provider model into this route. |
| `opencode` | `openai/gpt-5.6-sol` | `openai/gpt-5.6-sol`, `openai/gpt-5.6-luna` | Only route supporting packet `agent`; use a primary agent listed by the installed CLI. |
| `claude` | `claude-opus-5` | `claude-opus-5` | Uses the full model ID rather than the moving `opus` alias for reproducibility. |

`cmd/lucind-ai/cli.go` is the supported-executor source of truth; each executor's `KnownModels()` owns its closed model list. Unlisted executor or model values fail before dispatch and never silently fall back or cross provider billing. Each executor supplies its own default when `model` is omitted.

An `agent` value is valid only with `opencode`. It selects a system prompt and tool-permission profile, not a provider. The CLI rejects it for other executors. Opencode can exit 0 while silently replacing a requested subagent-only profile with its default primary agent; the executor detects that warning and forces a blocked result rather than accepting an uncontrolled prompt substitution.

## Authoring preference

Executor selection remains manual Orchestrator judgment, not code-level routing. Planning artifacts usually favor `cursor-agent`; broad implementation usually favors `agy`; bounded judgment-heavy apply units may favor `cursor-agent`. `validate` is not a dispatch phase. Prefer observed aptitude and explicit human direction over phase labels.

## Provider operations

Use installed CLI help and model-list commands for invocation details. Re-check after provider upgrades. `agy` can load MCP servers through `~/.gemini/antigravity-cli/mcp_config.json` or install a compatible Claude Code plugin by path; agents do not transfer with plugins. There is no `agy mcp` subcommand. `agy plugin import claude` does not inspect the normal plugin directory, and `agy plugin validate` can reject a package whose `.claude-plugin/plugin.json` installs successfully. Plain MCPs such as CodeGraph and Context7 belong in the MCP config, not plugin installation.

Opencode route failures preserve captured stdout and stderr for diagnosis. Quota failure does not authorize a silent route change; obtain Orchestrator approval for a different Execution Route.

### `agy` account pool and the wave-level quota gate

`agy` is Antigravity CLI, not the classic Gemini CLI: it keeps a single OAuth token with no embedded email at `~/.gemini/antigravity-cli/antigravity-oauth-token`, and its own free local slash command `agy --print "/usage" --output-format json` is the only place remaining quota is exposed (per model-family group, `weekly` and `5h` buckets; `num_turns` is always `0` for it, so checking costs no model quota).

`scripts/agy-pool` manages a pool of saved Google account credentials for this one token file: `init`/`add`/`list`/`save <email> [--force]`/`use <email>`/`current`/`next`/`count`/`usage [--refresh]`/`best <min-fraction>`. Since the token carries no email, `save`'s identity check compares against the pool's own bookkeeping (`.active`), not who is really logged into `agy` — pass `--force` when you know better. `usage` always checks the active account live and reads every other saved account from its own cache (refreshing one means temporarily swapping its credential in, which this pool never does casually — see the next paragraph); `save` also opportunistically caches a fresh reading since the credential is already active at that point.

`lucind-ai run`'s `--min-quota <fraction>` flag (default `0.10`) gates a **whole wave**, never a single lane: the check runs once per `runDispatch` invocation, before any lane starts (`ExecuteBatch`), and only when the batch includes an `agy`-executed packet. Below the threshold it asks `agy-pool best` for whichever pooled account has the most `gemini-5h` quota left and switches to it (`agy-pool use`) before dispatching; if no pooled account clears the minimum, the wave is blocked with an error and nothing dispatches — no lane, no ledger row. `--min-quota 0` disables the check. This is why the gate is wave-scoped and not per-lane: rotating the shared credential file mid-flight would pull it out from under every other lane already dispatching concurrently within the same wave.

`scripts/agyr` is a separate, simpler wrapper (detects `429`/quota-exhausted output from a single `agy` invocation and retries against `agy-pool next`) kept in the repo but not the recommended path for lucind-ai dispatches — the wave-level `--min-quota` gate above supersedes it for that use case. It remains available for manual, sequential `agy` use outside lucind-ai.

A blind in-process Claude panel is an audit fallback outside `lucind-ai`, not an executor route or stable orchestration dependency. If a separate review protocol authorizes it after opencode quota failure, freeze the same diff and packet for context-isolated judges and record the degraded same-family audit. Never route that fallback to the context-carrying Orchestrator.

## Worktree environment

Executor worktrees live as siblings under the user's home, never system temp. Each needs its own CodeGraph index. Do not copy or symlink indexes. Stop worktree containers before starting another stack that publishes the same host ports.
