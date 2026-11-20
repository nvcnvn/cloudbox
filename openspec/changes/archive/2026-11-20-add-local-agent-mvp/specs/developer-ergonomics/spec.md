# Developer ergonomics changes

At the sandbox adoption level there is no pull request, so the command line is
not a convenience over the SCM flow — it is the entire product. The existing
capability already says as much one rung up ("the evidence-check level needs no
promotion verbs"); this delta says it for the rung below, where there is no SCM
integration to carry any part of the loop.

The second rule follows from who is running the loop. An AI coding agent is a
program, and the explain-and-propose mechanism this capability already
specifies — every blocked attempt with destination, timestamp, and attributed
workload, plus a ready-to-submit allowlist proposal — is only usable by a
program if it arrives as data. Nothing about what is rendered changes; what
changes is that a machine-readable rendering of the same facts exists, and that
outcomes are distinguishable without reading the text.

```gherkin
Feature: Developer ergonomics changes

  # @openspec: ADDED
  Rule: At the sandbox level the command line carries the whole loop
    Where no source-control integration is configured, every step of the
    sealed-iteration loop MUST be reachable from the command line: declaring
    an application and its contract, creating a sandbox, applying a bundle,
    running the declared suite, reading status and evidence, and destroying
    the sandbox. No step MAY require a caller to address the control plane's
    interface directly.

    Scenario: An agent completes the sealed loop using only the command line
      Given a registered sandbox cluster and no source-control integration
      When an agent declares an application, creates a sandbox, applies a bundle, runs the declared suite, reads the evidence, and destroys the sandbox
      Then every step is performed through the command line
      And no step requires addressing the control plane's interface directly

  # @openspec: ADDED
  Rule: Command output is available as data and outcomes carry distinct exit statuses
    Every command MUST be able to render its result in a machine-readable form
    carrying the same facts as its human rendering, and MUST distinguish by
    exit status between success, a request the product refused, and a failure
    to reach the control plane. A program MUST NOT have to parse prose to
    learn what happened.

    Scenario: Explain renders blocked attempts and the proposal as data
      Given a sandbox whose workload was denied egress to two undeclared endpoints
      When status with the explain option is run in machine-readable mode
      Then each blocked attempt is rendered as data with destination, timestamp, and attributed workload
      And the allowlist change proposal is rendered as data alongside them

    Scenario: The machine-readable rendering carries the same facts as the human one
      Given a sandbox with a recorded blocked egress attempt
      When status with the explain option is run in both renderings
      Then the machine-readable rendering carries the same facts as the human rendering

    Scenario: A refusal and an unreachable control plane are distinguishable without reading text
      Given a command that the control plane refuses and a command issued against an unreachable control plane
      When each is run
      Then their exit statuses differ from each other and from a successful run
```
