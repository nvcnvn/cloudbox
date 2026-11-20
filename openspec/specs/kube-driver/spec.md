# Kube driver

The `kube` driver is the production path deferred by ADR 0007: a real-Kubernetes
implementation of the `cluster.Cluster` contract that the sim driver has been
modelling. Its purpose is not new product behaviour — every requirement it
serves is already specified under `sandbox-seal`, `sandbox-lifecycle`, and
`substrate-parity`. Its purpose is to make those requirements true against a
real CNI instead of a simulation.

Two properties matter more than the rest and are specified here rather than left
to implementation. First, the driver MUST NOT be allowed to widen the interface
to make itself easier to write: `internal/cluster/cluster.go` is the contract,
and a method real Kubernetes cannot satisfy as written is a finding to surface,
not a signature to change. Second, on a cluster whose CNI silently ignores
NetworkPolicy, the driver MUST fail the enforcement probe rather than let a
sandbox be reported sealed — the honest failure is the whole point of N7, and it
is the single behaviour most worth proving on real infrastructure.

Every scenario in this capability requires a real cluster and is therefore
tagged `@conformance`, which the default sim run excludes (see
`conformance-ci`).

```gherkin
@conformance
Feature: Kube driver

  Rule: The kube driver satisfies the cluster contract without widening it
    The `kube` driver MUST implement every method of the cluster contract as
    written. The contract MUST NOT gain, lose, or change a method to
    accommodate the real driver; a method real Kubernetes cannot satisfy as
    specified MUST be surfaced as a spec-visible finding instead.

    Scenario: The control plane boots against a real cluster
      Given a reachable Kubernetes cluster with an enforcing CNI
      When cloudboxd starts with the kube driver
      Then the control plane reports healthy
      And the cluster contract is served by the kube driver

    Scenario: The kube driver installs and lists the product CRDs
      Given cloudboxd running against a real cluster
      When the control plane installs the product custom resource definitions
      Then the definitions are present on the real cluster
      And listing them through the driver returns the installed set

  Rule: Sealing a real namespace admits only cluster DNS and the egress proxy
    Sealing a namespace on a real cluster MUST install standard NetworkPolicy
    v1 whose only admitted egress is cluster DNS and the product egress proxy,
    using no vendor policy custom resources.

    Scenario: A sealed namespace carries a standard default-deny policy
      Given a sealed sandbox on a real cluster
      When the namespace's network policies are read from the cluster
      Then a default-deny ingress and egress policy is present
      And every policy read is standard NetworkPolicy v1
      And no vendor policy custom resource is used

  Rule: A real workload's blocked egress is denied and recorded
    Under a real seal, a connection from inside the sandbox to a destination
    outside the allowlist MUST be denied by the cluster, and the attempt MUST
    be recorded with its destination and attribution.

    Scenario: An undeclared destination is denied on real infrastructure
      Given a sealed sandbox on a real cluster running a workload
      And "api.other-vendor.com" is not on the application allowlist
      When the workload attempts to connect to "api.other-vendor.com"
      Then the connection is denied by the cluster
      And the blocked attempt is recorded with its destination and attribution

    Scenario: An allowlisted destination is reachable through the proxy
      Given a sealed sandbox on a real cluster whose allowlist declares one external endpoint
      When a workload connects to that declared endpoint
      Then the connection succeeds through the egress proxy

    Scenario: In-sandbox service-to-service traffic and cluster DNS survive the seal
      Given a sealed sandbox on a real cluster running two services
      When one service resolves and calls the other by its short name
      Then name resolution and the in-sandbox connection succeed

  Rule: A non-enforcing CNI fails the probe and never yields a sealed sandbox
    On a cluster whose CNI accepts NetworkPolicy objects without enforcing
    them, the enforcement probe MUST fail and no sandbox on that cluster MUST
    be reported sealed or emit evidence. Because registration probes the
    candidate cluster, such a cluster MUST be refused as a sandbox host, and
    no sandbox MUST be created on it. The control plane MUST report the cause
    as unenforced network policy rather than a generic failure.

    Scenario: A cluster that accepts but ignores NetworkPolicy is caught
      Given a real cluster whose CNI accepts NetworkPolicy objects without enforcing them
      When it is registered as a sandbox host
      Then the registration is refused naming unproven network policy enforcement
      And no sandbox can be created on it

  Rule: Real workload readiness and squeeze failure are observed, not simulated
    The driver MUST report a real workload's readiness from the cluster's own
    status, and MUST report a workload terminated for exceeding its memory
    limit as out-of-memory rather than as a generic failure.

    Scenario: A real workload becomes ready
      Given a sealed sandbox on a real cluster
      When a bundle whose workload starts successfully is applied
      Then the driver reports the workload ready from the cluster's status

    Scenario: A workload killed for exceeding its memory limit is reported as out-of-memory
      Given a sealed sandbox on a real cluster
      When a workload is applied that exceeds its memory limit under the squeezed capacity transform
      Then the driver reports that workload out-of-memory
      And the sandbox surfaces capacity-squeeze-incompatible diagnostics

  Rule: The substrate is read from the live cluster
    The driver MUST report the cluster's Kubernetes minor version and its
    installed substrate components from the live cluster, so that a substrate
    digest computed against a real cluster reflects what is actually installed.

    Scenario: The substrate digest reflects the live cluster
      Given a real cluster with a known Kubernetes minor version and installed operators
      When the application substrate is inspected through the driver
      Then the reported minor version and components match the live cluster
      And the substrate digest is computed from those live values

  Rule: The simulation arrangement surface is absent under the kube driver
    A control plane running the kube driver MUST NOT register the sim driver's
    test-arrangement routes. Requests to them MUST NOT be served.

    Scenario: Simulation arrangement routes are not served on a real cluster
      Given cloudboxd running against a real cluster
      When a simulation arrangement route is requested
      Then the request is not served

  Rule: Egress evaluation reflects live cluster state
    The driver MUST evaluate a connection attempt against what the cluster
    actually enforces for that namespace — its seal and its live allowlist —
    and MUST NOT report an attempt it did not evaluate as allowed. An
    unevaluated default of "allowed, unfiltered" is a false containment claim.

    Scenario: An allowlisted destination evaluates as proxy-mediated
      Given a sealed sandbox on a real cluster whose allowlist declares one external endpoint
      When the driver evaluates an attempt to that declared endpoint
      Then the attempt is reported allowed through the egress proxy

    Scenario: An undeclared destination evaluates as denied
      Given a sealed sandbox on a real cluster
      And "api.other-vendor.com" is not on the application allowlist
      When the driver evaluates an attempt to "api.other-vendor.com"
      Then the attempt is reported denied

    Scenario: An unsealed namespace is never reported as filtered
      Given a namespace on a real cluster that carries no seal
      When the driver evaluates an attempt from that namespace
      Then the attempt is reported unfiltered rather than proxy-mediated

  Rule: Attempt collection does not depend on a sandbox being inspected
    The control plane MUST collect the egress proxy's attempt records
    independently of whether anyone reads the sandbox, so that a proxy which
    restarts before the first inspection does not take uncollected records with
    it.

    Scenario: A blocked attempt survives a proxy restart before any inspection
      Given a sealed sandbox on a real cluster running a workload
      And the workload has attempted a destination outside the allowlist
      When the egress proxy restarts before the sandbox is inspected
      Then the blocked attempt is still recorded with its destination and attribution

  Rule: The proxy's attempt record is bounded and its truncation is reported
    The egress proxy MUST bound how many attempts it retains, so that a
    long-running sandbox cannot exhaust its memory and lose every record. When
    the bound discards attempts, the number discarded MUST be reported; a
    silently truncated record presented as a complete one is a false count.

    Scenario: Attempts beyond the retention bound are discarded with a reported count
      Given a sealed sandbox on a real cluster whose workload makes more blocked attempts than the proxy retains
      When the control plane collects the proxy's records
      Then the retained attempts are recorded
      And the number of discarded attempts is reported

  Rule: Unrecoverable record loss is surfaced, never absorbed
    When attempt records are lost — the proxy restarted with records
    uncollected, or its bound discarded them — the sandbox status and the run's
    evidence MUST state that the egress record is incomplete. A reduced
    violation count MUST NOT be presented as a complete one.

    Scenario: A sandbox whose records were lost reports an incomplete egress record
      Given a sealed sandbox on a real cluster whose egress proxy lost uncollected records
      When the sandbox status is inspected
      Then the egress record is reported incomplete

    Scenario: Evidence carries the incompleteness rather than a diminished count
      Given a sandbox run whose egress record is incomplete
      When the control plane assembles the run's evidence
      Then the evidence states that the egress-violation count is incomplete

  Rule: The proxy's attempt surface is reachable only by the control plane
    The egress proxy's attempt surface MUST NOT be readable from outside the
    control plane's collection path. A workload elsewhere on the cluster MUST
    NOT be able to read which destinations a sandbox attempted.

    Scenario: A pod outside the sandbox cannot read its attempt records
      Given a sealed sandbox on a real cluster with recorded egress attempts
      When a workload in an unsealed namespace requests that sandbox's attempt surface
      Then the request is refused
      And the control plane's own collection still returns the records

  Rule: An egress proxy that cannot be pulled leaves allowlisted egress denied on a real cluster
    When a sealed namespace's egress proxy cannot be pulled, the driver MUST
    let the control plane report it unavailable, naming the image, within the
    proxy's progress deadline, and a real workload's request to an allowlisted
    destination MUST fail rather than leave the namespace unmediated.

    Scenario: An unpullable proxy image is reported and allowlisted egress fails closed
      Given a sealed sandbox on a real cluster whose allowlist declares one external endpoint
      When its egress proxy is redeployed from an image the cluster cannot pull
      Then the sandbox reports the egress proxy unavailable naming that image
      And the sandbox is not reported as enforcing its allowlist
      And a workload's request to the declared endpoint fails
```
