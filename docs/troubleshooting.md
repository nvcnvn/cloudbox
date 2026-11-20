# Troubleshooting

Each outcome below is something CloudBox tells you on purpose. Most of them
are refusals: the product declining to pretend.

## Cluster refused: unproven network policy enforcement

**What it means.** Registration ran the enforcement probe, and the cluster
could not show that it enforces NetworkPolicy. Either its CNI accepts policy
objects and ignores them, or the probe's canary pods could not run. A seal on
such a cluster would contain nothing, so it is not accepted as a sandbox host.

**What to do.** Install a CNI that enforces NetworkPolicy (Calico, for
example), or make a cluster with `scripts/kind-cluster.sh`, then run
`cloudbox cluster register` again. If you believe the CNI enforces policy,
check that pods can be scheduled and that the `busybox` canary image can be
pulled.

## Operation refused: no integration configured

**What it means.** The operation depends on an external system this release
does not integrate with: source control for evidence checks, GitOps for
promotion, production ingest for production state. Rather than record a
result it could not have produced, the control plane refuses, names the
missing integration, and records nothing.

**What to do.** Nothing at the sandbox level needs these. Use `cloudbox
evidence` to read a run's evidence directly. Evidence checks and promotion
arrive with their integrations at higher adoption levels.

## Egress proxy unavailable

**What it means.** The sandbox's egress proxy could not be provisioned. Most
often its image could not be pulled, and the error names the image. The seal
still denies everything by default, but nothing can enforce the allowlist,
so allowlisted egress is denied too, and the sandbox reports
`allowlistEnforced: false`.

**What to do.** Check that the cluster can pull the named image. A released
build names a digest on `ghcr.io`. An unreleased build names
`cloudbox-egress-proxy:dev`, which `scripts/kind-cluster.sh` loads into the
cluster. For an air-gapped cluster, mirror the image and set
`CLOUDBOX_PROXY_IMAGE` for `cloudboxd`. Then recreate the sandbox.

## Substrate parity unverified

**What it means.** Evidence could not compare the sandbox's substrate with
production's, because no production substrate is registered. CloudBox
reports "unverified" rather than a match it never made. Unverified parity
does not invalidate evidence, but it can never support a promotion.

**What to do.** This is the normal state at the sandbox level, and nothing
needs fixing. To compare against production, register it with `cloudbox
cluster register <context> --production`. Later evidence then reports a match
or a mismatch.

## Control plane unreachable

**What it means.** Exit status `3`: no control plane answered. Either the
address given with `--server` is wrong, or the local control plane could not
be started or has stopped.

**What to do.** With `--server`, check the address. Without it, check that
`cloudboxd` is installed beside `cloudbox` (or on `PATH`), and read the local
control plane's log in your state directory (`~/.local/state/cloudbox/cloudboxd.log`
by default). `cloudbox server status` reports whether one is running. A
restarted control plane has lost its registrations and evidence: register
your cluster again.
