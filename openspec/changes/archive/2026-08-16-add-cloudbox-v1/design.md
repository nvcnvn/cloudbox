# Design: add-cloudbox-v1

## Context

CloudBox is greenfield: this change establishes the v1 spec baseline from
`draft.md` (draft 5). There is no existing code, no existing specs, and no
in-force ADRs (`adr/` does not exist yet). The draft is the product of five
drafting rounds and has already resolved several architecture-level questions
(GitOps coexistence, egress proxy transparency, substrate lockfile scope);
this design records those resolutions as decisions so implementation changes
inherit them, and carries the still-open questions forward.

The product's shape: an add-on control plane (controller + five CRDs) beside a
team's existing Kubernetes resources, providing sealed production-shaped
sandboxes (L1), signed evidence checks on PRs (L2), gated GitOps write-back
promotion (L3), and a strict single-writer production mode (L4). Nothing below
L3 touches production write access.

Stakeholders: teams adopting AI coding agents (primary), Kubernetes teams
wanting per-developer environments and stronger PR checks, and later platform
teams wanting the audited write-path invariant.

## Goals / Non-Goals

**Goals:**
- Specify v1 so every capability is independently valuable and adoption is
  incremental (each ladder level is a complete product for some team).
- Make evidence honest by construction: machine-gathered, control-plane-signed,
  scoped to observed behavior, with everything unproven declared.
- Run on any conformant Kubernetes ≥ 1.29 with a NetworkPolicy-enforcing CNI —
  no vendor CNI, no mesh, no manifest language, no CD engine.
- Keep the team's existing flow the mechanism of record: their PRs, their
  branch protection, their Argo/Flux.

**Non-Goals (v1):**
- Replacing code review or CD; owning production RBAC below L4.
- Multi-namespace applications in the default mechanism (vcluster opt-up S4 is
  post-v1); linked dependency sandboxes (S8 full form is v1.x — v1 uses stubs).
- Load-realism claims from sandboxes (canary G10 is post-v1); production data
  values by default; in-house masking or rollout engines.
- Adversarial-hardening roadmap items (N8 v1.x): DNS anomaly detection,
  per-FQDN egress volume metering.

## Decisions

1. **Seal floor: standard NetworkPolicy v1 + product egress proxy** (over
   vendor policy CRDs, a service mesh, or eBPF-only enforcement). Portability
   is the wedge — the major managed offerings and default k3s already enforce
   NetworkPolicy v1. FQDN allowlisting, which NetworkPolicy cannot express, is
   done by the managed egress proxy; deny-all admits only cluster DNS and the
   proxy, so there is no route around it. Transparent pod-level redirection
   (iptables/DNS-interception helper) covers non-HTTP protocols and
   proxy-unaware workloads. When a DNS-aware CNI (e.g. Cilium) is detected the
   allowlist MAY compile to native policy — same contract, better mechanism.
   Because a non-enforcing CNI fails silently, enforcement is probe-verified
   at setup and sandbox creation (N7): never report sealed, never emit
   evidence, on an unverified seal.

2. **Bundle digest is the identity everywhere** (over git SHA or image tags).
   Diff, evidence, soak inheritance, promotion, and live-state verification
   all key on the content-addressed digest of rendered manifests — "the bytes
   that ran are the bytes that ship." This is what makes evidence transfer at
   merge sound (G7) and what makes write-back verifiable (G9). Consequence:
   renders must be deterministic, so intake fails non-deterministic renders
   rather than tolerate flapping digests.

3. **Recorded transforms instead of overlays.** The two legitimate per-
   environment differences that live in manifests — namespace and capacity —
   are controller-applied, digest-preserving, evidence-declared transforms
   (B3, S7), not user templating. Everything else environment-variant is
   confined to the four-kind boundary contract (C1–C3). Alternative rejected:
   an overlay/templating mechanism — it reintroduces the per-environment
   drift the product exists to eliminate, and growth of the variance list is
   the design's declared failure signal.

