"""Bundles page object: applying manifest sets and inspecting the results.
All manifest fixtures the intake scenarios need live here too, so steps carry
no YAML.
"""

import hashlib

PLAIN_MIXED_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: web
          image: web:1.0
---
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  ports:
    - port: 80
---
apiVersion: acme.example.com/v1
kind: WidgetCache
metadata:
  name: cache
spec:
  size: small
"""


def sha256_digest(yaml_text):
    return "sha256:" + hashlib.sha256(yaml_text.encode()).hexdigest()


# References the widget-operator's CRD group so the substrate lockfile scopes
# to that operator (P1); used by the real-cluster substrate scenario.
WIDGET_BUNDLE_YAML = """\
apiVersion: widgets.example.com/v1
kind: Widget
metadata:
  name: sample-widget
spec:
  size: small
"""

# The sim-reconciliation scenario's pair: the same two workloads with and
# without the Service that real cluster DNS needs (sim DIVERGENCES.md #1).
SHORTNAME_NO_SERVICE_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: web
          image: web:1.0
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-api
spec:
  template:
    spec:
      containers:
        - name: auth-api
          image: auth:1.0
"""

SHORTNAME_WITH_SERVICE_YAML = SHORTNAME_NO_SERVICE_YAML + """\
---
apiVersion: v1
kind: Service
metadata:
  name: auth-api
spec:
  ports:
    - port: 80
"""

# Real-cluster workload bundles (@conformance): runnable images, since these
# pods actually start. The pod label app=<name> is how the driver attributes
# pod-level evidence (OOM kills) to the admitted workload.
READY_WORKLOAD_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels: {app: web}
spec:
  replicas: 1
  selector:
    matchLabels: {app: web}
  template:
    metadata:
      labels: {app: web}
    spec:
      containers:
        - name: web
          image: busybox:1.36
          command: ["sh", "-c", "sleep 1000000000"]
          resources:
            requests: {cpu: 50m, memory: 32Mi}
            limits: {cpu: 100m, memory: 128Mi}
"""

# Attempts egress to an undeclared destination, both directly (the CNI must
# deny it) and through the injected egress proxy (which must refuse and
# record it). The direct leg targets an IP literal: an undeclared FQDN has
# nothing to resolve to, and IP-literal egress is exactly what the seal's
# containment statement promises to block.
EGRESS_ATTEMPT_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prober
  labels: {app: prober}
spec:
  replicas: 1
  selector:
    matchLabels: {app: prober}
  template:
    metadata:
      labels: {app: prober}
    spec:
      containers:
        - name: prober
          image: busybox:1.36
          command:
            - sh
            - -c
            - >
              i=0; while [ $i -lt 30 ]; do
              if nc -w 2 cloudbox-egress-proxy 3128 </dev/null; then break; fi;
              i=$((i+1)); sleep 2; done;
              if nc -w 3 1.1.1.1 443 </dev/null;
              then echo DIRECT:CONNECTED; else echo DIRECT:BLOCKED; fi;
              if wget -q -O /dev/null -T 10 http://api.other-vendor.com/;
              then echo PROXY:ALLOWED; else echo PROXY:DENIED; fi;
              sleep 1000000000
"""

# Attempts a blocked destination continuously, so at any instant the proxy
# holds attempts the control plane has not collected yet. Restarting the proxy
# under this workload loses real records, rather than only being conservatively
# marked as possibly lossy.
EGRESS_LOOP_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: looper
  labels: {app: looper}
spec:
  replicas: 1
  selector:
    matchLabels: {app: looper}
  template:
    metadata:
      labels: {app: looper}
    spec:
      containers:
        - name: looper
          image: busybox:1.36
          command:
            - sh
            - -c
            - >
              i=0; while [ $i -lt 30 ]; do
              if nc -w 2 cloudbox-egress-proxy 3128 </dev/null; then break; fi;
              i=$((i+1)); sleep 2; done;
              while true; do
              wget -q -O /dev/null -T 5 http://api.other-vendor.com/ 2>/dev/null;
              echo LOOP:ATTEMPTED; sleep 1; done
