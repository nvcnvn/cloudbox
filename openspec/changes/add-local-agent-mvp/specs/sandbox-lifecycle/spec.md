# Sandbox lifecycle changes

The capability's local-sandbox rule bundles two things together: a *property*
(the cluster is user-controlled, so its evidence must not be promotable or
postable) and a *mechanism* (the product provisions a local Kind cluster from
the lockfile). The property is what makes the rule load-bearing; the mechanism
is a convenience the product does not yet own for real — it is implemented only
against the simulated driver, so the released driver would fail the rule as
written.

This delta keeps the property and drops the mechanism. Registering a cluster
the developer or agent controls becomes the specified path, and product-managed
local provisioning is recorded as direction in design.md rather than required
here. The new rule adds one thing the old one left implicit: a user-controlled
cluster is chosen by the user, so it may not enforce NetworkPolicy at all, and
that MUST be refused at registration rather than discovered when a sandbox
quietly fails to seal. The `sandbox-seal` enforcement probe (N7) remains the
last line; this is the early one.

```gherkin
Feature: Sandbox lifecycle changes

  # @openspec: REMOVED
  Rule: Local sandboxes provision from the substrate lockfile

  # @openspec: ADDED
  Rule: User-controlled clusters are registrable sandbox hosts
    A developer or agent MUST be able to register a cluster they control as a
    sandbox host. Sandboxes on it MUST hold the same seal and iteration
    semantics as a sandbox on a managed cluster, and their evidence MUST be
    marked as produced on a user-controlled cluster and MUST be neither
    promotable nor postable as a check.

    Scenario: A registered user-controlled cluster hosts sandboxes with identical seal semantics
      Given a cluster the developer controls whose CNI enforces NetworkPolicy
      When they register it as a sandbox host and create a sandbox on it
      Then the sandbox enforces the same seal and iteration semantics as a sandbox on a managed cluster

    Scenario: Evidence from a user-controlled cluster cannot reach a check or a promotion
      Given evidence produced by a sandbox on a user-controlled cluster
      When an evidence check is posted or a promotion is opened from it
      Then the attempt is refused because the evidence source is not a control-plane-managed sandbox

    Scenario: Evidence records that its cluster was user-controlled
      Given a sandbox on a registered user-controlled cluster
      When the run's evidence is assembled
      Then the evidence records that it was produced on a user-controlled cluster

  # @openspec: ADDED
  Rule: A sandbox host that cannot be shown to enforce NetworkPolicy is refused
    Registration MUST confirm that the candidate cluster enforces
    NetworkPolicy before accepting it as a sandbox host. A cluster that cannot
    be shown to enforce MUST be refused, and the refusal MUST name unproven
    enforcement as the cause rather than reporting a generic failure.

    Scenario: A non-enforcing cluster is refused at registration
      Given a cluster whose CNI does not enforce NetworkPolicy
      When it is registered as a sandbox host
      Then the registration is refused naming unproven network policy enforcement
      And no sandbox can be created on it

    Scenario: An enforcing cluster is accepted
      Given a cluster whose CNI enforces NetworkPolicy
      When it is registered as a sandbox host
      Then the registration is accepted
```
