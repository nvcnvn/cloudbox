# Tasks: add-cloudbox-v1

Implementation MUST honour the in-force ADRs under `adr/` (0001–0006).
Sequencing follows design.md's migration plan: L1 core → L2 evidence → L3
promotion → L4 strict → data fidelity. Each implementation task is one spec
Rule taken red → green: run the suite so its scenarios fail for the right
reason, implement until they pass, then commit.

## 1. First-time setup — skip this whole section if `acceptance-tests/` already exists

- [x] 1.1 Confirm the acceptance stack: `stack: python` is already recorded in `openspec/config.yaml` — verify it and do not re-ask
- [x] 1.2 Set up the application skeleton (controller binary, CLI entry point, run script) so the app can boot; record the implementation-stack choice (e.g. Go + kubebuilder for controller and CLI) as a new ADR if it is durable
- [x] 1.3 Create `acceptance-tests/` at the repo root as an independent Python project (behave >= 1.2.7, behave-html-formatter, requests, beautifulsoup4) that boots the app before the suite and shuts it down after
- [x] 1.4 Copy the acceptance-test-authoring skill's Python reference files verbatim into `acceptance-tests/`: `extract_gherkin.py`, `openspec_effective_spec.py`, `run_acceptance.py`, `behave.ini`, `environment.py`, and the shared lintrc as `.gherkin-lintrc` — `steps/` and `environment.py` at the `acceptance-tests/` root — so the runner extracts every `spec.md` under `openspec/` into `acceptance-tests/.extracted/` and discovers the extracted features, excluding `openspec/changes/archive/`
- [x] 1.5 Make the single test command (`python run_acceptance.py`) always generate an HTML report under `acceptance-tests/reports/`
- [x] 1.6 Add the spec-lint command (extract, then `npx gherkin-lint` with `.gherkin-lintrc` over `.extracted`) and gitignore `acceptance-tests/.extracted/` and `acceptance-tests/reports/`
- [x] 1.7 Write `acceptance-tests/README.md` with instructions for running the suite and where the HTML report is written

## 2. L1 foundation — control plane, bundle intake, boundary contract

- [x] 2.1 control-plane: five CRDs under one API group, user workloads unwrapped, dependency graph validated — red → green → commit
- [x] 2.2 control-plane: server-side enforcement with thin CLI; offline check advisory, server intake authoritative — red → green → commit
- [x] 2.3 control-plane: shared-cluster and split-cluster topologies with identical evidence semantics — red → green → commit
- [x] 2.4 bundle-intake: apply accepts plain multi-document Kubernetes YAML of any kind — red → green → commit
- [x] 2.5 bundle-intake: every apply produces a content-addressed bundle recorded server-side — red → green → commit
- [x] 2.6 bundle-intake: uniform namespace stripped by recorded transform; multi-namespace bundles fail with guidance — red → green → commit
- [x] 2.7 bundle-intake: cluster-scoped resources rejected naming manifest and fix — red → green → commit
- [x] 2.8 bundle-intake: cross-namespace/FQDN references linted with suggested rewrites — red → green → commit
- [x] 2.9 bundle-intake: in-bundle references limited to short names and declared aliases — red → green → commit
- [x] 2.10 bundle-intake: Helm/Kustomize enter as rendered output — red → green → commit
- [x] 2.11 bundle-intake: non-deterministic renders fail intake with a determinism error — red → green → commit
- [x] 2.12 bundle-intake: offline `check` reports the full fix list without a cluster and exits nonzero on blockers — red → green → commit
- [x] 2.13 boundary-contract: contract limited to the four declared kinds — red → green → commit
- [x] 2.14 boundary-contract: apply fails on undeclared or unvalued secrets — red → green → commit
- [x] 2.15 boundary-contract: no variance mechanism outside the contract; namespace/capacity are recorded transforms — red → green → commit
- [x] 2.16 boundary-contract: declared dependencies satisfiable by stubs, recorded as stubbed — red → green → commit

## 3. L1 core — sandbox lifecycle, seal, substrate parity

