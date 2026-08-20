# 0009 — Unimplemented integrations refuse rather than simulate

- Status: accepted
- Date: 2026-08-19
- Supersedes: —

## Context

Every external system CloudBox depends on except the cluster itself — source
control, GitOps sync, production state, audit sinks, datastore seeding — exists
in the codebase as an in-memory arrangement surface driven by `/simctl/*`. That
was the correct shape for building the product: the whole v1 capability set was
specified and verified against those surfaces before any of them had a real
counterpart.

ADR 0008 withheld `/simctl/*` under the real driver, which stops a *test* from
arranging fake state. It does not stop a *user*. `POST /v1/promotions` under
`--driver kube` today opens a promotion, writes it to a map, and returns
success — having promoted nothing. The same holds for posting an evidence check
and setting production state. As long as nothing was released this cost
nothing; a public release turns it into a product that reports success for work
it did not do.

The alternative shapes were available and each fails differently. Removing the
unimplemented capabilities would discard specified, sim-verified behaviour and
force a spec change when the integration lands. Documenting the limitation
relies on the user reading documentation at the moment they are being lied to.
Leaving it alone trades a discoverable absence for an undiscoverable falsehood.

## Decision

An operation whose external integration is not present MUST refuse, MUST name
the missing integration as the reason, and MUST record no result. Accepting the
request into a simulated surface and reporting success is forbidden.

The condition is the *absence of the integration*, not the name of the driver.
In the simulation suite the arrangement surfaces constitute the integration's
presence, so both paths follow one rule rather than two.

Capabilities remain specified and implemented while their integration is
absent; they become reachable when it lands, with no spec change. Refusal is a
boundary the release declares, not a feature it removes.

## Consequences

- The released artifact advertises its own scope: a caller learns what the
  product does not yet do from the product, at the moment they ask for it.
- Every future change that adds a capability ahead of its integration inherits
  this obligation, and every change that lands an integration must retire the
  corresponding refusal deliberately.
- The product can ship one rung of ADR 0005's ladder without the rungs above it
  becoming latent falsehoods — a precondition for the ladder being adoptable
  incrementally at all.
- A refusal path is now a testable product surface, so "this does not work yet"
  is asserted by the acceptance suite rather than left to prose.
