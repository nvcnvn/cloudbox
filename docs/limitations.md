# Limitations

This page lists what this release of CloudBox does not do. Read it before
relying on the release.

## Trust and identity

The control plane does not authenticate the identity a request acts as. The
identity a command sends is taken as given. The control plane is not a trust
boundary between users: anyone who can reach it can act as anyone. This
release is built for one developer or agent on one machine, talking to a
control plane they started themselves. Do not expose the control plane on a
shared network.

There is no TLS between the command line and the control plane.

## State and evidence retention

The control plane keeps its state in memory.

- Evidence is lost when the local control plane stops. Nothing is written to
  disk, and nothing is uploaded anywhere. If you need a run's evidence, read
  it before you stop the control plane. This is the release's retention
  policy, not an accident.
- Registered clusters and applications do not survive a control-plane
  restart. After a restart, register your cluster and declare your
  application again.
- Sandboxes are disposable. Destroying a sandbox removes its namespace and
  everything CloudBox placed on the cluster for it.

## Operations this release refuses

Only the sandbox level (L1) is available. Operations that depend on an
integration this release does not have are refused. The refusal names the
missing integration, and nothing is recorded. The release never pretends to
succeed:

- **Posting an evidence check** to a pull request (L2) is refused. Evidence
  from a user-controlled cluster could never be posted anyway.
- **Opening a promotion** (L3 and L4) is refused.
- **Setting production state** is refused, so there is no production
  substrate to compare against. Substrate parity is therefore reported as
  unverified, and it is never reported as a match.

## Clusters

- CloudBox does not create a cluster for you. You bring a Kubernetes cluster
  with a CNI that enforces NetworkPolicy, and register it.
- Managed Kubernetes offerings (EKS, GKE, AKS) are not supported in this
  release.

## Evidence signing

Evidence is signed by the control plane. In this release the signature is a
digest, not a keyed signature, so it does not prove which control plane
produced the evidence. That is acceptable only because evidence cannot leave
your machine in this release.
