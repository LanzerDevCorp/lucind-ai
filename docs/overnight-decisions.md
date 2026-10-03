# Overnight decisions (herdr-agent-factory)

Autonomous session started 2026-10-03. Decision format: `D<n> | date-time | task | context | options | chosen | why | how to revert | commits`.

## Decisions

D-ROT-0 | 2026-10-03 | preflight | `agy-pool list`: 3 saved profiles (lanzerdev20, ponenofeik5, corp.systems.lanzer), each containing only `google_accounts.json` and `oauth_creds.json`; none has `antigravity-oauth-token`, so `agy-pool use` would not change the account `agy` uses | enable rotation / disable | ACCOUNT ROTATION DISABLED all night | rule R0.3 of the mission | n/a (rotation is simply not used) | none

D1 | 2026-10-03 | T1b | attestations were keyed by sha256(toplevel path), invisible from the primary root when made in a lane worktree | key by toplevel (keep) / key by git common dir / store the entry in the lane and copy | key by git common dir | all worktrees of one repo share a namespace; the tree hash (not the path) binds an attestation to content; smallest change | revert `RepoID` input in `internal/attest` and callers in `cmd/lucind-ai/attest.go` | 6b82c49, aa8a693

D2 | 2026-10-03 | T1b | one unexplained `FAIL` count (3) from `go test ./...` on the feature branch right after `make install`; 3 reruns clean | investigate with `-race -count=20` / record | recorded | identified later: `TestLeaseAcquisitionAndMonotonicFence` (internal/feature/feature_test.go:297) is timing-flaky when the whole suite runs in parallel; passes 3/3 in isolation; unrelated to attest/packet changes. Treat a lone failure of it as a flake, rerun that package | n/a | n/a

Note for tomorrow: the T1 worktree `~/git_root/lucind-ai-worktrees/lane-t1-hmac-attestation` and branch `lane/t1-hmac-attestation` are kept (deletion is forbidden overnight).

## Registro de rotaciones

- 2026-10-03 | lanzerdev20@gmail.com | preflight | `list`/`current` read-only; active account lanzerdev20, usage cache 98%; no profile has `antigravity-oauth-token` -> rotation disabled (D-ROT-0)
