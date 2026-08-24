# Control room operations

Load this module for `serve`, approvals, ledger status, monitoring, or operator control.

## Entry

Run `lucind-ai -v` before dispatch and compare the embedded build with the checkout. Use the binary's usage output for current command syntax; the CLI does not currently expose a successful conventional top-level `--help` path, but invalid or empty invocation prints usage.

`lucind-ai serve` starts the approvals and status HTTP surface. Dispatch through the server is separately gated and must not be inferred from monitoring availability. The current Lane timeout defaults to 20 minutes and approval waits to 30 minutes; `--approval-timeout` bounds approval independently of Lane execution. Synthesis Lanes require an explicit larger timeout.

## Durable state

The SQLite ledger under `.lucind/` records Lane registrations, transitions, barrier events, features, attempts, leases, overlap evidence, and reconciliation state. It is authoritative runtime evidence within one local repository on one machine, not distributed coordination across clones.

Use feature status to inspect feature rows and attempts. Use the attempt ID printed by a feature run for recovery. Feature lease renewal and reconciliation-request renewal are different operations; never substitute one for the other.

## Operator report

Track Lane ID, worktree, status, result path, integrated or reverted classification, attempt and lease state, overlap request, check outcome, and preserved evidence. Keep decisions in durable state; a dashboard view or provider conversation is not the source of truth.
