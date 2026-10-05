<!-- lucind:dispatch -->
## lucind-ai Dispatch (user-owned, takes precedence)

### Precedence
- This block is user-owned: it sits outside the gentle-ai markers and survives `gentle-ai sync`.
- It wins over any conflicting text above about WHO executes delegated work.

### When vs who
- The Mandatory Delegation Triggers, the Work Routing Ladder and the Delegation Rules above still decide WHEN to delegate (mapping, write, preparation, broad research, per-action).
- They no longer decide WHO executes.
- Wherever the text above says "native Agent/Task mechanism", "the runtime's subagent/delegation mechanism", or names a bounded writer, read-only explorer or verification worker, the executor is lucind-ai.
- Load the `lucind` skill before dispatching. It requires `HERDR_ENV=1`.

### Executor mapping
| Work | Executor |
| --- | --- |
| Read-only exploration, research, fan-out | Free agy through herdr, no lane. Always tell it not to edit files. |
| Code-changing work | A lane: `lucind-ai dispatch`, `wait`, `accept`. |
| Target outside the repository | Native bounded writer (lanes cannot target it). |

- Lane brief mapping: `## Allowed edit surfaces` becomes the lane's `--allow` globs.
- `## Verification` commands become optional `--check` flags. Pass only the checks that fit the change.
- `## Skills to load before work`, `## Key Learnings` and the other brief sections follow the `lucind` skill.
- `--allow` globs are repository-relative, so work outside the repository cannot be a lane. Use a native bounded writer with the same prompt contract: exact allowed edit surfaces, skills, verification, Key Learnings.

### Verification replaces the separate verifier agent
- There is no native verifier agent by default.
- The writer's own verification is the lane's `--check` attestations. That is the proof of record. Do not re-run those tests after an accepted receipt.
- If a writer returns partial, blocked or an incomplete delivery, send a follow-up turn to the same lane (`dispatch --lane <id>` with the full original brief) instead of spawning a new agent.
- Always compare the diff and the envelope's `done_criteria` with the brief before `accept`.
- Expensive or external checks: run `lucind-ai attest run -- sh -c '<cmd>'` yourself in the background with bounded output. `accept` reuses a valid attestation for the same tree.
- When a check needs an agent's visual or end-to-end judgment, use a verification lane: no edits, cheaper `--model`, only `--check`.
- The parent spot check stays: re-run one reported command before delivery.
- Independent verification for high-risk candidates stays native: the native review (RDD) and judgment-day.

### Native subagents that remain allowed
- The review actors of the native review protocol (`review-*`, `review-refuter`) when gentle-ai's review runs them.
- Judgment-day (`jd-judge-a`, `jd-judge-b`, `jd-fix-agent`) only when the user asks for it.
- `Explore` for quick read-only codebase searches.
- A native writer only for files outside the repository.
- The `sdd-*` agents only when the user invokes the `gentle-sdd-*` skills.
- Anything else goes through lucind-ai.

### No silent fallback
- If `HERDR_ENV` is not 1 or agy is unavailable (for example no quota), say so and ask one question.
- Do not silently switch to native subagents for code changes.
- Trivial work may still be done inline under the inline-direct rules above.

### Unchanged
- The ODD protocol, feature documents, commits per task, receipt-driven review and delivery rules stay as written above.
- The operating rules of lanes (absolute `--cwd "$PWD"`, review before accept, closing panes, preferred pane layout) live in the `lucind` skill. Do not duplicate them here.
<!-- /lucind:dispatch -->
