# Kube driver changes

The driver's egress path is made to report what actually happened. Two defects
motivate this (see proposal.md): the proxy's attempt record can be lost without
anyone noticing, which quietly lowers the egress-violation count in evidence,
and `AttemptEgress` answers "allowed, unfiltered" for every destination without
evaluating anything.

Nothing here restates `sandbox-seal`. N4 already requires blocked attempts to be
recorded, attributed, and surfaced; these Rules constrain the mechanism the kube
driver uses to meet it, so that meeting it does not depend on the proxy staying
up and on someone inspecting the sandbox at the right moment.

The `cluster.Cluster` contract is untouched (ADR 0008). `AttemptEgress` is
satisfiable as written — the driver has everything it needs in the namespace's
live seal state — so it is satisfied rather than renegotiated.

Every scenario requires a real cluster and inherits the capability's
`@conformance` tag.

```gherkin
@conformance
Feature: Kube driver

  # @openspec: ADDED
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

  # @openspec: ADDED
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

  # @openspec: ADDED
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

  # @openspec: ADDED
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

  # @openspec: ADDED
  Rule: The proxy's attempt surface is reachable only by the control plane
    The egress proxy's attempt surface MUST NOT be readable from outside the
    control plane's collection path. A workload elsewhere on the cluster MUST
    NOT be able to read which destinations a sandbox attempted.

    Scenario: A pod outside the sandbox cannot read its attempt records
      Given a sealed sandbox on a real cluster with recorded egress attempts
      When a workload in an unsealed namespace requests that sandbox's attempt surface
      Then the request is refused
      And the control plane's own collection still returns the records
```
