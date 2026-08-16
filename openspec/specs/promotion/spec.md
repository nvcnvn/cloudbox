# Promotion

Promotion is the approval-gated write path to production at L3/L4: write-back
to the GitOps repo (recommended) or direct apply (strict mode). Below L3 the
product holds no production write access at all. Covers draft requirements G1,
G4, G5, G8 (queued, the v1 default), G9, G11, G12.

Scope notes (v1): auto-promote merge semantics (G8) and the canary stage
through a progressive-delivery controller (G10) are v1.x / post-v1 committed
direction, recorded in design.md. Per-application fast-track rollback approvals
(G11 MAY) are policy detail left to design.

```gherkin
Feature: Promotion

  Rule: Strict mode admits exactly one writer to production
    In strict mode, managed production namespaces MUST be writable only by
    the controller executing an approved promotion; human and agent roles are
    read-only there; no CLI flag targets production. Below L4 the product
    holds no production write credentials, or writes only to the GitOps repo.

    Scenario: A direct human write to a managed namespace is denied
      Given an application at strict mode with a managed production namespace
      When an engineer attempts to apply a manifest directly to that namespace
      Then the write is denied by the product-managed RBAC

    Scenario: No CLI path targets production directly
      Given the full v1 command surface
      When a user searches for a flag or verb that applies a bundle straight to production
      Then no such flag or verb exists at any adoption level

    Scenario: Below strict mode the product cannot write to production
      Given an application at the evidence-check level
      When the control plane's credentials are inspected
      Then the product holds no production write credentials
      And production is only observed and verified, never controlled

  Rule: Promotions require declared approvals and reject self-approval
    Approval policy — required approver count and allowed roles — is declared
    per application; self-approval MUST be rejected and MUST also be enforced
    server-side.

    Scenario: A promotion waits for the declared approvers
      Given an application policy requiring two approvers with the platform role
      When a promotion request is opened
      Then the promotion remains pending until two platform-role approvals are recorded

    Scenario: The promotion author cannot approve their own promotion
      Given a promotion request opened by developer Priya
      When Priya attempts to approve it
      Then the approval is rejected server-side as self-approval

  Rule: Every promotion transition is synchronously audited
    Every transition — created, approved, applied, failed — MUST be written
    to a synchronous audit log; if the audit sink is unavailable the
    transition MUST NOT proceed.

    Scenario: Transitions land in the audit log as they happen
      Given a promotion request moving through approval and apply
      When each transition occurs
      Then a synchronous audit record is written before the transition completes

    Scenario: An unavailable audit sink blocks the transition
      Given the audit sink is unreachable
      When an approved promotion attempts to apply
      Then the apply does not proceed until the audit record can be written

  Rule: Merge opens a queued promotion awaiting explicit approval
    In the v1 default merge semantics, a merge with valid evidence opens a
    promotion request that waits for explicit approval under the declared
    approval policy.

    Scenario: Merging a proven pull request queues a promotion
      Given a pull request with a passing evidence check at a write-back-level application
      When the pull request is merged
      Then a promotion request carrying the transferred evidence is opened
      And it awaits explicit approval under the application's policy

  Rule: Write-back promotion commits the bundle and verifies the applied result
    On approval in write-back mode, the controller commits the rendered
    bundle to the declared production path of the GitOps repository; the
    team's GitOps controller applies it; the promotion completes only when
    live state matches the bundle digest. Evidence and audit semantics MUST
    be identical in write-back and direct modes.

    Scenario: An approved write-back promotion completes on digest match
      Given an approved promotion for bundle digest A at a write-back application
      When the controller commits the rendered bundle to the declared repository path
      And the team's GitOps controller applies the commit
      Then the promotion completes only when the controller verifies live state matches digest A

    Scenario: Write-back and direct modes share evidence and audit semantics
      Given the same approved promotion executed once in write-back mode and once in direct mode
      When the audit log and promotion evidence are compared
      Then the recorded evidence and audit semantics are identical

  Rule: Rollback is a promotion
    The controller MUST retain previously applied production bundles and open
    a promotion for a prior digest in one command, carrying the original
    evidence plus its production history. A failed or partial apply MUST
    leave the promotion failed with the divergence recorded and the rollback
    path available.

    Scenario: One command opens a rollback promotion with history
      Given a previously applied production bundle with two weeks of observed-healthy history
      When an operator opens a rollback promotion for that digest in one command
      Then the promotion carries the bundle's original evidence and its production history

    Scenario: A partial apply fails the promotion and keeps rollback open
      Given an approved promotion whose apply partially succeeds
      When the controller detects the live-state divergence
      Then the promotion is left in the failed state with the divergence recorded
      And the rollback path remains available

  Rule: Break-glass access is audited and reconciled
    Strict mode MUST include a named emergency role with auto-expiring direct
    write access, no approval at grant time, every action audited. Any
    production write outside an approved promotion MUST be detected as
    divergence and MUST invalidate current-bundle evidence until a
    reconciling promotion lands. Strict mode without a configured break-glass
    role MUST fail setup.

    Scenario: Break-glass grants expiring audited access without approval
      Given a strict-mode application with a configured break-glass role
      When the emergency role requests break-glass access
      Then auto-expiring write credentials are granted immediately without approval
      And every action taken under them is audited

    Scenario: An out-of-band write invalidates evidence until reconciled
      Given a production write that did not come from an approved promotion
      When the controller detects the divergence
      Then the divergence is recorded and the current bundle's evidence is invalidated
      And validity returns only when a promotion adopting or reverting the divergence lands

    Scenario: Strict mode refuses setup without a break-glass role
      Given an application being configured for strict mode with no break-glass role
      When setup runs
      Then setup fails stating that strict mode requires a configured break-glass role
```
