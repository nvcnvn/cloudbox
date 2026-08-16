# Tasks: add-kube-driver-conformance

Implementation MUST honour the in-force ADRs under `adr/`: 0001–0006 and 0008.
ADR 0007 is superseded by 0008 and is historical context only.

`acceptance-tests/` already exists (Python/behave, `stack: python` in
`openspec/config.yaml`), so the first-time setup section does not apply and is
omitted.

Sequencing follows design.md's migration plan: tagging and selection first so the
default run goes green again, then CI for what already exists, then the driver
against the frozen contract, then the clusters, then real-clock timing. Each
implementation task takes one spec Rule red → green: run the suite so its
scenarios fail for the right reason, implement until they pass, then commit.

Two standing constraints apply to every task below. `internal/cluster/cluster.go`
MUST NOT change (ADR 0008, frozen contract) — if a task appears to require it,
stop and raise a spec finding instead. `/simctl/*` MUST remain registered only
when the sim driver is constructed; no task may add a test-arrangement endpoint
reachable under `--driver kube`.

## 1. Tagging and selection — restore a green default run

Adding this change's specs put 22 scenarios and 78 undefined steps into the
effective suite, so the default run is red until 1.1 lands. Do this first.

- [x] 1.1 conformance-ci: the default acceptance run excludes conformance-tagged scenarios and the conformance run selects only them — add the tag vocabulary, the `behave.ini` default exclusion, and a conformance mode in `run_acceptance.py` that boots `cloudboxd --driver kube`; keep the `OPENSPEC_ACCEPTANCE` tripwire intact — red → green → commit
- [x] 1.2 conformance-ci: scenarios the real run cannot honestly prove are excluded and recorded — define the subset with its exclusion reasons (soak windows specified in hours; scenarios arranged only through simulated source control, GitOps, audit sinks, production state, or datastore seeding) — red → green → commit

## 2. Continuous integration foundation

- [x] 2.1 Add the GitHub Actions pull-request workflow — the repository's first CI — running `go vet ./...`, `make build`, `make lint-specs`, and the full sim suite, publishing the HTML report as an artifact. The conformance stage is added in 5.2, which is where the corresponding spec Rule goes green; do not mark that Rule complete here

## 3. Kube driver behind the frozen contract

- [x] 3.1 Local and CI Kind setup: pin a Kind version, install an enforcing CNI, and **verify that the pinned version's default CNI does or does not enforce NetworkPolicy** rather than assuming — record the finding, since a non-enforcing default is what task 4.2 deliberately exploits. Add a `make conformance` target and kubeconfig wiring. Resolves design Open Question 2 (Calico or Cilium)
- [x] 3.2 kube-driver: the kube driver satisfies the cluster contract without widening it — control plane boots healthy against a real cluster, product CRDs installed and listed; confirm `internal/cluster/cluster.go` is byte-identical to its pre-change state — red → green → commit
- [x] 3.3 kube-driver: the simulation arrangement surface is absent under the kube driver — red → green → commit
- [x] 3.4 kube-driver: the substrate is read from the live cluster, so substrate digests reflect what is actually installed (ADR 0006) — red → green → commit
- [x] 3.5 kube-driver: real workload readiness reported from cluster status, and a memory-limit kill reported as out-of-memory with capacity-squeeze-incompatible diagnostics — red → green → commit
- [x] 3.6 kube-driver: sealing a real namespace installs standard NetworkPolicy v1 admitting only cluster DNS and the egress proxy, with no vendor policy CRDs (ADR 0001) — red → green → commit
- [x] 3.7 kube-driver: a real workload's blocked egress is denied by the cluster and recorded with destination and attribution; allowlisted destinations reachable through the proxy; in-sandbox traffic and cluster DNS survive the seal — assert through the product's own blocked-egress surface, not a test endpoint (ADR 0008) — red → green → commit
- [x] 3.8 kube-driver: a non-enforcing CNI fails the probe — sandbox not reported sealed, no evidence emitted, cause names unenforced network policy — red → green → commit

## 4. Enforcement gate and the non-enforcing cluster

- [x] 4.1 conformance-ci: a conformance run gates on proven NetworkPolicy enforcement — passes on an enforcing cluster, fails naming unproven enforcement otherwise, and reports no pass in that case — red → green → commit
- [ ] 4.2 Provision the deliberately non-enforcing Kind cluster in CI that tasks 3.8 and 4.1 assert against. Resolves design Open Question 3 (stock default CNI versus an enforcing CNI with policy disabled)

## 5. Real-clock timing and the full integration check

- [ ] 5.1 conformance-ci: conformance lifecycle expiry uses the real clock — short real TTL destroys the sandbox and removes its namespace, short real idle window destroys an inactive sandbox, with no simulated clock advance in the conformance path — red → green → commit
- [ ] 5.2 conformance-ci: continuous integration runs the full effective suite on every change — extend the 2.1 workflow with the conformance subset against Kind with an enforcing CNI, and confirm a failing subset fails the check — red → green → commit

## 6. Sim reconciliation

- [ ] 6.1 conformance-ci: a divergence between the simulation and the real driver is recorded and reconciled — correct `internal/sim/world.go` to match observed real behaviour and record each divergence with the behaviour that differed; expect the first findings around ADR 0001's transparent redirection — red → green → commit

## 7. Completion

- [ ] 7.1 Default run green: `make acceptance` passes with zero pending or undefined steps and an HTML report under `acceptance-tests/reports/`, and no conformance-tagged scenario is attempted or reported as a failure
- [ ] 7.2 Conformance run green: the tagged subset passes against Kind with an enforcing CNI, and the enforcement gate is verified in both directions against the non-enforcing cluster
- [ ] 7.3 Frozen-contract check: `internal/cluster/cluster.go` is unchanged by this change; every `/simctl/*` route is still unreachable under `--driver kube`
- [ ] 7.4 Verify the composition: `.extracted/` rebuilt, nothing loaded from `openspec/changes/archive/`, no duplicate scenarios across the two active changes, composition report clean
- [ ] 7.5 Resolve or explicitly carry forward design Open Questions 4 (conformance flake and retry policy) and 5 (egress proxy deployment shape under kube — sim correction or product change)
