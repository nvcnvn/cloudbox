# 0003 — Boundary contract and recorded transforms instead of overlays

- Status: accepted
- Date: 2026-08-14
- Supersedes: —

## Context

Environments legitimately differ in a few values. Every existing answer
(Kustomize overlays, Helm per-env values, templating) lets the variance list
grow unbounded — which reintroduces exactly the environment drift the product
exists to eliminate.

## Decision

Environment variance is confined to a declared, finite boundary contract of
exactly four kinds: secret names, ingress hostnames, the egress allowlist, and
internal application dependencies. The two legitimate in-manifest differences —
namespace and capacity — are controller-applied, digest-preserving, recorded
transforms declared in evidence, not user templating. No overlay mechanism
exists; the correct response to "template one more field" is a spec change.

## Consequences

- Bundles stay environment-agnostic and portable up the ladder unchanged.
- Growth of the variance list is a visible design-failure signal requiring a
  spec change, not a quiet feature.
- Some teams' existing overlay habits will not port; `check` must turn that
  into a fix list, not a rejection.
- An application occupies exactly one namespace per environment (multi-
  namespace topologies wait for the vcluster opt-up).
