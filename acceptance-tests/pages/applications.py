"""Applications page object: declaring and admitting Application resources,
including their boundary contract.
"""


class ApplicationsPage:
    def __init__(self, api):
        self._api = api
        self.last_response = None

    def draft(self, name, **fields):
        """Prepare an Application spec without admitting it."""
        spec = {"name": name, "owners": ["owner@example.com"], "approvers": []}
        spec.update(fields)
        return spec

    def with_dependency(self, spec, target_app, alias=None):
        contract = spec.setdefault(
            "contract",
            {"secretNames": [], "ingressHostnames": [], "egressAllowlist": [], "dependencies": []},
        )
        dep = {"app": target_app}
        if alias:
            dep["alias"] = alias
        contract["dependencies"].append(dep)
        return spec

    def admit(self, spec):
        self.last_response = self._api.post("/v1/applications", json=spec)
        return self.last_response

    def create(self, name, **fields):
        """Draft + admit, asserting success — for Given-steps that just need
        an application to exist."""
        resp = self.admit(self.draft(name, **fields))
        resp.raise_for_status()
        return resp.json()

    def rejected(self):
        return self.last_response is not None and self.last_response.status_code >= 400

    def rejection_message(self):
        return (self.last_response.json() or {}).get("error", "")
