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
