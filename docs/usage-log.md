# Usage Logging and Reporting

## 1. Overview

Lucind-ai provides per-call executor usage tracking and target share analysis:
1. **Per-call usage log**: Every executor attempt executed through `run.Execute` appends one JSON line (JSONL) to a persistent state file.
2. **Usage report command**: `lucind-ai usage report` aggregates records from the log, summarizes calls, token counts, and costs by provider, model, and lane role, and compares the core provider split against the target 60/15/25 distribution.

---

## 2. File Location and Permissions

- **Path**: `$XDG_STATE_HOME/lucind-ai/usage.jsonl` (fallback: `~/.local/state/lucind-ai/usage.jsonl`).
- **Directory Mode**: `0700` (`rwx------`).
- **File Mode**: `0600` (`rw-------`).
- **Concurrency & Atomicity**: The file is opened with `O_APPEND|O_CREATE|O_WRONLY`. Each JSON line is assembled entirely in memory and written in a single `write()` call, ensuring that concurrent lanes never interleave partial lines under POSIX atomic append guarantees.

---

## 3. Record Format

Each line in `usage.jsonl` is a self-contained JSON object with the following fields:

| Field | Type | Description |
|---|---|---|
| `ts` | string | RFC3339Nano UTC timestamp of the attempt record. |
| `run_id` | string | Identifier of the parent batch run. |
| `lane_id` | string | Identifier of the lane. |
| `attempt` | integer | 1-based attempt sequence number within the lane loop. |
| `executor` | string | Name of the executor invoked (e.g. `agy`, `herdr-agy`, `cursor-agent`, `claude`, `opencode`). |
| `provider` | string | Canonical provider name derived from executor (`agy`, `cursor`, `claude`, `opencode`, or executor name). |
| `model` | string | Model name used for the attempt. |
| `lane_role` | string | Role assigned to the lane (e.g. `apply`, `judge`, `research`). |
| `input_tokens` | integer | Prompt / input token count. |
| `output_tokens` | integer | Output / generated token count. |
| `thinking_tokens` | integer | Reasoning / thinking tokens (if reported separately). |
| `cache_read_tokens` | integer | Tokens read from cache. |
| `total_tokens` | integer | Total tokens consumed by the attempt. |
| `cost_usd` | float | Estimated cost in USD (0.0 when unknown). |
| `tokens_known` | boolean | True if token counts were successfully extracted or reported. |
| `duration_ms` | integer | Wall-clock execution duration in milliseconds. |
| `exit_code` | integer | Process exit code of the executor run. |
| `timed_out` | boolean | True if the attempt was stopped due to timeout. |
| `status` | string | Lane status after attempt (empty `""` at call time). |

---

## 4. Token & Cost Extraction Order

Token and cost metrics are resolved with the following precedence:

1. **Stdout JSON Extraction**:
   - `usagelog.ExtractUsage(outcome.Stdout)` inspects the final output of the executor.
   - **Antigravity (`agy`) shape**: Parses `usage` containing `input_tokens`, `output_tokens`, `thinking_tokens`, `cache_read_tokens`, and `total_tokens`.
   - **Claude shape**: Parses `total_cost_usd` or `cost_usd` at top level (or in `usage`); folds `cache_creation_input_tokens` into `input_tokens`; reads `cache_read_input_tokens`; and computes `total_tokens` as the sum of parts (`input_tokens + output_tokens + cache_read_tokens`) if absent.
2. **Progress Events Fallback**:
   - If stdout yields no parseable usage, `Execute` scans the `executor.ProgressEvent` stream of that call and takes the maximum `TotalTokens` and `CostUSD` reported.
3. **Unknown Fallback**:
   - If neither stdout nor progress events carry token counts, the record is logged with `tokens_known = false`, zero token counts, and zero cost.

### Error Handling & Failure Isolation
Logging operates on a fail-safe boundary: any error encountered while creating or writing to the usage log prints a single diagnostic line to `stderr` and does **not** fail the lane or interrupt execution.

---

## 5. Usage Report Command

```bash
lucind-ai usage report [--since <duration or YYYY-MM-DD>] [--file <path>] [--json]
```

### Options:
- `--since <value>`: Filter records since a given duration or date:
  - Go duration: e.g. `24h`, `30m`.
  - Explicit day suffix: e.g. `7d`, `1d`, `30d`.
  - Calendar date: `YYYY-MM-DD` (UTC).
- `--file <path>`: Read from a custom JSONL file path rather than the default state location.
- `--json`: Output the aggregated report in structured JSON format with stable field ordering.

### Missing File Behavior:
If the usage log file does not exist (or has no entries), the command exits with code `0` and prints `no usage recorded`.

---

## 6. Target Provider Split

The orchestrator targets the following split for factory work:

| Provider | Target Share | Notes |
|---|---|---|
| `agy` | 60% (0.60) | Heavy lanes: explorers, writers, fixes, tests, e2e. |
| `cursor` | 15% (0.15) | Verification lanes: blind judges and fast checks. |
| `claude` | 25% (0.25) | Orchestration, design, decisions, and escalation. |

### Calculation Rules:
- The target share is calculated against the sum of total tokens consumed by `agy`, `cursor`, and `claude`:
  $$\text{TargetTokens} = \text{tokens}(\text{agy}) + \text{tokens}(\text{cursor}) + \text{tokens}(\text{claude})$$
- Other providers (such as `opencode` or custom tools) are listed in the general providers summary but excluded from the 60/15/25 target share calculation.
- Delta is reported in percentage points ($\text{ActualShare} - \text{TargetShare}$).
- If total tokens across the three target providers is 0, shares are 0 and the target section displays `no token data`.

---

## 7. Known Limitation

Only lane calls made through `lucind-ai` are logged. Orchestrator Claude usage outside `claude` executor lanes (e.g. interactive planning or chat in the terminal) is not visible in this log.
