## Context

CloudBox v1 is fully specified and implemented against the simulated cluster
driver, and the `kube` driver now satisfies the frozen cluster contract with a
tagged conformance subset running on Kind with an enforcing CNI (ADR 0008).
Nothing has ever been released. The gap between "the suite is green" and "a
stranger can use this" is not one gap but four: the CLI carries only part of
the loop, the control plane cannot be reached without operating a daemon, the
substrate the product runs on has no user-facing acquisition path, and every
external system other than the cluster is an in-memory arrangement surface that
answers as if it were real.

ADR 0005 already anticipated this release. Its ladder puts **L1 — sealed
sandboxes, zero integrations** at the bottom rung and asserts that every level
is independently valuable. This change is the first test of that assertion:
ship L1 alone, to the public, and see whether a sealed box with honest evidence
is worth anything without a pull request attached to it.

The user is an AI coding agent iterating locally. That is not a marketing
framing — it changes design decisions. An agent supervises no processes, parses
no prose, and reads no README when it hits an error. It also happens to be the
user for whom containment matters most, which is why the seal (the one part of
the product a real CNI has actually proven) is also the part this user needs.

Constraints in force: the cluster contract is frozen (ADR 0008); enforcement is
server-side and the CLI is thin (ADR 0004); evidence from user-controlled
clusters is non-promotable and non-postable (ADR 0004); the seal is a standard
NetworkPolicy floor plus a product egress proxy (ADR 0001); the substrate
lockfile is application-scoped (ADR 0006).

## Goals / Non-Goals

**Goals:**

- An agent with a Kubernetes cluster completes the sealed-iteration loop —
  declare, create, apply, test, explain, amend, evidence, destroy — through the
  command line alone, with structured output at every step.
- One binary is the entry point; no process supervision is delegated to the
  user.
- Nothing the release exposes reports success for a system it does not have.
- Substrate parity stops claiming a match it never performed.
- The published containment claim states the mechanism's observability limit,
  not only its blocking guarantee.

**Non-Goals:**

- Source control, GitOps, promotion, evidence checks (L2–L4) — implemented,
  simulated, and untouched.
- Authentication, authorization, TLS, durable state, real evidence signing.
- Product-managed local cluster provisioning.
- Managed Kubernetes offerings (post-v1 per ADR 0008).

## Decisions

### D1: The user brings the cluster; the product does not create one

**Decision.** A developer or agent registers a cluster they control as a
sandbox host. The product verifies enforcement at registration and refuses a
cluster that cannot be shown to enforce NetworkPolicy. A documented script
creates a suitable local cluster, but it lives outside the product boundary.

**Why.** `sandbox-lifecycle`'s local rule required the product to provision a
Kind cluster from the lockfile, and `core.provisionLocalCluster` implements it
by type-asserting an off-contract `NewCluster(name, enforcing, userControlled)`
method that only the sim driver has. Under `--driver kube` that path returns a
500. So the one mode this release is built around is the one mode that has
never run for real — while the conformance harness has been standing up
registered enforcing clusters and running real sandboxes on them since ADR 0008.
The cheapest correct move is to ship the path that already works.

The deeper reason is boundary discipline. Product-managed provisioning means
owning Kind, a CNI installation, a container runtime, image loading, and every
failure mode of each — on the user's machine, where the product has the least
information and the least recourse. That surface is larger than everything else
in this change combined, and it buys one command of convenience.

**Alternatives considered.** *Implement `NewCluster` for the kube driver* —
rejected: it cannot be satisfied honestly. The kube driver's whole premise is
that it operates clusters described by kubeconfig contexts; conjuring one is a
different responsibility wearing the same method name, and ADR 0008 freezes the
contract precisely to stop that. *Keep the spec rule and ship it unimplemented*
— rejected: a public release with a known-failing MUST is the failure mode this
codebase exists to refuse.

**Consequence for the spec.** The old rule is removed rather than modified,
because the property it protected (user-controlled evidence is non-promotable
and non-postable, ADR 0004) survives while its mechanism does not. Removing and
re-adding states that honestly; a MODIFIED rule keeping the old name would
leave "provision from the substrate lockfile" describing a rule that provisions
nothing.

**Recorded direction.** `cloudbox local up` — a wrapper that stands up an
enforcing cluster and registers it — remains the intended ergonomics once the
release has users to justify owning that surface. It is direction, not a
requirement.

### D2: Enforcement of the enforcing-CNI precondition moves to registration

**Decision.** Registration probes the candidate cluster and refuses it if
NetworkPolicy enforcement cannot be shown, naming unproven enforcement.

**Why.** D1 hands cluster selection to the user, which introduces a failure the
managed topology never had: a cluster that looks fine and silently enforces
nothing. The `sandbox-seal` N7 probe already refuses to report such a sandbox
sealed, and that remains the last line — but for an agent, discovering this at
the first sandbox creation is a confusing failure several steps from its cause.
Failing at registration puts the error where the decision was made.

