# CloudBox — product specification (draft 4)

One-line pitch: **sealed sandboxes for the Kubernetes manifests you already
have, and a production gate that approves evidence instead of speculation.**

Status: draft 4 for review, 2026-08-14. Changes from draft 3: first-class
inter-application dependencies via linked sandboxes (S8, C1, §9.9 resolved),
traffic & synthetic test execution witnessing in promotion evidence (G3, G6),
non-HTTP transparent egress proxy ergonomics (N3, §9.2 resolved), PR rebase
soak time inheritance (S6, G7), capacity squeezing diagnostic hints (S7),
and interactive egress allowlist discovery (CLI, §5.7). Prior requirement IDs
are preserved.

---

## 1. The two problems

### Problem 1 — the review gate is in the wrong place

Every mainstream infrastructure workflow puts human review *before the work
runs*. A change is written, reviewed, approved — and only then applied, which
is the first time anyone learns whether it actually works. The consequences:

- Reviewers approve **speculation**: a diff nobody has executed.
- Iteration speed is capped by approval latency, so engineers batch changes,
  which makes each review bigger and worse.
- The failure surfaces *after* the approval — in staging on Thursday, in
  production on Friday — where it is most expensive.

The gate is not the mistake; its **position** is. Review should happen after
the change has demonstrably run somewhere real, and should approve that
demonstration.

### Problem 2 — no environment is actually reproducible

Watch how a feature gets built today, by a developer or an AI coding agent:

1. Stand up local infra (usually docker-compose) that only *resembles*
   production.
2. Build and test against that approximation.
3. Move to dev and discover the differences. Then staging — different again.
4. Finally touch production, where the last surprises live.

Every hop is a translation between environments, and every translation is a
place to fail. Teams try to fix this with more IaC discipline, but IaC
verifies what was *written*, not what the workload actually *depends on*:
the RDS endpoint in an env var, the undeclared third-party API, the operator
version that differs between staging and prod. Nothing in the toolchain can
answer the question that matters: **"does this workload depend on anything my
next environment doesn't have?"**

The rise of AI coding agents makes both problems acute. Agents iterate faster
than any approval process and need somewhere they can safely break things —
isolated, contained, with no write path to production and every promotion
audited.

## 2. The thesis

Both problems have the same root: environments are opaque, so we substitute
process (upfront review, environment ladders) for proof. The product makes
the proof cheap:

1. **Every change runs first in a sandbox that is sealed** — deny-all
   networking with an explicit allowlist. If the workload runs correctly in
   a sealed sandbox, then every code path it exercised did so with zero
   undeclared external dependencies. The proof is scoped to observed
   behavior — a path never exercised in the sandbox (the nightly job, the
   error handler) is not proven — which is why evidence records what ran,
   what tests/traffic were witnessed, and for how long, never more. That
   scoped property is what makes "it worked here" transferable to "it will
   deploy there."
2. **The sandbox's substrate is verified against production's** — same CRDs,
   operators, admission policies, same versions — so "production-shaped" is
   a checked fact, not a hope.
3. **Production changes only through a promotion request** that carries the
   exact manifests that ran, plus machine-gathered evidence: seal held,
   zero egress violations, substrate hash matched, observed test/traffic
   activity, linked dependency digests matched, and workloads healthy for N
   minutes. Reviewers approve a demonstration.
4. **What the sandbox cannot prove is declared, not hidden.** The seal is a
   closed-world check over exercised behavior; data, load, unexercised code
   paths, and external authorization (the seal proves the workload *reached*
   an allowlisted endpoint, not that production's credentials are authorized
   there) are open-world. Evidence states the data fidelity level the run
   had (§5.9), witnessed traffic/test coverage, and the capacity mode (S7),
   and residual risk is bounded by a canary stage in production (G10) —
   never waved away.

No new manifest language, no wrapper abstraction, no service mesh, no
specific CNI — only standard NetworkPolicy enforcement, which the seal
verifies rather than assumes (N7). Users bring plain Kubernetes YAML (or the
rendered output of Helm or Kustomize). The product adds a control plane *beside* their resources, never
a schema *above* them.

