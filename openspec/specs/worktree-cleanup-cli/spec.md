# Worktree Cleanup CLI Specification

## Purpose

Define CLI worktree cleanup behavior, flag support, and diagnostic reporting for clean and dirty worktrees.

## Requirements

### Requirement: Worktree cleanup CLI force flag and diagnostic status reporting

`lucind-ai worktree cleanup` MUST accept a `--force` (`-f`) flag. When invoked against a dirty worktree without `--force`, the command MUST exit 1, output the dirty status and diff diagnostic commands referencing `troubleshooting.md`, and preserve the worktree files on disk. When invoked with `--force` (`-f`), or against a clean or nonexistent worktree, cleanup MUST remove the worktree (if present) and exit 0.

#### Scenario: Refuse CLI cleanup on dirty worktree without force
- GIVEN a linked worktree where `PorcelainEmpty` reports false
- WHEN running `lucind-ai worktree cleanup --lane <id>` without `--force`
- THEN the command MUST exit 1, output porcelain status citing `troubleshooting.md`, and preserve files on disk

#### Scenario: Force CLI cleanup removes dirty worktree
- GIVEN a dirty linked worktree
- WHEN running `lucind-ai worktree cleanup --lane <id> --force` (or `-f`)
- THEN the command MUST delete the worktree and exit 0

#### Scenario: Clean or nonexistent worktree CLI cleanup succeeds
- GIVEN a clean linked worktree or nonexistent lane path
- WHEN running `lucind-ai worktree cleanup --lane <id>` without `--force`
- THEN the command MUST remove the worktree if present and exit 0