This reuses the conformance run's existing enforcement precondition rather than
inventing a second mechanism; `conformance-ci` already specifies exactly this
check and CI already exercises it against a deliberately non-enforcing cluster.

### D3: The CLI starts and reuses a control plane; it never becomes one

**Decision.** When no control plane is addressed, the CLI starts one on the
local machine and reuses it across commands. An explicitly addressed control
plane always wins. The started process is `cloudboxd` as it exists today.

**Why.** The alternative that "obviously" simplifies a local MVP — linking
`internal/core` into the CLI for local mode — is the one ADR 0004 forbids. It
would put validation, bundling, and evidence gathering in a client binary, and
it would create two execution models where enforcement is one thing locally and
another thing remotely. The invariant "0 forged evidence checks" is enforceable
only while there is exactly one place that mints evidence.

Starting a subprocess keeps that invariant intact and costs the agent nothing.
The control plane is the same server whoever launched it; the CLI gains no
authority by having launched it. Reuse across commands matters because an agent
issues many short commands and a per-command control plane would lose all state
between them — state which, being in-memory, is already fragile.

**Alternatives considered.** *Documented two-step (`cloudboxd &` then `--server`)*
— rejected on the agent-user premise: it delegates process lifecycle and log
capture to a caller that supervises nothing. *Embed core in the CLI* — rejected
per ADR 0004 above.

**Open mechanism.** Whether reuse is discovered through a loopback port plus a
state file or a unix socket is an implementation detail for tasks; both satisfy
the rule. The state file must record the daemon's build version so a stale
daemon from an older binary is replaced rather than reused.

### D4: Parity gets a third state rather than a safer default

**Decision.** Substrate parity becomes `verified match` / `mismatch` /
`unverified`. Unverified is produced when no production substrate is registered
and is never collapsed into either of the other two.

**Why.** `evidence.go:144` initialises `SubstrateMatch = true` and only compares
when a production cluster exists, so every sandbox in a production-less release
reports a match it never made. Two escapes are available and both are wrong.
Defaulting to `false` would mark every run mismatched and invalid, which is
equally untrue and additionally useless. Suppressing the field would leave
consumers to infer, and inference is how the current bug happened.

A third state is more work — the field stops being a boolean, the summary
renderer gains a case, and every consumer must handle it — and it is the only
option that says what actually happened. This is the same shape as the decision
in `harden-kube-egress-records`, where a possibly-incomplete egress record was
made to say so rather than present a diminished count as a complete one.

**Interaction with validity.** Unverified parity does **not** invalidate
evidence. P2 invalidates on *mismatch*, and an uncompared run is not a
mismatched one. What unverified parity does forbid is the check and the
promotion — but those refuse for other reasons already (D5, and ADR 0004's
user-controlled-cluster rule), so no new gate is needed. When production is
registered later, comparison resumes with no migration.

### D5: Unimplemented integrations refuse; they do not simulate

**Decision.** Under the released driver, operations whose external integration
does not exist — promotion, evidence checks, production state, audit sink
availability, datastore seeding — refuse with a reason naming the missing
integration, and record nothing.

**Why.** The `/simctl/*` surface is already withheld under `--driver kube`
(ADR 0008), which stops a *tester* from arranging fake state. It does not stop
a *user* from calling `POST /v1/promotions`, which today opens a promotion,
writes to an in-memory map, and returns success. Nothing was promoted. The
product's entire proposition is that its artifacts can be trusted; a recorded
promotion that promoted nothing is more damaging than any missing feature,
because a missing feature is discovered immediately and a false record is
discovered late.

Refusal is also how the release advertises its own boundary. An agent that
calls promote and receives "no GitOps integration is configured in this
release" has learned the product's shape from the product.

**Scope discipline.** This is a driver-conditioned refusal, not a code removal.
The promotion capability stays specified, implemented, and sim-verified; it
becomes reachable when its integration lands, with no spec change.

### D6: The containment statement declares what it cannot observe

**Decision.** The published statement gains a clause naming which egress is
recorded by destination (proxy-mediated, proxy-aware HTTP) and which is denied
without a destination ever being observed (everything else, at the CNI).

**Why.** Divergence 4 records that the released mechanism redirects nothing
transparently: proxy-aware HTTP clients reach the proxy via injected `http_proxy`
and are recorded by FQDN, while a direct connection — even to an allowlisted
destination — is denied by the CNI. Containment is *stronger* for that traffic;
observability is weaker. An operator reading a short blocked-attempt list will
conclude the workload attempted little, when it may have attempted a great deal
over protocols the record cannot name.

The existing rule already forbids overclaiming the *blocking* guarantee. This
extends the same honesty to the *recording* guarantee, which a public release
makes newly load-bearing because the blocked-attempt record is the evidence
artifact's egress-violation count.

### D7: Machine-readable output is a rendering, not a second surface

