# 0006 — Application-scoped substrate lockfile

- Status: accepted
- Date: 2026-08-14
- Supersedes: —

## Context

"Production-shaped" must be a checked fact: sandbox and production substrates
(Kubernetes version, CRDs, operators, admission config, named classes) are
hash-compared. The naive design digests the whole cluster — but then every
unrelated operator upgrade invalidates every application's evidence and
in-flight soak time, producing constant false staleness for zero signal.

## Decision

The substrate lockfile is scoped per application to what its bundles actually
reference: the Kubernetes minor version, the CRDs and operator releases the
bundles instantiate, applicable admission configurations, and the
gateway/storage/priority classes they name — with a digest over that set.
Drift detection carries the same scope: production drift invalidates only the
applications that reference the drifted component. Cloud identity bindings and
secret values are recorded declared-not-verified — the seal proves
reachability, never authorization.

## Consequences

- Evidence staleness is meaningful: a digest change always names a component
  the application actually depends on.
- Reference extraction from bundles becomes a load-bearing controller
  capability (what does this bundle instantiate/name?).
- Two applications on one cluster can have different parity verdicts at the
  same moment — correct, but needs clear surfacing.
- Declared-not-verified items (IAM, secret values) must be labeled in
  evidence so parity claims stay honest.
