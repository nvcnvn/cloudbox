# 0011 — User documentation is verified against the product

- Status: accepted
- Date: 2026-11-20
- Supersedes: —

## Context

CloudBox's doctrine is that claims are checked, never assumed. Enforcement is
probe-verified (ADR 0001), evidence is minted only by the control plane
(ADR 0004), and unimplemented integrations refuse rather than simulate
(ADR 0009). The first public release adds a new kind of claim: user
documentation. Documentation says what the commands are, what exit statuses
mean, what the seal contains, and what the release does not do.

Prose that is reviewed only by eye drifts from the binary it describes. For
this product, drift is not cosmetic. The primary consumer is an AI agent that
trusts what it reads. A containment page that claims more than the control
plane's published statement would undo, in the docs, the honesty the code
enforces.

Some of these facts the product already publishes itself:
`/v1/containment-statement`, the CLI's usage and help, and the CLI's exit
statuses. Others have no product authority: limitations and troubleshooting.

## Decision

User documentation is a specified capability (`user-documentation`) and is
verified by the acceptance suite like any other behaviour.

Where a page restates a fact the product publishes, the acceptance step
obtains the fact from the product and checks the page against it. The page is
never the source of truth for that fact. Where no product authority exists,
the page is checked for the statements its spec requires.

Checks assert coverage and required facts, never exact wording.

## Consequences

- A change that adds or renames a command, changes an exit status, alters the
  containment statement, or introduces a new refusal operators can meet MUST
  update the documentation in the same change, or its acceptance run fails.
- The CLI must be able to list its own commands. That is the oracle for the
  CLI reference.
- Documentation that cannot be checked mechanically, such as clarity, tone,
  and completeness of explanation, is verified by a stranger walk-through task
  at release, not by the suite.
- An HTTP API reference is not published until a change decides HTTP is a
  supported surface. Once published, it would fall under this ADR and need an
  oracle of its own.
