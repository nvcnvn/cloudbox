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
    def __init__(self, name=None):
        self.name = name or os.environ.get("CLOUDBOX_KUBE_CONTEXT", "kind-cloudbox-conformance")

    @classmethod
    def nonenforcing(cls):
        """The deliberately non-enforcing cluster (flannel: accepts
        NetworkPolicy objects, never enforces them)."""
        return cls(os.environ.get("CLOUDBOX_KUBE_NONENFORCING_CONTEXT",
                                  "kind-cloudbox-nonenforcing"))

    def accepts_network_policy(self):
        """The cluster admits a NetworkPolicy object (whether it enforces it
        is exactly what the probe scenarios test)."""
        manifest = (
            "apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\n"
            "metadata: {name: cloudbox-npcheck, namespace: default}\n"
            "spec: {podSelector: {}, policyTypes: [Ingress, Egress]}\n"
        )
        result = subprocess.run(
            ["kubectl", "--context", self.name, "apply", "-f", "-"],
            input=manifest, capture_output=True, text=True, timeout=60,
        )
        self._kubectl("delete", "networkpolicy", "cloudbox-npcheck", "-n", "default")
        return result.returncode == 0

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

    def workload_logs(self, namespace, workload):
        """The pods' logs for one admitted workload (app=<name> label).
        Returns what is readable so far — empty while the pod is still
        scheduling or its container has not started (callers poll)."""
        result = self._kubectl(
            "logs", "-n", namespace, "-l", "app=%s" % workload, "--tail=-1"
        )
        if result.returncode != 0:
            return ""
        return result.stdout

    def proxy_attempt_record(self, namespace):
        """The egress proxy's whole attempt record, read through the API
        server's service proxy — the same path the control plane collects it
        by: the attempts it still retains plus the count its retention bound
        discarded."""
        result = self._kubectl(
            "get", "--raw",
            "/api/v1/namespaces/%s/services/http:cloudbox-egress-proxy:admin/proxy/attempts"
            % namespace,
        )
        result.check_returncode()
        return json.loads(result.stdout)

    def proxy_attempts(self, namespace):
        """Just the retained attempts."""
        return self.proxy_attempt_record(namespace)["attempts"]

    def proxy_dropped(self, namespace):
        """How many attempts the proxy's retention bound discarded."""
        return self.proxy_attempt_record(namespace)["dropped"]

    def server_minor(self):
        result = self._kubectl("version", "-o", "json")
        result.check_returncode()
        server = json.loads(result.stdout)["serverVersion"]
        return "%s.%s" % (server["major"], server["minor"].rstrip("+"))

    # --- real substrate arrangement (installed with kubectl, the way a
    # platform team installs an operator; not a simulated surface) ---

    OPERATOR_MANIFESTS = """\
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.widgets.example.com
spec:
  group: widgets.example.com
  scope: Namespaced
  names: {plural: widgets, singular: widget, kind: Widget, listKind: WidgetList}
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema: {type: object, x-kubernetes-preserve-unknown-fields: true}
---
apiVersion: v1
kind: Namespace
metadata:
  name: operators
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: widget-operator
  namespace: operators
  labels:
    app.kubernetes.io/component: operator
    app.kubernetes.io/name: widget-operator
    app.kubernetes.io/version: "%(version)s"
  annotations:
    cloudbox.dev/owns-crds: widgets.example.com
spec:
  replicas: 1
  selector:
    matchLabels: {app: widget-operator}
  template:
    metadata:
      labels: {app: widget-operator}
    spec:
      containers:
        - name: operator
          image: busybox:1.36
          command: ["sh", "-c", "sleep 1000000000"]
"""

    def install_widget_operator(self, version="v1.0.0"):
        """Install a real operator: its CRD group plus a labeled Deployment.
        Idempotent; re-applying resets the version label."""
        result = subprocess.run(
            ["kubectl", "--context", self.name, "apply", "-f", "-"],
            input=self.OPERATOR_MANIFESTS % {"version": version},
            capture_output=True, text=True, timeout=120,
        )
        result.check_returncode()

    def widget_operator_version(self):
        result = self._kubectl(
            "get", "deployment", "widget-operator", "-n", "operators",
            "-o", "jsonpath={.metadata.labels.app\\.kubernetes\\.io/version}",
        )
        result.check_returncode()
        return result.stdout.strip()

    def set_widget_operator_version(self, version):
        result = self._kubectl(
            "label", "deployment", "widget-operator", "-n", "operators",
            "app.kubernetes.io/version=%s" % version, "--overwrite",
        )
        result.check_returncode()
