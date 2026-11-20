# CLI reference

`cloudbox` is the whole product surface at the sandbox level. Running
`cloudbox` with no arguments lists every command. Running a command without
flags prints its own usage. The acceptance suite checks this page against
the binary in both directions.

## Global options

```text
cloudbox [--server <url>] [--as <identity>] [-o json] <command> [flags]
```

- `--server <url>`: use this control plane. Without it, `cloudbox` starts a
  local control plane (`cloudboxd`, found beside `cloudbox` or on `PATH`) and
  reuses it for later commands. An addressed control plane always wins.
- `--as <identity>`: the identity the request asserts. It defaults to
  `$CLOUDBOX_AS`, then your login name. This release does not authenticate
  it.
- `-o json`: render the result as data. The JSON and the human rendering
  carry the same facts. Errors are rendered as `{"error", "outcome",
  "status"}`. A refusal adds `reason`, a code from `cloudbox reasons`, and a
  `not-found` refusal adds `subject`, the kind of thing that does not exist.
  A rejected bundle adds `findings`. Branch on `reason`, not on `error`.

## Exit statuses

| Exit status | Outcome | Meaning |
|---|---|---|
| `0` | success | The command did what it was asked. |
| `1` | refused | The control plane refused the request, or `check` found blockers. `reason` names which refusal it was. |
| `2` | usage | The command line was malformed: an unknown command or flag, or a missing argument. |
| `3` | unreachable | No control plane could be reached or started. |

## Commands

### `cloudbox check`

`cloudbox check -f <dir>`: the offline compatibility report for a directory
of manifests. It needs no cluster and no control plane, and runs the same
intake analysis that `apply` runs server-side. With `-o json` it reports
`findings` and `transforms`; a bundle with blockers is refused with reason
`bundle-rejected`, the same reason `apply` reports.

### `cloudbox cluster`

`cloudbox cluster register <kube-context> [--production]`: register a
cluster you control as a sandbox host. Registration probes NetworkPolicy
enforcement and refuses a cluster that cannot show it. With `--production`,
registers the production substrate that parity compares against.

### `cloudbox app`

- `cloudbox app create <name> [--allow <fqdn>]... [--secret <name>]...
  [--suite <name> --tests <n>]`: declare an application, its boundary
  contract (allowlisted hosts, secret names), and its test suite.
- `cloudbox app allow <name> <fqdn>...`: amend the allowlist.
- `cloudbox app show <name>`: the declaration.

### `cloudbox sandbox`

- `cloudbox sandbox create -a <app> [--ttl <seconds>]`: create a sealed
  sandbox and print its name.
- `cloudbox sandbox destroy -s <sandbox>`: destroy it and everything placed on
  the cluster for it.

### `cloudbox apply`

`cloudbox apply -a <app> -s <sandbox> -f <file-or-dir> [--record-egress]`:
apply a YAML file, or every `.yaml`/`.yml` in a directory, to a sandbox.
Unknown flags are refused by name. There is no flag that targets production.

### `cloudbox test`

`cloudbox test -s <sandbox> [--suite <name>]`: run the application's
declared suite inside the sandbox. The results become witnessed activity in
evidence.

### `cloudbox status`

`cloudbox status -s <sandbox> [--explain]`: sandbox state, seal, capacity
mode, and diagnostics. `--explain` adds each blocked egress attempt
(destination, time, workload) and a ready-to-submit allowlist proposal.

### `cloudbox evidence`

`cloudbox evidence -s <sandbox>`: the record of the run, including its
honest summary, validity, substrate parity, and signature.

### `cloudbox logs`

`cloudbox logs -s <sandbox> -w <workload>`: a workload's logs.

### `cloudbox exec`

`cloudbox exec -s <sandbox> -w <workload> -c <command>`: run a command in a
workload. Audited.

### `cloudbox port-forward`

`cloudbox port-forward -s <sandbox> -w <workload>`: reach a workload from
this machine through the control plane.

### `cloudbox server`

`cloudbox server status|stop`: report on, or stop, the local control plane.
Stopping it loses its evidence.

### `cloudbox reasons`

`cloudbox reasons`: the refusal-reason catalog, each code with a one-line
summary. It needs no control plane. A reason is never removed or renamed, so
a program may branch on any code it lists. Each one has an entry in
[Troubleshooting](troubleshooting.md).

### `cloudbox version`

`cloudbox version`: the build this binary came from. A locally built binary
reports `unreleased build (commit …)`.
