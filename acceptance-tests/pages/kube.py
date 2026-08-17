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

import base64
import json
import os
import subprocess
import tempfile
import uuid
from pathlib import Path

import requests

REPO = Path(__file__).resolve().parents[2]

# The proxy's attempt surface and the credential it requires (the control
# plane's own collection path presents the same header).
PROXY_SERVICE = "cloudbox-egress-proxy"
PROXY_ADMIN_PORT = 3129
PROXY_ADMIN_SECRET = "cloudbox-egress-proxy-token"
PROXY_ADMIN_TOKEN_KEY = "token"
PROXY_ADMIN_TOKEN_HEADER = "X-Cloudbox-Egress-Token"


class KubeClusterPage:
    def __init__(self, name=None):
        self.name = name or os.environ.get("CLOUDBOX_KUBE_CONTEXT", "kind-cloudbox-conformance")
        self._client_certs = None

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

    def create_unsealed_namespace(self):
        """A plain namespace with no seal on it — nothing the product manages."""
        name = "cloudbox-unsealed-%s" % uuid.uuid4().hex[:6]
        self._kubectl("create", "namespace", name).check_returncode()
        return name

    def delete_namespace(self, namespace):
        self._kubectl("delete", "namespace", namespace, "--wait=false")

    def evaluate_egress(self, namespace, destination):
        """Ask the driver how it evaluates one attempt from a namespace.

        The cluster contract's AttemptEgress has no product route under the kube
        driver (its only exposure is the sim-only /simctl surface, ADR 0008), so
        this drives the contract method through the conformance helper rather
        than the product growing an endpoint for the test's benefit. Returns the
        driver's EgressResult.
        """
        result = subprocess.run(
            [
                "go", "run", "./hack/conformance/egress-eval",
                "-context", self.name,
                "-namespace", namespace,
                "-destination", destination,
            ],
            cwd=str(REPO), capture_output=True, text=True, timeout=300,
        )
        if result.returncode != 0:
            raise AssertionError(
                "asking the driver to evaluate %r from %r failed:\n%s"
                % (destination, namespace, result.stderr)
            )
        return json.loads(result.stdout)

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

    # --- the proxy's attempt surface ---

    def _api_client_config(self):
        """The kubeconfig context's API server URL and client credentials, as
        files requests can use. kubectl cannot set request headers, and the
        attempt surface requires one, so this reads it directly."""
        if self._client_certs is None:
            result = self._kubectl("config", "view", "--raw", "--minify", "-o", "json")
            result.check_returncode()
            config = json.loads(result.stdout)
            cluster = config["clusters"][0]["cluster"]
            user = config["users"][0]["user"]
            directory = tempfile.mkdtemp(prefix="cloudbox-kubeapi-")

            def write(name, encoded):
                path = os.path.join(directory, name)
                with open(path, "wb") as handle:
                    handle.write(base64.b64decode(encoded))
                return path

            self._client_certs = (
                cluster["server"].rstrip("/"),
                write("ca.crt", cluster["certificate-authority-data"]),
                write("client.crt", user["client-certificate-data"]),
                write("client.key", user["client-key-data"]),
            )
        return self._client_certs

    def _api_get(self, path, headers=None):
        server, ca, cert, key = self._api_client_config()
        return requests.get(
            server + path, headers=headers or {}, verify=ca, cert=(cert, key), timeout=30,
        )

    def proxy_admin_token(self, namespace):
        """The namespace's egress-proxy credential, as the control plane reads
        it at collection time."""
        result = self._kubectl(
            "get", "secret", PROXY_ADMIN_SECRET, "-n", namespace,
            "-o", "jsonpath={.data.%s}" % PROXY_ADMIN_TOKEN_KEY,
        )
        result.check_returncode()
        return base64.b64decode(result.stdout).decode()

    def _attempts_path(self, namespace):
        return (
            "/api/v1/namespaces/%s/services/http:%s:admin/proxy/attempts"
            % (namespace, PROXY_SERVICE)
        )

    def proxy_attempt_record(self, namespace):
        """The egress proxy's whole attempt record, read through the API
        server's service proxy with the namespace's credential — the same path
        the control plane collects it by: the attempts it still retains, the
        count its retention bound discarded, and the proxy's incarnation."""
        response = self._api_get(
            self._attempts_path(namespace),
            headers={PROXY_ADMIN_TOKEN_HEADER: self.proxy_admin_token(namespace)},
        )
        response.raise_for_status()
        return response.json()

    def proxy_attempt_response_without_token(self, namespace):
        """The same read with no credential — for asserting it is refused."""
        return self._api_get(self._attempts_path(namespace))

    def read_attempt_surface_from_unsealed_namespace(self, namespace):
        """Run a workload in a fresh, unsealed namespace that goes for the
        sandbox's attempt surface directly over the pod network, and return its
        own output: whether it could reach the port at all, and whether the
        surface handed it the records."""
        scratch = "cloudbox-outsider-%s" % uuid.uuid4().hex[:6]
        self._kubectl("create", "namespace", scratch).check_returncode()
        try:
            target = "%s.%s.svc.cluster.local" % (PROXY_SERVICE, namespace)
            script = (
                "if nc -w 3 %(host)s %(port)d </dev/null; "
                "then echo CONNECT:OK; else echo CONNECT:FAILED; fi; "
                "if wget -q -O- -T 5 http://%(host)s:%(port)d/attempts; "
                "then echo READ:ALLOWED; else echo READ:REFUSED; fi"
                % {"host": target, "port": PROXY_ADMIN_PORT}
            )
            run = self._kubectl(
                "run", "outsider", "-n", scratch, "--image", "busybox:1.36",
                "--restart=Never", "--attach", "--rm", "--quiet",
                "--command", "--", "sh", "-c", script,
                timeout=300,
            )
            return run.stdout
        finally:
            self._kubectl("delete", "namespace", scratch, "--wait=false")

    def proxy_attempts(self, namespace):
        """Just the retained attempts."""
        return self.proxy_attempt_record(namespace)["attempts"]

    def proxy_dropped(self, namespace):
        """How many attempts the proxy's retention bound discarded."""
        return self.proxy_attempt_record(namespace)["dropped"]

    def restart_egress_proxy(self, namespace):
        """Delete the namespace's egress proxy pod and wait for its
        replacement to be ready — a real restart, losing whatever the process
        held in memory."""
        result = self._kubectl(
            "delete", "pods", "-n", namespace,
            "-l", "cloudbox.dev/component=egress-proxy", "--wait=true",
            timeout=180,
        )
        result.check_returncode()
        result = self._kubectl(
            "rollout", "status", "deployment/cloudbox-egress-proxy",
            "-n", namespace, "--timeout=180s", timeout=200,
        )
        result.check_returncode()

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
