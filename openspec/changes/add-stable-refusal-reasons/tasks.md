# Tasks: add-stable-refusal-reasons

Implementation MUST honour the in-force ADRs under `adr/`: 0001–0006 and
0008–0012. ADR 0007 is superseded by 0008 and is historical context only.

`acceptance-tests/` already exists (Python/behave, `stack: python`), so the
first-time setup section does not apply and is omitted. New steps go in
`acceptance-tests/steps/developer_ergonomics_steps.py`, and the
troubleshooting steps in `user_documentation_steps.py` are reworked. Command
strings and JSON field names live in page objects, not in step definitions.

Standing constraints:

1. No HTTP status, exit status, or message wording changes (design non-goals).
2. The CLI MUST NOT link `internal/core`. `internal/refusal` imports nothing
   from the module (design D2).
3. `internal/cluster/cluster.go` MUST NOT change (ADR 0008).
4. Specs-zone and code-zone edits are committed separately. `docs/`,
   `glossary/` and `adr/` are code zone.

## 1. Foundations

- [x] 1.1 Add `internal/refusal`: `type Reason string`, the 20 constants from design D1, and `Catalog()` returning code and one-line summary in a stable order; the package imports nothing from the module
- [x] 1.2 `core.Error` gains `Reason` and `Subject`; replace `errf` with `refuse(status, reason, …)` and `notFound(subject, …)`, then delete `errf` so the build fails at every unconverted site
- [x] 1.3 Convert every call site in `internal/core` per design D1's mapping rule, file by file (including the intake `&Error{}` in `bundles.go` → `bundle-rejected`); `go build ./...` and `go vet ./...` clean; the default acceptance run (`make acceptance`) stays green with no status or message change
- [x] 1.4 `internal/server`: `writeErr` emits `reason` and `subject`; non-`core.Error` → `internal-error`; give the direct `writeJSON` error paths explicit reasons (design D3)

## 2. Developer ergonomics — red → green → commit per rule

- [x] 2.1 developer-ergonomics: every refusal names a stable reason from the published catalog — CLI `failure` carries `reason` and `subject` from the body; machine-readable output adds them; `cloudbox check` offline rejections carry `bundle-rejected` via `internal/intake`; steps for the outline and the offline scenario
- [x] 2.2 developer-ergonomics: a not-found refusal names the kind of thing that was not found — steps for the sandbox and application rows
- [x] 2.3 developer-ergonomics: the command line lists the refusal-reason catalog — add `cloudbox reasons` (offline, both renderings) and list it in usage; steps read the spec table from the extracted feature, not a copy in the test

## 3. Documentation — red → green → commit per rule

- [x] 3.1 Add **Refusal reason** to `glossary/business.md` and point **Refusal** at it; define it on `docs/concepts.md` (design D6); the existing concepts rule stays green
- [x] 3.2 user-documentation: every outcome the release names has a troubleshooting entry — the docs page object locates refusal entries by reason code from `cloudbox reasons -o json`; rewrite `docs/troubleshooting.md` with one entry per reason (headings carry the code; the two existing refusal entries become `enforcement-unproven` and `integration-missing`), and keep the three entries for outcomes that are not refusals
- [x] 3.3 `docs/cli-reference.md`: document `reasons` and the `reason`/`subject` fields of refused machine-readable output; `docs/agent-guide.md`: show branching on `reason` once, where the guide meets its first refusal; the CLI reference and agent guide rules stay green

## 4. Completion

- [x] 4.1 `make lint-specs`, `make acceptance` and `make acceptance-specs` all pass: zero pending or undefined steps, HTML report generated
- [ ] 4.2 `make conformance` passes on Kind + Calico; the `enforcement-unproven` row is checked against the non-enforcing cluster there too
