# Control Room Execution Runbook

This is the unattended execution order. Run from the primary repository on `dev`, never from a linked worktree. Every command is preceded by its required state and followed by its proof. The runbook does not assume automatic wave advancement or automatic branch merge.

## 0. Preconditions and binary

```sh
test "$(git branch --show-current)" = dev
test -z "$(git status --porcelain --untracked-files=no)"
test "$(git rev-parse HEAD)" = 705cf492348202e3106bd80c12961ad4ea45aafd
lucind-ai -v
```

The first command proves the branch. The second allows existing untracked planning inputs while refusing tracked changes. The third proves the documented planning anchor. `lucind-ai -v` proves the installed binary is current enough to report its build; because this plan changes no binary, no install is required now.

Create the shared planning target before dispatching any planning packet:

```sh
PLAN_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/planning "$PLAN_SHA"
test "$(git rev-parse refs/heads/control-room/planning)" = "$PLAN_SHA"
lucind-ai feature create --id control-room-planning --parent refs/heads/control-room/planning --base-sha "$PLAN_SHA" --expected-parent-sha "$PLAN_SHA"
```

## 1. Planning fan-out

Create each feature's `openspec/changes/<feature-id>/` working directory as required by the planning templates. For each phase, dispatch the three agy lenses concurrently, wait for all three terminal envelopes, then dispatch the cursor-agent synthesis packet. Use the exact packet IDs, models, routing conditions, and path scopes in that feature's `sdd-flow.md`. A phase barrier is satisfied only when its canonical artifact exists and its synthesis result is terminal; then start the next phase.

Create the change directories and materialize dispatchable packets from the repository's phase templates. The generated packets are temporary orchestration inputs; their `allowed_paths` remain the canonical paths listed in each `sdd-flow.md`.

```sh
mkdir -p /tmp/opencode/control-room-planning-packets
for F in control-room-telemetry control-room-ledger control-room-capture control-room-serve control-room-ui-shell control-room-ui-views; do
  mkdir -p "openspec/changes/$F" "/tmp/opencode/control-room-planning-packets/$F"
done
python3 - <<'PY'
from pathlib import Path
import json
import re

root = Path("openspec/plans/control-room")
assets = Path("plugin/claude-code/skills/lucind-ai/assets")
out_root = Path("/tmp/opencode/control-room-planning-packets")
staging_feature = "control-room-planning"
staging_parent = "refs/heads/control-room/planning"
anchor = "705cf492348202e3106bd80c12961ad4ea45aafd"

for flow in sorted(root.glob("control-room-*/sdd-flow.md")):
    feature = flow.parent.name
    for line in flow.read_text().splitlines():
        if not line.startswith("- `"):
            continue
        fields = [part.strip() for part in line[3:].split(" — ", 4)]
        if len(fields) != 5:
            raise SystemExit(f"cannot parse planning packet row: {line}")
        packet_id = fields[0].strip("`")
        executor = fields[1]
        routed_by = fields[2].removeprefix("condition: ")
        model = fields[3].strip("`")
        allowed = [p.strip().strip("`") for p in fields[4].split(",")]
        phase = packet_id.split("-", 1)[0]
        if packet_id.endswith("-synthesis"):
            template = assets / f"{phase}-synthesis-packet-template.md"
        else:
            lens = packet_id.rsplit("-", 1)[-1]
            template = assets / f"{phase}-lens-{lens}-packet-template.md"
        text = template.read_text().replace("<change-id>", feature)
        front, body = text.split("---", 2)[1:]
        lines = []
        for item in front.strip().splitlines():
            key = item.split(":", 1)[0].strip()
            if key == "id":
                item = f"id: {packet_id}"
            elif key == "executor":
                item = f"executor: {executor}"
            elif key == "routed_by":
                item = f"routed_by: {routed_by}"
            elif key == "model":
                item = f"model: {model}"
            elif key == "allowed_paths":
                item = f"allowed_paths: {json.dumps(allowed)}"
            lines.append(item)
        lines.extend([
            f"feature: {staging_feature}",
            f"parent_ref: {staging_parent}",
            f"base_sha: {anchor}",
            f"expected_parent_sha: {anchor}",
            "legacy_main: false",
        ])
        packet = "---\n" + "\n".join(lines) + "\n---" + body
        (out_root / feature / f"{packet_id}.md").write_text(packet)
