"""The conformance run's enforcement precondition (conformance-ci).

A conformance result from a cluster that ignores NetworkPolicy would be
vacuous, so before reporting anything the run MUST prove the target cluster
enforces. The proof goes through the product's own seal path — a throwaway
sandbox pinned to the target cluster either comes back probe-verified sealed
or it does not (N7) — so the gate needs no test-only endpoint and works
identically against real and simulated clusters.

Used in two places: environment.py runs it in before_all for every
conformance run (a failed gate aborts the run, reporting no pass), and the
untagged conformance-ci scenarios exercise both of its directions in the
default sim suite.
"""

import uuid

OWNER = "conformance-gate@cloudbox.dev"


def check_enforcement(api, cluster_name):
    """Returns (passed, message). A False verdict's message names unproven
    network policy enforcement and states that no conformance pass is
    reported."""
    app = "conformance-gate-%s" % uuid.uuid4().hex[:6]
    api.post(
        "/v1/applications",
        json={"name": app, "owners": [OWNER], "sandboxCluster": cluster_name},
    ).raise_for_status()
    created = api.post(
        "/v1/sandboxes", json={"app": app},
        headers={"X-Cloudbox-User": OWNER}, timeout=300,
    )
    created.raise_for_status()
    sandbox = created.json()
    api.delete(
        "/v1/sandboxes/%s" % sandbox["name"], headers={"X-Cloudbox-User": OWNER}
    )
    if sandbox.get("sealed") and sandbox.get("sealVerified"):
        return True, ""
    return False, (
        "conformance run refused: unproven network policy enforcement on "
        "cluster %r (the seal's enforcement probe did not pass; state %r) — "
        "no conformance pass is reported" % (cluster_name, sandbox.get("state"))
    )