```
        DEVELOP (unblocked)                    SHIP (gated)
  ┌────────────────────────────┐      ┌───────────────────────────┐
  │  sealed sandbox            │      │  promotion request         │
  │  ┌──────────────────────┐  │      │   bundle digest            │
  │  │ your manifests,      │  │      │   normalized diff          │
  │  │ verbatim             │  │ ───▶ │   evidence:                │
  │  └──────────────────────┘  │      │    seal held ✓             │
  │  deny-all + allowlist      │      │    egress violations: 0    │
  │  substrate == prod (hash)  │      │    substrate match ✓       │
  │  apply/break/fix freely    │      │    traffic/tests: 84/84 ✓  │
  └────────────────────────────┘      │    deps: auth@prod-match ✓ │
       no approvals here              │    data: profile-synthetic │
                                      │    capacity: squeezed      │
                                      │    healthy for 2h14m       │
                                      │   → approve → controller   │
                                      │     applies same bytes     │
                                      └───────────────────────────┘
```

When promotion is bound to a pull request (G7), review latency stops being
waste: the sandbox lives for the PR's lifetime, so the hours a change spends
waiting for review accumulate as observed-healthy soak time in the evidence.
The cost of problem 1 becomes the fuel for problem 1's fix.

## 3. Who it's for

- **Teams adopting AI coding agents** who need a contained execution
  environment shaped like production: isolated, egress-locked, no write path
  to production, every promotion audited. Plain Kubernetes YAML is the
  format every model already knows — no proprietary schema to teach. Agents
  already work in pull requests; a PR that carries proof it ran sealed is a
  stronger artifact than any approval UI.
- **Teams already running Kubernetes** with existing manifests or Helm
  charts, who want per-developer environments and a safer path to production
  without migrating to a platform abstraction. Value must arrive in the
  first afternoon, on one app, with nothing rewritten.

Explicit non-target (v1): teams wanting a PaaS to assemble their stack for
them. This product gates and reproduces the stack you have; it does not
choose one for you.

## 4. Concepts

| Term | Meaning |
|---|---|
| **Application** | The policy boundary: owners, approvers, boundary contract, quotas, and declared upstream dependencies. One application = one namespace per environment. |
| **Bundle** | A content-addressed set of rendered, environment-agnostic manifests. The unit of apply, diff, and promotion — the bytes that ran are the bytes that ship. |
| **Sandbox** | A disposable, sealed, substrate-verified environment owned by one developer or agent. |
| **Linked sandbox** | A secondary, minimal sandbox provisioned or attached from an upstream Application's bundle digest to satisfy an inter-application dependency contract under strict namespace-peering NetworkPolicy. |
| **Seal** | Default-deny ingress/egress with an explicit FQDN allowlist, enforced for the sandbox's entire lifetime. The reproducibility *verifier*. |
| **Substrate** | Everything a bundle assumes exists: CRDs, operators, admission policies, gateway classes, versions. Captured in a lockfile; parity is hash-checked. |
| **Data profile** | A content-addressed statistical profile of production's data *shape* — schema digest, null rates, cardinalities, length and character distributions — captured without production values leaving production. The data analog of the substrate lockfile. |
| **Fidelity level** | The declared grade of a sandbox's data realism: `fixtures` → `schema-replay` → `profile-synthetic` → `masked-snapshot` → `live-clone`. Recorded per datastore in evidence; policy sets minimums. |
| **Exercised activity** | Machine-recorded test runs and synthetic/real traffic events proving that workload paths were actively executed under the seal rather than merely idling. |
| **Boundary contract** | The declared, finite set of values allowed to differ per environment: secret values, ingress hostnames, egress allowlist, and internal Application dependencies. Nothing else varies. |
| **Promotion** | An approval-gated request to apply a bundle to production, carrying the sandbox run's evidence. The only write path to production. May be bound to a pull request (G7). |

## 5. Requirements

Keywords **MUST / SHOULD / MAY** per RFC 2119. IDs are stable for review.
Requirements tagged **(v1.x)** are committed direction, not v1 scope.

### 5.1 Bundles (`B`)

- **B1.** `apply` MUST accept plain multi-document Kubernetes YAML of any
  kind — built-in or custom — with no product-specific schema or annotations
  required.
