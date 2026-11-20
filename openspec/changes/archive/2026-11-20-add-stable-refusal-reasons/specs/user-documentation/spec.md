# User documentation changes

The troubleshooting rule enumerated its outcomes in an `Examples:` table kept
by hand, because refusals were free text and there was nothing stable to key
an entry on (`add-user-documentation` design D5). Every refusal now names a
reason from a catalog the command line publishes (`developer-ergonomics`).
Coverage of refusals therefore comes from the product: each catalogued reason
has an entry keyed by its code. The table remains only for outcomes that are
not refusals and so have no reason.

```gherkin
Feature: User documentation changes

  # @openspec: MODIFIED
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
