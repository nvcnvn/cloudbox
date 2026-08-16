# 0008 — Kube driver behind a frozen contract, with a tagged conformance subset

- Status: accepted
- Date: 2026-08-14
- Supersedes: 0007

## Context

ADR 0007 chose Go for both binaries, put every cluster interaction behind a
driver interface, and ran the whole acceptance suite against a simulated
driver — deliberately deferring the kube driver until the sim-verified
contracts held. Those contracts now hold (102 green scenarios), which makes
0007's final consequence — "`cloudboxd --driver kube` fails loudly today" —
the next thing to retire. The load-bearing claim the sim cannot make remains
untested: that a sandbox the control plane reports *sealed* is actually sealed
by a real CNI. Nothing in the repository runs in continuous integration.

Everything else in 0007 — the Go stack, the two binaries, the driver
interface, the sim driver as the default acceptance substrate, `/simctl/*`
existing only when the sim driver is constructed — remains correct and is
carried forward unchanged.

## Decision

- **The cluster contract is frozen.** `internal/cluster/cluster.go` is the
  contract the sim driver already satisfies; the kube driver implements it as
  written. The contract gains, loses, or changes no method to accommodate the
  real driver — a method real Kubernetes cannot satisfy as specified is a
  spec-visible finding that pauses implementation, not a signature change.
- **A tagged conformance subset, excluded by default.** Cluster-effect
  scenarios carry `@conformance` and run against a real cluster (Kind with an
  enforcing CNI); the default run excludes the tag so a developer with no
  cluster is never blocked. Scenarios a real cluster cannot honestly prove
  more about than the sim — soak windows specified in hours; scenarios
  arranged only through simulated source control, GitOps sync, audit sinks,
  production state, or datastore seeding — are excluded from the subset with
  the reason recorded where the subset is defined.
- **The enforcement gate is proven in both directions.** A conformance run
  first proves the target cluster enforces NetworkPolicy and fails naming
  unproven enforcement otherwise; CI also runs the probe-failure scenario
  against a deliberately non-enforcing cluster, so the product's refusal to
  lie is itself tested on real infrastructure.
- **Time-dependent conformance uses the real clock** with short configured
  durations (second-granular TTL and idle expiry through the injected clock).
  A result obtained by advancing a simulated clock is never reported as
  conformance; soak stays simulated and out of the subset.
- **Conformance arranges through the product's own surface.** No
  test-arrangement endpoint exists under `--driver kube`; scenarios deploy
  real workloads and assert on product observables (recorded blocked egress,
  `status --explain`), not on a `/testctl`-style side door.
- **Sim divergences are corrected and recorded.** Where the kube driver's
  observed behaviour contradicts `internal/sim/world.go`, the sim is corrected
  to match and the divergence recorded with the behaviour that differed — the
  obligation 0007 asserted, now actionable.

## Consequences

- `cloudboxd --driver kube` becomes a working path; 0007's deliberate
  fatal-error posture ends with this ADR.
- The seal claim is bounded by a real CNI instead of a 284-line model, and a
  vacuous conformance run (non-enforcing CNI) is a loud failure, never a pass.
- CI cost: every pull request boots two Kind clusters — one enforcing, one
  deliberately not. The non-enforcing cluster serves exactly one scenario and
  needs no CNI install.
- The frozen contract may prove genuinely insufficient for real Kubernetes;
  that outcome halts code and reopens the specs, by design.
- The sim remains the primary, fast signal; its fidelity is now audited by
  every conformance run rather than asserted.
