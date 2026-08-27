# Delta for TDD WIP-Rescue Protocol

## ADDED Requirements

### Requirement: Prescriptive TDD WIP-rescue protocol documentation

`troubleshooting.md` and `lucind-apply` (`SKILL.md`) MUST document a prescriptive TDD WIP-rescue procedure for timed-out or blocked apply lanes. The documented procedure MUST instruct operators to inspect uncommitted diffs within preserved worktrees, create a partial WIP commit, update packet timeout parameters, and re-dispatch without losing uncommitted RED test or GREEN implementation progress.

#### Scenario: Operator executes TDD WIP-rescue after lane timeout
- GIVEN an apply lane timing out with uncommitted progress
- WHEN following the rescue protocol in `troubleshooting.md` and `lucind-apply`
- THEN the operator inspects diffs in the preserved worktree, commits WIP, updates packet timeout, and re-dispatches without data loss
