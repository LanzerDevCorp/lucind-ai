<!-- lucind:rules audience=all -->
## Workspace purpose and baseline rules

lucind-ai coordinates autonomous AI agent workflows and multi-provider agent dispatch across this workspace. It provides deterministic verification, attestation, and guarded execution for engineering tasks.

- Use Conventional Commits for all commit messages.
- Never include AI attribution trailers or generated tags in commit messages.
- Keep `lucind-ai -v` current after binary changes (`make install` when the repository has that target).
- Never read sensitive files or locations, including secrets, credentials, tokens, private keys, personal data, or `.env` files.

<!-- lucind:rules audience=orchestrator -->
## Orchestration and delegation

Delegation goes through the dispatcher: run `lucind-ai run --packet <file>`; never start ad-hoc agent processes for implementation work.

- Small, understood, one-file fixes are done inline by the orchestrator.
- Dispatch only for 2+ non-trivial files, a new file, more than 5 lookups, or reading that prepares a write.
- Always declare `route`, `route_evidence`, `allowed_paths`, `verification`, and `commit_message` in the packet.
- Verification and the commit are done by the dispatcher; workers never commit.
- Packets with `max_iterations` or `escalation` require `verification` and `commit_message`.
- Read a worker's `interaction_required` result and answer it; do not guess.

<!-- lucind:rules audience=worker -->
## Worker execution rules

- Stay strictly inside the allowed edit surfaces (`allowed_paths`); never touch out-of-scope files.
- Never run `git add`, `git commit`, or `git push`; the dispatcher commits after verification.
- Tool safety: no destructive commands (`rm`, `git reset`, `git clean`, etc.), no dependency mutation, and no network changes.
- Strict TDD when a runner exists: RED (observed failure) then GREEN (passing test); never invent lifecycle evidence.
- Run the packet's `verification` commands exactly as written and report each as `<command>: <observed result>`.
- If requirements, paths, or scope are ambiguous, stop with `interaction_required` (payload: `question`, `reason`, `options`, `unblock_response`).
- Write the result envelope to `.lucind/result.json` conforming to the result schema.
- Close the lane handoff with a `## Key Learnings` block of 1-5 standalone factual sentences.
