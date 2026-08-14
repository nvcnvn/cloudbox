# Intent-Driven OpenSpec Schema

`intent-driven` is `behaviour-driven` plus durable Architecture Decision
Records: contributor intent is captured in a proposal, observable behaviour is
written as executable Gherkin scenarios inside Markdown specs (the acceptance
suite every change must keep green), technical design is constrained by
currently in-force ADRs, and each change completes an ADR review before task
planning.

- Good fit: product or platform changes with meaningful behaviour and
  long-lived design decisions, cross-module work, or architecture choices that
  future changes should honor.
- Not a good fit: small tactical fixes, docs-only changes, dependency bumps, or
  behaviour-only work without durable design decisions, where `behaviour-driven`
  (the same workflow without the ADR step) is enough.

## Activate

Set this in `openspec/config.yaml`:

```yaml
schema: intent-driven
stack: javascript # or python
```

## Stage Gates

Artifact order:

```text
proposal -> (specs, design) -> adr -> tasks
```

`specs` and `design` each require only the proposal and can proceed in
parallel; `adr` requires `design`; `tasks` requires `specs` and `adr`.

Gate expectations:

- `proposal` states why the change matters and lists the capabilities that need
  behaviour specs.
- `specs` creates one fenced-Gherkin Markdown spec per capability at
  `specs/<capability>/spec.md`, extracted and linted with `gherkin-lint` before
  the artifact is complete.
- `design` explains the implementation approach and accounts for currently
  in-force ADRs.
- `adr` writes the per-change ADR review manifest at
  `openspec/changes/<change>/adr.md` after design and before task planning.
  Durable repository-level ADR files are created only when the change
  introduces a major architectural decision that should persist beyond the
  change.
- `tasks` are planned only after specs and ADR review are complete: first-time
  acceptance-suite setup (once per project, keyed on `stack:`), then one
  red → green → commit task per pending step definition, then completion with
  the full suite green. Implementation honours in-force ADRs under
  `<repo>/adr/`.

## Spec Format

A spec is a standard Markdown `spec.md`: prose may appear anywhere, and ALL
executable Gherkin lives inside fences opened by ` ```gherkin ` at column 0.
Each file's fences concatenate into exactly one `Feature:` (the capability);
`Rule:` is one requirement described with SHALL/MUST, and every `Rule:` has at
least one Given/When/Then `Scenario:`. Delta operations are `# @openspec:`
comments inside a fence, immediately above the `Rule:` they apply to:

````markdown
# User data export

```gherkin
Feature: User data export

  # @openspec: ADDED
  Rule: Users can export their own data
    The system MUST let a user export their own saved data.

    Scenario: Successful CSV export
      Given a user has saved data
      When the user exports their data as CSV
      Then the system provides a CSV file containing the user's data
```
````

At test time the Gherkin is extracted to `.feature` files under
`acceptance-tests/.extracted/` and linted with `gherkin-lint`, per the
`acceptance-test-authoring` skill. The machine-readable definition of the
format (identical to `behaviour-driven`'s) is the `format:` block in
`schema.yaml`.

## Acceptance Enforcement

The workflow is governed by the same two rules as `behaviour-driven`:

1. **Acceptance tests must always pass.** Code without a driving spec delta is
   reverted and redone spec-first — never patched into passing.
2. **Specs and code are never modified together.** A unit of work touches
   either `openspec/` or application code, never both (`tasks.md` exempt), per
   the `bdd-zone-check` skill.

The acceptance suite — extraction, runners, reports, and linting — is defined
by the [`acceptance-test-authoring`](https://github.com/intent-driven-dev/skills/tree/main/.agents/skills/acceptance-test-authoring)
skill, which requires a `stack:` key in `openspec/config.yaml`. Read the
skill's documentation for the supported stacks and setup details.

## ADR Persistence

The `adr` artifact completion signal is the change-local review manifest at
`openspec/changes/<change>/adr.md`. Existing files under the repository-level
`adr/` folder are context for a new change; they are not completion evidence
for that change.

Durable ADR files are generated under the target repository's top-level `adr/`
folder only when the change introduces a major architectural decision that
should persist beyond the change. They are not written inside the OpenSpec
change folder. Accepted ADRs are immutable. If a future decision changes a
prior ADR, create a new ADR that supersedes the old one and leave the original
file unchanged.

## Validate

```bash
openspec schema validate intent-driven
```

## Associated Skills

This schema declares its companion skills in `skills.txt`; they are installed automatically by Step 6 of `AGENT_INSTALL.md` into `.agents/skills/`, sourced from [intent-driven-dev/skills](https://github.com/intent-driven-dev/skills).

- `acceptance-test-authoring` — the acceptance-suite contract: Gherkin extraction from `spec.md` fences, effective-spec composition, runner setup for both stacks, linting, and reports.
- `architectural-decision-records` — drafting/reviewing ADRs; includes MADR, Nygard, and Y-statement templates.
- `bdd-zone-check` — spec-first discipline and specs/code zone isolation, with a reference enforcement hook.
- `c4-diagrams` — C4-style architecture diagrams in ASCII or Mermaid.
- `gherkin-authoring` — writing and reviewing Gherkin/BDD scenarios.
- `glossary` — keeping domain/technical terms consistent across artifacts.
- `grill-me` — relentless interviewing to stress-test a plan or design.
- `openspec-git-discipline` — git hygiene for OpenSpec propose/apply/archive workflows.

For more schemas, refer to https://github.com/intent-driven-dev/openspec-schemas.
