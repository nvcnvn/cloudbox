# Proposal: add-stable-refusal-reasons

## Why

A **refusal** carries an HTTP status and a sentence of prose. The command line
turns that into exit status `1` and passes the sentence through. An agent that
needs to know *which* refusal it met, for example "register a sandbox host
first" versus "this sandbox is not yours", has only two options. It can parse
the prose, which the developer-ergonomics spec says it MUST NOT have to do. Or
it can give up. The exit statuses separate refused from unreachable. They do
not separate one refusal from another.

The same gap shows up in the documentation. `user-documentation` requires a
troubleshooting entry for each outcome the release names, but the list of
outcomes is an `Examples:` table someone keeps by hand
(`add-user-documentation` design D5). A new refusal does not demand an entry
until someone remembers to add a row. That design left this as Open
Question 1 and carried it forward as this follow-up.

## What Changes

- **Every refusal names a stable reason.** The error body gains a `reason`
  field beside `error`. A reason is a short kebab-case code, for example
  `not-found`, `not-owner`, `integration-missing`, `enforcement-unproven`,
  `no-sandbox-host`, `not-sealed` or `bundle-rejected`. Reasons are coarse:
  each groups refusals by what the caller does next, not by which line of code
  produced them. The initial catalog has about 20 reasons covering today's 93
  refusal sites.
- **`not-found` names its subject.** A `not-found` refusal also carries a
  `subject`, the kind of thing that was missing (application, sandbox,
  cluster, …), so a program can tell what it got wrong without parsing a
  quoted name.
- **The reasons form a published, closed catalog.** A new `cloudbox reasons`
  command lists every reason with a one-line summary. It works offline and in
  both renderings. A reason is never renamed or reused for a different
  meaning. Adding a reason is allowed, and removing one is a breaking change.
- **The command line carries the reason through.** In machine-readable mode, a
  refused command's output includes `reason` (and `subject` when present)
  beside `error` and `outcome`. Exit statuses do not change.
- **The offline `check` uses the same reason.** A bundle that `cloudbox check`
  rejects offline reports `bundle-rejected`, the same reason `apply` reports
  when the control plane rejects it.
- **Troubleshooting coverage is checked against the product.** The
  troubleshooting guide holds one entry per reason in the catalog, keyed by
  the reason code. The acceptance check reads the catalog from the binary, not
  from a hand-kept table. Outcomes that are not refusals (egress proxy
  unavailable, substrate parity unverified, control plane unreachable) keep
  their enumerated entries.

### Non-goals

- **Changing HTTP statuses or exit statuses.** Each refusal keeps its current
  status and the command line keeps its four exit statuses. The reason is
  additive.
- **Rewording refusal messages.** The prose stays as it is. It is for people,
  and it may now change without breaking a program.
- **Fine-grained, per-site reasons.** Reasons are deliberately coarse; the
  prose and `subject` carry the detail.
- **Localised messages.** Possible later, now that programs need not read the
  prose, but not part of this change.
- **An HTTP API reference.** HTTP remains unsupported as a surface (ADR 0011).
  The catalog is published through the command line.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `developer-ergonomics`: adds the requirements that every refusal names a
  stable reason from a published catalog, that `not-found` names its subject,
  and that the catalog is listed by the command line and is append-only.
- `user-documentation`: troubleshooting coverage for refusals comes from the
  catalog. Every catalogued reason has an entry keyed by its code. The
  enumerated `Examples:` table keeps only the outcomes that are not refusals.

## Impact

**Modified code**
- `internal/core` — `Error` gains `Reason` and `Subject`, and every refusal
  site names a reason. The `errf` helper requires one, so a refusal without a
  reason does not compile.
- New leaf package `internal/refusal` holds the catalog: the codes, their
  summaries, and nothing else. `internal/core`, `internal/intake` and
  `cmd/cloudbox` import it. The CLI still does not link `internal/core`.
- `internal/server` — `writeErr` and the handful of direct `writeJSON` error
  paths emit `reason`. Non-`core.Error` failures report `internal-error`.
- `cmd/cloudbox` — the new `reasons` command; `failure` carries `reason` and
  `subject`; `check` rejections carry `bundle-rejected`; usage lists `reasons`.
- `docs/troubleshooting.md` and `docs/cli-reference.md` — one entry per reason,
  plus the `reasons` command.
- `acceptance-tests/` — new steps for the reason rules; the troubleshooting
  page object locates entries by reason code.

**In-force ADRs**
- 0004 (thin CLI; the control plane is the authority): the CLI reads the
  catalog from a leaf package that holds no enforcement. The control plane
  still decides every refusal.
- 0009 (refuse rather than simulate): `integration-missing` is the reason
  every such refusal names.
- 0011 (docs verified against the product): this change moves troubleshooting
  coverage from "no product authority" to "checked against the product".

**Risk if not done**
- Agents either parse prose, which breaks on every rewording, or treat all
  refusals alike. Troubleshooting falls behind each new refusal without any
  check noticing.
