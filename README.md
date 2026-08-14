# lucind-ai

Multi-agent orchestration for CLI coding agents. Supervisor pattern, two levels, never three:
one thread holds a task ledger and dispatches surgical packets; `agy` executes in parallel,
`opencode` audits, and the human runs whatever touches a secret.

Born inside a marketplace project and extracted here so it can evolve on its own.

## Why it exists

Delegating to a CLI agent is easy. Trusting what comes back is not.

The first real packet returned `done` with every criterion green and real command output
attached — and still had a defect, because the defect lived just outside what the criteria
asked. The next round did it again. Both times the packet had an explicit hard stop covering
exactly that case, and the agent walked past it and reported success without contradicting
itself.

Everything here is a consequence of that: hard stops that must be declared in the envelope,
indirection that must be proven consumed by a *terminal consumer*, work that is not delivered
until it is committed, and an audit lane run by a different model family.

## Install

### Claude Code

```bash
/plugin marketplace add <path-or-git-url-to-this-repo>
/plugin install lucind-ai@lucind-ai
```

For local development, a directory junction keeps one copy on disk:

```powershell
cmd /c mklink /J "$env:USERPROFILE\.claude\skills\lucind-ai" "<repo>\plugin\claude-code\skills\lucind-ai"
```

### agy (Antigravity)

The same plugin directory installs into `agy`, bringing skills, MCP servers and hooks across —
agents do not cross:

```bash
agy plugin install <repo>/plugin/claude-code
```

`agy plugin import claude` does **not** work; it never looks in `~/.claude/plugins/`. Install
by path.

### The project it runs in

Paste `templates/project-routing.md` into the project's own `CLAUDE.md`. Project level, not
global — a global file managed by another tool gets overwritten on update.

Create `.lucind/` in the project and gitignore it. That is where the ledger, the approval
queue, the dispatched packets and the returned envelopes live.

## What is in here

```
plugin/claude-code/skills/lucind-ai/
├── SKILL.md                          the contract: hard rules, routing, fallback, ledger
├── references/runtime.md             verified CLI surface, MCP wiring, verification traps
├── references/state-files.md         ledger and approval queue formats
├── assets/packet-template.md         agent packet
├── assets/human-packet-template.md   human packet
└── assets/result.schema.json         result envelope, with mandatory hard-stop declarations

templates/project-routing.md          the two-axis routing table to paste per project
docs/estado-real.html                 diagram of what actually runs today
```

## The four lanes

| Lane | Who | For |
|---|---|---|
| Execute | `agy` · gemini-3.7-flash | Bounded work with checkable done-criteria, in its own worktree |
| Audit | `opencode` · gpt-5.6-sol | Adversarial judgment on a diff, by a different model family |
| Fallback audit | Two blind Claude judges | Only on a quota error. Degrades Tier B to human merge |
| Human | You | Anything that needs a credential value or critical supervision |

The human is a lane, not an interruption. A human packet contains **one command** — the one
that requires the secret. Backups, merges, container restarts, verification and cleanup are
boilerplate the orchestrator runs.

## Status

Honest, because a plan that claims to be finished is the same failure this project exists to
catch.

| Piece | State |
|---|---|
| Skill, templates, envelope schema | written |
| Execution lane (`agy`) | one packet, two rounds, merged |
| Human lane | one packet, closed — it found two defects in its own instructions |
| Audit lane (`opencode`) | mandatory by rule, never run |
| Fallback panel | never run |
| Tier B auto-merge | never run |
| Ledger in engram | still on disk; `agy` can read engram, no packet has used it as a context channel |
| Turn-protocol hook | not implemented |

Open the diagram in `docs/` for the same picture, drawn.

## Prior art

The supervisor and Magentic task-ledger patterns, the three classes of agent error, and the
circuit breaker come from Chapter 19 of the Gentleman Programming book on AI orchestration
patterns. The second error class it describes — technically valid parameters, syntax fine,
semantics wrong, *treacherous* — is exactly what the early packets produced.
