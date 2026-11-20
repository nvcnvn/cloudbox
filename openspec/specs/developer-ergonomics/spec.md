# Developer ergonomics

The seal is only viable if living inside it beats docker-compose — these are
product requirements, not conveniences. Covers draft requirements X1, X2 and
the v1 command surface (§6.8). The offline `check` command is specified in the
bundle-intake capability; the witnessed `test` command is specified in the
evidence capability.

Scope note (v1): per-application policy disabling `exec` into agent-owned
sandboxes (X1 MAY) is recorded in design.md as a policy option.

```gherkin
Feature: Developer ergonomics

  Rule: Sealed sandboxes support logs, exec, and port-forward
    The CLI MUST provide logs, exec, and port-forward into sandbox workloads,
    scoped by RBAC to the sandbox owner and audited. Ingress for port-forward
    traverses the control plane, never a seal exception.

    Scenario: The owner tails logs from a sealed workload
      Given a developer who owns a running sealed sandbox
      When they run the logs command against a workload in it
      Then the workload's logs stream to them
      And the access is audited

    Scenario: Port-forward reaches the workload without piercing the seal
      Given a developer who owns a sealed sandbox running a web service
      When they port-forward to the service and open it locally
      Then the traffic traverses the control plane
      And no ingress exception is added to the seal

    Scenario: A non-owner is denied sealed ergonomics
      Given a sandbox owned by developer Priya
      When developer Minh attempts to exec into one of its workloads
      Then the request is denied as not the sandbox owner

  Rule: Status explains the seal and proposes the allowlist change
    The status command's explain mode MUST render every blocked egress
    attempt with destination, timestamp, and attributed workload, and emit a
    ready-to-submit allowlist change proposal for admin review. The apply
    command's record-egress mode provides the same loop interactively during
    first onboarding.

    Scenario: Explain turns blocked attempts into an allowlist proposal
      Given a sandbox whose workload was denied egress to two undeclared endpoints
      When the developer runs status with the explain option
      Then each blocked attempt is rendered with destination, timestamp, and attributed workload
      And a ready-to-submit allowlist change proposal for those endpoints is emitted for admin review

    Scenario: First onboarding records egress interactively
      Given a developer applying an existing application to a sandbox for the first time
      When they apply with the record-egress option
      Then observed egress attempts are gathered into the same allowlist proposal loop as they occur

  Rule: The evidence-check level needs no promotion verbs
    At the evidence-check level, the SCM integration plus check, apply,
    status, and test MUST be the whole product; the PR flow drives the same
    verbs and adds no separate command surface.

    Scenario: A team at the evidence-check level never touches promotion verbs
      Given an application at the evidence-check adoption level
      When the team works a pull request from open to merge
      Then only the check, apply, status, and test verbs and the SCM integration are involved
      And no promotion, approval, or rejection verb is required

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

  Rule: The command line lists every command it accepts
    The command line's usage MUST list every command it recognises, and MUST
    NOT list a command it does not recognise, so a program or a reader can
    learn the whole surface from the binary without external documentation.

    Scenario: The usage lists every recognised command
      Given the command line
      When its usage is requested
      Then every command the command line recognises is listed

    Scenario: The usage lists no command the command line rejects
      Given the commands the command line lists in its usage
      When each is requested for its help
      Then none is rejected as an unknown command
```