"""

# Makes more blocked attempts than the proxy retains, so its retention bound
# has to discard some. The iteration count must exceed the proxy's bound
# (cmd/cloudbox-proxy's -max-attempts default); it is deliberately just above
# it, since every iteration is a real request through the proxy.
EGRESS_FLOOD_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: flooder
  labels: {app: flooder}
spec:
  replicas: 1
  selector:
    matchLabels: {app: flooder}
  template:
    metadata:
      labels: {app: flooder}
    spec:
      containers:
        - name: flooder
          image: busybox:1.36
          command:
            - sh
            - -c
            - >
              i=0; while [ $i -lt 30 ]; do
              if nc -w 2 cloudbox-egress-proxy 3128 </dev/null; then break; fi;
              i=$((i+1)); sleep 2; done;
              i=0; while [ $i -lt 600 ]; do
              wget -q -O /dev/null -T 5 http://api.other-vendor.com/ 2>/dev/null;
              i=$((i+1)); done;
              echo FLOOD:DONE;
              sleep 1000000000
"""

# Fetches one declared external endpoint through the injected egress proxy.
ALLOWED_EGRESS_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fetcher
  labels: {app: fetcher}
spec:
  replicas: 1
  selector:
    matchLabels: {app: fetcher}
  template:
    metadata:
      labels: {app: fetcher}
    spec:
      containers:
        - name: fetcher
          image: busybox:1.36
          command:
            - sh
            - -c
            - >
              i=0; while [ $i -lt 30 ]; do
              if nc -w 2 cloudbox-egress-proxy 3128 </dev/null; then break; fi;
              i=$((i+1)); sleep 2; done;
              if wget -q -O /dev/null -T 20 http://example.com/;
              then echo FETCH:OK; else echo FETCH:FAILED; fi;
              sleep 1000000000
"""

# Two services in one sandbox: an httpd plus a caller that resolves it by
# short name through cluster DNS and connects directly (nc uses no proxy).
TWO_SERVICES_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels: {app: web}
spec:
  replicas: 1
  selector:
    matchLabels: {app: web}
  template:
    metadata:
      labels: {app: web}
    spec:
      containers:
        - name: web
          image: busybox:1.36
          command: ["sh", "-c", "mkdir -p /www && echo ok > /www/index.html && httpd -f -p 8080 -h /www"]
---
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector: {app: web}
  ports:
    - port: 8080
      targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: caller
  labels: {app: caller}
spec:
  replicas: 1
  selector:
    matchLabels: {app: caller}
  template:
    metadata:
      labels: {app: caller}
    spec:
      containers:
        - name: caller
          image: busybox:1.36
          env:
            - name: NAMESPACE
              valueFrom:
                fieldRef: {fieldPath: metadata.namespace}
          command:
            - sh
            - -c
            - >
              i=0; dns=DNS:FAIL; conn=CONN:FAIL;
              while [ $i -lt 45 ]; do
                if nslookup "web.$NAMESPACE.svc.cluster.local" >/dev/null 2>&1; then dns=DNS:OK; fi;
                if nc -w 2 web 8080 </dev/null; then conn=CONN:OK; fi;
                if [ "$dns" = DNS:OK ] && [ "$conn" = CONN:OK ]; then break; fi;
                i=$((i+1)); sleep 2;
              done;
              echo $dns; echo $conn; sleep 1000000000
"""

# Declares a 128Mi limit the squeezed transform halves to the 64Mi floor,
# then allocates ~100Mi: the kernel OOM-kills it under the squeeze.
MEMORY_HOG_YAML = """\
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hog
  labels: {app: hog}
spec:
  replicas: 1
  selector:
    matchLabels: {app: hog}
  template:
    metadata:
      labels: {app: hog}
    spec:
      containers:
        - name: hog
          image: busybox:1.36
          command: ["sh", "-c", "head -c 100m /dev/zero | tail; sleep 1000000000"]
          resources:
            requests: {cpu: 50m, memory: 32Mi}
            limits: {cpu: 100m, memory: 128Mi}
"""


