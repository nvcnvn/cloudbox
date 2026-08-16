# Control plane

The product adds a control plane beside the user's resources, never a schema
above them: five CRDs under one API group, all enforcement server-side, CI
systems treated as untrusted triggers. Covers draft requirements CP1–CP4.

```gherkin
Feature: Control plane

  # @openspec: ADDED
  Rule: Exactly five CRDs under one API group, beside user resources
    The product introduces Application, Sandbox, Bundle, PromotionRequest,
    and ClusterRegistry under one API group, and MUST NOT require wrapping or
    re-schematizing any user workload. Inter-application dependency graphs
    are validated across Application resources.

    Scenario: Installation adds five CRDs and nothing wraps user workloads
      Given a cluster where setup has installed the controller
      When the installed CRDs are listed
      Then exactly Application, Sandbox, Bundle, PromotionRequest, and ClusterRegistry exist under the product's API group
      And user workloads run unwrapped and unmodified beside them

    Scenario: A dangling dependency reference is caught across Applications
      Given an Application declaring a dependency on an application that does not exist
      When the Application is admitted
      Then validation rejects the dangling dependency reference

  # @openspec: ADDED
  Rule: Enforcement is server-side and the CLI is thin
    All validation, bundling, evidence gathering, signing, and enforcement
    run server-side in the controller; the CLI creates resources and watches
    status so there is no client-version drift in enforcement. The offline
    check is the one deliberate advisory exception; server-side intake
    remains authoritative.

    Scenario: An outdated CLI cannot weaken enforcement
      Given a developer running an outdated CLI version
      When they apply a bundle that current server-side intake rejects
      Then the apply is rejected by the controller regardless of the client version

    Scenario: Offline check is advisory and server intake decides
      Given a manifest directory that passes an outdated offline check
      When the bundle is applied to a sandbox
      Then server-side intake re-runs the analysis and its verdict is authoritative

  # @openspec: ADDED
  Rule: Sandbox and production clusters may be shared or separate
    Sandbox and production MAY be namespaces on the same cluster or different
    registered clusters; evidence semantics MUST be identical in both
    topologies.

    Scenario: Split-cluster and shared-cluster runs yield the same evidence semantics
      Given one application running sandboxes on the production cluster and another using a separate registered sandbox cluster
      When both produce evidence for equivalent runs
      Then the evidence carries the same facts with the same validity rules in both topologies

  # @openspec: ADDED
  Rule: CI systems are untrusted triggers
    All evidence MUST be gathered and signed by the control plane; commit and
    PR status checks MUST be posted only by the controller through the SCM
    integration. A pipeline can carry or display evidence; it MUST NOT be
    able to mint it.

    Scenario: A pipeline-forged evidence check is impossible
      Given a CI pipeline holding no control-plane signing capability
      When the pipeline attempts to post an evidence status check to a pull request
      Then the posted check is not the product's signed evidence check
      And the product's check appears only when the controller posts it through the SCM integration
```