4. **Capacity squeeze semantics: CPU scales, memory floors, topology stays**
   (S7). CPU is compressible (starvation slows), memory is not (an OOM-killed
   quorum member proves nothing), and replica topology is where N>1 bugs
   live. Default `squeezed`; `minimal` shifts N>1 behavior to the future
   canary; workload-internal sizing (e.g. JVM `-Xmx`) is never rewritten —
   incompatibility surfaces as an explicit `capacity-squeeze-incompatible`
   diagnostic steering to `capacity: full`.

5. **Substrate lockfile scoped to referenced components** (P1, resolving
   draft-4 open question 5; over whole-cluster lockfiles). Whole-cluster
   digests would invalidate every application's evidence on every unrelated
   operator upgrade, destroying soak time for no signal. Drift detection (P4)
   inherits the same scope.

6. **L2 evidence check is the adoption wedge; write-back is the recommended
   L3.** The controller posts a signed status check; the team's own branch
   protection gates merge — no merge-rule ownership, no production access.
   At L3 the controller commits proven bundles to the GitOps repo and the
   team's Argo/Flux applies them, with completion gated on live-digest
   verification. Direct apply exists only as the L4 strict alternative.
   Alternative rejected: competing with CD — feeding it is the add-on thesis.

7. **Trust boundary: the control plane mints everything** (CP2/CP4). All
   validation, bundling, evidence gathering, and signing are server-side; the
   CLI is a thin client; CI is an untrusted trigger that can display but never
   assert evidence; local (Kind) sandboxes iterate but produce non-promotable,
   non-postable evidence because the user controls the cluster. This is what
   makes "0 forged evidence checks" an enforceable metric.

8. **Data: verify shape, declare grade, sequence thin clones before the
   synthetic generator.** Profiles capture schema + statistics without values
   leaving production (D1); every run declares per-datastore fidelity (D2)
   and policy sets minimums with conditional rules (D3). Thin-clone drivers
   (D7) ship before the profile-synthetic generator (D6, v1.x) because
   integrations deliver higher fidelity sooner than an in-house engine.
   Real-data levels are admin-enabled and agent-gated (D8).

9. **Draft SHOULDs carried as v1 spec requirements** — an explicit authoring
   assumption of this change: lockfile-driven provisioning (P3) and
   thin-clone drivers (D7) are graded SHOULD in the draft but are written as
   testable Rules in the delta specs, because the schema requires normative
   Rules and both are load-bearing for the success metrics (30s readiness;
   migration catch rate). Descoping either is a spec change, not an
   implementation choice. Graded MAYs (S4 vcluster, rendered-only intake B5,
   exec-disable policy, rollback fast-track, native-CNI compile) stay out of
   the Rules and live here.

10. **Containment honesty is a spec'd product surface** (§2.5, N8): the
    cooperative (reproducibility) guarantee is complete; the adversarial one
    is strong but bounded, with residual channels (DNS tunneling, allowlisted
    exfiltration) named in published material. "Unbypassable" is banned
    vocabulary. This protects the primary (agent-platform) audience from
    buying a claim the mechanism doesn't make.

## Risks / Trade-offs

- [Silent NetworkPolicy non-enforcement produces false "sealed" claims] ->
  Probe-verification at setup and per-sandbox creation (N7); a sandbox that
  fails the probe never reports sealed and never emits evidence.
- [Transparent redirection helper (iptables/DNS interception) varies across
  CNIs and container runtimes] -> Conformance matrix in CI across the major
  managed offerings + k3s + Kind; fall back to proxy env-vars for HTTP
  workloads; N7 probe catches broken redirection as an unsealed sandbox.
- [Digest-match evidence transfer fails frequently on busy repos (stale
  branches re-render differently)] -> Determinism enforced at intake; document
  "require branches up to date" branch protection as the cheap v1 answer;
  merge-queue integration tracked as an open question.
- [30-second sandbox readiness is hard with heavyweight operators in the
  substrate] -> Pre-warmed namespace pools and shared per-application operator
  installs (P3 provisioning); readiness metric explicitly excludes the user's
  own workloads.
