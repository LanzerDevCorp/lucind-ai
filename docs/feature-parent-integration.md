# Feature parent integration

Lucind can dispatch work onto a named feature parent (not `main`), keep attempts and
overlap/reconciliation durable in the SQLite ledger, and recover interrupted promotions. It never
merges that parent into `main`, never owns review, and never owns delivery.

This page is the operator guide for the CLI shipped with schema v4. Flag names below match
`lucind-ai` usage text (`cmd/lucind-ai/cli.go`).

## Never promote to `main`

Lucind advances **the named feature parent only**. Feature closure and integration to `main` stay
fully external and manual.

When a parent is ready for delivery:

- Lucind does **not** fast-forward, merge, or otherwise update `main`.
- Lucind does **not** open a PR, drive review, or mark delivery complete.
- `main`, review state, and delivery state remain whatever they were before Lucind ran.

You close a feature the same way you would without Lucind: review the parent branch, merge it to
`main` (or not) in your own workflow, and leave Lucind out of that step. Managed parents reject
`main` / `refs/heads/main` at `feature create`; the only supported use of `main` is the **legacy
dispatch** path below, which names `main` for worktree targeting and still never promotes onto it.

## Quick path

1. Record the feature parent at a known commit:

   ```bash
   BASE=$(git rev-parse HEAD)
   lucind-ai feature create \
     --id user-auth \
     --parent refs/heads/feature-user-auth \
     --base-sha "$BASE" \
     --expected-parent-sha "$BASE"
   ```

2. Dispatch a packet that names the same four target fields (`feature`, `parent_ref`, `base_sha`,
   `expected_parent_sha`). Packets that omit them fail before a worktree is created, unless you
   declare legacy mode.

3. Inspect and, if an attempt was interrupted, recover:

   ```bash
   lucind-ai feature status --id user-auth
   lucind-ai feature recover --attempt <attempt-id>
   ```

4. If overlap requires a human decision, approve, decline, cancel, or renew the request. Then merge
   the feature parent to `main` yourself when — and only when — you want it on `main`.

## Commands

Every example uses the flags printed by the binary. Optional flags are shown in brackets, matching
that usage text.

### `feature create`

```text
usage: lucind-ai feature create --id <id> --parent <ref> --base-sha <sha> [--expected-parent-sha <sha>]
```

| Flag | Required | Meaning |
|------|----------|---------|
| `--id` | yes | Feature identifier (caller-chosen; reuse with the same parent/base is idempotent). |
| `--parent` | yes | Parent branch ref. `--parent-ref` is an alias. `main` / `refs/heads/main` is rejected. |
| `--base-sha` | yes | Immutable start commit of the parent. |
| `--expected-parent-sha` | no | Expected tip of the parent before the next promotion. |

```bash
lucind-ai feature create \
  --id user-auth \
  --parent refs/heads/feature-user-auth \
  --base-sha "$BASE"

lucind-ai feature create \
  --id user-auth \
  --parent-ref refs/heads/feature-user-auth \
  --base-sha "$BASE" \
  --expected-parent-sha "$BASE"
```

`--expected-parent-sha` is optional on this command. Packets still need all four target fields
unless they declare legacy mode.

### `feature status`

```text
usage: lucind-ai feature status [--id <id>]
```

```bash
lucind-ai feature status
lucind-ai feature status --id user-auth
```

Omit `--id` to list every registered feature (and any lease). Pass `--id` for one feature, its
attempts, and lease detail.

### `feature recover`

```text
usage: lucind-ai feature recover --attempt <id>
```

```bash
lucind-ai feature recover --attempt 3f2c1a8e-9b10-4c2d-a111-0b9c8d7e6f55
```

Recovery compares recorded refs to the live parent. Unchanged expected/current refs may resume;
a candidate ref that already matches means CAS succeeded and ledger finalization is replayed;
any other or missing ref stays blocked and keeps worktrees and evidence inspectable. A completed
attempt returns its stored result without promoting again.

### `reconcile approve`

```text
usage: lucind-ai reconcile approve --request <id> --source <feature> --target <feature> [--actor <name>] [--allowed-paths <paths>]
```

