"""SCM page object: delivers webhook events the way the SCM integration would
(PR opened / push / closed), and reads back the PR-bound sandbox.
"""


class ScmPage:
    def __init__(self, api):
        self._api = api
        self.last_response = None

    def _event(self, type_, app, pr, author="dev@example.com", manifests=""):
        self.last_response = self._api.post(
            "/v1/scm/events",
            json={"type": type_, "app": app, "pr": pr, "author": author, "manifests": manifests},
        )
        return self.last_response

    def open_pr(self, app, pr):
        resp = self._event("opened", app, pr)
        resp.raise_for_status()
        return resp.json()

    def push(self, app, pr, manifests):
        resp = self._event("push", app, pr, manifests=manifests)
        resp.raise_for_status()
        return resp.json()

    def close_pr(self, app, pr, merged=False):
        resp = self._event("merged" if merged else "closed", app, pr)
        resp.raise_for_status()
        return resp.json()
