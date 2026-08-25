# Lucind-AI Lane Protocol

## Domain Vocabulary (CONTEXT.md is binding)
- **Change**: Independent unit of work owned by one Orchestrator.
- **Lane**: Isolated unit of delegated work with its own worktree and branch.
- **Packet**: The `.md` file with YAML frontmatter defining the complete instruction set.
- **Envelope**: The `.lucind/result.json` file you MUST write in your worktree before completion.

## Result Envelope Protocol
Every lane (write or read-only) MUST write `.lucind/result.json` in its own worktree root.
- Validate against `.lucind/result.schema.json` in the worktree before writing.
- Printed output alone causes the lane to be treated as producing nothing.
- Required fields: `status` (done|blocked|deviated|failed), `summary`, `done_criteria[]`, `hard_stops[]`.
- Every done criterion from the packet MUST be listed with concrete evidence.
- Every hard stop from the packet MUST be listed, indicating whether it fired.

## Read-Only Lanes (`read_only: true`)
- Omit the `commit` field in `result.json`.
- Do not produce any unique commit.
- `git status --porcelain` MUST remain empty.
- `HEAD` MUST equal `git merge-base HEAD <primary HEAD>`.

## Commit Protocol (Write Lanes)
- Use Conventional Commits: `<type>(<scope>): <description>`.
- NEVER add "Co-authored-by", "Co-Authored-By", or any AI attribution trailers.
- Verify commit body with `git show -s --format="%b" HEAD`. If an attribution trailer was added, run `git commit --amend` to remove it.

## Universal Hard Stops
Declare all of these in the envelope whether or not they fired:
1. Any credential value would need to be chosen, generated, or written.
2. A done criterion is impossible or already true for an unexpected reason.
3. Satisfying one instruction requires violating another.

## Tool Preferences
- Use `rg` instead of `grep`, `fd` instead of `find`, `bat` instead of `cat`, `sd` instead of `sed`, `eza` instead of `ls`.
- For structural/codebase exploration, use CodeGraph before broad filesystem search.
