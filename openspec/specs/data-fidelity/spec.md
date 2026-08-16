# Data fidelity

The seal proves dependency closure because it is a closed-world check; data is
open-world — it cannot be proven equal, only representative. The product
verifies shape, declares grade, and bounds the rest downstream. No requirement
here moves production values by default. Covers draft requirements D1–D5, D7,
D8.

Scope notes (v1): the profile-synthetic generator (D6) is v1.x, deliberately
sequenced behind thin clones — recorded in design.md. The draft grades
thin-clone drivers (D7) SHOULD ship in v1; they are carried here as a v1
requirement (see design.md, Decisions). Masking is a partner integration,
never an in-house engine.

```gherkin
Feature: Data fidelity

  Rule: Each declared datastore has a content-addressed data profile lockfile
    The control plane MUST maintain a per-application profile per declared
    datastore — schema digest plus statistical profile (null rates,
    cardinalities, length distributions, character classes, row counts,
    referential fan-out) — profiled against production without production
    values leaving it.

    Scenario: Profiling captures shape while values stay in production
      Given an application declaring a PostgreSQL datastore
      When the control plane profiles the production database
      Then the profile records the schema digest and per-column statistics, content-addressed
      And no production value leaves the production environment

  Rule: Evidence declares the fidelity level of every datastore in the run
    Every run's evidence MUST declare, per datastore, one of: fixtures,
    schema-replay, profile-synthetic, masked-snapshot, live-clone.

    Scenario: A run's evidence grades each datastore
      Given a sandbox run against a schema-replay database and a fixtures cache
      When the evidence is assembled
      Then the database is declared at schema-replay and the cache at fixtures

  Rule: Fidelity policy minimums invalidate low-fidelity evidence
    Per-application policy MUST support minimum fidelity levels, including
    conditional rules such as requiring schema-replay or higher for bundles
    containing a migration; evidence below the applicable minimum MUST be
    marked invalid, failing the evidence check and blocking promotion.

    Scenario: A migration bundle run at fixtures produces invalid evidence
      Given an application policy requiring at least schema-replay for bundles containing a migration
      And a bundle containing a migration run at fixtures fidelity
      When the evidence is evaluated
      Then the evidence is marked invalid for fidelity below the applicable minimum
      And the evidence check fails and any promotion blocks

  Rule: Sandbox provisioning supports migration replay
    Provisioning MUST be able to instantiate a datastore from the profile
    lockfile's schema and run the bundle's migration chain against it, with
    failures surfaced in status and in evidence.

    Scenario: A failing migration surfaces before it reaches production
      Given a datastore instantiated from the profile lockfile's schema
      When the bundle's migration chain runs and one migration fails
      Then the failure appears in the sandbox status
      And the failure is recorded in the run's evidence

  Rule: Data profile drift invalidates stale evidence at its declared level
    Production schema changes or distribution shifts beyond threshold MUST
    update the profile lockfile digest so stale sandboxes stop producing
    valid evidence at their declared fidelity level.

    Scenario: A production schema change stales running sandboxes
      Given a sandbox provisioned from profile digest A
      When production's schema changes and the profile digest moves to B
      Then the sandbox's subsequent evidence is no longer valid at its declared fidelity level

  Rule: Thin-clone drivers provide live-clone fidelity as integrations
    Thin-clone drivers for externally managed databases (branching and clone
    services) ship as integrations providing live-clone fidelity; clone
    endpoints enter the sandbox only through the boundary contract as a
    per-sandbox secret plus an allowlist entry.

    Scenario: A database branch enters the sandbox through the contract
      Given an application using a database service with branching support
      When a sandbox is provisioned at live-clone fidelity
      Then a per-sandbox branch endpoint is supplied as a per-sandbox secret with an allowlist entry
      And the run's evidence declares that datastore at live-clone

  Rule: Real-data levels are admin-enabled and gated for agents
    Masked-snapshot and live-clone MUST be admin-enabled per application,
    never default, and MUST NOT be available to agent-owned sandboxes without
    explicit policy.

    Scenario: Real data is unavailable until an admin enables it
      Given an application where no admin has enabled real-data levels
      When a developer requests a masked-snapshot sandbox
      Then the request is refused because real-data levels are not enabled for the application

    Scenario: An agent sandbox cannot get real data without explicit policy
      Given an application with live-clone enabled but no agent real-data policy
      When an agent-owned sandbox requests live-clone fidelity
      Then the request is refused pending explicit policy for agent-owned sandboxes
```
