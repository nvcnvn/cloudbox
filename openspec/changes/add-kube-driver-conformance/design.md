## Context

CloudBox v1 shipped with 102 green acceptance scenarios, all of them driven
against `internal/sim/world.go` — 284 lines modelling the Kubernetes semantics
that roughly 4,500 lines of control plane depend on. ADR 0007 made that choice
deliberately and stated its own bound: "the sim driver is load-bearing test
infrastructure: its fidelity to Kubernetes semantics bounds what the suite can
prove." This change tests that bound for the contracts where a real cluster can
prove something a simulation cannot.

Current state that shapes the work:

- `internal/cluster/cluster.go` is a 20-method contract (`Driver` plus
  `Cluster`). The sim driver implements it; nothing else does.
- `cmd/cloudboxd/main.go:25` fails fatally on `--driver kube`.
- `/simctl/*` is 17 routes registered in `internal/server/server.go` only when
  the sim driver is constructed, used from 34 call sites in the suite.
- `acceptance-tests/environment.py` boots `cloudboxd --addr … --driver sim`
  once per suite and resets state per scenario via `POST /simctl/reset`.
- `behave.ini` configures formatters and `show_skipped`, but no tag filtering.
- `.gherkin-lintrc` already enables `no-duplicate-tags`,
  `no-partially-commented-tag-lines`, and `one-space-between-tags`, so the
  toolchain anticipates tags even though no spec has used one yet.
- Nothing runs in CI. There is no `.github/` directory.

In-force ADRs (all seven accepted, none superseded): 0001 NetworkPolicy floor
with egress proxy, 0002 bundle digest identity, 0003 boundary contract, 0004
control plane mints evidence, 0005 adoption ladder, 0006 application-scoped
substrate lockfile, 0007 Go control plane with simulated driver. 0001 and 0007
constrain this design directly.

## Goals / Non-Goals

**Goals:**
- A `kube` driver satisfying the existing cluster contract against real
  Kubernetes, with the contract unchanged.
- Real-cluster proof of the one claim the sim cannot make: that a sandbox
  reported sealed is sealed by a CNI that actually enforces NetworkPolicy.
- A conformance run that is meaningful or loudly broken, never vacuous.
- The first CI in this repository, covering build, vet, spec lint, the full sim
  suite, and the conformance subset.
- A standing rule for correcting the sim when the real driver contradicts it.

**Non-Goals:**
- Managed offerings (EKS/GKE/AKS) and k3s — credentials and per-run budget put
  them post-v1; `add-cloudbox-v1`'s full conformance matrix stays open.
- Real GitOps, SCM, audit sink, production state, or datastore backends.
- Porting `boundary-contract`, `bundle-intake`, `substrate-parity`, or
  `promotion` scenarios to the real driver.
- Soak-window conformance.
- Any change to a v1 requirement.

## Decisions

1. **New capabilities rather than modified v1 specs.** `openspec/specs/` is
   empty because `add-cloudbox-v1` is unarchived, so its capabilities are still
   active deltas. Declaring `sandbox-seal` modified here would put two active
   changes' deltas over one capability, which the effective-spec composition
   surfaces as duplicate scenarios — the condition `add-cloudbox-v1`'s task 9.2
   exists to catch. *Alternative considered:* tag scenarios inside
   `add-cloudbox-v1`'s specs so the same scenarios run twice. Rejected on two
   counts: it edits another active change's artifacts, and those scenarios
   arrange through `/simctl` and therefore cannot run under `kube` unmodified
   anyway. The conformance claim is genuinely a different claim — "the real
   driver enforces what the sim models" — and deserves its own requirements.

2. **Conformance arranges through the product's own surface, not a neutral test
   surface.** `AttemptEgress` is already a method on `cluster.Cluster`;
   `/simctl/sandboxes/{s}/egress-attempts` is merely a sim-only HTTP exposure of
   it. Under `kube` the equivalent is a real connection from a real pod, and the
   driver-neutral *observable* already exists in the product: N4 records blocked
   egress with destination, timestamp and attribution, and X2 renders it through
   `status --explain`. So conformance scenarios deploy real workloads and assert
   on product output. *Alternative considered:* a driver-neutral `/testctl/*`
   arrangement surface. Rejected — it recreates exactly the coupling ADR 0007
   confined to sim, and would ship test endpoints in the production binary.

3. **Time control: real clock with short durations; soak excluded.** The code
   already permits this honestly. `TTLSeconds` (`internal/core/sandboxes.go:49`)
   and `IdleExpirySeconds` (`internal/core/core.go:53`) are second-granular, and
   the clock is injected — `core.New(driver, now func() time.Time)` at
   `internal/core/core.go:158`. The kube driver passes `time.Now`, and a
   five-second TTL yields real expiry in about five seconds. Soak is specified
   in hours (S6 preserves three accumulated hours across a rebase) and cannot be
   real-clocked in CI, so it stays simulated and is excluded with the reason
   recorded. *Alternatives considered:* keep the simulated clock for conformance
   — rejected, a result obtained by moving a fake clock is not conformance;
   real-time soak — rejected, hours of CI wall-clock per run.

