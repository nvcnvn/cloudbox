# Cluster setup

CloudBox does not create clusters. You bring a Kubernetes cluster you
control, and register it as a sandbox host. This page takes you from no
cluster to a registered host.

## Requirements

The one property CloudBox depends on is **a CNI that enforces NetworkPolicy**.
The seal is standard NetworkPolicy. On a cluster whose network plugin accepts
NetworkPolicy objects but does not enforce them, a "sealed" sandbox would
contain nothing. CloudBox therefore checks this property rather than trusting
it.

Calico and Cilium enforce NetworkPolicy. Some network plugins accept the
objects and ignore them.

Managed offerings (EKS, GKE, AKS) are not supported in this release.

## Make a local cluster

The supported bootstrap script makes a Kind cluster with Calico:

```sh
scripts/kind-cluster.sh            # creates the Kind cluster "cloudbox"
scripts/kind-cluster.sh my-sandboxes   # or name it
```

It needs `docker`, `kind` and `kubectl`. It adds the kube context
`kind-<name>` to your kubeconfig, preloads the probe's canary image, and loads
the egress proxy image. Re-running it creates only what is missing. The
script lives outside the product: CloudBox itself never provisions clusters.

## Register the cluster

```sh
cloudbox cluster register kind-cloudbox
```

The argument is the cluster's kube context. Registration runs the
**enforcement probe**: a canary in a sealed scratch namespace must fail to
reach a destination that a canary in an unsealed namespace can reach. Both
scratch namespaces are deleted afterwards, whatever the result. A cluster
that passes is accepted as a sandbox host.

Every cluster you register is **user-controlled**. Sandboxes on it have the
same seal as anywhere else, but their evidence is marked user-controlled and
can never back a pull-request check or a promotion. Whoever controls a
cluster could have shaped the run.

To register a production cluster that substrate parity compares against,
use `cloudbox cluster register <context> --production`. Without one, parity
is reported as unverified, never as a match.

## When registration is refused

A cluster that cannot be shown to enforce NetworkPolicy is refused, and the
refusal names **unproven network policy enforcement**. No sandbox can be
created on a refused cluster, including by pinning an application to it.

To have the cluster accepted, install a CNI that enforces NetworkPolicy, such
as Calico, or use `scripts/kind-cluster.sh`, which installs Calico for you.
Then register it again. If the probe fails on a cluster you believe enforces
policy, check that pods can start: the probe needs both canaries to run, and
it reports unproven enforcement rather than guessing.

A host whose CNI stops enforcing after registration is still caught. Every
sandbox creation runs the same probe, and a sandbox that fails it never
reports itself sealed and produces no evidence.
