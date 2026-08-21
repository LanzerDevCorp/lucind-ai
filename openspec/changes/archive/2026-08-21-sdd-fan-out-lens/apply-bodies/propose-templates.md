# Packet sdd-fan-out-lens-propose-templates

## Goal

Create the four propose fan-out packet templates so three disjoint lens packets and one integrated-tree synthesizer satisfy the accepted planning protocol and the integrated RED contract tests.

## Why this is safe to dispatch now

The filenames, phase ownership, two-wave topology, precedence rule, and parser contract are settled in the design and delta spec. The packet is isolated to four new files and depends on the integrated RED tests that objectively define its GREEN boundary.

## Preconditions

- Packet `sdd-fan-out-lens-contract-tests` has integrated tests that parse the propose templates and check phase-local lens-path disjointness.
- The four allowed files do not exist before execution.
- Existing design templates remain available as read-only format precedents; they are not writable inputs.

## Allowed paths

Only these files may be created:

- `plugin/claude-code/skills/lucind-ai/assets/propose-lens-a-packet-template.md`
- `plugin/claude-code/skills/lucind-ai/assets/propose-lens-b-packet-template.md`
- `plugin/claude-code/skills/lucind-ai/assets/propose-lens-c-packet-template.md`
- `plugin/claude-code/skills/lucind-ai/assets/propose-synthesis-packet-template.md`

## Allowed paths outside the repository

None.

## Out of scope

- Do not edit tests, `SKILL.md`, explore/design templates, application code, runtime/DAG/CLI code, or build scripts.
- Do not weaken or rewrite the integrated RED assertions.
- Do not add specs/tasks templates, a generic template generator, broad formatting, dependency changes, or unrelated cleanup.
- Do not create planning artifacts from these templates; the files are passive packet assets only.

## Context

- Owned tasks, quoted from `tasks.md`: **1.3, 1.4** (`openspec/changes/sdd-fan-out-lens/tasks.md:28-29,57`). Strict TDD is active: make the integrated RED assertions pass for this owned slice without changing tests.
- The design requires dedicated `propose-lens-{a,b,c}-packet-template.md` and `propose-synthesis-packet-template.md` files at `openspec/changes/sdd-fan-out-lens/design.md:11-16`.
- Lens ownership is fixed at `openspec/changes/sdd-fan-out-lens/design.md:85`: A owns candidate and approach; B owns capability impact and specs; C owns risks, rollback, and test impact.
- The two-wave contract is `openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:5-20`; phase-skill versus packet precedence and the compression rule are at `:21-35`.
- Every template must parse, declare valid target admission, and give parallel lens lanes mutually disjoint draft paths (`openspec/changes/sdd-fan-out-lens/specs/sdd-planning-fan-out/spec.md:53-73`). The templates use `legacy_main: true`; dispatch supplies the expected main SHA at runtime, as currently documented at `plugin/claude-code/skills/lucind-ai/SKILL.md:157-176`.
- Existing design packet assets demonstrate required reading, slice ownership, result-envelope duties, and citation verification; `plugin/claude-code/skills/lucind-ai/assets/design-synthesis-packet-template.md:31-64` shows lens ownership and citation arbitration.
- The synthesizer must produce canonical `proposal.md` plus sectioned synthesis notes, verify every `file:line`, and keep its canonical ceiling strictly below the sum of the three lens ceilings (`openspec/changes/sdd-fan-out-lens/design.md:32-44,59-65`).
- The work-unit table deliberately grants individual files, not the parent `assets/` directory (`openspec/changes/sdd-fan-out-lens/tasks.md:51,57`). This conservative literal-file boundary prevents same-wave overlap with explore templates.

## Done Criteria & Hard Stops

### Done criteria

- [ ] All four allowed files exist and `git diff --name-only` lists no other changed path.
- [ ] The integrated propose-focused test/subtest passes and proves all four files parse with `packet.Parse`, each admits through `legacy_main: true` or complete feature fields, and the three lens `allowed_paths` are pairwise disjoint. Attach command and output.
- [ ] Each lens body names its exclusive slice, required reading, output skeleton, sibling ownership, and word ceiling; the synthesis body names all three drafts, canonical `proposal.md`, synthesis notes, citation verification, and a canonical ceiling strictly below the lens-ceiling sum. Evidence cites the created file lines.
- [ ] Every frontmatter or body indirection is demonstrably consumed: trace each allowed draft path through `packet.Parse`/`DisjointAllowedPaths`, and trace each lens output into the synthesizer's required-reading list and final canonical/notes outputs. A definition or forwarding mention alone is insufficient.
- [ ] Do not weaken an assertion. `git diff -- internal/packet/packet_test.go` must be empty.
- [ ] `git status --porcelain` confirms all changed files are within the four exact allowed paths.
- [ ] Commit the work with a conventional commit message. Evidence: `git status --porcelain` is empty and `git log --oneline -1` shows the commit, with no AI attribution in the message, trailers, source comments, or generated documentation.

### Hard stops

Return `status: blocked` instead of guessing if any of these fires:

- A required precondition is false or the RED dependency has not integrated the expected propose contract tests.
- The work requires any path outside the four exact allowed files.
- The sources leave a lens slice, output path, notes section, precedence rule, or budget relation undocumented or contradictory.
- Focused verification fails outside this packet's scope; failures solely owned by concurrent explore/docs packets must be reported as such, not repaired here.
- A required indirection has no identifiable terminal consumer.
- Completing the work requires test changes or implementation owned by another packet.