- **B2.** Every apply MUST produce a content-addressed **bundle** (manifest
  set + digest), recorded server-side. The digest is the identity used by
  diff and promotion.
- **B3.** Bundles MUST be environment-agnostic. Rejected at apply time:
  `metadata.namespace` on any manifest (namespaces are assigned per
  environment), cluster-scoped resources (substrate, not bundle — relaxed
  only under the vcluster mechanism, S4, where the sandbox owns a virtual
  cluster scope), and — best-effort lint — hardcoded cross-namespace/FQDN
  service references. The seal, not the lint, is the enforcement of last
  resort.
- **B4.** In-bundle service references MUST use same-namespace short names
  (`http://auth-api:8080`) or declared boundary contract alias hostnames (C1).
  Consequence: an application occupies exactly one namespace in every
  environment (see S4 for larger topologies; see S8 for inter-application
  dependencies).
- **B5.** Helm/Kustomize users are supported by rendering first; the bundle
  is always the rendered output. v1 MAY ship rendered-only intake.

### 5.2 Sandboxes (`S`)

- **S1.** Any developer or agent MUST be able to create a sandbox with one
  command and **no approval**. RBAC MUST restrict modification to the
  sandbox's owner.
- **S2.** Default mechanism: sealed namespace per sandbox on a shared
  cluster. Control-plane target: sandbox sealed and ready for `apply` in
  **≤ 30 seconds**; total readiness dominated only by the user's own
  workloads.
- **S3.** Local mechanism: a single command MUST provision a local (Kind)
  cluster from the application's substrate lockfile that behaves as a valid
  sandbox — same seal, same iteration semantics. Evidence from a local
  sandbox is **non-promotable**: the cluster is user-controlled, so its
  evidence is forgeable (CP4's threat model). Local runs are for iteration;
  `promote` MUST require evidence from a control-plane-managed sandbox.
- **S4.** Opt-up mechanism (MAY be post-v1): one virtual cluster per sandbox
  for bundles needing multiple namespaces or cluster-scoped resources; the
  seal applies at the host boundary.
- **S5.** Sandboxes MUST support TTL and idle expiry, and per-application
  resource quotas.
- **S6.** When the SCM integration is enabled, sandbox lifecycle MUST bind
  to the pull request: created on PR open, bundle re-rendered and re-applied
  on every push, TTL fired on PR close or merge. One branch = one sandbox.
  Observed-healthy duration accumulates only while the bundle digest is
  unchanged — a push that changes the digest resets the soak clock. If a PR
  rebase or upstream merge yields an identical rendered bundle digest,
  accumulated soak time MUST be preserved (**soak inheritance**).
- **S7.** Bundles carry production capacity (replicas, resource requests,
  PVC sizes) and MUST NOT be hand-edited to fit sandbox quotas. To make
  prod-sized bundles schedulable under S5 quotas, the controller MAY apply
  a **recorded capacity transform** at sandbox admission — bundle bytes and
  digest unchanged, mode declared in evidence:
  - `capacity: full` — no transform.
  - `capacity: squeezed` (default) — replica counts, topology, and
    scheduling constraints preserved; CPU requests/limits scaled by a
    product-defined factor (CPU is compressible: starvation slows, never
    kills); memory scaled only down to a per-container floor (memory is
    incompressible: an OOM-killed quorum member proves nothing). Keeps
    N>1 logic — quorum, leader election, rolling updates, PDBs — in the
    sandbox at reduced performance.
  - `capacity: minimal` — replicas floored to 1, requests scaled. The
    cheapest mode; N>1 behavior shifts entirely to the canary (G10).

  The controller MUST NOT rewrite workload-internal sizing (e.g. a JVM
  `-Xmx` env var); a workload that cannot survive squeezing surfaces as
  unhealthy. The controller and CLI MUST surface memory exhaustion or OOMKills
  as explicit `capacity-squeeze-incompatible` diagnostics to guide operators
  to configure `capacity: full`. Performance claims are never made below
  `full` (G6, G10).
- **S8.** When an application declares inter-application dependencies (C1),
  the controller MUST support provisioning or attaching **linked sandboxes**
  running the dependency application's bundle (defaulting to its
  `production-current` bundle digest). NetworkPolicy MUST strictly restrict
  cross-namespace traffic between the paired sandbox namespaces on declared
  ports. Linked sandboxes default to `capacity: minimal` (S7) unless
  configured otherwise.

