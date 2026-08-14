# Sandbox lifecycle

A sandbox is a disposable, sealed, substrate-verified environment owned by one
developer or agent. Creation is one command with no approval; lifecycle binds
to the pull request when the SCM integration is on. Covers draft requirements
S1–S3, S5–S7.

Scope notes (v1): the virtual-cluster opt-up mechanism for multi-namespace
bundles (S4) is post-v1; linked dependency sandboxes (S8) are v1.x — the v1
stub alternative is specified in the boundary-contract capability. Sealing
semantics live in the sandbox-seal capability; capacity claims and honesty
wording live in the evidence capability.

```gherkin
Feature: Sandbox lifecycle

  # @openspec: ADDED
  Rule: Sandbox creation is one command with no approval
    Any developer or agent MUST be able to create a sandbox with a single
    command and no approval; RBAC MUST restrict modification to the owner.

    Scenario: A developer creates a sandbox without waiting on anyone
      Given a developer with access to the application
      When they run the sandbox create command
      Then a sandbox owned by them is provisioned
      And no approval step occurs

    Scenario: A non-owner cannot modify another developer's sandbox
      Given a sandbox owned by developer Priya
      When developer Minh attempts to apply a bundle to that sandbox
      Then the request is denied as not the sandbox owner

  # @openspec: ADDED
  Rule: Shared-cluster sandboxes are sealed and ready within thirty seconds
    The default mechanism is a sealed namespace per sandbox on a shared
    cluster; control-plane readiness MUST be at most thirty seconds, with
    total readiness dominated only by the user's own workloads.

    Scenario: Control-plane readiness meets the thirty-second target
      Given a shared cluster registered with the control plane
      When a developer creates a sandbox
      Then the sandbox reports sealed and ready for apply within thirty seconds
      And any remaining wait is attributable to the user's own workloads

  # @openspec: ADDED
  Rule: Local sandboxes provision from the substrate lockfile
    One command MUST provision a local Kind cluster from the application's
    substrate lockfile behaving as a valid sandbox; local evidence MUST be
    non-promotable and non-postable because the cluster is user-controlled.

    Scenario: One command yields a local sandbox with the same seal semantics
      Given an application with a substrate lockfile
      When the developer creates a sandbox with the local option
      Then a local Kind cluster is provisioned from the lockfile
      And the sandbox enforces the same seal and iteration semantics as a managed sandbox

    Scenario: Local-sandbox evidence cannot reach a check or a promotion
      Given evidence produced by a local sandbox
      When the developer attempts to post an evidence check or open a promotion from it
      Then the attempt is refused because the evidence source is not a control-plane-managed sandbox

  # @openspec: ADDED
  Rule: Sandboxes expire and stay within application quotas
    Sandboxes MUST support TTL and idle expiry and per-application resource
    quotas.

    Scenario: A sandbox is destroyed when its TTL fires
      Given a sandbox created with a TTL of one day
      When the TTL elapses
      Then the sandbox and its workloads are destroyed

    Scenario: An idle sandbox expires
      Given a sandbox with idle expiry configured by the application
      When the sandbox sees no activity for the idle window
      Then the sandbox is destroyed

    Scenario: An apply exceeding the application quota is rejected
      Given an application quota of four CPUs per sandbox
      When a developer applies a bundle requesting eight CPUs after squeezing
      Then the apply is rejected with the quota that was exceeded

  # @openspec: ADDED
  Rule: Pull-request-bound sandboxes follow the pull request lifecycle
    With the SCM integration enabled, one branch is one sandbox: created on
    PR open, re-rendered and re-applied on every push, expired on close or
    merge. Soak time accumulates only while the bundle digest is unchanged;
    an identical re-rendered digest MUST preserve accumulated soak time.

    Scenario: Opening a pull request creates its sandbox
      Given the SCM integration is enabled for the application
      When a pull request is opened
      Then a sandbox bound to that pull request is created

    Scenario: A push that changes the bundle digest resets the soak clock
      Given a PR-bound sandbox healthy for three hours on digest A
      When a push re-renders the branch to digest B
      Then the new bundle is applied
      And observed-healthy duration restarts from zero

    Scenario: A rebase with an identical rendered digest inherits soak time
      Given a PR-bound sandbox healthy for three hours on digest A
      When a rebase re-renders the branch and the digest is still A
      Then the accumulated three hours of soak time are preserved

    Scenario: Closing the pull request expires its sandbox
      Given a PR-bound sandbox
      When the pull request is merged or closed
      Then the sandbox TTL fires and the sandbox is destroyed

  # @openspec: ADDED
  Rule: Capacity transforms are recorded and never edit bundle bytes
    Bundles carry production capacity and MUST NOT be hand-edited to fit
    quotas. The controller applies a recorded capacity transform at admission
    with the mode declared in evidence; workload-internal sizing MUST NOT be
    rewritten; autoscalers acting on transformed requests are suspended in
    squeezed and minimal modes and the suspension recorded.

    Scenario: Squeezed mode preserves topology while scaling resources
      Given a production-sized bundle with three replicas and quorum-based leader election
      When the bundle is admitted with the default squeezed capacity mode
      Then replica counts, topology, and scheduling constraints are preserved
      And CPU requests are scaled down while memory is only reduced to a per-container floor
      And the capacity mode "squeezed" is declared in evidence with the bundle digest unchanged

    Scenario: Minimal mode floors replicas to one
      Given a production-sized bundle with three replicas
      When the bundle is admitted with the minimal capacity mode
      Then replicas are floored to one and requests are scaled
      And the capacity mode "minimal" is declared in evidence

    Scenario: A workload that cannot survive squeezing is diagnosed, not hidden
      Given a workload whose container is OOM-killed under the squeezed transform
      When the developer checks the sandbox status
      Then a capacity-squeeze-incompatible diagnostic names the workload
      And the diagnostic guides the operator toward configuring full capacity mode

    Scenario: Workload-internal sizing is never rewritten
      Given a container whose JVM heap is set by an -Xmx environment variable
      When the squeezed transform is applied
      Then the environment variable is untouched

    Scenario: Autoscaler suspension is recorded in evidence
      Given a bundle with a horizontal pod autoscaler acting on CPU requests
      When the bundle is admitted in squeezed mode
      Then the autoscaler is suspended for the sandbox
      And the suspension is recorded in evidence
```