- [Squeezed capacity breaks memory-sensitive workloads and erodes trust] ->
  Explicit `capacity-squeeze-incompatible` diagnostics with a documented path
  to `capacity: full`; autoscaler suspension recorded in evidence; performance
  claims never made below `full`.
- [Evidence read as "verified working" despite G6 wording] -> Single
  summary-renderer implementation owns the wording; scoped-claim language is
  spec'd (G6 Rule) and lint-tested in acceptance scenarios.
- [Residual adversarial channels (DNS tunneling, allowlisted exfiltration)
  used to discredit the seal] -> Scope published per §2.5/N8; v1.x roadmap
  (DNS anomaly detection, egress volume metering) prioritized by
  agent-platform procurement needs (open question).
- [Thin-clone fidelity depends on external vendors' branching APIs] -> Driver
  interface with per-vendor integrations; every other fidelity level works
  without any vendor; clone endpoints confined to the boundary contract.
- [Five-CRD surface tempts scope growth toward a platform] -> CP1 caps the
  CRD set; C3 makes variance-list growth a spec-change event, not a feature.

## Migration Plan

Greenfield — no existing users to migrate. Implementation sequencing follows
the adoption ladder so each shipped layer is independently valuable:

1. **L1 core**: CRDs + controller skeleton, bundle intake (+ offline `check`),
   sandbox lifecycle, seal + probe verification, substrate lockfile +
   verification, ergonomics (`logs/exec/port-forward`, `status --explain`).
2. **L2**: evidence assembly + signing, SCM integration (GitHub App first),
   PR-bound sandboxes, witnessed `test`, evidence checks.
3. **L3**: promotion requests, approval policy, synchronous audit, write-back
   + live-digest verification, rollback.
4. **L4**: strict-mode RBAC, break-glass, divergence detection.
5. **Data fidelity** lands incrementally beside 1–3: profiles + fidelity
   declaration first (D1/D2), policy minimums + migration replay (D3/D4),
   thin-clone drivers (D7) as integrations mature.

Rollback strategy per adopting team is inherent to the ladder: levels are
per-application policy, so any application can step down a level without
migration; below L4 the product can be removed without touching production.

## Open Questions

1. Evidence retention and GC policy for bundles, evidence, and data profiles
   (draft §10.3) — audit artifacts need a declared lifecycle.
2. Rendered-only intake vs native Helm support in v1 (draft §10.4, B5 MAY).
3. Merge-queue scope: which SCM providers get merged-tree runs in v1, and is
   digest-match + "require branch up to date" acceptable without queue
   integration (draft §10.6)?
4. Data profile semantics beyond SQL — Kafka topics, object stores, caches
   (draft §10.7).
5. Canary evidence grade for post-v1 G10: which analysis providers and
   metrics count, and how canary policy is declared (draft §10.8).
6. N8 hardening priorities: which adversarial mitigations land first, driven
   by agent-platform procurement requirements (draft §10.10).
7. Evidence-check UX: one status check vs per-fact checks, and stale-evidence
   rendering per SCM provider (draft §10.11).
8. Confirm the SHOULD-as-MUST carries (Decision 9: P3 provisioning, D7 thin
   clones) with the product owner; if either is descoped, the corresponding
   delta spec Rule must be revised before implementation.
9. Kube-driver conformance — **resolved**: every v1 contract here is verified
   only against the sim driver (ADR 0007), and `--driver kube` is
   unimplemented. That gap is not descoped, it is sequenced: the `kube` driver
   and a tagged cluster-effect conformance run on Kind + an enforcing CNI are
   tracked by the `add-kube-driver-conformance` change. The managed-offering
   half of the Risks conformance matrix (EKS/GKE/AKS) stays post-v1 and does
   not gate v1 GA; sim-verified contracts plus the N7 probe (which refuses to
   report sealed on a non-enforcing CNI) carry v1.