### 5.3 The seal (`N`)

- **N1.** A sandbox MUST be sealed **before** any user workload is admitted:
  default-deny ingress and egress.
- **N2.** Egress MUST be limited to in-sandbox services, cluster DNS, linked
  sandbox dependencies (S8), and FQDN entries on the application's declared
  allowlist.
- **N3.** The seal's enforcement floor is **standard Kubernetes
  NetworkPolicy v1** — no vendor policy CRDs, no specific CNI. Default-deny
  is expressed as NetworkPolicy whose only admitted egress is cluster DNS
  and the product-managed egress proxy; the proxy enforces the FQDN
  allowlist. For non-HTTP protocols (raw TCP, gRPC, database wire protocols)
  or workloads not honoring proxy environment variables, the proxy provides
  transparent pod-level redirection (via local iptables or DNS interception
  helper) so sealing is unbypassable and transparent without workload
  modification. When a DNS-aware CNI (e.g. Cilium) is detected, the
  controller MAY compile the allowlist to native policy instead — same
  contract, better mechanism.
- **N4.** Every blocked egress attempt MUST be recorded with destination and
  timestamp, attributed to the sandbox, and surfaced in `status` and in
  promotion evidence.
- **N5.** The seal MUST NOT be weakened per-sandbox. Allowlist changes are
  application-policy changes, owned by admins and audited.
- **N6.** User-authored NetworkPolicy inside a bundle MAY further restrict,
  and MUST NOT widen, the seal.
- **N7.** A cluster whose CNI does not enforce NetworkPolicy fails
  *silently* — policies are accepted and ignored. `setup` and sandbox
  creation MUST probe-verify enforcement (create a canary workload, confirm
  a denied connection is actually denied). A sandbox on a non-enforcing
  cluster MUST NOT report itself sealed, and MUST NOT produce evidence.

### 5.4 Substrate parity (`P`)

- **P1.** The control plane MUST maintain a per-application **substrate
  lockfile**: Kubernetes version, CRDs, operator releases and versions,
  admission configurations, gateway classes, storage classes, and priority
  classes present in production, plus a digest over the set. Cloud identity
  bindings (IAM roles, workload identity annotations) are recorded as
  **declared-not-verified**: the seal proves an allowlisted endpoint was
  reachable, never that production's credentials are authorized there
  (§2.4).
- **P2.** Promotion evidence MUST include the sandbox's substrate digest.
  Evidence MUST be marked invalid on mismatch with production's digest;
  any admin override MUST be recorded in the audit log.
- **P3.** The control plane SHOULD provision sandbox substrates from the
  lockfile (shared-cluster operators; prebaked local images). Verification
  (P2) is the contract; provisioning is convenience.
- **P4.** Substrate drift in production (operator upgraded, CRD changed)
  MUST be detected and reflected in the lockfile digest so stale sandboxes
  stop producing valid evidence.

### 5.5 Diff, promotion, and evidence (`G`)

```
PR opened ─▶ sandbox created (S6) ─▶ push ─▶ render → apply → test/traffic
merge ─▶ re-render merge result ─▶ digest match? ──yes─▶ evidence transfers
                                        │                 └▶ promote (G8 policy)
                                        └──no─▶ evidence stale (G7)
                                                └▶ merged-tree run required
```

- **G1.** Production namespaces MUST be writable only by the controller
  executing an approved promotion. Product-managed RBAC keeps human and
  agent roles read-only in production. No `--env prod` flag exists anywhere
  in the CLI.
- **G2.** `diff` MUST compare a bundle against what production currently
  runs, normalized (defaulting, managed fields) so the diff is noise-free.
- **G3.** A promotion request MUST carry: bundle digest, source sandbox,
  normalized diff, and machine-gathered evidence — seal held, egress
  violation count, substrate digest match, data fidelity level per
  datastore (D2), capacity mode (S7), workload readiness, observed healthy
  duration, **witnessed activity & test results** (synthetic/test traffic
  events executed without egress violations), and **linked dependency bundle
  digests** with current production parity status (S8).