```bash
lucind-ai reconcile approve \
  --request 7aa1b2c3-d4e5-6789-abcd-ef0123456789 \
  --source user-auth \
  --target billing

lucind-ai reconcile approve \
  --request 7aa1b2c3-d4e5-6789-abcd-ef0123456789 \
  --source user-auth \
  --target billing \
  --actor "$USER" \
  --allowed-paths internal/auth/token.go,internal/billing/invoice.go
```

`--actor` defaults to the current OS username. `--allowed-paths` is a comma-separated override of
paths the candidate resolver may touch.

### `reconcile decline`

```text
usage: lucind-ai reconcile decline --request <id> [--actor <name>] [--reason <reason>]
```

```bash
lucind-ai reconcile decline \
  --request 7aa1b2c3-d4e5-6789-abcd-ef0123456789 \
  --reason "overlap is informational; keep the features independent"
```

### `reconcile cancel`

```text
usage: lucind-ai reconcile cancel --request <id> [--actor <name>] [--reason <reason>]
```

```bash
lucind-ai reconcile cancel \
  --request 7aa1b2c3-d4e5-6789-abcd-ef0123456789 \
  --actor "$USER" \
  --reason "superseded by a new overlap evaluation"
```

### `reconcile renew`

```text
usage: lucind-ai reconcile renew --request <id> [--base-sha <sha>] [--source-sha <sha>] [--target-sha <sha>] [--ttl <duration>]
```

```bash
lucind-ai reconcile renew \
  --request 7aa1b2c3-d4e5-6789-abcd-ef0123456789 \
  --base-sha "$BASE" \
  --ttl 15m
```

`--ttl` is a Go duration (default `15m`). `--source-sha`/`--target-sha` are optional overrides;
an omitted flag defaults to that feature's own current real `parent_ref` tip (falling back to
`expected_parent_sha`, then `base_sha` — the same fallback chain the overlap gate itself uses),
resolved live at renew time rather than carried forward from whatever the request being renewed
already had stored. This matters because the very first, automatically-created request for a
feature pair may have been seeded from a value that is not either feature's real branch tip (e.g.
an internal integration-lens merge commit); relying on that stored value across every renewal
would pin the reconciliation to a SHA neither feature is ever actually at, so the reuse check in
`evaluateOverlapGate` (`matchedOtherSHA == otherSHA`) would never converge no matter how many
times `approve`/`resolve` runs. Pass `--source-sha` and/or `--target-sha` explicitly only when
pinning a specific historical SHA on purpose. The same flags are accepted as the top-level alias
`lucind-ai renew` (same handler as `reconcile renew`).

## Legacy packets that do not name a feature

Existing packets that never declared `feature` / `parent_ref` / `base_sha` / `expected_parent_sha`
fail closed at admission. To dispatch them against `main` without implying promotion, declare
legacy mode **and** the expected SHA of `main`.

CLI flags on `run`:

```text
lucind-ai run --packet <path> [--packet <path> ...] [--timeout <duration>] [--approval-timeout <duration>] [--legacy-main] [--expected-parent-sha <sha>]
```

```bash
lucind-ai run \
  --packet .lucind/packets/legacy-lane.md \
  --legacy-main \
  --expected-parent-sha "$(git rev-parse refs/heads/main)"
```

`--legacy-main` without an expected SHA fails unless every packet already has frontmatter
`expected_parent_sha`. `--legacy-main` sets `LegacyMain` on the batch; `--expected-parent-sha`
fills that field only when the packet left it empty.

Equivalent frontmatter (the 2.1 packet fields, not extra CLI flags):

```yaml
---
id: legacy-lane
executor: agy
routed_by: pre-feature-parent packet
legacy_main: true
expected_parent_sha: 0123456789abcdef0123456789abcdef01234567
---

Work the packet as before. Do not infer a feature parent from the checkout.
```

`legacy_main` must be the boolean `true` or `false`. Combined with a non-empty
`expected_parent_sha`, admission fills a missing `parent_ref` with `main` and proceeds. This
declares `main` for **dispatch compatibility only**. It does not authorize a promotion onto
`main`.

