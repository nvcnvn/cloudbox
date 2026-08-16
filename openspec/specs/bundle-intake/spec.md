# Bundle intake

Bundles are the unit of apply, diff, evidence, and promotion: content-addressed
sets of rendered, environment-agnostic manifests. Intake accepts the Kubernetes
YAML teams already have — no product schema — and repairs what it can rather
than reject. Covers draft requirements B1–B5, X3, and the intake half of G7
(render determinism). The `check` CLI command is specified here because it is
an offline copy of this same intake analysis.

Scope notes (v1): native Helm/Kustomize rendering may ship as rendered-only
intake in v1 (B5 MAY — open question 4 in design.md). The vcluster path for
multi-namespace bundles (S4) is post-v1.

```gherkin
Feature: Bundle intake

  Rule: Apply accepts plain multi-document Kubernetes YAML of any kind
    Intake MUST accept built-in and custom resources with no product-specific
    schema, wrapper, or annotations required.

    Scenario: Applying unmodified manifests with built-in and custom resources
      Given a manifest directory containing a Deployment, a Service, and a custom resource
      And none of the manifests carry any CloudBox-specific field or annotation
      When the developer applies the directory to their sandbox
      Then the apply is accepted

  Rule: Every apply produces a content-addressed bundle
    The bundle digest MUST be recorded server-side and MUST be the identity
    used by diff, evidence, and promotion.

    Scenario: Identical manifest sets produce the same bundle digest
      Given two applies of byte-identical rendered manifests
      When both bundles are recorded
      Then both applies report the same bundle digest

    Scenario: The bundle digest is recorded by the control plane
      Given a developer applies a manifest directory to their sandbox
      When the apply completes
      Then the control plane holds a bundle record addressed by its digest

  Rule: A uniform namespace is stripped by a recorded namespace transform
    Bundles MUST be environment-agnostic: a single uniform metadata.namespace
    is stripped at admission and the transform declared in evidence; multiple
    distinct namespaces MUST fail with guidance.

    Scenario: Bundle with one uniform namespace is repaired at admission
      Given every manifest in the bundle declares metadata.namespace "team-a"
      When the bundle is admitted
      Then the namespace is stripped by a namespace transform
      And the transform is declared in the run's evidence
      And the bundle digest is unchanged by the transform

    Scenario: Bundle spanning two namespaces is rejected with guidance
      Given a bundle whose manifests declare namespaces "team-a" and "team-b"
      When the bundle is admitted
      Then the apply fails
      And the failure names the violating manifests and points to the multi-namespace path

  Rule: Cluster-scoped resources are rejected at intake
    Cluster-scoped resources belong to the substrate, not the bundle, and
    MUST be rejected naming the violating manifest and the fix.

    Scenario: Bundle containing a cluster-scoped resource is rejected
      Given a bundle containing a ClusterRole manifest
      When the bundle is admitted
      Then the apply fails
      And the failure names the ClusterRole manifest and states that cluster-scoped resources belong to the substrate

  Rule: Hardcoded cross-namespace references receive best-effort lint guidance
    Intake MUST lint hardcoded cross-namespace and FQDN service references
    with suggested rewrites; the seal, not the lint, is the enforcement of
    last resort.

    Scenario: A hardcoded cross-namespace service reference is flagged with a rewrite
      Given a manifest referencing "http://auth-api.other-team.svc.cluster.local:8080"
      When the bundle is admitted
      Then intake reports the reference with a suggested same-namespace rewrite
      And the apply is not blocked by the lint finding

  Rule: In-bundle service references use short names or declared aliases
    References MUST be same-namespace short names or alias hostnames declared
    in the boundary contract, so an application occupies exactly one namespace
    in every environment.

    Scenario: Same-namespace short-name reference is accepted
      Given a manifest referencing "http://auth-api:8080"
      When the bundle is admitted
      Then the reference passes intake analysis

    Scenario: Declared boundary contract alias is accepted
      Given the application's boundary contract declares the alias hostname "billing.deps.internal"
      And a manifest references "http://billing.deps.internal"
      When the bundle is admitted
      Then the reference passes intake analysis

  Rule: Helm and Kustomize enter as rendered output
    The bundle MUST always be the rendered output; rendering happens before
    intake.

    Scenario: Rendered chart output is admitted as a plain bundle
      Given the rendered output of a Helm chart as plain multi-document YAML
      When the developer applies the rendered output
      Then the bundle records the rendered manifests as its content

  Rule: Non-deterministic renders fail intake with a determinism error
    Digest-match evidence transfer presumes reproducible renders: pinned
    charts and values, no cluster lookups, timestamps, or randomness. A
    bundle whose render is non-deterministic MUST fail intake rather than
    produce flapping digests.

    Scenario: A render embedding a timestamp is rejected at intake
      Given a bundle whose rendered manifests embed a generation timestamp
      When the bundle is admitted
      Then intake fails with a determinism error naming the offending source

  Rule: The offline check reports compatibility without a cluster
    The check command MUST run intake analysis offline, report every blocker
    with its fix, exit nonzero on any blocker, and MUST NOT require a cluster.

    Scenario: Check reports intake violations as a fix list
      Given a manifest directory containing a cluster-scoped resource and a multi-namespace bundle
      And no cluster is reachable
      When the developer runs the offline check against the directory
      Then every violation is reported with the violating manifest and the suggested fix
      And the command exits nonzero

    Scenario: Check passes a compatible directory
      Given a manifest directory with no intake blockers
      And no cluster is reachable
      When the developer runs the offline check against the directory
      Then the command reports the directory compatible and exits zero

    Scenario: Check detects render determinism violations offline
      Given a bundle source whose render embeds a random value
      When the developer runs the offline check against the source
      Then the determinism violation is reported with the offending source
      And the command exits nonzero
```
