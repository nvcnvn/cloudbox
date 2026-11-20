# Design: add-user-documentation

## Context

After `add-local-agent-mvp`, the release has two pieces of user-facing prose:
a README (task 4.6) and a quickstart (task 4.5). The `release-integrity`
capability checks exactly two sentences in the quickstart. Everything else an
operator needs lives in places written for contributors: the ADRs under
`adr/`, the specs under `openspec/specs/`, `acceptance-tests/README.md`, and
`hack/conformance/README.md`.

Three things are true of the product today, and they shape this design:

- **The product already publishes some of its own claims.** The control plane
  serves `GET /v1/containment-statement`. The MVP gives the CLI structured
  output and distinct exit statuses (its D7). These are authoritative, and the
  documentation must agree with them rather than restate them.
- **The CLI does not say what it accepts.** `cmd/cloudbox/main.go` prints
  "Commands are added as their capability lands". A CLI reference has nothing
  to be checked against.
- **Refusals are free text.** `core.Error` carries a `Message` and an HTTP
  status. There is no stable reason identifier a troubleshooting entry could
  be keyed on.

The acceptance harness is Python/behave. Page objects live in
`acceptance-tests/pages/`, and step definitions stay free of selectors, paths,
and command strings.

## Goals / Non-Goals

**Goals:**

- An operator can learn the product's terms, limits, setup, commands, and
  failure modes from the repository without reading a spec or an ADR.
- Every page that restates something the product publishes is verified
  against the product by the acceptance suite.
- Documentation lands in an order that does not force rewrites: pages whose
  subject already exists land first.

**Non-Goals:**

- An HTTP API reference. The command line is the supported surface (MVP D7).
- A site generator, hosting, versioned docs, or search.
- Respecifying the quickstart or README content, which the MVP owns.
- Contributor documentation.

## Decisions

### D1: Plain Markdown under `docs/`, rendered by the code host

**Decision.** The documentation set is Markdown files in `docs/`, with
`docs/README.md` as the index. The repository README links the index. The MVP
quickstart lives at `docs/quickstart.md`. Pages link each other with relative
links.

**Why.** It needs zero toolchain and zero hosting, and the page object can
read the files directly. It renders on GitHub, and an agent with the
repository checked out can read it offline.

**Alternatives.** A static site generator (MkDocs, Hugo) was rejected for now:
it adds a build, a publish job, and a second place for links to break, all
before there is enough content to need navigation. Plain Markdown in `docs/`
is the input every generator accepts, so adopting one later costs nothing.
Docs inside `cmd/cloudbox` help text only was rejected: concepts and
containment are not command help.

### D2: Restated claims are checked against the product, never reviewed by eye

**Decision.** Where a page restates something the product publishes, the
acceptance step obtains that fact from the product and checks the page
against it:

| Page | Authority it is checked against |
|---|---|
| CLI reference | the CLI's usage listing and per-command help |
| CLI reference, exit statuses | actual exit statuses of a success, a refusal, and an unreachable control plane |
| Agent guide | each example command parsed by the CLI's help |
| Containment | `GET /v1/containment-statement` from a running control plane |
| Cluster setup | the existence and location of the named bootstrap script |
| Concepts | `glossary/business.md` |

Pages with no product authority (limitations, troubleshooting) are checked for
required statements, the same way `release-integrity` checks the quickstart.

**Why.** The product's doctrine is that claims are checked, never assumed. A
reference that drifts from the binary is worse than none, because an agent
will trust it.

**Alternatives.** Generating the reference from code was rejected: the CLI
does not use a framework with doc generation, and generation produces a
reference, not guides. It also cannot check the containment page. Review-only
documentation was rejected because it drifts silently, and drift is the
failure this change exists to prevent.

### D3: The checks assert coverage and required facts, not wording

**Decision.** Steps check that a term is defined, that a command has an entry,
that a statement's channels appear, and that a forbidden word is absent. They
do not compare paragraphs. The page object locates entries by heading and
matches facts by case-insensitive phrase. Each page's required phrases are
held in the page object, not in step text.

**Why.** Exact-text assertions make every copy edit a test failure, and the
docs then stop being edited.

**Trade-off.** A page can satisfy the checks and still be unclear. Clarity is
verified by MVP task 5.6, which walks the release as a stranger would, and the
equivalent completion task here.

### D4: The CLI enumerates its commands; the expected set comes from the specs

