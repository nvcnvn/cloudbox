# CloudBox acceptance suite

An independent Python (behave) project that executes the Gherkin specs under
`openspec/` against the running application. The runner extracts the Gherkin
from every `spec.md` into `.extracted/` on each run and composes the
**effective spec**: the source of truth (`openspec/specs/`) with every active
change's delta applied. `openspec/changes/archive/` never runs.

## Setup (once)

```sh
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
```

Go must be on PATH: the suite builds and boots `cloudboxd` (simulated cluster
driver, ADR 0007) before the run and shuts it down after.

## Running

```sh
.venv/bin/python run_acceptance.py                    # effective spec (default)
.venv/bin/python run_acceptance.py --specs            # source of truth only
.venv/bin/python run_acceptance.py --lint             # extract + gherkin-lint
.venv/bin/python run_acceptance.py --print-locations  # resolved composition
.venv/bin/python run_acceptance.py --dry-run          # scenario census, no run
```

Or from the repo root: `make acceptance`, `make acceptance-specs`,
`make lint-specs`.

**Always go through `run_acceptance.py`** — plain `behave` would run a stale
`.extracted/` tree; the environment tripwire fails loudly if you try.

## Reports

Every run writes an HTML report to `reports/behave-report.html`. Runner output
cites extracted paths: read `.extracted/X/spec.feature:N` as
`openspec/X/spec.md:N` (identical line numbers, by line fidelity).

## Layout

- `run_acceptance.py` — the single entry point (extract → compose → behave)
- `extract_gherkin.py`, `openspec_effective_spec.py` — canonical runner
  modules from the acceptance-test-authoring skill; edits must stay in port
  parity with the JS stack
- `environment.py` — behave hooks: builds/boots/stops cloudboxd, resets sim
  state per scenario, attaches page objects
- `steps/` — step definitions: intent-level only, no URLs or payload shapes
- `pages/` — page objects: all transport and payload knowledge lives here
- `.extracted/`, `reports/`, `.venv/` — generated, gitignored
