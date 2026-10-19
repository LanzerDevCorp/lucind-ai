<!-- lucind:dispatch -->
## lucind-ai Dispatch (user-owned, takes precedence)

### Precedence
- This block is user-owned: it sits outside the gentle-ai markers and survives `gentle-ai sync`.
- lucind-ai extends gentle-ai, it does not replace it. gentle-ai owns WHEN to delegate and HOW to work: the delegation triggers, the routing ladder, the Risk line, the verification gate, the prompt sections. This block only decides WHO executes, through the user's work mode.

### Who executes: ask the work mode
- Before delegating a role (`explorer`, `writer`, `verifier`, the same names gentle-ai uses), run `lucind-ai route <role> --cwd "$PWD"`. It prints `<executor> <model>` for the active work mode (project override first, then the global mode).
- `native <model>`: run the role with gentle-ai's own mechanism (the Agent/Task tool) on that model.
- `lane <model>`: run it through lucind-ai with that model (table below). Load the `lucind` skill first. It requires `HERDR_ENV=1`.
- If `route` fails because there is no modes config or no active mode, lucind-ai is not configured: follow gentle-ai unchanged.
- If the route says `lane` but `HERDR_ENV` is not 1 or agy is unavailable (for example no quota), say so and ask one question. Never switch executors silently.
- `lucind-ai mode list|show|use <name> [--project]|clear` inspects and switches the mode; the user decides when to switch, never switch it on your own.

### Lane executor mapping
| Role | Lane form |
| --- | --- |
| explorer | Free agy through herdr, no lane. Always tell it not to edit files. |
| writer | A lane: `lucind-ai dispatch --role writer`, `wait`, `accept`. |
| verifier | A lane with no edits, or free agy; it may run `lucind-ai attest run`. |

- `## Allowed edit surfaces` becomes the lane's `--allow` globs; `## Verification` commands become optional `--check` flags (pass only the ones that fit the change).
- `## Skills to load before work`, `## Key Learnings` and the other prompt sections follow the `lucind` skill. The size limit goes to a worker as a firm limit, as the skill says.
- Work outside the repository cannot be a lane (`--allow` is repository-relative): use a native writer with the same prompt contract.

### Verification
- gentle-ai's rules are unchanged: a Risk line that names a high-risk item gets one independent verifier after the change's own checks; the parent spot check stays.
- Who the verifier is comes from `lucind-ai route verifier`.
- A lane writer's `--check` attestations are its proof of record: do not re-run those tests after an accepted receipt. `accept` reuses a valid attestation for the same tree.
- Review the diff and the envelope's `done_criteria` against the prompt before `accept`. If a lane returns partial, blocked or incomplete work, send a follow-up turn to the same lane (`dispatch --lane <id>` with the full original prompt) instead of a new agent.
- Expensive or external checks: run `lucind-ai attest run -- sh -c '<cmd>'` yourself in the background with bounded output.

### Always native
- The review actors of gentle-ai's review (`review-*`, `review-refuter`), judgment-day (`jd-*`) when the user asks for it, `Explore`, and the `sdd-*` agents when the user invokes `gentle-sdd-*`.

### Unchanged
- The ODD protocol, feature documents, commits per task, receipt-driven review and delivery rules stay as gentle-ai writes them.
- The operating rules of lanes (absolute `--cwd "$PWD"`, review before accept, closing panes, preferred pane layout) live in the `lucind` skill. Do not duplicate them here.
<!-- /lucind:dispatch -->
