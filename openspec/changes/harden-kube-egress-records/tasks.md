# Tasks: harden-kube-egress-records

Implementation MUST honour the in-force ADRs under `adr/`: 0001–0006 and 0008.
ADR 0007 is superseded by 0008 and is historical context only.

`acceptance-tests/` already exists (Python/behave, `stack: python` in
`openspec/config.yaml`), so the first-time setup section does not apply and is
omitted. New scenarios extend `acceptance-tests/steps/kube_driver_steps.py` and
`acceptance-tests/pages/kube.py`; step definitions stay free of selectors and
URLs.

Two standing constraints apply to every task below. `internal/cluster/cluster.go`
MUST NOT change (ADR 0008, frozen contract) — if a task appears to require it,
stop and raise a spec finding instead. `/simctl/*` MUST remain registered only
when the sim driver is constructed; no task may add a test-arrangement endpoint
reachable under `--driver kube`.

Every new scenario is `@conformance` and runs only under `make conformance`, so
`make acceptance` MUST stay green throughout. Each implementation task takes one
spec Rule red → green: run the conformance suite so its scenarios fail for the
right reason, implement until they pass, then commit.

## 1. Proxy retention and truncation

- [x] 1.1 kube-driver: the proxy's attempt record is bounded and its truncation is reported — add bounded retention with a monotonic dropped counter to `cmd/cloudbox-proxy/main.go`, change `/attempts` to an object carrying attempts plus the dropped count, and surface the discarded count through `EgressAttempts` — red → green → commit
- [x] 1.2 The collector accepts both the new object shape and the pre-change bare array, so a namespace sealed before the upgrade keeps reporting until its next re-seal (design Migration Plan). Verify against a proxy deployment left on the old image

## 2. Collection independent of inspection

- [x] 2.1 kube-driver: attempt collection does not depend on a sandbox being inspected — add the single control-plane collector iterating sealed sandboxes on an interval, folding records through the existing `cluster.EgressObserver` capability; `refreshBlockedEgressLocked` stays for read-time freshness but stops being the only path — red → green → commit
- [x] 2.2 Settle the collection interval against conformance runtime and API-server load, and record the value chosen with its reason. Resolves design Open Question 3

## 3. Restart detection and honest incompleteness

- [x] 3.1 Add the proxy's startup incarnation identifier to the `/attempts` response and carry it through `EgressAttempts`; give the proxy deployment the resource requests, limits and readiness probe that keep a restart an event rather than routine
- [ ] 3.2 kube-driver: unrecoverable record loss is surfaced, never absorbed — an incarnation change after a prior collection marks the sandbox's egress record incomplete, and the incompleteness reaches sandbox status and the run's evidence rather than lowering the violation count — red → green → commit

## 4. The admin surface

- [ ] 4.1 kube-driver: the proxy's attempt surface is reachable only by the control plane — create the per-namespace Secret at seal time, require the token on `/attempts`, and present it from the collector — red → green → commit
- [ ] 4.2 Publish the residual exposure honestly (ADR 0001): record in `hack/conformance/README.md` or the proxy's package doc that the admin-port ingress policy stays permissive by necessity and the token is the actual control, so nobody later reads the NetworkPolicy as the protection

## 5. Egress evaluation

- [ ] 5.1 kube-driver: egress evaluation reflects live cluster state — replace the unconditional `{Allowed: true, Via: "unfiltered"}` stub in `internal/cluster/kube/seal.go` with evaluation against the namespace's seal and live allowlist per design D1's table, and delete the stale "implemented by task 3.7" comment — red → green → commit
- [ ] 5.2 Confirm the bounded claim is stated where a reader will meet it: `AttemptEgress` evaluates policy, not a packet, and packet-level enforcement remains the probe's claim

## 6. Sim reconciliation

- [ ] 6.1 Confirm no sim divergence arises — the sim records attempts synchronously and cannot lose them, so no `internal/sim/DIVERGENCES.md` entry is expected. If implementation contradicts that expectation, record the divergence and correct `internal/sim/world.go` (conformance-ci, ADR 0008)

## 7. Completion

- [ ] 7.1 Default run green: `make acceptance` passes with zero pending or undefined steps, and no conformance-tagged scenario is attempted or reported as a failure
- [ ] 7.2 Conformance run green: `make conformance` passes against Kind with an enforcing CNI, and the enforcement gate still refuses the non-enforcing cluster in both directions
- [ ] 7.3 Frozen-contract check: `internal/cluster/cluster.go` is byte-identical to its pre-change state; every `/simctl/*` route is still unreachable under `--driver kube`
- [ ] 7.4 Verify the composition: `.extracted/` rebuilt, nothing loaded from `openspec/changes/archive/`, no duplicate scenarios between this change's delta and the `kube-driver` source-of-truth spec, composition report clean
- [ ] 7.5 Resolve or explicitly carry forward design Open Questions 1 (aggregation and the meaning of `evidence.EgressViolations`) and 2 (whether an incomplete record should refuse evidence rather than annotate it)