PY
```

The literal dispatch shape for every feature and phase is:

```sh
lucind-ai run --packet /tmp/opencode/control-room-planning-packets/<feature>/<lens-a-id>.md --packet /tmp/opencode/control-room-planning-packets/<feature>/<lens-b-id>.md --packet /tmp/opencode/control-room-planning-packets/<feature>/<lens-c-id>.md
```

The planning templates do not carry target fields, while current admission requires them (`internal/run/run.go:267-285`). Before dispatch, the orchestrator MUST materialize each planning packet with `feature: control-room-planning`, `parent_ref: refs/heads/control-room/planning`, and live SHA fields. Do not dispatch the raw asset placeholder or use legacy mode: concurrent legacy batches would merge into the primary checkout. All 24 planning packets per phase group use agy for lenses and cursor-agent for synthesis; no opencode is spent here.

Run these phase groups in parallel across all six feature directories, but preserve phase order within each feature:

```text
P1: explore-<feature>-lens-a, -lens-b, -lens-c → explore-<feature>-synthesis
P2: propose-<feature>-lens-a, -lens-b, -lens-c → propose-<feature>-synthesis
P3: design-<feature>-lens-a, -lens-b, -lens-c → design-<feature>-synthesis
P4: spec-<feature>-lens-a, -lens-b, -lens-c → spec-<feature>-synthesis
P5: tasks-<feature>-lens-a, -lens-b, -lens-c → tasks-<feature>-synthesis
```

Success checks after each synthesis are the canonical artifact named by the template, a valid `.lucind/result.json`, and a terminal status. Do not proceed on `blocked`, `deviated`, or a missing synthesis note.

After P5, consolidate the staging planning parent explicitly:

```sh
git switch dev
git merge --ff-only refs/heads/control-room/planning
lucind-ai check --out openspec/plans/control-room/planning-mechanical.log
```

## 2. Feature target derivation

The commands in §1 create the staging ledger record; they do not merge anything. After P5, merge `refs/heads/control-room/planning` into `dev`. Before each implementation feature is applied, create its parent branch and feature record from the current `dev` tip; after promotion, merge that parent into `dev` before creating the next feature record. If a parent branch has unique planning commits, use its actual tip as both values and record the derivation in the run receipt. A stale expected SHA must block, not be guessed around.

## 3. Apply helper and first feature

For every feature, before `split`, replace the current static SHA values in its DAG with the live `BASE_SHA` and `EXPECTED_PARENT_SHA`. The replacement is mechanical and affects only that feature's plan directory:

```sh
F=control-room-telemetry
DAG="openspec/plans/control-room/$F/apply-dag.yaml"
BASE_SHA="$(git rev-parse refs/heads/dev)"
EXPECTED_PARENT_SHA="$(git rev-parse "refs/heads/control-room/$F")"
python3 - "$DAG" "$BASE_SHA" "$EXPECTED_PARENT_SHA" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
s = s.replace("expected_parent_sha: 705cf492348202e3106bd80c12961ad4ea45aafd", "expected_parent_sha: " + sys.argv[3])
s = s.replace("705cf492348202e3106bd80c12961ad4ea45aafd", sys.argv[2])
p.write_text(s)
PY
```

The same replacement is applied to each emitted packet only by `lucind-ai split`; the DAG remains the source of truth. Confirm `git rev-parse "$BASE_SHA"` and the feature parent ref before dispatch.

For each feature, run exactly:

```sh
lucind-ai split --dag openspec/plans/control-room/<feature>/apply-dag.yaml --out openspec/plans/control-room/<feature>/packets
```

The command must exit 0 and print one `lucind-ai run` line per wave. It proves YAML parsing, body-path existence, non-empty scopes, dependency validity, cycle absence, and global overlap validation. Because the current `dag.Emit` implementation emits `read_only_paths` but does not emit the packet boolean, restore the explicit boolean required by packet frontmatter before dispatch:

```sh
python3 - "openspec/plans/control-room/<feature>/packets" <<'PY'
from pathlib import Path
import sys
for p in Path(sys.argv[1]).glob("*.md"):
    s = p.read_text()
    if "\nread_only:" not in s:
        s = s.replace("agent: build\n", "agent: build\nread_only: false\n", 1)
        p.write_text(s)
