# Design: add-stable-refusal-reasons

## Context

A refusal today is `core.Error{Status, Message, Findings}`. `internal/server`'s
`writeErr` renders it as `{"error": <message>, "findings": [...]}` with the
HTTP status. A handful of server handlers write `{"error": ...}` directly:
invalid JSON (400), an out-of-contract contract (422), the removed `local`
option (400), and the sim-only raw-object routes. The command line maps any
non-2xx to exit status `1` and puts the message in `error` beside
`outcome: "refused"`.

There are 93 `errf` call sites in `internal/core` and one direct `&Error{}` for
intake rejection. `cloudbox check` rejects bundles offline through
`internal/intake` and never reaches `internal/core`.

Constraints from in-force ADRs: the CLI is a thin client and the control plane
decides every refusal (0004). The CLI must not link `internal/core`, a
standing constraint since `add-user-documentation`. HTTP is not a supported or
documented surface (0011). Documentation that restates a product fact is
checked against the product (0011).

## Goals / Non-Goals

**Goals:**

- Every error response carries a `reason` from a closed catalog, and a
  refusal without one cannot compile.
- `not-found` refusals also carry a `subject`.
- The catalog is listed by the command line, offline, in both renderings.
- Troubleshooting coverage of refusals is derived from the catalog.

**Non-Goals:**

- Changing any HTTP status, exit status, or message wording.
- Per-site reasons, localisation, or an HTTP API reference.

## Decisions

### D1: A coarse catalog keyed by what the caller does next

**Decision.** About 20 reasons, each grouping the refusals that call for the
same response. The initial mapping:

