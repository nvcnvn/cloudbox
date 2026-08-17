"""behave hooks for the acceptance suite: boot the app, attach page objects.

Two things here are load-bearing beyond ordinary setup.

1. THE WRAPPER TRIPWIRE. behave parses the feature files before before_all
   fires, so extraction cannot live here (see run_acceptance.py). That leaves a
   hole the JS stack does not have: running plain `behave` would happily execute
   a stale .extracted/ tree. before_all refuses to start unless the wrapper set
   OPENSPEC_ACCEPTANCE, turning a silent wrong-result run into a loud failure.

2. PAGE OBJECTS ONLY. Step definitions must contain no selectors, URLs or
   regexes. `context` is the World: a thin HTTP client plus page-object
   instances. See the skill's Page Object Model section.

This file must sit at the acceptance-tests/ root alongside steps/ -- behave
derives its base directory by walking UP from the first location argument until
it finds a steps/ directory, and loads environment.py from there.

The app under test is cloudboxd: built once in before_all, booted once on a
free port. The cluster driver comes from CLOUDBOX_DRIVER (default "sim", set
to "kube" by run_acceptance.py --conformance per ADR 0008). Under sim, state
resets between scenarios via /simctl/reset; under kube that surface does not
exist -- conformance scenarios own their arrangement through the product API.
"""

import os
import socket
import subprocess
import time
from pathlib import Path

import requests

import pages

HERE = Path(__file__).resolve().parent
REPO = HERE.parent

# behave.ini points the html formatter at reports/behave-report.html; the
# directory is gitignored, so guarantee it exists before formatters write.
(HERE / "reports").mkdir(exist_ok=True)


def _free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def before_all(context):
    if not os.environ.get("OPENSPEC_ACCEPTANCE"):
        raise RuntimeError(
            "Run the suite via `python run_acceptance.py` (not plain `behave`).\n"
            "The wrapper extracts the Gherkin from openspec/**/spec.md and composes "
            "the effective spec; running behave directly would execute a stale or "
            "missing .extracted/ tree."
        )

    subprocess.run(
        ["go", "build", "-o", str(REPO / "bin" / "cloudboxd"), "./cmd/cloudboxd"],
        cwd=REPO, check=True,
    )
    subprocess.run(
        ["go", "build", "-o", str(REPO / "bin" / "cloudbox"), "./cmd/cloudbox"],
        cwd=REPO, check=True,
    )

    context.driver = os.environ.get("CLOUDBOX_DRIVER", "sim")
    port = _free_port()
    context.base_url = "http://127.0.0.1:%d" % port
    context.cli_path = str(REPO / "bin" / "cloudbox")
    context.app = subprocess.Popen(
        [
            str(REPO / "bin" / "cloudboxd"),
            "--addr", "127.0.0.1:%d" % port,
            "--driver", context.driver,
        ]
    )

    # A real cluster takes longer to reach healthy (CRD installs, API round
    # trips) than the in-process sim.
    deadline = time.time() + (120 if context.driver == "kube" else 15)
    while True:
        try:
            if requests.get(context.base_url + "/healthz", timeout=1).ok:
                break
        except requests.ConnectionError:
            pass
        if context.app.poll() is not None:
            raise RuntimeError(
                "cloudboxd exited with code %d before becoming healthy "
                "(driver=%s)" % (context.app.returncode, context.driver)
            )
        if time.time() > deadline:
            context.app.terminate()
            raise RuntimeError("cloudboxd did not become healthy before the deadline")
        time.sleep(0.1)

    if context.driver == "kube":
        # The enforcement gate (conformance-ci): before reporting any
        # conformance result, prove the target cluster actually enforces
        # NetworkPolicy. An unproven cluster aborts the run — a vacuous pass
        # must be impossible.
        from pages.client import ApiClient
        from pages.gate import check_enforcement
        from pages.kube import KubeClusterPage

        target = KubeClusterPage().name
        passed, message = check_enforcement(ApiClient(context.base_url), target)
        if not passed:
            context.app.terminate()
            raise RuntimeError(message)


def after_all(context):
    app = getattr(context, "app", None)
    if app:
        app.terminate()
        app.wait(timeout=10)


def after_scenario(context, scenario):
    # Real-cluster runs have no /simctl/reset; tear down the scenario's
    # sandbox (and with it its namespace) so the cluster stays bounded.
    if getattr(context, "driver", "sim") != "sim":
        sandbox = getattr(context, "sandbox_name", None)
        if sandbox:
            try:
                context.sandboxes.destroy(sandbox)
            except Exception:
                pass
        # Namespaces a scenario made outside the product (an unsealed namespace
        # to evaluate against) are not sandboxes, so nothing else reclaims them.
        namespace = getattr(context, "unsealed_namespace", None)
        if namespace:
            try:
                context.kube.delete_namespace(namespace)
            except Exception:
                pass


def before_scenario(context, scenario):
    # /simctl/* exists only when the sim driver is constructed (ADR 0008);
    # under kube there is no arrangement surface to reset.
    if context.driver == "sim":
        requests.post(context.base_url + "/simctl/reset", timeout=5).raise_for_status()
    pages.attach(context)
