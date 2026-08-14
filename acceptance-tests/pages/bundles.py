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

    # --- actions ---

    def apply(self, app, sandbox, manifests, actor="dev@example.com"):
        self.last_manifests = manifests
        self.last_response = self._api.post(
            "/v1/apply",
            json={"app": app, "sandbox": sandbox, "manifests": manifests},
            headers={"X-Cloudbox-User": actor},
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