- **G4.** Approval policy (required approver count, allowed roles) is
  declared per application. Self-approval MUST be rejected.
- **G5.** On approval, the controller applies the recorded bundle bytes to
  the production namespace (direct mode; see G9 for write-back mode). Every
  transition (created, approved, applied, failed) MUST be written to a
  synchronous audit log; if the audit sink is unavailable, the transition
  MUST NOT proceed.
- **G6.** Evidence wording MUST stay honest: *"ran sealed with zero
  undeclared dependency attempts on a substrate matching production, at
  data fidelity F, capacity mode C, healthy for N, with T test/traffic events
  witnessed, linked to dependencies [AppB@digest (prod-match: true)]"* —
  never "verified working." Claims cover exercised code paths only; idle
  boot is distinguished from active traffic; no production load is implied;
  no production data is implied below `masked-snapshot`.
- **G7.** A promotion MAY be bound to a pull request. Binding rule: after
  merge, the controller re-renders the merge result; if the resulting
  digest equals the sandbox's bundle digest, the sandbox's evidence
  transfers to the promotion (preserving soak time via S6 soak inheritance).
  On mismatch the evidence MUST be marked stale and the promotion blocked
  until a sandbox run of the merged tree produces fresh evidence.
  Merge-queue integration SHOULD provide merged-tree runs where the SCM
  supports it; requiring branches to be up to date before merge is the cheap
  way to make digest-match hold. Digest-match presumes deterministic
  rendering: renders MUST be reproducible (pinned charts and values; no
  cluster `lookup`, timestamps, or randomness), and a bundle whose render
  is non-deterministic MUST fail intake with a determinism error rather than
  produce flapping digests.
- **G8.** Merge semantics are per-application policy. **Auto-promote**:
  merge with valid evidence and satisfied approver policy opens and
  approves the promotion; G4's approver requirements MAY be satisfied by
  required PR reviewers. **Queued**: merge opens a promotion request
  awaiting explicit approval. In both modes self-approval rejection maps to
  the SCM's author-cannot-approve rule and MUST also be enforced
  server-side.
- **G9.** Apply mode is per-application policy. **Direct**: the controller
  applies bundle bytes (G5). **Write-back**: approval commits the rendered
  bundle to the production path of the GitOps repository; the GitOps
  controller (Argo/Flux) applies; the promotion completes only when the
  controller verifies live state matches the bundle digest. Evidence and
  audit semantics MUST be identical in both modes.
- **G10.** Promotion SHOULD support a canary stage through an existing
  progressive-delivery controller (Argo Rollouts, Flagger): approval admits
  the bundle to a bounded canary; full rollout is gated on canary analysis;
  canary metrics are appended to the promotion's evidence. Load realism
  lives here — in production, bounded — never in sandbox claims. MAY land
  post-v1.
- **G11.** Rollback is a promotion. The controller MUST retain previously
  applied production bundles and MUST support opening a promotion request
  for a prior digest in one command, carrying that bundle's original
  evidence plus its production history (observed-healthy duration while
  live). Per-application policy MAY fast-track rollback approvals. A failed
  or partial apply MUST leave the promotion in state `failed` (G5) with the
  live-state divergence recorded, and the rollback path available.

### 5.6 Boundary contract (`C`)

- **C1.** Each application MUST declare the complete set of
  environment-variant values, limited to four kinds: **secret names**
  (values supplied per environment, never inside bundles), **ingress
  hostnames** (living on gateway listeners — substrate — never in bundles),
  the **egress allowlist** (external FQDNs), and **internal application
  dependencies** (target Application references and alias hostnames mapped
  to linked sandboxes in dev and target namespaces in prod).
- **C2.** Apply MUST fail when a bundle references a secret not declared in
  the contract or lacking a value for the target environment.
- **C3.** Any other environment variance is out of contract by design. The
  correct response to "I need to template one more field" is a change to
  this spec, not an overlay mechanism. Growth of this list is the design's
  failure signal. (Capacity is not templated variance either — it is a
  controller-applied transform recorded in evidence, S7.)

