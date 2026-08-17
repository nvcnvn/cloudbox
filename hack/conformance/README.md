# Conformance cluster provisioning

Scripts that provision the real clusters the `@conformance` subset runs
against (ADR 0008). Both are idempotent and export kubeconfigs under
`acceptance-tests/.kube/` (gitignored).

| Script | Cluster | Purpose |
|---|---|---|
| `kind-enforcing.sh` | `cloudbox-conformance` | The conformance run's target: Kind with Calico enforcing NetworkPolicy. |
| `kind-nonenforcing.sh` | `cloudbox-nonenforcing` | Deliberately non-enforcing cluster for the probe-failure and gate scenarios (task 4.2). |

## Pinned versions

- **Kind v0.32.0**, node image `kindest/node:v1.36.1` digest-pinned in the
  scripts (Kubernetes v1.36.1).
- **Calico v3.31.0** as the enforcing CNI, applied from the version-pinned
  upstream manifest.

Calico over Cilium (resolves design Open Question 2): the conformance floor
needs exactly standard NetworkPolicy v1 enforcement, which Calico provides
from a single manifest with a much lighter CI footprint than a helm-installed
Cilium. ADR 0001's MAY — compiling the allowlist to native policy where a
DNS-aware CNI is detected — remains open and unexercised; revisiting the CNI
choice is cheap if that MAY is ever picked up.

## Verified finding: kind v0.32.0's default CNI DOES enforce NetworkPolicy

Verified on 2026-08-16 rather than assumed (task 3.1): on a stock
`kind create cluster` (kind v0.32.0, kindnetd `v20260528-9350166c`, node
v1.36.1), two busybox pods in one namespace connected successfully; after
applying a default-deny ingress+egress NetworkPolicy, the same connection was
**blocked**. kindnetd embeds `kube-network-policies` and there is no
daemonset-level flag to turn enforcement off.

Consequences:

- The enforcing cluster still installs Calico explicitly — the seal claim
  must not ride on a default that a version bump can silently change.
- The deliberately non-enforcing cluster (design Open Question 3) **cannot**
  be stock Kind: its default now enforces. `kind-nonenforcing.sh` therefore
  disables the default CNI and installs flannel, which accepts NetworkPolicy
  objects without enforcing them — exactly the condition the probe-failure
  scenarios need.

## Verified finding: flannel v0.27.4 accepts but does not enforce NetworkPolicy

Verified on 2026-08-16 on the `cloudbox-nonenforcing` cluster: a default-deny
ingress+egress NetworkPolicy was admitted without error, and pod-to-pod
traffic continued to flow — the silent failure mode N7's probe exists to
catch, now arranged on real infrastructure. Provisioning notes: kindest/node
ships without the reference CNI plugins flannel delegates to, so the script
installs `containernetworking/plugins` v1.6.2 onto each node (via `/root`;
the node's `/tmp` is a tmpfs that shadows `docker cp` writes).

## Published residual exposure: the proxy's admin port stays open, the token is the control

The seal's `cloudbox-egress-proxy-admin` NetworkPolicy admits ingress on the
egress proxy's admin port (3129) **from any source**, and that is deliberate.
The control plane collects attempt records through the API server's service
proxy, whose source address is a cluster-topology detail; encoding it into the
seal would make the seal cluster-specific and quietly breakable by a topology
change.

What actually protects the surface is the per-namespace credential
(`cloudbox-egress-proxy-token`, created at seal time, presented as
`X-Cloudbox-Egress-Token`): `/attempts` refuses any request without it. So a
pod elsewhere on the cluster can still open a connection to that port and gets
401 — verified by the conformance scenario "A pod outside the sandbox cannot
read its attempt records", which reaches the port from an unsealed namespace and
asserts the refusal.

Stated here because the failure mode is a later reader assuming the
NetworkPolicy is the protection and removing the credential check as
redundant (ADR 0001: residual channels are published, not papered over).