| Reason | Refusals it covers (today's sites) | Caller's next move |
|---|---|---|
| `not-found` | application, sandbox, cluster, promotion, bundle digest, workload, datastore, PR binding, merge result, GitOps/applied record | fix the reference |
| `already-exists` | application name taken | pick another name, or use the existing one |
| `invalid-request` | invalid JSON, unknown mode/level/event type, missing name, the removed `local` option | fix the request |
| `out-of-contract` | contract fields outside the boundary contract, undeclared secret, dangling dependency | change the declaration |
| `bundle-rejected` | intake rejection with findings, online and offline | fix the manifests the findings name |
| `quota-exceeded` | apply over the application's CPU quota | shrink the bundle or choose a capacity mode |
| `not-owner` | sandbox owned by another identity | use your own sandbox |
| `not-permitted` | break-glass role, approver role, self-approval, managed production write, real-data levels disabled or agent-owned | ask someone who holds the role or policy |
| `seal-not-weakened` | per-sandbox allowlist addition | amend the application's contract |
| `evidence-not-postable` | evidence from a user-controlled cluster used to post or promote | expected at L1; read evidence locally |
| `unwitnessed-results` | pipeline-reported test results | run `cloudbox test` |
| `integration-missing` | every ADR 0009 refusal | none at L1 |
| `enforcement-unproven` | registration probe failed; evidence refused under N7 | use an enforcing CNI |
| `no-sandbox-host` | no host registered; app's sandbox cluster is not a registered host | register a host |
| `not-sealed` | apply or witnessed run before the seal is active | wait for or re-create the seal |
| `wrong-state` | sandbox destroyed, promotion not approved, already approved, nothing awaiting verification | re-read state |
| `prerequisite-missing` | no test suite declared, no SCM for the app, no production cluster or database, no branching-capable service, strict mode without a break-glass role | declare or register the prerequisite |
| `promotion-blocked` | evidence invalid, parity unverified | produce valid evidence |
| `audit-unavailable` | audit sink down | retry later |
| `internal-error` | any failure that is not a `core.Error` | report it |

Mapping rule for ambiguous sites: a missing *record* is `not-found`, while an
existing object in the wrong state is `wrong-state`. A missing *declaration or
registration* that the operation depends on is `prerequisite-missing`, except
for the sandbox host, which has its own reason because it is the first
refusal every new user meets.

**Why.** Agents branch on a small closed set. A reason per site would add
about 50 codes that differ only in the noun, and the noun is what `subject`
is for. Coarse reasons also keep troubleshooting at one entry per reason
(D5).

**Alternatives.** One reason per distinct message (rejected by the user during
planning: too large a catalog and too long a troubleshooting page). Reusing
HTTP statuses as the reason (rejected: 403 alone covers six different next
moves).

### D2: The catalog is a leaf package; the reason is required at the call site

**Decision.** A new package, `internal/refusal`, declares `type Reason string`,
the constants, and `Catalog() []Entry{Code, Summary}`. It imports nothing from
the module. `core.Error` gains `Reason refusal.Reason` and `Subject string`.
`errf(status, format, ...)` becomes `refuse(status, reason, format, ...)`, and
`notFound(subject, format, ...)` sets status 404, `not-found`, and the
subject. The old `errf` is deleted, so a call site without a reason does not
compile. `internal/intake` and `cmd/cloudbox` import `internal/refusal` for
`bundle-rejected` and for listing.

**Why.** The type system enforces the "every refusal" rule better than any
test sweep. A leaf package lets the CLI list the catalog offline without
linking `internal/core`, and holds no enforcement, so ADR 0004's authority
line is intact. The control plane still decides which reason applies.

**Alternatives.** Having the control plane publish the catalog at a route
that `cloudbox reasons` fetches was rejected: the command would need a running
control plane to list constants, and it would make an HTTP route part of a
documented contract (ADR 0011). Deriving the status from the reason was
rejected: `enforcement-unproven` is 422 at registration and 409 at evidence,
and existing acceptance steps assert those statuses.

### D3: Wire shape is additive

**Decision.** Error bodies become
`{"error", "reason", "subject"?, "findings"?}`. `writeErr` emits `reason`
from `core.Error`. A non-`core.Error` emits `internal-error`. The direct
`writeJSON` error paths in `internal/server` gain explicit reasons
(`invalid-request` or `out-of-contract`). The sim-only raw routes use
`not-found`. The CLI's `failure` gains `reason` and `subject`, read from the
body. Machine-readable output adds them beside `error`, `outcome` and
`status`, and the human rendering is unchanged. Usage and unreachable
failures have no reason: `outcome` already distinguishes them, and they are
not refusals.

**Why.** No existing field changes meaning, so nothing that reads today's
output breaks.

### D4: Append-only is enforced by the spec's own table

**Decision.** The `developer-ergonomics` delta carries the full catalog as an
`Examples:` table, checked in both directions against `cloudbox reasons`.
Once archived, removing a reason means a delta that edits that table. That
step is visible, reviewed, and ought to be marked breaking.

**Why.** It needs no baseline file or release-diff tooling, and the record
lives where the requirement does.

**Trade-off.** Adding a reason now takes a spec edit, not only a code change.
That is intended: a new reason is a new contract with agents.

### D5: Troubleshooting entries are keyed by reason code

**Decision.** Each refusal entry's heading contains its code in backticks,
for example ``## `no-sandbox-host`: no sandbox host is registered``. The docs
page object finds the entry by code, using a reason list it obtains by running
`cloudbox reasons -o json`. Today's two refusal entries ("cluster refused…",
"operation refused…") become the `enforcement-unproven` and
`integration-missing` entries. The three outcomes that are not refusals keep
their headings.

**Why.** This is ADR 0011's preferred form: the coverage list comes from the
product, not from a copy kept in the test.

### D6: Refusal reason becomes a business term

**Decision.** Add **Refusal reason** to `glossary/business.md`, so the concepts
page must define it (existing `user-documentation` rule). Update the
**Refusal** definition to point to it.

## Risks / Trade-offs

- [A site gets a plausible but wrong reason during the mechanical sweep] →
  the mapping rule above is applied by file, and design review checks the
  task's diff against the table. The acceptance outline covers the five
  reasons an L1 user is most likely to meet.
- [`cloudbox reasons` from an older CLI against a newer remote control plane
  lists a stale catalog] → `cloudbox` and `cloudboxd` ship in the same
  archive, and the release's control plane is local (limitations page).
  Revisit if remote control planes become supported.
- [Coarse reasons hide distinctions an agent later needs] → adding a reason is
  allowed. Splitting one is an addition plus a narrowing of the old reason's
  sites, which the append-only rule permits as long as the old code keeps
  its meaning.
- [The troubleshooting page grows from 5 to about 23 entries] → accepted. Most
  are two sentences, and an agent reaching for the page needs the entry it
  met.

## Migration Plan

This is purely additive on the wire. A client that ignores `reason` sees no
change. Roll back by reverting the commit. No persisted state is involved.

## Open Questions

None. Granularity (D1) was settled during planning: coarse, with `subject` on
`not-found`.
