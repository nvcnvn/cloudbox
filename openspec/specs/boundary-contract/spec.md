# Boundary contract

The boundary contract is the declared, finite set of values allowed to differ
per environment — and nothing else varies. Growth of this list is the design's
failure signal: the correct response to "I need to template one more field" is
a spec change, not an overlay mechanism. Covers draft requirements C1–C3 plus
the v1 stub alternative for inter-application dependencies (S8).

Scope note (v1): linked dependency sandboxes with parity-verified digests (S8)
are v1.x committed direction, recorded in design.md; v1 satisfies declared
dependencies with stubs.

```gherkin
Feature: Boundary contract

  Rule: Environment variance is limited to four declared kinds
    Each application MUST declare its complete environment-variant set,
    limited to secret names, ingress hostnames, the egress allowlist, and
    internal application dependencies. Secret values are supplied per
    environment and never live inside bundles; ingress hostnames live on
    gateway listeners, never in bundles.

    Scenario: A complete contract declares exactly the four kinds
      Given an application being configured
      When its boundary contract is declared
      Then it holds secret names, ingress hostnames, an egress allowlist, and internal application dependencies
      And no other kind of environment-variant value is accepted into the contract

  Rule: Apply fails on undeclared or unvalued secrets
    Apply MUST fail when a bundle references a secret not declared in the
    contract, or one lacking a value for the target environment.

    Scenario: A bundle referencing an undeclared secret is rejected
      Given a bundle whose Deployment mounts the secret "payment-api-key"
      And "payment-api-key" is not declared in the boundary contract
      When the bundle is applied
      Then the apply fails naming the undeclared secret

    Scenario: A declared secret with no value for the environment is rejected
      Given the contract declares the secret "payment-api-key"
      And no value is supplied for the sandbox environment
      When the bundle is applied
      Then the apply fails naming the secret missing a value for the target environment

  Rule: No variance mechanism exists outside the contract
    Any other environment variance is out of contract by design: no overlay
    or templating mechanism exists. Capacity and namespace are not templated
    variance either — they are controller-applied transforms recorded in
    evidence.

    Scenario: There is no way to template a fifth kind of value
      Given a team wanting a per-environment log level in their manifests
      When they look for an overlay or templating mechanism in the product
      Then none exists
      And the documented path is changing the product spec, not configuring an overlay

    Scenario: Namespace and capacity differences are transforms, not variance
      Given a bundle admitted to a sandbox with a namespace transform and a squeezed capacity transform
      When the run's evidence is inspected
      Then both transforms are recorded in evidence
      And the bundle bytes and digest are unchanged

  Rule: A declared dependency may be satisfied by a stub recorded as stubbed
    A declared internal application dependency MAY be satisfied by an
    allowlisted stub endpoint — a contract alias hostname mapped to a
    user-supplied mock or shared dev instance. Evidence MUST record the
    dependency as stubbed and MUST NOT present it as parity-verified.

    Scenario: An alias resolved to a stub is recorded honestly in evidence
      Given an application declaring a dependency on the billing application via the alias "billing.deps.internal"
      And the alias is mapped to a user-supplied mock endpoint on the allowlist
      When a sealed run completes and its evidence is assembled
      Then the dependency status is recorded as stubbed
      And the evidence nowhere presents the dependency as parity-verified
```
