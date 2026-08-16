# Evidence

Evidence is the machine-gathered, control-plane-signed record of a sealed
sandbox run — the input review has been missing. It is honest by construction:
it records what ran, what was witnessed, and for how long, never more. At L2 it
is delivered as a signed status check on the pull request, consumed by the
team's own branch protection. Covers draft requirements G2, G3, G6, G7 (binding
and transfer; render determinism at intake is specified in bundle-intake), G13,
and X4.

Scope notes (v1): merge-queue integration for merged-tree runs (G7 SHOULD) is
an open question on provider scope (design.md); the canary stage appending
production metrics to evidence (G10) is post-v1.

```gherkin
Feature: Evidence

  Rule: Diff compares a bundle against production, normalized
    The diff MUST compare a bundle against what production currently runs,
    normalized for defaulting, managed fields, and recorded intake transforms,
    so the diff is noise-free.

    Scenario: Server-side defaulting does not pollute the diff
      Given a bundle identical to production except for one changed container image
      When the developer diffs the bundle against production
      Then the diff shows only the image change
      And defaulted fields, managed fields, and recorded intake transforms produce no diff lines

  Rule: Evidence carries the machine-gathered record of the run
    Evidence MUST carry the bundle digest, source sandbox, normalized diff,
    and the machine-gathered facts: seal status, egress violation count,
    substrate digest match, per-datastore fidelity, capacity mode and intake
    transforms, workload readiness, observed healthy duration, witnessed
    activity and test results, and declared dependency status.

    Scenario: A completed run's evidence lists every gathered fact
      Given a sealed sandbox run that stayed healthy for two hours with a passing witnessed test suite
      When the control plane assembles the run's evidence
      Then the evidence carries the bundle digest, the source sandbox, and the normalized diff
      And it records seal status, egress violation count, and substrate digest match
      And it records per-datastore fidelity, capacity mode, intake transforms, readiness, and observed healthy duration
      And it records witnessed activity with test results and the declared dependency status

  Rule: Evidence wording stays scoped and honest
    Evidence MUST state what ran sealed at which fidelity, capacity mode, and
    duration with witnessed activity — never "verified working". Claims cover
    exercised code paths only; idle boot is distinguished from active traffic;
    no production load is implied; identity authorization and secret values
    are labeled declared-not-verified.

    Scenario: The evidence summary makes the scoped claim and no more
      Given evidence for a run at fidelity profile-synthetic, capacity squeezed, healthy for two hours, with eighty-four witnessed test events
      When the evidence summary is rendered
      Then it states the run was sealed with zero undeclared dependency attempts on a substrate matching production
      And it names the fidelity level, capacity mode, healthy duration, and witnessed activity count
      And it labels identity authorization and secret values declared-not-verified
      And it nowhere claims the change is verified working

    Scenario: Idle boot is distinguished from active traffic
      Given a run that booted healthy but received no test runs or traffic
      When the evidence summary is rendered
      Then the witnessed activity count is zero and the run is presented as idle, not exercised

  Rule: Evidence binds to the pull request by bundle digest
    After merge the controller re-renders the merge result: on digest match
    the sandbox's evidence transfers with its accumulated soak time; on
    mismatch the evidence MUST be marked stale — the check fails, the
    promotion blocks — until a sandbox run of the merged tree produces fresh
    evidence.

    Scenario: A clean merge inherits the branch's evidence
      Given a pull request whose sandbox produced valid evidence for digest A
      When the merge result re-renders to digest A
      Then the evidence transfers to the merged commit with its accumulated soak time

    Scenario: A divergent merge marks the evidence stale
      Given a pull request whose sandbox produced evidence for digest A
      When the merge result re-renders to a different digest B
      Then the evidence is marked stale
      And the evidence check fails and any promotion blocks until a merged-tree run produces fresh evidence

  Rule: Witnessed activity is attributed and signed by the control plane
    The test command MUST run a declared suite as a Job inside the sandbox;
    the control plane attributes the run and its traffic through the egress
    proxy and signs the results into evidence. CI systems can trigger the
    run; they MUST NOT be able to assert its results.

    Scenario: An in-sandbox test run becomes witnessed activity
      Given an application with a declared test suite
      When the developer runs the test command against their sandbox
      Then the suite executes as a Job inside the sealed sandbox
      And the control plane attributes the run and signs its results into the evidence as witnessed activity

    Scenario: A CI pipeline can trigger tests but cannot assert results
      Given a CI pipeline that triggers the test command
      When the pipeline reports its own test outcome to the control plane
      Then the reported outcome is not accepted as witnessed activity
      And only the control-plane-attributed run appears in evidence

  Rule: The evidence check is minted only by the control plane
    Evidence MUST be posted as a signed status check on the pull request by
    the control plane through the SCM integration — pass or fail plus the
    honest summary line, linking to the full record. A valid check MUST
    require a control-plane-managed sandbox, a held seal with zero
    violations, a substrate match, fidelity at or above the policy minimum,
    and witnessed activity at or above the policy minimum.

    Scenario: A qualifying run posts a passing signed check
      Given a managed-sandbox run with the seal held, zero egress violations, a substrate match, and fidelity and witnessed activity at policy minimums
      When the control plane evaluates the run for the pull request
      Then a signed passing status check is posted with the summary line and a link to the full evidence record

    Scenario: A run with a seal violation posts a failing check
      Given a managed-sandbox run that recorded one blocked egress attempt
      When the control plane evaluates the run for the pull request
      Then the posted status check fails and names the violation

    Scenario: Fidelity below the policy minimum fails the check
      Given an application policy requiring fidelity of at least schema-replay
      And a run whose datastore fidelity was fixtures
      When the control plane evaluates the run for the pull request
      Then the posted status check fails for fidelity below the policy minimum
```
