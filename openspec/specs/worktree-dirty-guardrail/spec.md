# Worktree Dirty Guardrail Specification

## Purpose

Ensure worktree removal and cleanup operations fail closed when uncommitted changes exist unless force is explicitly requested.

## Requirements

### Requirement: Worktree dirty guardrail check

`worktree.Cleanup` and `worktree.Remove` MUST check `PorcelainEmpty` before deleting any linked worktree directory, failing closed and returning `worktree.ErrWorktreeDirty` unless `force: true` is explicitly passed.

#### Scenario: Refuse cleanup on dirty worktree without force
- GIVEN a linked worktree where `PorcelainEmpty` reports false
- WHEN `worktree.Cleanup` or `worktree.Remove` runs without `force: true`
- THEN the operation MUST return `worktree.ErrWorktreeDirty` and preserve all worktree files on disk

#### Scenario: Force cleanup removes dirty worktree
- GIVEN a dirty linked worktree
- WHEN `worktree.Remove` or `worktree.Cleanup` runs with `force: true`
- THEN the worktree MUST be removed and the operation MUST succeed without returning `worktree.ErrWorktreeDirty`

#### Scenario: Clean worktree cleanup succeeds idempotently
- GIVEN a clean linked worktree where `PorcelainEmpty` reports true or a nonexistent worktree path
- WHEN `worktree.Cleanup` runs without `force: true`
- THEN the operation MUST remove the worktree if present and return nil error
