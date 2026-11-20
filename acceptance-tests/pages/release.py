"""The release boundary (release-integrity, ADR 0009): operations whose
external integration is absent, and the claims the release makes about itself.

In the sim suite the arrangement surfaces constitute an integration's
presence; withdrawing one is how a Given-step stands up "a released control
plane" without switching drivers.
"""

# Integration identifiers as the control plane names them.
SOURCE_CONTROL = "source control"
GITOPS = "GitOps"
PRODUCTION_INGEST = "production ingest"

_WIRE = {SOURCE_CONTROL: "sourceControl", GITOPS: "gitops", PRODUCTION_INGEST: "productionIngest"}


class ReleasePage:
    def __init__(self, api):
        self._api = api
        self.last_response = None

    def withdraw_integrations(self, *names):
        payload = {_WIRE[n]: False for n in names}
        self._api.post("/simctl/integrations", json=payload).raise_for_status()

    def open_promotion(self, sandbox, actor="dev@example.com"):
        self.last_response = self._api.post(
            "/v1/promotions", json={"sandbox": sandbox}, headers={"X-Cloudbox-User": actor}
        )
        return self.last_response

    def promotions(self, app):
        resp = self._api.get("/v1/promotions", params={"app": app})
        resp.raise_for_status()
        return resp.json().get("promotions") or []

    def post_evidence_check(self, sandbox, pr):
        self.last_response = self._api.post("/v1/evidence-checks", json={"sandbox": sandbox, "pr": pr})
        return self.last_response

    def checks(self, app, pr):
        resp = self._api.get("/v1/scm/prs/%s/%s/checks" % (app, pr))
        resp.raise_for_status()
        return resp.json().get("checks") or []

    def set_production_state(self, app, manifests):
        self.last_response = self._api.post("/simctl/production/%s" % app, json={"manifests": manifests})
        return self.last_response

    def production_status(self, app):
        resp = self._api.get("/v1/applications/%s/production-status" % app)
        resp.raise_for_status()
        return resp.json()

    def refusal(self):
        """(status, message) of the last request."""
        resp = self.last_response
        try:
            message = resp.json().get("error", "")
        except ValueError:
            message = resp.text
        return resp.status_code, message