### 5.7 Command surface (`CLI`)

```
setup                                      # install controller + control-plane CRDs on a cluster
login
app create <name> --owners ... --approvers ...
sandbox create|destroy|list [--local]      # manage sandbox instances
apply    -a app -s sandbox -f dir/ [--record-egress] # render → lint → bundle → apply (with optional egress discovery)
status   -a app -s sandbox                 # readiness + live seal violations + capacity diagnostics
diff     -a app -s sandbox                 # bundle vs production, normalized
promote  -a app -s sandbox                 # open promotion request with evidence
approve|reject <promotion-id>
serve                                      # local web dashboard (view + sandbox iterate)
```

The PR flow drives these same verbs through the SCM integration (CP4); it
adds no separate command surface. The `--record-egress` flag enables
interactive allowlist discovery during initial workload onboarding.

### 5.8 Control plane (`CP`)

- **CP1.** The product introduces exactly five CRDs — `Application`,
  `Sandbox`, `Bundle`, `PromotionRequest`, `ClusterRegistry` (clusters +
  substrate lockfiles) — under one API group. These sit beside user
  resources; the product MUST NOT require wrapping or re-schematizing any
  user workload. Inter-application dependency graphs (C1) are validated
  across `Application` CRDs.
- **CP2.** All validation, bundling, evidence gathering, and enforcement run
  server-side in the controller. The CLI is a thin client (create CRDs,
  watch status) so there is no client-version drift in enforcement.
- **CP3.** Sandbox and production MAY be the same cluster (namespaces) or
  different registered clusters; evidence semantics are identical.
- **CP4.** CI systems are untrusted triggers. All evidence MUST be gathered
  and signed by the control plane; commit and PR status checks MUST be
  posted by the controller through the SCM integration (e.g. a GitHub App),
  never computed or asserted by pipeline code. A pipeline can carry
  evidence; it MUST NOT be able to mint it.

### 5.9 Data fidelity (`D`)

The seal proves dependency closure because it is a closed-world check. Data
is open-world: it cannot be proven equal, only *representative* — so the
product verifies shape, declares grade, and bounds the rest with the canary
stage (G10). No requirement below moves production values by default.

- **D1.** The control plane MUST maintain a per-application **data profile
  lockfile** per declared datastore: schema digest plus a statistical
  profile (per-column null rates, cardinalities, length distributions,
  character classes, row counts, referential fan-out), content-addressed.
  Profiling runs against production; production values MUST NOT leave it.
- **D2.** Evidence MUST declare the fidelity level of every datastore in
  the run: `fixtures` | `schema-replay` | `profile-synthetic` |
  `masked-snapshot` | `live-clone`.
