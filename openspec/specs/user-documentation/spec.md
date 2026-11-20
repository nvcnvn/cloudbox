# User documentation

The release publishes documentation, and documentation is a set of claims
about the product. This capability specifies what the documentation set must
contain and, more importantly, how it is kept true. Where a page restates
something the product itself publishes — its commands, its exit statuses, its
containment statement — the page is checked against the product rather than
reviewed by eye. A page that contradicts the binary next to it is a defect of
the same kind as a binary that contradicts its spec.

The quickstart and the README are specified by `release-integrity` in
`add-local-agent-mvp`. This capability links them into the documentation set
and does not respecify their content.

```gherkin
Feature: User documentation

  Rule: The documentation has one front door that reaches every page
    The repository README MUST link the documentation index, every page in
    the documentation set MUST be reachable from that index, and every link
    between documentation pages MUST resolve. An unreachable page is
    unpublished, and a broken link is a claim that leads nowhere.

    Scenario: Every documentation page is reachable from the README
      Given the published documentation set
      When the links are followed from the repository README
      Then every page in the documentation set is reached

    Scenario: No link between documentation pages is broken
      Given the published documentation set
      When every link between documentation pages is followed
      Then each link resolves to an existing page and, where it names one, an existing heading

  Rule: Product terms are defined in the concepts page
    The concepts page MUST define, in user language, every term in the
    product's business glossary, and MUST describe the four adoption levels
    while presenting only the sandbox level as available in this release.

    Scenario: Every business glossary term is defined in the concepts page
      Given the product's business glossary
      When the concepts page is reviewed
      Then every glossary term has a definition there

    Scenario: The concepts page presents only the sandbox level as available
      Given the concepts page
      When an operator reads its account of the adoption levels
      Then each of the four adoption levels is described
      And only the sandbox level is presented as available in this release

  Rule: The limitations page states what this release does not do
    The documentation MUST include a limitations page that states the
    release's absent trust boundary, its evidence retention policy, its lack
    of durable control-plane state, and the operations it refuses, so an
    operator learns them before relying on the release.

    Scenario: The limitations page names the absent trust boundary
      Given the limitations page
      When an operator reviews it
      Then it states that the control plane does not authenticate the identity a request acts as
      And it does not present the control plane as a trust boundary between users

    Scenario: The limitations page states the evidence retention policy
      Given the limitations page
      When an operator reviews it
      Then it states that evidence is lost when the local control plane stops
      And it states that registered clusters and applications do not survive a control-plane restart

    Scenario: The limitations page names the operations this release refuses
      Given the limitations page
      When an operator reviews it
      Then evidence checks, promotion, and setting production state are named as refused in this release

  Rule: The cluster setup guide takes an operator from no cluster to a registered host
    The cluster setup guide MUST name a NetworkPolicy-enforcing CNI as a
    requirement, MUST point at a bootstrap script the project supports outside
    its test fixtures, and MUST explain why a cluster can be refused at
    registration and what the operator can change to have it accepted.

    Scenario: The setup guide names a supported bootstrap script that exists
      Given the cluster setup guide
      When the bootstrap script it names is located in the repository
      Then the script exists
      And it is not under the conformance test fixtures

    Scenario: The setup guide explains the refusal of a non-enforcing cluster
      Given the cluster setup guide
      When an operator reads how registration can fail
      Then it states that a cluster which cannot be shown to enforce NetworkPolicy is refused
      And it names what the operator can change to have the cluster accepted

  Rule: The CLI reference matches the command line in both directions
    The CLI reference MUST document every command the command line lists,
    MUST NOT document a command the command line does not recognise, and MUST
    state the exit status of each outcome as the command line actually returns
    it.

    Scenario: Every command the command line lists is documented
      Given the commands the command line lists in its usage
      When the CLI reference is reviewed
      Then each listed command has a reference entry

    Scenario: The CLI reference documents no command the command line lacks
      Given the commands documented in the CLI reference
      When each is requested for its help from the command line
      Then the command line recognises every documented command

    Scenario: The documented exit statuses are the ones the command line returns
      Given the exit statuses the CLI reference documents
      When a successful command, a refused command, and a command against an unreachable control plane are run
      Then each exit status is the one the reference documents for that outcome

  Rule: The agent guide walks the whole sealed loop with commands that exist
    The agent guide MUST cover every stage of the sealed-iteration loop, MUST
    show only commands and options the command line recognises, and MUST show
    blocked egress and the allowlist proposal in the machine-readable
    rendering an agent consumes.

    Scenario: The agent guide covers every stage of the sealed loop
      Given the agent guide
      When its sections are reviewed
      Then it covers declaring, creating, applying, testing, explaining, amending, reading evidence, and destroying

    Scenario: Every command in the agent guide is recognised by the command line
      Given the commands shown in the agent guide's examples
      When each is checked against the command line
      Then the command line recognises each command and each of its options

    Scenario: The agent guide shows blocked egress as data
      Given the agent guide
      When its explain step is reviewed
      Then it shows a blocked attempt and the allowlist proposal in the machine-readable rendering

  Rule: The containment page agrees with the product's containment statement
    The containment page MUST carry every blocked channel, residual channel,
    and recording limit that the control plane's published containment
    statement carries, MUST separate the cooperative guarantee from the
    adversarial one, and MUST NOT claim the seal is unbypassable.

    Scenario: The containment page carries everything the statement carries
      Given the containment statement the control plane publishes
      When the containment page is reviewed
      Then every blocked channel, residual channel, and recording limit in the statement appears on the page

    Scenario: The containment page claims nothing stronger than the statement
      Given the containment page
      When an operator reviews it
      Then the cooperative guarantee is stated separately from the adversarial one
      And the word "unbypassable" appears nowhere on the page

  Rule: Every outcome the release names has a troubleshooting entry
    The troubleshooting guide MUST hold one entry for each refusal reason the
    command line lists, keyed by the reason's code, and one entry for each
    other outcome the release names to an operator. Each entry MUST state
    what the outcome means and what the operator can do about it. The guide
    MUST NOT key an entry on a reason the command line does not list. An
    outcome without an entry is one the operator meets for the first time in
    production.

    Scenario: Every refusal reason has an entry with its meaning and remedy
      Given the troubleshooting guide
      And the refusal-reason catalog the command line lists
      When the entry for each listed reason is looked up by its code
      Then every listed reason has an entry
      And each entry states what the outcome means
      And each entry states what the operator can do about it

    Scenario: The guide keys no entry on a reason the product does not list
      Given the reason codes the troubleshooting guide keys its entries on
      When each is compared with the refusal-reason catalog the command line lists
      Then the command line lists every one of them

    Scenario Outline: A named outcome that is not a refusal has an entry
      Given the troubleshooting guide
      When the entry for <outcome> is looked up
      Then the entry states what the outcome means
      And the entry states what the operator can do about it

      Examples:
        | outcome                              |
        | an egress proxy reported unavailable |
        | substrate parity reported unverified |
        | an unreachable control plane         |
```
