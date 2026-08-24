---
id: fixture-feat-a
executor: cursor-agent
routed_by: sequential fixture dispatch for feature A of the three-hunk overlap toy
feature: feat-conflict-a
parent_ref: refs/heads/feature-conflict-a
base_sha: 0123456789abcdef0123456789abcdef01234567
expected_parent_sha: 0123456789abcdef0123456789abcdef01234567
allowed_paths: ["fixture/feat-a/"]
---

# Fixture feature A

Edit only `fixture/feat-a/`. Sequential dispatch; do not combine with feature B in one batch.
