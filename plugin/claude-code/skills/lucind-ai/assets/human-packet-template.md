# Human packet <id>

**Lane:** human · **Resolves:** <approval id, if any>
**Condition that assigned it:** needs a credential value | needs critical supervision

## The rule that shapes this template

**The human runs one command: the one that requires the secret.** Everything else — backups,
merges, bringing containers up, recreating services, checking criteria, cleaning up worktrees
and volumes — is boilerplate the orchestrator runs. If a step does not need a human, it does
not belong here.

No password reaches the orchestrator. The human chooses it, the human types it, and the
done-criteria are designed to be verifiable **without knowing it**.

Corollary worth checking before writing the packet: if the secret is **already on disk in a
file the human wrote**, aligning the rest of the system to that value needs no human at all —
read it from the container's environment. Ask before running it anyway; it mutates credential
state.

## Goal

One paragraph. What must be true when this is done.

## Context — verified, do not re-derive

Facts already established, with where they came from. Include any ordering hazard: what
breaks, and when.

## What you run

The one command. Give it exactly, in the project's primary shell, with the secret as a
placeholder the human replaces.

```powershell
<command with '<NUEVA>' where the secret goes>
```

Choose the value and store it in your password manager before running. It is never repeated
back, logged, or asked for.

**What this leaves broken:** the intermediate state after this command, so a broken stack
reads as expected rather than as a failure.

## What I run afterwards

List it, so the human knows the job is not theirs and can see it was done:

- …
- …

## Done criteria

Checked by the orchestrator, and designed so that none of them reveals the secret.

- [ ] … Evidence: `<command>`
- [ ] … Evidence: `<command>`

## Hard stops

Stop and report — do not improvise.

- The command fails, or its precondition does not hold.
- A step asks for a decision this packet does not make.
- Any instruction here conflicts with another.

## What you return

The terminal transcript is fine — the orchestrator extracts the envelope from it. What must
be present:

- whether the command succeeded
- anything that surprised you, including friction in these instructions
- any hard stop that fired

Never include a password in what you return.
