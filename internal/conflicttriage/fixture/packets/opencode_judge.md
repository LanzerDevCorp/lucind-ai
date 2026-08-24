---
id: fixture-opencode-judge
executor: opencode
model: openai/gpt-5.6-sol
routed_by: offline dual-judge rubric for the three-hunk fixture
feature: feat-judge-opencode
parent_ref: refs/heads/feature-judge-opencode
base_sha: 0123456789abcdef0123456789abcdef01234567
expected_parent_sha: 0123456789abcdef0123456789abcdef01234567
allowed_paths: ["internal/conflicttriage/fixture/packets/opencode_judge.md"]
read_only: true
---

# Opencode judge

Grade the three-hunk fixture. Separate the business hunk from the two mechanical
controls and declare ARBITRARY on the business hunk. Do not grade proposed_sha
or human timing. Emit JSON matching TriagePayload.
