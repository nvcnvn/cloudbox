"""Steps for the sandbox-seal capability."""

from behave import given, when, then

TWO_SERVICES = (
    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
    "spec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: web:1.0\n"
    "---\n"
    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: auth-api\n"
    "spec:\n  template:\n    spec:\n      containers:\n        - name: auth-api\n          image: auth:1.0\n"
)

NETPOL_WIDENING = (
    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n"
    "spec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: web:1.0\n"
    "---\n"
    "apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: open-egress\n"
    "spec:\n  podSelector: {}\n  egress:\n    - {}\n"
)


def _sealed_sandbox(context, allowlist=None, manifests=None):
    contract = {
        "secretNames": [], "ingressHostnames": [],
        "egressAllowlist": allowlist or [], "dependencies": [],
    }
    context.app_name = context.applications.create("web", contract=contract)["name"]
    context.sandbox_name = context.sandboxes.create(context.app_name)
    if manifests:
        context.bundles.apply(context.app_name, context.sandbox_name, manifests)
        context.bundles.last_response.raise_for_status()


def _attempt(context, destination, workload="web"):
    resp = context.api.post(
        "/simctl/sandboxes/%s/egress-attempts" % context.sandbox_name,
        json={"workload": workload, "destination": destination},
    )
    resp.raise_for_status()
    context.egress_result = resp.json()
    return context.egress_result


# --- N1: sealed before admission (task 3.7) ---


@given("a sandbox being provisioned")
def step_impl(context):
    context.app_name = context.applications.create("web")["name"]
    context.api.post("/simctl/hold-seal").raise_for_status()
    context.sandbox_name = context.sandboxes.create(context.app_name)


@when("the developer applies a bundle before the seal is in force")
def step_impl(context):
    context.manifests = context.bundles.plain_mixed_manifests()
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)


@then("no workload is admitted until default-deny ingress and egress are active")
def step_impl(context):
    assert context.bundles.last_response.status_code == 409, (
        context.bundles.last_response.text
    )
    assert "not sealed" in context.bundles.error_message()
    assert context.sandboxes.workloads(context.sandbox_name) == []
    # Once the seal is in force, the same apply is admitted.
    context.api.post(
        "/simctl/sandboxes/%s/complete-seal" % context.sandbox_name
    ).raise_for_status()
    context.bundles.apply(context.app_name, context.sandbox_name, context.manifests)
    context.bundles.last_response.raise_for_status()
    assert context.sandboxes.workloads(context.sandbox_name)


# --- N2: egress limited to allowlist (task 3.8) ---


@given('the application allowlist declares "api.stripe.com"')
def step_impl(context):
    _sealed_sandbox(context, allowlist=["api.stripe.com"])


@when('a workload connects to "{destination}"')
def step_impl(context, destination):
    _attempt(context, destination)


@then("the connection succeeds")
def step_impl(context):
    assert context.egress_result["allowed"], context.egress_result


@given('"api.other-vendor.com" is not on the application allowlist')
def step_impl(context):
    _sealed_sandbox(context, allowlist=["api.stripe.com"])


@when('a workload attempts to connect to "{destination}"')
def step_impl(context, destination):
    _attempt(context, destination)


@then("the connection is denied")
def step_impl(context):
    assert not context.egress_result["allowed"], context.egress_result


@given("a sealed sandbox running two services")
def step_impl(context):
    _sealed_sandbox(context, manifests=TWO_SERVICES)


@when("one service calls the other by its short name")
def step_impl(context):
    context.dns_result = _attempt(context, "cluster-dns")
    context.egress_result = _attempt(context, "auth-api")


@then("name resolution and the in-sandbox connection succeed")
def step_impl(context):
    assert context.dns_result["allowed"] and context.dns_result["via"] == "cluster-dns"
    assert context.egress_result["allowed"] and context.egress_result["via"] == "in-sandbox"


# --- N3: standard NetworkPolicy floor + proxy (task 3.9) ---


@given("a cluster whose CNI enforces standard NetworkPolicy v1 and has no vendor policy CRDs")
def step_impl(context):
    context.cluster = context.platform.ensure_ready_platform()
    crd_names = [c["name"] for c in context.platform.installed_crds(context.cluster)]
    assert not any("cilium" in n or "calico" in n for n in crd_names), crd_names
    context.app_name = context.applications.create("web")["name"]


@when("a sandbox is created")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name)


@then("the sandbox seals successfully")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["sealed"] and record["sealVerified"], record


@then("the only egress admitted by policy is cluster DNS and the egress proxy")
def step_impl(context):
    assert _attempt(context, "cluster-dns")["via"] == "cluster-dns"
    assert not _attempt(context, "api.undeclared-vendor.com")["allowed"]
    assert not _attempt(context, "203.0.113.7")["allowed"], "IP-literal egress must be denied"


@given("a workload speaking a database wire protocol that ignores proxy environment variables")
def step_impl(context):
    _sealed_sandbox(
        context,
        allowlist=["db.vendor.com"],
        manifests=context.bundles.prod_sized_manifests(name="db-client"),
    )


@when("the workload connects to an allowlisted database endpoint")
def step_impl(context):
    _attempt(context, "db.vendor.com", workload="db-client")


@then("the connection is transparently redirected through the egress proxy")
def step_impl(context):
    assert context.egress_result["allowed"], context.egress_result
    assert context.egress_result["via"] == "egress-proxy", context.egress_result


