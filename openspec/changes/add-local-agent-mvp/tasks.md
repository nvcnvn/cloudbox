# Tasks: add-local-agent-mvp

Implementation MUST honour the in-force ADRs under `adr/`: 0001–0006, 0008, and
the two this change records — 0009 (unimplemented integrations refuse rather
than simulate) and 0010 (the released substrate is a user-controlled cluster).
ADR 0007 is superseded by 0008 and is historical context only.

`acceptance-tests/` already exists (Python/behave, `stack: python` in
`openspec/config.yaml`), so the first-time setup section does not apply and is
omitted. New scenarios extend the existing harness: step definitions land
beside their capability's file under `acceptance-tests/steps/`, and the
command-line surface gets its own page object (`acceptance-tests/pages/cli.py`)
alongside the release-claim assertions (`acceptance-tests/pages/release.py`).
Step definitions stay free of selectors, URLs, and command strings.

Four standing constraints apply to every task below.

1. `internal/cluster/cluster.go` MUST NOT change (ADR 0008, frozen contract).
   If a task appears to require it, stop and raise a spec finding.
2. `/simctl/*` MUST remain registered only when the sim driver is constructed.
   No task may add a test-arrangement endpoint reachable under `--driver kube`.
3. The CLI MUST NOT link `internal/core` (ADR 0004). It starts a control plane;
   it never becomes one.
4. Specs-zone and code-zone edits are committed separately. Every task below is
   code unless it says otherwise.

Each implementation task takes one spec Rule red → green: run the suite so its
scenarios fail for the right reason, implement until they pass, then commit.
`make acceptance` MUST stay green throughout.

## 1. Honesty first — nothing is packaged until these land

These precede all packaging work (design Migration Plan step 1) so that no
published artifact ever carries a substrate-match default or an integration
that simulates.

- [ ] 1.1 substrate-parity: parity with no registered production is unverified, never matched — replace the `SubstrateMatch bool` default at `internal/core/evidence.go:144` with a three-state parity value (verified match / mismatch / unverified), produce `unverified` when `productionCluster()` reports none, and confirm unverified does NOT invalidate evidence (P2 invalidates on mismatch only) — red → green → commit
- [ ] 1.2 Extend the summary renderer for the unverified case so it states parity was not compared for want of a registered production substrate, and never renders "on a substrate matching production" — verify the `evidence` capability's honest-wording scenarios still pass unchanged
- [ ] 1.3 Audit every consumer of the parity value — evidence validity, the evidence check's preconditions, `status`, and the promotion gate — so none reads unverified as a match. A boolean-shaped read that silently coerces is the defect this task exists to prevent
- [ ] 1.4 release-integrity: an operation with no implemented integration refuses and records nothing — introduce the integration-presence predicate (ADR 0009: conditioned on the integration's absence, NOT on the driver name, so the sim suite's arrangement surfaces constitute presence) and apply it to promotion, evidence-check posting, and production-state ingest; each refuses naming the missing integration and writes no record — red → green → commit
- [ ] 1.5 Verify the refusals are total: after a refused promotion, listing promotions returns nothing; after a refused check, the PR carries no check; after refused production ingest, no production state exists. A refusal that still mutates state is the same defect in a quieter form
- [ ] 1.6 sandbox-seal: containment claims match the declared threat-model scope (MODIFIED) — extend `/v1/containment-statement` so the published claim names which egress is recorded by destination and which is denied without one, per divergence 4; the existing blocked/residual channel assertions MUST keep passing verbatim — red → green → commit

## 2. The released substrate (ADR 0010)

- [ ] 2.1 sandbox-lifecycle: a sandbox host that cannot be shown to enforce NetworkPolicy is refused — add the enforcement probe to cluster registration, reusing the `conformance-ci` precondition mechanism rather than inventing a second one; refusal names unproven enforcement and the cluster is not accepted as a host — red → green → commit
- [ ] 2.2 Resolve design Open Question 1 before 2.1 is complete: whether the registration probe needs its own scratch namespace (there is no sandbox yet) and, if so, what cleanup it guarantees on failure. Record the answer and its reason
- [ ] 2.3 sandbox-lifecycle: user-controlled clusters are registrable sandbox hosts — register a user-controlled cluster as a host, mark its sandboxes' evidence user-controlled, and refuse checks and promotions from that evidence (ADR 0004). Seal and iteration semantics MUST be identical to a managed host — red → green → commit
- [ ] 2.4 Retire the removed rule's code path: delete `provisionLocalCluster`'s off-contract `NewCluster` type assertion in `internal/core/substrate.go:208` and the `CreateSandboxOptions.Local` provisioning branch in `internal/core/sandboxes.go:150`. The user-controlled *property* survives on the registered-host path; only the product-provisions-Kind mechanism goes
- [ ] 2.5 Promote `hack/conformance/kind-enforcing.sh` to a user-facing bootstrap outside the conformance directory, so the documented path to a suitable cluster is a supported script rather than a test fixture. It stays outside the product boundary (ADR 0010)
- [ ] 2.6 Reconcile the sim: the sim driver's `NewCluster(name, enforcing, userControlled)` is how the suite arranges enforcing and non-enforcing hosts. Confirm registration-time refusal behaves identically on both drivers, and record any divergence in `internal/sim/DIVERGENCES.md` (conformance-ci, ADR 0008)

