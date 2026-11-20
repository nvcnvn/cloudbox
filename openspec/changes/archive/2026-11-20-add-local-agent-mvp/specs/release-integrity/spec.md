# Release integrity

A published artifact makes claims simply by existing: that it is a particular
build, that the components it deploys are the ones it says, and that the
operations it exposes do what their names suggest. CloudBox reaches its first
public release with most of its capability surface implemented against
simulated external systems, so the last of those claims is the one at risk —
promotion, evidence checks, production state, audit sinks, and datastore
seeding are all backed by in-memory arrangement surfaces that would accept a
request and report success.

This capability is the release's boundary. It says what the artifact must be
able to tell an operator about itself, and it forbids the product from
answering for a system it does not have. The doctrine is the one the rest of
the codebase already follows: degrade honestly, and never let a caller mistake
an unimplemented path for a working one.

```gherkin
Feature: Release integrity

  # @openspec: ADDED
  Rule: The released binaries identify the build they came from
    Each released binary MUST report a version that identifies the build it
    was produced from, so an operator reporting a defect and a maintainer
    reproducing it are talking about the same artifact. A binary MUST NOT
    report a fixed placeholder that cannot distinguish one build from another.

    Scenario: A released binary reports its build version
      Given a binary produced by the project's release process
      When its version is requested
      Then it reports the version identifying that build

    Scenario: A build made outside the release process is distinguishable
      Given a binary built locally rather than by the release process
      When its version is requested
      Then the report identifies it as an unreleased build
      And it does not present itself as a released version

  # @openspec: ADDED
  Rule: The egress proxy is deployed from a pinned, resolvable reference
    The seal's allowlist path depends on a product-supplied egress proxy. The
    driver MUST deploy it from a pinned reference that a target cluster can
    resolve, never from an image that only exists because a developer loaded
    it locally.

    Scenario: Sealing deploys the proxy from the release's pinned reference
      Given a registered cluster with no product images preloaded
      When a sandbox is sealed
      Then the egress proxy is deployed from the release's pinned image reference

  # @openspec: ADDED
  Rule: A sandbox whose proxy cannot be provisioned is never reported as allowlist-enforcing
    Default-deny holds without the proxy, but the FQDN allowlist does not. When
    the proxy cannot be provisioned, allowlisted egress MUST fail closed, the
    sandbox MUST report the proxy unavailable naming the reference it could not
    resolve, and the sandbox MUST NOT be reported as enforcing its allowlist.

    Scenario: An unresolvable proxy image surfaces rather than silently disabling the allowlist
      Given a cluster that cannot resolve the release's egress proxy image
      When a sandbox is created on it
      Then the sandbox reports the proxy unavailable naming the unresolvable reference
      And the sandbox is not reported as enforcing its allowlist

    Scenario: Allowlisted egress fails closed while the proxy is unavailable
      Given a sandbox whose egress proxy could not be provisioned
      And an external endpoint on the application's declared allowlist
      When a workload attempts that declared endpoint
      Then the attempt is denied rather than allowed unmediated

  # @openspec: ADDED
  Rule: An operation with no implemented integration refuses and records nothing
    Where an operation depends on an external system this release does not
    integrate with, the control plane MUST refuse it, MUST name the missing
    integration as the reason, and MUST NOT record any result. Accepting the
    request into a simulated surface and reporting success is forbidden: a
    recorded promotion that promoted nothing is worse than a refusal, because
    it is believed.

    Scenario: Opening a promotion refuses by naming the missing integration
      Given a released control plane with no source-control or GitOps integration
      When a promotion is opened
      Then the request is refused naming the missing integration
      And no promotion record is created

    Scenario: Posting an evidence check refuses rather than recording one
      Given a released control plane with no source-control integration
      When an evidence check is posted for a pull request
      Then the request is refused naming the missing integration
      And no status check is recorded

    Scenario: Setting production state refuses rather than accepting a value
      Given a released control plane with no production ingest
      When production state is set for an application
      Then the request is refused naming the missing integration
      And no production state is recorded for that application

  # @openspec: ADDED
  Rule: The release declares its required substrate and the trust it does not provide
    The published quickstart MUST name the cluster property the product
    depends on — a CNI that enforces NetworkPolicy — and MUST state that this
    release performs no authentication of the identity a request acts as, so
    an operator learns the boundary from the documentation rather than from
    an incident.

    Scenario: The quickstart names the enforcing-CNI requirement
      Given the release's published quickstart
      When an operator reviews it
      Then it names a NetworkPolicy-enforcing CNI as a requirement of the target cluster

    Scenario: The quickstart states that requester identity is unauthenticated
      Given the release's published quickstart
      When an operator reviews it
      Then it states that the control plane does not authenticate the identity a request acts as
      And it does not present the control plane as a trust boundary between users
```
