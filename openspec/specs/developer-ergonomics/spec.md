# Developer ergonomics

The seal is only viable if living inside it beats docker-compose — these are
product requirements, not conveniences. Covers draft requirements X1, X2 and
the v1 command surface (§6.8). The offline `check` command is specified in the
bundle-intake capability; the witnessed `test` command is specified in the
evidence capability.

Scope note (v1): per-application policy disabling `exec` into agent-owned
sandboxes (X1 MAY) is recorded in design.md as a policy option.

Every refusal names a stable reason from a closed catalog (ADR 0012). Reasons
are coarse on purpose: each one groups refusals by what the caller should do
next, and the prose message and, for `not-found`, the subject carry the
detail. The catalog table under "The command line lists the refusal-reason
catalog" is the record of every reason ever published. Removing a row from it
is a breaking change that a later delta has to make explicitly.

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

  Rule: Every refusal names a stable reason from the published catalog
    Every refusal the product returns MUST carry a reason: a short code from
    the refusal-reason catalog, carried beside the human message in the
    machine-readable rendering. The reason MUST depend only on what was
    refused and why, never on the wording of the message. A failure the
    product did not anticipate MUST carry the internal-error reason rather
    than none.

    Scenario Outline: A refusal carries the reason for what the caller met
      Given <situation>
      When <request>
      Then the request is refused
      And the refusal's reason is "<reason>"
      And the reason is one the refusal-reason catalog lists

      Examples:
        | situation                                         | request                                   | reason               |
        | a sandbox owned by developer Priya                | developer Minh destroys it                | not-owner            |
        | a control plane with no registered sandbox host   | an agent creates a sandbox                | no-sandbox-host      |
        | a cluster whose CNI does not enforce NetworkPolicy | an operator registers it as a sandbox host | enforcement-unproven |
        | a control plane with no source-control integration | an agent posts an evidence check          | integration-missing  |
        | a bundle with a manifest outside its contract     | an agent applies it to a sandbox          | bundle-rejected      |

    @conformance
    Scenario: A real cluster that cannot show enforcement is refused for that reason
      Given a cluster whose CNI does not enforce NetworkPolicy
      When an operator registers it as a sandbox host
      Then the request is refused
      And the refusal's reason is "enforcement-unproven"
      And the reason is one the refusal-reason catalog lists

    Scenario: The offline check names the same reason as the control plane
      Given a bundle with a manifest outside its contract
      When an agent checks it offline in the machine-readable rendering
      Then the request is refused
      And the refusal's reason is "bundle-rejected"

  Rule: A not-found refusal names the kind of thing that was not found
    A refusal whose reason is not-found MUST also name its subject, the kind
    of thing that does not exist, so a program can tell which of its
    references was wrong without reading the quoted name in the message.

    Scenario Outline: A missing object is named by its kind
      Given no <kind> named "ghost" exists
      When an agent <request> "ghost" in the machine-readable rendering
      Then the request is refused
      And the refusal's reason is "not-found"
      And the refusal's subject is "<kind>"

      Examples:
        | kind        | request                          |
        | sandbox     | reads the status of the sandbox  |
        | application | shows the application            |

  Rule: The command line lists the refusal-reason catalog
    The command line MUST list every reason in the refusal-reason catalog
    with a one-line summary, in both renderings, without reaching a control
    plane. The catalog is append-only: a reason once published MUST NOT be
    removed, renamed, or given a different meaning, and the command line
    MUST NOT list a reason this catalog does not record.

    Scenario: The catalog is listed without a control plane
      Given no control plane is reachable
      When the refusal-reason catalog is requested in the machine-readable rendering
      Then the command succeeds
      And each listed reason carries its code and a one-line summary

    Scenario Outline: Every recorded reason is listed by the command line
      Given the refusal-reason catalog
      When it is listed from the command line
      Then the listing includes the reason "<reason>"

      Examples:
        | reason               |
        | not-found            |
        | already-exists       |
        | invalid-request      |
        | out-of-contract      |
        | bundle-rejected      |
        | quota-exceeded       |
        | not-owner            |
        | not-permitted        |
        | seal-not-weakened    |
        | evidence-not-postable |
        | unwitnessed-results  |
        | integration-missing  |
        | enforcement-unproven |
        | no-sandbox-host      |
        | not-sealed           |
        | wrong-state          |
        | prerequisite-missing |
        | promotion-blocked    |
        | audit-unavailable    |
        | internal-error       |

    Scenario: The command line lists no reason the catalog does not record
      Given the refusal-reason catalog
      When it is listed from the command line
      Then every listed reason is one this specification records
```
