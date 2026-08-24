---
# note: build-scope template; does not grant toy.go (GenerateFixture writes it via git)
id: fixture-feat-b
executor: cursor-agent
routed_by: sequential disjoint build-scope template for fixture feature B
feature: feat-conflict-b
parent_ref: refs/heads/feature-conflict-b
base_sha: 0123456789abcdef0123456789abcdef01234567
expected_parent_sha: 0123456789abcdef0123456789abcdef01234567
allowed_paths: ["fixture/feat-b/"]
---

# Fixture feature B

This packet is a build-scope template for sequential disjoint dispatch, not a
lane that writes the three-hunk toy. GenerateFixture creates toy.go through git
independently of these packets. Edit only `fixture/feat-b/`. Sequential dispatch;
do not combine with feature A in one batch.
