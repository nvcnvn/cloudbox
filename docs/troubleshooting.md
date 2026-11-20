# Troubleshooting

Each outcome below is something CloudBox tells you on purpose. Most of them
are refusals: the product declining to pretend.

A refusal exits with status `1` and names a stable reason. With `-o json` it
is the `reason` field. The refusal entries below are keyed by that code, one
per code that `cloudbox reasons` lists. Branch on the code, not on the
message: the message is for people and may be reworded. The last three
entries are outcomes that are not refusals.

## Refusals

### `not-found`: the named object does not exist

**What it means.** A name or digest in the request refers to nothing the
control plane knows. The `subject` field says what kind of thing was missing:
`application`, `sandbox`, `cluster`, `promotion`, `bundle`, `workload`,
`datastore`, and so on. A restarted control plane has lost everything it
knew, so a name that worked before the restart is gone too.

**What to do.** Check the reference against `subject`. List or re-read what
you created, or create it first. After a control-plane restart, register your
cluster and declare your application again.

### `already-exists`: the name is taken

**What it means.** An application with this name is already declared. Nothing
was changed.

**What to do.** Pick another name, or use the existing application with
`cloudbox app show`.

### `invalid-request`: the request is malformed

**What it means.** The request could not be understood: it was not valid
JSON, a required name was missing, or it named an option that does not exist,
such as an unknown capacity mode or fidelity level, or the removed `local`
option.

**What to do.** Fix the request. The message names the field or value that
was wrong. Run the command without flags for its usage.

### `out-of-contract`: the declaration goes outside the boundary contract

**What it means.** The application's declaration tries to vary something the
boundary contract does not allow, such as a field outside the contract's four
kinds, a secret the contract does not declare, or a dependency on an
application that does not exist.

**What to do.** Change the declaration to stay inside the contract: declare
the secret, fix the dependency's name, or move the value into the bundle.
Environment variance beyond the contract is a change to the product, not to
your application.

### `bundle-rejected`: intake rejected the bundle

**What it means.** The bundle has at least one blocker, such as a
cluster-scoped resource or manifests spread over several namespaces. `apply`
and the offline `cloudbox check` report the same reason. The refusal's
`findings` name each offending manifest and the fix.

**What to do.** Fix the manifests the findings name, then run `cloudbox check`
again before applying. Retrying the same bundle meets the same refusal.

### `quota-exceeded`: the bundle exceeds the sandbox quota

**What it means.** After the capacity transform, the bundle requests more CPU
than the application's quota allows per sandbox. Nothing was admitted.

**What to do.** Shrink the bundle's CPU requests or replicas, then apply
again. The message names the quota and what the bundle requested.

### `not-owner`: the sandbox belongs to someone else

**What it means.** Only a sandbox's owner may modify it, read its logs, or
exec into it. The request acted as a different identity.

**What to do.** Use your own sandbox, or act as its owner with `--as`. This
release does not authenticate identities, so this protects against mistakes,
not against other users.

### `not-permitted`: the actor lacks the role or policy

**What it means.** The operation needs a role or policy the actor does not
hold. Examples: break-glass access without the break-glass role, approving
without an approver role, approving your own promotion, writing to managed
production in strict mode, or using real-data fidelity levels that are not
enabled for the application or not allowed for agent-owned sandboxes.

**What to do.** Ask someone who holds the role to do it, or ask an
administrator to change the application's policy. None of these are
available at the sandbox level.

### `seal-not-weakened`: the seal is never weakened per sandbox

**What it means.** A request tried to add a host to one sandbox's allowlist.
A sandbox's seal comes from its application's boundary contract and is never
widened per sandbox.

**What to do.** Amend the application's allowlist with `cloudbox app allow`,
then create a new sandbox. The change is recorded against the application.

### `evidence-not-postable`: the evidence came from a user-controlled cluster

**What it means.** The sandbox runs on a cluster you control, not one the
control plane manages, so its evidence cannot be posted to a pull request or
used for promotion.

**What to do.** This is expected at the sandbox level. Read the evidence
locally with `cloudbox evidence`.

### `unwitnessed-results`: reported test results are not accepted

**What it means.** Something outside the control plane reported test results,
for example a pipeline. Evidence holds only what the control plane witnessed
and signed itself.

**What to do.** Run the declared suite with `cloudbox test` so the control
plane witnesses the run.

### `integration-missing`: operation refused, no integration configured

