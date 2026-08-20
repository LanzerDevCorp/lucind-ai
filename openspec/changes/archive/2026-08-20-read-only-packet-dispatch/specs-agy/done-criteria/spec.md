# Done Criteria Specification

## Purpose

Define the contract and verification criteria for packet completion, establishing an unchanged-worktree requirement for read-only packets while preserving commit verification for write packets.

## Requirements

### Requirement: Write Packet Done Criterion 2 (Decision 2)

A write packet (`read_only: false` or omitted) MUST require that work is committed to the lane branch and the working tree is clean.

#### Scenario: Write packet completion evidence
- GIVEN a write packet being executed
- WHEN demonstrating satisfaction of mandatory done-criterion 2
- THEN evidence MUST provide an empty `git status --porcelain` and a valid commit via `git log --oneline -1`.

### Requirement: Read-Only Packet Done Criterion 2 (Decision 2)

A read-only packet (`read_only: true`) MUST replace mandatory done-criterion 2 with a requirement that the worktree carries no unique commits and no working-tree changes relative to the lane's birth point.

#### Scenario: Read-only packet zero mutation evidence
- GIVEN a read-only packet being executed
- WHEN demonstrating satisfaction of mandatory done-criterion 2
- THEN evidence MUST provide an empty `git status --porcelain` and proof that worktree `HEAD` equals `git merge-base HEAD <primary HEAD>`.

#### Scenario: Ignored protocol files permitted
- GIVEN a read-only packet execution writing `.lucind/result.json`
- WHEN checking working-tree cleanliness via `git status --porcelain`
- THEN gitignored `.lucind/` files MUST NOT appear in porcelain output and MUST NOT be considered working-tree mutations.

### Requirement: Result Envelope Schema Contract (Decision 2)

The result envelope (`.lucind/result.json`) MUST maintain the `commit` field as optional (`omitempty`), MUST NOT require `commit` for read-only packets, and MUST NOT permit post-hoc declaration or overriding of `read_only` within the envelope.

#### Scenario: Envelope commit omitted on read-only completion
- GIVEN a read-only packet completing execution
- WHEN generating `.lucind/result.json`
- THEN the envelope MUST validate against `result.schema.json` with the `commit` property omitted.

#### Scenario: Envelope prohibits read_only field injection
- GIVEN an agent generating `.lucind/result.json`
- WHEN attempting to include a `read_only` property in the envelope JSON
- THEN schema validation MUST reject the envelope due to `additionalProperties: false`.