class BundlesPage:
    def __init__(self, api):
        self._api = api
        self.last_response = None
        self.last_manifests = None

    # --- fixtures ---

    def plain_mixed_manifests(self):
        assert "cloudbox" not in PLAIN_MIXED_YAML.lower(), (
            "fixture must carry no CloudBox-specific fields"
        )
        return PLAIN_MIXED_YAML

    def namespaced_manifests(self, *namespaces):
        """One Deployment + one Service; namespaces assigned round-robin from
        the given list (one value → uniform namespace)."""
        docs = []
        kinds = [("apps/v1", "Deployment", "web"), ("v1", "Service", "web")]
        for i, (api, kind, name) in enumerate(kinds):
            ns = namespaces[i % len(namespaces)]
            docs.append(
                "apiVersion: %s\nkind: %s\nmetadata:\n  name: %s\n  namespace: %s\nspec: {}\n"
                % (api, kind, name, ns)
            )
        return "---\n".join(docs)

    def cluster_scoped_manifests(self):
        return (
            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec: {}\n"
            "---\n"
            "apiVersion: rbac.authorization.k8s.io/v1\nkind: ClusterRole\n"
            "metadata:\n  name: web-reader\nrules: []\n"
        )

    def manifests_referencing(self, url):
        return (
            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
            "spec:\n  template:\n    spec:\n      containers:\n"
            "        - name: web\n          image: web:1.0\n          env:\n"
            "            - name: BACKEND_URL\n              value: %s\n" % url
        )

    def rendered_helm_output(self):
        """Typical `helm template` output: release labels, comments naming the
        source templates — plain YAML, nothing product-specific."""
        return (
            "---\n"
            "# Source: shop/templates/deployment.yaml\n"
            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: shop\n"
            "  labels:\n    app.kubernetes.io/managed-by: Helm\n"
            "    app.kubernetes.io/instance: shop\nspec: {}\n"
            "---\n"
            "# Source: shop/templates/service.yaml\n"
            "apiVersion: v1\nkind: Service\nmetadata:\n  name: shop\n"
            "  labels:\n    app.kubernetes.io/managed-by: Helm\nspec: {}\n"
        )

    def timestamped_manifests(self):
        return (
            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
            "  annotations:\n    example.com/generated-at: \"2026-08-14T10:00:00Z\"\n"
            "spec: {}\n"
        )

    def random_value_manifests(self):
        return (
            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
            "  annotations:\n    example.com/render-id: \"3f9d2c81-5a44-4b7e-9c10-8e21d4a6f0b2\"\n"
            "spec: {}\n"
        )

    def prod_sized_manifests(self, name="web", replicas=3, cpu="1000m", memory="512Mi", env=None):
        env_block = ""
        if env:
            env_lines = "".join(
                "            - name: %s\n              value: \"%s\"\n" % (k, v)
                for k, v in env.items()
            )
            env_block = "          env:\n" + env_lines
        return (
            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: %s\n"
            "spec:\n  replicas: %d\n  template:\n    spec:\n      containers:\n"
            "        - name: %s\n          image: %s:1.0\n"
            "          resources:\n            requests:\n              cpu: \"%s\"\n              memory: \"%s\"\n"
            "%s" % (name, replicas, name, name, cpu, memory, env_block)
        )

    def hpa_manifests(self):
        return (
            self.prod_sized_manifests()
            + "---\n"
            + "apiVersion: autoscaling/v2\nkind: HorizontalPodAutoscaler\n"
            + "metadata:\n  name: web-hpa\nspec:\n  minReplicas: 3\n  maxReplicas: 10\n"
        )

    def apply_options(self, app, sandbox, manifests, capacity_mode=None, record_egress=False, actor="dev@example.com"):
        payload = {"app": app, "sandbox": sandbox, "manifests": manifests}
        if capacity_mode:
            payload["capacityMode"] = capacity_mode
        if record_egress:
            payload["recordEgress"] = True
        self.last_manifests = manifests
        # Under the kube driver an apply reaches a real API server per object.
        self.last_response = self._api.post(
            "/v1/apply", json=payload, headers={"X-Cloudbox-User": actor},
            timeout=120,
        )
        return self.last_response

    def secret_mounting_manifests(self, secret_name):
        return (
            "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
            "spec:\n  template:\n    spec:\n      containers:\n"
            "        - name: web\n          image: web:1.0\n          envFrom:\n"
            "            - secretRef:\n                name: %s\n" % secret_name
        )

    # --- actions ---

    def apply(self, app, sandbox, manifests, actor="dev@example.com"):
        self.last_manifests = manifests
        # Under the kube driver an apply reaches a real API server per object.
        self.last_response = self._api.post(
            "/v1/apply",
            json={"app": app, "sandbox": sandbox, "manifests": manifests},
            headers={"X-Cloudbox-User": actor},
            timeout=120,
        )
        return self.last_response

    def bundle_record(self, digest):
        return self._api.get("/v1/bundles/%s" % digest)

    # --- outcomes ---

    def accepted(self):
        return self.last_response is not None and self.last_response.ok

    def rejected(self):
        return self.last_response is not None and self.last_response.status_code >= 400

    def digest(self):
        return self.last_response.json()["digest"]

    def error_message(self):
        return (self.last_response.json() or {}).get("error", "")

    def findings(self):
        return (self.last_response.json() or {}).get("findings") or []

    def transforms(self):
        return (self.last_response.json() or {}).get("transforms") or []

    def digest_of_submitted_manifests(self):
        """What the digest must be if it covers the bytes as submitted —
        i.e. unchanged by any recorded transform."""
        return sha256_digest(self.last_manifests)
