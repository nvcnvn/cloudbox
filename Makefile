.PHONY: build acceptance acceptance-specs lint-specs conformance

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

# The @conformance subset against real Kind clusters (ADR 0008): the
# enforcing target plus the deliberately non-enforcing cluster the
# probe-failure and gate scenarios assert against. Provisioning is
# idempotent; kubeconfigs land under acceptance-tests/.kube/ (gitignored) and
# are merged so the driver sees both contexts.
conformance:
	@mkdir -p acceptance-tests/reports
	hack/conformance/kind-enforcing.sh
	hack/conformance/kind-nonenforcing.sh
	KUBECONFIG=$(CURDIR)/acceptance-tests/.kube/conformance.kubeconfig:$(CURDIR)/acceptance-tests/.kube/nonenforcing.kubeconfig \
		kubectl config view --flatten > acceptance-tests/.kube/merged.kubeconfig
	cd acceptance-tests && KUBECONFIG=$(CURDIR)/acceptance-tests/.kube/merged.kubeconfig \
		CLOUDBOX_KUBE_CONTEXT=kind-cloudbox-conformance \
		CLOUDBOX_KUBE_NONENFORCING_CONTEXT=kind-cloudbox-nonenforcing \
		.venv/bin/python run_acceptance.py --conformance
