# Packet sdd-fan-out-lens-skill-docs

## Goal

Update `SKILL.md` into the complete planning fan-out authoring and operator contract, covering parser admission fields, two-wave orchestration, precedence and compression, feature ownership, remediation, and the shipped CLI surface required by the integrated RED tests.

## Why this is safe to dispatch now

Every documentation outcome is specified against existing parser, CLI, and orchestration behavior; no runtime behavior is being designed. The packet owns one document and depends on integrated contract tests that pin its observable wording without sharing a writable path with the template packets.

## Preconditions

- Packet `sdd-fan-out-lens-contract-tests` has integrated the expanded `TestSkillAssetContract` assertions.
- `plugin/claude-code/skills/lucind-ai/SKILL.md:22-30`, `:126-253`, and `:284-300` still expose the documented update seams.
- Existing parser and CLI behavior cited below remains unchanged; this packet documents it and does not create it.

## Allowed paths

Only `plugin/claude-code/skills/lucind-ai/SKILL.md` may be modified.

## Allowed paths outside the repository

None.

## Out of scope

- Do not edit tests, templates, application/runtime/DAG/CLI code, build scripts, or any external skill installation.
- Do not weaken or rewrite the integrated RED assertions.
- Do not introduce a sidecar-based planning protocol, a fan-out generator, Go word-count enforcement, specs/tasks templates, or verify dual-dispatch work.
- Do not perform broad editorial rewrites, unrelated cleanup, dependency changes, or compatibility refactors.

## Context

- Owned tasks, quoted from `tasks.md`: **2.1, 2.2, 2.3, 2.4, 2.5** (`openspec/changes/sdd-fan-out-lens/tasks.md:34,36,38-41,58`). Strict TDD is active: make the integrated RED assertions pass without changing tests.
- The current frontmatter table at `plugin/claude-code/skills/lucind-ai/SKILL.md:22-30` omits `feature`, `parent_ref`, `base_sha`, `expected_parent_sha`, and `legacy_main`; the required contract is `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:37-51`.
- The current design-only pilot is at `plugin/claude-code/skills/lucind-ai/SKILL.md:126-151`. Promote the settled three-lens/two-wave model to the planning phases explore, propose, design, specs, and tasks, while preserving generic `lucind-ai run --packet` and explicitly not requiring `split` (`spec.md:5-20`).
- Existing lines `plugin/claude-code/skills/lucind-ai/SKILL.md:153-186` explain runtime-computed expected SHA, integration before synthesis, and silent admission failure. Generalize the protocol and add two-tier remediation: repair admission metadata before dispatch versus re-dispatch only a failed execution lane; synthesis starts only after all three lens IDs integrate (`openspec/changes/sdd-fan-out-lens/design.md:18-23`).
- Existing lines `plugin/claude-code/skills/lucind-ai/SKILL.md:199-227` establish compression and asymmetric precedence for design. Generalize them exactly as required by `spec.md:21-35`: phase skill owns document schema/content; packet owns topology, slices, ceilings, paths, and done criteria; canonical ceiling stays below the lens sum.
- Feature lifecycle ownership is settled at `openspec/changes/sdd-fan-out-lens/design.md:46-51`: the orchestrator runs `lucind-ai feature create` before dispatch; lanes do not create or move parent refs.
- The invocation block at `plugin/claude-code/skills/lucind-ai/SKILL.md:284-300` omits shipped subcommands `serve`, `feature`, `reconcile`, `renew` and run flags `--approval-timeout`, `--legacy-main`, `--expected-parent-sha`; retain the documented `run`, `split`, `check`, `--version`, `--packet`, and `--timeout` surfaces (`spec.md:37-39`).
- `internal/packet/packet_test.go:476-516` is the terminal automated reader for this document. Parser field truth comes from `internal/packet/packet.go:63-72,114-130`; CLI truth is cited in `spec.md:39` and `openspec/changes/sdd-fan-out-lens/design.md:69-76`.

## Done Criteria & Hard Stops

### Done criteria

- [ ] The focused `go test ./internal/packet -run TestSkillAssetContract` command passes with attached output; no assertion is weakened.
- [ ] `SKILL.md` documents all five feature-target keys, all required planning phases, integrated-HEAD wave 2, asymmetric precedence, the strict compression relation, feature lifecycle ownership, two-tier remediation, all named subcommands, and all named run flags. Evidence cites final file lines for each group.
- [ ] Every documented indirection has a terminal consumer: evidence traces feature keys to `packet.Parse`/admission, dispatch flags and subcommands to the existing CLI command/flag handlers, wave sequencing to the orchestrator procedure, and skill strings to `TestSkillAssetContract`. Merely listing a name is insufficient.
- [ ] `git diff -- internal/packet/packet_test.go` is empty, and `git diff --name-only` lists only the exact allowed path.
- [ ] No wording requires planning `apply-dag.yaml`/`split`, moves feature-parent refs from a lane, or claims the Go binary enforces editorial word ceilings.
- [ ] `git status --porcelain` confirms all changed files are within the exact allowed path.
- [ ] Commit the work with a conventional commit message. Evidence: `git status --porcelain` is empty and `git log --oneline -1` shows the commit, with no AI attribution in the message, trailers, source comments, or generated documentation.

### Hard stops

Return `status: blocked` instead of guessing if any of these fires:

- A required precondition is false or the RED dependency has not integrated the expected skill contract assertions.
- The work requires a path outside `plugin/claude-code/skills/lucind-ai/SKILL.md`.
- Existing parser/CLI behavior contradicts the design, tasks, or delta spec, or a requested documentation choice is not settled there.
- Focused verification fails outside this packet's scope; failures solely owned by concurrent template packets must be reported as such, not repaired here.
- A documented indirection has no identifiable terminal consumer in the parser, CLI, orchestrator procedure, or contract tests.
- Completing the work requires test changes, template changes, or implementation owned by another packet.
