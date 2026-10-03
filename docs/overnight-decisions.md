# Overnight decisions (herdr-agent-factory)

Autonomous session started 2026-10-03. Decision format: `D<n> | date-time | task | context | options | chosen | why | how to revert | commits`.

## Decisions

D-ROT-0 | 2026-10-03 | preflight | `agy-pool list`: 3 saved profiles (lanzerdev20, ponenofeik5, corp.systems.lanzer), each containing only `google_accounts.json` and `oauth_creds.json`; none has `antigravity-oauth-token`, so `agy-pool use` would not change the account `agy` uses | enable rotation / disable | ACCOUNT ROTATION DISABLED all night | rule R0.3 of the mission | n/a (rotation is simply not used) | none

D1 | 2026-10-03 | T1b | attestations were keyed by sha256(toplevel path), invisible from the primary root when made in a lane worktree | key by toplevel (keep) / key by git common dir / store the entry in the lane and copy | key by git common dir | all worktrees of one repo share a namespace; the tree hash (not the path) binds an attestation to content; smallest change | revert `RepoID` input in `internal/attest` and callers in `cmd/lucind-ai/attest.go` | 6b82c49, aa8a693

D2 | 2026-10-03 | T1b | one unexplained `FAIL` count (3) from `go test ./...` on the feature branch right after `make install`; 3 reruns clean | investigate with `-race -count=20` / record | recorded | identified later: `TestLeaseAcquisitionAndMonotonicFence` (internal/feature/feature_test.go:297) is timing-flaky when the whole suite runs in parallel; passes 3/3 in isolation; unrelated to attest/packet changes. Treat a lone failure of it as a flake, rerun that package | n/a | n/a

D3 | 2026-10-03 | T3 | legacy lanes carrying only `sdd_phase` (explore/spec/...) and no `lane_role`/`read_only` used to skip mechanical checks; the new predicate runs them | keep skipping on legacy SDD phases / fail closed | fail closed (checks run unless the lane is declared read-only or has a non-writing role) | the mission asks for a fail-closed role/read_only predicate; legacy SDD packets are being retired (T4/T5) | revert `RequiresMechanicalChecks` to the SDDPhase condition in accept.go and attempt.go | d5d07f2

D4 | 2026-10-03 | T4 | "make sdd-* derivation optional": lane roles apply/verify/archive always added sdd-apply/verify/archive | keep as-is / opt-in via explicit sdd_phase / new flag | opt-in via explicit sdd_phase (reuses the existing field, no new API) | consistent with how lens/synthesis already gate sdd-<phase>; ODD packets get no sdd-* skills | revert the three `if sddPhase != ""` guards in internal/skillset/skillset.go; NOTE required_skills (hence packet digest) of role-only apply/verify/archive packets change, so a replay of an old such packet gets a new digest | 85b4a24

D5 | 2026-10-03 | T8 | blind review item: the exit sentinel nonce is readable from run.sh by the agent (it could print a forged sentinel) | delete run.sh before running agy / accept | accepted | agy already runs with --dangerously-skip-permissions and can write any file, so exit code and output are not a security boundary; trust comes from the dispatcher (attestation, allowed_paths diff, judges). A forged sentinel without a matching exit.code fails closed (read error) | n/a | f721ce8
D6 | 2026-10-03 | T8 | review items: no hard-kill after C-c grace; state dirs of failed runs are never reaped | fix now / new task | new task T16 | closing panes/workspaces needs a policy for reused panes (rule: never close panes you did not create) that deserves its own task | n/a | f721ce8

D7 | 2026-10-03 | T9 | dispatcher commit and repository hooks | run hooks (repo policy) / --no-verify | --no-verify | attested verification already gates quality; hooks could be redirected by the worker (core.hooksPath) or add trailers, and would run with dispatcher authority; two blind reviewers recommended it | remove --no-verify in commit_step.go (defaultGitCommit) | bed8d65
D8 | 2026-10-03 | T9 | packet field design: commit_message requires verification; commit obligation value dispatcher; envelope.Commit must be empty | worker may also commit / dispatcher-only | dispatcher-only for packets that declare commit_message | one clear owner of the commit; legacy packets unchanged | drop CommitMessage handling in run.Execute and the dispatcher branch in accept | bed8d65

Note for tomorrow: the T1 worktree `~/git_root/lucind-ai-worktrees/lane-t1-hmac-attestation` and branch `lane/t1-hmac-attestation` are kept (deletion is forbidden overnight).

## Registro de rotaciones

- 2026-10-03 | lanzerdev20@gmail.com | preflight | `list`/`current` read-only; active account lanzerdev20, usage cache 98%; no profile has `antigravity-oauth-token` -> rotation disabled (D-ROT-0)