**What it means.** The operation depends on an external system this release
does not integrate with: source control for evidence checks, GitOps for
promotion, production ingest for production state. Rather than record a
result it could not have produced, the control plane refuses, names the
missing integration, and records nothing.

**What to do.** Nothing at the sandbox level needs these. Use `cloudbox
evidence` to read a run's evidence directly. Evidence checks and promotion
arrive with their integrations at higher adoption levels.

### `enforcement-unproven`: cluster refused, unproven network policy enforcement

**What it means.** The cluster could not show that it enforces NetworkPolicy.
At registration, the enforcement probe found a connection the seal must deny
was not denied: the CNI accepts policy objects and ignores them, or the
probe's canary pods could not run. A seal on such a cluster would contain
nothing, so it is not accepted as a sandbox host, and a sandbox on a cluster
that stopped enforcing produces no evidence.

**What to do.** Install a CNI that enforces NetworkPolicy (Calico, for
example), or make a cluster with `scripts/kind-cluster.sh`, then run
`cloudbox cluster register` again. If you believe the CNI enforces policy,
check that pods can be scheduled and that the `busybox` canary image can be
pulled.

### `no-sandbox-host`: no sandbox host can take the sandbox

**What it means.** No cluster is registered as a sandbox host, or the
application names a sandbox cluster that is not one. This is usually the
first refusal a new user meets.

**What to do.** Register a cluster you control with `cloudbox cluster
register <context>`. Registration runs the enforcement probe. See
[Cluster setup](cluster-setup.md).

### `not-sealed`: the seal is not active

**What it means.** Nothing is admitted to a sandbox, and no witnessed run
happens, until its default-deny ingress and egress are active.

**What to do.** Read `cloudbox status` and wait until the sandbox reports
sealed. If it never does, destroy the sandbox and create a new one.

### `wrong-state`: the object is not in a state that allows this

**What it means.** The object exists, but its state rules the request out:
the sandbox was destroyed, the promotion is not approved or was already
approved by you, or nothing is awaiting verification.

**What to do.** Read the object's current state and act on that. For a
destroyed sandbox, create a new one.

### `prerequisite-missing`: something the operation depends on is missing

**What it means.** The operation depends on a declaration or registration
that does not exist: the application declares no test suite, has no source
control enabled, or there is no production cluster, no production database,
no branching-capable data service, or no break-glass role for strict mode.

**What to do.** Declare or register what the message names, then retry. For
`cloudbox test`, declare a suite with `cloudbox app create --suite <name>
--tests <n>`.

### `promotion-blocked`: the evidence does not support promotion

**What it means.** The evidence is invalid, or substrate parity is
unverified because no production substrate was compared. Promotion needs
valid evidence of a verified match.

**What to do.** Produce valid evidence from a fresh run. To verify parity,
register production so later evidence can compare against it.

### `audit-unavailable`: the audit record cannot be written

**What it means.** The transition must be audited, and the audit sink is not
accepting records, so nothing proceeded.

**What to do.** Retry once the audit sink is back. Nothing was changed in the
meantime.

### `internal-error`: the control plane failed unexpectedly

**What it means.** The control plane failed in a way it did not anticipate.
This is not a refusal by design.

**What to do.** Retry once. If it recurs, report it with the message and the
control plane's log (`~/.local/state/cloudbox/cloudboxd.log` by default for a
local control plane).

## Other outcomes

### Egress proxy unavailable

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

### Substrate parity unverified

**What it means.** Evidence could not compare the sandbox's substrate with
production's, because no production substrate is registered. CloudBox
reports "unverified" rather than a match it never made. Unverified parity
does not invalidate evidence, but it can never support a promotion.

**What to do.** This is the normal state at the sandbox level, and nothing
needs fixing. To compare against production, register it with `cloudbox
cluster register <context> --production`. Later evidence then reports a match
or a mismatch.

### Control plane unreachable

**What it means.** Exit status `3`: no control plane answered. Either the
address given with `--server` is wrong, or the local control plane could not
be started or has stopped.

**What to do.** With `--server`, check the address. Without it, check that
`cloudboxd` is installed beside `cloudbox` (or on `PATH`), and read the local
control plane's log in your state directory (`~/.local/state/cloudbox/cloudboxd.log`
by default). `cloudbox server status` reports whether one is running. A
restarted control plane has lost its registrations and evidence: register
your cluster again.
