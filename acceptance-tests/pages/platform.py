"""Platform page object: clusters, setup, CRDs, and the user's own kubectl
path (simulated). All URLs and payload shapes for this flow live here.
"""

USER_DEPLOYMENT = {
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {"name": "user-web", "namespace": "user-ns"},
    "spec": {
        "replicas": 2,
        "template": {"spec": {"containers": [{"name": "web", "image": "web:1.0"}]}},
    },
}


class PlatformPage:
    def __init__(self, api):
        self._api = api

    def ensure_cluster(self, name, enforcing=True):
        self._api.post("/simctl/clusters", json={"name": name, "enforcing": enforcing}).raise_for_status()
        return name

    def run_setup(self, cluster):
        return self._api.post("/v1/setup", json={"cluster": cluster})

    def register_cluster(self, cluster, role):
        resp = self._api.post("/v1/clusters/register", json={"cluster": cluster, "role": role})
        resp.raise_for_status()
        return resp

    def ensure_ready_platform(self, name="main", enforcing=True):
        """One installed cluster registered as both sandbox host and
        production — the shared-cluster topology (CP3)."""
        self.ensure_cluster(name, enforcing)
        self.run_setup(name).raise_for_status()
        self.register_cluster(name, "sandbox")
        self.register_cluster(name, "production")
        return name

    def ensure_split_platform(self, components=None):
        """Separate sandbox and production clusters (CP3), optionally with the
        same substrate components installed on both."""
        self.ensure_cluster("sbx-cluster")
        self.run_setup("sbx-cluster").raise_for_status()
        self.register_cluster("sbx-cluster", "sandbox")
        self.ensure_cluster("prod-cluster")
        self.register_cluster("prod-cluster", "production")
        if components is not None:
            self.set_components("sbx-cluster", components)
            self.set_components("prod-cluster", components)
        return "sbx-cluster", "prod-cluster"

    def set_components(self, cluster, components, kubernetes_minor="1.31"):
        self._api.post(
            "/simctl/clusters/%s/components" % cluster,
            json={"kubernetesMinor": kubernetes_minor, "components": components},
        ).raise_for_status()

    def lockfile(self, app):
        resp = self._api.get("/v1/applications/%s/substrate-lockfile" % app)
        resp.raise_for_status()
        return resp.json()

    def audit_entries(self):
        resp = self._api.get("/v1/audit")
        resp.raise_for_status()
        return resp.json().get("entries") or []

    def installed_crds(self, cluster):
        resp = self._api.get("/v1/clusters/%s/crds" % cluster)
        resp.raise_for_status()
        return resp.json()["crds"]

    def place_user_workload(self, cluster):
        """Apply a plain Deployment the way the user's own kubectl would —
        outside any CloudBox API."""
        self._api.post(
            "/simctl/clusters/%s/objects" % cluster, json={"manifest": USER_DEPLOYMENT}
        ).raise_for_status()

    def user_workload_as_stored(self, cluster):
        resp = self._api.get(
            "/simctl/clusters/%s/objects" % cluster,
            params={
                "namespace": USER_DEPLOYMENT["metadata"]["namespace"],
                "kind": USER_DEPLOYMENT["kind"],
                "name": USER_DEPLOYMENT["metadata"]["name"],
            },
        )
        resp.raise_for_status()
        return resp.json()["manifest"]

    def user_workload_unmodified(self, cluster):
        return self.user_workload_as_stored(cluster) == USER_DEPLOYMENT
