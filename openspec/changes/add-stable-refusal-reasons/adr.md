# ADR Review Manifest

- Status: completed
- Review date: 2026-11-20

## Review Summary

ADR review completed for this change.

## In-Force ADRs Reviewed

- 0001 — standard NetworkPolicy floor with egress proxy
- 0002 — bundle digest as universal identity
- 0003 — boundary contract and recorded transforms over overlays
- 0004 — control plane mints all evidence (thin CLI; the catalog package holds no enforcement)
- 0005 — add-on adoption ladder feeding existing CD
- 0006 — application-scoped substrate lockfile
- 0008 — kube driver behind a frozen contract with a tagged conformance subset (supersedes 0007, which is historical only)
- 0009 — unimplemented integrations refuse rather than simulate (`integration-missing`)
- 0010 — released substrate is a user-controlled cluster
- 0011 — user documentation is verified against the product (troubleshooting coverage moves to a product oracle)

## New Durable ADRs Created

- [`adr/0012-refusals-carry-a-stable-append-only-reason.md`](../../../adr/0012-refusals-carry-a-stable-append-only-reason.md)
  — every refusal names a reason from a closed, append-only catalog that the
  command line publishes, and programs branch on reasons, never on prose.
  This commitment binds every future change that adds a refusal.
