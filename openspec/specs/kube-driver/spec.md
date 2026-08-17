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
    them, the enforcement probe MUST fail, the sandbox MUST NOT be reported
    sealed, and no evidence MUST be emitted for it. The control plane MUST
    report the cause as unenforced network policy rather than a generic
    failure.

    Scenario: A cluster that accepts but ignores NetworkPolicy is caught
      Given a real cluster whose CNI accepts NetworkPolicy objects without enforcing them
      When a sandbox is created on that cluster
      Then the enforcement probe fails
      And the sandbox is not reported sealed
      And no evidence is emitted for that sandbox
      And the reported cause names unenforced network policy

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
```
