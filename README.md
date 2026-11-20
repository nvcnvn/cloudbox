# CloudBox

Sealed, production-shaped Kubernetes sandboxes for developers and AI coding
agents, with evidence that says only what actually happened.

An agent applies your existing manifests to a sandbox whose egress is denied
by default. It learns what its code tried to reach, amends a declared
contract, and ends with a signed record of the run. Nothing in that record
claims more than was observed.

## The sealed-iteration loop

```sh
cloudbox cluster register kind-cloudbox        # a cluster you control; enforcement is probed
cloudbox app create web --allow api.github.com # declare the boundary contract
cloudbox sandbox create -a web                 # sealed before anything runs
cloudbox apply -a web -s web-sbx-1 -f ./k8s/   # the YAML you already have
cloudbox test -s web-sbx-1                     # the declared suite, witnessed
cloudbox status -s web-sbx-1 --explain         # what was blocked, and a proposed fix
cloudbox app allow web api.stripe.com          # amend the declaration, never the seal
cloudbox evidence -s web-sbx-1                 # the record of the run
cloudbox sandbox destroy -s web-sbx-1
```

Every command takes `-o json`, and exit statuses separate success, a refusal,
and an unreachable control plane. An agent reads outcomes as data.

## What the seal does, and does not, claim

A sealed sandbox can reach its own services, cluster DNS, and the hosts on
its declared allowlist, through the product's egress proxy. All direct
egress, every other host, all ingress, and all writes to production are
blocked. The seal is not unbreakable. DNS tunneling and exfiltration through
an allowlisted endpoint remain open, and the product names them rather than
hiding them.

Recording is weaker than blocking. An attempt that reaches the egress proxy
is recorded with its destination. Everything else is denied by NetworkPolicy
without its destination being recorded. The blocked-attempt list is a floor,
not a complete account. The control plane publishes this statement at
`/v1/containment-statement`.

## This release

- **Sandbox level only.** You bring a cluster whose CNI enforces
  NetworkPolicy (`scripts/kind-cluster.sh` makes one). Evidence checks,
  promotion, and production state are refused, never simulated.
- **No trust boundary.** The control plane authenticates no one. Run it for
  yourself, on your own machine.
- **Evidence retention: evidence lives in the local control plane's memory
  and is lost when it stops.** This is the policy, not an accident: read or
  keep a run's evidence before `cloudbox server stop`.

## Start

- [Quickstart](docs/quickstart.md): from no cluster to evidence in about ten
  minutes.
- Documentation: [docs/README.md](docs/README.md), including
  [Concepts](docs/concepts.md) and [Limitations](docs/limitations.md).

## License

Apache 2.0. See [LICENSE](LICENSE).