## 3. The command line becomes the product

- [ ] 3.1 control-plane: the command line may start and reuse a local control plane — start `cloudboxd` when none is addressed, reuse it across commands, and let an explicit `--server` always win. Enforcement stays server-side; the CLI links no `internal/core` — red → green → commit
- [ ] 3.2 Record the daemon's build version in the local state file so a stale daemon from an older binary is replaced rather than reused. Resolve design Open Question 3 first: whether that file may also record registered clusters, or whether that crosses into the durable-state decision this change defers. Draw the line and state it
- [ ] 3.3 developer-ergonomics: at the sandbox level the command line carries the whole loop — add the missing verbs to `cmd/cloudbox/` (declare application and contract, create sandbox, apply, run the declared suite, read evidence, destroy) alongside the existing `check`, `status`, `logs`, `exec`, `port-forward` — red → green → commit
- [ ] 3.4 developer-ergonomics: command output is available as data and outcomes carry distinct exit statuses — add the structured rendering as a projection of the same server responses the human rendering uses (design D7: one surface, two projections), and give success, product refusal, and unreachable control plane distinct exit statuses — red → green → commit
- [ ] 3.5 Verify the equivalence the spec asserts: for a sandbox with recorded blocked egress, the structured and human renderings carry the same facts. A structured rendering that carries *more* than the human one has become a second surface and must be corrected, not documented

## 4. Release engineering

- [ ] 4.1 Resolve design Open Question 2 before any of 4.2–4.4: which registry publishes the egress proxy image, digest-pinned versus version-tagged, and multi-architecture coverage. The last one bites immediately — `hack/conformance/build-proxy-image.sh` already selects `arm64` or `amd64` per node. Record the decision with its reason
- [ ] 4.2 release-integrity: the egress proxy is deployed from a pinned, resolvable reference — replace `proxyImage()`'s `cloudbox-egress-proxy:dev` in `internal/cluster/kube/proxy.go:26` with the published pinned reference, and add the release job that builds and pushes it — red → green → commit
- [ ] 4.3 release-integrity: a sandbox whose proxy cannot be provisioned is never reported as allowlist-enforcing — report the proxy unavailable naming the unresolvable reference, keep allowlisted egress failing closed, and never report the sandbox as enforcing its allowlist. Default-deny still holds without the proxy; the allowlist does not — red → green → commit
- [ ] 4.4 release-integrity: the released binaries identify the build they came from — stamp the version at build time on both binaries, replacing the hardcoded `"cloudbox v1 (development)"` in `cmd/cloudbox/main.go`, and make a locally built binary identify itself as unreleased — red → green → commit
- [ ] 4.5 release-integrity: the release declares its required substrate and the trust it does not provide — write the quickstart naming the enforcing-CNI requirement and stating plainly that the control plane authenticates no one and is not a trust boundary between users — red → green → commit
- [ ] 4.6 Replace the two-line `README.md` with the front door: what the product is, the sealed-iteration loop an agent runs, the honest containment claim from 1.6, and a link to the quickstart. State the evidence retention policy explicitly (design Open Question 5: evidence dies with the daemon, which is a policy and should read as one)

## 5. Completion

- [ ] 5.1 Resolve design Open Question 4: decide which of this change's rules belong in the `@conformance` subset — registration-time enforcement refusal (2.1) is the strongest candidate, since CI already stands up a deliberately non-enforcing cluster. Tag accordingly and add matching rows to `acceptance-tests/CONFORMANCE.md`, keeping its recorded-exclusion table accurate
- [ ] 5.2 Default run green: `make acceptance` passes with zero pending or undefined steps, and no conformance-tagged scenario is attempted or reported as a failure
- [ ] 5.3 Conformance run green: `make conformance` passes against Kind with an enforcing CNI, and the enforcement gate still refuses the non-enforcing cluster in both directions
- [ ] 5.4 Frozen-contract and boundary check: `internal/cluster/cluster.go` is byte-identical to its pre-change state, every `/simctl/*` route is still unreachable under `--driver kube`, and `cmd/cloudbox/` imports no `internal/core` package
- [ ] 5.5 Verify the composition: `.extracted/` rebuilt, nothing loaded from `openspec/changes/archive/`, no duplicate scenarios between this change's deltas and their source-of-truth specs, composition report clean
- [ ] 5.6 Walk the release as a stranger would: from a machine with no cluster, follow the quickstart verbatim through the whole loop to signed evidence. Every step that required knowledge not in the quickstart is a defect in the quickstart, not in the walker
