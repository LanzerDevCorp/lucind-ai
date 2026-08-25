# Delta for Stability Evidence Receipt

## ADDED Requirements

### Requirement: Bounded evidence sanitization

Evidence captured during campaign execution MUST be sanitized before worktree cleanup by capping stream captures to 4096 bytes, stripping absolute paths and credentials, and hashing raw stream payloads.

#### Scenario: Diagnostic log sanitization

- GIVEN diagnostic streams exceeding 4096 bytes
- WHEN evidence records are persisted
- THEN output MUST be truncated to 4096 bytes, payloads hashed, and absolute paths stripped

### Requirement: Terminal stability receipt generation

A campaign passing all three sequential Trials and post-cleanup baseline check MUST persist an immutable canonical JSON Stability Receipt (RFC 8785) binding candidate commit SHA, build version, fixture digests, Trial records, and pass verdict.

#### Scenario: Stability receipt generation

- GIVEN three passed trials and clean baseline check
- WHEN the campaign completes successfully
- THEN a canonical JSON Stability Receipt MUST be written binding candidate SHA, build, and trial records

### Requirement: Non-mutating delivery boundary

Completion and certification of a Stability Campaign MUST NOT create Git tags, push commits to remotes, mutate release branches, bump semantic versions, or create issue tracker records.

#### Scenario: Non-mutating release exit

- GIVEN a certified campaign with approved receipt
- WHEN execution completes
- THEN the command MUST exit 0 without creating git tags or pushing to remotes

#### Scenario: Prohibited release flag rejection

- GIVEN `stability run` invoked with `--push`, `--tag`, or `--release`
- WHEN arguments are parsed
- THEN the command MUST reject invalid flags and halt non-zero
