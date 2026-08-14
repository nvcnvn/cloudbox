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
        return (self.last_response.json() or {}).get("findings", [])

    def transforms(self):
        return (self.last_response.json() or {}).get("transforms", [])

    def digest_of_submitted_manifests(self):
        """What the digest must be if it covers the bytes as submitted —
        i.e. unchanged by any recorded transform."""
        return sha256_digest(self.last_manifests)
