# 0004 — The control plane mints all evidence; CI is an untrusted trigger

- Status: accepted
- Date: 2026-08-14
- Supersedes: —

## Context

Evidence is only worth gating merges and promotions on if it cannot be forged.
CI pipelines run user-controlled code; CLIs drift across versions; local
clusters are user-controlled. If any of these could assert evidence, the
product's core metric (zero forged or pipeline-minted checks) is unenforceable.

## Decision

All validation, bundling, evidence gathering, signing, and enforcement run
server-side in the controller. The CLI is a thin client (create CRDs, watch
status); the offline `check` is the one deliberate advisory exception, with
server-side intake remaining authoritative. CI systems are untrusted triggers:
they may invoke runs and display evidence, never compute or assert it. Status
checks are posted only by the controller through the SCM integration. Evidence
from user-controlled (local) sandboxes is non-promotable and non-postable.

## Consequences

- "0 forged evidence checks" becomes an enforceable invariant, not a hope.
- There is no client-version drift in enforcement; old CLIs cannot weaken
  anything.
- The SCM integration (e.g. GitHub App) is a trusted, product-operated
  component and a hard dependency for L2+.
- Witnessed activity requires control-plane attribution (in-sandbox test Jobs,
  egress-proxy observation) — CI-reported test results carry no weight.