- **D3.** Per-application policy MUST support minimum fidelity levels,
  including conditional rules (e.g. "bundles containing a migration require
  ≥ `schema-replay`"). Evidence below the applicable minimum MUST be marked
  invalid for promotion.
- **D4.** Sandbox provisioning MUST support **migration replay**:
  instantiate a datastore from the profile lockfile's schema and run the
  bundle's migration chain against it. Failures surface in `status` and in
  evidence.
- **D5.** Data profile drift in production (schema change, distribution
  shift beyond threshold) MUST update the lockfile digest so stale
  sandboxes stop producing valid evidence at their declared level —
  the data analog of P4.
- **D6. (v1.x)** A **profile-synthetic generator** MUST produce data
  satisfying the profile and SHOULD oversample the pathological tail:
  nulls, empty strings, unicode edge cases, max-length values, duplicates,
  skewed cardinalities. Its job is to be nastier than average production
  data — this is where "breaks on wrong data" bugs are caught.
- **D7. (v1.x)** **Thin-clone drivers** (Neon and PlanetScale branching,
  Aurora clones, Database Lab) MAY provide `live-clone` fidelity for
  externally managed databases. Clone endpoints enter the sandbox through
  the boundary contract: a per-sandbox secret plus an allowlist entry (C1).
- **D8.** Real-data levels (`masked-snapshot`, `live-clone`) MUST be
  admin-enabled per application, never default, and MUST NOT be available
  to agent-owned sandboxes without explicit policy. Masking itself is a
  partner integration, not an in-house engine (§6).

## 6. Stack requirements

**Required:** any conformant Kubernetes ≥ 1.29 (managed, bare metal, or
Kind) whose CNI enforces standard NetworkPolicy v1 — true of the major
managed offerings and default k3s, and probe-verified at setup (N7); no
vendor policy CRDs are ever required. Plus: the controller + five CRDs
(single binary install); the bundled egress proxy (with transparent pod-level
redirect helper); cert-manager for webhook certificates.

**Used when detected, never required:** a DNS-aware CNI (better seal
mechanism), Gateway API (hostname boundary via listeners), a log store for
audit shipping, an SCM integration (GitHub App or GitLab equivalent) for
PR-linked promotion (G7, CP4), a progressive-delivery controller (Argo
Rollouts, Flagger) for the canary stage (G10), database branching services
(D7).

**Deliberately absent:** no required *specific* CNI (only standard
NetworkPolicy enforcement, above), no service mesh, no bundled
database/queue/cache, no compliance-profile engine, no manifest language,
no in-house data-masking engine (D8), no rollout engine (G10 is an
integration). Opinionated stack presets can exist later as bundle
generators + substrate lockfiles, but nothing in §5 may depend on them.

## 7. Non-goals (v1)

- Choosing or installing an application stack for the user.
- Multi-namespace applications in the default mechanism (S4 is the path;
  inter-app dependencies use S8 linked sandboxes).
- Production data *values* in sandboxes by default: the v1 fidelity ladder
  is built from shape, not values (D1–D5); real-data levels exist only as
  admin-enabled opt-ins (D8).
- Load-realism claims about sandboxes: load evidence comes from the canary
  stage in production (G10), never from a sandbox.
- Compiling network topology from a declared dependency graph (users write
  their own NetworkPolicy; the seal bounds the blast radius).
- Managing git. In write-back mode (G9) the product commits rendered
  bundles to a declared path and verifies the applied result; it does not
  own branching, history, or repository structure.

## 8. Success metrics

1. **Time-to-first-sealed-sandbox** on an existing app: ≤ 1 hour from
   install, nothing rewritten.
2. **Sandbox control-plane readiness**: ≤ 30 s (S2) — the product must beat
   the docker-compose approximation it replaces.
3. **Evidence integrity**: 0 production writes outside approved promotions
   (audited invariant, not aspiration).
4. **Promotion honesty**: % of promotions whose applied result matches the
   sandbox result (drift here means substrate verification is leaking).
5. **Migration catch rate**: migration failures caught at `schema-replay`
   fidelity in sandboxes vs. first surfacing in production. The number that
   justifies §5.9's existence.

## 9. Open questions

1. ~~GitOps coexistence~~ — **resolved** as the per-application apply-mode
   fork: direct vs. write-back (G9).
2. ~~Egress proxy transparency~~ — **resolved**: standard NetworkPolicy floor
   (N3, N7) combined with transparent pod-level traffic redirection and DNS
   interception helpers guarantees zero-bypass and full non-HTTP protocol
   support with no workload modifications needed.
3. Evidence retention: bundles, evidence, and data profiles are audit
   artifacts — retention and GC policy.
4. Rendered-only intake (B5) vs native Helm support in v1.
5. Substrate lockfile scope: cluster-wide inventory vs application-scoped
   subset — the digest's blast radius on unrelated operator upgrades, and
   how often drift invalidates in-flight PR soak time (P4 × S6) on a busy
   platform.
6. Merge-queue scope: which SCM providers get merged-tree runs (G7) in v1,
   and whether digest-match plus "require branch up to date" is an
   acceptable v1 without queue integration.
7. Data profile scope beyond SQL: what do `schema-replay` and
   `profile-synthetic` mean for a Kafka topic, an object store, a cache?
8. Canary evidence grade (G10): which analysis providers and metrics count
   as evidence, and how canary policy is declared per application.
9. ~~Inter-application dependencies~~ — **resolved**: first-class internal
   application dependencies declared in boundary contracts (C1) and backed
   by linked sandboxes (S8) with dependency bundle digests recorded and
   parity-verified in promotion evidence (G3, G6).