4. **Prove enforcement in both directions, using two Kind clusters.** A green
   seal run on a cluster that ignores NetworkPolicy proves nothing, so the
   subset asserts both that an enforcing CNI seals and that a non-enforcing one
   is caught. That needs two clusters in CI: one with an enforcing CNI, and one
   deliberately non-enforcing for the probe-failure scenario. Kind makes both
   cheap and credential-free. *Alternative considered:* trusting the enforcing
   cluster alone. Rejected — it leaves the most valuable assertion (that the
   product refuses to lie) untested on real infrastructure.

5. **Tag-based selection with an inverted default.** Conformance scenarios carry
   `@conformance`; the default run excludes it so a developer with no cluster is
   never blocked, and a conformance mode selects only it and boots `cloudboxd`
   with `--driver kube`. `behave.ini` gains the default exclusion and
   `environment.py` gains a driver mode; the wrapper keeps its
   `OPENSPEC_ACCEPTANCE` tripwire. `kube-driver`'s scenarios all require a
   cluster, so that file is tagged at the Feature level; `conformance-ci` mixes
   real-cluster and default-run scenarios and is tagged per scenario.

6. **The contract is frozen, and that is checked mechanically.** The temptation
   under a real driver is to widen `cluster.go` for convenience. Implementation
   treats `internal/cluster/cluster.go` as unchanged and verifies it with a diff
   check, so widening becomes a visible failure rather than a quiet slide. Where
   real Kubernetes genuinely cannot satisfy a method as written, that is a
   finding for the specs, not a signature change.

7. **Sim corrections are recorded, not silent.** ADR 0007 asserted the
   obligation; `conformance-ci` now specifies it. Each divergence found is
   corrected in `internal/sim/world.go` and recorded with the behaviour that
   differed, because a silent correction hides the fact that the previously
   green suite was proving something slightly wrong.

## Risks / Trade-offs

- [Kind's default CNI may not enforce NetworkPolicy, making a Kind seal test
  silently vacuous] -> Install an enforcing CNI explicitly and never rely on the
  default; verify the pinned Kind version's behaviour rather than assuming it.
  The enforcement precondition (`conformance-ci`) fails the run when enforcement
  is unproven, so the vacuous case cannot report a pass. Usefully, the same
  uncertainty is what the non-enforcing cluster in Decision 4 deliberately
  exploits.
- [Transparent pod-level redirection (ADR 0001) is the most CNI-sensitive
  mechanism in the product and the likeliest place the sim is wrong] -> Expect
  the first sim corrections here; ADR 0001 already provides the proxy env-var
  fallback for HTTP workloads, and N7's probe degrades an unredirected sandbox
  to unsealed rather than passing it.
- [Real-cluster scenarios are inherently slower and flakier than simulated ones]
  -> Keep the subset small and its waits in seconds; no silent retries or
  quiet truncation — a skipped or dropped scenario is reported.
- [Two Kind clusters roughly double CI setup cost] -> The non-enforcing cluster
  serves exactly one scenario and needs no CNI installation, so it is the
  cheaper of the two.
- [The sim suite stays the primary signal, so a divergence could sit unnoticed
  between conformance runs] -> Conformance runs on every pull request, not on a
  schedule, so divergence surfaces at the change that introduces it.
- [A frozen interface may prove genuinely insufficient for real Kubernetes] ->
  That outcome is a spec finding and pauses implementation for a spec revision,
  which is the intended behaviour rather than a failure mode.

## Migration Plan

No data or user migration — this is additive test infrastructure plus a driver
that was previously a fatal error. Sequencing:

1. **Tagging and selection first**, before any driver exists: add the tag
   vocabulary, the default exclusion, and the conformance mode. This restores a
   green default run while the new scenarios sit excluded, so the repository is
   never left red for structural reasons.
2. **CI for what already exists**: build, vet, lint-specs, sim suite. Immediate
   value, independent of the driver.
3. **The kube driver**, method group by method group against the frozen
   contract: namespaces and raw apply, then substrate reads, then workloads and
   readiness, then sealing, probing and egress.
4. **Kind clusters in CI** — enforcing, then the non-enforcing one.
5. **Real-clock lifecycle** (TTL, idle) last, since it depends on a working
   driver.

Rollback: the conformance job can be disabled independently of everything else,
and `--driver kube` returning to a fatal error restores exactly the v1 posture.
The default sim run never depends on any of it.

## Open Questions

1. **ADR 0007 needs superseding.** Its final consequence reads "The kube driver
   is deliberately unimplemented until the sim-verified contracts hold;
   `cloudboxd --driver kube` fails loudly today." This change makes that false.
   The `adr` step should record a new ADR superseding 0007 rather than editing
   it — carrying forward everything else in 0007, which remains correct.
2. **Calico or Cilium?** Cilium offers DNS-aware policy, which ADR 0001 names as
   a MAY for compiling the allowlist to native policy; choosing it might make
   that MAY cheap to explore later, at the cost of a heavier CI install than
   Calico.
3. **How is the non-enforcing cluster built** — stock Kind with its default CNI
   (if that version does not enforce), or an enforcing CNI with policy
   deliberately disabled? The second is more explicit and less
   version-dependent, but more setup.
4. **Flake policy for the conformance job**: does a real-cluster scenario get a
   retry, and if so, is the retry recorded so a flaky pass is not read as a
   clean one?
5. **Does the egress proxy need a different deployment shape under `kube`** than
   the sim models, and if so is that a sim correction or a product change?
