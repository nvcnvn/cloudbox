"""Page object for the continuous-integration configuration.

The conformance-ci capability's CI scenarios read the repository's actual
workflow — which stages it runs and whether a failing stage fails the check.
All knowledge of the workflow file's location and shape lives here.
"""

import os
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent.parent  # acceptance-tests/
WORKFLOW = HERE.parent / ".github" / "workflows" / "ci.yml"

REQUIRED_STAGES = {
    "builds both binaries": "make build",
    "vets the sources": "go vet ./...",
    "lints the extracted specifications": "make lint-specs",
    "runs the simulation suite": "make acceptance",
    "runs the conformance subset": "make conformance",
}


class CIConfigPage:
    def exists(self):
        return WORKFLOW.is_file()

    def text(self):
        return WORKFLOW.read_text()

    def missing_stages(self):
        text = self.text()
        return [name for name, command in REQUIRED_STAGES.items() if command not in text]

    def propagates_failures(self):
        """No stage may swallow its exit code: GitHub Actions fails a job on
        any nonzero step unless continue-on-error or shell-level swallowing
        intervenes. Comments don't count."""
        effective = "\n".join(
            line for line in self.text().splitlines()
            if not line.lstrip().startswith("#")
        )
        return "continue-on-error" not in effective and "|| true" not in effective

    def run_conformance_against_broken_cluster(self):
        """A real conformance invocation whose target cluster cannot exist —
        the cheapest honestly failing conformance run. Returns the completed
        process; its exit code is what a CI step consumes."""
        env = dict(os.environ)
        env["KUBECONFIG"] = str(HERE / ".kube" / "does-not-exist.kubeconfig")
        env["CLOUDBOX_KUBE_CONTEXT"] = "kind-no-such-cluster"
        env.pop("OPENSPEC_ACCEPTANCE", None)
        return subprocess.run(
            [sys.executable, "run_acceptance.py", "--conformance"],
            cwd=HERE, env=env, capture_output=True, text=True, timeout=300,
        )
