# 0012 — Refusals carry a stable, append-only reason

- Status: accepted
- Date: 2026-11-20
- Supersedes: —

## Context

CloudBox refuses on purpose and often. It refuses rather than simulate
(ADR 0009), and it refuses clusters it cannot prove enforce policy
(ADR 0010). Its primary user is an AI agent. A refusal was an HTTP status
plus prose, and the command line collapsed every refusal to exit status `1`.
An agent could not tell "register a host first" from "not your sandbox"
without parsing prose, and the developer-ergonomics spec forbids requiring
that. The troubleshooting guide could not be checked against the product,
because there was nothing stable to check it against (ADR 0011).

## Decision

Every refusal names a **reason**: a short code from a closed catalog, carried
beside the human message. Reasons are coarse and grouped by what the caller
does next, not by call site. The catalog lives in a leaf package with no
enforcement in it, and the command line lists it without a control plane.
The catalog is append-only. A published reason is never removed, renamed, or
given a new meaning. Programs and documentation branch on reasons. The
message is for people and may change freely.

## Consequences

- A change that adds a refusal MUST give it an existing reason or add one.
  Adding one takes an edit to the catalog spec table and a troubleshooting
  entry, both of which the acceptance suite checks.
- Removing or redefining a reason is a breaking change and needs an explicit
  spec delta saying so.
- Message wording is no longer a compatibility surface.
- Troubleshooting coverage of refusals is checked against the binary
  (ADR 0011's preferred form) instead of an enumerated table.