**Decision.** Structured output renders the same facts the human rendering
carries, from the same server responses. Exit statuses distinguish success, a
product refusal, and an unreachable control plane.

**Why.** The temptation is to give agents a "JSON API mode" that returns richer
data than the human path. That splits the product in two and guarantees drift.
The control plane already returns structured responses; the CLI's human
rendering is a projection of them, and the structured rendering should be
another projection of the same thing. The spec states the equivalence so it is
testable rather than aspirational.

Exit statuses matter more than they appear to. An agent that cannot distinguish
"the product refused your bundle" from "the daemon is down" will retry the
wrong one — the first needs a different bundle, the second needs a restart.

## Risks / Trade-offs

- **A public seal claim invites adversarial testing the product has not had.**
  -> The threat-model rule (N8, §2.5) and D6 bound the claim in the published
  statement, and the release states plainly that it authenticates no one.
  Residual channels are named rather than discovered.

- **BYO substrate is a real onboarding cliff: Docker, Kind, and a CNI install
  before the first sandbox.** -> A documented, tested script (the conformance
  harness's, promoted to a user-facing path) plus a quickstart that names the
  requirement up front. Accepted deliberately in exchange for not owning the
  provisioning surface; `cloudbox local up` is the recorded escape hatch if
  the cliff proves fatal to adoption.

- **In-memory state means a daemon restart loses application registrations.**
  -> Sandboxes are disposable by design, so the loss is a re-registration
  rather than lost work. Mitigated in practice by making registration cheap
  and idempotent from a file the agent already owns. If this proves painful,
  it is evidence for the CRD-versus-database decision rather than a reason to
  pre-empt it.

- **Refusing operations by driver could diverge from the sim suite, where the
  same operations must keep working.** -> The refusal is conditioned on the
  absence of an integration, not on the driver name, and the sim's arrangement
  surfaces are exactly what constitutes that integration's presence in the sim
  suite. Both paths then follow one rule rather than two.

- **The agent-user premise is a bet.** Nothing in the release is agent-specific
  beyond structured output and exit codes; if the premise is wrong, the release
  is still a usable local sandbox tool, and if it is right, the explain-and-
  propose loop is the differentiator against plain `kubectl`.

- **Shipping L1 alone tests ADR 0005's "every level is independently valuable"
  claim in public.** -> That is the point. A negative result is worth more
  before L2's integration cost is paid than after.

## Migration Plan

Nothing is deployed and there are no users, so there is no migration in the
usual sense. The sequencing that matters is internal:

1. The honesty fixes (D4, D5, D6) land before any packaging work, so no
   published artifact ever carries the substrate-match default or an
   integration that simulates.
2. Registration and enforcement probing (D1, D2) land next; they make the
   conformance harness's cluster path a product path.
3. CLI completion and structured output (D7) and control-plane lifecycle (D3)
   follow, since they depend on the verbs' server-side behaviour being settled.
4. Release engineering last: pinned proxy image, version stamping, quickstart,
   and the containment statement — the point at which the claims become public.

Rollback is deletion of a tag; nothing persists on a user's cluster except
namespaces and the proxy deployment, both of which sandbox destruction already
removes.

## Open Questions

1. **Does the enforcement probe at registration need its own canary namespace,
   or can it reuse the seal probe's mechanism unchanged?** The seal probe runs
   inside a sandbox being provisioned; registration has no sandbox yet. If the
   probe needs a scratch namespace, its cleanup on failure must be specified.
2. **How is the pinned proxy image published, and by whom?** The reference must
   be resolvable from an arbitrary user cluster, which implies a public
   registry and a release job that pushes it. Registry choice, tagging scheme
   (digest-pinned versus version-tagged), and multi-architecture coverage are
   unresolved, and the last one bites immediately: the conformance harness
   already selects `arm64` or `amd64` per node.
3. **Should the daemon's local state file record more than the build version —
   in particular, registered clusters — so a restart is less costly?** This
   edges toward the durable-state decision the change explicitly defers, and
   the line between "reconnection hint" and "persistence" needs drawing before
   implementation rather than during it.
4. **Do any of this change's new rules belong in the `@conformance` subset?**
   Enforcement refusal at registration (D2) is genuinely cluster-dependent and
   arguably must be proven against the non-enforcing cluster CI already stands
   up; the rest is control-plane logic. If any are tagged,
   `acceptance-tests/CONFORMANCE.md` gains rows and its recorded-exclusion
   table must stay accurate.
5. **Carried forward from `add-cloudbox-v1` Open Question 1 (evidence retention
   and GC).** A local release makes it less pressing — evidence dies with the
   daemon — but that *is* a retention policy, and it should be stated as one in
   the quickstart rather than left as an accident of implementation.

No in-force ADR needs revisiting. This design sits inside 0001, 0004, 0005,
0006, and 0008 as written; the release-boundary decisions (D1, D3, D5) are new
commitments rather than reversals, which is what the adr step should record.
