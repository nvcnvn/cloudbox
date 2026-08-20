# Proposal: add-local-agent-mvp

## Why

CloudBox v1 is specified across twelve capabilities and implemented against the
simulated driver, but nothing in it can be handed to anyone. The parts that a
real cluster has actually proven — sealing, enforcement probing, egress
evaluation, workload admission, substrate inspection — are exactly the parts a
developer or an AI coding agent needs to run code in a sealed box on their own
machine. Everything still simulated (source control, GitOps, production state,
audit sinks, datastores) is what a public release would have to fake.

This change ships the proven half and refuses to pretend about the other. The
target user is an AI coding agent iterating locally: it applies manifests to a
sealed sandbox, is denied undeclared egress, reads back a machine-readable
account of what it tried to reach, amends its declared contract, and produces
honest evidence of the run. That loop needs no SCM integration, no promotion,
no durable state, and no authentication — which is why it is reachable now.

## What Changes

- **The CLI becomes the whole product surface.** At the sandbox level there is
  no pull request and no SCM integration, so every verb the loop needs must
  exist on the command line: create an application, create a sandbox, apply,
  run the witnessed test suite, read status and evidence, destroy. Today the
  CLI has `check`, `status`, `logs`, `exec`, and `port-forward`; the rest of the
  loop is reachable only over HTTP.
- **Every command speaks machine-readable output.** The consumer is a program.
  Commands emit structured output on request and distinguish outcomes by exit
  code, so an agent reads the blocked-egress record and the allowlist proposal
  as data rather than parsing prose.
- **The CLI starts and reuses a local control plane.** The agent runs one
  binary. Authority stays server-side (ADR 0004): the CLI starts the same
  control plane it would otherwise be pointed at, and never gains enforcement
  of its own.
- **A user-controlled cluster is a first-class sandbox host.** The developer or
  agent brings a Kubernetes cluster with an enforcing CNI and registers it.
  Sandboxes on it hold the same seal and iteration semantics, and their
  evidence is marked user-controlled — non-promotable and non-postable, exactly
  as `sandbox-lifecycle` already requires. Product-managed local cluster
  provisioning is deferred, which **narrows an existing MUST**.
- **Substrate parity stops claiming a match it never made.** With no production
  cluster registered — the normal state for every sandbox in this release —
  evidence currently reports a substrate match. Parity with nothing to compare
  against becomes an explicit unverified state that is never read as a match.
- **The published containment statement names its recording limit.** Only
  proxy-aware HTTP egress reaches the product's proxy and is recorded by FQDN;
  all other egress is denied by the CNI without an FQDN record. That is
  stronger containment and weaker observability, and the public claim must say
  so rather than let it be discovered.
- **Unimplemented integrations refuse instead of simulating.** Promotion,
  evidence checks, production state, audit sinks, and datastore seeding are
  backed by in-memory arrangement surfaces. In the released driver they refuse
  with a named reason and record nothing.
- **The release identifies itself.** Real build version on both binaries, a
  pinned and resolvable egress-proxy image instead of a locally loaded `:dev`
  tag, and a quickstart that names the supported substrate.

### Non-goals

- **Source control, GitOps, promotion, and the evidence check (L2–L4).** They
  are specified, implemented against the simulation, and untouched here. This
  release is the sandbox level only, and `developer-ergonomics` already states
  that level needs no promotion verbs.
- **Authentication, authorization, and TLS.** A single agent on one machine
  reaching a control plane it started itself. The identity header stays what it
  is, and the release says so rather than implying a trust boundary it has not
  built.
- **Durable control-plane state.** Sandboxes are disposable and the daemon is
  local; losing state on restart costs the agent a re-registration. Persistence
  is inseparable from the CRD-versus-database decision, which this change does
  not take.
- **Real evidence signing.** The current signature is an unkeyed digest. It is
  not load-bearing while evidence cannot be posted or promoted, and honest
  wording is enforced by the `evidence` capability regardless. Signing lands
  with the surface that transfers evidence off the machine.
- **Product-managed local cluster provisioning.** Recorded as direction, not
  built: it is a Kind, CNI, and container-runtime surface the product would own
  in full, and the conformance harness already proves a registered enforcing
  cluster works.
- **Managed offerings (EKS/GKE/AKS).** Post-v1 per ADR 0008 and unchanged.

## Capabilities

### New Capabilities

- `release-integrity`: What the published artifact claims about itself and
  refuses to fake — build version identification on the released binaries, a
  pinned and resolvable egress-proxy image reference with honest failure when
  it cannot be provisioned, and the rule that an operation depending on an
  unimplemented integration refuses with a named reason and records no result
  rather than writing to a simulated surface.

### Modified Capabilities

- `developer-ergonomics`: adds the requirement that at the sandbox adoption
  level the command line alone is the whole product surface — carrying every
  verb the sealed-iteration loop needs — and that command output is available
  in a machine-readable form with outcomes distinguished by exit status, so a
  program is a first-class consumer of the explain-and-propose loop.
- `sandbox-lifecycle`: separates the user-controlled cluster property from the
  mechanism that creates it. Registering a user-controlled cluster as a sandbox
  host becomes the specified path, with identical seal and iteration semantics
  and evidence marked non-promotable and non-postable; product-managed local
  provisioning is no longer required for that path.
- `substrate-parity`: adds that parity is unverified — never matched — when no
  production substrate is registered to compare against, and that evidence and
  its consumers must distinguish unverified parity from a verified match.
- `control-plane`: adds that the command line may start and reuse a local
  control plane on the developer's machine without weakening server-side
  authority; enforcement belongs to the control plane whoever started it.
- `sandbox-seal`: adds that the published containment statement names which
  egress is recorded by destination and which is denied without a destination
  record, so the mechanism's observability limit is declared rather than
  discovered.

## Impact

**Modified code**
- `cmd/cloudbox/` — the missing verbs, structured output, exit-code semantics,
  and local control-plane lifecycle.
- `cmd/cloudboxd/main.go` — build version, and a listen mode the CLI can start.
- `internal/core/evidence.go`, `internal/core/substrate.go` — the unverified
  parity state replacing the implicit match at `evidence.go:144`.
- `internal/server/server.go` — named refusals for operations whose integration
  is not implemented under the kube driver.
- `internal/cluster/kube/proxy.go` — pinned image reference and honest failure
  when the proxy cannot be provisioned.
- `hack/` and `Makefile` — the enforcing-cluster bootstrap becomes a documented
  path for users, not a conformance-only script.
- `README.md`, quickstart, and the published containment statement.
- `acceptance-tests/` — scenarios for the new rules, plus page objects. Step
  definitions stay free of selectors and URLs.

**Unchanged interfaces**
- `internal/cluster/cluster.go` stays frozen (ADR 0008).

**In-force ADRs**
- 0001 (NetworkPolicy floor with egress proxy), 0004 (control plane mints all
  evidence; thin CLI; evidence from user-controlled sandboxes is non-promotable
  and non-postable), 0005 (the adoption ladder, whose L1 rung is exactly this
  release), 0006 (application-scoped substrate lockfile), and 0008 (frozen
  cluster contract) constrain this change. ADR 0007 is superseded by 0008 and
  is historical context only. A new ADR is expected for the release boundary —
  which integrations refuse rather than simulate, and why a user-controlled
  cluster is the released substrate.

**Risk if not done**
- The product's only credible half stays unreachable, and the first public
  release either waits on integrations that are months out or ships a control
  plane whose promotion and evidence-check paths write to memory and call it
  success.
