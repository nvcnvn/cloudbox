# ADR Review Manifest

- Status: completed
- Review date: 2026-08-19

## Review Summary

ADR review completed for this change. Every file under `adr/` was read and the
supersession graph walked: 0008 supersedes 0007, so 0007 is historical context
only and was not treated as a live commitment. The highest sequence in use
before this change was 0008.

Two of design.md's decisions clear the durable bar — they establish boundaries
that constrain changes beyond this one and are not captured by any in-force
ADR. The remaining decisions were reviewed and deliberately not recorded:

- **D3 (the CLI starts and reuses a control plane, never becomes one)** is an
  extension within ADR 0004, not a divergence from it. 0004 already forbids
  moving enforcement into the client; permitting the CLI to launch the same
  server it would otherwise be pointed at changes no architectural boundary.
- **D4 (a third substrate-parity state)** and **D6 (the containment statement
  declares its recording limit)** are spec-level honesty corrections inside
  decisions already made (0006, 0001), captured in the delta specs.
- **D7 (machine-readable output is a rendering of the same facts)** is an
  implementation constraint, not an architectural commitment.

No in-force ADR needs revisiting, so no superseding ADR was written and no
existing ADR file was modified.

## In-Force ADRs Reviewed

- `adr/0001-standard-networkpolicy-floor-with-egress-proxy.md` — constrains the
  seal mechanism whose observability limit D6 makes explicit.
- `adr/0002-bundle-digest-as-universal-identity.md` — unchanged by this change.
- `adr/0003-boundary-contract-and-recorded-transforms-over-overlays.md` —
  unchanged by this change.
- `adr/0004-control-plane-mints-all-evidence.md` — governs D3, and supplies the
  rule that evidence from user-controlled sandboxes is non-promotable and
  non-postable, which ADR 0010 preserves.
- `adr/0005-add-on-adoption-ladder-feeding-existing-cd.md` — this release is its
  L1 rung shipped alone; ADR 0009 is what makes shipping one rung honest.
- `adr/0006-application-scoped-substrate-lockfile.md` — the lockfile whose
  production counterpart is absent in this release, motivating D4.
- `adr/0008-kube-driver-behind-a-frozen-contract-with-a-tagged-conformance-subset.md`
  — freezes the cluster contract, which is why ADR 0010 does not add a
  provisioning method to the driver.

Historical, not in force: `adr/0007-go-control-plane-with-simulated-cluster-driver.md`
(superseded by 0008).

## New Durable ADRs Created

- `adr/0009-unimplemented-integrations-refuse-rather-than-simulate.md` — an
  operation whose external integration is absent refuses, names the missing
  integration, and records nothing.
- `adr/0010-released-substrate-is-a-user-controlled-cluster.md` — the product
  does not create clusters; a user-controlled cluster is registered as a sandbox
  host and its enforcement is proven at registration.
