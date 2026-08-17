## Context

The kube driver's egress record travels a chain of four links, and three of them
can drop a record without saying so:

```
workload ──▶ egress proxy ──▶ /attempts ──▶ control plane ──▶ evidence
             (in-memory,      (read on      (folds into      (counts
              unbounded,       sandbox       BlockedEgress)   len(BlockedEgress))
              single replica)  inspection)
                   │                │                              │
             OOM or restart    never read =              a smaller number,
             loses the tail    never collected           indistinguishable
                                                         from a quiet run
```

Today the proxy holds attempts in an unbounded slice
(`cmd/cloudbox-proxy/main.go`), runs as a single replica with no resource limits
(`internal/cluster/kube/proxy.go`), and the control plane collects only inside
`refreshBlockedEgressLocked`, which runs when a sandbox is read
(`internal/core/sandboxes.go`). `evidence.EgressViolations` is then
`len(sb.BlockedEgress)`. Every link is best-effort, and the final number is
presented as fact.

`Cluster.AttemptEgress` is a separate defect in the same area: under kube it
returns `{Allowed: true, Via: "unfiltered"}` unconditionally
(`internal/cluster/kube/seal.go`). It is unreachable — `/simctl` is registered
only for the sim driver — so this is a latent lie rather than an active one.

In-force ADRs: 0001–0006 and 0008 (0008 supersedes 0007). ADR 0001 makes the
proxy a mandatory product-managed component and requires residual channels be
published rather than papered over. ADR 0008 freezes `internal/cluster/cluster.go`
and requires a method real Kubernetes cannot satisfy to be surfaced as a finding
rather than reshaped. ADR 0004 makes the control plane the sole minter of
evidence, which is why an incomplete record is an evidence-integrity defect
rather than a logging nuisance.

## Goals / Non-Goals

**Goals:**

- A blocked attempt that happened is either recorded, or the sandbox says its
  record is incomplete. No third outcome.
- Egress evaluation under kube derives from live cluster state.
- The attempt surface is not an open read for anything on the cluster.
- No change to `internal/cluster/cluster.go` and no new requirement outside
  `kube-driver`.

**Non-Goals:**

- Durable storage of attempt history beyond a sandbox's lifetime.
- Transparent redirection (ADR 0001 roadmap; DIVERGENCES entry 4).
- Changing what the sim driver does — it records synchronously and cannot lose
  a record, so no new divergence arises.
- Wildcard allowlist semantics.

## Decisions

### D1. `AttemptEgress` evaluates live seal state, not a live packet

The driver reads the namespace's seal (presence of the `cloudbox-default-deny`
policy) and its live allowlist ConfigMap, then answers:

| Condition | Result |
|---|---|
| No seal on the namespace | `Allowed: true, Via: "unfiltered"` |
| Destination is a Service in the namespace | `Allowed: true, Via: "in-sandbox"` |
| Destination is cluster DNS | `Allowed: true, Via: "cluster-dns"` |
| FQDN on the live allowlist | `Allowed: true, Via: "egress-proxy"` |
| Otherwise | `Allowed: false, Via: "denied"` |

*Alternative considered:* run a canary pod per call and report what it observed.
That is packet-truth rather than policy-truth, but costs seconds and a pod per
evaluation. Rejected: packet-level enforcement is already proven by
`ProbeEnforcement` and by the conformance egress scenarios, which assert from a
real workload's own output. This method answers the question the contract
actually asks — what the cluster enforces for this namespace — and the residual
gap is stated below rather than hidden.

*Bounded claim:* this evaluates policy, not a packet. A CNI that stopped
enforcing after the probe would not be caught here; it is caught by the probe,
which is where that claim belongs.

### D2. Bounded retention keeps raw records, and reports what it dropped

The proxy retains a bounded number of raw attempts and maintains a monotonic
`dropped` counter. The `/attempts` response becomes an object carrying the
attempts, the dropped count, and the proxy's incarnation, instead of a bare
array.

*Alternative considered — aggregate by (destination, source):* keep one entry
per distinct destination with first-seen, last-seen and a count. This is
strictly better on memory (the common case is a workload retrying one blocked
destination in a loop — exactly what the conformance prober does) and loses
nothing N4 asks for. **Rejected for this change** because
`evidence.EgressViolations` is `len(BlockedEgress)`: aggregation silently
redefines a number that already appears in minted evidence from "attempts" to
"distinct destinations". That is a change to the `evidence` capability's
meaning, and it belongs in a proposal that says so. Carried to Open Questions.

