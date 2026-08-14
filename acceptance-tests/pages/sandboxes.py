"""Sandboxes page object: creation and inspection."""

DEFAULT_DEV = "dev@example.com"


class SandboxesPage:
    def __init__(self, api):
        self._api = api
        self.last_response = None

    def create(self, app, owner=DEFAULT_DEV):
        self.last_response = self._api.post(
            "/v1/sandboxes", json={"app": app}, headers={"X-Cloudbox-User": owner}
        )
        self.last_response.raise_for_status()
        return self.last_response.json()["name"]

    def evidence(self, sandbox):
        resp = self._api.get("/v1/sandboxes/%s/evidence" % sandbox)
        resp.raise_for_status()
        return resp.json()
