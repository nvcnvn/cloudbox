# Conformance CI

This capability defines the contract around the real-cluster run: which
scenarios it covers, what it refuses to claim, and how the simulation is kept
honest against it. Nothing in this repository currently runs in continuous
integration, so the CI contract is specified here rather than left to a
configuration file nobody reads.

The governing principle is that a conformance run must be either meaningful or
loudly broken, never quietly vacuous. A cluster whose CNI ignores NetworkPolicy
would let every seal scenario pass while proving nothing, so a verified
enforcement precondition gates the run. The same principle sets the subset's
boundary: a scenario is included when a real cluster can prove something the
simulation cannot, and excluded — explicitly, with the reason recorded — when it
cannot.

Scenarios tagged `@conformance` require a real cluster. The untagged scenarios
here describe the default run and the CI configuration, and therefore execute in
the ordinary sim suite.

```gherkin
Feature: Conformance CI

  Rule: The default acceptance run excludes conformance-tagged scenarios
    The default acceptance run MUST execute the simulation suite and MUST NOT
    attempt any scenario tagged for the real-cluster run. A developer with no
    cluster available MUST be able to run the default suite to completion, and
    the run MUST NOT report those scenarios as failures.

    Scenario: The default run skips the real-cluster scenarios
      Given a working copy with no Kubernetes cluster available
      When the default acceptance run is invoked
      Then the run completes without attempting any conformance-tagged scenario
      And no conformance-tagged scenario is reported as a failure

    Scenario: The conformance run selects only the tagged subset
      Given the acceptance suite with its conformance-tagged scenarios
      When the conformance run is invoked
      Then only conformance-tagged scenarios are selected

  Rule: A conformance run gates on proven NetworkPolicy enforcement
    Before reporting any conformance result, the run MUST confirm that the
    target cluster actually enforces NetworkPolicy. If enforcement cannot be
    proven, the run MUST fail and name unproven enforcement as the cause; it
    MUST NOT report a pass.

    Scenario: A cluster with an enforcing CNI satisfies the precondition
      Given a real cluster whose CNI enforces NetworkPolicy
      When the conformance run checks its enforcement precondition
      Then the precondition passes and the run proceeds

    Scenario: A cluster that cannot prove enforcement fails the run
      Given a real cluster whose CNI does not enforce NetworkPolicy
      When the conformance run checks its enforcement precondition
      Then the run fails naming unproven network policy enforcement
      And the run reports no conformance pass

  Rule: Conformance lifecycle expiry uses the real clock
    Scenarios in the conformance subset that depend on elapsed time MUST use
    the real clock with short configured durations. The conformance run MUST
    NOT advance a simulated clock; a result obtained by moving a fake clock
    MUST NOT be reported as conformance.

    @conformance
    Scenario: A short time-to-live expires a real sandbox in real time
      Given a sandbox on a real cluster with a time-to-live of a few seconds
      When that time-to-live elapses in real time
      Then the sandbox is destroyed
      And its namespace is removed from the cluster

    @conformance
    Scenario: A short idle window expires an inactive real sandbox in real time
      Given a sandbox on a real cluster with an idle-expiry window of a few seconds
      When the sandbox sees no activity for longer than that window
      Then the sandbox is destroyed

  Rule: Scenarios the real run cannot honestly prove are excluded and recorded
    A scenario MUST be excluded from the conformance subset when a real cluster
    cannot prove more than the simulation does, and the reason MUST be recorded
    where the subset is defined. Soak-window scenarios, whose windows are
    specified in hours, MUST be excluded. Scenarios arranged only through
    simulated external systems — source control, GitOps sync, audit sinks,
    production state, and datastore seeding — MUST be excluded.

    Scenario: Soak-window scenarios are absent from the conformance subset
      Given the defined conformance subset
      When its scenarios are listed
      Then no soak-window scenario is present
      And the exclusion reason is recorded with the subset definition

    Scenario: Scenarios arranged only through simulated external systems are absent
      Given the defined conformance subset
      When its scenarios are listed
      Then no scenario arranged only through a simulated external system is present

  Rule: Continuous integration runs the full effective suite on every change
    Every proposed change MUST be checked in continuous integration by
    compiling both binaries, vetting the sources, linting the extracted
    specifications, running the full simulation suite, and running the
    conformance subset against a real cluster with an enforcing CNI. A failure
    in any of these MUST fail the check.

    Scenario: The integration check covers every required stage
      Given the continuous-integration configuration
      When its required stages are read
      Then it builds both binaries, vets the sources, lints the extracted specifications, runs the simulation suite, and runs the conformance subset

    Scenario: A failing conformance subset fails the check
      Given a continuous-integration run whose conformance subset fails
      When the check result is reported
      Then the check fails

  Rule: A divergence between the simulation and the real driver is recorded and reconciled
    Where the kube driver's observed behaviour contradicts the simulation, the
    simulation MUST be corrected to match the real driver, and the divergence
    MUST be recorded with the behaviour that differed. An uncorrected
    divergence MUST NOT be left silent, because the simulation's fidelity
    bounds what the whole suite can prove.

    Scenario: An observed divergence is recorded against the simulation
      Given a behaviour where the real driver contradicts the simulation
      When that divergence is reconciled
      Then the simulation is corrected to match the real driver
      And the divergence is recorded with the behaviour that differed
```
