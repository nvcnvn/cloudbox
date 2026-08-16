"""Page object for the real conformance cluster (@conformance scenarios).

Encapsulates every identifier of the cluster-side view: the kubeconfig
(KUBECONFIG, exported by `make conformance`), the kubeconfig context name
(CLOUDBOX_KUBE_CONTEXT), and the kubectl invocations that read real cluster
state. Step definitions assert on intent-level methods only.

This is the *observer's* path to the cluster — verifying that what the
product claims matches what the cluster holds. Arrangement still goes through
the product's own surface (ADR 0008: no test-arrangement endpoint under
--driver kube).
"""

import json
import os
import subprocess


class KubeClusterPage:
    def __init__(self):
        self.name = os.environ.get("CLOUDBOX_KUBE_CONTEXT", "kind-cloudbox-conformance")

    def _kubectl(self, *args, timeout=60):
        return subprocess.run(
            ["kubectl", "--context", self.name, *args],
            capture_output=True, text=True, timeout=timeout,
        )

    def reachable(self):
        return self._kubectl("get", "--raw", "/readyz", timeout=15).returncode == 0

    def crd_names(self):
        result = self._kubectl("get", "crds", "-o", "json")
        result.check_returncode()
        return {item["metadata"]["name"] for item in json.loads(result.stdout)["items"]}