*Drop policy:* oldest-first. The alternative — keep the first N, drop new
arrivals — preserves the most diagnostically useful records (first contact with
a destination) but makes a loop-retrying workload mask a later, different
destination. Oldest-first plus a reported drop count keeps the count honest and
the recent picture accurate; the aggregation decision above is the real fix.

### D3. Collection runs on a timer, not on inspection

A single control-plane collector iterates sealed sandboxes on an interval and
folds records in through the existing `cluster.EgressObserver` capability.
`refreshBlockedEgressLocked` stays as it is for read-time freshness; it stops
being the only path.

*Alternative considered:* the proxy pushes to the control plane. Rejected — the
kube driver deliberately assumes no direct network path from the pod network to
`cloudboxd`, which is why collection goes through the API server's service proxy
today. Reversing that direction would be a new architectural assumption.

### D4. A restart marks the record incomplete, conservatively

The proxy generates an incarnation identifier at startup and returns it with
every collection. When the control plane sees an incarnation change for a
namespace it has already collected from, it marks that sandbox's egress record
incomplete.

This is deliberately conservative: a restart that lost nothing still marks the
record incomplete, because the proxy cannot prove the negative. Proving it would
require the proxy to persist a sequence across restarts, which a stateless pod
cannot do. Given the choice between claiming completeness we cannot demonstrate
and admitting doubt we cannot rule out, this codebase's consistent answer is to
admit the doubt.

To keep that from being noisy, the proxy deployment gains resource requests and
limits and a readiness probe, so a restart becomes an event rather than a
routine.

### D5. The admin surface is defended by a credential, not by policy

At seal time the control plane creates a per-namespace Secret; the proxy reads
the token and requires it on `/attempts`, and the collector presents it.

*Alternative considered:* narrow the `cloudbox-egress-proxy-admin` NetworkPolicy
with an `ipBlock` for the node CIDR. Rejected — the source address of API-server
service-proxy traffic is a cluster-topology detail, and encoding it into the
seal would make the seal cluster-specific and quietly breakable by a topology
change. **The ingress policy therefore stays permissive on the admin port by
necessity; the token is the actual control.** Stating that plainly is the point:
the policy is not what is protecting this surface.

## Risks / Trade-offs

- **A proxy that starts, records, and dies inside one collection interval still
  loses records** → D4 catches it: the incarnation changes, so the sandbox is
  marked incomplete rather than reporting a clean short count.
- **Conservative incompleteness could become routine noise if proxies are
  evicted often** → resource requests/limits and a readiness probe (D4); if it
  persists, the follow-up is aggregation plus a smaller footprint, not a looser
  claim.
- **Timer collection adds API-server load proportional to sealed sandboxes** →
  one collector iterating sealed sandboxes, not a goroutine per sandbox;
  interval tunable.
- **Version skew**: a namespace sealed before an upgrade runs an older proxy
  image returning a bare array → the collector accepts both the array and the
  object shape for one release.
- **`AttemptEgress` reports policy, not packet** → stated as a bounded claim
  (D1); packet truth stays with the probe and the conformance scenarios.

## Migration Plan

1. Proxy: bounded retention, dropped counter, incarnation, token check. Response
   shape becomes an object; the old bare array remains parseable by the
   collector.
2. Driver: pass the incarnation and dropped count through `EgressAttempts`;
   deploy the Secret, resource limits and readiness probe at seal time.
3. Control plane: timer collection; incompleteness on the sandbox record and in
   evidence.
4. `AttemptEgress` live evaluation.
5. Conformance scenarios for each Rule.

Rollback is per-step; nothing here changes persisted state or the cluster
contract. A re-seal redeploys the proxy, so reverting the image is sufficient to
revert steps 1–2.

## Open Questions

1. **Should aggregation replace raw attempt records?** It is the better data
   model (D2) but redefines `evidence.EgressViolations` from attempts to
   distinct destinations. That is a `evidence` requirement change and needs its
   own proposal. Deferred deliberately, not overlooked.
2. **Should an incomplete egress record refuse evidence rather than annotate
   it?** The probe-failure path refuses evidence outright (409, "no evidence").
   An incomplete containment record is a weaker defect than an unproven seal,
   so this design annotates. If review reads ADR 0004 as requiring that minted
   evidence be complete rather than merely honest about its gaps, this becomes
   a refusal and the specs change accordingly.
3. **Collection interval.** 15s is proposed as short relative to pod restart
   timing; to be confirmed against conformance runtime and API-server load.

No in-force ADR needs revisiting. This change operates inside ADR 0001's proxy
mechanism and ADR 0008's frozen contract, and contradicts neither.
