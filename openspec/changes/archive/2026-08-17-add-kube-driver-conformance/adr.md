# ADR Review Manifest

- Status: completed
- Review date: 2026-08-14

## Review Summary

ADR review completed for this change. All seven existing repository-level ADRs
were read and the supersession graph built from their `Supersedes` fields: every
one was `accepted` with an empty `Supersedes`, so all seven were in force going
into this change. One decision in design.md met the durability bar and required
superseding a prior ADR; the remainder are tactical (CNI choice, cluster
topology in CI, task sequencing) or already covered by the in-force set.

## In-Force ADRs Reviewed

- [0001-standard-networkpolicy-floor-with-egress-proxy.md](../../../adr/0001-standard-networkpolicy-floor-with-egress-proxy.md) — constrains this change directly: the seal floor the kube driver must implement with standard NetworkPolicy v1 plus the product egress proxy, and the transparent-redirection mechanism most likely to diverge from the simulation.
- [0002-bundle-digest-as-universal-identity.md](../../../adr/0002-bundle-digest-as-universal-identity.md) — unaffected; digest identity is computed above the driver.
- [0003-boundary-contract-and-recorded-transforms-over-overlays.md](../../../adr/0003-boundary-contract-and-recorded-transforms-over-overlays.md) — unaffected; the contract kinds and recorded transforms are driver-independent.
- [0004-control-plane-mints-all-evidence.md](../../../adr/0004-control-plane-mints-all-evidence.md) — reinforced: the probe-failure requirement means a non-enforcing cluster yields no evidence at all.
- [0005-add-on-adoption-ladder-feeding-existing-cd.md](../../../adr/0005-add-on-adoption-ladder-feeding-existing-cd.md) — unaffected; adoption levels are policy, not substrate.
- [0006-application-scoped-substrate-lockfile.md](../../../adr/0006-application-scoped-substrate-lockfile.md) — relevant: the kube driver reads Kubernetes minor version and installed components from the live cluster so lockfile digests reflect reality.
- [0007-go-control-plane-with-simulated-cluster-driver.md](../../../adr/0007-go-control-plane-with-simulated-cluster-driver.md) — **superseded by 0008.** Its stack and driver-interface decisions are carried forward; only its time-bound consequence that `--driver kube` fails loudly no longer describes current behaviour. Its file is left frozen.

## New Durable ADRs Created

- [0008-kube-driver-behind-a-frozen-contract-with-a-tagged-conformance-subset.md](../../../adr/0008-kube-driver-behind-a-frozen-contract-with-a-tagged-conformance-subset.md) — supersedes ADR-0007. Records the frozen cluster contract, the tagged conformance subset and its recorded exclusions, the enforcement gate proven in both directions, the real-clock rule for time-dependent conformance, arrangement through the product's own surface rather than a test endpoint, and the sim-divergence reconciliation obligation.
