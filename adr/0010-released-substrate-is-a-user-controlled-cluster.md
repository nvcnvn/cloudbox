# 0010 — The released substrate is a user-controlled cluster the product does not provision

- Status: accepted
- Date: 2026-08-19
- Supersedes: —

## Context

The first public release targets the bottom rung of ADR 0005's ladder: sealed
sandboxes with zero integrations, run locally by a developer or an AI coding
agent. That rung needs a cluster, and `sandbox-lifecycle` required the product
to provide one — "one command MUST provision a local Kind cluster from the
application's substrate lockfile".

That requirement is satisfied only by the simulated driver. `provisionLocalCluster`
type-asserts an off-contract `NewCluster(name, enforcing, userControlled)`
method that the kube driver does not implement and, under ADR 0008's frozen
contract, must not acquire: the kube driver operates clusters named by
kubeconfig context, and conjuring one is a different responsibility wearing the
same method name. Meanwhile the conformance harness has been standing up
registered enforcing clusters and running real sandboxes on them since ADR 0008.

Owning provisioning means owning Kind, a CNI installation, a container runtime,
image loading, and every failure mode of each — on the user's machine, where
the product has the least information and the least recourse. It is a larger
surface than everything else in the first release combined, in exchange for one
command of convenience.

Handing cluster selection to the user introduces a hazard the managed topology
never had: a cluster that looks healthy and enforces no NetworkPolicy at all.
The `sandbox-seal` N7 probe already refuses to report such a sandbox sealed, but
that failure arrives several steps from its cause.

## Decision

The released product does not create clusters. A developer or agent registers a
cluster they control as a sandbox host; a documented script that stands up a
suitable local cluster lives outside the product boundary.

Registration proves NetworkPolicy enforcement and refuses a cluster that cannot
be shown to enforce, naming unproven enforcement — the same precondition the
conformance run already applies, moved to where the user makes the choice. The
N7 probe remains the last line; this is the early one.

Sandboxes on a user-controlled cluster hold identical seal and iteration
semantics, and their evidence is marked user-controlled and stays non-promotable
and non-postable, as ADR 0004 requires.

Product-managed local provisioning is recorded as direction, not requirement.

## Consequences

- `sandbox-lifecycle`'s local-provisioning rule is removed rather than modified:
  the property it protected (user-controlled evidence cannot be promoted or
  posted) survives, its mechanism does not.
- Onboarding gains a real cliff — a container runtime, a cluster, and an
  enforcing CNI stand between a new user and their first sandbox. The release
  accepts this in exchange for not owning that surface, and names the
  requirement in the quickstart rather than discovering it at first failure.
- The path the release ships on is the path continuous integration already
  exercises, so the substrate story is proven by the existing conformance run
  rather than by new provisioning code.
- Future managed topologies (shared team clusters, and eventually the managed
  offerings ADR 0008 defers) are additive: they register the same way and
  differ only in who controls the cluster.
- If the onboarding cliff proves fatal to adoption, a `cloudbox local up`
  wrapper is the recorded escape hatch, and it can be added without revisiting
  this decision's boundary — the product would then provision a cluster it
  still registers through this same path.