Packets that are not in legacy mode must set all four of `feature`, `parent_ref`, `base_sha`, and
`expected_parent_sha`. Lucind does not infer any of them from `HEAD`.

## Schema v4 migration and rollback

First open of a ledger on a v4 binary migrates **additively** from v3. v3 data (lanes, events,
approvals) is preserved. v4 adds seven tables and does not rewrite or drop v3 rows:

| Table | Holds |
|-------|--------|
| `features` | Feature id, parent ref, base SHA, expected parent SHA, `created` / `active` / `disabled` |
| `integration_attempts` | Attempt identity, lease fence, candidate SHA, terminal result |
| `feature_leases` | Per-feature owner, fence, expiry |
| `overlap_evidence` | Versioned overlap JSON, hash, class |
| `reconciliation_requests` | Direction, actor, evidence binding, expiry, `awaiting` / `approved` / `declined` / `cancelled` / `expired` |
| `reconciliation_candidates` | Resolver output, allowed paths, model, checks, candidate SHA |
| `integration_events` | Append-only audit |

There is no down-migration. Rolling the **binary** back to a pre-v4 `lucind-ai` leaves those tables
in SQLite untouched. The older binary has no `feature` or `reconcile` subcommands, so those
commands disappear from the CLI while v4 rows remain. Reinstalling a v4 binary resumes against the
same data; `migrate` is a no-op when `schema_migrations` already records version 4.

Do not delete v4 tables to "undo" the feature. That is data loss, not rollback.

## Optional real-Sonnet end-to-end probe

Automated tests never call a live model. They inject a fake resolver and only assert that a
reconciliation candidate records model `sonnet`.

The optional probe is a **manual**, **subscription-dependent** check of the real invoker:

- **What it is.** `internal/resolve.RealInvoker` runs

  ```bash
  claude -p --model sonnet --permission-mode acceptEdits --output-format json
  ```

  in the candidate worktree (prompt on stdin, JSON on stdout), with a 400-line conflict-marker
  bound and a five-minute timeout. After a successful resolve, the binary still runs
  `lucind-checks.sh` and only then CAS-promotes the **feature parent** (never `main`).

- **Why it is manual and optional.** It needs a live Anthropic Claude Code subscription (`claude`
  on `PATH`, already authenticated). That is the same "never a metered API key" constraint as the
  rest of Lucind. CI cannot assume the subscription, the network, or a conflicted tree of bounded
  size, so `go test ./...` is not this probe.

- **How to run it if you choose to.** Create two overlapping features, let promotion record a
  required-overlap reconciliation request, and `reconcile approve` it so a candidate worktree is
  built with merge conflicts **under** 400 conflict-marker lines. With `claude` authenticated,
  Lucind invokes Sonnet for that candidate. Confirm the candidate reaches `integrated` (or a
  recorded `failed`/`stale` with preserved artifacts), that `lucind-checks.sh` ran, and that
  `main` did not move.

  To confirm the argv alone, without Lucind's promotion path, run the same `claude` invocation by
  hand in a scratch conflicted worktree. That checks the subscription and the binary name; it does
  not replace the full candidate/CAS path above.

Skip the probe when no Anthropic subscription is available. The feature is still covered by the
table-driven and `t.TempDir()` Git/SQLite tests.

## Operator checklist

- [ ] `feature create` used a non-`main` parent ref and a real base SHA.
- [ ] Packets name `feature`, `parent_ref`, `base_sha`, and `expected_parent_sha`, **or** declare
      `legacy_main: true` / `--legacy-main` plus `expected_parent_sha` / `--expected-parent-sha`.
- [ ] `feature status` / `feature recover` are the inspection and resume tools; do not delete
      worktrees of blocked or stale attempts.
- [ ] Overlap decisions go through `reconcile approve|decline|cancel|renew`.
- [ ] Closing the feature means merging the parent to `main` yourself. Lucind will not do it.
- [ ] Binary rollback leaves v4 tables in place; do not DROP them.
- [ ] The Sonnet probe stays optional and off CI.
