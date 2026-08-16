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
```
