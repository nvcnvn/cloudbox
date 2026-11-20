"""The release boundary (release-integrity, ADR 0009): operations whose
external integration is absent, and the claims the release makes about itself.

In the sim suite the arrangement surfaces constitute an integration's
presence; withdrawing one is how a Given-step stands up "a released control
plane" without switching drivers.
"""

# Integration identifiers as the control plane names them.
SOURCE_CONTROL = "source control"
GITOPS = "GitOps"
PRODUCTION_INGEST = "production ingest"

_WIRE = {SOURCE_CONTROL: "sourceControl", GITOPS: "gitops", PRODUCTION_INGEST: "productionIngest"}


class ReleasePage:
    def __init__(self, api):
        self._api = api
        self.last_response = None

    def withdraw_integrations(self, *names):
        payload = {_WIRE[n]: False for n in names}
        self._api.post("/simctl/integrations", json=payload).raise_for_status()

    def open_promotion(self, sandbox, actor="dev@example.com"):
        self.last_response = self._api.post(
            "/v1/promotions", json={"sandbox": sandbox}, headers={"X-Cloudbox-User": actor}
        )
        return self.last_response

    def promotions(self, app):
        resp = self._api.get("/v1/promotions", params={"app": app})
        resp.raise_for_status()
        return resp.json().get("promotions") or []

    def post_evidence_check(self, sandbox, pr):
        self.last_response = self._api.post("/v1/evidence-checks", json={"sandbox": sandbox, "pr": pr})
        return self.last_response

    def checks(self, app, pr):
        resp = self._api.get("/v1/scm/prs/%s/%s/checks" % (app, pr))
        resp.raise_for_status()
        return resp.json().get("checks") or []

    def set_production_state(self, app, manifests):
        self.last_response = self._api.post("/simctl/production/%s" % app, json={"manifests": manifests})
        return self.last_response

    def production_status(self, app):
        resp = self._api.get("/v1/applications/%s/production-status" % app)
        resp.raise_for_status()
        return resp.json()

    def refusal(self):
        """(status, message) of the last request."""
        resp = self.last_response
        try:
            message = resp.json().get("error", "")
        except ValueError:
            message = resp.text
        return resp.status_code, message


# --- release builds (release-integrity: version identity, pinned proxy) ------

import json as _json
import os as _os
import shutil as _shutil
import socket as _socket
import subprocess as _subprocess
import tempfile as _tempfile
import time as _time
from pathlib import Path as _Path

import requests as _requests

_REPO = _Path(__file__).resolve().parents[2]

# What a release build is stamped with in the suite. The digest is a fixture:
# the scenario asserts the binary deploys exactly what its release stamped.
TEST_RELEASE_VERSION = "v0.0.0-acceptance"
TEST_PROXY_IMAGE = "ghcr.io/nvcnvn/cloudbox-egress-proxy@sha256:" + "a" * 64

_built = {}


def release_build():
    """Binaries produced by the project's release process (make
    release-binaries), built once per run."""
    if "dir" not in _built:
        out = _tempfile.mkdtemp(prefix="cloudbox-release-")
        _subprocess.run(
            ["make", "release-binaries", "VERSION=" + TEST_RELEASE_VERSION,
             "PROXY_IMAGE=" + TEST_PROXY_IMAGE, "OUT=" + out],
            cwd=_REPO, check=True, capture_output=True,
        )
        _built["dir"] = out
    return _Path(_built["dir"])


class ReleasedControlPlane:
    """A control plane from a release build, on the sim driver."""

    def __init__(self):
        with _socket.socket() as s:
            s.bind(("127.0.0.1", 0))
            port = s.getsockname()[1]
        self.url = "http://127.0.0.1:%d" % port
        env = dict(_os.environ)
        env.pop("CLOUDBOX_PROXY_IMAGE", None)  # the stamped reference, unoverridden
        self._proc = _subprocess.Popen(
            [str(release_build() / "cloudboxd"), "--addr", "127.0.0.1:%d" % port, "--driver", "sim"],
            env=env, stdout=_subprocess.DEVNULL, stderr=_subprocess.DEVNULL,
        )
        deadline = _time.time() + 15
        while _time.time() < deadline:
            try:
                if _requests.get(self.url + "/healthz", timeout=1).ok:
                    return
            except _requests.ConnectionError:
                pass
            _time.sleep(0.1)
        raise RuntimeError("released cloudboxd did not become healthy")

    def post(self, path, body, **kw):
        resp = _requests.post(self.url + path, json=body, timeout=30, **kw)
        resp.raise_for_status()
        return resp.json() if resp.content else {}

    def get(self, path):
        resp = _requests.get(self.url + path, timeout=30)
        resp.raise_for_status()
        return resp.json()

    def stop(self):
        self._proc.terminate()
        self._proc.wait(timeout=10)


def version_report(binary):
    """(cli json report) for the CLI, or the daemon's --version text."""
    if binary.name == "cloudbox":
        out = _subprocess.run([str(binary), "-o", "json", "version"], capture_output=True, text=True, check=True)
        return _json.loads(out.stdout)
    out = _subprocess.run([str(binary), "--version"], capture_output=True, text=True, check=True)
    first = out.stdout.splitlines()[0]
    return {"version": first.split(" ", 1)[1], "released": "unreleased" not in first}
