# Read-Only Done Criterion Specification

## Purpose

Replace mandatory done-criterion 2 for `read_only: true` packets; keep it unchanged for write packets.

## Requirements

### Requirement: Write Packets Keep Criterion 2

Write packets (`read_only` omitted or `false`) MUST keep mandatory criterion 2 exactly as-is: the work is committed, with evidence `git status --porcelain` empty and `git log --oneline -1`. (Design Decision 2.)

#### Scenario: Write packet commit evidence
- GIVEN a write packet reporting `status: done`
- WHEN criterion 2 is checked
- THEN porcelain MUST be empty and `HEAD` MUST show the commit that carries the work

### Requirement: Read-Only Packets Replace Criterion 2

A `read_only: true` packet MUST replace mandatory criterion 2 with: the worktree carries no unique commits and no working-tree changes relative to the lane's birth point. Evidence MUST be `git status --porcelain` empty AND the worktree's `HEAD` equals `git merge-base HEAD <primary HEAD>`. The merge-base check MUST NOT be omitted: porcelain-empty alone MUST NOT satisfy the criterion. (Design Decision 2.)

#### Scenario: Read-only unchanged tree
- GIVEN a `read_only: true` packet reporting `status: done`
- WHEN criterion 2 is checked
- THEN porcelain MUST be empty AND `HEAD` MUST equal `git merge-base HEAD <primary HEAD>`

#### Scenario: Clean tree after a commit is not enough
- GIVEN a `read_only: true` packet whose worktree has unique commits and empty porcelain
- WHEN criterion 2 is checked
- THEN the criterion MUST NOT be considered met

### Requirement: Envelope File Is Not a Mutation

A written `.lucind/result.json` MUST NOT count as a working-tree mutation, because `.lucind/` is gitignored. (Design Decision 2.)

#### Scenario: Envelope does not dirty porcelain
- GIVEN a lane that wrote only `.lucind/result.json`
- WHEN `git status --porcelain` is run
- THEN the output MUST be empty

### Requirement: Templates Document the Exception

`packet-template.md` MUST stay write-default (no `read_only` in the example frontmatter) and MUST add a note that `read_only: true` swaps criterion 2 for the unchanged-tree text. `human-packet-template.md` MUST NOT change. `SKILL.md` MUST document explore as dispatchable via `lucind-ai run` and MUST apply the same criterion-2 exception. (Design Decision 2.)

#### Scenario: Write skeleton unchanged
- GIVEN the packet template skeleton
- WHEN an author copies it
- THEN example frontmatter MUST omit `read_only` and criterion 2 MUST still require a commit

#### Scenario: Read-only note present
- GIVEN a packet author setting `read_only: true`
- WHEN they follow the template
- THEN the template MUST tell them to replace criterion 2 with the unchanged-tree check

#### Scenario: Human template untouched
- GIVEN `human-packet-template.md`
- WHEN this change is applied
- THEN that file MUST be unmodified
