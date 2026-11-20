# Business glossary

Product and workflow terms. The user-facing concepts page (`docs/concepts.md`)
defines every term in this table (ADR 0011).

| Term | Definition | Use When | Avoid |
| --- | --- | --- | --- |
| Sandbox | A disposable, sealed, substrate-verified environment owned by one developer or agent. | Naming the unit an agent creates, iterates in, and destroys. | "environment", "namespace" (implementation detail) |
| Seal | Default-deny ingress and egress with an explicit FQDN allowlist, enforced for a sandbox's whole lifetime. | Describing what contains a sandbox. | "firewall", "isolation" |
| Allowlist | The declared set of FQDNs a sealed sandbox may reach beyond in-sandbox services and cluster DNS. | Naming what the agent amends to permit egress. | "whitelist", "exceptions" |
| Bundle | A content-addressed set of rendered, environment-agnostic Kubernetes manifests; the unit of apply, diff, evidence, and promotion. | Naming what is applied to a sandbox and what evidence is about. | "release", "chart", "deployment" |
| Boundary contract | The declared, finite set of values allowed to differ per environment. | Naming what an application declares may vary between sandbox and production. | "overlay", "config", "values" |
| Substrate | Everything a bundle assumes exists in the cluster: Kubernetes version, CRDs, operators, admission policies, and storage, gateway, and priority classes. | Naming what parity is checked over. | "platform", "infrastructure" |
| Substrate parity | Whether a sandbox's substrate matches the registered production substrate over what the application's bundles reference; matched, mismatched, or unverified. | Describing the comparison and its three outcomes. | "environment parity", treating unverified as matched |
| Evidence | The machine-gathered, control-plane-signed record of a sealed sandbox run, recording what ran, what was witnessed, and for how long. | Naming the output of a sandbox run. | "test report", "attestation" |
| Sandbox host | A cluster registered with the control plane to run sandboxes, accepted only if it can be shown to enforce NetworkPolicy. | Naming where sandboxes run. | "target cluster", "runner" |
| User-controlled cluster | A sandbox host the developer or agent controls; evidence from it is non-promotable and non-postable. | Naming the substrate this release runs on. | "local cluster" (names a mechanism, not the property) |
| Adoption level | One rung of the add-on ladder: L1 sealed sandboxes, L2 evidence checks, L3 write-back promotion, L4 strict single-writer mode. | Saying which part of the product a team uses. | "tier", "plan" |
| Sealed-iteration loop | The cycle an agent runs at L1: declare, create, apply, test, explain, amend, read evidence, destroy. | Describing the agent workflow end to end. | "dev loop" without the sealed qualifier |
| Containment statement | The control plane's published claim of what the seal blocks, what residual channels remain, and which egress is recorded by destination. | Naming the authority the containment page is checked against. | "security guarantee" |
| Residual channel | A path the seal does not close, such as DNS tunneling or exfiltration through allowlisted endpoints. | Naming the declared limits of containment. | "loophole", "bypass" |
| Application | The declared unit a team brings to the product: its boundary contract, its bundles, and its policy; sandboxes and evidence belong to one application. | Naming what an agent declares before creating a sandbox. | "project", "service" |
| Declared suite | The test command an application declares, run inside a sealed sandbox so its results are witnessed in evidence. | Naming the tests whose run evidence records. | "CI", "test job" |
| Allowlist proposal | A ready-to-submit allowlist change the product emits from recorded blocked egress, for review before it is declared. | Describing what explain offers after a blocked attempt. | "auto-allow" (nothing is allowed without amendment) |
| Substrate lockfile | The per-application record of the substrate its bundles reference, used to provision and compare sandbox substrate. | Naming what parity is checked against. | "manifest lock", "dependency lock" |
| Dependency stub | An allowlisted stand-in endpoint satisfying a declared dependency on another application, recorded in evidence as stubbed. | Describing how a sandbox reaches a dependency it does not run. | "mock" |
| Fidelity level | The declared grade of a datastore's data in a run — fixtures, schema-replay, profile-synthetic, masked-snapshot, or live-clone — named in evidence. | Describing how representative a run's data was. | "test data quality" |
| Refusal | An outcome where the product declines a request and names the reason, recording nothing. | Describing a declined operation as opposed to a failure. | "error" when the product is behaving as designed |
