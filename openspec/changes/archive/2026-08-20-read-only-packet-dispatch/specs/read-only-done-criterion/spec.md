# Read-Only Done Criterion Specification

## Purpose

Define what mandatory done-criterion 2 ("the work is committed") means for a write packet versus a `read_only: true` packet, and update the authoring assets that state it.

## Requirements

### Requirement: Write Packets Keep Criterion 2 Unchanged

A write packet (`read_only` omitted or `false`) MUST keep mandatory criterion 2 exactly as it exists today: the work is committed, evidenced by an empty `git status --porcelain` and a commit visible via `git log --oneline -1`. (Design Decision 2.)

#### Scenario: Write packet commit evidence
- GIVEN a write packet reporting `status: done`
- WHEN criterion 2 is checked
- THEN porcelain MUST be empty and `HEAD` MUST show the commit carrying the work

### Requirement: Read-Only Packets Replace Criterion 2

A `read_only: true` packet MUST replace mandatory criterion 2 with: the worktree carries no unique commits and no working-tree changes relative to the lane's birth point. Evidence MUST be `git status --porcelain` empty **and** the worktree's `HEAD` equal to `git merge-base HEAD <primary HEAD>`. Porcelain-empty alone MUST NOT satisfy the criterion — the merge-base check MUST NOT be omitted, because a lane could commit and still leave a clean tree afterward. (Design Decision 2.)

#### Scenario: Read-only unchanged-tree evidence
- GIVEN a `read_only: true` packet reporting `status: done`
- WHEN criterion 2 is checked
- THEN porcelain MUST be empty AND `HEAD` MUST equal `git merge-base HEAD <primary HEAD>`

#### Scenario: A clean tree after a commit is not enough
- GIVEN a `read_only: true` packet whose worktree carries a unique commit but reports empty porcelain
- WHEN criterion 2 is checked
- THEN the criterion MUST NOT be considered met

### Requirement: The Protocol Envelope Is Not a Mutation

A written `.lucind/result.json` MUST NOT count as a working-tree mutation for either packet class, because `.lucind/` is gitignored. (Design Decision 2.)

#### Scenario: Envelope write does not dirty porcelain
- GIVEN a lane that has written only `.lucind/result.json`
- WHEN `git status --porcelain` is run in that worktree
- THEN the output MUST be empty

### Requirement: Authoring Assets Document the Exception

`packet-template.md` MUST stay write-default — its example frontmatter MUST NOT include `read_only` — and MUST gain a note that setting `read_only: true` swaps criterion 2 for the unchanged-tree text above. `human-packet-template.md` MUST NOT change: it has no frontmatter, is not parsed by `packet.Parse`, and carries no "work is committed" criterion. `SKILL.md` MUST document `explore` as dispatchable via `lucind-ai run` and MUST state the same criterion-2 exception. (Design Decision 2.)

#### Scenario: Write skeleton stays the default
- GIVEN the packet template skeleton
- WHEN an author copies it without editing frontmatter
- THEN the example MUST omit `read_only` and criterion 2 MUST still require a commit

#### Scenario: Read-only note is present and actionable
- GIVEN a packet author who sets `read_only: true`
- WHEN they follow the template
- THEN the template MUST tell them to replace criterion 2 with the unchanged-tree check

#### Scenario: Human packet template is untouched
- GIVEN `human-packet-template.md`
- WHEN this change is applied
- THEN that file MUST be unmodified
