# Quickstart

From a machine with no cluster to a sealed sandbox, a blocked attempt, an
amended allowlist, and evidence. Allow about ten minutes, most of it spent
creating the cluster.

Read [Limitations](limitations.md) first. In short: this release
authenticates no one, and evidence is lost when the local control plane stops.

## What you need

- **A Kubernetes cluster whose CNI enforces NetworkPolicy.** This is the one
  property CloudBox depends on. A cluster whose network plugin accepts
  NetworkPolicy objects but ignores them is refused at registration, because
  a seal on it would contain nothing. The script below makes a suitable
  cluster: Kind with Calico.
- `docker`, `kind` and `kubectl` to run that script.
- The `cloudbox` and `cloudboxd` binaries, side by side on your `PATH`. Take
  them from a GitHub release, or build them from a checkout with
  `make build` (they land in `bin/`). A locally built binary reports itself
  as an unreleased build.

## 1. Make a cluster

```sh
scripts/kind-cluster.sh
```

This creates the Kind cluster `cloudbox` with Calico, and adds the kube
context `kind-cloudbox` to your kubeconfig. CloudBox never creates clusters
itself. This script is a convenience outside the product.

## 2. Register it as a sandbox host

```sh
cloudbox cluster register kind-cloudbox
```

The first command you run starts a local control plane (`cloudboxd`), and
every later command reuses it. Registration runs the enforcement probe. A
canary that the seal must block is checked to be really blocked. A cluster
that fails the probe is refused with "unproven network policy enforcement".
A cluster you register is user-controlled, so its evidence can never back a
pull-request check or a promotion.

## 3. Declare an application

```sh
cloudbox app create web --allow api.github.com --suite smoke --tests 3
```

The boundary contract declares what may differ between environments. Here it
is one allowlisted host, plus a declared test suite. Nothing outside the
contract varies.

## 4. Create a sandbox and apply your manifests

```sh
cloudbox sandbox create -a web
cloudbox apply -a web -s web-sbx-1 -f ./k8s/
```

Use the sandbox name that `sandbox create` prints. `apply` takes a YAML file
or a directory of them: the Kubernetes manifests you already have, with no
CloudBox schema. Intake rejects what cannot run sealed and names the fix.
`cloudbox check -f ./k8s/` runs the same analysis offline, before you start.

## 5. Run the suite, then see what was blocked

```sh
cloudbox test -s web-sbx-1
cloudbox status -s web-sbx-1 --explain
```

`--explain` lists each blocked egress attempt that reached the egress proxy
(destination, time, workload) and proposes an allowlist change. Egress that
never reaches the proxy is blocked without its destination being recorded, so
the list is a floor, not a complete account.

## 6. Amend the declaration

```sh
cloudbox app allow web api.stripe.com
```

Nothing is ever allowed on the fly: you change the declaration, and the seal
follows it.

## 7. Read the evidence, then clean up

```sh
cloudbox evidence -s web-sbx-1
cloudbox sandbox destroy -s web-sbx-1
cloudbox server stop
```

Evidence states what ran sealed, at which data fidelity, for how long, and
what the suite witnessed. Substrate parity reads "unverified" because no
production substrate is registered, and it never claims a match it did not
make. **Evidence is lost when the local control plane stops.** That is this
release's retention policy, so read it, or keep its output, before
`server stop`.

## For agents

Every command takes `-o json` and renders the same facts as data. Exit
statuses separate the outcomes without parsing text: `0` success, `1` the
product refused the request, `2` the command line was malformed, `3` no
control plane could be reached.

## Trust

The control plane does not authenticate the identity a request acts as. It
is not a trust boundary between users. Run it for yourself, on your own
machine, and do not expose it on a shared network.
