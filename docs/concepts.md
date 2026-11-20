# Concepts

CloudBox runs your application's Kubernetes manifests in a sealed,
production-shaped sandbox and records what happened there as evidence. This
page defines the terms the rest of the documentation, the command line, and
evidence use. Each term has one meaning everywhere.

## The sandbox

### Sandbox

A disposable environment owned by one developer or agent. It is sealed before
anything runs in it, its substrate is checked, and it is destroyed when you
are done. Creating one needs no approval.

### Sandbox host

A Kubernetes cluster registered with CloudBox to run sandboxes. A cluster is
accepted as a host only if CloudBox can show that it enforces NetworkPolicy.
A cluster that cannot be shown to enforce it is refused.

### User-controlled cluster

A sandbox host that you control rather than CloudBox. In this release every
sandbox host is user-controlled. Evidence from a user-controlled cluster is
marked as such. It can never be posted as a pull-request check or used for a
promotion, because whoever controls the cluster could have shaped the run.

### Seal

What contains a sandbox. Ingress and egress are denied by default for the
sandbox's whole lifetime. Workloads can reach other services in the same
sandbox, cluster DNS, declared dependencies, and the hostnames on the
allowlist, and nothing else. The seal is in place before any workload is
admitted, and it is never weakened for a single sandbox.

### Allowlist

The hostnames (FQDNs) your application declares it needs to reach outside
the sandbox. It is part of the boundary contract. To reach a new host, you
change the declaration. Nothing is ever allowed on the fly.

### Allowlist proposal

When a workload is denied egress, CloudBox records the attempt: the
destination, the time, and the workload. Explaining a sandbox's status turns
those records into a ready-to-submit allowlist change for someone to review.
Accepting the proposal is a declaration change. The proposal itself allows
nothing.

### Containment statement

The control plane's own published claim about the seal. It lists what is
blocked, which residual channels remain, and which egress is recorded by
destination. The documentation never claims more than this statement does.

### Residual channel

A path the seal does not close. The two named residual channels are DNS
tunneling and exfiltration through an endpoint on your allowlist. The seal is
strong containment. It is not an unbreakable one, and CloudBox does not claim
that it is.

## What you declare

### Application

The unit you bring to CloudBox: its boundary contract, its bundles, and its
policy. Every sandbox and every piece of evidence belongs to one application.

### Boundary contract

The complete, finite list of things allowed to differ between environments.
There are exactly four kinds: secret names, ingress hostnames, the egress
allowlist, and dependencies on other applications. Anything else that differs
between your sandbox and production is a defect, not a configuration option.
Secret values are supplied per environment and never live in bundles.

### Bundle

A set of rendered Kubernetes manifests, identified by a digest of its content.
The same bundle is what you apply to a sandbox, what evidence is about, and
(at higher adoption levels) what is promoted. Bundles are the Kubernetes YAML
you already have, with no CloudBox-specific schema.

### Dependency stub

A stand-in endpoint for another application your application depends on,
used when that application is not running alongside yours. Evidence records
the dependency as stubbed, so a run against a stub is never presented as a run
against the real thing.

### Declared suite

The test command your application declares. Running it inside a sealed
sandbox makes its results witnessed activity in the evidence.

## The substrate

### Substrate

Everything a bundle assumes already exists in the cluster: the Kubernetes
version, CRDs, operators, admission policies, and storage, gateway, and
priority classes.

### Substrate lockfile

CloudBox's per-application record of the substrate your bundles actually
reference. Only what your bundles use is recorded and compared.

### Substrate parity

Whether a sandbox's substrate matches production's, over what the lockfile
covers. It has three outcomes:

- **matched**: compared, and the same.
- **mismatched**: compared, and different. Evidence from the run is invalid.
- **unverified**: not compared, because no production substrate is
  registered. This is the normal state in this release. Unverified is never
  reported as a match.

## Evidence

### Evidence

The record of a sealed sandbox run, gathered by machine and signed by the
control plane. It says what ran, at which fidelity level, for how long it
stayed healthy, and what the declared suite witnessed. It never says
"verified working": it covers the code paths that were exercised and nothing
more. An idle sandbox is reported as idle, not as tested.

### Fidelity level

How representative each datastore's data was in a run. Evidence names one
level per datastore: fixtures, schema-replay, profile-synthetic,
masked-snapshot, or live-clone.

## How you use it

### Sealed-iteration loop

The cycle an agent runs at the sandbox level: declare an application and its
contract, create a sandbox, apply a bundle, run the declared suite, explain
anything that was blocked, amend the declaration, read the evidence, and
destroy the sandbox.

### Refusal

An outcome in which CloudBox declines a request, names the reason, and records
nothing. A refusal means the product is working as designed. When an
operation depends on an integration this release does not have, CloudBox
refuses rather than pretending to succeed.

### Adoption level

CloudBox is adopted one rung at a time, per application. Each rung is useful
on its own, and nothing below L3 can write to production.

#### L1 — Sealed sandboxes

Sealed sandboxes and evidence on your own machine, with no integrations. This
is the sandbox level, and it is the only level available in this release.

#### L2 — Evidence checks

Evidence is posted to the pull request as a signed status check that your own
branch protection consumes. This level is not available in this release.

#### L3 — Write-back promotion

Proven bundles are committed to your GitOps repository, and your own
controller applies them. This level is not available in this release.

#### L4 — Strict single-writer mode

CloudBox becomes the only writer to production, applying directly behind
approval gates. This level is not available in this release.
