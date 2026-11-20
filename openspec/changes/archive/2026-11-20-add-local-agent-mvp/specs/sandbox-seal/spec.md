# Sandbox seal changes

The containment rule already forces the published claim to separate the
cooperative guarantee from the adversarial one and to name its residual
channels. A public release adds a limit that rule does not currently reach: not
what the seal blocks, but what it can *tell you* it blocked.

In the released mechanism only proxy-aware HTTP egress reaches the product's
proxy and is recorded by destination; everything else is denied by the CNI
before any destination is observed (recorded as divergence 4 in the sim
divergence record). Containment is stronger for that traffic and observability
is weaker, and an operator who assumes every denial appears in the blocked-
attempt record will draw a false conclusion from a short list. The statement
must say which denials it can name.

The rule below is the existing one with that obligation added; its original
scenario is unchanged.

```gherkin
Feature: Sandbox seal changes

  # @openspec: MODIFIED
  Rule: Containment claims match the declared threat-model scope
    Published containment guarantees MUST separate the cooperative guarantee
    from the adversarial one, name the residual channels (DNS tunneling,
    exfiltration through allowlisted endpoints), and MUST NOT claim the seal
    is unbypassable. They MUST also name which egress the mechanism records by
    destination and which it denies without observing one, so that the
    blocked-attempt record is not read as a complete account of everything the
    seal stopped.

    Scenario: The containment statement declares blocked and residual channels
      Given the product's published containment statement for sealed sandboxes
      When an operator reviews it
      Then direct egress, non-allowlisted FQDNs, ingress, and production writes are listed as blocked
      And DNS tunneling and exfiltration through allowlisted endpoints are listed as residual channels
      And the word "unbypassable" appears nowhere in the claim

    Scenario: The containment statement declares the limit of destination recording
      Given the product's published containment statement for sealed sandboxes
      When an operator reviews it
      Then egress that reaches the product's proxy is described as recorded by destination
      And egress that does not reach the proxy is described as denied without a recorded destination
      And the blocked-attempt record is not presented as a complete account of denied egress
```
