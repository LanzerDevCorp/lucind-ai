# HMAC Test Attestation

`lucind-ai attest` provides deterministic, tamper-evident test attestation. It allows an orchestrator or automated agent to verify that a required test command has already run and passed against the current state of a repository's working tree, avoiding redundant test execution when nothing has changed.

## Commands

### `lucind-ai attest run -- <command> [args...]`

Executes the specified command in the current working directory:
- Passes standard input, output, and error streams through directly.
- Records timestamps (`started_at` and `finished_at`).
- After the command terminates, computes a git tree hash of the working tree (including uncommitted edits and untracked, non-ignored files) without modifying the repository's real git index.
- Computes an HMAC-SHA256 signature (`mac`) across canonical fields of the attestation entry.
- Atomically writes the attestation record to the state directory and marks it read-only (`0444`).
- Exits with the target command's exact exit code. Even if the command fails (`exit_code != 0`), an attestation entry is recorded; failing entries never count as a passing attestation.

Example:
```bash
lucind-ai attest run -- go test ./...
```

### `lucind-ai attest verify --command "<exact command string>"`

Verifies that a valid attestation exists for the specified command in the current repository:
- Exits `0` only if an attestation entry exists where:
  1. `command` matches the exact string.
  2. `exit_code == 0`.
  3. `tree_hash` matches the current working tree hash.
  4. The HMAC-SHA256 signature (`mac`) verifies with the local secret key.
- Otherwise, exits `1` and prints a single short reason to stderr:
  - `no entry`: No attestation entry found for this command in this repository.
  - `tree changed`: The working tree has uncommitted edits, untracked files, or deletions since the last test run.
  - `tests failed`: The recorded test execution failed (`exit_code != 0`).
  - `bad mac`: The attestation entry was forged, corrupted, or tampered with.

Example:
```bash
lucind-ai attest verify --command "go test ./..."
```

## Threat Model

The attestation mechanism's threat model is **accidental, not adversarial**:
- **Protected against**: Accidental repeats, out-of-order execution, stale cached results, forgotten uncommitted edits, newly added untracked files, deleted files, and inadvertent corruption.
- **Out of scope**: Adversarial circumvention by the local user. The secret signing key is owned by and readable by the local user account (`mode 0600`). A user or process running under the same UID with access to the key file could technically compute valid HMAC signatures. The purpose is coordination integrity and reliable orchestration, not defense against a hostile local user.

## Secret Key and Storage

- **Key location**: `$XDG_CONFIG_HOME/lucind-ai/attest.key` (fallback: `~/.config/lucind-ai/attest.key`).
  - 32 cryptographically secure random bytes generated on first use.
  - File permissions are restricted to `0600`.
  - **Atomic key creation**: The key is written in full (mode `0600`) to a private temp file in the key directory and published with `os.Link`, which fails with `EEXIST` if another process won the race; the loser then reads the winner's complete key. Readers never see a partial key under the final name, even if the creator dies mid-write, and processes racing on first use converge on identical key bytes. A key file that is not exactly 32 bytes is rejected.
  - Keys are never written inside any repository or printed to logs/terminals.
- **Log location**: `$XDG_STATE_HOME/lucind-ai/attestations/<repo_id>/` (fallback: `~/.local/state/lucind-ai/attestations/<repo_id>/`).
  - `<repo_id>` is the SHA-256 hex string of the absolute repository git common directory path (`git rev-parse --git-common-dir`, made absolute and cleaned relative to the directory it was run in). This ensures all linked git worktrees and the primary repository root share the same attestation namespace, while different repositories remain isolated.
  - Individual entries are stored as `<finished_at unix nanos>-<tree_hash[:12]>.json`.
  - **0444-before-rename**: Log entries are written to a temporary file (`*.tmp`), permissions are set to `0444` (read-only) *before* `os.Rename`, and the file is then renamed into place. Log entries never exist as writable files under their final name, eliminating the post-rename writable permission window.

## Reuse in Acceptance (`lucind-ai accept`)

`lucind-ai accept` reuses a valid attestation instead of redundantly re-running `lucind-checks.sh`:
- **Attested command**: The canonical attested command string is exactly `sh lucind-checks.sh` (matching `integrate.Check`).
- **Matching criteria**: In `Verifier.Verify`, before running mechanical checks, the verifier looks for an existing attestation satisfying all of:
  1. Exact command: `sh lucind-checks.sh`.
  2. Exit code: `0`.
  3. Valid HMAC-SHA256 signature (`mac`) matching the local secret key.
  4. Tree hash: `TreeHash` matches the git tree hash of the frozen candidate commit (`git rev-parse <CandidateCommit>^{tree}` in the primary root).
- **Attested acceptance**: If a valid attestation is found:
  - Mechanical check execution (`v.check`) is skipped.
  - `version` is set to `attest:v1`.
  - `output` is set to `attested:<tree-hash>`.
  - Owned isolation is still created and cleaned as usual.
  - The receipt is persisted with `ChecksHash` computed over `checks:v1`, `attest:v1`, and `attested:<tree-hash>`, strictly distinguishing attested receipts from fresh execution receipts.
- **Fallback safety**: If no attestation exists, or if the MAC is invalid, tree hash mismatches, exit code is non-zero, command differs, or attestation lookup encounters an error, the verifier falls back to running `lucind-checks.sh` in owned isolation. Invalid attestations fail closed and never bypass verification.

## Working Tree Hashing and Ignored Files

To ensure any uncommitted change or untracked file invalidates attestation:
1. A temporary index file path is created outside the repository.
2. The index is seeded using `git read-tree HEAD` with `GIT_INDEX_FILE` pointing to the temporary index.
3. Uncommitted and untracked (non-ignored) files are added using `git add -A`.
4. The tree object is written using `git write-tree`.
5. The temporary index file is deleted.

The repository's actual index (`.git/index`) is never read, locked, or modified.

### Ignored-Files Rule

Git tree hashing excludes files that match `.gitignore` patterns:
- **Covered**: Tracked files, modified tracked files, staged changes, and untracked non-ignored files are all captured in `TreeHash`.
- **Not covered**: Ignored files (e.g. build artifacts, virtualenvs, local caches, `.DS_Store`) are excluded from `git add -A` and do not alter `TreeHash`.
- Because ignored files are excluded from `TreeHash`, an isolated checkout of the candidate commit hashes identically to the committed tree of a worktree, allowing clean deterministic verification across worktrees and detached isolations.

## Dispatcher commit

For packets declaring `commit_message`:
- **Worker role**: The worker implements changes and writes `.lucind/result.json` with status `done`, without committing (`commit: ""` in result envelope; worktree `HEAD` matches `baseSHA`).
- **Verification under attestation**: The dispatcher executes each command in `verification` sequentially under attestation (`attest.RunAndRecord`). Any non-zero exit code halts execution and fails the lane (`lane.Failed`). An execution error blocks the lane (`lane.Blocked`).
- **Attestation gate**: After all commands succeed, `attest.HasValidAttestation` checks that each verification command has a valid HMAC attestation for the current worktree tree hash. If missing, the lane is blocked (`lane.Blocked`).
- **Conventional Commit**: The dispatcher stages changes excluding `.lucind/` (`git -C <worktree> add -A -- . ':(exclude).lucind'`) and commits with the declared `commit_message`. Fallback identity `lucind-ai <lucind-ai@localhost>` is used only when the repository lacks configured identity. No trailers are appended.

