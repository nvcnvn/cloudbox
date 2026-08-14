# 0007 — Go control plane and CLI; acceptance runs on a simulated cluster driver

- Status: accepted
- Date: 2026-08-14
- Supersedes: —

## Context

Task 1.2 of add-cloudbox-v1 requires an application skeleton and a durable
record of the implementation-stack choice. Separately, the acceptance suite
must boot "the app" and drive ~80 scenarios spanning control-plane logic and
cluster effects (seals, probes, readiness); booting real clusters per scenario
would be minutes each and flaky, and CNI enforcement still could not be
asserted from a unit-style harness.

## Decision

- **Go** for both binaries: `cloudboxd` (control plane, HTTP API) and
  `cloudbox` (thin CLI per ADR 0004). Go is the Kubernetes ecosystem language
  and ships static binaries for the single-binary install (§7).
- Cluster interactions go through a **cluster driver interface**. Two drivers:
  `kube` (real clusters — the production path) and `sim` (in-process model of
  the Kubernetes semantics the specs exercise: namespaces, NetworkPolicy
  accept-vs-enforce, workload readiness and OOM, DNS/proxy egress paths,
  multiple registered clusters).
- The **acceptance suite runs against `cloudboxd --driver sim`**: hermetic,
  fast, and honest about what it verifies — the product's behavior contracts,
  not CNI conformance. Real-driver conformance across managed CNIs is the
  separate integration matrix named in design.md's risks.
- Sim mode exposes a **`/simctl/*` test-arrangement surface** (e.g. reset,
  making a simulated CNI non-enforcing) so Given-steps can arrange the world.
  It exists only when the sim driver is constructed; a kube-driver server
  never registers those routes.

## Consequences

- Acceptance scenarios stay in domain language and run in seconds; the same
  page objects can later target a kube-driver deployment for conformance runs.
- The sim driver is load-bearing test infrastructure: its fidelity to
  Kubernetes semantics bounds what the suite can prove, so any semantic it
  models must match the real driver's observed behavior as that lands.
- The kube driver is deliberately unimplemented until the sim-verified
  contracts hold; `cloudboxd --driver kube` fails loudly today.
