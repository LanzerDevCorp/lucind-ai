# Explorer Fan-Out (`lucind-ai explore`)

## 1. Overview

`lucind-ai explore` performs parallel, multi-perspective codebase exploration for an objective, followed by a unified synthesis lane that condenses the evidence into a bounded handoff.

It implements the read-only explorer fan-out protocol (route `fanout`):
1. **Parallel Lens Execution**: Runs three distinct read-only explorer lens lanes concurrently (`structural`, `textual`, `historical`).
2. **Synthesis Lane**: A single synthesis lane receives the bounded outputs from all lenses and generates a unified handoff of approximately 2,000 tokens with explicit `path:line` evidence citations.
3. **Terminal Output**: Prints the unified handoff to `stdout` preceded by a standard banner and records `handoff.md` in the run directory.

---

## 2. Command Usage

```bash
lucind-ai explore --objective <text> [--scope <path> ...] [--id <prefix>] [--timeout <duration>]
```

### Flags

| Flag | Description | Default |
|---|---|---|
| `--objective` | Detailed exploration task or research objective (required, up to 2000 bytes, valid UTF-8, no NUL). | *(required)* |
| `--scope` | Repeatable repository-relative path restricting exploration area (at most 20 entries, each at most 200 bytes, no leading `/`, no `..` segments). | none (whole workspace) |
| `--id` | Run prefix and lane identifier (must match `^[a-z0-9][a-z0-9-]{0,39}$`). | `explore-<UTC timestamp yyyymmddhhmmss>` |
| `--timeout` | Per-lane execution timeout. | `30m` (same default as `run`) |

---

## 3. The Three Exploration Lenses

The explorer fan-out deploys three independent read-only lenses simultaneously:

### 1. Structural Lens (`<id>-structural`)
- **Focus**: Symbol definitions, type hierarchies, call graphs, incoming and outgoing callers/callees, and change blast radius.
- **Method**: CodeGraph exploration first via `codegraph_explore` MCP tool or upstream read-only CLI commands (`codegraph status`, `codegraph query`, `codegraph explore`, `codegraph callers`, `codegraph callees`, `codegraph impact`, `codegraph affected`). Ground every finding with exact `path:line` pointers and explicit symbol signatures.

### 2. Textual Lens (`<id>-textual`)
- **Focus**: High-speed textual search across code, configuration, error messages, and documentation.
- **Method**: Fast search using `rg`, `fd`, and `bat` (never `cat`, `grep`, `find`, or `ls`). Gathers verbatim code snippets, pattern distribution, keyword occurrences, and negative search evidence (patterns confirmed absent).

### 3. Historical Lens (`<id>-historical`)
- **Focus**: Git archaeology, past commit rationales, regression histories, and architectural evolution.
- **Method**: Git history inspection using `git log -S <symbol>`, `git log -G <regex>`, `git blame -L <start>,<end>`, and `git log --stat`. Ground claims in specific commit hashes and blame ranges.

---

## 4. Quota and Execution Model

- **Executor Runtime**: Lanes are executed using the `agy` runtime (default model `gemini-3.8-flash-medium`).
- **Quota Consumption**: A full explore invocation consumes **4 lane calls** against the Antigravity pool:
  - Batch 1: 3 simultaneous quota burns running the 3 lens lanes in parallel (bounded by `--max-parallel 3`).
  - Batch 2: 1 quota burn running the synthesis lane.
- **Strictly Read-Only**: Every lane is dispatched with `read_only: true` and `lane_role: lens` / `synthesis`. Lenses create isolated git worktrees but make no commits and leave no modified working-tree files.

---

## 5. Bounds and Truncation

To protect LLM context windows and prevent runaway output:
- **Lens Output Budget**: Each individual lens lane is instructed to return at most ~2,000 tokens of verified facts backed by `path:line` citations.
- **Tail Truncation**: When assembling the synthesis prompt, each lens summary is bounded to the **last 6,000 bytes**. Slicing always aligns to a valid UTF-8 rune boundary without splitting multi-byte runes, and prepends a visible `[truncated]` note when truncation occurs.
- **Fence Safety**: Objective and lens summaries are rendered inside markdown code fences longer than any contiguous backtick run within the content, preventing prompt injection or premature fence closures.
- **Synthesis Handoff Budget**: The synthesis lane synthesizes all evidence into a bounded handoff of at most ~2,000 tokens, maintaining verified consensus facts, highlighting any contradictions between lenses, and listing open questions.

---

## 6. Run Directory & File Storage

All generated packets and results are stored in a dedicated per-run directory:
- **Location**: `$XDG_STATE_HOME/lucind-ai/explore/<id>/` (fallback: `~/.local/state/lucind-ai/explore/<id>/`).
- **Directory Mode**: `0700` (`rwx------`).
- **File Mode**: `0600` (`rw-------`).

### Emitted Files

- `<run-dir>/<id>-structural.md`: Structural lens packet.
- `<run-dir>/<id>-textual.md`: Textual lens packet.
- `<run-dir>/<id>-historical.md`: Historical lens packet.
- `<run-dir>/<id>-synthesis.md`: Synthesis lane packet (written if at least one lens succeeds).
- `<run-dir>/handoff.md`: Final synthesis summary handoff.

---

## 7. Exit Codes and Error Handling

| Exit Code | Condition | Behavior |
|---|---|---|
| `0` | **Success** | Synthesis lane reached `status: done`. The handoff is printed to stdout and saved to `<run-dir>/handoff.md`. |
| `1` | **Validation Failure** | Invalid or missing `--objective`, invalid `--id`, or invalid `--scope`. Error printed to stderr; zero lanes dispatched. |
| `1` | **All Lenses Failed** | All three lens lanes failed or deviated. Failures printed to stderr; synthesis lane is never started. |
| `1` | **Synthesis Failed** | Synthesis lane failed or deviated. Synthesis diagnosis printed to stderr. |

### Terminal Output on Success

When synthesis succeeds, stdout begins with the handoff banner:
```text
explore <id>: handoff from synthesis lane <id>-synthesis
<unified handoff text with path:line citations and open questions>
```
