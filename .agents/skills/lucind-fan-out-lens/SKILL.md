---
name: lucind-fan-out-lens
description: >-
  Guide for multi-lens planning phases (explore, propose, design, spec, tasks).
  Use when dispatched to author a lens-a, lens-b, or lens-c planning document.
---

# Lucind Fan-Out Planning Lens Guide

## Lens Ownership Model
- **Lens A (Primary Authority)**: Owns core problem definition, candidate approaches, architecture decisions, and task dependency ordering. Lens B and C must assume Lens A's declarations.
- **Lens B (Capabilities & Scope)**: Owns user scenarios, path disjointness, and acceptance criteria.
- **Lens C (Risks & Verification)**: Owns risk matrix, failure modes, trade-offs, and verification test pairing.

## Word Budget & Formatting
- **Budget**: Maximum 1000 words per lens document (excluding the `## Citation Manifest` section).
- **Structure**: Follow the exact skeleton provided in the packet. Do not improvise heading names.

## Citation Manifest Protocol
Every planning lens MUST terminate with a `## Citation Manifest` section.
- Exactly one row per unique `file:line` or `file:start-end` range.
- Group citations by file in alphabetical order, lines ascending.
- State the explicit claim supported by each citation.
- Validate before committing:
  ```bash
  ./lucind-lane-check.sh --file <path> --budget 1000 --verify-citations --skip-git --skip-result
  ```
