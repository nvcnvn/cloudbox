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

    def network_policies(self, namespace):
        """Standard NetworkPolicy v1 objects in the namespace, as the cluster
        stores them."""
        result = self._kubectl("get", "networkpolicies", "-n", namespace, "-o", "json")
        result.check_returncode()
        return json.loads(result.stdout)["items"]

    def vendor_policy_objects(self, namespace):
        """Instances of vendor policy CRDs (Calico/Cilium) in the namespace —
        the seal must use none (ADR 0001)."""
        found = []
        crds = self._kubectl("get", "crds", "-o", "json")
        crds.check_returncode()
        for item in json.loads(crds.stdout)["items"]:
            group = item["spec"]["group"]
            if not any(vendor in group for vendor in ("projectcalico.org", "cilium.io")):
                continue
            if "policy" not in item["metadata"]["name"]:
                continue
            if item["spec"]["scope"] != "Namespaced":
                continue
            result = self._kubectl(
                "get", item["metadata"]["name"], "-n", namespace, "-o", "json"
            )
            if result.returncode == 0:
                for obj in json.loads(result.stdout)["items"]:
                    found.append("%s/%s" % (item["metadata"]["name"], obj["metadata"]["name"]))
        return found

    def namespace_exists(self, namespace):
        return self._kubectl("get", "namespace", namespace).returncode == 0
