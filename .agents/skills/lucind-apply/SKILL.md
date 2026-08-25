---
name: lucind-apply
description: >-
  Step-by-step TDD and implementation guide for lucind-ai apply lanes.
  Use when implementing code changes or bug fixes under an apply-* packet.
---

# Lucind Apply Lane Guide

## Strict TDD Lifecycle
1. **RED Phase**:
   - Write the unit/integration test reproducing the issue or asserting the new requirement.
   - Run the test suite (`go test ./<pkg>/... -count=1`).
   - Confirm the test fails for the EXPECTED reason (not a compilation error or panic).
2. **GREEN Phase**:
   - Implement the minimal required logic to turn the test green.
   - Run the test suite and confirm it passes.
3. **SWEEP Phase**:
   - Run the full project check: `CGO_ENABLED=0 go build ./...` and `go test ./... -race -count=1`.
   - Run `go vet ./...` and verify `gofmt -l` produces no diffs.

## Mutation Testing for New Assertions
If adding new assertions:
1. Temporarily break the production logic.
2. Confirm the test fails naming the specific condition.
3. Restore the production logic and confirm green status.

## Pre-Commit Verification Checklist
- [ ] `git status --porcelain` shows only paths within `## Allowed paths`.
- [ ] `./lucind-checks.sh` passes cleanly.
- [ ] Commit message follows Conventional Commits: `<type>(<scope>): <summary>`.
- [ ] Commit body has NO AI attribution trailers (`git show -s --format="%b" HEAD`).
- [ ] `.lucind/result.json` is written with full `done_criteria` and `hard_stops`.
