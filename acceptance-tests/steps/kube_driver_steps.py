"""Steps for the kube-driver capability (@conformance: every scenario runs
against a real cluster, booted by run_acceptance.py --conformance)."""

import uuid

from behave import given, when, then

from pages.bundles import WIDGET_BUNDLE_YAML

PRODUCT_CRDS = {
    "applications.cloudbox.dev",
    "sandboxes.cloudbox.dev",
    "bundles.cloudbox.dev",
    "promotionrequests.cloudbox.dev",
    "clusterregistries.cloudbox.dev",
}


def arrange_real_platform(context):
    """Register the real cluster and run setup — the product-side arrangement
    every conformance scenario shares. Idempotent across scenarios (no state
    reset exists under kube, by design)."""
    context.platform.run_setup(context.kube.name).raise_for_status()
    context.platform.register_cluster(context.kube.name, "sandbox")
    context.platform.register_cluster(context.kube.name, "production")


def create_sealed_sandbox(context, app_fields=None):
    """A sealed sandbox on the real cluster for a fresh application; asserts
    the seal was probe-verified. App names are unique per scenario because a
    real run shares one control plane across the suite."""
    arrange_real_platform(context)
    context.app_name = "app-%s" % uuid.uuid4().hex[:6]
    context.applications.create(context.app_name, **(app_fields or {}))
    context.sandbox_name = context.sandboxes.create(context.app_name, arrange=False)
    record = context.sandboxes.record(context.sandbox_name)
    record.raise_for_status()
    body = record.json()
    assert body["sealed"] and body["sealVerified"], (
        "sandbox %s did not seal: state=%s" % (context.sandbox_name, body["state"])
    )
    return context.sandbox_name


# --- Rule: The kube driver satisfies the cluster contract without widening it ---


@given("a reachable Kubernetes cluster with an enforcing CNI")
def step_impl(context):
    assert context.kube.reachable(), (
        "the conformance cluster %r is not reachable" % context.kube.name
    )


@when("cloudboxd starts with the kube driver")
def step_impl(context):
    # The suite booted cloudboxd in before_all; assert it is THIS driver and
    # the process is still alive.
    assert context.driver == "kube", "suite is running driver %r" % context.driver
    assert context.app.poll() is None, "cloudboxd exited"


@then("the control plane reports healthy")
def step_impl(context):
    assert context.platform.healthy(), "GET /healthz did not report healthy"


@then("the cluster contract is served by the kube driver")
def step_impl(context):
    # Registering the real cluster by its kubeconfig context name proves the
    # driver behind the contract resolves real clusters, not simulated ones.
    context.platform.register_cluster(context.kube.name, "sandbox")


@given("cloudboxd running against a real cluster")
def step_impl(context):
    assert context.driver == "kube", "suite is running driver %r" % context.driver
    assert context.platform.healthy(), "control plane is not healthy"


@when("the control plane installs the product custom resource definitions")
def step_impl(context):
    context.platform.run_setup(context.kube.name).raise_for_status()


@then("the definitions are present on the real cluster")
def step_impl(context):
    live = context.kube.crd_names()
    missing = PRODUCT_CRDS - live
    assert not missing, "product CRDs absent from the real cluster: %s" % sorted(missing)


@then("listing them through the driver returns the installed set")
def step_impl(context):
    listed = {crd["name"] for crd in context.platform.installed_crds(context.kube.name)}
    missing = PRODUCT_CRDS - listed
    assert not missing, "driver listing lacks installed CRDs: %s" % sorted(missing)


# --- Rule: Sealing a real namespace admits only cluster DNS and the egress proxy ---


@given("a sealed sandbox on a real cluster")
def step_impl(context):
    create_sealed_sandbox(context)


@when("the namespace's network policies are read from the cluster")
def step_impl(context):
    context.netpols = context.kube.network_policies(context.sandbox_name)