@then("the workload required no modification")
def step_impl(context):
    manifest = context.sandboxes.workloads(context.sandbox_name)[0]["manifest"]
    containers = manifest["spec"]["template"]["spec"]["containers"]
    env_names = [e["name"] for e in containers[0].get("env") or []]
    assert not any("PROXY" in n.upper() for n in env_names), (
        "sealing must not inject proxy variables into the workload: %s" % env_names
    )


# --- N4: blocked attempts recorded and attributed (task 3.10) ---


@given("a workload attempts to reach an undeclared endpoint")
def step_impl(context):
    _sealed_sandbox(context, manifests=context.bundles.plain_mixed_manifests())
    _attempt(context, "api.undeclared-vendor.com", workload="web")


@when("the connection is denied")
def step_impl(context):
    assert not context.egress_result["allowed"]


@then("the attempt is recorded with destination, timestamp, and the attributed workload")
def step_impl(context):
    blocked = context.sandboxes.record(context.sandbox_name).json()["blockedEgress"]
    assert blocked, "no blocked attempts recorded"
    attempt = blocked[0]
    assert attempt["destination"] == "api.undeclared-vendor.com", attempt
    assert attempt["at"] and attempt["workload"] == "web", attempt


@then("the record appears in the sandbox status and in the run's evidence")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["blockedEgress"], "status must show the blocked attempt"
    evidence = context.sandboxes.evidence(context.sandbox_name)
    assert evidence["egressViolations"] >= 1, evidence


# --- N5: never weakened per sandbox (task 3.11) ---


@given("a developer who owns a sandbox")
def step_impl(context):
    _sealed_sandbox(context)


@when("they attempt to add an FQDN to the allowlist for that sandbox only")
def step_impl(context):
    context.allowlist_attempt = context.api.post(
        "/v1/sandboxes/%s/allowlist-requests" % context.sandbox_name,
        json={"fqdn": "api.new-vendor.com"},
        headers={"X-Cloudbox-User": "dev@example.com"},
    )


@then("the request is refused")
def step_impl(context):
    assert context.allowlist_attempt.status_code == 403, context.allowlist_attempt.text


@then("the path offered is an audited application-policy change for admin review")
def step_impl(context):
    message = context.allowlist_attempt.json()["error"]
    assert "application-policy" in message and "admin" in message and "audited" in message, message


# --- N6: user policy narrows, never widens (task 3.12) ---


@given("a bundle containing a NetworkPolicy permitting egress to an undeclared endpoint")
def step_impl(context):
    context.netpol_manifests = NETPOL_WIDENING


@when("the bundle runs in the sealed sandbox")
def step_impl(context):
    _sealed_sandbox(context, manifests=context.netpol_manifests)


@then("connections to the undeclared endpoint are still denied")
def step_impl(context):
    assert not _attempt(context, "api.undeclared-vendor.com")["allowed"]


# --- N7: probe-verified enforcement (task 3.13) ---


@given("a cluster whose CNI accepts but does not enforce NetworkPolicy")
def step_impl(context):
    context.platform.ensure_ready_platform(enforcing=False)
    context.app_name = context.applications.create("web")["name"]


@when("setup or sandbox creation runs the enforcement probe")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name)


@then("the probe detects that a denied connection was not denied")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["state"] == "unsealed-cluster-not-enforcing", record


@then("the sandbox does not report itself sealed")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert not record["sealed"] and not record["sealVerified"], record


@then("the sandbox produces no evidence")
def step_impl(context):
    resp = context.api.get("/v1/sandboxes/%s/evidence" % context.sandbox_name)
    assert resp.status_code == 409, resp.text
    assert "no evidence" in resp.json()["error"], resp.text


@given("a cluster whose CNI enforces NetworkPolicy")
def step_impl(context):
    context.platform.ensure_ready_platform(enforcing=True)
    context.app_name = context.applications.create("web")["name"]


@when("the enforcement probe creates a canary workload and attempts a denied connection")
def step_impl(context):
    context.sandbox_name = context.sandboxes.create(context.app_name)


@then("the connection is denied and the seal is reported verified")
def step_impl(context):
    record = context.sandboxes.record(context.sandbox_name).json()
    assert record["sealed"] and record["sealVerified"] and record["state"] == "ready", record


# --- N8: containment claims match the declared scope (task 3.14) ---


@given("the product's published containment statement for sealed sandboxes")
def step_impl(context):
    resp = context.api.get("/v1/containment-statement")
    resp.raise_for_status()
    context.containment = resp.json()


@when("an operator reviews it")
def step_impl(context):
    context.containment_text = str(context.containment).lower()


@then("direct egress, non-allowlisted FQDNs, ingress, and production writes are listed as blocked")
def step_impl(context):
    blocked = " ".join(context.containment["blockedChannels"]).lower()
    for expectation in ["direct egress", "fqdn", "ingress", "production"]:
        assert expectation in blocked, "missing %r in blocked channels: %s" % (expectation, blocked)


@then("DNS tunneling and exfiltration through allowlisted endpoints are listed as residual channels")
def step_impl(context):
    residual = " ".join(context.containment["residualChannels"]).lower()
    assert "dns" in residual and "tunnel" in residual, residual
    assert "allowlisted endpoints" in residual, residual


@then('the word "unbypassable" appears nowhere in the claim')
def step_impl(context):
    assert "unbypassable" not in context.containment_text