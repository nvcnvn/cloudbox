"""Sandboxes page object: creation and inspection."""

DEFAULT_DEV = "dev@example.com"


class SandboxesPage:
    def __init__(self, api, platform):
        self._api = api
        self._platform = platform
        self.last_response = None

    def create(self, app, owner=DEFAULT_DEV, local=False, ttl_seconds=0, expect_ok=True, arrange=True):
        if arrange:
            self._platform.ensure_ready_platform()
        payload = {"app": app, "local": local}
        if ttl_seconds:
            payload["ttlSeconds"] = ttl_seconds
        # Creation seals and probe-verifies the namespace; on a real cluster
        # the probe runs live canary pods, so allow well beyond the default.
        self.last_response = self._api.post(
            "/v1/sandboxes", json=payload, headers={"X-Cloudbox-User": owner},
            timeout=300,
        )
        if expect_ok:
            self.last_response.raise_for_status()
            return self.last_response.json()["name"]
        return None

    def record(self, sandbox):
        return self._api.get("/v1/sandboxes/%s" % sandbox)

    def workloads(self, sandbox):
        resp = self._api.get("/v1/sandboxes/%s/workloads" % sandbox)
        resp.raise_for_status()
        return resp.json().get("workloads") or []

    def destroy(self, sandbox, owner=DEFAULT_DEV):
        return self._api.delete(
            "/v1/sandboxes/%s" % sandbox, headers={"X-Cloudbox-User": owner}
        )

    def evidence(self, sandbox):
        resp = self._api.get("/v1/sandboxes/%s/evidence" % sandbox)
        resp.raise_for_status()
        return resp.json()
