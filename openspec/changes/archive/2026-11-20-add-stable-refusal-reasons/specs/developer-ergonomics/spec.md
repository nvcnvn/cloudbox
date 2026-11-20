# Developer ergonomics changes

A refusal tells a program *that* it was refused, through exit status `1`, but
not *which* refusal it met. The only thing that tells refusals apart is a
sentence of prose, and the existing rule says a program MUST NOT have to parse
prose to learn what happened. This delta gives every refusal a stable,
machine-readable reason from a closed catalog. The command line publishes that
catalog, so a program and the documentation can both learn the set from the
binary.

Reasons are coarse on purpose. Each one groups refusals by what the caller
should do next. The prose message and, for `not-found`, the subject carry the
detail. The catalog below is the record of every reason ever published.
Removing a row from it is a breaking change that a later delta has to make
explicitly.

```gherkin
Feature: Developer ergonomics changes

  # @openspec: ADDED
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

  # @openspec: ADDED
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

  # @openspec: ADDED
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