- [x] 3.1 sandbox-lifecycle: one-command no-approval creation with owner-scoped RBAC — red → green → commit
- [x] 3.2 sandbox-lifecycle: shared-cluster sandboxes sealed and ready within thirty seconds — red → green → commit
- [x] 3.3 sandbox-lifecycle: local Kind sandboxes from the lockfile; local evidence non-promotable and non-postable — red → green → commit
- [x] 3.4 sandbox-lifecycle: TTL, idle expiry, and per-application quotas — red → green → commit
- [x] 3.5 sandbox-lifecycle: PR-bound lifecycle with soak reset on digest change and soak inheritance on identical digest — red → green → commit
- [x] 3.6 sandbox-lifecycle: recorded capacity transforms (squeezed default, minimal, full), squeeze-incompatible diagnostics, autoscaler suspension — red → green → commit
- [x] 3.7 sandbox-seal: sealed before any workload is admitted — red → green → commit
- [x] 3.8 sandbox-seal: egress limited to in-sandbox services, cluster DNS, and the allowlist — red → green → commit
- [x] 3.9 sandbox-seal: standard NetworkPolicy floor with egress proxy and transparent redirection (ADR 0001) — red → green → commit
- [x] 3.10 sandbox-seal: blocked egress recorded with destination, timestamp, attribution — red → green → commit
- [x] 3.11 sandbox-seal: no per-sandbox weakening; allowlist changes are audited admin policy — red → green → commit
- [x] 3.12 sandbox-seal: user NetworkPolicy narrows but never widens — red → green → commit
- [x] 3.13 sandbox-seal: probe-verified enforcement; non-enforcing clusters never sealed, never evidenced — red → green → commit
- [x] 3.14 sandbox-seal: published containment claims match the declared threat-model scope — red → green → commit
- [x] 3.15 substrate-parity: application-scoped lockfile with digest; declared-not-verified identity/secrets — red → green → commit
- [x] 3.16 substrate-parity: evidence carries substrate digest, invalid on mismatch, audited override — red → green → commit
- [x] 3.17 substrate-parity: sandbox substrates provisioned from the lockfile — red → green → commit
- [x] 3.18 substrate-parity: production drift invalidates only referencing applications (ADR 0006) — red → green → commit

## 4. L1 ergonomics

- [x] 4.1 developer-ergonomics: `logs`, `exec`, `port-forward` owner-scoped, audited, seal-preserving — red → green → commit
- [x] 4.2 developer-ergonomics: `status --explain` renders blocked egress and emits an allowlist proposal; `apply --record-egress` onboarding loop — red → green → commit
- [x] 4.3 developer-ergonomics: evidence-check level needs no promotion verbs — red → green → commit

## 5. L2 — evidence and the signed check

- [x] 5.1 evidence: normalized diff against production — red → green → commit
- [x] 5.2 evidence: evidence carries the full machine-gathered record — red → green → commit
- [x] 5.3 evidence: honest scoped wording, idle vs exercised, declared-not-verified labels — red → green → commit
- [x] 5.4 evidence: PR binding — digest-match transfer with soak, staleness on mismatch — red → green → commit
- [x] 5.5 evidence: witnessed in-sandbox `test` runs attributed and signed; CI can trigger, never assert — red → green → commit
- [x] 5.6 evidence: signed evidence check minted only by the control plane with the five validity conditions — red → green → commit
- [x] 5.7 control-plane: CI untrusted — pipeline-forged checks impossible (ADR 0004) — red → green → commit

## 6. L3 — promotion

- [x] 6.1 promotion: declared approval policy with server-side self-approval rejection — red → green → commit
- [x] 6.2 promotion: synchronous audit of every transition; unavailable sink blocks — red → green → commit
- [x] 6.3 promotion: merge opens a queued promotion awaiting explicit approval — red → green → commit
- [x] 6.4 promotion: write-back commits the bundle, GitOps applies, completion on live digest match; parity with direct mode — red → green → commit
- [x] 6.5 promotion: rollback is a promotion with retained bundles, original evidence, production history; failed applies recorded — red → green → commit

## 7. L4 — strict mode

- [x] 7.1 promotion: strict mode single-writer RBAC; no production-targeting CLI path; no prod credentials below L4 — red → green → commit
- [x] 7.2 promotion: break-glass with auto-expiring audited access; divergence detection invalidates evidence until reconciled; setup fails without a break-glass role — red → green → commit

## 8. Data fidelity

- [x] 8.1 data-fidelity: content-addressed data profile lockfiles; values never leave production — red → green → commit
- [x] 8.2 data-fidelity: per-datastore fidelity declared in every run's evidence — red → green → commit
- [x] 8.3 data-fidelity: policy minimums with conditional rules; below-minimum evidence invalid — red → green → commit
- [x] 8.4 data-fidelity: migration replay from the profile schema with surfaced failures — red → green → commit
- [x] 8.5 data-fidelity: profile drift stales evidence at its declared level — red → green → commit
- [x] 8.6 data-fidelity: thin-clone drivers deliver live-clone through the boundary contract — red → green → commit
- [x] 8.7 data-fidelity: real-data levels admin-enabled and agent-gated — red → green → commit

## 9. Completion

- [x] 9.1 Run the full suite: every scenario passes, zero pending/undefined steps, HTML report generated under `acceptance-tests/reports/`
- [x] 9.2 Verify the composition: `.extracted/` rebuilt, nothing loaded from `openspec/changes/archive/`, no duplicate scenarios, composition report clean
