# Kube driver changes

`sandbox-lifecycle` now refuses a sandbox host at registration when it cannot
be shown to enforce NetworkPolicy, and a sandbox may only land on a registered
host, including when an application pins its sandboxes to a named cluster.
On real infrastructure, then, a non-enforcing cluster never gets as far as a
sandbox. The probe that catches it runs at registration, not at sandbox
creation.

The rule below keeps its name and its guarantee: a non-enforcing CNI still
fails the probe, still never yields a sealed sandbox, and the cause is still
named. Its scenario moves to where a real cluster now meets the probe. A
cluster that stops enforcing after it was registered (a CNI replaced
underneath the control plane) is still caught by the sandbox-creation probe
(`sandbox-seal`, N7). That path is proven in the simulation, which can change
a cluster's enforcement after registration. A real CI cluster cannot do that
without reinstalling its CNI mid-run, and the divergence is recorded in
`internal/sim/DIVERGENCES.md`.

The second rule proves on real infrastructure what `release-integrity` states
for every driver: an egress proxy that cannot be pulled is reported, and the
allowlist it would have enforced fails closed. The simulation models the pull
failure; only a real cluster shows the Deployment's progress deadline expiring
and a real workload's request finding no proxy to reach.

```gherkin
@conformance
Feature: Kube driver changes

  # @openspec: MODIFIED
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

  # @openspec: ADDED
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
