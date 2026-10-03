---
name: lucind-fan-out-lens
description: >-
  Trigger: fan-out explorer lane, read-only research, codebase investigation, lens exploration, synthesis lane.
  Protocol for read-only structural, textual, and historical exploration lenses and unified evidence synthesis.
---

# Lucind Fan-Out Explorer Lens Guide

## Read-Only Exploration Principles

- **Strictly Read-Only**: Never edit files, create new files, or run mutating commands. Do not run formatters, linters that modify files, build scripts that generate artifacts, or git commands that alter state.
- **Bounded Evidence**: Return at most about 2,000 tokens of exact `path:line` evidence plus a short list of open questions.
- **Fact vs. Assumption Discipline**: Strictly distinguish verified facts from assumptions. State observed code facts backed by explicit `path:line` references; explicitly label inferences, hypotheses, or unverified claims as assumptions.

## The Three Exploration Lenses

### Structural Lens
- **Method**: Query CodeGraph first before broad filesystem exploration using the `codegraph_explore` MCP tool or read-only upstream CLI commands (`codegraph status`, `codegraph query`, `codegraph explore`, `codegraph node`, `codegraph files`, `codegraph callers`, `codegraph callees`, `codegraph impact`, `codegraph affected`). Trace symbol definitions, type hierarchies, callers, callees, dependency chains, and blast radius of potential modifications before opening individual files.
- **Output Rules**: Report symbol definitions, call graphs, incoming and outgoing dependencies, and blast radius calculations. Ground every finding with exact `path:line` pointers and explicit symbol signatures.

### Textual Lens
- **Method**: High-speed textual search across the workspace using `rg`, `fd`, and `bat` (never use `cat`, `grep`, `find`, or `ls`). Search exact keywords, error strings, configuration keys, log statements, type usages, and pattern occurrences across the workspace.
- **Output Rules**: Report literal matches, match distributions across packages, and verbatim code snippets with exact `path:line` references. Include negative search results (patterns confirmed absent) when relevant.

### Historical Lens
- **Method**: Git archaeology across repository history using `git log -S <symbol>`, `git log -G <regex>`, `git blame -L <start>,<end> <path>`, and `git log --stat` over the target area. Trace commit history, commit messages, bug fixes, refactor rationales, and past architectural shifts. Query Engram persistent memory only if the packet explicitly authorizes it.
- **Output Rules**: Report relevant commit hashes, commit summaries, author rationale, past regression contexts, and timeline evolution. Ground every historical claim in specific commits or blame ranges.

## Synthesis Lane Protocol

A `synthesis` lane merges the outputs of the structural, textual, and historical lenses into a single cohesive handoff.
- **Token Budget**: Produce a bounded handoff of at most ~2,000 tokens total.
- **Preserve Contradictions**: Keep discrepancies and conflicting signals between lenses clearly visible; do not paper over differences or force premature consensus.
- **Zero Speculation**: Do not invent or add new claims not substantiated by the underlying lens outputs.
- **Output Structure**:
  1. **Executive Summary**: Verified consensus facts across lenses.
  2. **Structural & Textual Evidence**: Key symbols, call paths, blast radius, and exact `path:line` citations.
  3. **Historical Context**: Relevant commits, prior decisions, and historical constraints.
  4. **Contradictions & Gaps**: Divergences between structural, textual, or historical observations.
  5. **Open Questions**: Concise list of unresolved questions or technical risks for the orchestrator.
