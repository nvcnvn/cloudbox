.PHONY: build acceptance acceptance-specs lint-specs

build:
	go build -o bin/cloudboxd ./cmd/cloudboxd
	go build -o bin/cloudbox ./cmd/cloudbox

# The effective spec: source of truth + active change deltas (the default run).
acceptance:
	@mkdir -p acceptance-tests/reports
	cd acceptance-tests && .venv/bin/python run_acceptance.py

# Source-of-truth-only regression run.
acceptance-specs:
	@mkdir -p acceptance-tests/reports
	cd acceptance-tests && .venv/bin/python run_acceptance.py --specs

# Extract the Gherkin from openspec/**/spec.md and lint it.
lint-specs:
	cd acceptance-tests && .venv/bin/python run_acceptance.py --lint
