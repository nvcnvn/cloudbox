# 0002 — Content-addressed bundle digest as the universal identity

- Status: accepted
- Date: 2026-08-14
- Supersedes: —

## Context

Diff, evidence, soak accumulation, merge-time evidence transfer, promotion,
and live-state verification all need one identity for "the change." Candidates:
git commit SHA, image tags, or a content address over the rendered manifests.
Git SHAs identify sources, not what actually runs; rebases and re-renders can
change bytes without changing intent, and vice versa.

## Decision

Every apply produces a content-addressed bundle (rendered manifest set +
digest), recorded server-side. The digest is the sole identity used by diff,
evidence, soak inheritance, promotion, and applied-state verification — the
bytes that ran are the bytes that ship. Renders must therefore be
deterministic (pinned charts/values; no cluster lookups, timestamps, or
randomness); a non-deterministic render fails intake.

## Consequences

- Evidence transfer at merge is sound: identical digest ⇒ identical workload,
  so soak time and evidence carry over; any byte change ⇒ stale evidence.
- Write-back promotion is verifiable end-to-end (live state hash-checked
  against the promoted digest).
- Bundle bytes are immutable through the pipeline; per-environment adjustments
  must be digest-preserving recorded transforms, never edits.
- Determinism is a hard intake gate teams must meet before anything else works.
