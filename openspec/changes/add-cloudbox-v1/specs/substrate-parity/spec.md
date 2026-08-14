# Substrate parity

The substrate is everything a bundle assumes exists: Kubernetes version, CRDs,
operators, admission policies, gateway/storage/priority classes. Parity between
sandbox and production is a hash-checked fact, scoped to what the application's
bundles actually reference. Covers draft requirements P1–P4.

Scope note: the draft grades lockfile-driven provisioning (P3) SHOULD; it is
carried here as a v1 requirement — descoping it requires a spec change (see
design.md, Decisions).

```gherkin
Feature: Substrate parity

  # @openspec: ADDED
  Rule: Each application has a substrate lockfile scoped to what it references
    The control plane MUST maintain a per-application lockfile — Kubernetes
    minor version, the CRDs and operator releases the bundles instantiate,
    applicable admission configurations, and the named classes — with a
    digest over that set. Cloud identity bindings and secret values are
    recorded declared-not-verified.

    Scenario: The lockfile captures only components the application references
      Given an application whose bundles instantiate the cert-manager CRDs and name one storage class
      When the control plane maintains its substrate lockfile
      Then the lockfile records the Kubernetes minor version, those CRDs and their operator releases, applicable admission configurations, and that storage class
      And a digest over that set identifies the lockfile

    Scenario: An upgrade to an unreferenced operator leaves the digest unchanged
      Given an operator installed on the cluster that the application's bundles never reference
      When that operator is upgraded
      Then the application's substrate digest is unchanged
      And the application's in-flight evidence and soak time remain valid

    Scenario: Identity bindings and secret values are declared, not verified
      Given an application whose workloads use an IAM role binding and named secrets
      When the substrate lockfile is maintained
      Then the identity bindings and secret values are recorded as declared-not-verified

  # @openspec: ADDED
  Rule: Evidence carries the substrate digest and is invalid on mismatch
    Evidence MUST include the sandbox's substrate digest and MUST be marked
    invalid when it mismatches production's digest; any admin override MUST
    be recorded in the audit log.

    Scenario: A substrate mismatch invalidates the run's evidence
      Given a sandbox whose substrate digest differs from production's
      When the run's evidence is evaluated
      Then the evidence is marked invalid for substrate mismatch

    Scenario: An admin override of a mismatch is audited
      Given evidence marked invalid for substrate mismatch
      When an admin overrides the invalidation
      Then the override and the admin are recorded in the audit log

  # @openspec: ADDED
  Rule: Sandbox substrates are provisioned from the lockfile
    The control plane provisions sandbox substrates from the lockfile —
    shared-cluster operators, prebaked local images. Verification is the
    contract; provisioning is the convenience that makes it pass.

    Scenario: A shared-cluster sandbox is provisioned to match the lockfile
      Given an application substrate lockfile pinning an operator release
      When a sandbox is created on the shared cluster
      Then the sandbox's substrate provides that operator release
      And the sandbox's substrate digest matches the lockfile

  # @openspec: ADDED
  Rule: Production substrate drift invalidates only referencing applications
    Drift in production MUST be detected and reflected in the lockfile digest
    so stale sandboxes stop producing valid evidence — scoped so only
    applications referencing the drifted component are invalidated.

    Scenario: An operator upgrade invalidates evidence for applications that use it
      Given applications A and B where only A's bundles instantiate operator X's CRDs
      When operator X is upgraded in production
      Then application A's lockfile digest changes and its stale sandboxes stop producing valid evidence
      And application B's lockfile digest and evidence are unaffected
```
