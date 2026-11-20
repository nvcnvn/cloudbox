# ADR Review Manifest

- Status: completed
- Review date: 2026-08-17

## Review Summary

ADR review completed for this change. All eight repository-level ADRs were read
and the supersession graph built from their `Supersedes` fields: 0007 is
superseded by 0008, leaving 0001–0006 and 0008 in force. No decision in
design.md meets the durability bar, and no new repository-level ADR file was
created.

Every decision here operates *inside* commitments already recorded. D1 satisfies
a method of the contract ADR 0008 froze, rather than renegotiating it — which is
what ADR 0008 already prescribes for this situation. D2–D4 are mechanism choices
about retention, collection timing and restart detection within ADR 0001's
product-managed proxy; a future change could reverse any of them without
disturbing an architectural boundary. D5 chooses a credential over a network
policy for one internal endpoint, which is a tactical defence choice, not a new
security architecture.

The two decisions that *would* have met the bar were both deliberately deferred
to Open Questions rather than made here: redefining `evidence.EgressViolations`
via aggregation (Open Question 1), and whether an incomplete containment record
should refuse evidence outright rather than annotate it (Open Question 2). The
second in particular is a reading of ADR 0004 that would deserve its own ADR if
adopted. Recording either as a durable decision before that argument has been
had would be inventing an ADR to fill a section.

## In-Force ADRs Reviewed

- [0001-standard-networkpolicy-floor-with-egress-proxy.md](../../../adr/0001-standard-networkpolicy-floor-with-egress-proxy.md) — constrains this change most directly: the egress proxy is mandatory and product-managed, and residual channels must be published rather than papered over. D5's admission that the ingress policy stays permissive on the admin port, with the token as the actual control, is that requirement applied.
- [0002-bundle-digest-as-universal-identity.md](../../../adr/0002-bundle-digest-as-universal-identity.md) — unaffected; digest identity is computed above the driver.
- [0003-boundary-contract-and-recorded-transforms-over-overlays.md](../../../adr/0003-boundary-contract-and-recorded-transforms-over-overlays.md) — unaffected; contract kinds and recorded transforms are driver-independent.
- [0004-control-plane-mints-all-evidence.md](../../../adr/0004-control-plane-mints-all-evidence.md) — the reason this change matters: the control plane is the sole minter of evidence, so a silently diminished egress-violation count is an integrity defect in a signed artifact, not a logging gap. Open Question 2 turns on how strictly this ADR is read.
- [0005-add-on-adoption-ladder-feeding-existing-cd.md](../../../adr/0005-add-on-adoption-ladder-feeding-existing-cd.md) — unaffected; adoption levels are policy, not substrate.
- [0006-application-scoped-substrate-lockfile.md](../../../adr/0006-application-scoped-substrate-lockfile.md) — unaffected; this change touches no substrate read.
- [0008-kube-driver-behind-a-frozen-contract-with-a-tagged-conformance-subset.md](../../../adr/0008-kube-driver-behind-a-frozen-contract-with-a-tagged-conformance-subset.md) — governs D1: `internal/cluster/cluster.go` stays frozen, and a method real Kubernetes can satisfy must be satisfied rather than reshaped. `AttemptEgress` is satisfiable from live namespace state, so it is implemented, not surfaced as a finding.
- [0007-go-control-plane-with-simulated-cluster-driver.md](../../../adr/0007-go-control-plane-with-simulated-cluster-driver.md) — **superseded by 0008**; historical context only, reviewed to confirm nothing here revives a superseded commitment.

## New Durable ADRs Created

- None — no major durable architectural decisions were introduced.
