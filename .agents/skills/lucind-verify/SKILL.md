---
name: lucind-verify
description: >-
  Protocol for qualitative verification lanes.
  Use when conducting read-only qualitative audits of candidate implementations.
---

# Lucind Qualitative Verify Lane Guide

## Read-Only Lane Constraints
- **Zero Modifications**: Do NOT edit source files or commit changes. Only `.lucind/result.json` is writable.
- **No Test Executions**: Mechanical check logs (`lucind-checks.sh`) are pre-computed and provided in the packet Context. Do not re-run tests.

## Qualitative Audit Dimensions
1. **Specification Compliance**: Check each requirement in `specs/*/spec.md` against concrete code symbols (`file:line`).
2. **Edge Cases & Boundary Conditions**: Look for unhandled nil pointers, race conditions, error wrapping, and timeout leaks.
3. **Test Quality**: Verify that tests assert real terminal behaviors rather than mocking internal implementation artifacts.

## Result Envelope Findings Format
Populate the `findings` array in `.lucind/result.json`:
```json
{
  "status": "done",
  "summary": "Qualitative verification completed.",
  "findings": [
    {
      "finding": "Description of defect or observation",
      "evidence": "path/to/file.go:123-145",
      "affects": "Spec Requirement X.Y"
    }
  ],
  "done_criteria": [...],
  "hard_stops": [...]
}
```
