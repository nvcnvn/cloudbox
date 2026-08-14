# ADR Review Manifest

- Status: completed
- Review date: 2026-08-14

## Review Summary

ADR review completed for this change. `<repo>/adr/` did not exist before this
change, so there was no supersession graph to walk; the six ADRs below start
the sequence. Decisions in design.md that are tactical or spec-local (capacity
squeeze semantics, data-fidelity sequencing, SHOULD-as-MUST carries) were
deliberately not promoted to repository-level ADRs — they live in the delta
specs and design.md.

## In-Force ADRs Reviewed

- None — `<repo>/adr/` had no in-force ADRs before this change.

## New Durable ADRs Created

- [0001-standard-networkpolicy-floor-with-egress-proxy.md](../../../adr/0001-standard-networkpolicy-floor-with-egress-proxy.md) — seal enforcement floor is standard NetworkPolicy v1 plus the product egress proxy, probe-verified.
- [0002-bundle-digest-as-universal-identity.md](../../../adr/0002-bundle-digest-as-universal-identity.md) — content-addressed bundle digest is the sole identity for diff, evidence, soak, and promotion; renders must be deterministic.
- [0003-boundary-contract-and-recorded-transforms-over-overlays.md](../../../adr/0003-boundary-contract-and-recorded-transforms-over-overlays.md) — four-kind boundary contract and digest-preserving recorded transforms; no overlay mechanism.
- [0004-control-plane-mints-all-evidence.md](../../../adr/0004-control-plane-mints-all-evidence.md) — server-side authority; CI untrusted; thin CLI; local evidence non-promotable.
- [0005-add-on-adoption-ladder-feeding-existing-cd.md](../../../adr/0005-add-on-adoption-ladder-feeding-existing-cd.md) — per-application L1–L4 ladder; feed the team's CD, never replace it.
- [0006-application-scoped-substrate-lockfile.md](../../../adr/0006-application-scoped-substrate-lockfile.md) — substrate lockfile and drift detection scoped to referenced components.
