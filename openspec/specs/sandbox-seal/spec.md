# Sandbox seal

The seal is default-deny ingress and egress with an explicit FQDN allowlist,
enforced for the sandbox's entire lifetime. It is the reproducibility verifier
(closed-world proof over exercised behavior) and a strong-but-bounded
containment boundary for adversarial workloads (draft §2.5). Covers draft
requirements N1–N8 (v1 scope).

Scope notes (v1): the adversarial-hardening roadmap in N8 — DNS query logging
with rate limits and anomaly detection, per-FQDN egress volume metering — is
v1.x committed direction, recorded in design.md. Compiling the allowlist to
native policy on DNS-aware CNIs (N3) is a MAY, also recorded in design.md.

```gherkin
Feature: Sandbox seal

  Rule: A sandbox is sealed before any workload is admitted
    Default-deny ingress and egress MUST be in force before the first user
    workload is admitted.

    Scenario: Workloads are only admitted into an already-sealed sandbox
      Given a sandbox being provisioned
      When the developer applies a bundle before the seal is in force
      Then no workload is admitted until default-deny ingress and egress are active

  Rule: Egress is limited to in-sandbox services, cluster DNS, and the allowlist
    Egress MUST be limited to in-sandbox services, cluster DNS, linked
    dependency endpoints, and FQDN entries on the application's declared
    allowlist.

    Scenario: An allowlisted external endpoint is reachable
      Given the application allowlist declares "api.stripe.com"
      When a workload connects to "api.stripe.com"
      Then the connection succeeds

    Scenario: An undeclared external endpoint is denied
      Given "api.other-vendor.com" is not on the application allowlist
      When a workload attempts to connect to "api.other-vendor.com"
      Then the connection is denied

    Scenario: In-sandbox services and cluster DNS remain reachable
      Given a sealed sandbox running two services
      When one service calls the other by its short name
      Then name resolution and the in-sandbox connection succeed

  Rule: The seal floor is standard NetworkPolicy with a product egress proxy
    Enforcement MUST require only standard Kubernetes NetworkPolicy v1 — no
    vendor policy CRDs, no specific CNI. The only admitted egress is cluster
    DNS and the product-managed egress proxy, which enforces the FQDN
    allowlist; non-HTTP protocols and proxy-unaware workloads are sealed via
    transparent pod-level redirection with no workload modification.

    Scenario: Sealing works on a cluster with only standard NetworkPolicy
      Given a cluster whose CNI enforces standard NetworkPolicy v1 and has no vendor policy CRDs
      When a sandbox is created
      Then the sandbox seals successfully
      And the only egress admitted by policy is cluster DNS and the egress proxy

    Scenario: A raw TCP database connection is sealed without modifying the workload
      Given a workload speaking a database wire protocol that ignores proxy environment variables
      When the workload connects to an allowlisted database endpoint
      Then the connection is transparently redirected through the egress proxy
      And the workload required no modification

  Rule: Every blocked egress attempt is recorded and attributed
    Blocked attempts MUST be recorded with destination and timestamp,
    attributed to the sandbox and workload, and surfaced in status and in
    evidence.

    Scenario: A denied connection shows up in status and evidence
      Given a workload attempts to reach an undeclared endpoint
      When the connection is denied
      Then the attempt is recorded with destination, timestamp, and the attributed workload
      And the record appears in the sandbox status and in the run's evidence

  Rule: The seal is never weakened per sandbox
    Allowlist changes MUST be application-policy changes, owned by admins and
    audited; no sandbox-scoped exception exists.

    Scenario: A sandbox owner cannot widen their own allowlist
      Given a developer who owns a sandbox
      When they attempt to add an FQDN to the allowlist for that sandbox only
      Then the request is refused
      And the path offered is an audited application-policy change for admin review

  Rule: User NetworkPolicy narrows but never widens the seal
    NetworkPolicy authored inside a bundle MAY further restrict the seal and
    MUST NOT widen it.

    Scenario: A bundle policy permitting extra egress does not widen the seal
      Given a bundle containing a NetworkPolicy permitting egress to an undeclared endpoint
      When the bundle runs in the sealed sandbox
      Then connections to the undeclared endpoint are still denied

  Rule: Seal enforcement is probe-verified, never assumed
    A CNI that ignores NetworkPolicy fails silently, so setup and sandbox
    creation MUST probe-verify that a denied connection is actually denied. A
    sandbox on a non-enforcing cluster MUST NOT report itself sealed and MUST
    NOT produce evidence.

    Scenario: A non-enforcing cluster is caught by the probe
      Given a cluster whose CNI accepts but does not enforce NetworkPolicy
      When setup or sandbox creation runs the enforcement probe
      Then the probe detects that a denied connection was not denied
      And the sandbox does not report itself sealed
      And the sandbox produces no evidence

    Scenario: An enforcing cluster passes the probe
      Given a cluster whose CNI enforces NetworkPolicy
      When the enforcement probe creates a canary workload and attempts a denied connection
      Then the connection is denied and the seal is reported verified

  Rule: Containment claims match the declared threat-model scope
    Published containment guarantees MUST separate the cooperative guarantee
    from the adversarial one, name the residual channels (DNS tunneling,
    exfiltration through allowlisted endpoints), and MUST NOT claim the seal
    is unbypassable.

    Scenario: The containment statement declares blocked and residual channels
      Given the product's published containment statement for sealed sandboxes
      When an operator reviews it
      Then direct egress, non-allowlisted FQDNs, ingress, and production writes are listed as blocked
      And DNS tunneling and exfiltration through allowlisted endpoints are listed as residual channels
      And the word "unbypassable" appears nowhere in the claim
```
