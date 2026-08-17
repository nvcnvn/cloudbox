# Proposal: add-kube-driver-conformance

## Why

Every one of CloudBox v1's 102 acceptance scenarios is verified against the
simulated cluster driver, and `cloudboxd --driver kube` fails loudly by design
(ADR 0007). That was the right trade for building the product — hermetic, fast,
honest about what it proved — but it leaves the load-bearing claim untested:
that a sandbox the control plane calls *sealed* is actually sealed by a real
CNI. A 284-line simulation currently bounds the credibility of every evidence
artifact the product mints. This change closes that gap for the cluster-effect
contracts and puts the whole suite in CI, which today runs nowhere.

## What Changes

- Implement the **`kube` cluster driver** behind the existing
  `cluster.Driver`/`cluster.Cluster` interface. The interface is the contract
  and is **not widened** for the real driver's convenience; where real
  Kubernetes cannot satisfy a method as written, that is a spec-visible finding,
  not an interface change.
- `cloudboxd --driver kube` becomes a working path instead of a fatal error.
- Introduce a **tagged conformance subset**: the cluster-effect scenarios run
  against a real cluster; everything else stays on sim. The default
  `make acceptance` run continues to execute the sim suite and MUST exclude
  conformance-tagged scenarios, so a developer without a cluster is never
  blocked.
- Add **GitHub Actions CI** — nothing in this repository currently runs in CI.
  A pull-request workflow runs `go vet`, `make build`, `make lint-specs`, the
  full sim suite, and the conformance subset on Kind with an enforcing CNI.
- Establish the **sim-fidelity reconciliation rule**: where the kube driver's
  observed behaviour contradicts `internal/sim/world.go`, the sim is corrected
  and the divergence is recorded. ADR 0007 asserted this obligation; this change
  makes it actionable.
- Resolve Open Question 9 of `add-cloudbox-v1`.

### Non-goals

- **Managed offerings and k3s.** EKS/GKE/AKS need cloud credentials and per-run
  budget; the full conformance matrix in `add-cloudbox-v1`'s design remains open
  for them. Kind with an enforcing CNI is the whole of this change's substrate.
- **External-system realism.** GitOps sync, SCM checks, audit sinks, production
  state, and database seeding stay on their simulated arrangement surfaces. They
  are orthogonal to the cluster driver, and a real Argo or GitHub would prove
  nothing about sealing.
- **Porting the whole suite.** `boundary-contract`, `bundle-intake`, and
  `substrate-parity` scenarios touch no cluster arrangement at all; they are
  pure control-plane logic that would behave identically against a real cluster
  and only run slower. `promotion` is the most arrangement-heavy capability in
  the suite but almost entirely through GitOps/SCM/audit, not the cluster.
- **Soak-window conformance.** Soak is specified in hours (S6: three hours of
  accumulated soak preserved across a rebase). It cannot be honestly real-clocked
  in CI, so soak stays on the simulated clock and out of the subset.
- **No changes to v1 requirements.** Nothing here relaxes, restates, or
  re-verifies an existing Rule; the conformance run asserts the same behaviour
  against a different substrate.

## Capabilities

### New Capabilities

- `kube-driver`: A real-Kubernetes implementation of the cluster contract —
  namespaces, standard NetworkPolicy sealing, enforcement probing, egress
  evaluation, workload admission and readiness, and substrate inspection —
  including the requirement that a non-enforcing CNI causes the probe to fail
  rather than a sandbox to be falsely reported sealed, and that the sim-only
  arrangement surface is never registered under this driver.
- `conformance-ci`: The tagged conformance subset and its continuous-integration
  contract — which scenarios run against a real cluster and which are excluded
  and why, the default run's exclusion of conformance tags, real-clock lifecycle
  timing, the enforcing-CNI precondition, and the rule for reconciling sim
  fidelity against observed real-driver behaviour.

### Modified Capabilities

None. No existing requirement changes.

`openspec/specs/` is currently empty because `add-cloudbox-v1` has not been
archived, so every v1 capability still exists as an active delta. Declaring one
of them modified here would place two active changes' deltas over the same
capability, which the effective-spec composition would surface as duplicate
scenarios — the condition `add-cloudbox-v1`'s own completion task 9.2 checks.
The new behaviour in this change is genuinely additive: a driver that did not
exist, a CI contract that did not exist, and a reconciliation rule that was
stated as an ADR consequence but never specified.

## Impact

**New code**
- `internal/cluster/kube/` — the driver implementation.
- `.github/workflows/` — the first CI configuration in the repository.

**Modified code**
- `cmd/cloudboxd/main.go:25` — the `kube` case stops calling `log.Fatalf`.
- `internal/server/server.go` — the `/simctl/*` registration stays conditional
  on the sim driver (ADR 0007); the conformance subset needs an arrangement path
  that does not depend on those routes.
- `internal/sim/world.go` — corrected wherever the real driver contradicts it.
- `acceptance-tests/` — conformance tagging, runner tag-filtering, and a
  cluster-targeted configuration reusing the existing page objects. Step
  definitions stay free of selectors and URLs.
- `Makefile` — a conformance target alongside `acceptance`.

**Dependencies**
- A Kubernetes client library for Go, and Kind plus an enforcing CNI (Calico or
  Cilium) in CI. The pinned Kind version's default CNI must be verified rather
  than assumed: if `kindnetd` does not enforce NetworkPolicy, a Kind-based seal
  test would be silently vacuous.

**Interfaces**
- `internal/cluster/cluster.go` is unchanged.

**In-force ADRs**
- 0001 (NetworkPolicy floor with egress proxy) and 0007 (Go control plane with
  simulated cluster driver) constrain this change most directly. A new ADR is
  expected for the conformance-subset and time-control decisions.
