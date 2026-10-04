# lucind-ai

Glossary of the domain. Claude orchestrates, Antigravity (`agy`) implements, `lucind-ai` enforces
only what needs no judgment. See `docs/product.md` for the flow and CLI.

## Language

**Lane**:
One delegated unit of code-changing work run by agy in its own herdr pane. Identified by
`YYYYMMDD-HHMMSS-<4 hex>` (UTC) and stored under `<git toplevel>/.lucind/lanes/<id>/`.
_Avoid_: Agent, task, branch, or worktree

**Brief**:
Free Markdown sent to agy that states goal, scope, acceptance criteria and constraints. `dispatch`
appends a contract footer with the lane id, allowed globs, result path and the attest command.
_Avoid_: Packet, spec, or template

**Base tree / Final tree**:
Git tree hashes of the working tree (including uncommitted and untracked, non-ignored files). The
base tree is recorded at `dispatch`; the final tree is computed at `accept`. Changed files are the
diff between the two.
_Avoid_: Base commit or HEAD

**Allowed globs**:
The `--allow` patterns that bound where a lane may write. Enforced by agy's PreToolUse hook while
the lane runs and again by `accept` on every file changed since the base tree.
_Avoid_: Write scope or informal file list

**Result envelope**:
The `result.json` agy writes at the end of a lane, validated against
`internal/result/result.schema.json` and keyed by `lane_id`. The Stop hook re-enters agy with the
schema error at most twice before marking the lane `failed`.
_Avoid_: Packet or report

**Attestation**:
An HMAC-signed record of `{command, exit code, tree hash}` produced by `lucind-ai attest run` and
stored outside the repo. A passing attestation matching the final tree proves the checks ran.
_Avoid_: Test log or CI result

**Receipt**:
The `receipt.json` written by `accept`: base and final tree, changed files, verdict
(`accepted` or `rejected`), reasons and evidence.
_Avoid_: Approval or merge

**Free agy vs lane**:
Read-only work (explore, research) runs as a free agy session through herdr with no contract.
Anything that changes code runs as a lane. Nothing enforces read-only outside a lane, so the prompt
must say so.
_Avoid_: Using a lane for read-only work

**Plugin**:
The agy plugin embedded in the binary and installed by `lucind-ai plugin install`. It carries the
PreToolUse and Stop hooks plus the `lucind-lane` rule and `lucind-result` skill. Without
`LUCIND_LANE` every hook is a no-op.
_Avoid_: Extension or opencode plugin
