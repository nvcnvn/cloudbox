# Proposal: add-user-documentation

## Why

`add-local-agent-mvp` makes CloudBox something that can be handed to someone,
but what it hands over is a binary, a two-line README, and — once task 4.5
lands — a quickstart. The quickstart gets an operator to a first **sandbox**.
It does not explain what a **seal**, a **boundary contract**, or **substrate
parity** is. It does not list the commands, what their exit statuses mean, or
what to do when the product refuses. It does not say what the release
deliberately leaves out.

The primary user is an AI coding agent that reads no prose when it hits an
error. Its operator does read prose, though, and the operator decides whether
to trust the box. Every claim the product makes about itself — containment,
honesty about **evidence**, refusing rather than simulating — is weakened if
the only way to learn its limits is to hit them.

There is a second reason. This product's doctrine is that claims are checked,
never assumed. Documentation is a set of claims about the product. If it is
not checked against the product, it will drift from it. So this change
specifies the documentation the same way the product is specified, and the
acceptance suite verifies it.

## What Changes

- **A documentation set under `docs/`, reached from the README.** The README
  (MVP task 4.6) stays the front door. It links a documentation index, and
  every page is reachable from that index.
- **Concepts.** One page defines the product's terms in user language:
  **bundle**, **boundary contract**, **seal**, **allowlist**, **substrate**,
  **substrate parity**, **evidence**, **user-controlled cluster**, and the
  **adoption levels** L1–L4. It says plainly that only L1 is available in this
  release.
- **Limitations.** One page states what this release does not do: no
  authentication, no durable control-plane state, evidence that is lost when
  the daemon stops, and L2–L4 operations that refuse.
- **Cluster setup.** A guide that takes an operator from no cluster to a
  registered **sandbox host**. It covers the enforcing-CNI requirement, the
  supported bootstrap script (MVP task 2.5), registration, and why a cluster
  is refused.
- **CLI reference.** Every command, its flags, the machine-readable rendering,
  and the exit statuses. It is verified against the CLI in both directions, so
  it can neither omit a command nor document one that does not exist.
- **Agent guide.** The **sealed-iteration loop** — declare, create, apply,
  test, explain, amend, read evidence, destroy — walked with commands the
  suite proves exist. Blocked egress and the allowlist proposal are shown as
  data, because that is how an agent reads them.
- **Containment.** The threat model as an operator reads it. It is checked
  against the containment statement the control plane publishes, so the page
  can never claim more than the product does.
- **Troubleshooting.** One entry per outcome the release names: a cluster
  refused for unproven enforcement, an operation refused for a missing
  integration, an egress proxy reported unavailable, parity reported
  unverified, and an unreachable control plane. Each entry says what the
  outcome means and what the operator can do.
- **The CLI lists its own commands.** Today `cloudbox` prints "Commands are
  added as their capability lands". The CLI reference cannot be checked
  against a CLI that does not say what it accepts.
- **A product glossary.** `glossary/business.md` and `glossary/technical.md`
  hold the canonical terms. The concepts page is the user-facing rendering of
  the business glossary.

### Non-goals

- **The quickstart and README content.** They stay in `add-local-agent-mvp`
  (tasks 4.5 and 4.6), which already specifies them. This change links them
  into the documentation set and does not respecify them.
- **An HTTP API reference.** The MVP makes the command line the product
  surface (its D7: one surface, two renderings). Documenting the 63
  `cloudboxd` routes would turn HTTP into a supported contract that no change
  has decided to support.
- **A documentation site.** Plain Markdown in the repository, rendered by the
  code host. A site generator, hosting, and versioned docs come later, if at
  all.
- **Contributor documentation.** ADRs, specs, `acceptance-tests/README.md`,
  and `hack/conformance/README.md` already serve contributors and are not
  touched.
- **Translations.**

## Capabilities

### New Capabilities

- `user-documentation`: The documentation set the release publishes, and the
  rules that keep it true. A single front door reaches every page. Product
  terms are defined before use. Limitations are stated, not discovered. The
  CLI reference and the agent guide match the CLI. The containment page agrees
  with the control plane's published statement. Every outcome the release
  names has a troubleshooting entry.

### Modified Capabilities

- `developer-ergonomics`: adds the requirement that the command line lists
  every command it accepts. A program and a reader can then learn the surface
  from the binary itself, and the CLI reference can be verified against it.

## Impact

**New files**
- `docs/` — index, concepts, limitations, cluster setup, CLI reference, agent
  guide, containment, troubleshooting. The MVP quickstart joins this
  directory.
- `glossary/business.md`, `glossary/technical.md`.
- `acceptance-tests/pages/docs.py` (a page object over the documentation
  files) and `acceptance-tests/steps/user_documentation_steps.py`.

**Modified code**
- `cmd/cloudbox/main.go` — the usage text enumerates the commands.
- `README.md` — links the documentation index. It is otherwise owned by MVP
  task 4.6.

**Dependencies on `add-local-agent-mvp`**
- Concepts, limitations, the index, and the glossary can land now.
- Cluster setup depends on MVP tasks 2.1 and 2.5. The CLI reference and agent
  guide depend on 3.3 and 3.4. Containment depends on 1.6. Troubleshooting
  depends on 1.4, 2.1, and 4.3.

**In-force ADRs**
- 0004 (thin CLI; the control plane is the authority), 0005 (adoption ladder,
  which the concepts page explains), 0009 (refuse rather than simulate, which
  the limitations and troubleshooting pages describe), and 0010 (the released
  substrate is a user-controlled cluster, which the setup guide describes).
  A new ADR records that user documentation is specified and verified against
  the product, never restated independently.

**Risk if not done**
- The release's honesty is enforced in the code and invisible to the people
  deciding whether to trust it. Operators learn the residual channels, the
  missing trust boundary, and evidence loss from incidents rather than from
  the docs.
