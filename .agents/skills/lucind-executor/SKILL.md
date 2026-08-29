---
name: lucind-executor
description: >-
  Core operational manual for executing lucind-ai lanes as an agent.
  Use when dispatched to run a lucind-ai packet across any lifecycle phase.
---

# Lucind-AI Executor Manual

## Overview
As an executor, you operate within an isolated git worktree (`../lucind-ai-worktrees/<lane-id>`). Your entry point is the packet file (`.lucind/packets/<packet-id>.md`).

## Packet Schema Reference
- `id`: Unique lane identifier (determines branch `lucind/<id>`).
- `executor`: Executor assigned to this lane.
- `routed_by`: Condition justifying routing.
- `model`: Target LLM model.
- `read_only`: If `true`, lane makes no commits.
- `allowed_paths`: Whitelist of editable paths.

## Execution Sequence
1. **Preconditions**: Verify all preconditions stated in the packet before taking any action. If any fails, stop and report `blocked`.
2. **Implementation / Analysis**: Follow the phase-specific skill (`lucind-apply`, `lucind-fan-out-lens`, or `lucind-verify`).
3. **Evidence Collection**: Gather real command outputs, git status, and symbol citations.
4. **Envelope Generation**: Write `.lucind/result.json` validating against `.lucind/result.schema.json`.

## Wave Viability Rule
Strict TDD RED and GREEN steps for a single unit MUST remain in the same lane. `internal/integrate.Check` gates batches against passing test suites (`lucind-checks.sh`); splitting RED and GREEN across separate waves causes bisection failure.
