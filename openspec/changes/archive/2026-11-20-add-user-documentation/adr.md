# ADR Review Manifest

- Status: completed
- Review date: 2026-11-20

## Review Summary

ADR review completed for this change. Design D2 and D3 — documentation that
restates product facts is checked against the product, by coverage rather than
wording — establish a commitment every later change touching the CLI, exit
statuses, containment, or refusals must honour, so it is recorded as a new
repository-level ADR. D1 (Markdown under `docs/`), D5 (troubleshooting keyed by
outcome), D6 (concepts render the business glossary), and D7 (sequencing) are
tactical and are not recorded as ADRs.

## In-Force ADRs Reviewed

- `adr/0001-standard-networkpolicy-floor-with-egress-proxy.md` — the containment page describes this floor and its recording limit.
- `adr/0002-bundle-digest-as-universal-identity.md` — the concepts page defines the bundle in these terms.
- `adr/0003-boundary-contract-and-recorded-transforms-over-overlays.md` — the concepts page defines the boundary contract in these terms.
- `adr/0004-control-plane-mints-all-evidence.md` — thin CLI; the docs describe the control plane as the authority and user-controlled evidence as non-postable.
- `adr/0005-add-on-adoption-ladder-feeding-existing-cd.md` — the concepts page explains L1–L4.
- `adr/0006-application-scoped-substrate-lockfile.md` — the concepts page defines substrate and parity in these terms.
- `adr/0008-kube-driver-behind-a-frozen-contract-with-a-tagged-conformance-subset.md` — no doc check needs a real cluster; none is tagged `@conformance`.
- `adr/0009-unimplemented-integrations-refuse-rather-than-simulate.md` — limitations and troubleshooting describe these refusals.
- `adr/0010-released-substrate-is-a-user-controlled-cluster.md` — the cluster setup guide describes this substrate.

`adr/0007-go-control-plane-with-simulated-cluster-driver.md` is superseded by
0008 and was read as historical context only. No in-force ADR is revisited.

## New Durable ADRs Created

- `adr/0011-user-documentation-is-verified-against-the-product.md`