PY
```

Then run each printed wave exactly once, in order, waiting for exit code 0:

```sh
lucind-ai run --packet openspec/plans/control-room/<feature>/packets/<node-a>.md --packet openspec/plans/control-room/<feature>/packets/<node-b>.md
```

Use the exact node commands listed below; do not combine adjacent waves.

### A1 — `control-room-telemetry`

```sh
BASE_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/control-room-telemetry "$BASE_SHA"
lucind-ai feature create --id control-room-telemetry --parent refs/heads/control-room/control-room-telemetry --base-sha "$BASE_SHA" --expected-parent-sha "$BASE_SHA"
lucind-ai split --dag openspec/plans/control-room/control-room-telemetry/apply-dag.yaml --out openspec/plans/control-room/control-room-telemetry/packets
lucind-ai run --packet openspec/plans/control-room/control-room-telemetry/packets/schema-v6.md
lucind-ai check --out openspec/plans/control-room/control-room-telemetry/mechanical.log
lucind-ai feature status --id control-room-telemetry
```

The feature status must show the named parent and an integrated attempt. If it does not, stop and use the printed attempt with `lucind-ai feature recover --attempt <id>` only after human review.

## 4. Sequential feature integration

After A1, merge its parent branch into `dev` explicitly, verify, then refresh the next feature's parent and SHA fields before running it:

```sh
git switch dev
git merge --ff-only refs/heads/control-room/control-room-telemetry
lucind-ai check --out openspec/plans/control-room/control-room-telemetry/post-merge.log
```

Repeat the same four-step boundary for each feature: refresh DAG SHA fields, split, run every wave, check, inspect feature status, then merge the feature parent into `dev`.

### A2 — `control-room-ledger`

```sh
BASE_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/control-room-ledger "$BASE_SHA"
lucind-ai feature create --id control-room-ledger --parent refs/heads/control-room/control-room-ledger --base-sha "$BASE_SHA" --expected-parent-sha "$BASE_SHA"
lucind-ai split --dag openspec/plans/control-room/control-room-ledger/apply-dag.yaml --out openspec/plans/control-room/control-room-ledger/packets
lucind-ai run --packet openspec/plans/control-room/control-room-ledger/packets/ledger-runs.md --packet openspec/plans/control-room/control-room-ledger/packets/ledger-progress.md --packet openspec/plans/control-room/control-room-ledger/packets/ledger-lanes.md
lucind-ai check --out openspec/plans/control-room/control-room-ledger/mechanical.log
lucind-ai feature status --id control-room-ledger
git switch dev && git merge --ff-only refs/heads/control-room/control-room-ledger
lucind-ai check --out openspec/plans/control-room/control-room-ledger/post-merge.log
```

### A3 — `control-room-capture`

```sh
BASE_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/control-room-capture "$BASE_SHA"
lucind-ai feature create --id control-room-capture --parent refs/heads/control-room/control-room-capture --base-sha "$BASE_SHA" --expected-parent-sha "$BASE_SHA"
lucind-ai split --dag openspec/plans/control-room/control-room-capture/apply-dag.yaml --out openspec/plans/control-room/control-room-capture/packets
lucind-ai run --packet openspec/plans/control-room/control-room-capture/packets/exec-contract.md
lucind-ai run --packet openspec/plans/control-room/control-room-capture/packets/exec-stream-agy.md --packet openspec/plans/control-room/control-room-capture/packets/exec-stream-cursor.md --packet openspec/plans/control-room/control-room-capture/packets/exec-stream-opencode.md
lucind-ai run --packet openspec/plans/control-room/control-room-capture/packets/run-writer.md
lucind-ai check --out openspec/plans/control-room/control-room-capture/mechanical.log
lucind-ai feature status --id control-room-capture
git switch dev && git merge --ff-only refs/heads/control-room/control-room-capture
lucind-ai check --out openspec/plans/control-room/control-room-capture/post-merge.log
```

Do not mark A3 complete until empirical stream probes exist for agy, cursor-agent, and opencode. A decoder failure must degrade to blocking JSON, not fail the lane; unresolved shape evidence remains R2.

### A4 — `control-room-serve`

```sh
BASE_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/control-room-serve "$BASE_SHA"
lucind-ai feature create --id control-room-serve --parent refs/heads/control-room/control-room-serve --base-sha "$BASE_SHA" --expected-parent-sha "$BASE_SHA"
lucind-ai split --dag openspec/plans/control-room/control-room-serve/apply-dag.yaml --out openspec/plans/control-room/control-room-serve/packets
lucind-ai run --packet openspec/plans/control-room/control-room-serve/packets/serve-model.md --packet openspec/plans/control-room/control-room-serve/packets/serve-hub.md --packet openspec/plans/control-room/control-room-serve/packets/serve-worktrees.md
lucind-ai run --packet openspec/plans/control-room/control-room-serve/packets/serve-handlers.md
lucind-ai check --out openspec/plans/control-room/control-room-serve/mechanical.log
lucind-ai feature status --id control-room-serve
git switch dev && git merge --ff-only refs/heads/control-room/control-room-serve
lucind-ai check --out openspec/plans/control-room/control-room-serve/post-merge.log
```

### A5 — `control-room-ui-shell`

```sh
BASE_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/control-room-ui-shell "$BASE_SHA"
lucind-ai feature create --id control-room-ui-shell --parent refs/heads/control-room/control-room-ui-shell --base-sha "$BASE_SHA" --expected-parent-sha "$BASE_SHA"
lucind-ai split --dag openspec/plans/control-room/control-room-ui-shell/apply-dag.yaml --out openspec/plans/control-room/control-room-ui-shell/packets
lucind-ai run --packet openspec/plans/control-room/control-room-ui-shell/packets/ui-shell.md
lucind-ai run --packet openspec/plans/control-room/control-room-ui-shell/packets/ui-fleet.md --packet openspec/plans/control-room/control-room-ui-shell/packets/ui-dag.md --packet openspec/plans/control-room/control-room-ui-shell/packets/ui-flows.md
lucind-ai check --out openspec/plans/control-room/control-room-ui-shell/mechanical.log
lucind-ai feature status --id control-room-ui-shell
git switch dev && git merge --ff-only refs/heads/control-room/control-room-ui-shell
lucind-ai check --out openspec/plans/control-room/control-room-ui-shell/post-merge.log
```

### A6 — `control-room-ui-views`

```sh
BASE_SHA="$(git rev-parse refs/heads/dev)"
git branch --force control-room/control-room-ui-views "$BASE_SHA"
lucind-ai feature create --id control-room-ui-views --parent refs/heads/control-room/control-room-ui-views --base-sha "$BASE_SHA" --expected-parent-sha "$BASE_SHA"
lucind-ai split --dag openspec/plans/control-room/control-room-ui-views/apply-dag.yaml --out openspec/plans/control-room/control-room-ui-views/packets
lucind-ai run --packet openspec/plans/control-room/control-room-ui-views/packets/ui-features.md --packet openspec/plans/control-room/control-room-ui-views/packets/ui-timeline.md --packet openspec/plans/control-room/control-room-ui-views/packets/ui-approvals.md
lucind-ai run --packet openspec/plans/control-room/control-room-ui-views/packets/cli-wiring.md
lucind-ai check --out openspec/plans/control-room/control-room-ui-views/mechanical.log
lucind-ai feature status --id control-room-ui-views
git switch dev && git merge --ff-only refs/heads/control-room/control-room-ui-views
lucind-ai check --out openspec/plans/control-room/control-room-ui-views/post-merge.log
```

## 5. Final barrier and delivery

```sh
git switch dev
lucind-ai check --out openspec/plans/control-room/final-mechanical.log
git status --short
test -z "$(git status --porcelain --untracked-files=no)"
```

Success requires `git status` to contain only the intended documentation/planning artifacts, `lucind-ai check` exit 0, all six feature records to exist with matching parent refs and latest attempts promoted, and no unresolved R1-R4 or decomposition risk. Any failure is a stop-and-report condition; this runbook never assumes that a feature branch merged itself.
