# Conformance subset definition

The conformance subset is the set of scenarios tagged `@conformance` in the
effective spec. It runs against a real cluster via
`python run_acceptance.py --conformance`, which selects only the tag and boots
`cloudboxd --driver kube` (ADR 0008). Everything untagged is the default sim
suite.

A scenario belongs in the subset when a real cluster proves something the
simulation cannot: CNI enforcement of the seal, real workload readiness and
out-of-memory kills, live substrate reads, real-clock lifecycle expiry. A
scenario is excluded — explicitly, with its reason recorded below — when a
real cluster cannot honestly prove more than the sim already does. This file
is the subset definition the `conformance-ci` capability's exclusion
scenarios assert against.

## Recorded exclusions

| Excluded | Reason |
|---|---|
| Soak-window scenarios (sandbox-lifecycle S6 and its evidence/substrate-parity echoes: soak reset on digest change, soak preserved across a rebase, soak transferring with evidence) | Soak windows are specified in hours — S6 preserves three accumulated hours across a rebase. Hours of wall clock cannot be honestly real-clocked in CI, and a result obtained by advancing a simulated clock must never be reported as conformance (ADR 0008). Soak stays on the simulated clock in the default run. |
| Scenarios arranged only through simulated source control (pushes, merges, rebases via the sim SCM surface) | The arrangement is the simulation. A real cluster proves nothing about an SCM interaction; a real GitHub would prove nothing about sealing. Orthogonal to the cluster driver by design. |
| Scenarios arranged only through simulated GitOps sync (promotion write-back, sync-state transitions) | Same boundary: GitOps realism is an external-system concern, not a cluster-effect one. The promotion capability is arrangement-heavy through GitOps/SCM/audit, not the cluster. |
| Scenarios arranged only through simulated audit sinks (audit-log writes, unavailable-sink blocking) | The audit sink is an external system; its availability semantics are fully exercised in the sim and gain nothing from a real CNI. |
| Scenarios arranged only through simulated production state (direct-write denial, strict-mode posture, break-glass) | Production state exists only as a simulated arrangement; there is no real production in CI to arrange against, and pretending otherwise would be a vacuous pass. |
| Scenarios arranged only through simulated datastore seeding (data-fidelity profiles, thin clones, migration replay) | Seeded data is a simulated arrangement surface; real seeding needs real datastores with real data, which is out of this change's scope (non-goal: external-system realism). |
