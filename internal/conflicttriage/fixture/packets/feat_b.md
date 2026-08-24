---
id: fixture-feat-b
executor: cursor-agent
routed_by: sequential fixture dispatch for feature B of the three-hunk overlap toy
feature: feat-conflict-b
parent_ref: refs/heads/feature-conflict-b
base_sha: 0123456789abcdef0123456789abcdef01234567
expected_parent_sha: 0123456789abcdef0123456789abcdef01234567
allowed_paths: ["fixture/feat-b/"]
---

# Fixture feature B

Edit only `fixture/feat-b/`. Sequential dispatch; do not combine with feature A in one batch.
