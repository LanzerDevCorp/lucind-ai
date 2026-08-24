---
id: fixture-claude-judge
executor: claude
model: claude-opus-5
routed_by: offline dual-judge rubric for the three-hunk fixture
feature: feat-judge-claude
parent_ref: refs/heads/feature-judge-claude
base_sha: 0123456789abcdef0123456789abcdef01234567
expected_parent_sha: 0123456789abcdef0123456789abcdef01234567
allowed_paths: ["internal/conflicttriage/fixture/packets/claude_judge.md"]
read_only: true
---

# Claude judge

Grade the three-hunk fixture. Separate the business hunk from the two mechanical
controls and declare ARBITRARY on the business hunk. Do not grade proposed_sha
or human timing. Emit JSON matching TriagePayload.
