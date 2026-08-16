# Proposal: add-cloudbox-v1

## Why

Infrastructure review today gates speculation, not evidence: changes are approved before they have ever run, and no environment in the dev→staging→prod ladder is actually reproducible, so failures surface late where they are most expensive. CloudBox v1 makes proof cheap — every change runs first in a sealed, substrate-verified sandbox, and the machine-gathered, control-plane-signed evidence of that run is delivered into the team's existing PR and GitOps flow. The rise of AI coding agents (fast iteration, high change volume, need for contained execution) makes this urgent now.

## What Changes

- Establish the complete CloudBox v1 product specification from `draft.md` (draft 5), covering the adoption ladder L1–L4: sandbox → evidence check → gated promotion (write-back) → strict mode.
- Introduce content-addressed **bundles** as the unit of apply, diff, evidence, and promotion — plain Kubernetes YAML in, no product schema required (B family).
- Introduce **sealed sandboxes**: one-command, no-approval, default-deny ingress/egress with an explicit FQDN allowlist, enforced via standard NetworkPolicy v1 plus a product-managed egress proxy; enforcement is probe-verified, never assumed (S, N families).
- Introduce **substrate parity**: per-application lockfile (scoped to what bundles reference) with hash-checked verification against production; drift invalidates evidence (P family).
- Introduce **evidence** and the **L2 evidence check**: signed, honest-by-construction records of sealed runs, posted as SCM status checks by the control plane only — CI is an untrusted trigger (G, CP families).
- Introduce **gated promotion** for L3/L4: write-back to the GitOps repo (recommended) or direct apply (strict), approval policy, synchronous audit, rollback-as-promotion, break-glass with divergence detection (G family).
- Introduce the **boundary contract**: exactly four kinds of environment variance (secret names, ingress hostnames, egress allowlist, internal app dependencies); everything else is invariant or a recorded controller transform (C family).
- Introduce **data fidelity**: profile lockfiles, five declared fidelity levels, migration replay, thin-clone drivers; production values never leave production by default (D family).
- Introduce the **CLI/ergonomics surface**: `check`, `apply`, `status --explain`, `test`, `logs/exec/port-forward`, `diff`, `promote` — sealed-life ergonomics that beat docker-compose (X family, §6.8).
- Scope note: requirements tagged **(v1.x)** or **(post-v1)** in the draft (linked sandboxes S8, auto-promote G8, canary G10, synthetic generator D6, N8 adversarial hardening roadmap) are recorded as committed direction in design, not as v1 spec deltas.

## Capabilities

### New Capabilities

- `bundle-intake`: Accepting plain multi-doc Kubernetes YAML, producing content-addressed bundles, environment-agnostic normalization with recorded intake transforms, rejection-with-guidance, offline `check` (B1–B5, X3).
- `sandbox-lifecycle`: One-command no-approval sandbox creation, shared-cluster namespace mechanism, local Kind mechanism, TTL/idle expiry and quotas, PR-bound lifecycle with soak accumulation and inheritance, recorded capacity transforms (S1–S3, S5–S7).
- `sandbox-seal`: Default-deny sealing before workload admission, FQDN allowlist via egress proxy on a standard NetworkPolicy v1 floor, blocked-egress recording, no per-sandbox weakening, probe-verified enforcement, honestly scoped containment claims (N1–N8 v1 scope, §2.5).
- `substrate-parity`: Application-scoped substrate lockfile and digest, evidence invalidation on mismatch, lockfile-driven provisioning, scoped drift detection (P1–P4).
- `evidence`: Evidence content and honest wording, witnessed activity attribution, PR binding with digest-match transfer and staleness, deterministic-render requirement, signed L2 status checks minted only by the control plane (G2, G3, G6, G7, G13, X4, CP4).
- `promotion`: L3/L4 write path — approval policy with self-approval rejection, synchronous audit, write-back and direct modes with identical semantics, queued merge semantics, rollback as promotion, strict-mode single-writer invariant and break-glass (G1, G4, G5, G8 queued, G9, G11, G12).
- `boundary-contract`: The four declared kinds of environment variance; apply fails on undeclared secrets; no overlay mechanism by design (C1–C3).
- `developer-ergonomics`: Sealed `logs`/`exec`/`port-forward`, `status --explain` with allowlist proposals, offline `check`, witnessed in-sandbox `test`, and the v1 command surface (X1–X4, §6.8).
- `control-plane`: Five CRDs under one API group, server-side authority with thin CLI, same-or-separate cluster topology, CI-as-untrusted-trigger (CP1–CP4).
- `data-fidelity`: Data profile lockfiles, per-datastore fidelity declaration, policy minimums with conditional rules, migration replay, drift detection, thin-clone drivers, admin-gated real-data levels (D1–D5, D7, D8).

### Modified Capabilities

(none — this is the first change; `openspec/specs/` is empty)

## Impact

- Greenfield: no existing code or specs. This change creates the v1 spec baseline that all implementation changes build on.
- Creates ten new capability specs under `openspec/specs/` when synced/archived.
- Acceptance-test harness (Python/behave per `openspec/config.yaml`) will consume the Gherkin scenarios embedded in these specs.
- External surface commitments: Kubernetes ≥ 1.29 with NetworkPolicy-enforcing CNI, cert-manager, SCM integration (GitHub App / GitLab equivalent) for L2+, GitOps controller (Argo/Flux) for L3 write-back; deliberately absent: service mesh, vendor CNI, CD engine, manifest language (§7).
- Open questions carried into design: evidence retention/GC, rendered-only vs native Helm intake, merge-queue provider scope, data profiles beyond SQL, canary evidence grade, N8 hardening priorities, evidence-check UX granularity (§10 items 3, 4, 6, 7, 8, 10, 11).
