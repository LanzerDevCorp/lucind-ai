# OpenCode integration

This distribution keeps `plugin/claude-code/skills/lucind-ai/**` canonical and
ships an exact copy for OpenCode. The native plugin only exposes the safe
`lucind_ai` tool: it invokes `lucind-ai` with an argv array, the active
directory, and OpenCode cancellation. Orchestration remains in the skill and
the Go binary remains the source of truth.

Install globally (honors `XDG_CONFIG_HOME`, otherwise `$HOME/.config`):

```sh
make install
make install-opencode-plugin
make verify-opencode-plugin
```

Claude Code and OpenCode are separate runtimes. `/lucind-ai` in Claude Code is
still provided by the Claude plugin; OpenCode loads this native plugin and
skill globally. Restart OpenCode after installing or changing config, plugin,
or skill files.
