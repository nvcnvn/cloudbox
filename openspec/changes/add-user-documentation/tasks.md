# Tasks: add-user-documentation

Implementation MUST honour the in-force ADRs under `adr/`: 0001–0006 and
0008–0011. ADR 0007 is superseded by 0008 and is historical context only.

`acceptance-tests/` already exists (Python/behave, `stack: python` in
`openspec/config.yaml`), so the first-time setup section does not apply and is
omitted. New steps land in `acceptance-tests/steps/user_documentation_steps.py`,
plus the existing `developer_ergonomics_steps.py` for the usage-listing rule.
Documentation access goes through a new page object,
`acceptance-tests/pages/docs.py`, which holds file paths, headings, and
required phrases. Step definitions stay free of paths, selectors, and command
strings.

Standing constraints:

1. Checks assert coverage and required facts, never exact wording (design D3,
   ADR 0011).
2. A page that restates a product fact is checked against the product, not
   against a copy kept in the test (design D2).
3. `internal/cluster/cluster.go` MUST NOT change (ADR 0008). The CLI MUST NOT
   link `internal/core` (ADR 0004).
4. Specs-zone and code-zone edits are committed separately. `docs/`,
   `glossary/`, and `adr/` are code zone.

Each rule task takes one spec Rule red → green: run the suite so its scenarios
fail for the right reason, write the page or code until they pass, then
commit. Sections 2–6 wait on the `add-local-agent-mvp` tasks named in their
headings (design D7).

## 1. Foundations — no MVP dependency

- [x] 1.1 Resolve design Open Question 2 first — does this change gate the MVP's completion (MVP 5.2 waits for these docs) or follow it (MVP archives first)? Record the answer and its reason in design.md. It decides whether sections 2–6 are part of the release
- [x] 1.2 Review `glossary/business.md` and `glossary/technical.md` (seeded during planning) against the source-of-truth specs and the MVP deltas. Every business term is one the concepts page must define (design D6), so add any missing product term and remove anything that is implementation detail
- [x] 1.3 Add `acceptance-tests/pages/docs.py`: reads the files under `docs/` and the README, follows relative links and anchors, locates entries by heading, and matches required phrases case-insensitively
- [x] 1.4 user-documentation: the documentation has one front door that reaches every page — create `docs/README.md` (index) and link it from the repository README; if MVP 4.5 has already landed a quickstart elsewhere, move it to `docs/quickstart.md` and update the `release-integrity` quickstart page object (design Open Question 3) — red → green → commit
- [x] 1.5 user-documentation: product terms are defined in the concepts page — write `docs/concepts.md` defining every business glossary term, and describe L1–L4 with only the sandbox level presented as available — red → green → commit
- [x] 1.6 user-documentation: the limitations page states what this release does not do — write `docs/limitations.md` covering no authentication, no durable control-plane state, evidence lost when the daemon stops (MVP design Open Question 5), and the refused operations — red → green → commit

## 2. Cluster setup — after MVP 2.1 and 2.5

- [x] 2.1 user-documentation: the cluster setup guide takes an operator from no cluster to a registered host — write `docs/cluster-setup.md`: enforcing-CNI requirement, the bootstrap script at the path MVP 2.5 chose, registration, and the non-enforcing refusal with what to change. The step asserts the named script exists and is not under `hack/conformance/` — red → green → commit

## 3. Containment — after MVP 1.6

- [x] 3.1 user-documentation: the containment page agrees with the product's containment statement — write `docs/containment.md`. The step fetches `/v1/containment-statement` from the suite's running control plane and asserts every blocked channel, residual channel, and recording limit appears on the page; "unbypassable" appears nowhere — red → green → commit

## 4. Command line — after MVP 3.3 and 3.4

- [x] 4.1 developer-ergonomics: the command line lists every command it accepts — replace the placeholder usage in `cmd/cloudbox/main.go` with the command list. The page object holds the spec-required command set (design D4) — red → green → commit
- [x] 4.2 user-documentation: the CLI reference matches the command line in both directions — write `docs/cli-reference.md`: each command with its flags, the machine-readable rendering, and the exit status of each outcome. The steps check the reference against the usage listing, per-command help, and actual exit statuses — red → green → commit
- [x] 4.3 user-documentation: the agent guide walks the whole sealed loop with commands that exist — write `docs/agent-guide.md` covering all eight loop stages, with the explain step shown in machine-readable form. The step extracts each example command from fenced `sh` blocks and checks it against the CLI's help — red → green → commit

## 5. Troubleshooting — after MVP 1.4, 2.1, and 4.3

- [x] 5.1 user-documentation: every outcome the release names has a troubleshooting entry — write `docs/troubleshooting.md` with one heading per outcome in the spec's Examples table, each with "What it means" and "What to do" — red → green → commit
- [x] 5.2 Record design Open Question 1 (stable refusal reasons) as a follow-up proposal or as an explicit decision not to pursue it; do not implement it in this change

## 6. Completion

- [x] 6.1 Default run green: `make acceptance` passes with zero pending or undefined steps from this change, and no scenario here is tagged `@conformance`
- [x] 6.2 Verify the composition: `.extracted/` rebuilt, `make lint-specs` clean, no duplicate scenarios between this change's `developer-ergonomics` delta and its source-of-truth spec
- [x] 6.3 Boundary check: `internal/cluster/cluster.go` unchanged, and `cmd/cloudbox/` imports no `internal/core` package
- [ ] 6.4 Walk the documentation as a stranger would: starting from the repository README with no prior knowledge, reach a sealed sandbox, read a blocked attempt, and find the remedy for a refused registration using only `docs/`. Every point where knowledge outside the docs was needed is a defect in the docs (design D3's clarity check)
