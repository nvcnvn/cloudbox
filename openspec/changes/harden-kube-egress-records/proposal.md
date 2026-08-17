# Proposal: harden-kube-egress-records

## Why

The kube driver's egress path has two places where the product can report less
than the truth, and both fail silently — the one failure mode this codebase
consistently refuses to tolerate.

The egress proxy accumulates blocked attempts in an unbounded in-memory slice,
in a single replica, with no memory limit and no persistence. The control plane
collects those records only when a sandbox is *read*. A proxy that restarts, is
rescheduled, or dies of its own unbounded growth loses every attempt not yet
collected — and `evidence.EgressViolations` is a count of what survived, so the
run's evidence quietly reports fewer violations than actually happened. Nothing
in the product notices. `sandbox-seal`'s N4 requires that blocked attempts be
recorded and surfaced in evidence; on a long-lived sandbox the kube driver
currently satisfies that only by luck.

Separately, `Cluster.AttemptEgress` under the kube driver returns
`{Allowed: true, Via: "unfiltered"}` for every destination without evaluating
anything. It is unreachable today — its only exposure is the `/simctl` surface,
which is never registered under `--driver kube` — so it costs nothing now and
everything on the day something routes to it. A stub that answers "allowed,
unfiltered" is precisely the lie the driver's own doctrine forbids: the package
degrades honestly, so that a sandbox is never reported sealed on a cluster it
could not actually seal.

Both are small. Both are the kind of defect that is cheap now and expensive
after someone has trusted the number.

## What Changes

- **The egress proxy's attempt record becomes bounded and loss-aware.** The
  proxy retains a bounded number of attempts and counts what it dropped;
  truncation is reported, never silent. The proxy identifies its own incarnation
  so the control plane can tell a restart from a quiet stretch, and a sandbox
  whose records may be incomplete says so rather than presenting a diminished
  count as a complete one.
- **The control plane stops collecting only on read.** Attempt collection
  becomes independent of whether anyone happens to inspect the sandbox, closing
  the window in which a restart discards uncollected records.
- **`AttemptEgress` under kube evaluates real state instead of asserting.** The
  driver decides from what the cluster actually enforces — the namespace's seal
  and its live allowlist — and reports proxy-mediated, denied, in-sandbox, or
  cluster-DNS accordingly. It MUST NOT report an unevaluated attempt as allowed.
- **The proxy's admin surface stops being world-readable on-cluster.** The
  `/attempts` endpoint currently accepts ingress from any source on its port;
  any pod in an unsealed namespace can read one sandbox's attempted
  destinations. This is disclosure only — the endpoint is read-only and nothing
  can forge a record through it — but the same endpoint is the trust root for
  the violation count in evidence, and it should not be an open read.

### Non-goals

- **Wildcard or subdomain allowlist entries.** `sandbox-seal` specifies "FQDN
  entries on the application's declared allowlist"; exact-match is the specified
  behaviour, not a gap. Widening it is an allowlist-semantics change for its own
  proposal, with its own threat-model argument.
- **Transparent pod-level redirection.** ADR 0001's roadmap mechanism, recorded
  as `internal/sim/DIVERGENCES.md` entry 4. Proxy-unaware and non-HTTP workloads
  stay sealed by the CNI, unrecorded by FQDN, exactly as today.
- **Durable storage of attempt records.** Records survive within a sandbox's
  lifetime and their loss is surfaced; a datastore for egress history is a
  different product.
- **Changing the cluster contract.** `internal/cluster/cluster.go` stays frozen
  (ADR 0008). `AttemptEgress` is satisfiable as written; this change satisfies
  it rather than renegotiating it.
- **The sim driver's egress model.** The sim records attempts synchronously and
  cannot lose them; nothing here changes its behaviour or requires a new
  divergence entry.

## Capabilities

### New Capabilities

None. Every behaviour here belongs to the driver that already owns it.

### Modified Capabilities

- `kube-driver`: adds requirements that the driver's egress evaluation reflect
  live cluster state rather than an unevaluated default, that the proxy's
  attempt record be bounded with non-silent truncation, that record loss across
  a proxy restart be surfaced rather than silently reducing the violation count,
  and that the proxy's attempt surface not be readable from outside the control
  plane's collection path.

`sandbox-seal` is deliberately **not** modified. N4 already requires blocked
attempts to be recorded, attributed, and surfaced in evidence, and the
threat-model rule already governs published containment claims. The gap is not
in what v1 requires — it is that the kube driver's mechanism does not reliably
meet it. Restating a requirement the driver fails would confuse a specification
gap with an implementation one.

## Impact

**Modified code**
- `cmd/cloudbox-proxy/main.go` — bounded retention with a dropped counter, an
  incarnation identifier on the attempts response, and authentication on the
  admin listener.
- `internal/cluster/kube/proxy.go` — collection carries the incarnation and
  dropped count through to the control plane; the deployment gains the admin
  credential, and the resource limits and probes a single-replica component
  needs to not be the thing that loses the records.
- `internal/cluster/kube/seal.go` — `AttemptEgress` evaluates live seal state;
  the `cloudbox-egress-proxy-admin` ingress policy narrows to the collection
  path.
- `internal/core/sandboxes.go` — record-loss is folded into the sandbox record
  and surfaced, rather than absorbed into a lower count.
- `acceptance-tests/` — new `@conformance` scenarios for loss reporting and
  egress evaluation, plus their page objects. Step definitions stay free of
  selectors and URLs.

**Unchanged interfaces**
- `internal/cluster/cluster.go` (frozen, ADR 0008).
- The `cluster.EgressObserver` optional capability keeps its shape; what travels
  through it gains fidelity.

**In-force ADRs**
- 0001 (NetworkPolicy floor with egress proxy) and 0008 (kube driver behind a
  frozen contract) constrain this change directly. No new ADR is expected: this
  is a fidelity correction inside decisions already made, not a new
  architectural direction.

**Risk if not done**
- An evidence artifact that under-reports egress violations is worse than one
  that reports none, because it is trusted.