**Decision.** `cloudbox` usage lists every command it accepts (the new
`developer-ergonomics` rule). The page object holds the set of commands the
specs require — `check`, `status`, `logs`, `exec`, `port-forward`, and the MVP
loop verbs — and the step asserts that the usage lists exactly that set. The
CLI reference is then checked against the usage, not against a list kept in
the test.

**Why.** Without this, "every command is documented" has no oracle.
Enumeration is a one-line-per-command change to the usage text and it helps
agents directly.

**Alternatives.** Parsing the Go source for the command switch was rejected as
brittle and as coupling a test to implementation structure.

### D5: Troubleshooting is keyed by outcome, enumerated from the specs

**Decision.** The troubleshooting guide has one heading per named outcome. The
`Examples:` table in the spec is the enumeration, and the page object maps each
outcome to its heading. Each entry has a "What it means" part and a "What to
do" part.

**Why.** Refusals are free text today (`core.Error.Message`). Keying entries on
message text would break on every rewording.

**Trade-off.** A new refusal added by a future change does not automatically
demand an entry. It needs a new `Examples:` row. Open Question 1 asks whether
refusals should carry a stable reason so that the coverage check can come from
the product instead.

### D6: The concepts page renders the business glossary

**Decision.** `glossary/business.md` is the canonical list of product terms.
The concepts page defines each one in user language, and the step checks that
coverage. `glossary/technical.md` holds implementation terms and is not
rendered for users.

**Why.** It gives one canonical vocabulary for specs and docs, and a
mechanical check that the docs keep up when a term is added.

### D7: Sequenced behind the MVP, page by page

**Decision.** Pages land when their subject exists:

1. Now: glossary, index, concepts, limitations, and the README link.
2. After MVP 2.1 and 2.5: cluster setup.
3. After MVP 1.6: containment.
4. After MVP 3.3 and 3.4: the CLI usage listing, the CLI reference, and the
   agent guide.
5. After MVP 1.4, 2.1, and 4.3: troubleshooting.

**Why.** Writing the CLI reference before the verbs exist means writing it
twice.

## Risks / Trade-offs

- **[This change's scenarios join the effective spec, so `make acceptance`
  stays red until the docs land. MVP task 5.2 requires that run green with
  zero undefined steps.]** -> Accepted: the documentation is part of the
  release (Open Question 2, resolved), so the MVP is not complete until these
  pages exist. Until then the red scenarios are an accurate statement of what
  the release still lacks.
- **[Phrase-coverage checks pass on unhelpful prose.]** -> D3 accepts this. A
  stranger walk-through is the clarity check, and it is a completion task.
- **[The containment check needs a running control plane.]** -> The sim suite
  already boots one. `/v1/containment-statement` is driver-independent, so the
  check needs no `@conformance` tag.
- **[The glossary becomes a second place to edit when a term changes.]** -> The
  concepts check fails when a business term has no definition, so the glossary
  and the docs cannot silently diverge in that direction.

## Migration Plan

This is additive only. `docs/` and `glossary/` are new. The README gains one
link. The CLI usage text changes from a placeholder to a list, which changes
nothing for any caller that does not parse usage text. Rollback is deleting
the files and reverting the usage text.

## Open Questions

1. **Carried forward (2026-11-20): stable refusal reasons are a follow-up, not
   part of this change.** A `reason` field beside `core.Error.Message` would
   let troubleshooting coverage be checked against the product (D2) instead
   of an enumerated table (D5), and would give agents something finer than
   exit statuses to branch on. It changes every error response's shape, so it
   belongs in its own `developer-ergonomics` change. Until then, D5's
   enumerated table stands, and a change that adds a refusal operators can
   meet must add a troubleshooting row (ADR 0011).
2. **Resolved (2026-11-20): the documentation is part of the release and gates
   the MVP's completion.** MVP task 5.2 (default run green) waits for this
   change's sections 2–5, and the two changes complete together. Reason: a
   release whose limits, refusals, and containment boundary are documented
   only in specs has not told its operators what it is. This is the same
   honesty the MVP enforces in code, and MVP task 5.6 (walk the release as a
   stranger) already assumes the pages exist.
3. **Resolved (2026-11-20): the quickstart lives at `docs/quickstart.md`.**
   MVP task 4.5 placed it there, linked from the documentation index, so no
   move was needed. `release-integrity`'s quickstart steps read it through the
   same docs page object.
