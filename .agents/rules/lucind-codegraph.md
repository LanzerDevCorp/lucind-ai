# CodeGraph Usage in lucind-ai

The `codegraph` MCP server is configured and available in this environment.

## Exploration Order for Structural Questions
When analyzing call graphs, symbol references, architecture, or cross-package impacts:
1. Prefer the `codegraph_explore` MCP tool first.
2. Fall back to `rg`/`fd` only when the query is purely textual or CodeGraph is unavailable.

## Worktree Indexing
Each linked worktree has its own isolated `.codegraph/` index at its root. Never copy or symlink an index across worktrees.
