# Delta for packet-authoring-contract

## ADDED Requirements

### Requirement: Target-Free Packet Authoring and Late Binding

Packet templates MUST be authorable without hardcoded feature targets and SHALL bind feature identity, parent ref, and base SHA dynamically at wave dispatch. Packets omitting `allowed_paths` MUST default to open scope without triggering diff-boundary validation failures.

#### Scenario: Target-free packet template binds feature target at dispatch

- GIVEN a packet template authored without feature target fields
- WHEN the orchestrator supplies feature, parent ref, and base SHA at wave dispatch
- THEN packet parsing and admission MUST bind the target and admit the packet into the wave

#### Scenario: Packet omitting allowed paths defaults to open scope safely

- GIVEN a packet template that omits `allowed_paths`
- WHEN the packet is parsed and admitted
- THEN AllowedPaths MUST remain empty and diff-boundary scope checks MUST be skipped

#### Scenario: Malformed frontmatter or invalid JSON array fails admission

- GIVEN a packet with invalid frontmatter or a non-array `allowed_paths` value
- WHEN packet parsing validates the document
- THEN parsing MUST return a schema error and reject admission before worktree creation
