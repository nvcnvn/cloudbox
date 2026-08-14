# 0005 — Add-on adoption ladder feeding existing CD, never replacing it

- Status: accepted
- Date: 2026-08-14
- Supersedes: —

## Context

Infrastructure products die on migration cost: platforms that demand teams
rewrite manifests, change CD, or hand over production on day one. CloudBox's
buyers (agent-adopting teams, existing Kubernetes teams) already have PRs,
branch protection, and Argo/Flux they trust.

## Decision

CloudBox is an add-on structured as a per-application adoption ladder — L1
sealed sandboxes (zero integrations), L2 signed evidence checks consumed by the
team's own branch protection, L3 write-back promotion committing proven bundles
to the team's GitOps repo (their controller applies; CloudBox verifies the
applied digest), L4 strict single-writer mode. Every level is independently
valuable; nothing below L3 has production write access; direct apply exists
only as the L4 alternative. The product ships no CD engine, no manifest
language, no mesh, no rollout engine — those remain the team's or arrive as
integrations.

## Consequences

- Sandboxes, bundles, and evidence must carry up the ladder unchanged — no
  per-level rework is tolerable.
- The team's flow stays the mechanism of record; CloudBox owns proof, not
  process (no merge-rule ownership at L2).
- Ladder progression (% of applications advancing within 90 days) is the
  strategy's truth-teller metric.
- Feature pressure to "just apply it ourselves" below L4 must be refused; the
  write path is the top rung, opted into after trust is earned.
