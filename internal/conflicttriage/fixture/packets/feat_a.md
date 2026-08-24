---
# note: build-scope template; does not grant toy.go (GenerateFixture writes it via git)
id: fixture-feat-a
executor: cursor-agent
routed_by: sequential disjoint build-scope template for fixture feature A
feature: feat-conflict-a
parent_ref: refs/heads/feature-conflict-a
base_sha: 0123456789abcdef0123456789abcdef01234567
expected_parent_sha: 0123456789abcdef0123456789abcdef01234567
allowed_paths: ["fixture/feat-a/"]
---

# Fixture feature A

This packet is a build-scope template for sequential disjoint dispatch, not a
lane that writes the three-hunk toy. GenerateFixture creates toy.go through git
independently of these packets. Edit only `fixture/feat-a/`. Sequential dispatch;
do not combine with feature B in one batch.