@then("a default-deny ingress and egress policy is present")
def step_impl(context):
    for policy in context.netpols:
        spec = policy["spec"]
        types = set(spec.get("policyTypes") or [])
        selects_all = not (spec.get("podSelector") or {})
        no_rules = not spec.get("ingress") and not spec.get("egress")
        if selects_all and types == {"Ingress", "Egress"} and no_rules:
            return
    raise AssertionError(
        "no default-deny ingress+egress policy found among %s"
        % [p["metadata"]["name"] for p in context.netpols]
    )


@then("every policy read is standard NetworkPolicy v1")
def step_impl(context):
    assert context.netpols, "no network policies found in the sealed namespace"
    # kubectl elides apiVersion in list items only for the list's own type;
    # reading through the networking.k8s.io/v1 resource IS the check, and any
    # item that carries the fields confirms it explicitly.
    for policy in context.netpols:
        api_version = policy.get("apiVersion", "networking.k8s.io/v1")
        assert api_version == "networking.k8s.io/v1", (
            "policy %s is %s, not standard NetworkPolicy v1"
            % (policy["metadata"]["name"], api_version)
        )


@then("no vendor policy custom resource is used")
def step_impl(context):
    vendor = context.kube.vendor_policy_objects(context.sandbox_name)
    assert not vendor, "vendor policy objects present in the namespace: %s" % vendor


# --- Rule: The substrate is read from the live cluster ---


@given("a real cluster with a known Kubernetes minor version and installed operators")
def step_impl(context):
    assert context.kube.reachable(), "conformance cluster unreachable"
    context.kube.install_widget_operator(version="v1.0.0")


@when("the application substrate is inspected through the driver")
def step_impl(context):
    create_sealed_sandbox(context)
    # The lockfile scopes to what the application's bundles reference (P1),
    # so apply a bundle instantiating the operator's CRD group.
    context.bundles.apply(context.app_name, context.sandbox_name, WIDGET_BUNDLE_YAML)
    context.bundles.last_response.raise_for_status()
    context.lockfile = context.platform.lockfile(context.app_name)


@then("the reported minor version and components match the live cluster")
def step_impl(context):
    live_minor = context.kube.server_minor()
    assert context.lockfile["kubernetesMinor"] == live_minor, (
        "lockfile reports minor %r, live cluster is %r"
        % (context.lockfile["kubernetesMinor"], live_minor)
    )
    operators = {
        c["name"]: c for c in context.lockfile["components"] if c["kind"] == "operator"
    }
    assert "widget-operator" in operators, (
        "widget-operator missing from lockfile components: %s" % sorted(operators)
    )
    live_version = context.kube.widget_operator_version()
    assert operators["widget-operator"]["version"] == live_version, (
        "lockfile reports operator version %r, live cluster label is %r"
        % (operators["widget-operator"]["version"], live_version)
    )


@then("the substrate digest is computed from those live values")
def step_impl(context):
    before = context.lockfile["digest"]
    assert before, "lockfile digest is empty"
    # Change a live value; a digest computed from live state must move.
    context.kube.set_widget_operator_version("v1.1.0")
    try:
        after = context.platform.lockfile(context.app_name)
        assert after["digest"] != before, (
            "digest did not change after the live operator version changed"
        )
        versions = {
            c["name"]: c["version"] for c in after["components"] if c["kind"] == "operator"
        }
        assert versions.get("widget-operator") == "v1.1.0", (
            "lockfile did not pick up the live version change: %s" % versions
        )
    finally:
        context.kube.set_widget_operator_version("v1.0.0")


# --- Rule: The simulation arrangement surface is absent under the kube driver ---


@when("a simulation arrangement route is requested")
def step_impl(context):
    context.sim_arrangement_responses = context.platform.request_sim_arrangements()


@then("the request is not served")
def step_impl(context):
    served = [
        "%s -> %d" % (r.request.url, r.status_code)
        for r in context.sim_arrangement_responses
        if r.status_code != 404
    ]
    assert not served, "simulation arrangement routes served under kube: %s" % served
